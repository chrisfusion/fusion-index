package integration

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func createType(t *testing.T, name string) map[string]any {
	t.Helper()
	resp := mustPost(t, "/api/v1/types", map[string]any{"name": name})
	assertStatus(t, resp, http.StatusCreated)
	var created map[string]any
	mustDecode(t, resp, &created)
	return created
}

func uniqueTypeName() string {
	return fmt.Sprintf("type-%d", time.Now().UnixNano())
}

// ---- Type CRUD ----

func TestCreateType(t *testing.T) {
	ty := createType(t, uniqueTypeName())
	if ty["id"] == nil {
		t.Fatal("expected id in response")
	}
	if ty["name"] == nil {
		t.Fatal("expected name in response")
	}
}

func TestCreateTypeWithDescription(t *testing.T) {
	name := uniqueTypeName()
	resp := mustPost(t, "/api/v1/types", map[string]any{
		"name":        name,
		"description": "a test type",
	})
	assertStatus(t, resp, http.StatusCreated)
	var ty map[string]any
	mustDecode(t, resp, &ty)
	assertFieldEqual(t, ty, "description", "a test type")
}

func TestCreateTypeDuplicateName(t *testing.T) {
	name := uniqueTypeName()
	createType(t, name)
	resp := mustPost(t, "/api/v1/types", map[string]any{"name": name})
	assertStatus(t, resp, http.StatusConflict)
}

