package handlers

import (
	"fmt"
	"strings"

	"go-hephaestus/internal/core/domain"
)

func missingSecret(value string) bool {
	return value == "" || value == "********" || value == "••••••••"
}
func missingSecretPointer(value *string) bool { return value == nil || missingSecret(*value) }

func reuseBackupDBCredentials(next, saved *domain.BackupDbConfig) error {
	if missingSecret(next.Password) {
		if next.DBType != saved.DBType || next.Host != saved.Host || next.Port != saved.Port || next.Username != saved.Username || next.DatabaseName != saved.DatabaseName {
			return fmt.Errorf("supply a fresh database password when testing a different database destination")
		}
		next.Password = saved.Password
	}
	if (missingSecretPointer(next.SSHPassword) && saved.SSHPassword != nil) || (missingSecretPointer(next.SSHKey) && saved.SSHKey != nil) {
		if stringPointer(next.SSHHost) != stringPointer(saved.SSHHost) || stringPointer(next.SSHUser) != stringPointer(saved.SSHUser) || portPointer(next.SSHPort) != portPointer(saved.SSHPort) {
			return fmt.Errorf("supply fresh SSH credentials when testing a different SSH destination")
		}
		if missingSecretPointer(next.SSHPassword) {
			next.SSHPassword = saved.SSHPassword
		}
		if missingSecretPointer(next.SSHKey) {
			next.SSHKey = saved.SSHKey
		}
	}
	return nil
}

func stringPointer(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
func portPointer(value *int) int {
	if value == nil || *value == 0 {
		return 22
	}
	return *value
}

func reuseBackupDestinationCredentials(next, saved *domain.BackupDestination) error {
	fields := []string{"secretAccessKey", "password", "sshKey"}
	usesSaved := false
	for _, field := range fields {
		value, _ := next.Config[field].(string)
		old, _ := saved.Config[field].(string)
		if missingSecret(value) && old != "" {
			usesSaved = true
		}
	}
	if !usesSaved {
		return nil
	}
	if next.DestType != saved.DestType {
		return fmt.Errorf("supply fresh credentials when changing storage type")
	}
	for _, key := range []string{"host", "port", "username", "authType", "endpoint", "bucket", "region", "accountId", "accessKeyId", "path"} {
		if fmt.Sprint(next.Config[key]) != fmt.Sprint(saved.Config[key]) {
			return fmt.Errorf("supply fresh credentials when changing destination %s", key)
		}
	}
	if next.Config == nil {
		next.Config = make(map[string]interface{})
	}
	for _, field := range fields {
		value, _ := next.Config[field].(string)
		if missingSecret(value) {
			next.Config[field] = saved.Config[field]
		}
	}
	return nil
}

func sameVaultIdentity(url, email string, saved *domain.VaultwardenConfig) bool {
	return strings.TrimRight(strings.TrimSpace(url), "/") == strings.TrimRight(saved.ServerURL, "/") && strings.EqualFold(strings.TrimSpace(email), saved.Email)
}
