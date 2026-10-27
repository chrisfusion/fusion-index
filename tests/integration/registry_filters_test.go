package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
)

type artifactListItem struct {
	ID                  int64  `json:"id"`
	FullName            string `json:"fullName"`
	VersionCount        int32  `json:"versionCount"`
	LargestVersionBytes int64  `json:"largestVersionBytes"`
}

func listArtifactIDs(t *testing.T, query string) []int64 {
	t.Helper()
	resp := mustGet(t, "/api/v1/artifacts?pageSize=100&"+query)
	assertStatus(t, resp, http.StatusOK)
	var page struct {
		Items []artifactListItem `json:"items"`
		Total int64              `json:"total"`
	}
	mustDecode(t, resp, &page)
	if page.Total != int64(len(page.Items)) {
		t.Errorf("%s: total %d but %d items", query, page.Total, len(page.Items))
	}
	ids := make([]int64, len(page.Items))
	for i, a := range page.Items {
		ids[i] = a.ID
	}
	return ids
}

func getArtifact(t *testing.T, id int) artifactListItem {
	t.Helper()
	resp := mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d", id))
	assertStatus(t, resp, http.StatusOK)
	var a artifactListItem
	mustDecode(t, resp, &a)
	return a
}

func assertIDs(t *testing.T, what string, got []int64, want ...int) {
	t.Helper()
	w := make([]int64, len(want))
	for i, id := range want {
		w[i] = int64(id)
	}
	if fmt.Sprint(got) != fmt.Sprint(w) {
		t.Errorf("%s: got %v, want %v", what, got, w)
	}
}

func uploadOK(t *testing.T, artifactID int, version, name string, size int) int64 {
	t.Helper()
	resp := uploadFile(t, artifactID, version, name, strings.Repeat("x", size))
	assertStatus(t, resp, http.StatusCreated)
	var f map[string]any
	mustDecode(t, resp, &f)
	return int64(f["id"].(float64))
}

func TestArtifactStatsMaintained(t *testing.T) {
	id := artifactWithVersions(t, "org.stats.maintained", "1.0.0", "2.0.0")
	uploadOK(t, id, "1.0.0", "a.bin", 100)
	big := uploadOK(t, id, "2.0.0", "b.bin", 300)
	uploadOK(t, id, "2.0.0", "c.bin", 50)

	a := getArtifact(t, id)
	if a.VersionCount != 2 || a.LargestVersionBytes != 350 {
		t.Errorf("after uploads: got count=%d largest=%d, want 2/350", a.VersionCount, a.LargestVersionBytes)
	}

	resp := mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d/versions/2.0.0", id))
	var v map[string]any
	mustDecode(t, resp, &v)
	if v["totalBytes"].(float64) != 350 {
		t.Errorf("version totalBytes: got %v, want 350", v["totalBytes"])
	}

	assertStatus(t, mustDelete(t, fmt.Sprintf("/api/v1/artifacts/%d/versions/2.0.0/files/%d", id, big)), http.StatusNoContent)
	if a = getArtifact(t, id); a.LargestVersionBytes != 100 {
		t.Errorf("after file delete: largest=%d, want 100", a.LargestVersionBytes)
	}
	assertStatus(t, mustDelete(t, fmt.Sprintf("/api/v1/artifacts/%d/versions/1.0.0", id)), http.StatusNoContent)
	if a = getArtifact(t, id); a.VersionCount != 1 || a.LargestVersionBytes != 50 {
		t.Errorf("after version delete: got count=%d largest=%d, want 1/50", a.VersionCount, a.LargestVersionBytes)
	}
}

func TestArtifactStatsConcurrentUploads(t *testing.T) {
	versions := []string{"1.0.0", "1.1.0", "1.2.0", "1.3.0", "1.4.0", "1.5.0", "1.6.0", "1.7.0"}
	id := artifactWithVersions(t, "org.stats.concurrent", versions...)

	var wg sync.WaitGroup
	for i, v := range versions {
		for j := 0; j < 3; j++ {
			wg.Add(1)
			go func(v string, size int) {
				defer wg.Done()
				resp := uploadFile(t, id, v, fmt.Sprintf("f%d.bin", size), strings.Repeat("x", size))
				resp.Body.Close()
				if resp.StatusCode != http.StatusCreated {
					t.Errorf("upload %s: status %d", v, resp.StatusCode)
				}
			}(v, (i+1)*10+j)
		}
	}
	wg.Wait()

	// Largest version is 1.7.0: 80 + 81 + 82.
	if a := getArtifact(t, id); a.VersionCount != 8 || a.LargestVersionBytes != 243 {
		t.Errorf("got count=%d largest=%d, want 8/243", a.VersionCount, a.LargestVersionBytes)
	}
}

