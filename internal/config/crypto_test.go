package config

import (
	"strings"
	"testing"
)

func TestCredentialEncryptionValidation(t *testing.T) {
	key, fallback := defaultSecretKey, isDefaultKey
	t.Cleanup(func() { defaultSecretKey, isDefaultKey = key, fallback })
	SetSecretKey("test-only-encryption-key-with-at-least-32-characters")
	sealed, err := EncryptText("secret-password")
	if err != nil {
		t.Fatal(err)
	}
	plain, err := DecryptText(sealed)
	if err != nil || plain != "secret-password" {
		t.Fatalf("roundtrip: %q, %v", plain, err)
	}
	parts := strings.Split(sealed, ":")
	for _, invalid := range []string{"00:00:00", ":" + parts[1] + ":" + parts[2], parts[0] + ":00:" + parts[2], parts[0] + ":" + parts[1] + ":00"} {
		if _, err := DecryptText(invalid); err == nil {
			t.Errorf("accepted malformed ciphertext %q", invalid)
		}
	}
	for _, weak := range []string{"short", "replace_with_a_secure_random_32_byte_key", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"} {
		SetSecretKey(weak)
		if !IsUsingDefaultSecretKey() {
			t.Errorf("accepted weak key %q", weak)
		}
	}
}
