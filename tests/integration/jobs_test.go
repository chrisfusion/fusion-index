package integration

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// createTemplateForTest creates a template and returns its first version's ID.
func createTemplateForTest(t *testing.T, suffix string) (templateID int, templateVersionID int) {
	t.Helper()
	name := fmt.Sprintf("job-test-template-%s-%d", suffix, time.Now().UnixNano())
	resp := mustPost(t, "/api/v1/templates", fmt.Sprintf(
		`{"name":%q,"dockerImage":"registry.example.com/base:1.0"}`, name))
	assertStatus(t, resp, http.StatusCreated)

	var created map[string]any
	mustDecode(t, resp, &created)
	templateID = int(created["id"].(float64))

	vResp := mustGet(t, fmt.Sprintf("/api/v1/templates/%d/versions/1", templateID))
	assertStatus(t, vResp, http.StatusOK)
	var v map[string]any
	mustDecode(t, vResp, &v)
	templateVersionID = int(v["id"].(float64))
	return
}

func createJobBody(name string, tvID int) string {
	return fmt.Sprintf(`{
		"name":%q,
		"templateVersionId":%d,
		"dockerImage":"registry.example.com/etl:1.0",
		"gitUrl":"https://github.com/org/repo.git",
		"gitRef":"main"
	}`, name, tvID)
}

func TestCreateAndGetJob(t *testing.T) {
	_, tvID := createTemplateForTest(t, "create")
	name := fmt.Sprintf("create-job-%d", time.Now().UnixNano())

	resp := mustPost(t, "/api/v1/jobs", createJobBody(name, tvID))
	assertStatus(t, resp, http.StatusCreated)

	var created map[string]any
	mustDecode(t, resp, &created)
	assertFieldEqual(t, created, "name", name)
	assertFieldEqual(t, created, "latestVersionNumber", float64(1))

	id := int(created["id"].(float64))
	getResp := mustGet(t, fmt.Sprintf("/api/v1/jobs/%d", id))
	assertStatus(t, getResp, http.StatusOK)
	var got map[string]any
	mustDecode(t, getResp, &got)
	assertFieldEqual(t, got, "id", float64(id))
}

func TestDuplicateJobNameReturns409(t *testing.T) {
	_, tvID := createTemplateForTest(t, "dup")
	name := fmt.Sprintf("dup-job-%d", time.Now().UnixNano())
	body := createJobBody(name, tvID)

	assertStatus(t, mustPost(t, "/api/v1/jobs", body), http.StatusCreated)
	assertStatus(t, mustPost(t, "/api/v1/jobs", body), http.StatusConflict)
}

func TestJobNotFoundReturns404(t *testing.T) {
	assertStatus(t, mustGet(t, "/api/v1/jobs/999999"), http.StatusNotFound)
}

func TestListJobs(t *testing.T) {
	resp := mustGet(t, "/api/v1/jobs")
	assertStatus(t, resp, http.StatusOK)
	var page map[string]any
	mustDecode(t, resp, &page)
	if page["items"] == nil {
		t.Fatal("expected items field")
	}
}

func TestUpdateJob(t *testing.T) {
	_, tvID := createTemplateForTest(t, "upd")
	name := fmt.Sprintf("update-job-%d", time.Now().UnixNano())
	resp := mustPost(t, "/api/v1/jobs", createJobBody(name, tvID))
	assertStatus(t, resp, http.StatusCreated)
	var created map[string]any
	mustDecode(t, resp, &created)
	id := int(created["id"].(float64))

	updateResp := mustPut(t, fmt.Sprintf("/api/v1/jobs/%d", id), `{"description":"new description"}`)
	assertStatus(t, updateResp, http.StatusOK)
	var updated map[string]any
	mustDecode(t, updateResp, &updated)
	assertFieldEqual(t, updated, "description", "new description")
}

func TestDeleteJob(t *testing.T) {
	_, tvID := createTemplateForTest(t, "del")
	name := fmt.Sprintf("delete-job-%d", time.Now().UnixNano())
	resp := mustPost(t, "/api/v1/jobs", createJobBody(name, tvID))
	assertStatus(t, resp, http.StatusCreated)
	var created map[string]any
	mustDecode(t, resp, &created)
	id := int(created["id"].(float64))

	assertStatus(t, mustDelete(t, fmt.Sprintf("/api/v1/jobs/%d", id)), http.StatusNoContent)
	assertStatus(t, mustGet(t, fmt.Sprintf("/api/v1/jobs/%d", id)), http.StatusNotFound)
}

func TestPublishJobVersionAndRetrieve(t *testing.T) {
	_, tvID := createTemplateForTest(t, "ver")
	name := fmt.Sprintf("versioned-job-%d", time.Now().UnixNano())
	resp := mustPost(t, "/api/v1/jobs", createJobBody(name, tvID))
	assertStatus(t, resp, http.StatusCreated)
	var created map[string]any
	mustDecode(t, resp, &created)
	id := int(created["id"].(float64))

	v2Resp := mustPost(t, fmt.Sprintf("/api/v1/jobs/%d/versions", id), fmt.Sprintf(`{
		"templateVersionId":%d,
		"dockerImage":"registry.example.com/etl:2.0",
		"gitUrl":"https://github.com/org/repo.git",
		"gitRef":"v2"
	}`, tvID))
	assertStatus(t, v2Resp, http.StatusCreated)
	var v2 map[string]any
	mustDecode(t, v2Resp, &v2)
	assertFieldEqual(t, v2, "versionNumber", float64(2))

	listResp := mustGet(t, fmt.Sprintf("/api/v1/jobs/%d/versions", id))
	assertStatus(t, listResp, http.StatusOK)
	var versions []any
	mustDecode(t, listResp, &versions)
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}

	getResp := mustGet(t, fmt.Sprintf("/api/v1/jobs/%d/versions/2", id))
	assertStatus(t, getResp, http.StatusOK)
	var v map[string]any
	mustDecode(t, getResp, &v)
	assertFieldEqual(t, v, "dockerImage", "registry.example.com/etl:2.0")
}
