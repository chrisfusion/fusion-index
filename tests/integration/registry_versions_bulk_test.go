package integration

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

type bulkVersionDeleteResult struct {
	Deleted  int64 `json:"deleted"`
	Skipped  int64 `json:"skipped"`
	DryRun   bool  `json:"dryRun"`
	Versions []struct {
		ArtifactID int64  `json:"artifactId"`
		Version    string `json:"version"`
	} `json:"versions"`
}

func bulkDeleteVersions(t *testing.T, path string, wantStatus int) bulkVersionDeleteResult {
	t.Helper()
	resp := mustDelete(t, path)
	assertStatus(t, resp, wantStatus)
	var r bulkVersionDeleteResult
	if wantStatus == http.StatusOK {
		mustDecode(t, resp, &r)
	}
	return r
}

func deletedVersions(r bulkVersionDeleteResult, artifactID int) []string {
	var out []string
	for _, v := range r.Versions {
		if v.ArtifactID == int64(artifactID) {
			out = append(out, v.Version)
		}
	}
	sort.Strings(out)
	return out
}

func remainingVersions(t *testing.T, artifactID int) []string {
	t.Helper()
	resp := mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d/versions", artifactID))
	assertStatus(t, resp, http.StatusOK)
	var versions []struct {
		Version string `json:"version"`
	}
	mustDecode(t, resp, &versions)
	out := make([]string, len(versions))
	for i, v := range versions {
		out[i] = v.Version
	}
	sort.Strings(out)
	return out
}

func assertVersions(t *testing.T, what string, got []string, want ...string) {
	t.Helper()
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("%s: got %v, want %v", what, got, want)
	}
}

func artifactWithVersions(t *testing.T, prefix string, versions ...string) int {
	t.Helper()
	a := createArtifact(t, prefix)
	id := int(a["id"].(float64))
	for _, v := range versions {
		createVersion(t, id, v)
	}
	return id
}

func tagVersion(t *testing.T, artifactID int, tag, version string) {
	t.Helper()
	assertStatus(t, mustPut(t, fmt.Sprintf("/api/v1/artifacts/%d/tags/%s", artifactID, tag), map[string]any{"version": version}), http.StatusOK)
}

func TestBulkDeleteVersionsKeepLast(t *testing.T) {
	id := artifactWithVersions(t, "org.bulk.keeplast", "1.0.0", "1.1.0", "1.2.0", "2.0.0")
	tagVersion(t, id, "latest", "1.0.0")

	resp := uploadFile(t, id, "1.1.0", "old.bin", "old")
	assertStatus(t, resp, http.StatusCreated)
	var f map[string]any
	mustDecode(t, resp, &f)
	stored := filepath.Join(testArtifactDir, fmt.Sprintf("%d/1/1/0/%d/old.bin", id, int64(f["id"].(float64))))
	if _, err := os.Stat(stored); err != nil {
		t.Fatalf("uploaded file not in storage: %v", err)
	}

	path := fmt.Sprintf("/api/v1/artifacts/%d/versions?keepLast=2", id)

	dry := bulkDeleteVersions(t, path+"&dryRun=true", http.StatusOK)
	if !dry.DryRun || dry.Deleted != 1 || dry.Skipped != 1 {
		t.Errorf("dry run: got %+v, want deleted=1 skipped=1 dryRun", dry)
	}
	assertVersions(t, "after dry run", remainingVersions(t, id), "1.0.0", "1.1.0", "1.2.0", "2.0.0")

	r := bulkDeleteVersions(t, path, http.StatusOK)
	if r.DryRun || r.Deleted != 1 || r.Skipped != 1 {
		t.Errorf("delete: got %+v, want deleted=1 skipped=1", r)
	}
	assertVersions(t, "deleted", deletedVersions(r, id), "1.1.0")
	// 2.0.0 and 1.2.0 are the newest two; 1.0.0 is kept by its tag.
	assertVersions(t, "remaining", remainingVersions(t, id), "1.0.0", "1.2.0", "2.0.0")
	if _, err := os.Stat(stored); !os.IsNotExist(err) {
		t.Errorf("file of deleted version still in storage: %v", err)
	}
}

