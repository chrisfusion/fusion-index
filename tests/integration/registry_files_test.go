package integration

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"testing"
)

func uploadFile(t *testing.T, artifactID int, version, filename, content string) *http.Response {
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

	url := fmt.Sprintf("%s/api/v1/artifacts/%d/versions/%s/files", testServer.URL, artifactID, version)
	resp, err := http.Post(url, mw.FormDataContentType(), &buf)
	if err != nil {
		t.Fatalf("upload file: %v", err)
	}
	return resp
}

func setupArtifactWithVersion(t *testing.T, prefix, version string) (artifactID int) {
	t.Helper()
	a := createArtifact(t, prefix)
	artifactID = int(a["id"].(float64))
	createVersion(t, artifactID, version)
	return artifactID
}

func TestUploadFile(t *testing.T) {
	artifactID := setupArtifactWithVersion(t, "org.upfile", "1.0.0")
	resp := uploadFile(t, artifactID, "1.0.0", "hello.bin", "hello content")
	assertStatus(t, resp, http.StatusCreated)

	var f map[string]any
	mustDecode(t, resp, &f)
	assertFieldEqual(t, f, "status", "AVAILABLE")
	assertFieldEqual(t, f, "name", "hello.bin")
}

func TestUploadFileDownloadURL(t *testing.T) {
	artifactID := setupArtifactWithVersion(t, "org.dlurl", "1.0.0")
	resp := uploadFile(t, artifactID, "1.0.0", "test.bin", "content")
	assertStatus(t, resp, http.StatusCreated)

	var f map[string]any
	mustDecode(t, resp, &f)
	if f["downloadUrl"] == nil || f["downloadUrl"] == "" {
		t.Error("expected downloadUrl in file response")
	}
}

func TestDownloadFile(t *testing.T) {
	artifactID := setupArtifactWithVersion(t, "org.download", "1.0.0")
	uploadResp := uploadFile(t, artifactID, "1.0.0", "data.bin", "binary data")
	assertStatus(t, uploadResp, http.StatusCreated)

	var f map[string]any
	mustDecode(t, uploadResp, &f)
	fileID := int(f["id"].(float64))

	dlResp, err := http.Get(testServer.URL + fmt.Sprintf("/api/v1/artifacts/%d/versions/1.0.0/files/%d/download", artifactID, fileID))
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	defer dlResp.Body.Close()
	assertStatus(t, dlResp, http.StatusOK)
	if dlResp.Header.Get("Content-Disposition") == "" {
		t.Error("expected Content-Disposition header")
	}
}

func TestGetFileMetadata(t *testing.T) {
	artifactID := setupArtifactWithVersion(t, "org.getmeta", "1.0.0")
	uploadResp := uploadFile(t, artifactID, "1.0.0", "meta.bin", "meta content")
	assertStatus(t, uploadResp, http.StatusCreated)

	var uploaded map[string]any
	mustDecode(t, uploadResp, &uploaded)
	fileID := int(uploaded["id"].(float64))

	metaResp := mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d/versions/1.0.0/files/%d", artifactID, fileID))
	assertStatus(t, metaResp, http.StatusOK)
	var meta map[string]any
	mustDecode(t, metaResp, &meta)
	assertFieldEqual(t, meta, "name", "meta.bin")
}

func TestListFiles(t *testing.T) {
	artifactID := setupArtifactWithVersion(t, "org.listfiles", "1.0.0")
	uploadFile(t, artifactID, "1.0.0", "file1.bin", "a")
	uploadFile(t, artifactID, "1.0.0", "file2.bin", "b")

	resp := mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d/versions/1.0.0/files", artifactID))
	assertStatus(t, resp, http.StatusOK)
	var files []any
	mustDecode(t, resp, &files)
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
}

func TestDeleteFile(t *testing.T) {
	artifactID := setupArtifactWithVersion(t, "org.delfile", "1.0.0")
	uploadResp := uploadFile(t, artifactID, "1.0.0", "gone.bin", "to be deleted")
	assertStatus(t, uploadResp, http.StatusCreated)

	var f map[string]any
	mustDecode(t, uploadResp, &f)
	fileID := int(f["id"].(float64))

	assertStatus(t, mustDelete(t, fmt.Sprintf("/api/v1/artifacts/%d/versions/1.0.0/files/%d", artifactID, fileID)), http.StatusNoContent)
	assertStatus(t, mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d/versions/1.0.0/files/%d", artifactID, fileID)), http.StatusNotFound)
}

func TestFileNotFound(t *testing.T) {
	artifactID := setupArtifactWithVersion(t, "org.filenotfound", "1.0.0")
	assertStatus(t, mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d/versions/1.0.0/files/999999999", artifactID)), http.StatusNotFound)
}

func TestUploadFileMissingField(t *testing.T) {
	artifactID := setupArtifactWithVersion(t, "org.missingfile", "1.0.0")
	url := fmt.Sprintf("%s/api/v1/artifacts/%d/versions/1.0.0/files", testServer.URL, artifactID)
	resp, err := http.Post(url, "application/json", bytes.NewReader([]byte("{}")))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestUploadFileVersionNotFound(t *testing.T) {
	a := createArtifact(t, "org.fileversionnotfound")
	id := int(a["id"].(float64))
	resp := uploadFile(t, id, "9.9.9", "nope.bin", "content")
	assertStatus(t, resp, http.StatusNotFound)
}

func TestUploadFileDuplicateName(t *testing.T) {
	artifactID := setupArtifactWithVersion(t, "org.dupfile", "1.0.0")
	uploadFile(t, artifactID, "1.0.0", "same.bin", "first upload")
	resp := uploadFile(t, artifactID, "1.0.0", "same.bin", "second upload")
	assertStatus(t, resp, http.StatusConflict)
}

func TestMultipleFilesPerVersion(t *testing.T) {
	artifactID := setupArtifactWithVersion(t, "org.multifile", "1.0.0")
	uploadFile(t, artifactID, "1.0.0", "wheel.whl", "wheel content")
	uploadFile(t, artifactID, "1.0.0", "source.tar.gz", "tarball content")

	resp := mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d/versions/1.0.0/files", artifactID))
	assertStatus(t, resp, http.StatusOK)
	var files []any
	mustDecode(t, resp, &files)
	if len(files) != 2 {
		t.Fatalf("expected 2 files for version, got %d", len(files))
	}
}
