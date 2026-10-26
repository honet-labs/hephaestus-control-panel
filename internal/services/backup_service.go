package services

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"
	"go-hephaestus/internal/queue"
	"go-hephaestus/internal/repository"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ActiveBackupTaskLog struct {
	Step      string `json:"step"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

type ActiveBackupTask struct {
	ID        string                `json:"id"`
	Type      string                `json:"type"` // "backup" or "restore"
	Target    string                `json:"target"`
	StartedAt time.Time             `json:"startedAt"`
	Status    string                `json:"status"` // "running", "success", "failed"
	Error     string                `json:"error,omitempty"`
	Logs      []ActiveBackupTaskLog `json:"logs"`
	listeners map[chan ActiveBackupTaskLog]bool
	done      chan struct{}
	mu        sync.RWMutex
}

func (t *ActiveBackupTask) GetLogsSnapshot() []ActiveBackupTaskLog {
	t.mu.RLock()
	defer t.mu.RUnlock()
	snapshot := make([]ActiveBackupTaskLog, len(t.Logs))
	copy(snapshot, t.Logs)
	return snapshot
}

type BackupService struct {
	backupRepo    *repository.BackupRepository
	sshService    *SSHService
	dockerService *DockerService
	activeTasks   map[string]*ActiveBackupTask
	taskMu        sync.RWMutex
}

func NewBackupService(backupRepo *repository.BackupRepository, sshService *SSHService, dockerService *DockerService) *BackupService {
	s := &BackupService{
		backupRepo:    backupRepo,
		sshService:    sshService,
		dockerService: dockerService,
		activeTasks:   make(map[string]*ActiveBackupTask),
	}

	// Register backup job handlers to worker pool
	wp := queue.GetWorkerPool()
	wp.RegisterHandler("database_backup", s.HandleBackupJob)
	wp.RegisterHandler("container_backup", s.HandleContainerBackupJob)

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

func (s *BackupService) HandleContainerBackupJob(ctx context.Context, job *domain.Job, updateProgress func(progress int, msg string)) error {
	connectionID, _ := job.Payload["connectionId"].(string)
	containerID, _ := job.Payload["containerId"].(string)
	stopDuringBackup, _ := job.Payload["stopDuringBackup"].(bool)

	if containerID == "" {
		return fmt.Errorf("missing containerId in job payload")
	}

	updateProgress(10, fmt.Sprintf("Starting container backup for %s...", containerID))

	req := domain.BackupContainerRunRequest{
		ConnectionID:     connectionID,
		ContainerID:      containerID,
		StopDuringBackup: stopDuringBackup,
	}

	_, err := s.BackupContainerStream(ctx, req, func(step, status, message string) {
		progress := 50
		if step == "archive" {
			progress = 80
		} else if step == "completed" {
			progress = 100
		}
		updateProgress(progress, message)
	})
	return err
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

	tempDir := ResolveLocalBackupPath("")
	_ = os.MkdirAll(tempDir, 0755)
	tempFilePath := filepath.Join(tempDir, fmt.Sprintf(".tmp_%s_%s", uuid.New().String()[:8], filename))

	updateProgress(20, fmt.Sprintf("Executing %s streaming dump...", dbCfg.DBType))
	fileSize, err := s.executeDumpStreamingToFile(ctx, dbCfg, filename, tempFilePath)
	if err != nil {
		_ = os.Remove(tempFilePath)
		errMsg := err.Error()
		_ = s.backupRepo.UpdateHistoryStatus(ctx, historyID, "failed", 0, &errMsg)
		return fmt.Errorf("dump failed: %w", err)
	}

	updateProgress(80, fmt.Sprintf("Uploading archive to destination (%s)...", dest.DestType))
	if err := s.uploadFileToDestination(ctx, tempFilePath, filename, dest); err != nil {
		_ = os.Remove(tempFilePath)
		errMsg := err.Error()
		_ = s.backupRepo.UpdateHistoryStatus(ctx, historyID, "failed", fileSize, &errMsg)
		return fmt.Errorf("upload failed: %w", err)
	}

	_ = s.backupRepo.UpdateHistoryStatus(ctx, historyID, "success", fileSize, nil)
	updateProgress(100, fmt.Sprintf("Backup completed (%s, %d bytes)", filename, fileSize))
	return nil
}

func (s *BackupService) executeDumpStreamingToFile(ctx context.Context, dbCfg *domain.BackupDbConfig, filename, targetFilePath string) (int64, error) {
	if dbCfg.SSHHost != nil && *dbCfg.SSHHost != "" {
		return s.executeDumpSSHToFile(ctx, dbCfg, filename, targetFilePath)
	}
	return s.executeDumpDirectToFile(ctx, dbCfg, targetFilePath)
}

func (s *BackupService) executeDumpDirectToFile(ctx context.Context, dbCfg *domain.BackupDbConfig, targetFilePath string) (int64, error) {
	outFile, err := os.OpenFile(targetFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return 0, fmt.Errorf("failed to create temporary backup file: %w", err)
	}
	defer outFile.Close()

	gw := gzip.NewWriter(outFile)
	defer gw.Close()

	var cmd *exec.Cmd
	switch dbCfg.DBType {
	case "postgresql":
		dumpBin := "pg_dump"
		if p, err := exec.LookPath("pg_dump"); err == nil {
			dumpBin = p
		} else {
			// pg_dump binary is not present in PATH, fallback to native PostgreSQL dump
			_ = gw.Close()
			_ = outFile.Close()
			_ = os.Remove(targetFilePath)
			return s.dumpPostgreSQLNativeToFile(ctx, dbCfg, targetFilePath)
		}
		cmd = exec.CommandContext(ctx, dumpBin,
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
		return 0, fmt.Errorf("unsupported database type for direct dump: %s", dbCfg.DBType)
	}

	var stderr bytes.Buffer
	cmd.Stdout = gw
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errStr := strings.TrimSpace(stderr.String())
		if errStr == "" {
			errStr = err.Error()
		}
		if dbCfg.DBType == "postgresql" {
			// If pg_dump failed with clear network/credential errors, report them directly without misleading fallback
			if strings.Contains(errStr, "Connection refused") ||
				strings.Contains(errStr, "connection refused") ||
				strings.Contains(errStr, "authentication failed") ||
				strings.Contains(errStr, "does not exist") ||
				strings.Contains(errStr, "could not translate host name") {
				return 0, fmt.Errorf("pg_dump failed: %s", errStr)
			}
			// Attempt native fallback in case of syntax or version mismatch
			_ = gw.Close()
			_ = outFile.Close()
			_ = os.Remove(targetFilePath)
			nSize, nErr := s.dumpPostgreSQLNativeToFile(ctx, dbCfg, targetFilePath)
			if nErr == nil {
				return nSize, nil
			}
			return 0, fmt.Errorf("pg_dump failed: %s (native fallback failed: %v)", errStr, nErr)
		}
		return 0, fmt.Errorf("database dump failed: %s", errStr)
	}

	if err := gw.Close(); err != nil {
		return 0, err
	}
	if err := outFile.Close(); err != nil {
		return 0, err
	}

	fi, err := os.Stat(targetFilePath)
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

func (s *BackupService) dumpPostgreSQLNativeToFile(ctx context.Context, dbCfg *domain.BackupDbConfig, targetFilePath string) (int64, error) {
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		dbCfg.Username, dbCfg.Password, dbCfg.Host, dbCfg.Port, dbCfg.DatabaseName)
	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		return 0, fmt.Errorf("failed to connect via native PostgreSQL driver: %w", err)
	}
	defer conn.Close(ctx)

	outFile, err := os.OpenFile(targetFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return 0, err
	}
	defer outFile.Close()

	gw := gzip.NewWriter(outFile)
	defer gw.Close()

	header := fmt.Sprintf("-- Hephaestus PostgreSQL Native Backup Dump\n-- Database: %s\n-- Timestamp: %s\n\n", dbCfg.DatabaseName, time.Now().Format(time.RFC3339))
	if _, err := io.WriteString(gw, header); err != nil {
		return 0, err
	}

	rows, err := conn.Query(ctx, `SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename`)
	if err != nil {
		return 0, err
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
		if _, err := io.WriteString(gw, fmt.Sprintf("\n-- Data for Name: %s\n", table)); err != nil {
			return 0, err
		}
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
			line := fmt.Sprintf("INSERT INTO \"%s\" (%s) VALUES (%s);\n", table, colList, strings.Join(valStrs, ", "))
			if _, err := io.WriteString(gw, line); err != nil {
				dataRows.Close()
				return 0, err
			}
		}
		dataRows.Close()
	}

	if err := gw.Close(); err != nil {
		return 0, err
	}
	if err := outFile.Close(); err != nil {
		return 0, err
	}

	fi, err := os.Stat(targetFilePath)
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

func (s *BackupService) executeDumpSSHToFile(ctx context.Context, dbCfg *domain.BackupDbConfig, filename, targetFilePath string) (int64, error) {
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
		dumpCmd = fmt.Sprintf("export PATH=$PATH:/usr/local/bin:/usr/bin:/bin; if command -v pg_dump >/dev/null 2>&1; then PGPASSWORD='%s' pg_dump -h '%s' -p %d -U '%s' -d '%s' | gzip > '%s'; else echo 'NO_DUMP_CLI'; exit 127; fi",
			escapeShell(dbCfg.Password), escapeShell(dbCfg.Host), dbCfg.Port, escapeShell(dbCfg.Username), escapeShell(dbCfg.DatabaseName), escapeShell(remotePath))
	case "mysql", "mariadb":
		dumpCmd = fmt.Sprintf("export PATH=$PATH:/usr/local/bin:/usr/local/mysql/bin:/opt/lampp/bin:/usr/bin:/bin; if command -v mariadb-dump >/dev/null 2>&1; then mariadb-dump -h '%s' -P %d -u '%s' %s '%s' | gzip > '%s'; elif command -v mysqldump >/dev/null 2>&1; then mysqldump -h '%s' -P %d -u '%s' %s '%s' | gzip > '%s'; else echo 'NO_DUMP_CLI'; exit 127; fi",
			escapeShell(dbCfg.Host), dbCfg.Port, escapeShell(dbCfg.Username), passFlag, escapeShell(dbCfg.DatabaseName), escapeShell(remotePath),
			escapeShell(dbCfg.Host), dbCfg.Port, escapeShell(dbCfg.Username), passFlag, escapeShell(dbCfg.DatabaseName), escapeShell(remotePath))
	default:
		return 0, fmt.Errorf("unsupported DB type for SSH dump: %s", dbCfg.DBType)
	}

	stdout, stderr, exitCode, err := s.sshService.ExecuteCommand(remoteHostCfg, dumpCmd)
	if err != nil || exitCode != 0 {
		errDetail := strings.TrimSpace(stderr + "\n" + stdout)
		if exitCode == 127 || strings.Contains(errDetail, "NO_DUMP_CLI") || strings.Contains(errDetail, "command not found") || strings.Contains(errDetail, "not found") {
			return s.executeDumpViaSSHTunnelToFile(ctx, dbCfg, filename, targetFilePath)
		}
		if errDetail == "" && err != nil {
			errDetail = err.Error()
		}
		return 0, fmt.Errorf("remote dump command failed (exit %d): %s", exitCode, errDetail)
	}

	// Download dump file via SFTP directly to targetFilePath on disk
	reader, _, err := s.sshService.SftpDownloadWithConfig(remoteHostCfg, remotePath)
	if err != nil {
		return 0, fmt.Errorf("failed to download remote dump: %w", err)
	}
	defer reader.Close()

	localFile, err := os.OpenFile(targetFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return 0, err
	}
	defer localFile.Close()

	if _, err := io.Copy(localFile, reader); err != nil {
		return 0, err
	}

	// Clean up remote file safely
	_, _, _, _ = s.sshService.ExecuteCommand(remoteHostCfg, fmt.Sprintf("rm -f '%s'", escapeShell(remotePath)))

	fi, err := localFile.Stat()
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

func (s *BackupService) executeDumpViaSSHTunnelToFile(ctx context.Context, dbCfg *domain.BackupDbConfig, filename, targetFilePath string) (int64, error) {
	var size int64
	err := s.withSSHTunnel(ctx, dbCfg, func(tunneledCfg *domain.BackupDbConfig) error {
		s, err := s.executeDumpDirectToFile(ctx, tunneledCfg, targetFilePath)
		if err != nil {
			return err
		}
		size = s
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("SSH tunnel dump failed: %w", err)
	}
	return size, nil
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
		dumpBin := "pg_dump"
		if p, err := exec.LookPath("pg_dump"); err == nil {
			dumpBin = p
		} else {
			// pg_dump binary is not present in PATH, fallback to native PostgreSQL dump
			return s.dumpPostgreSQLNative(ctx, dbCfg)
		}
		cmd = exec.CommandContext(ctx, dumpBin,
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
		errStr := strings.TrimSpace(stderr.String())
		if errStr == "" {
			errStr = err.Error()
		}
		if dbCfg.DBType == "postgresql" {
			if strings.Contains(errStr, "Connection refused") ||
				strings.Contains(errStr, "connection refused") ||
				strings.Contains(errStr, "authentication failed") ||
				strings.Contains(errStr, "does not exist") ||
				strings.Contains(errStr, "could not translate host name") {
				return nil, fmt.Errorf("pg_dump failed: %s", errStr)
			}
			data, nErr := s.dumpPostgreSQLNative(ctx, dbCfg)
			if nErr == nil {
				return data, nil
			}
			return nil, fmt.Errorf("pg_dump failed: %s (native fallback failed: %v)", errStr, nErr)
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
		return nil, fmt.Errorf("failed to connect via native PostgreSQL driver: %w", err)
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

// ResolveLocalBackupPath resolves any user-specified path to the persistent backup volume on host
func ResolveLocalBackupPath(rawPath string) string {
	rawPath = strings.TrimSpace(rawPath)
	if rawPath == "" || rawPath == "." {
		return "/app/backups"
	}

	cleaned := filepath.Clean(filepath.ToSlash(rawPath))

	// Direct persistent container mount targets
	if cleaned == "/app/backups" || cleaned == "/opt/backups" || cleaned == "backups" {
		return "/app/backups"
	}
	if strings.HasPrefix(cleaned, "/app/backups/") {
		return cleaned
	}
	if strings.HasPrefix(cleaned, "/opt/backups/") {
		return "/app/backups/" + strings.TrimPrefix(cleaned, "/opt/backups/")
	}

	// Normalizing /backup or /backups variants
	if cleaned == "/backup" {
		return "/app/backups"
	}
	if strings.HasPrefix(cleaned, "/backup/") {
		return filepath.Join("/app/backups", strings.TrimPrefix(cleaned, "/backup/"))
	}
	if strings.HasPrefix(cleaned, "/backups/") {
		return filepath.Join("/app/backups", strings.TrimPrefix(cleaned, "/backups/"))
	}

	// Relative paths (e.g. "database", "mysql/daily")
	if !filepath.IsAbs(cleaned) {
		return filepath.Join("/app/backups", cleaned)
	}

	// Arbitrary absolute paths like /database or /opt/hephaestus/database:
	// Route safely to /app/backups to ensure persistence on host filesystem
	trimmed := strings.TrimPrefix(cleaned, "/")
	if strings.HasPrefix(trimmed, "opt/hephaestus/") {
		trimmed = strings.TrimPrefix(trimmed, "opt/hephaestus/")
	}
	return filepath.Join("/app/backups", trimmed)
}

func (s *BackupService) uploadFileToDestination(ctx context.Context, localFilePath, filename string, dest *domain.BackupDestination) error {
	defer func() {
		// Clean up temporary staging file if it hasn't been moved
		_ = os.Remove(localFilePath)
	}()

	switch dest.DestType {
	case "local":
		rawPath, _ := dest.Config["path"].(string)
		targetDir := ResolveLocalBackupPath(rawPath)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return fmt.Errorf("failed to create backup directory '%s': %w", targetDir, err)
		}
		targetFile := filepath.Join(targetDir, filename)
		if localFilePath == targetFile {
			return nil
		}

		// Fast-path: atomic rename on the same filesystem
		if err := os.Rename(localFilePath, targetFile); err == nil {
			return nil
		}

		// Fallback streaming copy
		src, err := os.Open(localFilePath)
		if err != nil {
			return err
		}
		defer src.Close()

		dst, err := os.OpenFile(targetFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}
		defer dst.Close()

		_, err = io.Copy(dst, src)
		return err

	case "nas", "nfs", "nas_ssh":
		host, _ := dest.Config["host"].(string)
		if host == "" {
			rawPath, _ := dest.Config["path"].(string)
			targetDir := ResolveLocalBackupPath(rawPath)
			if err := os.MkdirAll(targetDir, 0755); err != nil {
				return fmt.Errorf("failed to create backup directory '%s': %w", targetDir, err)
			}
			targetFile := filepath.Join(targetDir, filename)
			if localFilePath == targetFile {
				return nil
			}
			if err := os.Rename(localFilePath, targetFile); err == nil {
				return nil
			}
			src, err := os.Open(localFilePath)
			if err != nil {
				return err
			}
			defer src.Close()
			dst, err := os.OpenFile(targetFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
			if err != nil {
				return err
			}
			defer dst.Close()
			_, err = io.Copy(dst, src)
			return err
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
		src, err := os.Open(localFilePath)
		if err != nil {
			return err
		}
		defer src.Close()

		return s.sshService.SftpUploadWithConfig(remoteHostCfg, remoteFile, src)

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

		src, err := os.Open(localFilePath)
		if err != nil {
			return err
		}
		defer src.Close()

		fi, err := src.Stat()
		if err != nil {
			return err
		}

		_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:        aws.String(bucket),
			Key:           aws.String(filename),
			Body:          src,
			ContentLength: aws.Int64(fi.Size()),
		})
		return err

	default:
		return fmt.Errorf("unsupported backup destination type: %s", dest.DestType)
	}
}

func (s *BackupService) uploadToDestination(ctx context.Context, data []byte, filename string, dest *domain.BackupDestination) error {
	switch dest.DestType {
	case "local":
		rawPath, _ := dest.Config["path"].(string)
		targetDir := ResolveLocalBackupPath(rawPath)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return fmt.Errorf("failed to create backup directory '%s': %w", targetDir, err)
		}
		targetFile := filepath.Join(targetDir, filename)
		return os.WriteFile(targetFile, data, 0644)

	case "nas", "nfs", "nas_ssh":
		host, _ := dest.Config["host"].(string)
		if host == "" {
			// If no remote host specified, treat as local filesystem/mount path
			rawPath, _ := dest.Config["path"].(string)
			targetDir := ResolveLocalBackupPath(rawPath)
			if err := os.MkdirAll(targetDir, 0755); err != nil {
				return fmt.Errorf("failed to create backup directory '%s': %w", targetDir, err)
			}
			targetFile := filepath.Join(targetDir, filename)
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
	if err := s.uploadToDestination(ctx, testData, testFilename, dest); err != nil {
		return err
	}
	// Clean up temporary test file on local filesystem
	if dest.DestType == "local" || (dest.DestType == "nas" && dest.Config != nil && dest.Config["host"] == "") {
		rawPath, _ := dest.Config["path"].(string)
		targetDir := ResolveLocalBackupPath(rawPath)
		_ = os.Remove(filepath.Join(targetDir, testFilename))
	}
	return nil
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
		var lastErr error
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
			lastErr = connErr
		}

		dumpBin := "pg_dump"
		if p, err := exec.LookPath("pg_dump"); err == nil {
			dumpBin = p
		} else if lastErr != nil {
			return "", fmt.Errorf("PostgreSQL connection failed: %w", lastErr)
		}

		cmd := exec.CommandContext(testCtx, dumpBin, "--schema-only", "-h", dbCfg.Host, "-p", fmt.Sprintf("%d", dbCfg.Port), "-U", dbCfg.Username, "-d", dbCfg.DatabaseName)
		cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", dbCfg.Password))
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if cliErr := cmd.Run(); cliErr != nil {
			errStr := strings.TrimSpace(stderr.String())
			if errStr != "" {
				return "", fmt.Errorf("%s", errStr)
			}
			if lastErr != nil {
				return "", fmt.Errorf("PostgreSQL connection failed: %v (pgx: %v)", cliErr, lastErr)
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

// -------------------------------------------------------------
// Docker Container Backup & Restore
// -------------------------------------------------------------

func GetContainerBackupDir() string {
	baseDir := ResolveLocalBackupPath("")
	target := filepath.Join(baseDir, "container-docker")
	if err := os.MkdirAll(target, 0755); err == nil {
		return target
	}
	fallback := filepath.Join("backup", "container-docker")
	_ = os.MkdirAll(fallback, 0755)
	return fallback
}

func getPossibleContainerDirs() []string {
	return []string{
		GetContainerBackupDir(),
		"/app/backups/container-docker",
		"backup/container-docker",
		"backups/container-docker",
		"/opt/backups/container-docker",
	}
}

func sanitizeContainerName(name string) string {
	name = strings.TrimPrefix(name, "/")
	reg := regexp.MustCompile(`[^a-zA-Z0-9_\-\.]`)
	clean := reg.ReplaceAllString(name, "_")
	if clean == "" {
		return "container"
	}
	return clean
}

func ReadContainerBackupMetadata(archivePath string) (*domain.ContainerBackupMetadata, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	gzReader, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("failed to open gzip archive: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if header.Name == "metadata.json" || strings.HasSuffix(header.Name, "/metadata.json") {
			var meta domain.ContainerBackupMetadata
			if err := json.NewDecoder(tarReader).Decode(&meta); err != nil {
				return nil, fmt.Errorf("failed to decode metadata.json: %w", err)
			}
			if fi, err := f.Stat(); err == nil {
				meta.FileSize = fi.Size()
			}
			meta.Filename = filepath.Base(archivePath)
			meta.FilePath = archivePath
			return &meta, nil
		}
	}
	return nil, errors.New("metadata.json not found in archive")
}

func (s *BackupService) FindContainerBackupFile(filename string) (string, error) {
	filename = filepath.Base(filename)
	for _, dir := range getPossibleContainerDirs() {
		p := filepath.Join(dir, filename)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("container backup file '%s' not found in backup/container-docker", filename)
}

func (s *BackupService) ListContainerBackups(ctx context.Context) ([]domain.ContainerBackupMetadata, error) {
	seen := make(map[string]bool)
	var list []domain.ContainerBackupMetadata

	for _, dir := range getPossibleContainerDirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasSuffix(name, ".tar.gz") && !strings.HasSuffix(name, ".tar") {
				continue
			}
			if seen[name] {
				continue
			}
			seen[name] = true

			fullPath := filepath.Join(dir, name)
			meta, err := ReadContainerBackupMetadata(fullPath)
			if err != nil {
				fi, _ := e.Info()
				var size int64
				var mtime time.Time
				if fi != nil {
					size = fi.Size()
					mtime = fi.ModTime()
				}
				parts := strings.Split(strings.TrimSuffix(strings.TrimSuffix(name, ".gz"), ".tar"), "_")
				cName := name
				if len(parts) >= 2 {
					cName = parts[1]
				}
				meta = &domain.ContainerBackupMetadata{
					ID:            "cbk-" + name,
					ContainerName: cName,
					Filename:      name,
					FilePath:      fullPath,
					FileSize:      size,
					CreatedAt:     mtime,
					Status:        "success",
				}
			}
			list = append(list, *meta)
		}
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})

	return list, nil
}

func (s *BackupService) InspectContainerBackup(ctx context.Context, filename string) (*domain.ContainerBackupMetadata, error) {
	filePath, err := s.FindContainerBackupFile(filename)
	if err != nil {
		return nil, err
	}
	return ReadContainerBackupMetadata(filePath)
}

// -------------------------------------------------------------
// Active Background Task Registry
// -------------------------------------------------------------

func (s *BackupService) RegisterTask(id, taskType, target string) *ActiveBackupTask {
	s.taskMu.Lock()
	defer s.taskMu.Unlock()

	task := &ActiveBackupTask{
		ID:        id,
		Type:      taskType,
		Target:    target,
		StartedAt: time.Now(),
		Status:    "running",
		Logs:      make([]ActiveBackupTaskLog, 0),
		listeners: make(map[chan ActiveBackupTaskLog]bool),
		done:      make(chan struct{}),
	}
	s.activeTasks[id] = task
	return task
}

func (s *BackupService) GetActiveTask(id string) *ActiveBackupTask {
	s.taskMu.RLock()
	defer s.taskMu.RUnlock()
	return s.activeTasks[id]
}

func (s *BackupService) AppendTaskLog(id, step, status, msg string) {
	s.taskMu.RLock()
	task, exists := s.activeTasks[id]
	s.taskMu.RUnlock()

	if !exists || task == nil {
		return
	}

	task.mu.Lock()
	defer task.mu.Unlock()

	entry := ActiveBackupTaskLog{
		Step:      step,
		Status:    status,
		Message:   msg,
		Timestamp: time.Now().Format("15:04:05"),
	}
	task.Logs = append(task.Logs, entry)

	for ch := range task.listeners {
		select {
		case ch <- entry:
		default:
		}
	}
}

func (s *BackupService) FinishTask(id, status, errMsg string) {
	s.taskMu.Lock()
	task, exists := s.activeTasks[id]
	s.taskMu.Unlock()

	if !exists || task == nil {
		return
	}

	task.mu.Lock()
	task.Status = status
	task.Error = errMsg
	select {
	case <-task.done:
	default:
		close(task.done)
	}
	for ch := range task.listeners {
		close(ch)
		delete(task.listeners, ch)
	}
	task.mu.Unlock()

	time.AfterFunc(20*time.Minute, func() {
		s.taskMu.Lock()
		delete(s.activeTasks, id)
		s.taskMu.Unlock()
	})
}

func (s *BackupService) SubscribeTask(id string) (chan ActiveBackupTaskLog, func(), []ActiveBackupTaskLog, bool) {
	s.taskMu.RLock()
	task, exists := s.activeTasks[id]
	s.taskMu.RUnlock()

	if !exists || task == nil {
		return nil, nil, nil, false
	}

	task.mu.Lock()
	defer task.mu.Unlock()

	ch := make(chan ActiveBackupTaskLog, 200)
	task.listeners[ch] = true

	snapshot := make([]ActiveBackupTaskLog, len(task.Logs))
	copy(snapshot, task.Logs)

	unsubscribe := func() {
		task.mu.Lock()
		defer task.mu.Unlock()
		delete(task.listeners, ch)
	}

	return ch, unsubscribe, snapshot, true
}

// ReconcileHistory checks for any stuck "running" records in backup_history
// and reconciles them against actual archives on disk or timeouts.
func (s *BackupService) ReconcileHistory(ctx context.Context) {
	pool, err := database.GetPool()
	if err != nil {
		return
	}

	rows, err := pool.Query(ctx, `SELECT id, db_name, db_type, dest_type, filename, started_at FROM backup_history WHERE status = 'running'`)
	if err != nil {
		return
	}
	defer rows.Close()

	type stuckRecord struct {
		id        string
		dbName    string
		dbType    string
		destType  string
		filename  string
		startedAt time.Time
	}
	var stuckList []stuckRecord
	for rows.Next() {
		var sr stuckRecord
		if err := rows.Scan(&sr.id, &sr.dbName, &sr.dbType, &sr.destType, &sr.filename, &sr.startedAt); err == nil {
			stuckList = append(stuckList, sr)
		}
	}

	now := time.Now()
	for _, rec := range stuckList {
		// If actively being processed by background worker/task, do not modify
		s.taskMu.RLock()
		_, isRunning := s.activeTasks[rec.id]
		s.taskMu.RUnlock()
		if isRunning {
			continue
		}

		// Container Backups
		if rec.destType == "container-docker" || (rec.dbType == "docker" && !strings.Contains(rec.destType, "RESTORE")) {
			if filePath, err := s.FindContainerBackupFile(rec.filename); err == nil {
				if fi, err := os.Stat(filePath); err == nil && fi.Size() > 0 {
					_ = s.backupRepo.UpdateHistoryStatus(ctx, rec.id, "success", fi.Size(), nil)
					continue
				}
			}
			if now.Sub(rec.startedAt) > 5*time.Minute {
				errMsg := "Container backup process was interrupted or ended unexpectedly"
				_ = s.backupRepo.UpdateHistoryStatus(ctx, rec.id, "failed", 0, &errMsg)
			}
		} else if strings.Contains(rec.destType, "RESTORE") {
			if now.Sub(rec.startedAt) > 15*time.Minute {
				errMsg := "Container restore process timed out or server restarted"
				_ = s.backupRepo.UpdateHistoryStatus(ctx, rec.id, "failed", 0, &errMsg)
			}
		} else {
			if now.Sub(rec.startedAt) > 30*time.Minute {
				errMsg := "Database backup job timed out or worker crashed"
				_ = s.backupRepo.UpdateHistoryStatus(ctx, rec.id, "failed", 0, &errMsg)
			}
		}
	}
}

// -------------------------------------------------------------
// Container Backup (Async & Streaming)
// -------------------------------------------------------------

func (s *BackupService) StartContainerBackupAsync(req domain.BackupContainerRunRequest) (string, error) {
	if s.dockerService == nil {
		return "", errors.New("docker service is not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	conn, err := s.dockerService.resolveConnection(ctx, req.ConnectionID)
	if err != nil {
		return "", fmt.Errorf("failed to resolve Docker connection: %w", err)
	}
	inspect, err := s.dockerService.InspectContainer(ctx, req.ConnectionID, req.ContainerID)
	if err != nil {
		return "", fmt.Errorf("failed to inspect container: %w", err)
	}

	cleanName := strings.TrimPrefix(inspect.Name, "/")
	if req.CustomName != "" {
		cleanName = strings.TrimSpace(req.CustomName)
	} else if req.BackupName != "" {
		cleanName = strings.TrimSpace(req.BackupName)
	}
	cleanName = sanitizeContainerName(cleanName)

	now := time.Now()
	timeStr := now.Format("20060102_150405")
	filename := fmt.Sprintf("container_%s_%s.tar.gz", cleanName, timeStr)
	historyID := fmt.Sprintf("cbh-%s", uuid.New().String()[:8])

	_ = s.backupRepo.CreateHistory(context.Background(), domain.BackupHistoryEntry{
		ID:        historyID,
		DBName:    cleanName,
		DBType:    "docker",
		DestType:  "container-docker",
		Filename:  filename,
		FileSize:  0,
		Status:    "running",
		StartedAt: now,
	})

	s.RegisterTask(historyID, "backup", fmt.Sprintf("%s on %s", cleanName, conn.Name))
	s.AppendTaskLog(historyID, "init", "info", fmt.Sprintf("Initiating backup snapshot for container '%s' on Docker host '%s'...", cleanName, conn.Name))

	go func() {
		bgCtx, bgCancel := context.WithTimeout(context.Background(), 60*time.Minute)
		defer bgCancel()

		var logBuffer strings.Builder
		logFn := func(step, status, message string) {
			s.AppendTaskLog(historyID, step, status, message)
			ts := time.Now().Format("15:04:05")
			logBuffer.WriteString(fmt.Sprintf("[%s] [%s] %s\n", ts, strings.ToUpper(step), message))
		}

		defer func() {
			if r := recover(); r != nil {
				errMsg := fmt.Sprintf("Process panicked: %v", r)
				logFn("error", "error", errMsg)
				s.FinishTask(historyID, "failed", errMsg)
				fullLogs := logBuffer.String()
				_ = s.backupRepo.UpdateHistoryStatus(context.Background(), historyID, "failed", 0, &fullLogs)
			}
		}()

		meta, err := s.executeContainerBackupCore(bgCtx, historyID, conn, inspect, cleanName, filename, timeStr, req, logFn)
		fullLogs := logBuffer.String()

		if err != nil {
			s.FinishTask(historyID, "failed", err.Error())
			_ = s.backupRepo.UpdateHistoryStatus(context.Background(), historyID, "failed", 0, &fullLogs)
		} else {
			s.FinishTask(historyID, "success", "")
			var finalSize int64
			if meta != nil {
				finalSize = meta.FileSize
			}
			_ = s.backupRepo.UpdateHistoryStatus(context.Background(), historyID, "success", finalSize, &fullLogs)
		}
	}()

	return historyID, nil
}

func (s *BackupService) executeContainerBackupCore(
	ctx context.Context,
	historyID string,
	conn *domain.DockerConnection,
	inspect *domain.ContainerInspectDetails,
	cleanName, filename, timeStr string,
	req domain.BackupContainerRunRequest,
	logFn func(step, status, message string),
) (*domain.ContainerBackupMetadata, error) {
	safeLog := func(step, status, msg string) {
		if logFn != nil {
			logFn(step, status, msg)
		}
	}

	backupTag := fmt.Sprintf("hcp-backup-%s:%s", cleanName, timeStr)
	targetDir := GetContainerBackupDir()
	_ = os.MkdirAll(targetDir, 0755)
	targetFilePath := filepath.Join(targetDir, filename)

	tempDir := ResolveLocalBackupPath("")
	_ = os.MkdirAll(tempDir, 0755)
	tempImageFile := filepath.Join(tempDir, fmt.Sprintf(".tmp_img_%s.tar", uuid.New().String()[:8]))
	defer os.Remove(tempImageFile)

	wasRunning := inspect.IsRunning
	if req.StopDuringBackup && wasRunning {
		safeLog("quiesce", "progress", fmt.Sprintf("Stopping container '%s' temporarily for consistent snapshot...", cleanName))
		_ = s.dockerService.StopContainer(ctx, req.ConnectionID, req.ContainerID)
		defer func() {
			safeLog("quiesce", "info", fmt.Sprintf("Restarting container '%s' after backup...", cleanName))
			_ = s.dockerService.StartContainer(context.Background(), req.ConnectionID, req.ContainerID)
		}()
	}

	// 1. Commit container & export image tar
	if conn.HostType == "ssh" || conn.RemoteHostID != nil {
		safeLog("commit", "progress", fmt.Sprintf("Executing docker commit on remote host '%s' -> '%s'...", conn.Name, backupTag))
		commitCmd := fmt.Sprintf("commit %s %s", shellQuote(req.ContainerID), shellQuote(backupTag))
		if _, _, err := s.dockerService.runRemoteDockerCommand(ctx, conn, commitCmd); err != nil {
			errMsg := fmt.Sprintf("docker commit failed: %v", err)
			safeLog("error", "error", errMsg)
			return nil, fmt.Errorf("remote commit failed: %w", err)
		}

		defer func() {
			safeLog("cleanup", "info", fmt.Sprintf("Removing temporary snapshot image '%s' on remote host...", backupTag))
			_, _, _ = s.dockerService.runRemoteDockerCommand(context.Background(), conn, fmt.Sprintf("rmi -f %s", shellQuote(backupTag)))
		}()

		imgDst, err := os.Create(tempImageFile)
		if err != nil {
			safeLog("error", "error", fmt.Sprintf("Failed to create temporary image file: %v", err))
			return nil, err
		}

		safeLog("stream", "progress", fmt.Sprintf("Streaming docker image layers from remote '%s' to HCP storage...", conn.Name))
		errSave := s.dockerService.StreamRemoteDockerSave(ctx, conn, backupTag, imgDst, safeLog)
		imgDst.Close()
		if errSave != nil {
			safeLog("error", "error", fmt.Sprintf("remote docker save streaming failed: %v", errSave))
			return nil, errSave
		}
	} else {
		// Local Docker
		safeLog("commit", "progress", fmt.Sprintf("Committing local container '%s' -> '%s'...", cleanName, backupTag))
		commitEndpoint := fmt.Sprintf("/commit?container=%s&repo=%s&tag=%s",
			url.QueryEscape(req.ContainerID),
			url.QueryEscape("hcp-backup-"+cleanName),
			url.QueryEscape(timeStr),
		)
		if err := s.dockerService.doLocalPost(ctx, conn, commitEndpoint, nil); err != nil {
			errMsg := fmt.Sprintf("docker commit failed: %v", err)
			safeLog("error", "error", errMsg)
			return nil, fmt.Errorf("local commit failed: %w", err)
		}

		defer func() {
			safeLog("cleanup", "info", fmt.Sprintf("Removing temporary snapshot image '%s' on local daemon...", backupTag))
			delClient, delBaseURL := s.dockerService.getLocalHTTPClient(conn)
			delReq, _ := http.NewRequestWithContext(context.Background(), "DELETE", fmt.Sprintf("%s/images/%s?force=true", delBaseURL, url.PathEscape(backupTag)), nil)
			if delReq != nil {
				_, _ = delClient.Do(delReq)
			}
		}()

		safeLog("stream", "progress", "Streaming docker image archive from local Docker daemon...")
		saveEndpoint := fmt.Sprintf("/images/%s/get", url.PathEscape(backupTag))
		client, baseURL := s.dockerService.getLocalStreamingHTTPClient(conn)
		saveReq, err := http.NewRequestWithContext(ctx, "GET", baseURL+saveEndpoint, nil)
		if err != nil {
			safeLog("error", "error", err.Error())
			return nil, err
		}
		saveResp, err := client.Do(saveReq)
		if err != nil {
			safeLog("error", "error", fmt.Sprintf("docker save failed: %v", err))
			return nil, fmt.Errorf("local save failed: %w", err)
		}
		defer saveResp.Body.Close()

		imgDst, err := os.Create(tempImageFile)
		if err != nil {
			safeLog("error", "error", err.Error())
			return nil, err
		}
		_, errCopy := io.Copy(imgDst, saveResp.Body)
		imgDst.Close()
		if errCopy != nil {
			safeLog("error", "error", fmt.Sprintf("failed to stream image: %v", errCopy))
			return nil, errCopy
		}
	}

	// 2. Build metadata
	now := time.Now()
	metadata := domain.ContainerBackupMetadata{
		ID:                   fmt.Sprintf("cbk-%s", uuid.New().String()[:8]),
		ContainerID:          req.ContainerID,
		ContainerName:        cleanName,
		Image:                inspect.Image,
		BackupTag:            backupTag,
		SourceConnectionID:   req.ConnectionID,
		SourceConnectionName: conn.Name,
		Filename:             filename,
		FilePath:             targetFilePath,
		Status:               "success",
		CreatedAt:            now,
		Ports:                inspect.Ports,
		Volumes:              inspect.Volumes,
		Env:                  inspect.Env,
		RestartPolicy:        inspect.RestartPolicy,
		NetworkMode:          inspect.NetworkMode,
		Command:              inspect.Command,
		CPULimit:             inspect.CPULimit,
		MemoryLimitMB:        inspect.MemoryLimitMB,
	}

	// 3. Package metadata.json + image.tar into targetFilePath (.tar.gz)
	safeLog("package", "progress", fmt.Sprintf("Compressing image archive and metadata into %s...", filename))
	outGzFile, err := os.Create(targetFilePath)
	if err != nil {
		safeLog("error", "error", err.Error())
		return nil, fmt.Errorf("failed to create target file: %w", err)
	}
	defer outGzFile.Close()

	gzWriter := gzip.NewWriter(outGzFile)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// Add metadata.json
	metaBytes, _ := json.MarshalIndent(metadata, "", "  ")
	if err := tarWriter.WriteHeader(&tar.Header{
		Name:    "metadata.json",
		Mode:    0644,
		Size:    int64(len(metaBytes)),
		ModTime: now,
	}); err != nil {
		safeLog("error", "error", err.Error())
		return nil, err
	}
	if _, err := tarWriter.Write(metaBytes); err != nil {
		safeLog("error", "error", err.Error())
		return nil, err
	}

	// Add image.tar
	imgStat, err := os.Stat(tempImageFile)
	if err != nil {
		safeLog("error", "error", err.Error())
		return nil, err
	}
	if err := tarWriter.WriteHeader(&tar.Header{
		Name:    "image.tar",
		Mode:    0644,
		Size:    imgStat.Size(),
		ModTime: now,
	}); err != nil {
		safeLog("error", "error", err.Error())
		return nil, err
	}
	imgSrc, err := os.Open(tempImageFile)
	if err != nil {
		safeLog("error", "error", err.Error())
		return nil, err
	}
	defer imgSrc.Close()

	if _, err := io.Copy(tarWriter, imgSrc); err != nil {
		safeLog("error", "error", err.Error())
		return nil, err
	}

	_ = tarWriter.Close()
	_ = gzWriter.Close()
	_ = outGzFile.Close()

	finalStat, err := os.Stat(targetFilePath)
	var finalSize int64
	if err == nil {
		finalSize = finalStat.Size()
		metadata.FileSize = finalSize
	}

	safeLog("complete", "success", fmt.Sprintf("Container snapshot '%s' created successfully! Archive: %s (%.2f MB)", cleanName, filename, float64(finalSize)/(1024*1024)))
	return &metadata, nil
}

func (s *BackupService) BackupContainer(ctx context.Context, req domain.BackupContainerRunRequest) (*domain.ContainerBackupMetadata, error) {
	taskID, err := s.StartContainerBackupAsync(req)
	if err != nil {
		return nil, err
	}
	task := s.GetActiveTask(taskID)
	if task != nil {
		<-task.done
		if task.Status == "failed" {
			return nil, errors.New(task.Error)
		}
	}
	return s.InspectContainerBackup(ctx, fmt.Sprintf("container_%s", req.ContainerID))
}

func (s *BackupService) BackupContainerStream(ctx context.Context, req domain.BackupContainerRunRequest, logFn func(step, status, message string)) (*domain.ContainerBackupMetadata, error) {
	taskID, err := s.StartContainerBackupAsync(req)
	if err != nil {
		return nil, err
	}

	ch, unsub, snapshot, ok := s.SubscribeTask(taskID)
	if !ok {
		return nil, errors.New("failed to attach to container backup task")
	}
	defer unsub()

	if logFn != nil {
		for _, l := range snapshot {
			logFn(l.Step, l.Status, l.Message)
		}
	}

	for entry := range ch {
		if logFn != nil {
			logFn(entry.Step, entry.Status, entry.Message)
		}
	}

	task := s.GetActiveTask(taskID)
	if task != nil && task.Status == "failed" {
		return nil, errors.New(task.Error)
	}
	return s.InspectContainerBackup(ctx, req.ContainerID)
}

// -------------------------------------------------------------
// Container Restore (Async & Streaming)
// -------------------------------------------------------------

func (s *BackupService) StartContainerRestoreAsync(req domain.RestoreContainerRequest) (string, error) {
	if s.dockerService == nil {
		return "", errors.New("docker service is not initialized")
	}

	filePath, err := s.FindContainerBackupFile(req.Filename)
	if err != nil {
		return "", err
	}
	var archiveSize int64
	if fi, err := os.Stat(filePath); err == nil {
		archiveSize = fi.Size()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	targetConn, err := s.dockerService.resolveConnection(ctx, req.TargetConnectionID)
	if err != nil {
		return "", fmt.Errorf("failed to resolve target Docker connection: %w", err)
	}

	containerName := strings.TrimSpace(req.NewContainerName)
	if containerName == "" {
		containerName = strings.TrimSpace(req.ContainerName)
	}
	containerName = sanitizeContainerName(containerName)
	historyID := fmt.Sprintf("crh-%s", uuid.New().String()[:8])
	now := time.Now()

	destLabel := fmt.Sprintf("RESTORE (%s)", targetConn.Name)
	_ = s.backupRepo.CreateHistory(context.Background(), domain.BackupHistoryEntry{
		ID:        historyID,
		DBName:    containerName,
		DBType:    "docker",
		DestType:  destLabel,
		Filename:  req.Filename,
		FileSize:  archiveSize,
		Status:    "running",
		StartedAt: now,
	})

	s.RegisterTask(historyID, "restore", fmt.Sprintf("%s -> %s", containerName, targetConn.Name))
	s.AppendTaskLog(historyID, "init", "info", fmt.Sprintf("Initiating restore pipeline for '%s' into host '%s'...", containerName, targetConn.Name))

	go func() {
		bgCtx, bgCancel := context.WithTimeout(context.Background(), 60*time.Minute)
		defer bgCancel()

		var logBuffer strings.Builder
		logFn := func(step, status, message string) {
			s.AppendTaskLog(historyID, step, status, message)
			ts := time.Now().Format("15:04:05")
			logBuffer.WriteString(fmt.Sprintf("[%s] [%s] %s\n", ts, strings.ToUpper(step), message))
		}

		defer func() {
			if r := recover(); r != nil {
				errMsg := fmt.Sprintf("Restore process panicked: %v", r)
				logFn("error", "error", errMsg)
				s.FinishTask(historyID, "failed", errMsg)
				fullLogs := logBuffer.String()
				_ = s.backupRepo.UpdateHistoryStatus(context.Background(), historyID, "failed", archiveSize, &fullLogs)
			}
		}()

		_, err := s.executeContainerRestoreCore(bgCtx, filePath, targetConn, containerName, req, logFn)
		fullLogs := logBuffer.String()

		if err != nil {
			s.FinishTask(historyID, "failed", err.Error())
			_ = s.backupRepo.UpdateHistoryStatus(context.Background(), historyID, "failed", archiveSize, &fullLogs)
		} else {
			s.FinishTask(historyID, "success", "")
			_ = s.backupRepo.UpdateHistoryStatus(context.Background(), historyID, "success", archiveSize, &fullLogs)
		}
	}()

	return historyID, nil
}

func (s *BackupService) executeContainerRestoreCore(
	ctx context.Context,
	filePath string,
	targetConn *domain.DockerConnection,
	containerName string,
	req domain.RestoreContainerRequest,
	logFn func(step, status, message string),
) (*domain.ContainerInspectDetails, error) {
	safeLog := func(step, status, msg string) {
		if logFn != nil {
			logFn(step, status, msg)
		}
	}

	safeLog("archive", "info", fmt.Sprintf("Locating backup archive '%s'...", req.Filename))
	archiveFile, err := os.Open(filePath)
	if err != nil {
		safeLog("error", "error", fmt.Sprintf("Failed to open backup archive: %v", err))
		return nil, fmt.Errorf("failed to open backup archive: %w", err)
	}
	defer archiveFile.Close()

	gzReader, err := gzip.NewReader(archiveFile)
	if err != nil {
		safeLog("error", "error", fmt.Sprintf("Failed to initialize gzip reader: %v", err))
		return nil, fmt.Errorf("failed to initialize gzip reader: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	var metadata *domain.ContainerBackupMetadata
	var tempImagePath string

	tempDir := ResolveLocalBackupPath("")
	_ = os.MkdirAll(tempDir, 0755)

	safeLog("extract", "progress", fmt.Sprintf("Reading and unpacking backup archive '%s'...", filepath.Base(filePath)))
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			safeLog("error", "error", fmt.Sprintf("Error reading archive entry: %v", err))
			return nil, fmt.Errorf("error reading archive entry: %w", err)
		}

		if header.Name == "metadata.json" || strings.HasSuffix(header.Name, "/metadata.json") {
			var meta domain.ContainerBackupMetadata
			if err := json.NewDecoder(tarReader).Decode(&meta); err == nil {
				metadata = &meta
				safeLog("extract", "info", fmt.Sprintf("Read metadata: Source container '%s' with image tag '%s'", meta.ContainerName, meta.BackupTag))
			}
		} else if header.Name == "image.tar" || strings.HasSuffix(header.Name, "/image.tar") {
			tempImagePath = filepath.Join(tempDir, fmt.Sprintf(".restore_img_%s.tar", uuid.New().String()[:8]))
			outFile, err := os.Create(tempImagePath)
			if err != nil {
				safeLog("error", "error", fmt.Sprintf("Failed to create temporary image file: %v", err))
				return nil, fmt.Errorf("failed to create temporary image file: %w", err)
			}
			safeLog("extract", "progress", fmt.Sprintf("Extracting raw container image tarball (%.2f MB)...", float64(header.Size)/(1024*1024)))
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				_ = os.Remove(tempImagePath)
				safeLog("error", "error", fmt.Sprintf("Failed to extract image.tar: %v", err))
				return nil, fmt.Errorf("failed to extract image.tar: %w", err)
			}
			outFile.Close()
		}
	}

	if tempImagePath != "" {
		defer os.Remove(tempImagePath)
	}

	if metadata == nil {
		safeLog("error", "error", "invalid backup archive: metadata.json missing")
		return nil, errors.New("invalid backup archive: metadata.json missing")
	}
	if tempImagePath == "" {
		safeLog("error", "error", "invalid backup archive: image.tar missing")
		return nil, errors.New("invalid backup archive: image.tar missing")
	}

	if containerName == "" {
		containerName = metadata.ContainerName
	}
	containerName = strings.TrimPrefix(containerName, "/")

	// Check if container already exists on target connection
	safeLog("check", "info", fmt.Sprintf("Checking for existing container '%s' on target Docker host '%s'...", containerName, targetConn.Name))
	existingDetails, errInspect := s.dockerService.InspectContainer(ctx, req.TargetConnectionID, containerName)
	if errInspect == nil && existingDetails != nil {
		if !req.ReplaceExisting {
			errMsg := fmt.Sprintf("container '%s' already exists on target Docker host '%s'. Please choose a different name or enable 'Replace existing container'", containerName, targetConn.Name)
			safeLog("error", "error", errMsg)
			return nil, errors.New(errMsg)
		}
		safeLog("check", "progress", fmt.Sprintf("Replacing existing container '%s' on destination...", containerName))
		_ = s.dockerService.RemoveContainer(ctx, req.TargetConnectionID, containerName, true)
	}

	// Load image into target connection
	if targetConn.HostType == "ssh" || targetConn.RemoteHostID != nil {
		imgF, err := os.Open(tempImagePath)
		if err != nil {
			safeLog("error", "error", err.Error())
			return nil, err
		}
		defer imgF.Close()

		safeLog("load", "progress", fmt.Sprintf("Loading image into remote Docker host '%s' via SSH...", targetConn.Name))
		if err := s.dockerService.StreamRemoteDockerLoad(ctx, targetConn, imgF, safeLog); err != nil {
			safeLog("error", "error", fmt.Sprintf("docker load failed on remote host: %v", err))
			return nil, fmt.Errorf("docker load failed on remote host: %w", err)
		}
	} else {
		imgF, err := os.Open(tempImagePath)
		if err != nil {
			safeLog("error", "error", err.Error())
			return nil, err
		}
		defer imgF.Close()

		safeLog("load", "progress", "Piping image into local Docker daemon via /images/load...")
		client, baseURL := s.dockerService.getLocalStreamingHTTPClient(targetConn)
		loadReq, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/images/load", imgF)
		if err != nil {
			safeLog("error", "error", err.Error())
			return nil, err
		}
		loadReq.Header.Set("Content-Type", "application/x-tar")
		resp, err := client.Do(loadReq)
		if err != nil {
			safeLog("error", "error", fmt.Sprintf("docker load failed on local host: %v", err))
			return nil, fmt.Errorf("docker load failed on local host: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			b, _ := io.ReadAll(resp.Body)
			errMsg := fmt.Sprintf("docker load returned status %d: %s", resp.StatusCode, string(b))
			safeLog("error", "error", errMsg)
			return nil, errors.New(errMsg)
		}
		safeLog("load", "success", "Image loaded successfully into local Docker daemon.")
	}

	// Prepare deploy request
	var portBindings []string
	if len(req.PortBindings) > 0 {
		for contPortStr, hostPortStr := range req.PortBindings {
			portBindings = append(portBindings, fmt.Sprintf("%s:%s", hostPortStr, contPortStr))
		}
	} else if len(req.Ports) > 0 {
		for _, p := range req.Ports {
			if p.HostPort != "" && p.ContainerPort != "" {
				proto := p.Protocol
				if proto == "" || proto == "tcp" {
					portBindings = append(portBindings, fmt.Sprintf("%s:%s", p.HostPort, p.ContainerPort))
				} else {
					portBindings = append(portBindings, fmt.Sprintf("%s:%s/%s", p.HostPort, p.ContainerPort, proto))
				}
			}
		}
	} else if len(metadata.Ports) > 0 {
		for _, p := range metadata.Ports {
			if p.HostPort != "" && p.ContainerPort != "" {
				proto := p.Protocol
				if proto == "" || proto == "tcp" {
					portBindings = append(portBindings, fmt.Sprintf("%s:%s", p.HostPort, p.ContainerPort))
				} else {
					portBindings = append(portBindings, fmt.Sprintf("%s:%s/%s", p.HostPort, p.ContainerPort, proto))
				}
			}
		}
	}

	var envVars []string
	if len(req.EnvStrings) > 0 {
		envVars = req.EnvStrings
	} else if len(req.Env) > 0 {
		for _, e := range req.Env {
			if e.Key != "" {
				envVars = append(envVars, fmt.Sprintf("%s=%s", e.Key, e.Value))
			}
		}
	} else if len(metadata.Env) > 0 {
		for _, e := range metadata.Env {
			if e.Key != "" {
				envVars = append(envVars, fmt.Sprintf("%s=%s", e.Key, e.Value))
			}
		}
	}

	var volumeBindings []string
	if len(req.Volumes) > 0 {
		for _, v := range req.Volumes {
			if v.HostPath != "" && v.ContainerPath != "" {
				volumeBindings = append(volumeBindings, fmt.Sprintf("%s:%s", v.HostPath, v.ContainerPath))
			}
		}
	} else if len(metadata.Volumes) > 0 {
		for _, v := range metadata.Volumes {
			if v.HostPath != "" && v.ContainerPath != "" {
				volumeBindings = append(volumeBindings, fmt.Sprintf("%s:%s", v.HostPath, v.ContainerPath))
			}
		}
	}

	restartPolicy := metadata.RestartPolicy
	if req.RestartPolicy != "" {
		restartPolicy = req.RestartPolicy
	}
	startAfter := true
	if req.StartAfter != nil {
		startAfter = *req.StartAfter
	} else if req.StartAfterRestore != nil {
		startAfter = *req.StartAfterRestore
	}

	imageToUse := metadata.BackupTag
	if imageToUse == "" {
		imageToUse = metadata.Image
	}

	networkMode := metadata.NetworkMode
	if networkMode != "" && networkMode != "bridge" && networkMode != "host" && networkMode != "none" {
		targetNets, errNets := s.dockerService.ListNetworks(ctx, req.TargetConnectionID)
		if errNets == nil {
			found := false
			for _, n := range targetNets {
				if n.Name == networkMode {
					found = true
					break
				}
			}
			if !found {
				networkMode = "bridge"
			}
		}
	}

	deployReq := domain.DeployContainerRequest{
		Name:           containerName,
		Image:          imageToUse,
		PortBindings:   portBindings,
		VolumeBindings: volumeBindings,
		EnvVars:        envVars,
		RestartPolicy:  restartPolicy,
		NetworkMode:    networkMode,
		Command:        metadata.Command,
		CPULimit:       metadata.CPULimit,
		MemoryLimitMB:  metadata.MemoryLimitMB,
		StartAfter:     &startAfter,
	}

	safeLog("deploy", "progress", fmt.Sprintf("Configuring and deploying container '%s' on '%s'...", containerName, targetConn.Name))
	createdID, err := s.dockerService.DeployContainerStream(ctx, req.TargetConnectionID, deployReq, safeLog)
	if err != nil {
		safeLog("error", "error", fmt.Sprintf("Failed to deploy restored container: %v", err))
		return nil, fmt.Errorf("failed to deploy restored container: %w", err)
	}

	safeLog("complete", "success", fmt.Sprintf("Container '%s' restored and started successfully (ID: %s)!", containerName, createdID[:12]))
	return s.dockerService.InspectContainer(ctx, req.TargetConnectionID, createdID)
}

func (s *BackupService) RestoreContainer(ctx context.Context, req domain.RestoreContainerRequest) (*domain.ContainerInspectDetails, error) {
	taskID, err := s.StartContainerRestoreAsync(req)
	if err != nil {
		return nil, err
	}
	task := s.GetActiveTask(taskID)
	if task != nil {
		<-task.done
		if task.Status == "failed" {
			return nil, errors.New(task.Error)
		}
	}
	return s.dockerService.InspectContainer(ctx, req.TargetConnectionID, req.NewContainerName)
}

func (s *BackupService) RestoreContainerStream(ctx context.Context, req domain.RestoreContainerRequest, logFn func(step, status, message string)) (*domain.ContainerInspectDetails, error) {
	taskID, err := s.StartContainerRestoreAsync(req)
	if err != nil {
		return nil, err
	}

	ch, unsub, snapshot, ok := s.SubscribeTask(taskID)
	if !ok {
		return nil, errors.New("failed to attach to container restore task")
	}
	defer unsub()

	if logFn != nil {
		for _, l := range snapshot {
			logFn(l.Step, l.Status, l.Message)
		}
	}

	for entry := range ch {
		if logFn != nil {
			logFn(entry.Step, entry.Status, entry.Message)
		}
	}

	task := s.GetActiveTask(taskID)
	if task != nil && task.Status == "failed" {
		return nil, errors.New(task.Error)
	}
	return s.dockerService.InspectContainer(ctx, req.TargetConnectionID, req.NewContainerName)
}

func (s *BackupService) DeleteContainerBackup(ctx context.Context, filename string) error {
	filePath, err := s.FindContainerBackupFile(filename)
	if err == nil {
		_ = os.Remove(filePath)
	}
	pool, errPool := database.GetPool()
	if errPool == nil {
		_, _ = pool.Exec(ctx, `DELETE FROM backup_history WHERE filename = $1`, filepath.Base(filename))
	}
	return nil
}
