package fusion.index.registry.storage;

import fusion.index.registry.enums.StorageBackend;

import java.io.InputStream;

/**
 * Abstraction over artifact storage backends (filesystem, S3).
 * Implementations are CDI beans qualified with {@link StorageType}.
 */
public interface ArtifactStorage {

    /**
     * Persist the given data stream and return the resolved storage path/key.
     *
     * @param suggestedPath hint for the storage path (e.g. "jobVersionId/filename")
     * @param data          the artifact content
     * @param sizeHint      byte length, or -1 if unknown
     * @param contentType   MIME type, may be null
     * @return the resolved path that can later be passed to {@link #retrieve} and {@link #delete}
     */
    String store(String suggestedPath, InputStream data, long sizeHint, String contentType);

    /**
     * Open the stored artifact for streaming.
     *
     * @param storagePath the path returned by a previous {@link #store} call
     * @return an open InputStream; caller is responsible for closing it
     */
    InputStream retrieve(String storagePath);

    /**
     * Remove the stored artifact. Idempotent — no exception if already absent.
     *
     * @param storagePath the path returned by a previous {@link #store} call
     */
    void delete(String storagePath);

    /** The backend type this implementation serves. */
    StorageBackend getBackend();
}
