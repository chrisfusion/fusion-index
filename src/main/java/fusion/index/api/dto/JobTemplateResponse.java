package fusion.index.api.dto;

import java.time.Instant;

public class JobTemplateResponse {
    public long    id;
    public String  name;
    public String  description;
    public String  dockerImage;
    public int     latestVersionNumber;
    public Instant createdAt;
    public Instant updatedAt;
}
