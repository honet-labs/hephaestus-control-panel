package services

import (
	"os"
	"testing"
)

func TestVendorDataPrepperFixtureMatchesHCPValidator(t *testing.T) {
	content, err := os.ReadFile("../../tests/vendor/dataprepper-pipelines/audit.yml")
	if err != nil {
		t.Fatal(err)
	}
	valid, message, line := NewDataPrepperService(nil).ValidateYAML(string(content))
	if !valid || line != 0 {
		t.Fatalf("vendor pipeline fixture invalid for HCP: %s (line %d)", message, line)
	}
}
