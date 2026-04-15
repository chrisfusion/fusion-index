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
