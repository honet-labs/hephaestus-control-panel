package config

import (
	"crypto/tls"
	"os"
	"strings"
)

// Default to certificate verification. Prefer installing an internal CA over
// the documented deployment-wide compatibility escape hatch.
func IntegrationTLSConfig() *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: strings.EqualFold(os.Getenv("ALLOW_INSECURE_INTEGRATION_TLS"), "true")}
}