func TestCreateTypeMissingName(t *testing.T) {
	resp := mustPost(t, "/api/v1/types", map[string]any{})
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestGetType(t *testing.T) {
	ty := createType(t, uniqueTypeName())
	id := int(ty["id"].(float64))
	resp := mustGet(t, fmt.Sprintf("/api/v1/types/%d", id))
	assertStatus(t, resp, http.StatusOK)
	var got map[string]any
	mustDecode(t, resp, &got)
	assertFieldEqual(t, got, "name", ty["name"])
}

func TestGetTypeNotFound(t *testing.T) {
	assertStatus(t, mustGet(t, "/api/v1/types/999999999"), http.StatusNotFound)
}

func TestUpdateType(t *testing.T) {
	ty := createType(t, uniqueTypeName())
	id := int(ty["id"].(float64))
	newName := uniqueTypeName()
	resp := mustPut(t, fmt.Sprintf("/api/v1/types/%d", id), map[string]any{
		"name":        newName,
		"description": "updated",
	})
	assertStatus(t, resp, http.StatusOK)
	var updated map[string]any
	mustDecode(t, resp, &updated)
	assertFieldEqual(t, updated, "name", newName)
	assertFieldEqual(t, updated, "description", "updated")
}

func TestUpdateTypeConflict(t *testing.T) {
	name1 := uniqueTypeName()
	name2 := uniqueTypeName()
	createType(t, name1)
	ty2 := createType(t, name2)
	id2 := int(ty2["id"].(float64))
	resp := mustPut(t, fmt.Sprintf("/api/v1/types/%d", id2), map[string]any{
		"name": name1,
	})
	assertStatus(t, resp, http.StatusConflict)
}

func TestDeleteType(t *testing.T) {
	ty := createType(t, uniqueTypeName())
	id := int(ty["id"].(float64))
	assertStatus(t, mustDelete(t, fmt.Sprintf("/api/v1/types/%d", id)), http.StatusNoContent)
	assertStatus(t, mustGet(t, fmt.Sprintf("/api/v1/types/%d", id)), http.StatusNotFound)
}

func TestListTypes(t *testing.T) {
	createType(t, uniqueTypeName())
	resp := mustGet(t, "/api/v1/types")
	assertStatus(t, resp, http.StatusOK)
	var items []any
	mustDecode(t, resp, &items)
	if len(items) == 0 {
		t.Fatal("expected at least one type in list")
	}
}

// ---- Artifact type assignment ----

func TestAssignTypeToArtifact(t *testing.T) {
	a := createArtifact(t, "org.typeassign")
	artifactID := int(a["id"].(float64))
	ty := createType(t, uniqueTypeName())
	typeID := int(ty["id"].(float64))

	resp := mustPut(t, fmt.Sprintf("/api/v1/artifacts/%d/types/%d", artifactID, typeID), nil)
	assertStatus(t, resp, http.StatusOK)
	var got map[string]any
	mustDecode(t, resp, &got)
	assertFieldEqual(t, got, "name", ty["name"])
}

func TestAssignTypeIdempotent(t *testing.T) {
	a := createArtifact(t, "org.typeidempotent")
	artifactID := int(a["id"].(float64))
	ty := createType(t, uniqueTypeName())
	typeID := int(ty["id"].(float64))

	mustPut(t, fmt.Sprintf("/api/v1/artifacts/%d/types/%d", artifactID, typeID), nil)
	resp := mustPut(t, fmt.Sprintf("/api/v1/artifacts/%d/types/%d", artifactID, typeID), nil)
	assertStatus(t, resp, http.StatusOK)
}

func TestAssignTypeArtifactNotFound(t *testing.T) {
	ty := createType(t, uniqueTypeName())
	typeID := int(ty["id"].(float64))
	resp := mustPut(t, fmt.Sprintf("/api/v1/artifacts/999999999/types/%d", typeID), nil)
	assertStatus(t, resp, http.StatusNotFound)
}

func TestAssignTypeNotFound(t *testing.T) {
	a := createArtifact(t, "org.typenotfound")
	artifactID := int(a["id"].(float64))
	resp := mustPut(t, fmt.Sprintf("/api/v1/artifacts/%d/types/999999999", artifactID), nil)
	assertStatus(t, resp, http.StatusNotFound)
}

func TestUnassignType(t *testing.T) {
	a := createArtifact(t, "org.typeunassign")
	artifactID := int(a["id"].(float64))
	ty := createType(t, uniqueTypeName())
	typeID := int(ty["id"].(float64))

	mustPut(t, fmt.Sprintf("/api/v1/artifacts/%d/types/%d", artifactID, typeID), nil)
	assertStatus(t, mustDelete(t, fmt.Sprintf("/api/v1/artifacts/%d/types/%d", artifactID, typeID)), http.StatusNoContent)

	// After unassign, assignment should be gone
	resp := mustDelete(t, fmt.Sprintf("/api/v1/artifacts/%d/types/%d", artifactID, typeID))
	assertStatus(t, resp, http.StatusNotFound)
}

func TestUnassignTypeNotAssigned(t *testing.T) {
	a := createArtifact(t, "org.typenotassigned")
	artifactID := int(a["id"].(float64))
	ty := createType(t, uniqueTypeName())
	typeID := int(ty["id"].(float64))

	resp := mustDelete(t, fmt.Sprintf("/api/v1/artifacts/%d/types/%d", artifactID, typeID))
	assertStatus(t, resp, http.StatusNotFound)
}

func TestListArtifactTypes(t *testing.T) {
	a := createArtifact(t, "org.listtypes")
	artifactID := int(a["id"].(float64))
	ty := createType(t, uniqueTypeName())
	typeID := int(ty["id"].(float64))

	mustPut(t, fmt.Sprintf("/api/v1/artifacts/%d/types/%d", artifactID, typeID), nil)

	resp := mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d/types", artifactID))
	assertStatus(t, resp, http.StatusOK)
	var items []any
	mustDecode(t, resp, &items)
	if len(items) != 1 {
		t.Fatalf("expected 1 type, got %d", len(items))
	}
}

// ---- Artifact type filter ----

func TestListArtifactsFilterByType(t *testing.T) {
	typeName := uniqueTypeName()
	ty := createType(t, typeName)
	typeID := int(ty["id"].(float64))

	a := createArtifact(t, "org.typefilter")
	artifactID := int(a["id"].(float64))
	mustPut(t, fmt.Sprintf("/api/v1/artifacts/%d/types/%d", artifactID, typeID), nil)

	resp := mustGet(t, fmt.Sprintf("/api/v1/artifacts?type=%s", typeName))
	assertStatus(t, resp, http.StatusOK)
	var page map[string]any
	mustDecode(t, resp, &page)
	items := page["items"].([]any)
	if len(items) == 0 {
		t.Fatal("expected artifact to appear in type-filtered list")
	}
}

func TestListArtifactsFilterByMultipleTypes(t *testing.T) {
	typeName1 := uniqueTypeName()
	typeName2 := uniqueTypeName()
	ty1 := createType(t, typeName1)
	ty2 := createType(t, typeName2)
	typeID1 := int(ty1["id"].(float64))
	typeID2 := int(ty2["id"].(float64))

	a1 := createArtifact(t, "org.multitype1")
	a2 := createArtifact(t, "org.multitype2")
	mustPut(t, fmt.Sprintf("/api/v1/artifacts/%d/types/%d", int(a1["id"].(float64)), typeID1), nil)
	mustPut(t, fmt.Sprintf("/api/v1/artifacts/%d/types/%d", int(a2["id"].(float64)), typeID2), nil)

	// OR semantics: both artifacts should appear
	resp := mustGet(t, fmt.Sprintf("/api/v1/artifacts?type=%s&type=%s", typeName1, typeName2))
	assertStatus(t, resp, http.StatusOK)
	var page map[string]any
	mustDecode(t, resp, &page)
	items := page["items"].([]any)
	if len(items) < 2 {
		t.Fatalf("expected at least 2 artifacts for multi-type OR filter, got %d", len(items))
	}
}

// ---- Types embedded in artifact responses ----

func TestArtifactGetIncludesTypes(t *testing.T) {
	a := createArtifact(t, "org.typesembed")
	artifactID := int(a["id"].(float64))
	ty := createType(t, uniqueTypeName())
	typeID := int(ty["id"].(float64))
	mustPut(t, fmt.Sprintf("/api/v1/artifacts/%d/types/%d", artifactID, typeID), nil)

	resp := mustGet(t, fmt.Sprintf("/api/v1/artifacts/%d", artifactID))
	assertStatus(t, resp, http.StatusOK)
	var got map[string]any
	mustDecode(t, resp, &got)
	types, ok := got["types"].([]any)
	if !ok || len(types) != 1 {
		t.Fatalf("expected 1 type in artifact response, got %v", got["types"])
	}
}

func TestArtifactListIncludesTypes(t *testing.T) {
	typeName := uniqueTypeName()
	ty := createType(t, typeName)
	typeID := int(ty["id"].(float64))
	a := createArtifact(t, "org.listembed")
	artifactID := int(a["id"].(float64))
	mustPut(t, fmt.Sprintf("/api/v1/artifacts/%d/types/%d", artifactID, typeID), nil)

	resp := mustGet(t, fmt.Sprintf("/api/v1/artifacts?name=%s", "org.listembed"))
	assertStatus(t, resp, http.StatusOK)
	var page map[string]any
	mustDecode(t, resp, &page)
	items := page["items"].([]any)
	if len(items) == 0 {
		t.Fatal("expected at least one artifact")
	}
	// Verify that at least the newly created one has the type embedded
	for _, item := range items {
		m := item.(map[string]any)
		if int(m["id"].(float64)) == artifactID {
			types, ok := m["types"].([]any)
			if !ok || len(types) == 0 {
				t.Fatal("expected types in artifact list response")
			}
			return
		}
	}
	t.Fatal("artifact not found in list response")
}
