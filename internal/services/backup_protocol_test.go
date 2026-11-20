package services

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"go-hephaestus/internal/core/domain"
)

func TestS3CompatibleBackupUploads(t *testing.T) {
	var uploads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil || r.Method != http.MethodPut || !strings.HasPrefix(r.URL.Path, "/audit-bucket/") || !strings.Contains(string(body), "audit-payload") || !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 ") {
			t.Errorf("invalid signed S3 request: %s %s, body=%q, err=%v", r.Method, r.URL.Path, body, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		uploads.Add(1)
		w.Header().Set("ETag", `"audit"`)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	s := &BackupService{}
	dest := &domain.BackupDestination{DestType: "s3", Config: map[string]interface{}{"bucket": "audit-bucket", "endpoint": server.URL, "accessKeyId": "audit-access", "secretAccessKey": "audit-secret"}}
	if err := s.uploadToDestination(context.Background(), []byte("audit-payload"), "memory.sql", dest); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "dump.sql")
	if err := os.WriteFile(file, []byte("audit-payload"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.uploadFileToDestination(context.Background(), file, "file.sql", dest); err != nil {
		t.Fatal(err)
	}
	if uploads.Load() != 2 {
		t.Fatalf("expected two uploads, got %d", uploads.Load())
	}
}

func TestBackupS3RegionPolicy(t *testing.T) {
	for _, tc := range []struct{ kind, endpoint, configured, want string }{
		{"s3", "http://127.0.0.1:9000", "", "us-east-1"},
		{"r2", "https://account.r2.cloudflarestorage.com", "", "auto"},
		{"s3", "https://account.r2.cloudflarestorage.com", "", "auto"},
		{"s3", "https://s3.eu-west-1.amazonaws.com", "eu-west-1", "eu-west-1"},
	} {
		dest := &domain.BackupDestination{DestType: tc.kind, Config: map[string]interface{}{"endpoint": tc.endpoint, "region": tc.configured}}
		if got := backupS3Region(dest); got != tc.want {
			t.Errorf("%s %s: got %s want %s", tc.kind, tc.endpoint, got, tc.want)
		}
	}
}
