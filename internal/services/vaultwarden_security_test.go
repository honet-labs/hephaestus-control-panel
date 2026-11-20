package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestVaultCachedSessionRequiresMatchingPassword(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/accounts/prelogin" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"kdf":0,"kdfIterations":1}`))
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	s := NewVaultwardenService(nil)
	s.sessions[s.getCacheKey(server.URL, "owner@example.test")] = &vwCachedSession{PasswordBinding: sha256.Sum256([]byte("correct")), AccessToken: "privileged-token", ExpiresAt: time.Now().Add(time.Hour)}
	if auth, err := s.authenticateAndGetKeys(context.Background(), server.URL, "owner@example.test", "wrong-password"); err == nil || auth != nil {
		t.Fatal("wrong password reused another profile's cached token")
	}
	if auth, err := s.authenticateAndGetKeys(context.Background(), server.URL, "owner@example.test", "correct"); err != nil || auth.AccessToken != "privileged-token" {
		t.Fatalf("valid cached session lost: %v", err)
	}
	if s.getCacheKey(server.URL+"/Vault", "owner@example.test") == s.getCacheKey(server.URL+"/vault", "owner@example.test") {
		t.Fatal("case-sensitive endpoint paths collided")
	}
}

func TestVaultKDFRejectsUntrustedResourceRequests(t *testing.T) {
	s := NewVaultwardenService(nil)
	memory, parallel := 1000000, 256
	for _, input := range []*bwPreloginResponse{nil, {Kdf: 0, KdfIterations: -1}, {Kdf: 0, KdfIterations: 2000001}, {Kdf: 1, KdfIterations: 600000}, {Kdf: 1, KdfIterations: 3, KdfMemory: &memory}, {Kdf: 1, KdfIterations: 3, KdfParallelism: &parallel}, {Kdf: 99, KdfIterations: 1}} {
		if _, err := s.deriveMasterKey("test@example.test", "secret", input); err == nil {
			t.Errorf("accepted unsafe KDF parameters: %+v", input)
		}
	}
	key, err := s.deriveMasterKey("test@example.test", "secret", &bwPreloginResponse{Kdf: 0, KdfIterations: 1})
	if err != nil || len(key) != 32 {
		t.Fatalf("valid derivation failed: %v", err)
	}
}

func TestVaultCBCAndAuthenticatedCipher(t *testing.T) {
	s := NewVaultwardenService(nil)
	key, mac := bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32)
	if _, err := s.aesCbcDecrypt(make([]byte, 16), key, []byte{1}); err == nil {
		t.Fatal("accepted invalid IV")
	}
	sealed, err := s.encryptCipherString("audit-secret", key, mac)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := s.decryptCipherBytes(sealed, key, mac)
	if err != nil || string(plain) != "audit-secret" {
		t.Fatalf("roundtrip: %v", err)
	}
	if _, err := s.decryptCipherBytes(sealed, key, key); err == nil {
		t.Fatal("accepted incorrect authentication key")
	}
}
