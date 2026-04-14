package dto

import (
	"fmt"
	"time"

	db "fusion-platform/fusion-index/internal/db/sqlc"
)

// ---- Pagination ----

type PageResponse[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

// ---- Templates ----

type TemplateResponse struct {
	ID                  int64   `json:"id"`
	Name                string  `json:"name"`
	Description         *string `json:"description"`
	DockerImage         string  `json:"dockerImage"`
	LatestVersionNumber int32   `json:"latestVersionNumber"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

type TemplateVersionResponse struct {
	ID               int64     `json:"id"`
	TemplateID       int64     `json:"templateId"`
	VersionNumber    int32     `json:"versionNumber"`
	DockerImage      string    `json:"dockerImage"`
	DefaultRunConfig *string   `json:"defaultRunConfig"`
	Changelog        *string   `json:"changelog"`
	CreatedAt        time.Time `json:"createdAt"`
}

// ---- Jobs ----

type JobResponse struct {
	ID                  int64     `json:"id"`
	Name                string    `json:"name"`
	Description         *string   `json:"description"`
	TemplateVersionID   int64     `json:"templateVersionId"`
	LatestVersionNumber int32     `json:"latestVersionNumber"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

type JobVersionResponse struct {
	ID                int64     `json:"id"`
	JobID             int64     `json:"jobId"`
	VersionNumber     int32     `json:"versionNumber"`
	DockerImage       string    `json:"dockerImage"`
	GitURL            string    `json:"gitUrl"`
	GitRef            string    `json:"gitRef"`
	GitSubpath        *string   `json:"gitSubpath"`
	RunConfig         *string   `json:"runConfig"`
	TemplateVersionID int64     `json:"templateVersionId"`
	ArtifactCount     int64     `json:"artifactCount"`
	CreatedAt         time.Time `json:"createdAt"`
}

// ---- Artifacts ----

type ArtifactResponse struct {
	ID             int64     `json:"id"`
	JobVersionID   int64     `json:"jobVersionId"`
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

func ToTemplateResponse(t db.JobTemplate) TemplateResponse {
	return TemplateResponse{
		ID:                  t.ID,
		Name:                t.Name,
		Description:         t.Description,
		DockerImage:         t.DockerImage,
		LatestVersionNumber: t.LatestVersionNumber,
		CreatedAt:           t.CreatedAt.Time,
		UpdatedAt:           t.UpdatedAt.Time,
	}
}

func ToTemplateVersionResponse(v db.JobTemplateVersion) TemplateVersionResponse {
	return TemplateVersionResponse{
		ID:               v.ID,
		TemplateID:       v.TemplateID,
		VersionNumber:    v.VersionNumber,
		DockerImage:      v.DockerImage,
		DefaultRunConfig: v.DefaultRunConfig,
		Changelog:        v.Changelog,
		CreatedAt:        v.CreatedAt.Time,
	}
}

func ToJobResponse(j db.Job) JobResponse {
	return JobResponse{
		ID:                  j.ID,
		Name:                j.Name,
		Description:         j.Description,
		TemplateVersionID:   j.TemplateVersionID,
		LatestVersionNumber: j.LatestVersionNumber,
		CreatedAt:           j.CreatedAt.Time,
		UpdatedAt:           j.UpdatedAt.Time,
	}
}

func ToJobVersionResponse(v db.JobVersion, artifactCount int64) JobVersionResponse {
	return JobVersionResponse{
		ID:                v.ID,
		JobID:             v.JobID,
		VersionNumber:     v.VersionNumber,
		DockerImage:       v.DockerImage,
		GitURL:            v.GitUrl,
		GitRef:            v.GitRef,
		GitSubpath:        v.GitSubpath,
		RunConfig:         v.RunConfig,
		TemplateVersionID: v.TemplateVersionID,
		ArtifactCount:     artifactCount,
		CreatedAt:         v.CreatedAt.Time,
	}
}

func ToArtifactResponse(a db.Artifact) ArtifactResponse {
	return ArtifactResponse{
		ID:             a.ID,
		JobVersionID:   a.JobVersionID,
		Name:           a.Name,
		ContentType:    a.ContentType,
		SizeBytes:      a.SizeBytes,
		StorageBackend: a.StorageBackend,
		Status:         a.Status,
		DownloadURL:    fmt.Sprintf("/api/v1/artifacts/%d/download", a.ID),
		CreatedAt:      a.CreatedAt.Time,
		UpdatedAt:      a.UpdatedAt.Time,
	}
}
