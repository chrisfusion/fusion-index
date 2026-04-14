package dto

// ---- Templates ----

type CreateTemplateRequest struct {
	Name             string  `json:"name"             binding:"required,max=255"`
	Description      *string `json:"description"      binding:"omitempty,max=2000"`
	DockerImage      string  `json:"dockerImage"      binding:"required,max=500"`
	DefaultRunConfig *string `json:"defaultRunConfig"`
	Changelog        *string `json:"changelog"        binding:"omitempty,max=2000"`
}

type UpdateTemplateRequest struct {
	Description *string `json:"description" binding:"omitempty,max=2000"`
	DockerImage *string `json:"dockerImage" binding:"omitempty,max=500"`
}

type PublishTemplateVersionRequest struct {
	DockerImage      string  `json:"dockerImage"      binding:"required,max=500"`
	DefaultRunConfig *string `json:"defaultRunConfig"`
	Changelog        *string `json:"changelog"        binding:"omitempty,max=2000"`
}

// ---- Jobs ----

type CreateJobRequest struct {
	Name              string  `json:"name"              binding:"required,max=255"`
	Description       *string `json:"description"       binding:"omitempty,max=2000"`
	TemplateVersionID int64   `json:"templateVersionId" binding:"required"`
	DockerImage       string  `json:"dockerImage"       binding:"required,max=500"`
	GitURL            string  `json:"gitUrl"            binding:"required,max=1000"`
	GitRef            string  `json:"gitRef"            binding:"required,max=255"`
	GitSubpath        *string `json:"gitSubpath"        binding:"omitempty,max=500"`
	RunConfig         *string `json:"runConfig"`
	Changelog         *string `json:"changelog"         binding:"omitempty,max=2000"`
}

type UpdateJobRequest struct {
	Description *string `json:"description" binding:"omitempty,max=2000"`
}

type PublishJobVersionRequest struct {
	TemplateVersionID int64   `json:"templateVersionId" binding:"required"`
	DockerImage       string  `json:"dockerImage"       binding:"required,max=500"`
	GitURL            string  `json:"gitUrl"            binding:"required,max=1000"`
	GitRef            string  `json:"gitRef"            binding:"required,max=255"`
	GitSubpath        *string `json:"gitSubpath"        binding:"omitempty,max=500"`
	RunConfig         *string `json:"runConfig"`
	Changelog         *string `json:"changelog"         binding:"omitempty,max=2000"`
}
