package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestCreateAndGetTemplate(t *testing.T) {
	body := `{"name":"test-template-create","description":"A test template","dockerImage":"registry.example.com/spark:3.5","changelog":"Initial"}`
	resp := mustPost(t, "/api/v1/templates", body)
	assertStatus(t, resp, http.StatusCreated)

	var created map[string]any
	mustDecode(t, resp, &created)
	assertFieldEqual(t, created, "name", "test-template-create")
	assertFieldEqual(t, created, "latestVersionNumber", float64(1))

	id := int(created["id"].(float64))
	resp2 := mustGet(t, fmt.Sprintf("/api/v1/templates/%d", id))
	assertStatus(t, resp2, http.StatusOK)

	var got map[string]any
	mustDecode(t, resp2, &got)
	assertFieldEqual(t, got, "id", float64(id))
	assertFieldEqual(t, got, "name", "test-template-create")
}

func TestListTemplates(t *testing.T) {
	resp := mustGet(t, "/api/v1/templates")
	assertStatus(t, resp, http.StatusOK)

	var page map[string]any
	mustDecode(t, resp, &page)
	if page["items"] == nil {
		t.Fatal("expected items field")
	}
}

func TestDuplicateTemplateNameReturns409(t *testing.T) {
	body := `{"name":"duplicate-template","dockerImage":"registry.example.com/test:1.0"}`
	resp1 := mustPost(t, "/api/v1/templates", body)
	assertStatus(t, resp1, http.StatusCreated)

	resp2 := mustPost(t, "/api/v1/templates", body)
	assertStatus(t, resp2, http.StatusConflict)
}

func TestMissingRequiredFieldReturns400(t *testing.T) {
	resp := mustPost(t, "/api/v1/templates", `{"description":"no image or name"}`)
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestTemplateNotFoundReturns404(t *testing.T) {
	resp := mustGet(t, "/api/v1/templates/999999")
	assertStatus(t, resp, http.StatusNotFound)
}

func TestPublishTemplateVersionAndRetrieve(t *testing.T) {
	resp := mustPost(t, "/api/v1/templates", `{"name":"versioned-template","dockerImage":"registry.example.com/base:1.0"}`)
	assertStatus(t, resp, http.StatusCreated)
	var created map[string]any
	mustDecode(t, resp, &created)
	id := int(created["id"].(float64))

	v2resp := mustPost(t, fmt.Sprintf("/api/v1/templates/%d/versions", id),
		`{"dockerImage":"registry.example.com/base:2.0","changelog":"Upgraded base image"}`)
	assertStatus(t, v2resp, http.StatusCreated)
	var v2 map[string]any
	mustDecode(t, v2resp, &v2)
	assertFieldEqual(t, v2, "versionNumber", float64(2))

	listResp := mustGet(t, fmt.Sprintf("/api/v1/templates/%d/versions", id))
	assertStatus(t, listResp, http.StatusOK)
	var versions []any
	mustDecode(t, listResp, &versions)
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}

	getResp := mustGet(t, fmt.Sprintf("/api/v1/templates/%d/versions/2", id))
	assertStatus(t, getResp, http.StatusOK)
	var v map[string]any
	mustDecode(t, getResp, &v)
	assertFieldEqual(t, v, "dockerImage", "registry.example.com/base:2.0")
}

func TestUpdateTemplate(t *testing.T) {
	resp := mustPost(t, "/api/v1/templates", `{"name":"update-me-template","dockerImage":"registry.example.com/base:1.0"}`)
	assertStatus(t, resp, http.StatusCreated)
	var created map[string]any
	mustDecode(t, resp, &created)
	id := int(created["id"].(float64))

	updateResp := mustPut(t, fmt.Sprintf("/api/v1/templates/%d", id), `{"description":"updated description"}`)
	assertStatus(t, updateResp, http.StatusOK)
	var updated map[string]any
	mustDecode(t, updateResp, &updated)
	assertFieldEqual(t, updated, "description", "updated description")
}

func TestDeleteTemplate(t *testing.T) {
	resp := mustPost(t, "/api/v1/templates", `{"name":"delete-me-template","dockerImage":"registry.example.com/base:1.0"}`)
	assertStatus(t, resp, http.StatusCreated)
	var created map[string]any
	mustDecode(t, resp, &created)
	id := int(created["id"].(float64))

	delResp := mustDelete(t, fmt.Sprintf("/api/v1/templates/%d", id))
	assertStatus(t, delResp, http.StatusNoContent)

	getResp := mustGet(t, fmt.Sprintf("/api/v1/templates/%d", id))
	assertStatus(t, getResp, http.StatusNotFound)
}

// ---- HTTP helpers ----

func mustPost(t *testing.T, path, body string) *http.Response {
	t.Helper()
	resp, err := http.Post(testServer.URL+path, "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func mustGet(t *testing.T, path string) *http.Response {
	t.Helper()
	resp, err := http.Get(testServer.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func mustPut(t *testing.T, path, body string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPut, testServer.URL+path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", path, err)
	}
	return resp
}

func mustDelete(t *testing.T, path string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodDelete, testServer.URL+path, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", path, err)
	}
	return resp
}

func mustDecode(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func assertStatus(t *testing.T, resp *http.Response, expected int) {
	t.Helper()
	if resp.StatusCode != expected {
		t.Errorf("expected status %d, got %d", expected, resp.StatusCode)
	}
}

func assertFieldEqual(t *testing.T, m map[string]any, key string, expected any) {
	t.Helper()
	if m[key] != expected {
		t.Errorf("field %q: expected %v (%T), got %v (%T)", key, expected, expected, m[key], m[key])
	}
}
