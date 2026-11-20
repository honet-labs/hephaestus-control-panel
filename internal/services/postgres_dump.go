package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"go-hephaestus/internal/config"
	"go-hephaestus/internal/core/domain"
)

var postgresClientVersionPattern = regexp.MustCompile(`\(PostgreSQL\)\s+(\d+)(?:\.\d+)?`)

// Newer pg_dump clients may emit SQL unsupported by the source server's major
// version (for example transaction_timeout on PostgreSQL 16). Require a matching
// major so a successful dump can be restored to the same server generation.
func postgresDumpBinary(ctx context.Context, db *domain.BackupDbConfig) (string, error) {
	primary := os.Getenv("PG_DUMP_BIN")
	if primary == "" {
		primary = "pg_dump"
	}
	primary, err := exec.LookPath(primary)
	if err != nil {
		return "", fmt.Errorf("pg_dump is required for a restorable PostgreSQL backup: %w", err)
	}
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	dbConfig := config.DBConfig{Host: db.Host, Port: db.Port, User: db.Username, Password: db.Password, Database: db.DatabaseName}
	conn, err := pgx.Connect(probeCtx, dbConfig.ConnString())
	if err != nil {
		return "", fmt.Errorf("verify PostgreSQL backup server version: %w", err)
	}
	var version int
	err = conn.QueryRow(probeCtx, `SELECT current_setting('server_version_num')::int`).Scan(&version)
	_ = conn.Close(probeCtx)
	if err != nil {
		return "", fmt.Errorf("read PostgreSQL backup server version: %w", err)
	}
	major := version / 10000
	candidates := []string{primary}
	if os.Getenv("PG_DUMP_BIN") == "" {
		candidates = append(candidates, fmt.Sprintf("pg_dump%d", major), filepath.FromSlash(fmt.Sprintf("/usr/libexec/postgresql%d/pg_dump", major)), filepath.FromSlash(fmt.Sprintf("/usr/lib/postgresql/%d/bin/pg_dump", major)))
	}
	for _, candidate := range candidates {
		binary, err := exec.LookPath(candidate)
		if err != nil {
			continue
		}
		output, err := exec.CommandContext(probeCtx, binary, "--version").Output()
		if err != nil {
			continue
		}
		match := postgresClientVersionPattern.FindSubmatch(output)
		if len(match) != 2 {
			continue
		}
		clientMajor, _ := strconv.Atoi(string(match[1]))
		if clientMajor == major {
			return binary, nil
		}
	}
	return "", fmt.Errorf("PostgreSQL %d backup requires pg_dump of the same major version; install pg_dump%d or set PG_DUMP_BIN to a compatible client", major, major)
}
