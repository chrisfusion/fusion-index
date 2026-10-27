package integration

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	db "fusion-platform/fusion-index/internal/db/sqlc"
)

// createFileRow inserts a file row directly, bypassing the upload handler, so tests
// can produce the PENDING/ERROR states that only interrupted or failed uploads leave.
func createFileRow(t *testing.T, versionID int64, name, status string) int64 {
	t.Helper()
	ctx := context.Background()
	f, err := testQueries.CreateArtifactFile(ctx, db.CreateArtifactFileParams{
		VersionID:      versionID,
		Name:           name,
		StorageBackend: "FILESYSTEM",
		StoragePath:    "pending",
	})
	if err != nil {
		t.Fatalf("create file row: %v", err)
	}
	if status != "PENDING" {
		if err := testQueries.UpdateArtifactFileStatus(ctx, db.UpdateArtifactFileStatusParams{ID: f.ID, Status: status}); err != nil {
			t.Fatalf("update file status: %v", err)
		}
	}
	return f.ID
}

func incompleteFileIDs(t *testing.T, olderThan time.Time) map[int64]bool {
	t.Helper()
	resp := mustGet(t, "/api/v1/admin/files/incomplete?pageSize=1000&olderThan="+url.QueryEscape(olderThan.Format(time.RFC3339)))
	assertStatus(t, resp, http.StatusOK)
	var page struct {
		Items []struct {
			ID     int64  `json:"id"`
			Status string `json:"status"`
		} `json:"items"`
	}
	mustDecode(t, resp, &page)
	ids := make(map[int64]bool, len(page.Items))
	for _, f := range page.Items {
		if f.Status != "PENDING" && f.Status != "ERROR" {
			t.Errorf("file %d listed with status %s", f.ID, f.Status)
		}
		ids[f.ID] = true
	}
	return ids
}

func TestAdminIncompleteFiles(t *testing.T) {
	artifactID := setupArtifactWithVersion(t, "org.admin.incomplete", "1.0.0")
	v := mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d/versions/1.0.0", artifactID))
	assertStatus(t, v, http.StatusOK)
	var version map[string]any
	mustDecode(t, v, &version)
	versionID := int64(version["id"].(float64))

	resp := uploadFile(t, artifactID, "1.0.0", "ok.bin", "ok")
	assertStatus(t, resp, http.StatusCreated)
	var available map[string]any
	mustDecode(t, resp, &available)
	availableID := int64(available["id"].(float64))

	pendingID := createFileRow(t, versionID, "pending.bin", "PENDING")
	errorID := createFileRow(t, versionID, "error.bin", "ERROR")

	// Leftover object of a failed upload at the path the upload handler would have used.
	leftover := filepath.Join(testArtifactDir, fmt.Sprintf("%d/1/0/0/%d/error.bin", artifactID, errorID))
	if err := os.MkdirAll(filepath.Dir(leftover), 0755); err != nil {
		t.Fatalf("mkdir leftover: %v", err)
	}
	if err := os.WriteFile(leftover, []byte("partial"), 0644); err != nil {
		t.Fatalf("write leftover: %v", err)
	}

	protectedID := setupArtifactWithVersion(t, "org.admin.incomplete-protected", "1.0.0")
	assertStatus(t, mustPut(t, fmt.Sprintf("/api/v1/artifacts/%d/tags/%s", protectedID, testProtectedTag), map[string]any{"version": "1.0.0"}), http.StatusOK)
	pv := mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d/versions/1.0.0", protectedID))
	var protectedVersion map[string]any
	mustDecode(t, pv, &protectedVersion)
	protectedFileID := createFileRow(t, int64(protectedVersion["id"].(float64)), "kept.bin", "ERROR")

	future := time.Now().Add(time.Hour)

	if ids := incompleteFileIDs(t, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)); ids[pendingID] || ids[errorID] {
		t.Error("files newer than olderThan must not be listed")
	}
	ids := incompleteFileIDs(t, future)
	if !ids[pendingID] || !ids[errorID] || !ids[protectedFileID] {
		t.Errorf("expected pending, error and protected files listed, got %v", ids)
	}
	if ids[availableID] {
		t.Error("AVAILABLE file must not be listed")
	}

	resp = mustDelete(t, "/api/v1/admin/files/incomplete?olderThan="+url.QueryEscape(future.Format(time.RFC3339)))
	assertStatus(t, resp, http.StatusOK)
	var result map[string]any
	mustDecode(t, resp, &result)
	if result["deleted"].(float64) < 2 || result["skipped"].(float64) < 1 {
		t.Errorf("expected >=2 deleted and >=1 skipped, got %v", result)
	}

	ids = incompleteFileIDs(t, future)
	if ids[pendingID] || ids[errorID] {
		t.Error("pending/error files still listed after delete")
	}
	if !ids[protectedFileID] {
		t.Error("file of protected artifact must be skipped")
	}
	if _, err := os.Stat(leftover); !os.IsNotExist(err) {
		t.Errorf("leftover storage object not removed: %v", err)
	}
	assertStatus(t, mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d/versions/1.0.0/files/%d", artifactID, availableID)), http.StatusOK)
}

func TestAdminIncompleteFilesRequiresOlderThan(t *testing.T) {
	assertStatus(t, mustGet(t, "/api/v1/admin/files/incomplete"), http.StatusBadRequest)
	assertStatus(t, mustDelete(t, "/api/v1/admin/files/incomplete"), http.StatusBadRequest)
}
