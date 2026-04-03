package fusion.index.api.dto;

import jakarta.validation.constraints.Size;

public class UpdateJobRequest {

    @Size(max = 2000)
    public String description;
}
