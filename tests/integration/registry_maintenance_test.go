package integration

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestArtifactDeleteRemovesStorage(t *testing.T) {
	id := artifactWithVersions(t, "org.maint.delete", "1.0.0", "2.0.0")
	f1 := uploadOK(t, id, "1.0.0", "a.bin", 10)
	f2 := uploadOK(t, id, "2.0.0", "b.bin", 10)
	paths := []string{
		filepath.Join(testArtifactDir, fmt.Sprintf("%d/1/0/0/%d/a.bin", id, f1)),
		filepath.Join(testArtifactDir, fmt.Sprintf("%d/2/0/0/%d/b.bin", id, f2)),
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("uploaded file missing: %v", err)
		}
	}

	assertStatus(t, mustDelete(t, fmt.Sprintf("/api/v1/artifacts/%d", id)), http.StatusNoContent)
	for _, p := range paths {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("file of deleted artifact still in storage: %s (%v)", p, err)
		}
	}
	assertStatus(t, mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d", id)), http.StatusNotFound)
}

func typeNames(t *testing.T, path string) map[string]bool {
	t.Helper()
	resp := mustGet(t, path)
	assertStatus(t, resp, http.StatusOK)
	var types []struct {
		Name string `json:"name"`
	}
	mustDecode(t, resp, &types)
	names := make(map[string]bool, len(types))
	for _, ty := range types {
		names[ty.Name] = true
	}
	return names
}

func TestUnusedTypes(t *testing.T) {
	usedName, unusedName := uniqueTypeName()+"-used", uniqueTypeName()+"-unused"
	used := createType(t, usedName)
	createType(t, unusedName)
	id := artifactWithVersions(t, "org.maint.types")
	assertStatus(t, mustPut(t, fmt.Sprintf("/api/v1/artifacts/%d/types/%d", id, int(used["id"].(float64))), nil), http.StatusOK)

	unused := typeNames(t, "/api/v1/types?unused=true")
	if !unused[unusedName] || unused[usedName] {
		t.Errorf("unused=true: got %v", unused)
	}
	if inUse := typeNames(t, "/api/v1/types?unused=false"); !inUse[usedName] || inUse[unusedName] {
		t.Errorf("unused=false: got %v", inUse)
	}

	future := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	resp := mustGet(t, "/api/v1/admin/types/unused?olderThan="+future)
	assertStatus(t, resp, http.StatusOK)
	var page struct {
		Total int64 `json:"total"`
	}
	mustDecode(t, resp, &page)
	if page.Total < 1 {
		t.Errorf("admin list: total=%d", page.Total)
	}
	resp = mustDelete(t, "/api/v1/admin/types/unused?olderThan="+future)
	assertStatus(t, resp, http.StatusOK)

	all := typeNames(t, "/api/v1/types")
	if all[unusedName] || !all[usedName] {
		t.Errorf("after cleanup: unused gone=%v, used kept=%v", !all[unusedName], all[usedName])
	}
	assertStatus(t, mustDelete(t, "/api/v1/admin/types/unused"), http.StatusBadRequest)
}

func TestTagHygiene(t *testing.T) {
	id := artifactWithVersions(t, "org.maint.tags", "1.0.0", "2.0.0")
	uploadOK(t, id, "2.0.0", "a.bin", 10)
	tagVersion(t, id, "latest", "2.0.0")
	tagVersion(t, id, "old", "1.0.0") // version without files → broken
	noLatest := artifactWithVersions(t, "org.maint.tags", "1.0.0")

	tags := func(query string) map[string]bool {
		t.Helper()
		resp := mustGet(t, "/api/v1/tags?pageSize=1000&"+query)
		assertStatus(t, resp, http.StatusOK)
		var page struct {
			Items []struct {
				ArtifactID int64  `json:"artifactId"`
				Tag        string `json:"tag"`
				Version    string `json:"version"`
				Broken     bool   `json:"broken"`
			} `json:"items"`
		}
		mustDecode(t, resp, &page)
		out := make(map[string]bool)
		for _, it := range page.Items {
			if it.ArtifactID == int64(id) {
				out[fmt.Sprintf("%s@%s broken=%v", it.Tag, it.Version, it.Broken)] = true
			}
		}
		return out
	}
	if got := tags("broken=true"); len(got) != 1 || !got["old@1.0.0 broken=true"] {
		t.Errorf("broken=true: got %v", got)
	}
	if got := tags("tag=latest"); len(got) != 1 || !got["latest@2.0.0 broken=false"] {
		t.Errorf("tag=latest: got %v", got)
	}

	assertIDs(t, "missingTag=latest", listArtifactIDs(t, "name=org.maint.tags&missingTag=latest"), noLatest)
	assertStatus(t, mustGet(t, "/api/v1/tags?broken=perhaps"), http.StatusBadRequest)
}

