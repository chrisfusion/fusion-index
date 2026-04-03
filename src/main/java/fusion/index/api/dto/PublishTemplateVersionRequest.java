package fusion.index.api.dto;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;

public class PublishTemplateVersionRequest {

    @NotBlank
    @Size(max = 500)
    public String dockerImage;

    public String defaultRunConfig;

    @Size(max = 2000)
    public String changelog;
}