func TestArtifactListSort(t *testing.T) {
	small := artifactWithVersions(t, "org.sorttest", "1.0.0")
	many := artifactWithVersions(t, "org.sorttest", "1.0.0", "1.1.0", "1.2.0")
	mid := artifactWithVersions(t, "org.sorttest", "1.0.0", "2.0.0")
	uploadOK(t, small, "1.0.0", "a.bin", 500)
	uploadOK(t, many, "1.2.0", "a.bin", 10)
	uploadOK(t, mid, "2.0.0", "a.bin", 200)

	assertIDs(t, "versionCount desc (default)", listArtifactIDs(t, "name=org.sorttest&sort=versionCount"), many, mid, small)
	assertIDs(t, "versionCount asc", listArtifactIDs(t, "name=org.sorttest&sort=versionCount&order=asc"), small, mid, many)
	assertIDs(t, "largestVersion desc", listArtifactIDs(t, "name=org.sorttest&sort=largestVersion"), small, mid, many)
	assertIDs(t, "largestVersion asc", listArtifactIDs(t, "name=org.sorttest&sort=largestVersion&order=asc"), many, mid, small)
	assertIDs(t, "createdAt desc", listArtifactIDs(t, "name=org.sorttest"), mid, many, small)
	assertIDs(t, "createdAt asc", listArtifactIDs(t, "name=org.sorttest&order=asc"), small, many, mid)

	assertStatus(t, mustGet(t, "/api/v1/artifacts?sort=size"), http.StatusBadRequest)
	assertStatus(t, mustGet(t, "/api/v1/artifacts?order=up"), http.StatusBadRequest)
}

func TestArtifactListMaintenanceFilters(t *testing.T) {
	empty := artifactWithVersions(t, "org.filtertest")
	noFiles := artifactWithVersions(t, "org.filtertest", "1.0.0")
	withFiles := artifactWithVersions(t, "org.filtertest", "1.0.0")
	uploadOK(t, withFiles, "1.0.0", "a.bin", 10)
	incomplete := artifactWithVersions(t, "org.filtertest", "1.0.0")
	resp := mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d/versions/1.0.0", incomplete))
	var v map[string]any
	mustDecode(t, resp, &v)
	createFileRow(t, int64(v["id"].(float64)), "broken.bin", "ERROR")
	tagVersion(t, withFiles, "release", "1.0.0")

	q := "name=org.filtertest&order=asc&"
	assertIDs(t, "maxVersions=0 (admin artifacts/empty)", listArtifactIDs(t, q+"maxVersions=0"), empty)
	assertIDs(t, "minVersions=1&hasFiles=false (admin artifacts/no-files)", listArtifactIDs(t, q+"minVersions=1&hasFiles=false"), noFiles)
	assertIDs(t, "hasIncompleteFiles=true", listArtifactIDs(t, q+"hasIncompleteFiles=true"), incomplete)
	assertIDs(t, "hasFiles=true", listArtifactIDs(t, q+"hasFiles=true"), withFiles, incomplete)
	// Filters combine with AND (they used to be mutually exclusive).
	assertIDs(t, "name AND tag", listArtifactIDs(t, q+"tag=release"), withFiles)
	assertIDs(t, "olderThan in the past", listArtifactIDs(t, q+"olderThan=2000-01-01T00:00:00Z"))

	assertStatus(t, mustGet(t, "/api/v1/artifacts?hasFiles=maybe"), http.StatusBadRequest)
	assertStatus(t, mustGet(t, "/api/v1/artifacts?minVersions=-1"), http.StatusBadRequest)
}

