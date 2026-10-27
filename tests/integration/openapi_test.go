package integration

import (
	"net/http"
	"testing"
)

// The spec is hand-written YAML converted to JSON at request time; a malformed
// edit only shows up when it is served.
func TestOpenAPISpecServed(t *testing.T) {
	resp := mustGet(t, "/api/openapi.json")
	assertStatus(t, resp, http.StatusOK)
	var spec struct {
		Paths map[string]any `json:"paths"`
	}
	mustDecode(t, resp, &spec)
	for _, p := range []string{"/api/v1/artifacts", "/api/v1/versions", "/api/v1/tags", "/api/v1/retention-policies", "/api/v1/admin/types/unused", "/q/metrics"} {
		if _, ok := spec.Paths[p]; !ok {
			t.Errorf("path %s missing from served spec", p)
		}
	}
}
