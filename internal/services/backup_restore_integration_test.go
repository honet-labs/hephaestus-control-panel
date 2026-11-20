package services

import (
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"go-hephaestus/internal/config"
	"go-hephaestus/internal/core/domain"
)

// The production dump method must produce an archive that restores schema and
// typed data through PostgreSQL's real client, not merely a nonempty gzip file.
func TestPostgresBackupRestoreRoundTrip(t *testing.T) {
	if os.Getenv("HCP_AUDIT_POSTGRES") != "1" {
		t.Skip("isolated PostgreSQL test not enabled")
	}
	dump, err := exec.LookPath("pg_dump")
	if err != nil {
		if os.Getenv("HCP_AUDIT_REQUIRE_DUMP") == "1" {
			t.Fatal(err)
		}
		t.Skip("pg_dump unavailable; restore test required in CI")
	}
	psql, err := exec.LookPath("psql")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("using dump client %s", dump)
	port, err := strconv.Atoi(os.Getenv("HCP_AUDIT_POSTGRES_PORT"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db := config.DBConfig{Host: "127.0.0.1", Port: port, User: os.Getenv("HCP_AUDIT_POSTGRES_USER"), Password: os.Getenv("HCP_AUDIT_POSTGRES_PASSWORD")}
	admin, err := pgx.Connect(ctx, db.AdminConnString())
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(context.Background())
	source := fmt.Sprintf("hcp_audit_dump_%d", time.Now().UnixNano())
	target := source + "_restore"
	for _, name := range []string{source, target} {
		if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
			t.Fatal(err)
		}
		defer func(name string) {
			_, err := admin.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
			if err != nil {
				t.Errorf("cleanup %s: %v", name, err)
			}
		}(name)
	}
	db.Database = source
	conn, err := pgx.Connect(ctx, db.ConnString())
	if err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(ctx, `CREATE TABLE "odd""table" (id INTEGER PRIMARY KEY, data BYTEA, enabled BOOLEAN, message TEXT, nullable TEXT); INSERT INTO "odd""table" VALUES (1,decode('00ff42','hex'),true,'backup ''quoted'' unicode Ω',NULL)`)
	_ = conn.Close(ctx)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "roundtrip.sql.gz")
	s := &BackupService{}
	if _, err := s.executeDumpDirectToFile(ctx, &domain.BackupDbConfig{DBType: "postgresql", Host: db.Host, Port: db.Port, Username: db.User, Password: db.Password, DatabaseName: source}, file); err != nil {
		t.Fatal(err)
	}
	archive, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	reader, err := gzip.NewReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	cmd := exec.CommandContext(ctx, psql, "-X", "-h", db.Host, "-p", strconv.Itoa(db.Port), "-U", db.User, "-d", target, "-v", "ON_ERROR_STOP=1")
	cmd.Env = append(os.Environ(), "PGPASSWORD="+db.Password)
	cmd.Stdin = reader
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("restore failed: %v: %s", err, output)
	}
	db.Database = target
	restored, err := pgx.Connect(ctx, db.ConnString())
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close(context.Background())
	var data []byte
	var enabled bool
	var message string
	var nullable *string
	if err := restored.QueryRow(ctx, `SELECT data,enabled,message,nullable FROM "odd""table" WHERE id=1`).Scan(&data, &enabled, &message, &nullable); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", data) != "00ff42" || !enabled || message != "backup 'quoted' unicode Ω" || nullable != nil {
		t.Fatalf("restored data mismatch: %x %t %q %v", data, enabled, message, nullable)
	}
}
