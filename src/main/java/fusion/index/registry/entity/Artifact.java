package fusion.index.registry.entity;

import fusion.index.registry.enums.ArtifactStatus;
import fusion.index.registry.enums.StorageBackend;
import io.quarkus.hibernate.orm.panache.PanacheEntity;
import jakarta.persistence.*;

import java.time.Instant;

@Entity
@Table(name = "artifact")
public class Artifact extends PanacheEntity {

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumn(name = "job_version_id", nullable = false)
    public JobVersion jobVersion;

    @Column(name = "name", nullable = false, length = 255)
    public String name;

    @Column(name = "content_type", length = 255)
    public String contentType;

    @Column(name = "size_bytes")
    public Long sizeBytes;

    @Enumerated(EnumType.STRING)
    @Column(name = "storage_backend", nullable = false, length = 20)
    public StorageBackend storageBackend;

    @Column(name = "storage_path", nullable = false, columnDefinition = "TEXT")
    public String storagePath;

    @Enumerated(EnumType.STRING)
    @Column(name = "status", nullable = false, length = 20)
    public ArtifactStatus status = ArtifactStatus.PENDING;

    @Column(name = "created_at", nullable = false)
    public Instant createdAt;

    @Column(name = "updated_at", nullable = false)
    public Instant updatedAt;

    @PrePersist
    void prePersist() {
        createdAt = Instant.now();
        updatedAt = Instant.now();
    }

    @PreUpdate
    void preUpdate() {
        updatedAt = Instant.now();
    }
}
