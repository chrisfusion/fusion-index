package fusion.index.registry.entity;

import io.quarkus.hibernate.orm.panache.PanacheEntity;
import jakarta.persistence.*;

import java.time.Instant;
import java.util.ArrayList;
import java.util.List;
import java.util.Optional;

@Entity
@Table(name = "job_template")
public class JobTemplate extends PanacheEntity {

    @Column(name = "name", nullable = false, unique = true)
    public String name;

    @Column(name = "description", columnDefinition = "TEXT")
    public String description;

    @Column(name = "docker_image", nullable = false, length = 500)
    public String dockerImage;

    @Column(name = "latest_version_number", nullable = false)
    public int latestVersionNumber = 0;

    @Column(name = "created_at", nullable = false)
    public Instant createdAt;

    @Column(name = "updated_at", nullable = false)
    public Instant updatedAt;

    @OneToMany(mappedBy = "template", cascade = CascadeType.ALL, orphanRemoval = true, fetch = FetchType.LAZY)
    @OrderBy("versionNumber ASC")
    public List<JobTemplateVersion> versions = new ArrayList<>();

    @PrePersist
    void prePersist() {
        createdAt = Instant.now();
        updatedAt = Instant.now();
    }

    @PreUpdate
    void preUpdate() {
        updatedAt = Instant.now();
    }

    public static Optional<JobTemplate> findByName(String name) {
        return find("name", name).firstResultOptional();
    }
}
