package integration

import (
	"fmt"
	"net/http"
	"testing"
)

func TestAssignTag(t *testing.T) {
	a := createArtifact(t, "org.assigntag")
	id := int(a["id"].(float64))
	createVersion(t, id, "1.0.0")

	resp := mustPut(t, fmt.Sprintf("/api/v1/artifacts/%d/tags/latest", id), map[string]any{
		"version": "1.0.0",
	})
	assertStatus(t, resp, http.StatusOK)
	var tag map[string]any
	mustDecode(t, resp, &tag)
	assertFieldEqual(t, tag, "tag", "latest")
}

func TestAssignTagVersionNotFound(t *testing.T) {
	a := createArtifact(t, "org.tagnoversion")
	id := int(a["id"].(float64))
	resp := mustPut(t, fmt.Sprintf("/api/v1/artifacts/%d/tags/latest", id), map[string]any{
		"version": "9.9.9",
	})
	assertStatus(t, resp, http.StatusNotFound)
}

func TestAssignTagArtifactNotFound(t *testing.T) {
	resp := mustPut(t, "/api/v1/artifacts/999999999/tags/latest", map[string]any{
		"version": "1.0.0",
	})
	assertStatus(t, resp, http.StatusNotFound)
}

func TestAssignTagInvalidSemver(t *testing.T) {
	a := createArtifact(t, "org.taginvalidsemver")
	id := int(a["id"].(float64))
	resp := mustPut(t, fmt.Sprintf("/api/v1/artifacts/%d/tags/latest", id), map[string]any{
		"version": "bad",
	})
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestDeleteTag(t *testing.T) {
	a := createArtifact(t, "org.deltag")
	id := int(a["id"].(float64))
	createVersion(t, id, "1.0.0")

	mustPut(t, fmt.Sprintf("/api/v1/artifacts/%d/tags/stable", id), map[string]any{"version": "1.0.0"})
	assertStatus(t, mustDelete(t, fmt.Sprintf("/api/v1/artifacts/%d/tags/stable", id)), http.StatusNoContent)
}

func TestDeleteTagNotFound(t *testing.T) {
	a := createArtifact(t, "org.deltagnotfound")
	id := int(a["id"].(float64))
	assertStatus(t, mustDelete(t, fmt.Sprintf("/api/v1/artifacts/%d/tags/nonexistent", id)), http.StatusNotFound)
}

func TestTagMovesOnReassign(t *testing.T) {
	a := createArtifact(t, "org.tagmove")
	id := int(a["id"].(float64))
	createVersion(t, id, "1.0.0")
	createVersion(t, id, "2.0.0")

	assertStatus(t, mustPut(t, fmt.Sprintf("/api/v1/artifacts/%d/tags/current", id), map[string]any{"version": "1.0.0"}), http.StatusOK)

	// Reassign to v2
	resp := mustPut(t, fmt.Sprintf("/api/v1/artifacts/%d/tags/current", id), map[string]any{"version": "2.0.0"})
	assertStatus(t, resp, http.StatusOK)
	var tag map[string]any
	mustDecode(t, resp, &tag)
	assertFieldEqual(t, tag, "tag", "current")

	// v1 must not carry the tag anymore
	v1Resp := mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d/versions/1.0.0", id))
	var v1 map[string]any
	mustDecode(t, v1Resp, &v1)
	if len(v1["tags"].([]any)) != 0 {
		t.Fatal("expected tag to have moved away from v1")
	}
}
