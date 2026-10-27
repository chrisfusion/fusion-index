package dto

// ---- Artifacts ----

type CreateArtifactRequest struct {
	FullName    string  `json:"fullName"    binding:"required,max=500"`
	Description *string `json:"description" binding:"omitempty,max=2000"`
}

type UpdateArtifactRequest struct {
	Description *string `json:"description" binding:"omitempty,max=2000"`
}

// ---- Versions ----

type CreateVersionRequest struct {
	Version string   `json:"version" binding:"required"`
	Config  *string  `json:"config"`
	Tags    []string `json:"tags"`
}

// ---- Tags ----

type AssignTagRequest struct {
	Version string `json:"version" binding:"required"`
}

// ---- Types ----

type CreateTypeRequest struct {
	Name        string  `json:"name"        binding:"required,max=255"`
	Description *string `json:"description" binding:"omitempty,max=2000"`
}

type UpdateTypeRequest struct {
	Name        string  `json:"name"        binding:"required,max=255"`
	Description *string `json:"description" binding:"omitempty,max=2000"`
}

// ---- Retention ----

// SetRetentionRequest needs at least one of the two fields (checked in the handler).
type SetRetentionRequest struct {
	KeepLast   *int32 `json:"keepLast"   binding:"omitempty,min=1"`
	MaxAgeDays *int32 `json:"maxAgeDays" binding:"omitempty,min=1"`
}
