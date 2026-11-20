package services

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go-hephaestus/internal/config"
	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/repository"
)

const vendorTestPassword = "Hcp-CI-Only-Audit-2026!"

// All vendor destinations are fixed loopback ports. This suite cannot be pointed
// at a customer/production service by supplying an arbitrary URL.
func TestVendorLive(t *testing.T) {
	vendor := os.Getenv("HCP_AUDIT_VENDOR")
	if vendor == "" {
		t.Skip("opt-in real vendor containers are not enabled")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: config.RejectIntegrationRedirect}
	switch vendor {
	case "opensearch":
		// Only the ephemeral demo certificate is untrusted. Production TLS policy
		// is separately tested below and must reject this certificate when enabled.
		client.Transport = &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true}}
		testVendorOpenSearch(t, ctx, client)
	case "prometheus":
		testVendorPrometheus(t, ctx, client)
	case "grafana":
		testVendorGrafana(t, ctx, client)
	case "dataprepper":
		testVendorDataPrepper(t, ctx, client)
	case "otel":
		testVendorOTel(t, ctx, client)
	case "vaultwarden":
		testVendorVaultwarden(t, ctx, client)
	case "s3":
		testVendorS3(t, ctx, client)
	default:
		t.Fatalf("unsupported vendor fixture %q", vendor)
	}
}

func vendorCall(ctx context.Context, client *http.Client, method, url string, body interface{}, auth func(*http.Request)) (int, []byte, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth != nil {
		auth(req)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	return resp.StatusCode, raw, err
}

func requireVendorCall(t *testing.T, ctx context.Context, client *http.Client, method, url string, body interface{}, auth func(*http.Request)) []byte {
	t.Helper()
	status, raw, err := vendorCall(ctx, client, method, url, body, auth)
	if err != nil || status < 200 || status >= 300 {
		t.Fatalf("vendor request %s %s failed: status=%d err=%v body=%s", method, url, status, err, raw)
	}
	return raw
}

func awaitVendor(t *testing.T, ctx context.Context, client *http.Client, url string, auth func(*http.Request), accept func(int, []byte) bool) {
	t.Helper()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		status, body, err := vendorCall(ctx, client, http.MethodGet, url, nil, auth)
		if err == nil && accept(status, body) {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("vendor readiness failed for %s: last status=%d err=%v", url, status, err)
		case <-ticker.C:
		}
	}
}

func vendorOK(status int, _ []byte) bool { return status == http.StatusOK }
func vendorAdmin(req *http.Request)      { req.SetBasicAuth("admin", vendorTestPassword) }

func testVendorOpenSearch(t *testing.T, ctx context.Context, client *http.Client) {
	const base = "https://127.0.0.1:19200"
	awaitVendor(t, ctx, client, base+"/_cluster/health", vendorAdmin, vendorOK)
	service := NewOpenSearchService()
	if _, err := service.TestConnection(ctx, "127.0.0.1", 19200, "admin", vendorTestPassword, true, false); err != nil {
		t.Fatal(err)
	}
	if _, err := service.TestConnection(ctx, "127.0.0.1", 19200, "admin", "incorrect-password", true, false); err == nil {
		t.Fatal("OpenSearch accepted incorrect credentials")
	}
	if _, err := service.TestConnection(ctx, "127.0.0.1", 19200, "admin", vendorTestPassword, true, true); err == nil {
		t.Fatal("OpenSearch accepted untrusted demo certificate with VerifySSL enabled")
	}
	requireVendorCall(t, ctx, client, http.MethodPut, base+"/hcp-audit", map[string]interface{}{"mappings": map[string]interface{}{"properties": map[string]interface{}{"@timestamp": map[string]string{"type": "date"}}}}, vendorAdmin)
	requireVendorCall(t, ctx, client, http.MethodPost, base+"/hcp-audit/_doc?refresh=wait_for", map[string]interface{}{"@timestamp": time.Now().UTC().Format(time.RFC3339Nano), "message": "hcp-vendor-audit"}, vendorAdmin)
	report := NewReportService(nil, nil, nil, nil)
	_, _, rows, err := report.fetchOpenSearchLive(ctx, &domain.OpenSearchConfig{Host: "127.0.0.1", Port: 19200, Username: "admin", Password: vendorTestPassword, UseSSL: true, VerifySSL: false}, domain.ReportQueryDataRequest{TimeRange: "1h", SourceConfig: map[string]interface{}{"indexPattern": "hcp-audit"}})
	if err != nil || len(rows) != 1 {
		t.Fatalf("HCP OpenSearch report query: rows=%d err=%v", len(rows), err)
	}
}

