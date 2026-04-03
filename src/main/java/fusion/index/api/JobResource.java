package fusion.index.api;

import fusion.index.api.dto.*;
import fusion.index.api.mapper.JobMapper;
import fusion.index.registry.service.JobService;
import jakarta.inject.Inject;
import jakarta.validation.Valid;
import jakarta.ws.rs.*;
import jakarta.ws.rs.core.MediaType;
import jakarta.ws.rs.core.Response;
import org.eclipse.microprofile.openapi.annotations.Operation;
import org.eclipse.microprofile.openapi.annotations.tags.Tag;

import java.util.List;
import java.util.stream.Collectors;

@Path("/api/v1/jobs")
@Produces(MediaType.APPLICATION_JSON)
@Consumes(MediaType.APPLICATION_JSON)
@Tag(name = "Jobs")
public class JobResource {

    @Inject JobService jobService;
    @Inject JobMapper  mapper;

    @GET
    @Operation(summary = "List jobs")
    public PageResponse<JobResponse> list(
            @QueryParam("page")     @DefaultValue("0")  int page,
            @QueryParam("pageSize") @DefaultValue("20") int pageSize) {
        List<JobResponse> items = jobService.listAll(page, pageSize).stream()
            .map(mapper::toResponse)
            .collect(Collectors.toList());
        return new PageResponse<>(items, jobService.countAll(), page, pageSize);
    }

    @POST
    @Operation(summary = "Create a job (also publishes version 1)")
    public Response create(@Valid CreateJobRequest req) {
        var job = jobService.create(req.name, req.description, req.templateVersionId,
                                    req.dockerImage, req.gitUrl, req.gitRef,
                                    req.gitSubpath, req.runConfig, req.changelog);
        return Response.status(Response.Status.CREATED)
            .entity(mapper.toResponse(job))
            .build();
    }

    @GET
    @Path("/{id}")
    @Operation(summary = "Get a job by ID")
    public JobResponse get(@PathParam("id") long id) {
        return mapper.toResponse(jobService.findById(id));
    }

    @PUT
    @Path("/{id}")
    @Operation(summary = "Update a job")
    public JobResponse update(@PathParam("id") long id,
                              @Valid UpdateJobRequest req) {
        return mapper.toResponse(jobService.update(id, req.description));
    }

    @DELETE
    @Path("/{id}")
    @Operation(summary = "Delete a job")
    public Response delete(@PathParam("id") long id) {
        jobService.delete(id);
        return Response.noContent().build();
    }

    @GET
    @Path("/{id}/versions")
    @Operation(summary = "List all versions of a job")
    public List<JobVersionResponse> listVersions(@PathParam("id") long id) {
        return jobService.listVersions(id).stream()
            .map(mapper::toVersionResponse)
            .collect(Collectors.toList());
    }

    @POST
    @Path("/{id}/versions")
    @Operation(summary = "Publish a new job version")
    public Response publishVersion(@PathParam("id") long id,
                                   @Valid PublishJobVersionRequest req) {
        var version = jobService.publishVersion(id, req.templateVersionId, req.dockerImage,
                                                req.gitUrl, req.gitRef, req.gitSubpath,
                                                req.runConfig, req.changelog);
        return Response.status(Response.Status.CREATED)
            .entity(mapper.toVersionResponse(version))
            .build();
    }

    @GET
    @Path("/{id}/versions/{versionNumber}")
    @Operation(summary = "Get a specific job version")
    public JobVersionResponse getVersion(@PathParam("id") long id,
                                          @PathParam("versionNumber") int versionNumber) {
        return mapper.toVersionResponse(jobService.findVersion(id, versionNumber));
    }
}
