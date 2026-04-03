package fusion.index.api.dto;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Size;

public class PublishJobVersionRequest {

    @NotNull
    public Long templateVersionId;

    @NotBlank
    @Size(max = 500)
    public String dockerImage;

    @NotBlank
    @Size(max = 1000)
    public String gitUrl;

    @NotBlank
    @Size(max = 255)
    public String gitRef;

    @Size(max = 500)
    public String gitSubpath;

    public String runConfig;

    @Size(max = 2000)
    public String changelog;
}
