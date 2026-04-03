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

import java.io.InputStream;

@Path("/api/v1/artifacts")
@Tag(name = "Artifacts")
@Produces(MediaType.APPLICATION_JSON)
public class ArtifactByIdResource {

    @Inject ArtifactService artifactService;
    @Inject ArtifactMapper  mapper;

    @GET
    @Path("/{id}")
    @Operation(summary = "Get artifact metadata")
    public ArtifactResponse get(@PathParam("id") long id) {
        return mapper.toResponse(artifactService.findById(id));
    }

    @GET
    @Path("/{id}/download")
    @Produces(MediaType.APPLICATION_OCTET_STREAM)
    @Operation(summary = "Download artifact content")
    public Response download(@PathParam("id") long id) {
        Artifact artifact = artifactService.findById(id);
        String filename   = artifact.name;
        String mime       = artifact.contentType != null ? artifact.contentType : "application/octet-stream";

        StreamingOutput stream = out -> {
            try (InputStream is = artifactService.retrieve(id)) {
                is.transferTo(out);
            }
        };
        return Response.ok(stream, mime)
            .header("Content-Disposition", "attachment; filename=\"" + filename + "\"")
            .build();
    }

    @DELETE
    @Path("/{id}")
    @Operation(summary = "Delete an artifact")
    public Response delete(@PathParam("id") long id) {
        artifactService.delete(id);
        return Response.noContent().build();
    }
}