func TestBulkDeleteVersionsRangeAndList(t *testing.T) {
	id := artifactWithVersions(t, "org.bulk.range", "0.9.0", "1.0.0", "1.5.0", "2.0.0", "3.0.0")

	r := bulkDeleteVersions(t, fmt.Sprintf("/api/v1/artifacts/%d/versions?from=1.0.0&below=2.0.0", id), http.StatusOK)
	assertVersions(t, "range deleted", deletedVersions(r, id), "1.0.0", "1.5.0")

	r = bulkDeleteVersions(t, fmt.Sprintf("/api/v1/artifacts/%d/versions?version=0.9.0&version=3.0.0&version=7.7.7", id), http.StatusOK)
	assertVersions(t, "list deleted", deletedVersions(r, id), "0.9.0", "3.0.0")
	assertVersions(t, "remaining", remainingVersions(t, id), "2.0.0")
}

func TestBulkDeleteVersionsOlderThan(t *testing.T) {
	id := artifactWithVersions(t, "org.bulk.older", "1.0.0", "1.1.0")

	r := bulkDeleteVersions(t, fmt.Sprintf("/api/v1/artifacts/%d/versions?olderThan=2000-01-01T00:00:00Z", id), http.StatusOK)
	if r.Deleted != 0 {
		t.Errorf("nothing is older than 2000, got %+v", r)
	}
	// Filters combine with AND: everything is older than 2999, keepLast still saves the newest.
	r = bulkDeleteVersions(t, fmt.Sprintf("/api/v1/artifacts/%d/versions?olderThan=2999-01-01T00:00:00Z&keepLast=1", id), http.StatusOK)
	assertVersions(t, "deleted", deletedVersions(r, id), "1.0.0")
	assertVersions(t, "remaining", remainingVersions(t, id), "1.1.0")
}

func TestBulkDeleteVersionsProtectedArtifact(t *testing.T) {
	id := artifactWithVersions(t, "org.bulk.protected", "1.0.0", "2.0.0")
	tagVersion(t, id, testProtectedTag, "2.0.0")

	r := bulkDeleteVersions(t, fmt.Sprintf("/api/v1/artifacts/%d/versions?below=9.0.0", id), http.StatusOK)
	if r.Deleted != 0 || r.Skipped != 2 {
		t.Errorf("protected artifact: got %+v, want deleted=0 skipped=2", r)
	}
	assertVersions(t, "remaining", remainingVersions(t, id), "1.0.0", "2.0.0")
}

func TestAdminBulkDeleteVersions(t *testing.T) {
	// 900.x versions keep this registry-wide delete away from other tests' data.
	a := artifactWithVersions(t, "org.bulk.admin-a", "900.0.0", "900.1.0")
	b := artifactWithVersions(t, "org.bulk.admin-b", "900.0.0", "900.1.0", "900.2.0")

	r := bulkDeleteVersions(t, "/api/v1/admin/versions?from=900.0.0&keepLast=1", http.StatusOK)
	assertVersions(t, "deleted a", deletedVersions(r, a), "900.0.0")
	assertVersions(t, "deleted b", deletedVersions(r, b), "900.0.0", "900.1.0")
	assertVersions(t, "remaining a", remainingVersions(t, a), "900.1.0")
	assertVersions(t, "remaining b", remainingVersions(t, b), "900.2.0")
}

func TestBulkDeleteVersionsValidation(t *testing.T) {
	id := artifactWithVersions(t, "org.bulk.validation", "1.0.0")
	base := fmt.Sprintf("/api/v1/artifacts/%d/versions", id)

	bulkDeleteVersions(t, base, http.StatusBadRequest)
	bulkDeleteVersions(t, base+"?keepLast=0", http.StatusBadRequest)
	bulkDeleteVersions(t, base+"?below=1.x", http.StatusBadRequest)
	bulkDeleteVersions(t, base+"?olderThan=yesterday", http.StatusBadRequest)
	bulkDeleteVersions(t, "/api/v1/admin/versions", http.StatusBadRequest)
	bulkDeleteVersions(t, "/api/v1/admin/versions?version=1.0.0", http.StatusBadRequest)
	bulkDeleteVersions(t, "/api/v1/artifacts/999999999/versions?keepLast=1", http.StatusNotFound)
	assertVersions(t, "remaining", remainingVersions(t, id), "1.0.0")
}
