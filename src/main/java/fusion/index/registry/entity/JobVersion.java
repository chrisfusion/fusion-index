package fusion.index.registry.entity;

import io.quarkus.hibernate.orm.panache.PanacheEntity;
import jakarta.persistence.*;

import java.time.Instant;
import java.util.ArrayList;
import java.util.List;
import java.util.Optional;

@Entity
@Table(
    name = "job_version",
    uniqueConstraints = @UniqueConstraint(columnNames = {"job_id", "version_number"})
)
public class JobVersion extends PanacheEntity {

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumn(name = "job_id", nullable = false)
    public Job job;

    @Column(name = "version_number", nullable = false)
    public int versionNumber;

    @Column(name = "docker_image", nullable = false, length = 500)
    public String dockerImage;

    @Column(name = "git_url", nullable = false, length = 1000)
    public String gitUrl;

    @Column(name = "git_ref", nullable = false, length = 255)
    public String gitRef;

    @Column(name = "git_subpath", length = 500)
    public String gitSubpath;

    @Column(name = "run_config", columnDefinition = "TEXT")
    public String runConfig;

    // Snapshot — stores the ID of the template version used at creation time
    @Column(name = "template_version_id", nullable = false)
    public long templateVersionId;

    @Column(name = "created_at", nullable = false)
    public Instant createdAt;

    @OneToMany(mappedBy = "jobVersion", cascade = CascadeType.ALL, orphanRemoval = true, fetch = FetchType.LAZY)
    @OrderBy("createdAt ASC")
    public List<Artifact> artifacts = new ArrayList<>();

    @PrePersist
    void prePersist() {
        createdAt = Instant.now();
    }

    public static Optional<JobVersion> findByJobAndVersion(long jobId, int versionNumber) {
        return find("job.id = ?1 and versionNumber = ?2", jobId, versionNumber).firstResultOptional();
    }
}
