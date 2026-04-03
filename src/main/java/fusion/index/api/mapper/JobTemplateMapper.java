package fusion.index.api.mapper;

import fusion.index.api.dto.JobTemplateResponse;
import fusion.index.api.dto.JobTemplateVersionResponse;
import fusion.index.registry.entity.JobTemplate;
import fusion.index.registry.entity.JobTemplateVersion;
import jakarta.enterprise.context.ApplicationScoped;

@ApplicationScoped
public class JobTemplateMapper {

    public JobTemplateResponse toResponse(JobTemplate t) {
        JobTemplateResponse r = new JobTemplateResponse();
        r.id                  = t.id;
        r.name                = t.name;
        r.description         = t.description;
        r.dockerImage         = t.dockerImage;
        r.latestVersionNumber = t.latestVersionNumber;
        r.createdAt           = t.createdAt;
        r.updatedAt           = t.updatedAt;
        return r;
    }

    public JobTemplateVersionResponse toVersionResponse(JobTemplateVersion v) {
        JobTemplateVersionResponse r = new JobTemplateVersionResponse();
        r.id                = v.id;
        r.templateId        = v.template.id;
        r.versionNumber     = v.versionNumber;
        r.dockerImage       = v.dockerImage;
        r.defaultRunConfig  = v.defaultRunConfig;
        r.changelog         = v.changelog;
        r.createdAt         = v.createdAt;
        return r;
    }
}
