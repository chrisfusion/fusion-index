package fusion.index.api;

import io.quarkus.test.junit.QuarkusTest;
import io.restassured.http.ContentType;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import static io.restassured.RestAssured.given;
import static org.hamcrest.Matchers.*;

@QuarkusTest
class JobResourceTest {

    private int templateVersionId;

    @BeforeEach
    void createTemplate() {
        int templateId = given()
            .contentType(ContentType.JSON)
            .body("""
                {
                  "name": "job-test-template-%d",
                  "dockerImage": "registry.example.com/base:1.0"
                }
                """.formatted(System.nanoTime()))
            .when().post("/api/v1/templates")
            .then().statusCode(201)
            .extract().path("id");

        templateVersionId = given()
            .when().get("/api/v1/templates/" + templateId + "/versions/1")
            .then().statusCode(200)
            .extract().path("id");
    }

    @Test
    void createAndGetJob() {
        int jobId = given()
            .contentType(ContentType.JSON)
            .body("""
                {
                  "name": "my-etl-job-%d",
                  "templateVersionId": %d,
                  "dockerImage": "registry.example.com/etl:1.0",
                  "gitUrl": "https://github.com/org/repo.git",
                  "gitRef": "main"
                }
                """.formatted(System.nanoTime(), templateVersionId))
            .when().post("/api/v1/jobs")
            .then()
            .statusCode(201)
            .body("latestVersionNumber", equalTo(1))
            .extract().path("id");

        given()
            .when().get("/api/v1/jobs/" + jobId)
            .then()
            .statusCode(200)
            .body("id", equalTo(jobId));
    }

    @Test
    void listJobs() {
        given()
            .when().get("/api/v1/jobs")
            .then()
            .statusCode(200)
            .body("items", notNullValue());
    }

    @Test
    void duplicateJobNameReturns409() {
        String name = "dup-job-" + System.nanoTime();
        String body = """
            {
              "name": "%s",
              "templateVersionId": %d,
              "dockerImage": "registry.example.com/etl:1.0",
              "gitUrl": "https://github.com/org/repo.git",
              "gitRef": "main"
            }
            """.formatted(name, templateVersionId);

        given().contentType(ContentType.JSON).body(body).when().post("/api/v1/jobs").then().statusCode(201);
        given().contentType(ContentType.JSON).body(body).when().post("/api/v1/jobs").then().statusCode(409);
    }

    @Test
    void invalidTemplateVersionReturns404() {
        given()
            .contentType(ContentType.JSON)
            .body("""
                {
                  "name": "bad-template-job-%d",
                  "templateVersionId": 999999,
                  "dockerImage": "registry.example.com/etl:1.0",
                  "gitUrl": "https://github.com/org/repo.git",
                  "gitRef": "main"
                }
                """.formatted(System.nanoTime()))
            .when().post("/api/v1/jobs")
            .then()
            .statusCode(404);
    }

    @Test
    void publishJobVersion() {
        int jobId = given()
            .contentType(ContentType.JSON)
            .body("""
                {
                  "name": "versioned-job-%d",
                  "templateVersionId": %d,
                  "dockerImage": "registry.example.com/etl:1.0",
                  "gitUrl": "https://github.com/org/repo.git",
                  "gitRef": "main"
                }
                """.formatted(System.nanoTime(), templateVersionId))
            .when().post("/api/v1/jobs")
            .then().statusCode(201)
            .extract().path("id");

        given()
            .contentType(ContentType.JSON)
            .body("""
                {
                  "templateVersionId": %d,
                  "dockerImage": "registry.example.com/etl:2.0",
                  "gitUrl": "https://github.com/org/repo.git",
                  "gitRef": "v2.0"
                }
                """.formatted(templateVersionId))
            .when().post("/api/v1/jobs/" + jobId + "/versions")
            .then()
            .statusCode(201)
            .body("versionNumber", equalTo(2));

        given()
            .when().get("/api/v1/jobs/" + jobId + "/versions")
            .then()
            .statusCode(200)
            .body("size()", equalTo(2));
    }

    @Test
    void notFoundReturns404() {
        given().when().get("/api/v1/jobs/999999").then().statusCode(404);
    }
}
