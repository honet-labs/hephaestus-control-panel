package middleware

import (
	"net/url"
	"strings"
	"testing"
)

func TestQuerySecretsAreRedacted(t *testing.T) {
	raw := "token=credential1&TOKEN=credential2&password=credential3&api_key=credential4&clientSecret=credential5&configId=visible"
	redacted := redactQuery(raw)
	if strings.Contains(redacted, "credential") {
		t.Fatal("secret remained in query log")
	}
	query, err := url.ParseQuery(redacted)
	if err != nil || query.Get("configId") != "visible" {
		t.Fatalf("non-secret query lost: %q", redacted)
	}
	if got := redactQuery("token=%zz"); got != "[invalid query]" {
		t.Fatalf("invalid query not suppressed: %q", got)
	}
}
