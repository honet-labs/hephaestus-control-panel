package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"go-hephaestus/internal/config"
	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"
	"go-hephaestus/internal/repository"
	"golang.org/x/crypto/hkdf"
)

func testVendorGrafana(t *testing.T, ctx context.Context, client *http.Client) {
	const base = "http://127.0.0.1:13000"
	awaitVendor(t, ctx, client, base+"/api/health", nil, vendorOK)
	accountRaw := requireVendorCall(t, ctx, client, http.MethodPost, base+"/api/serviceaccounts", map[string]string{"name": "hcp-audit", "role": "Admin"}, vendorAdmin)
	var account struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(accountRaw, &account); err != nil || account.ID < 1 {
		t.Fatalf("Grafana service-account response: %v", err)
	}
	tokenRaw := requireVendorCall(t, ctx, client, http.MethodPost, fmt.Sprintf("%s/api/serviceaccounts/%d/tokens", base, account.ID), map[string]string{"name": "hcp-audit"}, vendorAdmin)
	var token struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(tokenRaw, &token); err != nil || token.Key == "" {
		t.Fatalf("Grafana token response: %v", err)
	}
	requireVendorCall(t, ctx, client, http.MethodPost, base+"/api/datasources", map[string]interface{}{"name": "hcp-audit", "uid": "hcp-audit-prometheus", "type": "prometheus", "access": "proxy", "url": "http://prometheus:9090", "isDefault": true}, vendorAdmin)
	cfg := &domain.GrafanaConfig{Host: base, Token: token.Key, DatasourceUID: "hcp-audit-prometheus"}
	query := domain.ReportQueryDataRequest{TimeRange: "1h", SourceConfig: map[string]interface{}{"query": "vector(42)"}}
	report := NewReportService(nil, nil, nil, nil)
	points, _, err := report.fetchGrafanaLive(ctx, cfg, query)
	if err != nil || len(points) == 0 {
		t.Fatalf("HCP Grafana query: points=%d err=%v", len(points), err)
	}
	for _, point := range points {
		if point.Value != 42 {
			t.Fatalf("Grafana sample mismatch: %+v", point)
		}
	}
	cfg.Token = "invalid-service-account-token"
	if _, _, err := report.fetchGrafanaLive(ctx, cfg, query); err == nil {
		t.Fatal("Grafana accepted an invalid token")
	}
	_, owner := vendorVaultDatabase(t, ctx)
	repo := repository.NewConfigRepository()
	if err := repo.SaveGrafana(ctx, domain.GrafanaConfig{ID: "vendor-grafana", Name: "vendor", Host: base, Token: cfg.Token, DatasourceUID: cfg.DatasourceUID, IsActive: true}, owner.ID, owner.Role); err != nil {
		t.Fatal(err)
	}
	report.configRepo = repo
	query.SourceConfig["targetHosts"] = []string{"host-a", "host-b"}
	response, err := report.queryGrafanaData(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Points) != 0 || len(response.Series) != 0 || len(response.TableRows) != 0 {
		t.Fatal("failed multi-host Grafana query fabricated report data")
	}
	delete(query.SourceConfig, "targetHosts")
	response, err = report.queryGrafanaData(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Points) != 0 {
		t.Fatal("failed Grafana query relabelled simulated Prometheus data as live")
	}
}

func vendorVaultDatabase(t *testing.T, ctx context.Context) (*repository.VaultwardenRepository, *domain.User) {
	t.Helper()
	port, err := strconv.Atoi(os.Getenv("HCP_AUDIT_POSTGRES_PORT"))
	if err != nil {
		t.Fatal("isolated PostgreSQL port is required for the vendor vault test")
	}
	cfg := config.DBConfig{Host: "127.0.0.1", Port: port, User: os.Getenv("HCP_AUDIT_POSTGRES_USER"), Password: os.Getenv("HCP_AUDIT_POSTGRES_PASSWORD"), Database: fmt.Sprintf("hcp_audit_vendor_%d", time.Now().UnixNano()), MaxConns: 5}
	admin, err := pgx.Connect(ctx, cfg.AdminConnString())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close(context.Background()) })
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{cfg.Database}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, err := admin.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{cfg.Database}.Sanitize()+" WITH (FORCE)")
		if err != nil {
			t.Errorf("cleanup vendor database: %v", err)
		}
	})
	if err := database.InitDatabase(ctx, &config.Config{DB: cfg}); err != nil {
		t.Fatal(err)
	}
	pool, err := database.GetPool()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	owner, err := repository.NewUserRepository().CreateInitialAdmin(ctx, "vendor-admin", "test-only-hash")
	if err != nil {
		t.Fatal(err)
	}
	return repository.NewVaultwardenRepository(), owner
}