func TestVersionListFilters(t *testing.T) {
	id := artifactWithVersions(t, "org.vfilter", "1.0.0", "1.1.0", "2.0.0", "3.0.0")
	uploadOK(t, id, "2.0.0", "a.bin", 10)
	tagVersion(t, id, "stable", "1.0.0")
	resp := mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d/versions/1.1.0", id))
	var v map[string]any
	mustDecode(t, resp, &v)
	createFileRow(t, int64(v["id"].(float64)), "broken.bin", "PENDING")

	list := func(query string) []string {
		t.Helper()
		resp := mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d/versions?%s", id, query))
		assertStatus(t, resp, http.StatusOK)
		var vs []struct {
			Version string `json:"version"`
		}
		mustDecode(t, resp, &vs)
		out := make([]string, len(vs))
		for i, x := range vs {
			out[i] = x.Version
		}
		return out
	}
	assertVersions(t, "no filter, newest first", list(""), "3.0.0", "2.0.0", "1.1.0", "1.0.0")
	assertVersions(t, "keepLast=2", list("keepLast=2"), "1.1.0", "1.0.0")
	assertVersions(t, "keepLast=2&deletable=true (what DELETE would remove)", list("keepLast=2&deletable=true"), "1.1.0")
	assertVersions(t, "hasFiles=false (admin versions/empty)", list("hasFiles=false"), "3.0.0", "1.0.0")
	assertVersions(t, "hasIncompleteFiles=true", list("hasIncompleteFiles=true"), "1.1.0")
	assertVersions(t, "range", list("from=1.1.0&below=3.0.0"), "2.0.0", "1.1.0")

	// The dry run of the bulk delete agrees with the deletable list.
	r := bulkDeleteVersions(t, fmt.Sprintf("/api/v1/artifacts/%d/versions?keepLast=2&dryRun=true", id), http.StatusOK)
	assertVersions(t, "dry run", deletedVersions(r, id), "1.1.0")

	files := mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d/versions/1.1.0/files?status=PENDING&status=ERROR", id))
	assertStatus(t, files, http.StatusOK)
	var fs []map[string]any
	mustDecode(t, files, &fs)
	if len(fs) != 1 || fs[0]["name"] != "broken.bin" {
		t.Errorf("status filter: got %v", fs)
	}
	assertStatus(t, mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d/versions/1.1.0/files?status=GONE", id)), http.StatusBadRequest)
}

func TestVersionListAllPaginated(t *testing.T) {
	// 800.x versions keep this registry-wide list away from other tests' data.
	a := artifactWithVersions(t, "org.vall-a", "800.0.0", "800.1.0")
	b := artifactWithVersions(t, "org.vall-b", "800.0.0")

	get := func(query string) (total int64, items []string) {
		t.Helper()
		resp := mustGet(t, "/api/v1/versions?"+query)
		assertStatus(t, resp, http.StatusOK)
		var page struct {
			Total int64 `json:"total"`
			Items []struct {
				ArtifactID int64  `json:"artifactId"`
				Version    string `json:"version"`
			} `json:"items"`
		}
		mustDecode(t, resp, &page)
		for _, it := range page.Items {
			items = append(items, fmt.Sprintf("%d:%s", it.ArtifactID, it.Version))
		}
		return page.Total, items
	}

	from := "from=" + url.QueryEscape("800.0.0") + "&below=801.0.0"
	total, items := get(from + "&pageSize=2")
	if total != 3 || len(items) != 2 {
		t.Errorf("page 0: total=%d items=%v", total, items)
	}
	assertVersions(t, "page 0", items, fmt.Sprintf("%d:800.1.0", a), fmt.Sprintf("%d:800.0.0", a))
	total, items = get(from + "&pageSize=2&page=1")
	assertVersions(t, "page 1", items, fmt.Sprintf("%d:800.0.0", b))
	if total != 3 {
		t.Errorf("page 1 total=%d", total)
	}
	if total, _ = get(from + "&pageSize=2&page=5"); total != 3 {
		t.Errorf("past last page total=%d, want 3", total)
	}
	total, _ = get(from + "&keepLast=1")
	if total != 1 {
		t.Errorf("keepLast=1 per artifact: total=%d, want 1", total)
	}
	assertStatus(t, mustGet(t, "/api/v1/versions?version=1.0.0"), http.StatusBadRequest)
}
