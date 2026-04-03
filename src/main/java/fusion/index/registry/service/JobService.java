package fusion.index.registry.service;

import fusion.index.registry.entity.Job;
import fusion.index.registry.entity.JobTemplateVersion;
import fusion.index.registry.entity.JobVersion;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import jakarta.transaction.Transactional;
import jakarta.ws.rs.NotFoundException;
import jakarta.ws.rs.WebApplicationException;
import jakarta.ws.rs.core.Response;

import java.util.List;

@ApplicationScoped
public class JobService {

    @Inject
    JobTemplateService templateService;

    @Transactional
    public Job create(String name, String description, long templateVersionId,
                      String dockerImage, String gitUrl, String gitRef,
                      String gitSubpath, String runConfig, String changelog) {
        if (Job.findByName(name).isPresent()) {
            throw new WebApplicationException(
                "A job with name '" + name + "' already exists.",
                Response.Status.CONFLICT
            );
        }
        JobTemplateVersion templateVersion = (JobTemplateVersion) JobTemplateVersion.findByIdOptional(templateVersionId)
            .orElseThrow(() -> new NotFoundException("Job template version not found: " + templateVersionId));

        Job job = new Job();
        job.name = name;
        job.description = description;
        job.templateVersion = templateVersion;
        job.persist();

        publishVersion(job, templateVersion, dockerImage, gitUrl, gitRef, gitSubpath, runConfig, changelog);
        return job;
    }

    public Job findById(long id) {
        return (Job) Job.findByIdOptional(id)
            .orElseThrow(() -> new NotFoundException("Job not found: " + id));
    }

    @Transactional
    public Job update(long id, String description) {
        Job job = findById(id);
        if (description != null) job.description = description;
        return job;
    }

    @Transactional
    public void delete(long id) {
        Job job = findById(id);
        job.delete();
    }

    public List<Job> listAll(int page, int pageSize) {
        return Job.findAll()
            .page(page, pageSize)
            .list();
    }

    public long countAll() {
        return Job.count();
    }

    @Transactional
    public JobVersion publishVersion(long jobId, long templateVersionId,
                                      String dockerImage, String gitUrl, String gitRef,
                                      String gitSubpath, String runConfig, String changelog) {
        Job job = findById(jobId);
        JobTemplateVersion templateVersion = (JobTemplateVersion) JobTemplateVersion.findByIdOptional(templateVersionId)
            .orElseThrow(() -> new NotFoundException("Job template version not found: " + templateVersionId));
        return publishVersion(job, templateVersion, dockerImage, gitUrl, gitRef, gitSubpath, runConfig, changelog);
    }

    private JobVersion publishVersion(Job job, JobTemplateVersion templateVersion,
                                       String dockerImage, String gitUrl, String gitRef,
                                       String gitSubpath, String runConfig, String changelog) {
        int nextVersion = job.latestVersionNumber + 1;

        JobVersion version = new JobVersion();
        version.job = job;
        version.versionNumber = nextVersion;
        version.dockerImage = dockerImage;
        version.gitUrl = gitUrl;
        version.gitRef = gitRef;
        version.gitSubpath = gitSubpath;
        version.runConfig = runConfig;
        version.templateVersionId = templateVersion.id;
        version.persist();

        job.latestVersionNumber = nextVersion;
        return version;
    }

    public List<JobVersion> listVersions(long jobId) {
        findById(jobId); // ensure exists
        return JobVersion.find("job.id", jobId).list();
    }

    public JobVersion findVersion(long jobId, int versionNumber) {
        return JobVersion.findByJobAndVersion(jobId, versionNumber)
            .orElseThrow(() -> new NotFoundException(
                "Job version not found: job=" + jobId + " version=" + versionNumber
            ));
    }
}
