package fusion.index.registry.entity;

import io.quarkus.hibernate.orm.panache.PanacheEntity;
import jakarta.persistence.*;

import java.time.Instant;
import java.util.Optional;

@Entity
@Table(
    name = "job_template_version",
    uniqueConstraints = @UniqueConstraint(columnNames = {"template_id", "version_number"})
)
public class JobTemplateVersion extends PanacheEntity {

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumn(name = "template_id", nullable = false)
    public JobTemplate template;

    @Column(name = "version_number", nullable = false)
    public int versionNumber;

    @Column(name = "docker_image", nullable = false, length = 500)
    public String dockerImage;

    @Column(name = "default_run_config", columnDefinition = "TEXT")
    public String defaultRunConfig;

    @Column(name = "changelog", columnDefinition = "TEXT")
    public String changelog;

    @Column(name = "created_at", nullable = false)
    public Instant createdAt;

    @PrePersist
    void prePersist() {
        createdAt = Instant.now();
    }

    public static Optional<JobTemplateVersion> findByTemplateAndVersion(long templateId, int versionNumber) {
        return find("template.id = ?1 and versionNumber = ?2", templateId, versionNumber).firstResultOptional();
    }
}
