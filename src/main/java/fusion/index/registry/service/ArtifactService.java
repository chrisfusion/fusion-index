package fusion.index.registry.service;

import fusion.index.registry.entity.Artifact;
import fusion.index.registry.entity.JobVersion;
import fusion.index.registry.enums.ArtifactStatus;
import fusion.index.registry.enums.StorageBackend;
import fusion.index.registry.storage.ArtifactStorage;
import fusion.index.registry.storage.StorageType;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.enterprise.inject.Any;
import jakarta.enterprise.inject.Instance;
import jakarta.inject.Inject;
import jakarta.transaction.Transactional;
import jakarta.ws.rs.NotFoundException;

import java.io.InputStream;
import java.util.List;

@ApplicationScoped
public class ArtifactService {

    @Inject
    @Any
    Instance<ArtifactStorage> storageInstances;

    @Inject
    JobService jobService;

    @Inject
    @org.eclipse.microprofile.config.inject.ConfigProperty(name = "fusion.index.storage.backend", defaultValue = "FILESYSTEM")
    StorageBackend activeBackend;

    private ArtifactStorage storage() {
        for (ArtifactStorage s : storageInstances) {
            if (s.getBackend() == activeBackend) {
                return s;
            }
        }
        throw new IllegalStateException("No ArtifactStorage implementation for backend: " + activeBackend);
    }

    @Transactional
    public Artifact register(long jobId, int versionNumber, String name, String contentType) {
        JobVersion jobVersion = jobService.findVersion(jobId, versionNumber);

        Artifact artifact = new Artifact();
        artifact.jobVersion = jobVersion;
        artifact.name = name;
        artifact.contentType = contentType;
        artifact.storageBackend = activeBackend;
        artifact.storagePath = jobVersion.id + "/" + name;
        artifact.status = ArtifactStatus.PENDING;
        artifact.persist();
        return artifact;
    }

    @Transactional
    public Artifact store(long artifactId, InputStream data, long sizeHint) {
        Artifact artifact = findById(artifactId);
        try {
            String resolvedPath = storage().store(artifact.storagePath, data, sizeHint, artifact.contentType);
            artifact.storagePath = resolvedPath;
            artifact.sizeBytes = sizeHint > 0 ? sizeHint : null;
            artifact.status = ArtifactStatus.AVAILABLE;
        } catch (Exception e) {
            artifact.status = ArtifactStatus.ERROR;
            throw e;
        }
        return artifact;
    }

    public InputStream retrieve(long artifactId) {
        Artifact artifact = findById(artifactId);
        if (artifact.status != ArtifactStatus.AVAILABLE) {
            throw new IllegalStateException("Artifact is not available: status=" + artifact.status);
        }
        // Use the backend stored on the artifact, not the currently active one
        ArtifactStorage storage = storageForBackend(artifact.storageBackend);
        return storage.retrieve(artifact.storagePath);
    }

    @Transactional
    public void delete(long artifactId) {
        Artifact artifact = findById(artifactId);
        try {
            storageForBackend(artifact.storageBackend).delete(artifact.storagePath);
        } catch (Exception ignored) {
            // Best-effort storage cleanup; metadata deletion proceeds regardless
        }
        artifact.delete();
    }

    public Artifact findById(long id) {
        return (Artifact) Artifact.findByIdOptional(id)
            .orElseThrow(() -> new NotFoundException("Artifact not found: " + id));
    }

    public List<Artifact> listForJobVersion(long jobId, int versionNumber) {
        JobVersion jobVersion = jobService.findVersion(jobId, versionNumber);
        return Artifact.find("jobVersion.id", jobVersion.id).list();
    }

    private ArtifactStorage storageForBackend(StorageBackend backend) {
        for (ArtifactStorage s : storageInstances) {
            if (s.getBackend() == backend) {
                return s;
            }
        }
        throw new IllegalStateException("No ArtifactStorage implementation for backend: " + backend);
    }
}
