package integration

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func createVersion(t *testing.T, artifactID int, version string) map[string]any {
	t.Helper()
	resp := mustPost(t, fmt.Sprintf("/api/v1/artifacts/%d/versions", artifactID), map[string]any{
		"version": version,
	})
	assertStatus(t, resp, http.StatusCreated)
	var v map[string]any
	mustDecode(t, resp, &v)
	return v
}

func TestCreateVersion(t *testing.T) {
	a := createArtifact(t, "org.ver")
	id := int(a["id"].(float64))
	v := createVersion(t, id, "1.0.0")
	assertFieldEqual(t, v, "version", "1.0.0")
	assertFieldEqual(t, v, "major", float64(1))
	assertFieldEqual(t, v, "minor", float64(0))
	assertFieldEqual(t, v, "patch", float64(0))
}

func TestCreateVersionWithConfig(t *testing.T) {
	a := createArtifact(t, "org.cfgver")
	id := int(a["id"].(float64))
	config := `{"key":"value"}`
	resp := mustPost(t, fmt.Sprintf("/api/v1/artifacts/%d/versions", id), map[string]any{
		"version": "2.0.0",
		"config":  config,
	})
	assertStatus(t, resp, http.StatusCreated)
	var v map[string]any
	mustDecode(t, resp, &v)
	assertFieldEqual(t, v, "config", config)
}

func TestCreateVersionWithTags(t *testing.T) {
	a := createArtifact(t, "org.tagver")
	id := int(a["id"].(float64))
	resp := mustPost(t, fmt.Sprintf("/api/v1/artifacts/%d/versions", id), map[string]any{
		"version": "1.0.0",
		"tags":    []string{"latest", "stable"},
	})
	assertStatus(t, resp, http.StatusCreated)
	var v map[string]any
	mustDecode(t, resp, &v)
	tags := v["tags"].([]any)
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(tags))
	}
}

func TestCreateVersionDuplicate(t *testing.T) {
	a := createArtifact(t, "org.dupver")
	id := int(a["id"].(float64))
	createVersion(t, id, "1.0.0")
	resp := mustPost(t, fmt.Sprintf("/api/v1/artifacts/%d/versions", id), map[string]any{
		"version": "1.0.0",
	})
	assertStatus(t, resp, http.StatusConflict)
}

func TestCreateVersionInvalidSemver(t *testing.T) {
	a := createArtifact(t, "org.badsemver")
	id := int(a["id"].(float64))
	resp := mustPost(t, fmt.Sprintf("/api/v1/artifacts/%d/versions", id), map[string]any{
		"version": "not-a-version",
	})
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestCreateVersionArtifactNotFound(t *testing.T) {
	resp := mustPost(t, "/api/v1/artifacts/999999999/versions", map[string]any{
		"version": "1.0.0",
	})
	assertStatus(t, resp, http.StatusNotFound)
}

func TestGetVersion(t *testing.T) {
	a := createArtifact(t, "org.getver")
	id := int(a["id"].(float64))
	createVersion(t, id, "3.1.4")
	resp := mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d/versions/3.1.4", id))
	assertStatus(t, resp, http.StatusOK)
	var v map[string]any
	mustDecode(t, resp, &v)
	assertFieldEqual(t, v, "version", "3.1.4")
}

func TestGetVersionNotFound(t *testing.T) {
	a := createArtifact(t, "org.getvernotfound")
	id := int(a["id"].(float64))
	assertStatus(t, mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d/versions/9.9.9", id)), http.StatusNotFound)
}

func TestListVersions(t *testing.T) {
	a := createArtifact(t, "org.listver")
	id := int(a["id"].(float64))
	createVersion(t, id, "1.0.0")
	createVersion(t, id, "1.1.0")
	createVersion(t, id, "2.0.0")

	resp := mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d/versions", id))
	assertStatus(t, resp, http.StatusOK)
	var versions []any
	mustDecode(t, resp, &versions)
	if len(versions) != 3 {
		t.Fatalf("expected 3 versions, got %d", len(versions))
	}
}

func TestDeleteVersion(t *testing.T) {
	a := createArtifact(t, "org.delver")
	id := int(a["id"].(float64))
	createVersion(t, id, "1.0.0")

	assertStatus(t, mustDelete(t, fmt.Sprintf("/api/v1/artifacts/%d/versions/1.0.0", id)), http.StatusNoContent)
	assertStatus(t, mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d/versions/1.0.0", id)), http.StatusNotFound)
}

func TestDeleteArtifactCascadesVersions(t *testing.T) {
	a := createArtifact(t, "org.cascdel")
	id := int(a["id"].(float64))
	createVersion(t, id, "1.0.0")

	assertStatus(t, mustDelete(t, fmt.Sprintf("/api/v1/artifacts/%d", id)), http.StatusNoContent)
	// After artifact is deleted, version endpoint returns 404
	assertStatus(t, mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d/versions/1.0.0", id)), http.StatusNotFound)
}

func TestVersionTagsMovedOnReassign(t *testing.T) {
	a := createArtifact(t, "org.movetag")
	id := int(a["id"].(float64))
	createVersion(t, id, "1.0.0")
	createVersion(t, id, "2.0.0")

	// Assign "latest" to v1
	tagResp := mustPut(t, fmt.Sprintf("/api/v1/artifacts/%d/tags/latest", id), map[string]any{
		"version": "1.0.0",
	})
	assertStatus(t, tagResp, http.StatusOK)

	// Reassign "latest" to v2 — should move
	tagResp2 := mustPut(t, fmt.Sprintf("/api/v1/artifacts/%d/tags/latest", id), map[string]any{
		"version": "2.0.0",
	})
	assertStatus(t, tagResp2, http.StatusOK)
	var t2 map[string]any
	mustDecode(t, tagResp2, &t2)

	v2Resp := mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d/versions/2.0.0", id))
	var v2 map[string]any
	mustDecode(t, v2Resp, &v2)
	v2Tags := v2["tags"].([]any)
	if len(v2Tags) == 0 {
		t.Fatal("expected 'latest' tag to appear on v2 after reassign")
	}

	// Confirm v1 no longer holds "latest"
	v1Resp := mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d/versions/1.0.0", id))
	var v1 map[string]any
	mustDecode(t, v1Resp, &v1)
	v1Tags := v1["tags"].([]any)
	if len(v1Tags) != 0 {
		t.Fatal("expected v1 to have no tags after 'latest' was moved")
	}
}

func TestMultipleTagsPerVersion(t *testing.T) {
	a := createArtifact(t, "org.multitag")
	id := int(a["id"].(float64))

	suffix := time.Now().UnixNano()
	tag1 := fmt.Sprintf("t1-%d", suffix)
	tag2 := fmt.Sprintf("t2-%d", suffix)

	assertStatus(t, mustPut(t, fmt.Sprintf("/api/v1/artifacts/%d/tags/%s", id, tag1), map[string]any{"version": "1.0.0"}), http.StatusNotFound)

	createVersion(t, id, "1.0.0")
	assertStatus(t, mustPut(t, fmt.Sprintf("/api/v1/artifacts/%d/tags/%s", id, tag1), map[string]any{"version": "1.0.0"}), http.StatusOK)
	assertStatus(t, mustPut(t, fmt.Sprintf("/api/v1/artifacts/%d/tags/%s", id, tag2), map[string]any{"version": "1.0.0"}), http.StatusOK)

	resp := mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d/versions/1.0.0", id))
	var v map[string]any
	mustDecode(t, resp, &v)
	tags := v["tags"].([]any)
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags on version, got %d", len(tags))
	}
}
