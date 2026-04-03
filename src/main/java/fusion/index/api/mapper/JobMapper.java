package fusion.index.api.mapper;

import fusion.index.api.dto.JobResponse;
import fusion.index.api.dto.JobVersionResponse;
import fusion.index.registry.entity.Job;
import fusion.index.registry.entity.JobVersion;
import jakarta.enterprise.context.ApplicationScoped;

@ApplicationScoped
public class JobMapper {

    public JobResponse toResponse(Job j) {
        JobResponse r = new JobResponse();
        r.id                  = j.id;
        r.name                = j.name;
        r.description         = j.description;
        r.templateVersionId   = j.templateVersion.id;
        r.latestVersionNumber = j.latestVersionNumber;
        r.createdAt           = j.createdAt;
        r.updatedAt           = j.updatedAt;
        return r;
    }

    public JobVersionResponse toVersionResponse(JobVersion v) {
        JobVersionResponse r = new JobVersionResponse();
        r.id                = v.id;
        r.jobId             = v.job.id;
        r.versionNumber     = v.versionNumber;
        r.dockerImage       = v.dockerImage;
        r.gitUrl            = v.gitUrl;
        r.gitRef            = v.gitRef;
        r.gitSubpath        = v.gitSubpath;
        r.runConfig         = v.runConfig;
        r.templateVersionId = v.templateVersionId;
        r.createdAt         = v.createdAt;
        r.artifactCount     = v.artifacts.size();
        return r;
    }
}
