package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"

	"go-hephaestus/internal/core/domain"
)

func TestIntegrationClientsDoNotFollowRedirects(t *testing.T) {
	var reached atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached.Add(1); w.WriteHeader(http.StatusOK) }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	clients := map[string]*http.Client{"vault": NewVaultwardenService(nil).httpClient, "report": NewReportService(nil, nil, nil, nil).httpClient, "status": NewStatusPageService(nil, nil, nil, nil, nil, nil, nil).httpClient}
	for name, client := range clients {
		t.Run(name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodPost, source.URL, nil)
			req.Header.Set("Authorization", "Bearer secret")
			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusTemporaryRedirect {
				t.Fatalf("redirect followed: %d", resp.StatusCode)
			}
		})
	}
	if reached.Load() != 0 {
		t.Fatal("credential-bearing redirect reached another destination")
	}
}

func TestReportTLSRejectsUntrustedCertificate(t *testing.T) {
	t.Setenv("ALLOW_INSECURE_INTEGRATION_TLS", "false")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer server.Close()
	client := NewReportService(nil, nil, nil, nil).httpClient
	if resp, err := client.Get(server.URL); err == nil {
		resp.Body.Close()
		t.Fatal("accepted untrusted TLS certificate")
	}
}

func TestOpenSearchReportsHonorPerProfileTLS(t *testing.T) {
	t.Setenv("ALLOW_INSECURE_INTEGRATION_TLS", "false")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":0},"hits":[]},"aggregations":{"events_over_time":{"buckets":[{"key_as_string":"2026-11-20T12:00:00Z","doc_count":1}]}}}`))
	}))
	defer server.Close()
	endpoint, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(endpoint.Port())
	cfg := &domain.OpenSearchConfig{Host: endpoint.Hostname(), Port: port, UseSSL: true, VerifySSL: false}
	report := NewReportService(nil, nil, nil, nil)
	query := domain.ReportQueryDataRequest{TimeRange: "1h"}
	if _, _, _, err := report.fetchOpenSearchLive(context.Background(), cfg, query); err != nil {
		t.Fatalf("explicit per-profile untrusted TLS failed: %v", err)
	}
	cfg.VerifySSL = true
	if _, _, _, err := report.fetchOpenSearchLive(context.Background(), cfg, query); err == nil {
		t.Fatal("profile requiring certificate verification accepted an untrusted certificate")
	}
}
