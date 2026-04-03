package fusion.index.api.dto;

import java.time.Instant;

public class JobVersionResponse {
    public long    id;
    public long    jobId;
    public int     versionNumber;
    public String  dockerImage;
    public String  gitUrl;
    public String  gitRef;
    public String  gitSubpath;
    public String  runConfig;
    public long    templateVersionId;
    public Instant createdAt;
    public int     artifactCount;
}
