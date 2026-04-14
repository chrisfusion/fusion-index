package integration

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"testing"
	"time"
)

func createJobForArtifactTest(t *testing.T) (jobID int, versionNumber int) {
	t.Helper()
	_, tvID := createTemplateForTest(t, "art")
	name := fmt.Sprintf("artifact-job-%d", time.Now().UnixNano())
	resp := mustPost(t, "/api/v1/jobs", createJobBody(name, tvID))
	assertStatus(t, resp, http.StatusCreated)
	var created map[string]any
	mustDecode(t, resp, &created)
	return int(created["id"].(float64)), 1
}

func uploadArtifact(t *testing.T, jobID, versionNumber int, filename, content string) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write([]byte(content)); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	mw.Close()

	url := fmt.Sprintf("%s/api/v1/jobs/%d/versions/%d/artifacts", testServer.URL, jobID, versionNumber)
	resp, err := http.Post(url, mw.FormDataContentType(), &buf)
	if err != nil {
		t.Fatalf("upload artifact: %v", err)
	}
	return resp
}

func TestUploadAndDownloadArtifact(t *testing.T) {
	jobID, vn := createJobForArtifactTest(t)

	uploadResp := uploadArtifact(t, jobID, vn, "hello.bin", "hello artifact content")
	assertStatus(t, uploadResp, http.StatusCreated)

	var artifact map[string]any
	mustDecode(t, uploadResp, &artifact)
	assertFieldEqual(t, artifact, "status", "AVAILABLE")
	assertFieldEqual(t, artifact, "name", "hello.bin")

	id := int(artifact["id"].(float64))
	dlResp, err := http.Get(testServer.URL + fmt.Sprintf("/api/v1/artifacts/%d/download", id))
	if err != nil {
		t.Fatalf("download artifact: %v", err)
	}
	defer dlResp.Body.Close()
	assertStatus(t, dlResp, http.StatusOK)
	if dlResp.Header.Get("Content-Disposition") == "" {
		t.Error("expected Content-Disposition header")
	}
}

func TestListArtifactsForJobVersion(t *testing.T) {
	jobID, vn := createJobForArtifactTest(t)

	resp := mustGet(t, fmt.Sprintf("/api/v1/jobs/%d/versions/%d/artifacts", jobID, vn))
	assertStatus(t, resp, http.StatusOK)
	var list []any
	mustDecode(t, resp, &list)
	if list == nil {
		t.Fatal("expected array response")
	}
}

func TestListAllArtifactsPaginated(t *testing.T) {
	resp := mustGet(t, "/api/v1/artifacts")
	assertStatus(t, resp, http.StatusOK)
	var page map[string]any
	mustDecode(t, resp, &page)
	if page["items"] == nil {
		t.Fatal("expected items field")
	}
}

func TestArtifactNotFoundReturns404(t *testing.T) {
	assertStatus(t, mustGet(t, "/api/v1/artifacts/999999"), http.StatusNotFound)
}

func TestDeleteArtifact(t *testing.T) {
	jobID, vn := createJobForArtifactTest(t)

	uploadResp := uploadArtifact(t, jobID, vn, "delete-me.bin", "to be deleted")
	assertStatus(t, uploadResp, http.StatusCreated)
	var artifact map[string]any
	mustDecode(t, uploadResp, &artifact)
	id := int(artifact["id"].(float64))

	assertStatus(t, mustDelete(t, fmt.Sprintf("/api/v1/artifacts/%d", id)), http.StatusNoContent)
	assertStatus(t, mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d", id)), http.StatusNotFound)
}

func TestArtifactDownloadURLInResponse(t *testing.T) {
	jobID, vn := createJobForArtifactTest(t)

	uploadResp := uploadArtifact(t, jobID, vn, "url-test.bin", "content")
	assertStatus(t, uploadResp, http.StatusCreated)
	var artifact map[string]any
	mustDecode(t, uploadResp, &artifact)

	if artifact["downloadUrl"] == nil || artifact["downloadUrl"] == "" {
		t.Error("expected downloadUrl in artifact response")
	}
}
