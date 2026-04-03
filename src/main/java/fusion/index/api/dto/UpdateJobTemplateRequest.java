package fusion.index.api.dto;

import jakarta.validation.constraints.Size;

public class UpdateJobTemplateRequest {

    @Size(max = 2000)
    public String description;

    @Size(max = 500)
    public String dockerImage;
}
