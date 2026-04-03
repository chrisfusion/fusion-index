package fusion.index.registry.service;

import fusion.index.registry.entity.JobTemplate;
import fusion.index.registry.entity.JobTemplateVersion;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.transaction.Transactional;
import jakarta.ws.rs.NotFoundException;
import jakarta.ws.rs.WebApplicationException;
import jakarta.ws.rs.core.Response;

import java.util.List;

@ApplicationScoped
public class JobTemplateService {

    @Transactional
    public JobTemplate create(String name, String description, String dockerImage,
                              String defaultRunConfig, String changelog) {
        if (JobTemplate.findByName(name).isPresent()) {
            throw new WebApplicationException(
                "A job template with name '" + name + "' already exists.",
                Response.Status.CONFLICT
            );
        }
        JobTemplate template = new JobTemplate();
        template.name = name;
        template.description = description;
        template.dockerImage = dockerImage;
        template.persist();

        publishVersion(template, dockerImage, defaultRunConfig, changelog);
        return template;
    }

    public JobTemplate findById(long id) {
        return (JobTemplate) JobTemplate.findByIdOptional(id)
            .orElseThrow(() -> new NotFoundException("Job template not found: " + id));
    }

    @Transactional
    public JobTemplate update(long id, String description, String dockerImage) {
        JobTemplate template = findById(id);
        if (description != null) template.description = description;
        if (dockerImage != null)  template.dockerImage = dockerImage;
        return template;
    }

    @Transactional
    public void delete(long id) {
        JobTemplate template = findById(id);
        long jobCount = template.versions.stream()
            .mapToLong(v -> (long) v.count("templateVersion.id", v.id))
            .sum();
        if (jobCount > 0) {
            throw new WebApplicationException(
                "Cannot delete template that is referenced by existing jobs.",
                Response.Status.CONFLICT
            );
        }
        template.delete();
    }

    public List<JobTemplate> listAll(int page, int pageSize) {
        return JobTemplate.findAll()
            .page(page, pageSize)
            .list();
    }

    public long countAll() {
        return JobTemplate.count();
    }

    @Transactional
    public JobTemplateVersion publishVersion(long templateId, String dockerImage,
                                              String defaultRunConfig, String changelog) {
        JobTemplate template = findById(templateId);
        return publishVersion(template, dockerImage, defaultRunConfig, changelog);
    }

    private JobTemplateVersion publishVersion(JobTemplate template, String dockerImage,
                                               String defaultRunConfig, String changelog) {
        int nextVersion = template.latestVersionNumber + 1;

        JobTemplateVersion version = new JobTemplateVersion();
        version.template = template;
        version.versionNumber = nextVersion;
        version.dockerImage = dockerImage != null ? dockerImage : template.dockerImage;
        version.defaultRunConfig = defaultRunConfig;
        version.changelog = changelog;
        version.persist();

        template.latestVersionNumber = nextVersion;
        return version;
    }

    public List<JobTemplateVersion> listVersions(long templateId) {
        findById(templateId); // ensure exists
        return JobTemplateVersion.find("template.id", templateId).list();
    }

    public JobTemplateVersion findVersion(long templateId, int versionNumber) {
        return JobTemplateVersion.findByTemplateAndVersion(templateId, versionNumber)
            .orElseThrow(() -> new NotFoundException(
                "Template version not found: template=" + templateId + " version=" + versionNumber
            ));
    }
}
