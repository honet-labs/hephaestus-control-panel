package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestVaultPreloginEndpointCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		legacyStatus, identityCalls int
		wantError                   bool
	}{
		{"legacy", http.StatusOK, 0, false},
		{"identity", http.StatusNotFound, 1, false},
		{"unauthorized", http.StatusUnauthorized, 0, true},
		{"rate-limited", http.StatusTooManyRequests, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("unexpected method %s", r.Method)
				}
				switch r.URL.Path {
				case "/vault/api/accounts/prelogin":
					w.WriteHeader(tc.legacyStatus)
				case "/vault/identity/accounts/prelogin":
					calls.Add(1)
				default:
					t.Errorf("unexpected path %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
				_, _ = w.Write([]byte(`{"kdf":0,"kdfIterations":600000}`))
			}))
			defer server.Close()
			result, err := NewVaultwardenService(nil).doPrelogin(context.Background(), server.URL+"/vault", "audit@example.test")
			if (err != nil) != tc.wantError {
				t.Fatalf("unexpected prelogin error: %v", err)
			}
			if !tc.wantError && (result == nil || result.KdfIterations != 600000) {
				t.Fatal("prelogin parameters lost")
			}
			if got := calls.Load(); int(got) != tc.identityCalls {
				t.Fatalf("Identity calls=%d, want %d", got, tc.identityCalls)
			}
		})
	}
}

func TestVaultCipherRecipientRequiresAuthenticatedProfile(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		wantError  bool
	}{
		{"valid", `{"id":"server-account-id"}`, http.StatusOK, false},
		{"missing-id", `{}`, http.StatusOK, true},
		{"invalid-json", `{`, http.StatusOK, true},
		{"unauthorized", `{}`, http.StatusUnauthorized, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/accounts/profile" || r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer test-token" {
					t.Error("profile request did not bind authenticated account")
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			id, err := NewVaultwardenService(nil).cipherRecipient(context.Background(), &vwAuthContext{ServerURL: server.URL, AccessToken: "test-token"})
			if (err != nil) != tc.wantError {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.wantError && id != "server-account-id" {
				t.Fatal("incorrect encryption recipient")
			}
		})
	}
}
