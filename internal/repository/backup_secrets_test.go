package repository

import (
	"go-hephaestus/internal/config"
	"testing"
)

func TestDestinationSecretEncryptionExactlyOnce(t *testing.T) {
	stored := map[string]interface{}{}
	for _, field := range destinationSecretFields {
		value, err := config.EncryptText("secret-" + field)
		if err != nil {
			t.Fatal(err)
		}
		stored[field] = value
	}
	next := map[string]interface{}{"password": "********", "sshKey": "", "secretAccessKey": "********"}
	if err := encryptDestinationSecrets(next, stored); err != nil {
		t.Fatal(err)
	}
	for _, field := range destinationSecretFields {
		if next[field] != stored[field] {
			t.Fatalf("ciphertext changed on masked save: %s", field)
		}
		plain, err := config.DecryptText(next[field].(string))
		if err != nil || plain != "secret-"+field {
			t.Fatalf("double encrypted %s: %v", field, err)
		}
	}
	next = map[string]interface{}{"sshKey": "new-private-key"}
	if err := encryptDestinationSecrets(next, nil); err != nil {
		t.Fatal(err)
	}
	if next["sshKey"] == "new-private-key" {
		t.Fatal("private key stored plaintext")
	}
}
