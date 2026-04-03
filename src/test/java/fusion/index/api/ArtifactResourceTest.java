package fusion.index.api;

import io.quarkus.test.junit.QuarkusTest;
import io.restassured.http.ContentType;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.io.File;
import java.io.IOException;
import java.nio.file.Files;

import static io.restassured.RestAssured.given;
import static org.hamcrest.Matchers.*;

@QuarkusTest
class ArtifactResourceTest {

    private int jobId;
    private int versionNumber = 1;

    @BeforeEach
    void createJobWithVersion() {
        int templateId = given()
            .contentType(ContentType.JSON)
            .body("""
                {
                  "name": "artifact-test-template-%d",
                  "dockerImage": "registry.example.com/base:1.0"
                }
                """.formatted(System.nanoTime()))
            .when().post("/api/v1/templates")
            .then().statusCode(201)
            .extract().path("id");

        int tvId = given()
            .when().get("/api/v1/templates/" + templateId + "/versions/1")
            .then().statusCode(200)
            .extract().path("id");

        jobId = given()
            .contentType(ContentType.JSON)
            .body("""
                {
                  "name": "artifact-test-job-%d",
                  "templateVersionId": %d,
                  "dockerImage": "registry.example.com/etl:1.0",
                  "gitUrl": "https://github.com/org/repo.git",
                  "gitRef": "main"
                }
                """.formatted(System.nanoTime(), tvId))
            .when().post("/api/v1/jobs")
            .then().statusCode(201)
            .extract().path("id");
    }

    @Test
    void uploadAndDownloadArtifact() throws IOException {
        File tempFile = File.createTempFile("artifact-test", ".bin");
        tempFile.deleteOnExit();
        Files.write(tempFile.toPath(), "hello artifact content".getBytes());

        int artifactId = given()
            .multiPart("file", tempFile, "application/octet-stream")
            .when().post("/api/v1/jobs/" + jobId + "/versions/" + versionNumber + "/artifacts")
            .then()
            .statusCode(201)
            .body("status", equalTo("AVAILABLE"))
            .body("name", equalTo(tempFile.getName()))
            .extract().path("id");

        given()
            .when().get("/api/v1/artifacts/" + artifactId + "/download")
            .then()
            .statusCode(200)
            .header("Content-Disposition", containsString("attachment"));
    }

    @Test
    void listArtifacts() {
        given()
            .when().get("/api/v1/jobs/" + jobId + "/versions/" + versionNumber + "/artifacts")
            .then()
            .statusCode(200)
            .body("$", instanceOf(java.util.List.class));
    }

    @Test
    void notFoundArtifactReturns404() {
        given().when().get("/api/v1/artifacts/999999").then().statusCode(404);
    }

    @Test
    void deleteArtifact() throws IOException {
        File tempFile = File.createTempFile("delete-test", ".bin");
        tempFile.deleteOnExit();
        Files.write(tempFile.toPath(), "to be deleted".getBytes());

        int artifactId = given()
            .multiPart("file", tempFile, "application/octet-stream")
            .when().post("/api/v1/jobs/" + jobId + "/versions/" + versionNumber + "/artifacts")
            .then().statusCode(201)
            .extract().path("id");

        given().when().delete("/api/v1/artifacts/" + artifactId).then().statusCode(204);
        given().when().get("/api/v1/artifacts/" + artifactId).then().statusCode(404);
    }
}
