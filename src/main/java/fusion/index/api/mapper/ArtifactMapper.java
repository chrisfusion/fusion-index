package fusion.index.api.mapper;

import fusion.index.api.dto.ArtifactResponse;
import fusion.index.registry.entity.Artifact;
import jakarta.enterprise.context.ApplicationScoped;

@ApplicationScoped
public class ArtifactMapper {

    public ArtifactResponse toResponse(Artifact a) {
        ArtifactResponse r = new ArtifactResponse();
        r.id             = a.id;
        r.jobVersionId   = a.jobVersion.id;
        r.name           = a.name;
        r.contentType    = a.contentType;
        r.sizeBytes      = a.sizeBytes;
        r.storageBackend = a.storageBackend;
        r.status         = a.status;
        r.downloadUrl    = "/api/v1/artifacts/" + a.id + "/download";
        r.createdAt      = a.createdAt;
        r.updatedAt      = a.updatedAt;
        return r;
    }
}
