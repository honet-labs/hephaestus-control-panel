package services

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/queue"
	"go-hephaestus/internal/repository"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type BackupService struct {
	backupRepo *repository.BackupRepository
	sshService *SSHService
}

func NewBackupService(backupRepo *repository.BackupRepository, sshService *SSHService) *BackupService {
	s := &BackupService{
		backupRepo: backupRepo,
		sshService: sshService,
	}

	// Register backup job handler to worker pool
	wp := queue.GetWorkerPool()
	wp.RegisterHandler("database_backup", s.HandleBackupJob)

	return s
}

func (s *BackupService) TriggerBackup(ctx context.Context, dbConfigID, destinationID string) (string, error) {
	wp := queue.GetWorkerPool()
	job, err := wp.Enqueue("database_backup", map[string]interface{}{
		"dbConfigId":    dbConfigID,
		"destinationId": destinationID,
	}, 1)
	if err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *BackupService) HandleBackupJob(ctx context.Context, job *domain.Job, updateProgress func(progress int, msg string)) error {
	dbConfigID, _ := job.Payload["dbConfigId"].(string)
	destinationID, _ := job.Payload["destinationId"].(string)

	if dbConfigID == "" || destinationID == "" {
		return fmt.Errorf("missing dbConfigId or destinationId in job payload")
	}

	updateProgress(5, "Loading database and destination configurations...")
	dbCfg, err := s.backupRepo.GetRawDBConfig(ctx, dbConfigID)
	if err != nil {
		return fmt.Errorf("database config not found: %w", err)
	}

	dest, err := s.backupRepo.GetRawDestination(ctx, destinationID)
	if err != nil {
		return fmt.Errorf("backup destination not found: %w", err)
	}

	timestamp := time.Now().Format("2006-01-02T15-04-05")
	filename := fmt.Sprintf("%s_%s.sql.gz", dbCfg.DatabaseName, timestamp)
	historyID := fmt.Sprintf("bh-%s", uuid.New().String()[:8])

	_ = s.backupRepo.CreateHistory(ctx, domain.BackupHistoryEntry{
		ID:            historyID,
		DBConfigID:    &dbConfigID,
		DestinationID: &destinationID,
		DBName:        dbCfg.DatabaseName,
		DBType:        dbCfg.DBType,
		DestType:      dest.DestType,
		Filename:      filename,
		FileSize:      0,
		Status:        "running",
		StartedAt:     time.Now(),
	})

	updateProgress(20, fmt.Sprintf("Executing %s dump...", dbCfg.DBType))
	rawDump, err := s.executeDump(ctx, dbCfg, filename)
	if err != nil {
		errMsg := err.Error()
		_ = s.backupRepo.UpdateHistoryStatus(ctx, historyID, "failed", 0, &errMsg)
		return fmt.Errorf("dump failed: %w", err)
	}

	updateProgress(60, "Compressing dump archive (gzip)...")
	compressedData, err := compressGzip(rawDump)
	if err != nil {
		errMsg := err.Error()
		_ = s.backupRepo.UpdateHistoryStatus(ctx, historyID, "failed", 0, &errMsg)
		return fmt.Errorf("compression failed: %w", err)
	}
	fileSize := int64(len(compressedData))

	updateProgress(80, fmt.Sprintf("Uploading to destination (%s)...", dest.DestType))
	if err := s.uploadToDestination(ctx, compressedData, filename, dest); err != nil {
		errMsg := err.Error()
		_ = s.backupRepo.UpdateHistoryStatus(ctx, historyID, "failed", fileSize, &errMsg)
		return fmt.Errorf("upload failed: %w", err)
	}

	_ = s.backupRepo.UpdateHistoryStatus(ctx, historyID, "success", fileSize, nil)
	updateProgress(100, fmt.Sprintf("Backup completed (%s, %d bytes)", filename, fileSize))
	return nil
}