func testVendorVaultwarden(t *testing.T, ctx context.Context, client *http.Client) {
	const base = "http://127.0.0.1:18280"
	awaitVendor(t, ctx, client, base+"/alive", nil, vendorOK)
	email := fmt.Sprintf("hcp-%d@example.test", time.Now().UnixNano())
	service := NewVaultwardenService(nil)
	master, err := service.deriveMasterKey(email, vendorTestPassword, &bwPreloginResponse{Kdf: 0, KdfIterations: 600000})
	if err != nil {
		t.Fatal(err)
	}
	enc, mac, userKey := make([]byte, 32), make([]byte, 32), make([]byte, 64)
	if _, err := io.ReadFull(hkdf.Expand(sha256.New, master, []byte("enc")), enc); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(hkdf.Expand(sha256.New, master, []byte("mac")), mac); err != nil {
		t.Fatal(err)
	}
	if _, err := rand.Read(userKey); err != nil {
		t.Fatal(err)
	}
	wrappedKey, err := service.encryptCipherString(string(userKey), enc, mac)
	if err != nil {
		t.Fatal(err)
	}
	registration := map[string]interface{}{"email": email, "masterPasswordHash": service.computePasswordHash(master, vendorTestPassword), "key": wrappedKey, "kdf": 0, "kdfIterations": 600000}
	status, _, err := vendorCall(ctx, client, http.MethodPost, base+"/api/accounts/register", registration, nil)
	if status == http.StatusNotFound {
		status, _, err = vendorCall(ctx, client, http.MethodPost, base+"/api/accounts/register/finish", registration, nil)
	}
	if err != nil || status < 200 || status >= 300 {
		t.Fatalf("Vaultwarden fixture registration: status=%d err=%v", status, err)
	}
	if ok, message, count := service.TestConnection(ctx, base, email, vendorTestPassword); !ok || count != 0 {
		t.Fatalf("HCP initial vault connection: ok=%v count=%d %s", ok, count, message)
	}
	if ok, _, _ := service.TestConnection(ctx, base, email, "incorrect-password"); ok {
		t.Fatal("incorrect password reused a cached vendor session")
	}
	repo, owner := vendorVaultDatabase(t, ctx)
	saved, err := repo.SaveConfig(ctx, domain.VaultwardenConfig{Name: "vendor-vault", ServerURL: base, Email: email, MasterPassword: vendorTestPassword, IsActive: true, Visibility: "private"}, owner.ID, owner.Role)
	if err != nil {
		t.Fatal(err)
	}
	service.repo = repo
	request := domain.CreateVaultCipherRequest{Type: 1, Name: "hcp-audit-login", Username: "audit-user", Password: "audit-item-password", URI: "https://example.test"}
	item, err := service.CreateCipher(ctx, owner.ID, owner.Role, request, saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	request.Name = "hcp-audit-updated"
	if _, err := service.UpdateCipher(ctx, owner.ID, owner.Role, item.ID, request, saved.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SyncVault(ctx, owner.ID, owner.Role, saved.ID); err != nil {
		t.Fatal(err)
	}
	items, err := service.GetCiphers(ctx, owner.ID, owner.Role, "", "", saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Name != request.Name || items[0].Password != request.Password {
		t.Fatal("vendor vault encrypted CRUD/decryption round-trip failed")
	}
	if err := service.DeleteCipher(ctx, owner.ID, owner.Role, item.ID, saved.ID); err != nil {
		t.Fatal(err)
	}
}
