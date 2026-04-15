package dto

import (
	"fmt"
	"time"

	db "fusion-platform/fusion-index/internal/db/sqlc"
	"fusion-platform/fusion-index/internal/semver"
)

// ---- Pagination ----

type PageResponse[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

// ---- Types ----

type TypeResponse struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// ---- Artifacts ----

type ArtifactResponse struct {
	ID          int64          `json:"id"`
	FullName    string         `json:"fullName"`
	Description *string        `json:"description"`
	Types       []TypeResponse `json:"types"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

// ---- Versions ----

type VersionResponse struct {
	ID         int64         `json:"id"`
	ArtifactID int64         `json:"artifactId"`
	Version    string        `json:"version"`
	Major      int32         `json:"major"`
	Minor      int32         `json:"minor"`
	Patch      int32         `json:"patch"`
	Config     *string       `json:"config"`
	Tags       []TagResponse `json:"tags"`
	CreatedAt  time.Time     `json:"createdAt"`
}

// ---- Tags ----

type TagResponse struct {
	ID         int64     `json:"id"`
	ArtifactID int64     `json:"artifactId"`
	Tag        string    `json:"tag"`
	VersionID  int64     `json:"versionId"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// ---- Files ----

type FileResponse struct {
	ID             int64     `json:"id"`
	VersionID      int64     `json:"versionId"`
	Name           string    `json:"name"`
	ContentType    *string   `json:"contentType"`
	SizeBytes      *int64    `json:"sizeBytes"`
	StorageBackend string    `json:"storageBackend"`
	Status         string    `json:"status"`
	DownloadURL    string    `json:"downloadUrl"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// ---- Mappers ----

func ToTypeResponse(t db.RegistryArtifactType) TypeResponse {
	return TypeResponse{
		ID:          t.ID,
		Name:        t.Name,
		Description: t.Description,
		CreatedAt:   t.CreatedAt.Time,
		UpdatedAt:   t.UpdatedAt.Time,
	}
}

func ToArtifactResponse(a db.RegistryArtifact, types []db.RegistryArtifactType) ArtifactResponse {
	typeResps := make([]TypeResponse, len(types))
	for i, t := range types {
		typeResps[i] = ToTypeResponse(t)
	}
	return ArtifactResponse{
		ID:          a.ID,
		FullName:    a.FullName,
		Description: a.Description,
		Types:       typeResps,
		CreatedAt:   a.CreatedAt.Time,
		UpdatedAt:   a.UpdatedAt.Time,
	}
}

func ToVersionResponse(v db.RegistryArtifactVersion, tags []db.RegistryArtifactTag) VersionResponse {
	tagResps := make([]TagResponse, len(tags))
	for i, t := range tags {
		tagResps[i] = ToTagResponse(t)
	}
	sv := semver.Semver{Major: v.Major, Minor: v.Minor, Patch: v.Patch}
	return VersionResponse{
		ID:         v.ID,
		ArtifactID: v.ArtifactID,
		Version:    sv.String(),
		Major:      v.Major,
		Minor:      v.Minor,
		Patch:      v.Patch,
		Config:     v.Config,
		Tags:       tagResps,
		CreatedAt:  v.CreatedAt.Time,
	}
}

func ToTagResponse(t db.RegistryArtifactTag) TagResponse {
	return TagResponse{
		ID:         t.ID,
		ArtifactID: t.ArtifactID,
		Tag:        t.Tag,
		VersionID:  t.VersionID,
		CreatedAt:  t.CreatedAt.Time,
		UpdatedAt:  t.UpdatedAt.Time,
	}
}

func ToFileResponse(f db.RegistryArtifactFile, artifactID int64, sv semver.Semver) FileResponse {
	return FileResponse{
		ID:             f.ID,
		VersionID:      f.VersionID,
		Name:           f.Name,
		ContentType:    f.ContentType,
		SizeBytes:      f.SizeBytes,
		StorageBackend: f.StorageBackend,
		Status:         f.Status,
		DownloadURL:    fmt.Sprintf("/api/v1/artifacts/%d/versions/%s/files/%d/download", artifactID, sv.String(), f.ID),
		CreatedAt:      f.CreatedAt.Time,
		UpdatedAt:      f.UpdatedAt.Time,
	}
}