func (s *BackupService) executeDump(ctx context.Context, dbCfg *domain.BackupDbConfig, filename string) ([]byte, error) {
	if dbCfg.SSHHost != nil && *dbCfg.SSHHost != "" {
		return s.executeDumpSSH(ctx, dbCfg, filename)
	}
	return s.executeDumpDirect(ctx, dbCfg)
}

func (s *BackupService) executeDumpDirect(ctx context.Context, dbCfg *domain.BackupDbConfig) ([]byte, error) {
	var cmd *exec.Cmd
	switch dbCfg.DBType {
	case "postgresql":
		cmd = exec.CommandContext(ctx, "pg_dump",
			"-h", dbCfg.Host,
			"-p", fmt.Sprintf("%d", dbCfg.Port),
			"-U", dbCfg.Username,
			"-d", dbCfg.DatabaseName,
		)
		cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", dbCfg.Password))
	case "mysql", "mariadb":
		var args []string
		args = append(args, "--protocol=tcp", "-h", dbCfg.Host, "-P", fmt.Sprintf("%d", dbCfg.Port), "-u", dbCfg.Username)
		if dbCfg.Password != "" {
			args = append(args, fmt.Sprintf("-p%s", dbCfg.Password))
		}
		args = append(args, "--single-transaction", "--quick", "--skip-lock-tables", dbCfg.DatabaseName)

		dumpBin := "mysqldump"
		if p, err := exec.LookPath("mariadb-dump"); err == nil {
			dumpBin = p
		} else if p, err := exec.LookPath("mysqldump"); err == nil {
			dumpBin = p
		}
		cmd = exec.CommandContext(ctx, dumpBin, args...)
	default:
		return nil, fmt.Errorf("unsupported database type for direct dump: %s", dbCfg.DBType)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if dbCfg.DBType == "postgresql" {
			// Fallback to native pgx connection dump
			return s.dumpPostgreSQLNative(ctx, dbCfg)
		}
		errStr := strings.TrimSpace(stderr.String())
		if errStr == "" {
			errStr = err.Error()
		}
		return nil, fmt.Errorf("database dump failed: %s", errStr)
	}
	return stdout.Bytes(), nil
}

