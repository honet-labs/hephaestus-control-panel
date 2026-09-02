package services

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
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
		cmd = exec.CommandContext(ctx, "mysqldump",
			"-h", dbCfg.Host,
			"-P", fmt.Sprintf("%d", dbCfg.Port),
			"-u", dbCfg.Username,
			fmt.Sprintf("-p%s", dbCfg.Password),
			dbCfg.DatabaseName,
		)
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
		return nil, fmt.Errorf("%v: %s", err, stderr.String())
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
	var dumpCmd string
	switch dbCfg.DBType {
	case "postgresql":
		dumpCmd = fmt.Sprintf("PGPASSWORD='%s' pg_dump -h '%s' -p %d -U '%s' -d '%s' > '%s'",
			escapeShell(dbCfg.Password), dbCfg.Host, dbCfg.Port, dbCfg.Username, dbCfg.DatabaseName, remotePath)
	case "mysql", "mariadb":
		dumpCmd = fmt.Sprintf("mysqldump -h '%s' -P %d -u '%s' -p'%s' '%s' > '%s'",
			dbCfg.Host, dbCfg.Port, dbCfg.Username, escapeShell(dbCfg.Password), dbCfg.DatabaseName, remotePath)
	default:
		return nil, fmt.Errorf("unsupported DB type for SSH dump: %s", dbCfg.DBType)
	}

	stdout, stderr, exitCode, err := s.sshService.ExecuteCommand(remoteHostCfg, dumpCmd)
	if err != nil || exitCode != 0 {
		return nil, fmt.Errorf("remote dump command failed (exit %d): %s %s %v", exitCode, stdout, stderr, err)
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

	// Clean up remote file
	_, _, _, _ = s.sshService.ExecuteCommand(remoteHostCfg, fmt.Sprintf("rm -f '%s'", remotePath))

	return data, nil
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
