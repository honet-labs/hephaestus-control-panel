package services

import (
	"net/url"
	"strings"

	"go-hephaestus/internal/core/domain"
)

func backupS3Region(dest *domain.BackupDestination) string {
	if region, ok := dest.Config["region"].(string); ok && strings.TrimSpace(region) != "" {
		return strings.TrimSpace(region)
	}
	endpoint, _ := dest.Config["endpoint"].(string)
	parsed, _ := url.Parse(endpoint)
	if dest.DestType == "r2" || (parsed != nil && strings.HasSuffix(parsed.Hostname(), ".r2.cloudflarestorage.com")) {
		return "auto"
	}
	return "us-east-1"
}
