package fusion.index.api.dto;

import java.time.Instant;

public class JobResponse {
    public long    id;
    public String  name;
    public String  description;
    public long    templateVersionId;
    public int     latestVersionNumber;
    public Instant createdAt;
    public Instant updatedAt;
}
