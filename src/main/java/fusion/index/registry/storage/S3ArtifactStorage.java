package fusion.index.registry.storage;

import fusion.index.registry.enums.StorageBackend;
import jakarta.enterprise.context.ApplicationScoped;
import org.eclipse.microprofile.config.inject.ConfigProperty;
import software.amazon.awssdk.core.sync.RequestBody;
import software.amazon.awssdk.services.s3.S3Client;
import software.amazon.awssdk.services.s3.model.*;

import jakarta.inject.Inject;
import java.io.InputStream;

@ApplicationScoped
@StorageType(StorageBackend.S3)
public class S3ArtifactStorage implements ArtifactStorage {

    @Inject
    S3Client s3;

    @ConfigProperty(name = "fusion.index.storage.s3.bucket")
    String bucket;

    @Override
    public String store(String suggestedPath, InputStream data, long sizeHint, String contentType) {
        PutObjectRequest.Builder builder = PutObjectRequest.builder()
            .bucket(bucket)
            .key(suggestedPath);

        if (contentType != null) {
            builder.contentType(contentType);
        }

        RequestBody body = sizeHint > 0
            ? RequestBody.fromInputStream(data, sizeHint)
            : RequestBody.fromContentProvider(() -> data, contentType != null ? contentType : "application/octet-stream");

        s3.putObject(builder.build(), body);
        return suggestedPath;
    }

    @Override
    public InputStream retrieve(String storagePath) {
        GetObjectRequest request = GetObjectRequest.builder()
            .bucket(bucket)
            .key(storagePath)
            .build();
        return s3.getObject(request);
    }

    @Override
    public void delete(String storagePath) {
        try {
            DeleteObjectRequest request = DeleteObjectRequest.builder()
                .bucket(bucket)
                .key(storagePath)
                .build();
            s3.deleteObject(request);
        } catch (NoSuchKeyException ignored) {
            // Idempotent — already absent
        }
    }

    @Override
    public StorageBackend getBackend() {
        return StorageBackend.S3;
    }
}
