package integration

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func createArtifact(t *testing.T, prefix string) map[string]any {
	t.Helper()
	name := fmt.Sprintf("%s.artifact-%d", prefix, time.Now().UnixNano())
	body := map[string]any{"fullName": name}
	resp := mustPost(t, "/api/v1/artifacts", body)
	assertStatus(t, resp, http.StatusCreated)
	var created map[string]any
	mustDecode(t, resp, &created)
	return created
}

func TestCreateArtifact(t *testing.T) {
	a := createArtifact(t, "org.test")
	if a["id"] == nil {
		t.Fatal("expected id in response")
	}
	if a["fullName"] == nil {
		t.Fatal("expected fullName in response")
	}
}

func TestCreateArtifactWithDescription(t *testing.T) {
	name := fmt.Sprintf("org.test.described-%d", time.Now().UnixNano())
	desc := "a test artifact"
	resp := mustPost(t, "/api/v1/artifacts", map[string]any{
		"fullName":    name,
		"description": desc,
	})
	assertStatus(t, resp, http.StatusCreated)
	var a map[string]any
	mustDecode(t, resp, &a)
	assertFieldEqual(t, a, "description", desc)
}

func TestCreateArtifactDuplicateName(t *testing.T) {
	a := createArtifact(t, "org.dup")
	resp := mustPost(t, "/api/v1/artifacts", map[string]any{"fullName": a["fullName"]})
	assertStatus(t, resp, http.StatusConflict)
}

func TestCreateArtifactMissingName(t *testing.T) {
	resp := mustPost(t, "/api/v1/artifacts", map[string]any{})
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestGetArtifact(t *testing.T) {
	a := createArtifact(t, "org.get")
	id := int(a["id"].(float64))
	resp := mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d", id))
	assertStatus(t, resp, http.StatusOK)
	var got map[string]any
	mustDecode(t, resp, &got)
	assertFieldEqual(t, got, "fullName", a["fullName"])
}

func TestGetArtifactNotFound(t *testing.T) {
	assertStatus(t, mustGet(t, "/api/v1/artifacts/999999999"), http.StatusNotFound)
}

func TestUpdateArtifact(t *testing.T) {
	a := createArtifact(t, "org.upd")
	id := int(a["id"].(float64))
	resp := mustPut(t, fmt.Sprintf("/api/v1/artifacts/%d", id), map[string]any{
		"description": "updated description",
	})
	assertStatus(t, resp, http.StatusOK)
	var updated map[string]any
	mustDecode(t, resp, &updated)
	assertFieldEqual(t, updated, "description", "updated description")
}

func TestDeleteArtifact(t *testing.T) {
	a := createArtifact(t, "org.del")
	id := int(a["id"].(float64))
	assertStatus(t, mustDelete(t, fmt.Sprintf("/api/v1/artifacts/%d", id)), http.StatusNoContent)
	assertStatus(t, mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d", id)), http.StatusNotFound)
}

func TestListArtifacts(t *testing.T) {
	resp := mustGet(t, "/api/v1/artifacts")
	assertStatus(t, resp, http.StatusOK)
	var page map[string]any
	mustDecode(t, resp, &page)
	if page["items"] == nil {
		t.Fatal("expected items field in response")
	}
}

func TestListArtifactsFilterByName(t *testing.T) {
	prefix := fmt.Sprintf("filtertest-%d", time.Now().UnixNano())
	createArtifact(t, prefix)

	resp := mustGet(t, fmt.Sprintf("/api/v1/artifacts?name=%s", prefix))
	assertStatus(t, resp, http.StatusOK)
	var page map[string]any
	mustDecode(t, resp, &page)
	items := page["items"].([]any)
	if len(items) == 0 {
		t.Fatal("expected at least one result for name filter")
	}
}

func TestListArtifactsFilterByTag(t *testing.T) {
	a := createArtifact(t, "org.tagfilter")
	artifactID := int(a["id"].(float64))
	tagName := fmt.Sprintf("tag-%d", time.Now().UnixNano())

	// Create a version and assign the tag.
	vResp := mustPost(t, fmt.Sprintf("/api/v1/artifacts/%d/versions", artifactID), map[string]any{
		"version": "1.0.0",
		"tags":    []string{tagName},
	})
	assertStatus(t, vResp, http.StatusCreated)

	resp := mustGet(t, fmt.Sprintf("/api/v1/artifacts?tag=%s", tagName))
	assertStatus(t, resp, http.StatusOK)
	var page map[string]any
	mustDecode(t, resp, &page)
	items := page["items"].([]any)
	if len(items) == 0 {
		t.Fatal("expected artifact to appear in tag-filtered list")
	}
}