func TestSizeFiltersAndTotalSort(t *testing.T) {
	// totals: a=300 (largest version 200), b=250 (largest 250), c=0
	a := artifactWithVersions(t, "org.maint.size", "1.0.0", "2.0.0")
	uploadOK(t, a, "1.0.0", "x.bin", 100)
	uploadOK(t, a, "2.0.0", "x.bin", 200)
	b := artifactWithVersions(t, "org.maint.size", "1.0.0")
	uploadOK(t, b, "1.0.0", "x.bin", 250)
	c := artifactWithVersions(t, "org.maint.size", "1.0.0")

	if got := getArtifact(t, a); got.LargestVersionBytes != 200 {
		t.Errorf("largest a=%d", got.LargestVersionBytes)
	}
	resp := mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d", a))
	var full map[string]any
	mustDecode(t, resp, &full)
	if full["totalBytes"].(float64) != 300 {
		t.Errorf("totalBytes a=%v", full["totalBytes"])
	}

	assertIDs(t, "totalSize desc", listArtifactIDs(t, "name=org.maint.size&sort=totalSize"), a, b, c)
	assertIDs(t, "largestVersion desc", listArtifactIDs(t, "name=org.maint.size&sort=largestVersion"), b, a, c)
	assertIDs(t, "minTotalBytes=260", listArtifactIDs(t, "name=org.maint.size&minTotalBytes=260"), a)

	resp = mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d/versions?minBytes=150", a))
	var vs []struct {
		Version string `json:"version"`
	}
	mustDecode(t, resp, &vs)
	if len(vs) != 1 || vs[0].Version != "2.0.0" {
		t.Errorf("minBytes=150: got %v", vs)
	}
	r := bulkDeleteVersions(t, fmt.Sprintf("/api/v1/artifacts/%d/versions?maxBytes=150&dryRun=true", a), http.StatusOK)
	assertVersions(t, "maxBytes dry run", deletedVersions(r, a), "1.0.0")
	assertStatus(t, mustGet(t, "/api/v1/artifacts?minTotalBytes=-5"), http.StatusBadRequest)
}