func (s *BackupService) dumpPostgreSQLNative(ctx context.Context, dbCfg *domain.BackupDbConfig) ([]byte, error) {
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		dbCfg.Username, dbCfg.Password, dbCfg.Host, dbCfg.Port, dbCfg.DatabaseName)
	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("direct pg_dump CLI unavailable and native connection failed: %w", err)
	}
	defer conn.Close(ctx)

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("-- Hephaestus PostgreSQL Native Backup Dump\n-- Database: %s\n-- Timestamp: %s\n\n", dbCfg.DatabaseName, time.Now().Format(time.RFC3339)))

	rows, err := conn.Query(ctx, `SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err == nil {
			tables = append(tables, t)
		}
	}

	for _, table := range tables {
		buf.WriteString(fmt.Sprintf("\n-- Data for Name: %s\n", table))
		dataRows, err := conn.Query(ctx, fmt.Sprintf(`SELECT * FROM "%s"`, table))
		if err != nil {
			continue
		}

		fieldDescs := dataRows.FieldDescriptions()
		colNames := make([]string, len(fieldDescs))
		for i, fd := range fieldDescs {
			colNames[i] = fmt.Sprintf(`"%s"`, string(fd.Name))
		}
		colList := strings.Join(colNames, ", ")

		for dataRows.Next() {
			vals, err := dataRows.Values()
			if err != nil {
				continue
			}
			valStrs := make([]string, len(vals))
			for i, v := range vals {
				if v == nil {
					valStrs[i] = "NULL"
				} else {
					valStrs[i] = fmt.Sprintf("'%s'", strings.ReplaceAll(fmt.Sprintf("%v", v), "'", "''"))
				}
			}
			buf.WriteString(fmt.Sprintf("INSERT INTO \"%s\" (%s) VALUES (%s);\n", table, colList, strings.Join(valStrs, ", ")))
		}
		dataRows.Close()
	}

	return buf.Bytes(), nil
}

func (s *BackupService) executeDumpSSH(ctx context.Context, dbCfg *domain.BackupDbConfig, filename string) ([]byte, error) {
	remoteHostCfg := &domain.RemoteHostConfig{
		ID:       "temp-ssh",
		Host:     *dbCfg.SSHHost,
		Port:     22,
		Username: "root",
		AuthType: "password",
	}
	if dbCfg.SSHPort != nil {
		remoteHostCfg.Port = *dbCfg.SSHPort
	}
	if dbCfg.SSHUser != nil {
		remoteHostCfg.Username = *dbCfg.SSHUser
	}
	if dbCfg.SSHAuth != nil {
		remoteHostCfg.AuthType = *dbCfg.SSHAuth
	}
	remoteHostCfg.Password = dbCfg.SSHPassword
	remoteHostCfg.SSHKey = dbCfg.SSHKey

	remotePath := fmt.Sprintf("/tmp/%s", filename)
	var passFlag string
	if dbCfg.Password != "" {
		passFlag = fmt.Sprintf("-p'%s'", escapeShell(dbCfg.Password))
	}

	var dumpCmd string
	switch dbCfg.DBType {
	case "postgresql":
		dumpCmd = fmt.Sprintf("export PATH=$PATH:/usr/local/bin:/usr/bin:/bin; if command -v pg_dump >/dev/null 2>&1; then PGPASSWORD='%s' pg_dump -h '%s' -p %d -U '%s' -d '%s' > '%s'; else echo 'NO_DUMP_CLI'; exit 127; fi",
			escapeShell(dbCfg.Password), escapeShell(dbCfg.Host), dbCfg.Port, escapeShell(dbCfg.Username), escapeShell(dbCfg.DatabaseName), escapeShell(remotePath))
	case "mysql", "mariadb":
		dumpCmd = fmt.Sprintf("export PATH=$PATH:/usr/local/bin:/usr/local/mysql/bin:/opt/lampp/bin:/usr/bin:/bin; if command -v mariadb-dump >/dev/null 2>&1; then mariadb-dump -h '%s' -P %d -u '%s' %s '%s' > '%s'; elif command -v mysqldump >/dev/null 2>&1; then mysqldump -h '%s' -P %d -u '%s' %s '%s' > '%s'; else echo 'NO_DUMP_CLI'; exit 127; fi",
			escapeShell(dbCfg.Host), dbCfg.Port, escapeShell(dbCfg.Username), passFlag, escapeShell(dbCfg.DatabaseName), escapeShell(remotePath),
			escapeShell(dbCfg.Host), dbCfg.Port, escapeShell(dbCfg.Username), passFlag, escapeShell(dbCfg.DatabaseName), escapeShell(remotePath))
	default:
		return nil, fmt.Errorf("unsupported DB type for SSH dump: %s", dbCfg.DBType)
	}

	stdout, stderr, exitCode, err := s.sshService.ExecuteCommand(remoteHostCfg, dumpCmd)
	if err != nil || exitCode != 0 {
		errDetail := strings.TrimSpace(stderr + "\n" + stdout)
		// If remote host does not have mysqldump/mariadb-dump installed (exit code 127 or command not found):
		// Automatically fallback to secure SSH tunnel dump using Hephaestus local engine!
		if exitCode == 127 || strings.Contains(errDetail, "NO_DUMP_CLI") || strings.Contains(errDetail, "command not found") || strings.Contains(errDetail, "not found") {
			return s.executeDumpViaSSHTunnel(ctx, dbCfg, filename)
		}
		if errDetail == "" && err != nil {
			errDetail = err.Error()
		}
		return nil, fmt.Errorf("remote dump command failed (exit %d): %s", exitCode, errDetail)
	}

	// Download dump file via SFTP with config directly
	reader, _, err := s.sshService.SftpDownloadWithConfig(remoteHostCfg, remotePath)
	if err != nil {
		return nil, fmt.Errorf("failed to download remote dump: %w", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	// Clean up remote file safely
	_, _, _, _ = s.sshService.ExecuteCommand(remoteHostCfg, fmt.Sprintf("rm -f '%s'", escapeShell(remotePath)))

	return data, nil
}

// withSSHTunnel creates a dynamic, secure SSH port forwarding tunnel to the database host
func (s *BackupService) withSSHTunnel(ctx context.Context, dbCfg *domain.BackupDbConfig, fn func(tunneledCfg *domain.BackupDbConfig) error) error {
	remoteHostCfg := &domain.RemoteHostConfig{
		ID:       "temp-ssh",
		Host:     *dbCfg.SSHHost,
		Port:     22,
		Username: "root",
		AuthType: "password",
	}
	if dbCfg.SSHPort != nil {
		remoteHostCfg.Port = *dbCfg.SSHPort
	}
	if dbCfg.SSHUser != nil {
		remoteHostCfg.Username = *dbCfg.SSHUser
	}
	if dbCfg.SSHAuth != nil {
		remoteHostCfg.AuthType = *dbCfg.SSHAuth
	}
	remoteHostCfg.Password = dbCfg.SSHPassword
	remoteHostCfg.SSHKey = dbCfg.SSHKey

	sshClient, err := s.sshService.Dial(remoteHostCfg)
	if err != nil {
		return fmt.Errorf("failed to establish SSH connection to %s: %w", remoteHostCfg.Host, err)
	}
	defer sshClient.Close()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to create local tunnel listener: %w", err)
	}
	defer listener.Close()

	localPort := listener.Addr().(*net.TCPAddr).Port
	tunnelCtx, cancelTunnel := context.WithCancel(ctx)
	defer cancelTunnel()

	targetDBHost := dbCfg.Host
	if targetDBHost == "" || targetDBHost == "localhost" {
		targetDBHost = "127.0.0.1"
	}
	targetDBPort := dbCfg.Port
	if targetDBPort <= 0 {
		switch dbCfg.DBType {
		case "postgresql":
			targetDBPort = 5432
		default:
			targetDBPort = 3306
		}
	}

	go func() {
		for {
			localConn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(lConn net.Conn) {
				defer lConn.Close()
				remoteConn, err := sshClient.Dial("tcp", fmt.Sprintf("%s:%d", targetDBHost, targetDBPort))
				if err != nil {
					return
				}
				defer remoteConn.Close()

				done := make(chan struct{}, 2)
				go func() {
					_, _ = io.Copy(remoteConn, lConn)
					done <- struct{}{}
				}()
				go func() {
					_, _ = io.Copy(lConn, remoteConn)
					done <- struct{}{}
				}()

				select {
				case <-done:
				case <-tunnelCtx.Done():
				}
			}(localConn)
		}
	}()

	tunneledCfg := *dbCfg
	tunneledCfg.Host = "127.0.0.1"
	tunneledCfg.Port = localPort
	return fn(&tunneledCfg)
}

func (s *BackupService) executeDumpViaSSHTunnel(ctx context.Context, dbCfg *domain.BackupDbConfig, filename string) ([]byte, error) {
	var dumpBytes []byte
	err := s.withSSHTunnel(ctx, dbCfg, func(tunneledCfg *domain.BackupDbConfig) error {
		res, err := s.executeDumpDirect(ctx, tunneledCfg)
		if err != nil {
			return err
		}
		dumpBytes = res
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("SSH tunnel dump failed: %w", err)
	}
	return dumpBytes, nil
}

func (s *BackupService) testDBConfigViaSSHTunnel(ctx context.Context, dbCfg *domain.BackupDbConfig) (string, error) {
	var resultMsg string
	err := s.withSSHTunnel(ctx, dbCfg, func(tunneledCfg *domain.BackupDbConfig) error {
		res, err := s.testDBConfigDirect(ctx, tunneledCfg)
		if err != nil {
			return err
		}
		resultMsg = res
		return nil
	})
	if err != nil {
		return "", err
	}
	return resultMsg, nil
}

func (s *BackupService) uploadToDestination(ctx context.Context, data []byte, filename string, dest *domain.BackupDestination) error {
	switch dest.DestType {
	case "local":
		path, _ := dest.Config["path"].(string)
		if path == "" {
			path = "backups"
		}
		_ = os.MkdirAll(path, 0755)
		targetFile := filepath.Join(path, filename)
		return os.WriteFile(targetFile, data, 0644)

	case "nas", "nfs", "nas_ssh":
		host, _ := dest.Config["host"].(string)
		if host == "" {
			// If no remote host specified, treat as local filesystem/mount path
			path, _ := dest.Config["path"].(string)
			if path == "" {
				path = "/opt/backups"
			}
			_ = os.MkdirAll(path, 0755)
			targetFile := filepath.Join(path, filename)
			return os.WriteFile(targetFile, data, 0644)
		}

		port := 22
		if p, ok := dest.Config["port"].(float64); ok && p > 0 {
			port = int(p)
		} else if pStr, ok := dest.Config["port"].(string); ok && pStr != "" {
			fmt.Sscanf(pStr, "%d", &port)
		}

		username, _ := dest.Config["username"].(string)
		if username == "" {
			username = "root"
		}
		authType, _ := dest.Config["authType"].(string)
		if authType == "" {
			authType = "password"
		}
		password, _ := dest.Config["password"].(string)
		sshKey, _ := dest.Config["sshKey"].(string)
		backupPath, _ := dest.Config["path"].(string)
		if backupPath == "" {
			backupPath = "/opt/backups"
		}

		remoteHostCfg := &domain.RemoteHostConfig{
			ID:       "nas-dest",
			Host:     host,
			Port:     port,
			Username: username,
			AuthType: authType,
		}
		if password != "" {
			remoteHostCfg.Password = &password
		}
		if sshKey != "" {
			remoteHostCfg.SSHKey = &sshKey
		}

		remoteFile := filepath.ToSlash(filepath.Join(backupPath, filename))
		return s.sshService.SftpUploadWithConfig(remoteHostCfg, remoteFile, bytes.NewReader(data))

	case "r2", "s3":
		bucket, _ := dest.Config["bucket"].(string)
		endpoint, _ := dest.Config["endpoint"].(string)
		accessKey, _ := dest.Config["accessKeyId"].(string)
		secretKey, _ := dest.Config["secretAccessKey"].(string)

		if endpoint == "" {
			accountID, _ := dest.Config["accountId"].(string)
			if accountID != "" {
				endpoint = fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)
			}
		}

		// Sanitize endpoint: trim trailing slashes and bucket name if copied directly from Cloudflare S3 API URL
		if endpoint != "" {
			endpoint = strings.TrimRight(endpoint, "/")
			if bucket != "" && strings.HasSuffix(endpoint, "/"+bucket) {
				endpoint = strings.TrimSuffix(endpoint, "/"+bucket)
			}
		}

		customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			if endpoint != "" {
				return aws.Endpoint{URL: endpoint, SigningRegion: "auto"}, nil
			}
			return aws.Endpoint{}, &aws.EndpointNotFoundError{}
		})

		cfg, err := awsConfig.LoadDefaultConfig(ctx,
			awsConfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
			awsConfig.WithEndpointResolverWithOptions(customResolver),
			awsConfig.WithRegion("auto"),
		)
		if err != nil {
			return err
		}

		s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
			o.UsePathStyle = true
		})

		_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(filename),
			Body:   bytes.NewReader(data),
		})
		return err

	default:
		return fmt.Errorf("unsupported backup destination type: %s", dest.DestType)
	}
}

// TestDestination verifies write capability to any backup storage target
func (s *BackupService) TestDestination(ctx context.Context, dest *domain.BackupDestination) error {
	testData := []byte(fmt.Sprintf("Hephaestus Connection Test at %s", time.Now().Format(time.RFC3339)))
	testFilename := fmt.Sprintf(".hephaestus_test_%d.txt", time.Now().Unix())
	return s.uploadToDestination(ctx, testData, testFilename, dest)
}

// TestDBConfig verifies connectivity, authentication, and database access for a database target
func (s *BackupService) TestDBConfig(ctx context.Context, dbCfg *domain.BackupDbConfig) (string, error) {
	if dbCfg.SSHHost != nil && *dbCfg.SSHHost != "" {
		return s.testDBConfigSSH(ctx, dbCfg)
	}
	return s.testDBConfigDirect(ctx, dbCfg)
}

func (s *BackupService) testDBConfigSSH(ctx context.Context, dbCfg *domain.BackupDbConfig) (string, error) {
	remoteHostCfg := &domain.RemoteHostConfig{
		ID:       "temp-test-ssh",
		Host:     *dbCfg.SSHHost,
		Port:     22,
		Username: "root",
		AuthType: "password",
	}
	if dbCfg.SSHPort != nil && *dbCfg.SSHPort > 0 {
		remoteHostCfg.Port = *dbCfg.SSHPort
	}
	if dbCfg.SSHUser != nil && *dbCfg.SSHUser != "" {
		remoteHostCfg.Username = *dbCfg.SSHUser
	}
	if dbCfg.SSHAuth != nil && *dbCfg.SSHAuth != "" {
		remoteHostCfg.AuthType = *dbCfg.SSHAuth
	}
	remoteHostCfg.Password = dbCfg.SSHPassword
	remoteHostCfg.SSHKey = dbCfg.SSHKey

	// 1. Verify SSH connectivity first
	sshOk, sshMsg := s.sshService.TestConnection(remoteHostCfg)
	if !sshOk {
		return "", fmt.Errorf("SSH connection failed: %s", sshMsg)
	}

	// 2. Prepare database probe command on remote host
	var passFlag string
	if dbCfg.Password != "" {
		passFlag = fmt.Sprintf("-p'%s'", escapeShell(dbCfg.Password))
	}

	var testCmd string
	switch dbCfg.DBType {
	case "postgresql":
		testCmd = fmt.Sprintf(
			"if command -v pg_isready >/dev/null 2>&1; then PGPASSWORD='%s' psql -h '%s' -p %d -U '%s' -d '%s' -c 'SELECT version();' 2>&1; elif command -v pg_dump >/dev/null 2>&1; then PGPASSWORD='%s' pg_dump --schema-only -h '%s' -p %d -U '%s' -d '%s' 2>&1; else (timeout 4 bash -c '</dev/tcp/%s/%d') 2>&1 && echo 'TCP_PORT_OK' || echo 'PORT_UNREACHABLE'; fi",
			escapeShell(dbCfg.Password), escapeShell(dbCfg.Host), dbCfg.Port, escapeShell(dbCfg.Username), escapeShell(dbCfg.DatabaseName),
			escapeShell(dbCfg.Password), escapeShell(dbCfg.Host), dbCfg.Port, escapeShell(dbCfg.Username), escapeShell(dbCfg.DatabaseName),
			escapeShell(dbCfg.Host), dbCfg.Port,
		)
	case "mysql", "mariadb":
		testCmd = fmt.Sprintf(
			"if command -v mariadb >/dev/null 2>&1; then mariadb -h '%s' -P %d -u '%s' %s -e 'SELECT VERSION();' '%s' 2>&1; elif command -v mysql >/dev/null 2>&1; then mysql -h '%s' -P %d -u '%s' %s -e 'SELECT VERSION();' '%s' 2>&1; elif command -v mysqldump >/dev/null 2>&1; then mysqldump --no-data -h '%s' -P %d -u '%s' %s '%s' 2>&1; else (timeout 4 bash -c '</dev/tcp/%s/%d') 2>&1 && echo 'TCP_PORT_OK' || echo 'PORT_UNREACHABLE'; fi",
			escapeShell(dbCfg.Host), dbCfg.Port, escapeShell(dbCfg.Username), passFlag, escapeShell(dbCfg.DatabaseName),
			escapeShell(dbCfg.Host), dbCfg.Port, escapeShell(dbCfg.Username), passFlag, escapeShell(dbCfg.DatabaseName),
			escapeShell(dbCfg.Host), dbCfg.Port, escapeShell(dbCfg.Username), passFlag, escapeShell(dbCfg.DatabaseName),
			escapeShell(dbCfg.Host), dbCfg.Port,
		)
	default:
		return "", fmt.Errorf("unsupported database type: %s", dbCfg.DBType)
	}

	stdout, stderr, exitCode, err := s.sshService.ExecuteCommand(remoteHostCfg, testCmd)
	output := strings.TrimSpace(stdout + "\n" + stderr)

	if err != nil || exitCode != 0 {
		cleanOut := strings.TrimSpace(output)
		if cleanOut == "" {
			cleanOut = fmt.Sprintf("exit code %d", exitCode)
		}
		var meaningfulLines []string
		for _, line := range strings.Split(cleanOut, "\n") {
			l := strings.TrimSpace(line)
			if l != "" && !strings.Contains(l, "[Warning] Using a password") {
				meaningfulLines = append(meaningfulLines, l)
			}
		}
		if len(meaningfulLines) > 0 {
			cleanOut = strings.Join(meaningfulLines, "; ")
		}
		return "", fmt.Errorf("%s", cleanOut)
	}

	if strings.Contains(output, "TCP_PORT_OK") || strings.Contains(output, "command not found") || strings.Contains(output, "not found") {
		// Attempt SSH tunnel direct test to verify credentials
		tunnelMsg, tunnelErr := s.testDBConfigViaSSHTunnel(ctx, dbCfg)
		if tunnelErr == nil {
			return fmt.Sprintf("SSH connection established & verified database '%s' via SSH tunnel (no remote CLI required).", dbCfg.DatabaseName), nil
		}
		_ = tunnelMsg
		return fmt.Sprintf("SSH connection established & database port %d is reachable on %s (CLI client not present on host).", dbCfg.Port, dbCfg.Host), nil
	}

	lines := strings.Split(output, "\n")
	var ver string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" && !strings.HasPrefix(l, "VERSION()") && !strings.HasPrefix(l, "version") && !strings.HasPrefix(l, "--") && !strings.HasPrefix(l, "(") && !strings.Contains(l, "[Warning]") {
			ver = l
			break
		}
	}

	if ver != "" {
		return fmt.Sprintf("Connection successful via SSH! %s (Database: %s)", ver, dbCfg.DatabaseName), nil
	}

	return fmt.Sprintf("Connection successful via SSH to %s (%s:%d) on database '%s'.", dbCfg.DBType, dbCfg.Host, dbCfg.Port, dbCfg.DatabaseName), nil
}

func (s *BackupService) testDBConfigDirect(ctx context.Context, dbCfg *domain.BackupDbConfig) (string, error) {
	testCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	switch dbCfg.DBType {
	case "postgresql":
		connConfig, err := pgx.ParseConfig("")
		if err == nil {
			connConfig.Host = dbCfg.Host
			connConfig.Port = uint16(dbCfg.Port)
			connConfig.Database = dbCfg.DatabaseName
			connConfig.User = dbCfg.Username
			connConfig.Password = dbCfg.Password
			connConfig.ConnectTimeout = 6 * time.Second

			conn, connErr := pgx.ConnectConfig(testCtx, connConfig)
			if connErr == nil {
				defer conn.Close(testCtx)
				var ver string
				_ = conn.QueryRow(testCtx, "SELECT version();").Scan(&ver)
				if ver != "" {
					parts := strings.Split(ver, ",")
					return fmt.Sprintf("Connection successful! %s (Database: %s)", strings.TrimSpace(parts[0]), dbCfg.DatabaseName), nil
				}
				return fmt.Sprintf("Connection successful! Connected to PostgreSQL database '%s'.", dbCfg.DatabaseName), nil
			}
		}

		cmd := exec.CommandContext(testCtx, "pg_dump", "--schema-only", "-h", dbCfg.Host, "-p", fmt.Sprintf("%d", dbCfg.Port), "-U", dbCfg.Username, "-d", dbCfg.DatabaseName)
		cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", dbCfg.Password))
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if cliErr := cmd.Run(); cliErr != nil {
			errStr := strings.TrimSpace(stderr.String())
			if errStr != "" {
				return "", fmt.Errorf("%s", errStr)
			}
			return "", fmt.Errorf("PostgreSQL connection failed: %v", cliErr)
		}
		return fmt.Sprintf("Connection successful! Verified PostgreSQL database '%s'.", dbCfg.DatabaseName), nil

	case "mysql", "mariadb":
		var passArg string
		var args []string
		args = append(args, "-h", dbCfg.Host, "-P", fmt.Sprintf("%d", dbCfg.Port), "-u", dbCfg.Username)
		if dbCfg.Password != "" {
			passArg = fmt.Sprintf("-p%s", dbCfg.Password)
			args = append(args, passArg)
		}
		clientBin := "mysql"
		if p, err := exec.LookPath("mariadb"); err == nil {
			clientBin = p
		} else if p, err := exec.LookPath("mysql"); err == nil {
			clientBin = p
		}
		cmd := exec.CommandContext(testCtx, clientBin, args...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			errStr := strings.TrimSpace(stderr.String())
			if errStr != "" {
				var cleanLines []string
				for _, line := range strings.Split(errStr, "\n") {
					l := strings.TrimSpace(line)
					if l != "" && !strings.Contains(l, "[Warning] Using a password") {
						cleanLines = append(cleanLines, l)
					}
				}
				if len(cleanLines) > 0 {
					return "", fmt.Errorf("%s", strings.Join(cleanLines, "; "))
				}
			}

			dumpArgs := []string{"--no-data", "-h", dbCfg.Host, "-P", fmt.Sprintf("%d", dbCfg.Port), "-u", dbCfg.Username}
			if dbCfg.Password != "" {
				dumpArgs = append(dumpArgs, passArg)
			}
			dumpBin := "mysqldump"
			if p, err := exec.LookPath("mariadb-dump"); err == nil {
				dumpBin = p
			} else if p, err := exec.LookPath("mysqldump"); err == nil {
				dumpBin = p
			}
			dumpCmd := exec.CommandContext(testCtx, dumpBin, dumpArgs...)
			var dumpErr bytes.Buffer
			dumpCmd.Stderr = &dumpErr
			if dErr := dumpCmd.Run(); dErr != nil {
				dStr := strings.TrimSpace(dumpErr.String())
				if dStr != "" {
					return "", fmt.Errorf("%s", dStr)
				}
				conn, netErr := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", dbCfg.Host, dbCfg.Port), 4*time.Second)
				if netErr != nil {
					return "", fmt.Errorf("cannot reach %s:%d: %w", dbCfg.Host, dbCfg.Port, netErr)
				}
				_ = conn.Close()
				return "", fmt.Errorf("database query failed: %v", err)
			}
			return fmt.Sprintf("Connection successful! Verified database '%s'.", dbCfg.DatabaseName), nil
		}

		outStr := stdout.String()
		lines := strings.Split(outStr, "\n")
		var ver string
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if l != "" && !strings.Contains(l, "VERSION()") && !strings.Contains(l, "[Warning]") {
				ver = l
				break
			}
		}
		if ver != "" {
			return fmt.Sprintf("Connection successful! %s (Database: %s)", ver, dbCfg.DatabaseName), nil
		}
		return fmt.Sprintf("Connection successful! Connected to database '%s'.", dbCfg.DatabaseName), nil

	default:
		return "", fmt.Errorf("unsupported database type: %s", dbCfg.DBType)
	}
}

func compressGzip(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(data); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func escapeShell(s string) string {
	return strings.ReplaceAll(s, "'", "'\\''")
}
