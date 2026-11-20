package integration_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-hephaestus/internal/config"
	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"
	"go-hephaestus/internal/handlers"
	"go-hephaestus/internal/repository"
	"go-hephaestus/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

// Opt in explicitly. Only a newly-created, uniquely named audit database is
// modified/dropped; the configured application's database is never opened.
func TestPostgresSecurity(t *testing.T) {
	if os.Getenv("HCP_AUDIT_POSTGRES") != "1" {
		t.Skip("set HCP_AUDIT_POSTGRES=1 for isolated live PostgreSQL tests")
	}
	port, err := strconv.Atoi(os.Getenv("HCP_AUDIT_POSTGRES_PORT"))
	if err != nil || port < 1 {
		t.Fatal("set HCP_AUDIT_POSTGRES_PORT to a loopback test PostgreSQL port")
	}
	ctx := context.Background()
	db := config.DBConfig{Host: "127.0.0.1", Port: port, User: os.Getenv("HCP_AUDIT_POSTGRES_USER"), Password: os.Getenv("HCP_AUDIT_POSTGRES_PASSWORD"), Database: fmt.Sprintf("hcp_audit_%d", time.Now().UnixNano()), MaxConns: 12}
	admin, err := pgx.Connect(ctx, db.AdminConnString())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close(ctx) })
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{db.Database}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(ctx, `SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname=$1 AND pid <> pg_backend_pid()`, db.Database)
		if _, err := admin.Exec(ctx, "DROP DATABASE "+pgx.Identifier{db.Database}.Sanitize()); err != nil {
			t.Errorf("cleanup test database: %v", err)
		}
	})
	if err := database.InitDatabase(ctx, &config.Config{DB: db}); err != nil {
		t.Fatal(err)
	}
	if err := database.InitDatabase(ctx, &config.Config{DB: db}); err != nil {
		t.Fatalf("idempotent migrations: %v", err)
	}
	pool, err := database.GetPool()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	users := repository.NewUserRepository()
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := users.CreateInitialAdmin(ctx, fmt.Sprintf("initial-%d", i), "test-hash"); err == nil {
				successes.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("concurrent setup created %d administrators", successes.Load())
	}
	owner, err := users.Create(ctx, "audit-owner", "test-hash", "OPERATOR", false)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := users.Create(ctx, "audit-reader", "test-hash", "VIEWER", false)
	if err != nil {
		t.Fatal(err)
	}
	outsider, err := users.Create(ctx, "audit-outsider", "test-hash", "VIEWER", false)
	if err != nil {
		t.Fatal(err)
	}
	ownerCtx := domain.ContextWithUser(ctx, owner)
	readerCtx := domain.ContextWithUser(ctx, reader)
	outsiderCtx := domain.ContextWithUser(ctx, outsider)

	t.Run("profile access matrix", func(t *testing.T) {
		inserts := map[string]string{
			"grafana":       `INSERT INTO grafana_configs(id,name,host,token,datasource_uid,user_id) VALUES($1,'audit','http://127.0.0.1','test','ds',$2)`,
			"prometheus":    `INSERT INTO prometheus_configs(id,name,user_id) VALUES($1,'audit',$2)`,
			"dataprepper":   `INSERT INTO dataprepper_configs(id,name,user_id) VALUES($1,'audit',$2)`,
			"opensearch":    `INSERT INTO opensearch_configs(id,name,host,username,user_id) VALUES($1,'audit','127.0.0.1','audit',$2)`,
			"opentelemetry": `INSERT INTO opentelemetry_configs(id,name,ssh_host,ssh_user,user_id) VALUES($1,'audit','127.0.0.1','audit',$2)`,
		}
		for profile, query := range inserts {
			t.Run(profile, func(t *testing.T) {
				id := "audit-" + profile
				if _, err := pool.Exec(ctx, query, id, owner.ID); err != nil {
					t.Fatal(err)
				}
				check := func(actor context.Context, action string, allowed bool) {
					t.Helper()
					err := repository.RequireConnectionAccess(actor, profile+"_configs", profile+"_shares", "config_id", id, action)
					if (err == nil) != allowed {
						t.Fatalf("%s allowed=%v: %v", action, allowed, err)
					}
				}
				check(ownerCtx, "manage", true)
				check(outsiderCtx, "read", false)
				shares := repository.NewConnectionShareRepository()
				if err := shares.AddShare(ctx, profile+"_shares", "config_id", id, reader.ID, "read", owner.ID); err != nil {
					t.Fatal(err)
				}
				check(readerCtx, "read", true)
				check(readerCtx, "manage", false)
				if err := shares.AddShare(ctx, profile+"_shares", "config_id", id, reader.ID, "manage", owner.ID); err != nil {
					t.Fatal(err)
				}
				check(readerCtx, "manage", true)
				if err := shares.DeleteShare(ctx, profile+"_shares", "config_id", id, reader.ID); err != nil {
					t.Fatal(err)
				}
				check(readerCtx, "read", false)
				if _, err := pool.Exec(ctx, "UPDATE "+profile+"_configs SET visibility='public' WHERE id=$1", id); err != nil {
					t.Fatal(err)
				}
				check(outsiderCtx, "read", true)
				check(outsiderCtx, "manage", false)
			})
		}
	})

	t.Run("Prometheus direct retrieval and selected write", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `UPDATE prometheus_configs SET visibility='private',is_active=true WHERE id='audit-prometheus'`); err != nil {
			t.Fatal(err)
		}
		repo := repository.NewConfigRepository()
		local, err := repo.GetPrometheusByID(ownerCtx, "audit-prometheus")
		if err != nil {
			t.Fatal(err)
		}
		local.Path = "/arbitrary/engine/file"
		if err := repo.SavePrometheus(ownerCtx, *local, owner.ID, owner.Role); err == nil {
			t.Fatal("non-admin retargeted a local file profile")
		}
		local.ID = "new-local-profile"
		if err := repo.SavePrometheus(ownerCtx, *local, owner.ID, owner.Role); err == nil {
			t.Fatal("non-admin created a local file profile")
		}
		if _, err := repo.GetActivePrometheus(outsiderCtx); err == nil {
			t.Fatal("report-style active lookup leaked another user's profile")
		}
		if _, err := repo.GetPrometheusByID(outsiderCtx, "audit-prometheus"); err == nil {
			t.Fatal("direct lookup bypassed profile authorization")
		}
		if err := repository.NewConnectionShareRepository().AddShare(ctx, "prometheus_shares", "config_id", "audit-prometheus", reader.ID, "read", owner.ID); err != nil {
			t.Fatal(err)
		}
		gin.SetMode(gin.TestMode)
		for _, tc := range []struct {
			user   *domain.User
			status int
		}{{reader, http.StatusForbidden}, {owner, http.StatusOK}} {
			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set("user", tc.user)
				c.Request = c.Request.WithContext(domain.ContextWithUser(ctx, tc.user))
			})
			r.Use(handlers.IntegrationAccessGuard(nil, nil))
			r.POST("/api/v1/prometheus/config", func(c *gin.Context) { c.Status(http.StatusOK) })
			res := httptest.NewRecorder()
			r.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/prometheus/config", bytes.NewBufferString(`{"instanceId":"audit-prometheus","yaml":"test"}`)))
			if res.Code != tc.status {
				t.Fatalf("selected profile write for user %d: %d", tc.user.ID, res.Code)
			}
		}
		service := services.NewOpenSearchService()
		if _, err := service.SetActiveConfig(readerCtx, "audit-opensearch", reader.ID, reader.Role); err == nil {
			t.Fatal("public OpenSearch reader activated global profile")
		}
	})

	t.Run("HTTP resource guard", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		r.Use(func(c *gin.Context) { c.Set("user", outsider); c.Request = c.Request.WithContext(outsiderCtx) })
		r.Use(handlers.IntegrationAccessGuard(nil, nil))
		r.PUT("/api/v1/otel/hosts/:id", func(c *gin.Context) { c.Status(http.StatusOK) })
		res := httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest(http.MethodPut, "/api/v1/otel/hosts/audit-opentelemetry", nil))
		if res.Code != http.StatusForbidden {
			t.Fatalf("public profile write: %d", res.Code)
		}
	})

	t.Run("session revocation", func(t *testing.T) {
		token := config.HashToken("audit-token")
		if _, err := users.CreateSession(ctx, owner.ID, token, time.Hour); err != nil {
			t.Fatal(err)
		}
		if err := users.UpdatePassword(ctx, owner.ID, "new-hash", false); err != nil {
			t.Fatal(err)
		}
		if _, _, err := users.GetSessionByToken(ctx, token); err == nil {
			t.Fatal("password change left session valid")
		}
		if _, err := users.CreateSession(ctx, owner.ID, token, time.Hour); err != nil {
			t.Fatal(err)
		}
		if err := users.UpdateUser(ctx, owner.ID, "", "admin-reset-hash", "", nil); err != nil {
			t.Fatal(err)
		}
		if _, _, err := users.GetSessionByToken(ctx, token); err == nil {
			t.Fatal("admin reset left session valid")
		}
	})

	t.Run("destination secrets survive editing", func(t *testing.T) {
		repo := repository.NewBackupRepository()
		d := domain.BackupDestination{ID: "audit-destination", Name: "audit", DestType: "s3", Config: map[string]interface{}{"secretAccessKey": "audit-secret", "password": "audit-pass", "sshKey": "audit-key", "bucket": "audit-bucket"}}
		if err := repo.SaveDestination(ctx, d); err != nil {
			t.Fatal(err)
		}
		for _, field := range []string{"secretAccessKey", "password", "sshKey"} {
			d.Config[field] = "********"
		}
		if err := repo.SaveDestination(ctx, d); err != nil {
			t.Fatal(err)
		}
		saved, err := repo.GetRawDestination(ctx, d.ID)
		if err != nil {
			t.Fatal(err)
		}
		if saved.Config["secretAccessKey"] != "audit-secret" || saved.Config["sshKey"] != "audit-key" || saved.Config["password"] != "audit-pass" {
			t.Fatal("secret changed/double encrypted")
		}
		list, err := repo.ListDestinations(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range list {
			if item.ID != d.ID {
				continue
			}
			for _, field := range []string{"secretAccessKey", "password", "sshKey"} {
				if item.Config[field] != "********" {
					t.Fatalf("unmasked %s", field)
				}
			}
		}
	})
}
