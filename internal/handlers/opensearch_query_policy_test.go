package handlers

import (
	"net/http"
	"testing"
)

func TestReadOnlyOpenSearchQuery(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		allowed      bool
	}{
		{http.MethodGet, "/_cluster/health", true},
		{http.MethodHead, "/index", true},
		{http.MethodPost, "/index/_search", true},
		{http.MethodPost, "/_msearch", true},
		{http.MethodPost, "/index/_count", true},
		{http.MethodPost, "/_bulk", false},
		{http.MethodPost, "/index/_delete_by_query", false},
		{http.MethodPut, "/index/_doc/1", false},
		{http.MethodDelete, "/index", false},
		{http.MethodPost, "/_cluster/reroute", false},
	} {
		if got := readOnlyOpenSearchQuery(tc.method, tc.path); got != tc.allowed {
			t.Errorf("%s %s: got %v", tc.method, tc.path, got)
		}
	}
}
