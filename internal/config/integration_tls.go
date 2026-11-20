package config

import (
	"crypto/tls"
	"net/http"
	"os"
	"strings"
)

// Credentials are bound to the configured endpoint, never a redirect target.
func RejectIntegrationRedirect(_ *http.Request, _ []*http.Request) error {
	return http.ErrUseLastResponse
}

// Default to certificate verification. Prefer installing an internal CA over
// the documented deployment-wide compatibility escape hatch.
func IntegrationTLSConfig() *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: strings.EqualFold(os.Getenv("ALLOW_INSECURE_INTEGRATION_TLS"), "true")}
}
