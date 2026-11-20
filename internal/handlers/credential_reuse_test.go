package handlers

import (
	"go-hephaestus/internal/core/domain"
	"testing"
)

func TestBackupCredentialsBoundToDestination(t *testing.T) {
	saved := domain.BackupDbConfig{DBType: "postgresql", Host: "trusted", Port: 5432, Username: "owner", DatabaseName: "db", Password: "secret"}
	next := saved
	next.Password = "********"
	next.Host = "attacker"
	if err := reuseBackupDBCredentials(&next, &saved); err == nil {
		t.Fatal("reused database secret on changed host")
	}
	next = saved
	next.Password = "********"
	if err := reuseBackupDBCredentials(&next, &saved); err != nil || next.Password != "secret" {
		t.Fatalf("same destination: %v", err)
	}
	storage := domain.BackupDestination{DestType: "s3", Config: map[string]interface{}{"endpoint": "https://trusted", "bucket": "backup", "secretAccessKey": "secret"}}
	request := domain.BackupDestination{DestType: "s3", Config: map[string]interface{}{"endpoint": "https://attacker", "bucket": "backup", "secretAccessKey": "********"}}
	if err := reuseBackupDestinationCredentials(&request, &storage); err == nil {
		t.Fatal("reused S3 secret on changed endpoint")
	}
	request.Config["endpoint"] = "https://trusted"
	if err := reuseBackupDestinationCredentials(&request, &storage); err != nil || request.Config["secretAccessKey"] != "secret" {
		t.Fatalf("same S3 destination: %v", err)
	}
}

func TestVaultIdentityBinding(t *testing.T) {
	saved := &domain.VaultwardenConfig{ServerURL: "https://vault.example", Email: "owner@example.test"}
	if sameVaultIdentity("https://attacker", "owner@example.test", saved) || sameVaultIdentity(saved.ServerURL, "attacker@example.test", saved) {
		t.Fatal("changed identity accepted")
	}
	if !sameVaultIdentity(saved.ServerURL+"/", "OWNER@example.test", saved) {
		t.Fatal("equivalent identity rejected")
	}
}
