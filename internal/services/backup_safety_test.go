package services

import (
	"context"
	"go-hephaestus/internal/core/domain"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackupPathsCannotEscapeStorageRoot(t *testing.T) {
	for _, input := range []string{"../../etc", "/app/backups/../../../etc", "..\\..\\data", "/opt/backups/../../etc", "database/daily", "/backup/mysql", ""} {
		got := ResolveLocalBackupPath(input)
		if got != "/app/backups" && !strings.HasPrefix(got, "/app/backups/") {
			t.Fatalf("escaped backup root: %q -> %q", input, got)
		}
		if path.Clean(got) != got {
			t.Fatalf("unclean output: %q", got)
		}
	}
	for _, input := range []string{"../../db", "C:\\data\\db", "\r\nsecret", "...", "normal"} {
		name := databaseArchiveFilename(input, "2026-11-20T12-00-00")
		if path.Base(name) != name || strings.ContainsAny(name, "/\\\r\n") {
			t.Fatalf("unsafe archive filename: %q", name)
		}
	}
}

func TestPostgresBackupFailsWithoutDumpClient(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	file := filepath.Join(t.TempDir(), "backup.sql.gz")
	s := &BackupService{}
	if _, err := s.executeDumpDirectToFile(context.Background(), &domain.BackupDbConfig{DBType: "postgresql"}, file); err == nil || !strings.Contains(err.Error(), "pg_dump is required") {
		t.Fatalf("missing dump client did not fail closed: %v", err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("partial backup retained after missing client: %v", err)
	}
}
