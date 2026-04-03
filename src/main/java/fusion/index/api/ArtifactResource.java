package fusion.index.api;

import fusion.index.api.dto.ArtifactResponse;
import fusion.index.api.mapper.ArtifactMapper;
import fusion.index.registry.entity.Artifact;
import fusion.index.registry.service.ArtifactService;
import jakarta.inject.Inject;
import jakarta.ws.rs.*;
import jakarta.ws.rs.core.MediaType;
import jakarta.ws.rs.core.Response;
import jakarta.ws.rs.core.StreamingOutput;
import org.eclipse.microprofile.openapi.annotations.Operation;
import org.eclipse.microprofile.openapi.annotations.tags.Tag;
import org.jboss.resteasy.reactive.multipart.FileUpload;
import org.jboss.resteasy.reactive.RestForm;

import java.io.IOException;
import java.io.InputStream;
import java.io.UncheckedIOException;
import java.nio.file.Files;
import java.util.List;
import java.util.stream.Collectors;

@Path("/api/v1/jobs")
@Tag(name = "Artifacts")
@Produces(MediaType.APPLICATION_JSON)
public class ArtifactResource {

    @Inject ArtifactService artifactService;
    @Inject ArtifactMapper  mapper;

    // -------------------------------------------------------------------------
    // Scoped under /api/v1/jobs/{jobId}/versions/{versionNumber}/artifacts
    // -------------------------------------------------------------------------

    @GET
    @Path("/{jobId}/versions/{versionNumber}/artifacts")
    @Operation(summary = "List artifacts for a job version")
    public List<ArtifactResponse> list(@PathParam("jobId") long jobId,
                                       @PathParam("versionNumber") int versionNumber) {
        return artifactService.listForJobVersion(jobId, versionNumber).stream()
            .map(mapper::toResponse)
            .collect(Collectors.toList());
    }

    @POST
    @Path("/{jobId}/versions/{versionNumber}/artifacts")
    @Consumes(MediaType.MULTIPART_FORM_DATA)
    @Operation(summary = "Upload an artifact for a job version")
    public Response upload(@PathParam("jobId") long jobId,
                           @PathParam("versionNumber") int versionNumber,
                           @RestForm("file") FileUpload file,
                           @RestForm("contentType") String contentType) {
        String resolvedContentType = contentType != null ? contentType
            : (file.contentType() != null ? file.contentType() : "application/octet-stream");

        Artifact artifact = artifactService.register(jobId, versionNumber,
                                                      file.fileName(), resolvedContentType);
        try {
            long size = Files.size(file.uploadedFile());
            try (InputStream is = Files.newInputStream(file.uploadedFile())) {
                artifactService.store(artifact.id, is, size);
            }
        } catch (IOException e) {
            throw new UncheckedIOException(e);
        }
        return Response.status(Response.Status.CREATED)
            .entity(mapper.toResponse(artifactService.findById(artifact.id)))
            .build();
    }
}
