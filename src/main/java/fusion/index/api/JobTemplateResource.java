package fusion.index.api;

import fusion.index.api.dto.*;
import fusion.index.api.mapper.JobTemplateMapper;
import fusion.index.registry.service.JobTemplateService;
import jakarta.inject.Inject;
import jakarta.validation.Valid;
import jakarta.ws.rs.*;
import jakarta.ws.rs.core.MediaType;
import jakarta.ws.rs.core.Response;
import org.eclipse.microprofile.openapi.annotations.Operation;
import org.eclipse.microprofile.openapi.annotations.tags.Tag;

import java.util.List;
import java.util.stream.Collectors;

@Path("/api/v1/templates")
@Produces(MediaType.APPLICATION_JSON)
@Consumes(MediaType.APPLICATION_JSON)
@Tag(name = "Job Templates")
public class JobTemplateResource {

    @Inject JobTemplateService service;
    @Inject JobTemplateMapper  mapper;

    @GET
    @Operation(summary = "List job templates")
    public PageResponse<JobTemplateResponse> list(
            @QueryParam("page")     @DefaultValue("0")  int page,
            @QueryParam("pageSize") @DefaultValue("20") int pageSize) {
        List<JobTemplateResponse> items = service.listAll(page, pageSize).stream()
            .map(mapper::toResponse)
            .collect(Collectors.toList());
        return new PageResponse<>(items, service.countAll(), page, pageSize);
    }

    @POST
    @Operation(summary = "Create a job template (also publishes version 1)")
    public Response create(@Valid CreateJobTemplateRequest req) {
        var template = service.create(req.name, req.description, req.dockerImage,
                                      req.defaultRunConfig, req.changelog);
        return Response.status(Response.Status.CREATED)
            .entity(mapper.toResponse(template))
            .build();
    }

    @GET
    @Path("/{id}")
    @Operation(summary = "Get a job template by ID")
    public JobTemplateResponse get(@PathParam("id") long id) {
        return mapper.toResponse(service.findById(id));
    }

    @PUT
    @Path("/{id}")
    @Operation(summary = "Update a job template")
    public JobTemplateResponse update(@PathParam("id") long id,
                                      @Valid UpdateJobTemplateRequest req) {
        return mapper.toResponse(service.update(id, req.description, req.dockerImage));
    }

    @DELETE
    @Path("/{id}")
    @Operation(summary = "Delete a job template")
    public Response delete(@PathParam("id") long id) {
        service.delete(id);
        return Response.noContent().build();
    }

    @GET
    @Path("/{id}/versions")
    @Operation(summary = "List all versions of a template")
    public List<JobTemplateVersionResponse> listVersions(@PathParam("id") long id) {
        return service.listVersions(id).stream()
            .map(mapper::toVersionResponse)
            .collect(Collectors.toList());
    }

    @POST
    @Path("/{id}/versions")
    @Operation(summary = "Publish a new template version")
    public Response publishVersion(@PathParam("id") long id,
                                   @Valid PublishTemplateVersionRequest req) {
        var version = service.publishVersion(id, req.dockerImage, req.defaultRunConfig, req.changelog);
        return Response.status(Response.Status.CREATED)
            .entity(mapper.toVersionResponse(version))
            .build();
    }

    @GET
    @Path("/{id}/versions/{versionNumber}")
    @Operation(summary = "Get a specific template version")
    public JobTemplateVersionResponse getVersion(@PathParam("id") long id,
                                                  @PathParam("versionNumber") int versionNumber) {
        return mapper.toVersionResponse(service.findVersion(id, versionNumber));
    }
}
