package fusion.index.api;

import fusion.index.registry.entity.*;
import fusion.index.registry.enums.ArtifactStatus;
import fusion.index.registry.enums.StorageBackend;

import java.time.Instant;

public class TestDataFactory {

    public static JobTemplate jobTemplate(String name) {
        JobTemplate t = new JobTemplate();
        t.name        = name;
        t.description = "Test template: " + name;
        t.dockerImage = "registry.example.com/test:latest";
        t.createdAt   = Instant.now();
        t.updatedAt   = Instant.now();
        return t;
    }

    public static JobTemplateVersion templateVersion(JobTemplate template, int versionNumber) {
        JobTemplateVersion v = new JobTemplateVersion();
        v.template      = template;
        v.versionNumber = versionNumber;
        v.dockerImage   = template.dockerImage;
        v.changelog     = "Version " + versionNumber;
        v.createdAt     = Instant.now();
        return v;
    }

    public static Job job(String name, JobTemplateVersion templateVersion) {
        Job j = new Job();
        j.name            = name;
        j.description     = "Test job: " + name;
        j.templateVersion = templateVersion;
        j.createdAt       = Instant.now();
        j.updatedAt       = Instant.now();
        return j;
    }

    public static JobVersion jobVersion(Job job, int versionNumber, JobTemplateVersion templateVersion) {
        JobVersion v = new JobVersion();
        v.job               = job;
        v.versionNumber     = versionNumber;
        v.dockerImage       = "registry.example.com/job:v" + versionNumber;
        v.gitUrl            = "https://github.com/test/repo.git";
        v.gitRef            = "main";
        v.gitSubpath        = null;
        v.runConfig         = "{\"key\":\"value\"}";
        v.templateVersionId = templateVersion.id;
        v.createdAt         = Instant.now();
        return v;
    }

    public static Artifact artifact(JobVersion jobVersion, String name) {
        Artifact a = new Artifact();
        a.jobVersion     = jobVersion;
        a.name           = name;
        a.contentType    = "application/octet-stream";
        a.sizeBytes      = 1024L;
        a.storageBackend = StorageBackend.FILESYSTEM;
        a.storagePath    = jobVersion.id + "/" + name;
        a.status         = ArtifactStatus.AVAILABLE;
        a.createdAt      = Instant.now();
        a.updatedAt      = Instant.now();
        return a;
    }
}
