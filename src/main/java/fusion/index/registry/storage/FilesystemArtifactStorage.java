package fusion.index.registry.storage;

import fusion.index.registry.enums.StorageBackend;
import jakarta.enterprise.context.ApplicationScoped;
import org.eclipse.microprofile.config.inject.ConfigProperty;

import java.io.IOException;
import java.io.InputStream;
import java.io.UncheckedIOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardCopyOption;

@ApplicationScoped
@StorageType(StorageBackend.FILESYSTEM)
public class FilesystemArtifactStorage implements ArtifactStorage {

    @ConfigProperty(name = "fusion.index.storage.filesystem.root")
    String rootDir;

    @Override
    public String store(String suggestedPath, InputStream data, long sizeHint, String contentType) {
        Path target = resolvedPath(suggestedPath);
        try {
            Files.createDirectories(target.getParent());
            Files.copy(data, target, StandardCopyOption.REPLACE_EXISTING);
        } catch (IOException e) {
            throw new UncheckedIOException("Failed to store artifact at " + target, e);
        }
        return suggestedPath;
    }

    @Override
    public InputStream retrieve(String storagePath) {
        Path target = resolvedPath(storagePath);
        try {
            return Files.newInputStream(target);
        } catch (IOException e) {
            throw new UncheckedIOException("Failed to retrieve artifact at " + target, e);
        }
    }

    @Override
    public void delete(String storagePath) {
        try {
            Files.deleteIfExists(resolvedPath(storagePath));
        } catch (IOException e) {
            throw new UncheckedIOException("Failed to delete artifact at " + storagePath, e);
        }
    }

    @Override
    public StorageBackend getBackend() {
        return StorageBackend.FILESYSTEM;
    }

    private Path resolvedPath(String storagePath) {
        return Path.of(rootDir).resolve(storagePath).normalize();
    }
}
