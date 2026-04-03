package fusion.index.api.dto;

import fusion.index.registry.enums.ArtifactStatus;
import fusion.index.registry.enums.StorageBackend;

import java.time.Instant;

public class ArtifactResponse {
    public long           id;
    public long           jobVersionId;
    public String         name;
    public String         contentType;
    public Long           sizeBytes;
    public StorageBackend storageBackend;
    public ArtifactStatus status;
    public String         downloadUrl;
    public Instant        createdAt;
    public Instant        updatedAt;
}