func TestRetentionPolicies(t *testing.T) {
	id := artifactWithVersions(t, "org.maint.retention", "1.0.0")
	t1 := int(createType(t, uniqueTypeName()+"-ret1")["id"].(float64))
	t2 := int(createType(t, uniqueTypeName()+"-ret2")["id"].(float64))
	for _, ty := range []int{t1, t2} {
		assertStatus(t, mustPut(t, fmt.Sprintf("/api/v1/artifacts/%d/types/%d", id, ty), nil), http.StatusOK)
	}

	type effective struct {
		Source string `json:"source"`
		Rule   *struct {
			KeepLast   *int `json:"keepLast"`
			MaxAgeDays *int `json:"maxAgeDays"`
		} `json:"rule"`
		FromTypes     []map[string]any `json:"fromTypes"`
		VersionFilter string           `json:"versionFilter"`
	}
	get := func() effective {
		t.Helper()
		resp := mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d/retention", id))
		assertStatus(t, resp, http.StatusOK)
		var e effective
		mustDecode(t, resp, &e)
		return e
	}

	if e := get(); e.Source != "none" || e.Rule != nil || e.VersionFilter != "" {
		t.Errorf("no policy: got %+v", e)
	}
	if ids := listArtifactIDs(t, "name=org.maint.retention&hasRetention=true"); len(ids) != 0 {
		t.Errorf("hasRetention before any policy: %v", ids)
	}

	// Types: {keepLast 3} and {keepLast 5, maxAgeDays 30} merge to keepLast 5, maxAge 30.
	assertStatus(t, mustPut(t, fmt.Sprintf("/api/v1/types/%d/retention", t1), map[string]any{"keepLast": 3}), http.StatusOK)
	assertStatus(t, mustPut(t, fmt.Sprintf("/api/v1/types/%d/retention", t2), map[string]any{"keepLast": 5, "maxAgeDays": 30}), http.StatusOK)
	e := get()
	if e.Source != "types" || len(e.FromTypes) != 2 || *e.Rule.KeepLast != 5 || *e.Rule.MaxAgeDays != 30 {
		t.Errorf("from types: got %+v", e)
	}
	if !strings.Contains(e.VersionFilter, "keepLast=5") || !strings.Contains(e.VersionFilter, "olderThan=") {
		t.Errorf("versionFilter: %q", e.VersionFilter)
	}
	// The filter is usable as-is on the version list.
	assertStatus(t, mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d/versions?%s", id, e.VersionFilter)), http.StatusOK)
	assertIDs(t, "hasRetention via type", listArtifactIDs(t, "name=org.maint.retention&hasRetention=true"), id)

	// Own policy wins over the types.
	assertStatus(t, mustPut(t, fmt.Sprintf("/api/v1/artifacts/%d/retention", id), map[string]any{"keepLast": 2}), http.StatusOK)
	if e = get(); e.Source != "artifact" || *e.Rule.KeepLast != 2 || e.Rule.MaxAgeDays != nil || e.VersionFilter != "keepLast=2" {
		t.Errorf("own policy: got %+v", e)
	}

	resp := mustGet(t, "/api/v1/retention-policies")
	assertStatus(t, resp, http.StatusOK)
	var all []map[string]any
	mustDecode(t, resp, &all)
	found := 0
	for _, p := range all {
		if p["artifactId"] != nil && int(p["artifactId"].(float64)) == id {
			found++
		}
		if p["typeId"] != nil && (int(p["typeId"].(float64)) == t1 || int(p["typeId"].(float64)) == t2) {
			found++
		}
	}
	if found != 3 {
		t.Errorf("list: found %d of 3 policies", found)
	}

	assertStatus(t, mustDelete(t, fmt.Sprintf("/api/v1/artifacts/%d/retention", id)), http.StatusNoContent)
	assertStatus(t, mustDelete(t, fmt.Sprintf("/api/v1/artifacts/%d/retention", id)), http.StatusNotFound)
	if e = get(); e.Source != "types" {
		t.Errorf("after own delete: source=%s", e.Source)
	}
	// Deleting a type takes its policy along.
	assertStatus(t, mustDelete(t, fmt.Sprintf("/api/v1/types/%d", t2)), http.StatusNoContent)
	if e = get(); len(e.FromTypes) != 1 || *e.Rule.KeepLast != 3 {
		t.Errorf("after type delete: got %+v", e)
	}

	assertStatus(t, mustPut(t, fmt.Sprintf("/api/v1/artifacts/%d/retention", id), map[string]any{}), http.StatusBadRequest)
	assertStatus(t, mustPut(t, fmt.Sprintf("/api/v1/artifacts/%d/retention", id), map[string]any{"keepLast": 0}), http.StatusBadRequest)
	assertStatus(t, mustGet(t, fmt.Sprintf("/api/v1/types/%d/retention", t2)), http.StatusNotFound)
	assertStatus(t, mustPut(t, "/api/v1/artifacts/999999999/retention", map[string]any{"keepLast": 1}), http.StatusNotFound)
}

func TestMetricsMaintenanceIndicators(t *testing.T) {
	id := artifactWithVersions(t, "org.maint.metrics", "1.0.0")
	tagVersion(t, id, "latest", "1.0.0") // version without files → broken tag

	resp := mustGet(t, "/q/metrics")
	assertStatus(t, resp, http.StatusOK)
	var m map[string]any
	mustDecode(t, resp, &m)
	for _, key := range []string{"artifactsWithoutFiles", "versionsWithoutFiles", "brokenTags"} {
		if v, ok := m[key].(float64); !ok || v < 1 {
			t.Errorf("%s: got %v, want >= 1", key, m[key])
		}
	}
	for _, key := range []string{"unusedTypes", "retentionPolicies", "storageBytesByBackend"} {
		if _, ok := m[key]; !ok {
			t.Errorf("%s missing", key)
		}
	}
	if backends, ok := m["storageBytesByBackend"].([]any); !ok || len(backends) == 0 {
		t.Errorf("storageBytesByBackend: got %v", m["storageBytesByBackend"])
	}
}