func testVendorPrometheus(t *testing.T, ctx context.Context, client *http.Client) {
	const base = "http://127.0.0.1:19090"
	awaitVendor(t, ctx, client, base+"/-/ready", nil, vendorOK)
	report := NewReportService(nil, nil, nil, nil)
	points, summary, err := report.fetchPrometheusLive(ctx, &domain.PrometheusConfig{ReloadURL: base + "/-/reload"}, "vector(42)", "1h", "avg", "")
	if err != nil || len(points) == 0 {
		t.Fatalf("HCP Prometheus range query: %d points, summary=%+v err=%v", len(points), summary, err)
	}
	for _, point := range points {
		if point.Value != 42 {
			t.Fatalf("unexpected Prometheus sample: %+v", point)
		}
	}
	_, owner := vendorVaultDatabase(t, ctx)
	repo := repository.NewConfigRepository()
	if err := repo.SavePrometheus(ctx, domain.PrometheusConfig{ID: "vendor-prometheus", Name: "vendor", ReloadURL: base + "/-/reload", IsActive: true}, owner.ID, owner.Role); err != nil {
		t.Fatal(err)
	}
	report.configRepo = repo
	report.promService = NewPrometheusService(repo, nil)
	response, err := report.queryPrometheusData(ctx, domain.ReportQueryDataRequest{TimeRange: "1h", SourceConfig: map[string]interface{}{"query": "invalid_promql(", "targetHosts": []string{"host-a", "host-b"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Points) != 0 || len(response.Series) != 0 || len(response.TableRows) != 0 {
		t.Fatal("failed multi-host Prometheus query fabricated report data")
	}
}

func testVendorDataPrepper(t *testing.T, ctx context.Context, client *http.Client) {
	const base = "http://127.0.0.1:12021"
	awaitVendor(t, ctx, client, base+"/", nil, func(status int, _ []byte) bool { return status == 200 || status == 404 })
	content, err := os.ReadFile("../../tests/vendor/dataprepper-pipelines/audit.yml")
	if err != nil {
		t.Fatal(err)
	}
	valid, message, line := NewDataPrepperService(nil).ValidateYAML(string(content))
	if !valid || line != 0 {
		t.Fatalf("HCP rejected vendor pipeline: %s, line=%d", message, line)
	}
	requireVendorCall(t, ctx, client, http.MethodPost, base+"/log/ingest", []map[string]string{{"message": "hcp-vendor-audit"}}, nil)
}

func testVendorOTel(t *testing.T, ctx context.Context, client *http.Client) {
	awaitVendor(t, ctx, client, "http://127.0.0.1:13133/", nil, vendorOK)
	payload := map[string]interface{}{"resourceMetrics": []interface{}{map[string]interface{}{"scopeMetrics": []interface{}{map[string]interface{}{"scope": map[string]string{"name": "hcp-audit"}, "metrics": []interface{}{map[string]interface{}{"name": "hcp_audit_metric", "gauge": map[string]interface{}{"dataPoints": []interface{}{map[string]interface{}{"asDouble": 42.5, "timeUnixNano": strconv.FormatInt(time.Now().UnixNano(), 10)}}}}}}}}}}
	requireVendorCall(t, ctx, client, http.MethodPost, "http://127.0.0.1:14318/v1/metrics", payload, nil)
	awaitVendor(t, ctx, client, "http://127.0.0.1:19464/metrics", nil, func(status int, body []byte) bool {
		return status == 200 && strings.Contains(string(body), "hcp_audit_metric") && strings.Contains(string(body), "42.5")
	})
}

func testVendorS3(t *testing.T, ctx context.Context, client *http.Client) {
	const base = "http://127.0.0.1:19000"
	awaitVendor(t, ctx, client, base+"/", nil, func(status int, _ []byte) bool { return status == 200 || status == 403 })
	sdk := s3.NewFromConfig(aws.Config{Region: "us-east-1", Credentials: credentials.NewStaticCredentialsProvider("hcp-audit-access", vendorTestPassword, "")}, func(options *s3.Options) { options.BaseEndpoint = aws.String(base); options.UsePathStyle = true })
	if _, err := sdk.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String("hcp-audit")}); err != nil {
		t.Fatal(err)
	}
	dest := &domain.BackupDestination{DestType: "s3", Config: map[string]interface{}{"endpoint": base, "bucket": "hcp-audit", "accessKeyId": "hcp-audit-access", "secretAccessKey": vendorTestPassword}}
	service := &BackupService{}
	if err := service.uploadToDestination(ctx, []byte("hcp-vendor-audit"), "audit.sql", dest); err != nil {
		t.Fatal(err)
	}
	object, err := sdk.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String("hcp-audit"), Key: aws.String("audit.sql")})
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(object.Body)
	_ = object.Body.Close()
	if err != nil || string(data) != "hcp-vendor-audit" {
		t.Fatalf("S3 object mismatch: %q %v", data, err)
	}
	bad := &domain.BackupDestination{DestType: "s3", Config: map[string]interface{}{"endpoint": base, "bucket": "hcp-audit", "accessKeyId": "hcp-audit-access", "secretAccessKey": "incorrect-password"}}
	if err := service.uploadToDestination(ctx, []byte("invalid"), "must-not-exist.sql", bad); err == nil {
		t.Fatal("S3-compatible vendor accepted incorrect credentials")
	}
	t.Logf("verified signed upload/download against %s", base)
}
