package fusion.index.api.dto;

import java.time.Instant;

public class JobTemplateVersionResponse {
    public long    id;
    public long    templateId;
    public int     versionNumber;
    public String  dockerImage;
    public String  defaultRunConfig;
    public String  changelog;
    public Instant createdAt;
}
