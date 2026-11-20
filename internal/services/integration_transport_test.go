package services

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
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
