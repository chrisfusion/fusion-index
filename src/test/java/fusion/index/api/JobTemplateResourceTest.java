package fusion.index.api;

import io.quarkus.test.junit.QuarkusTest;
import io.restassured.http.ContentType;
import org.junit.jupiter.api.Test;

import static io.restassured.RestAssured.given;
import static org.hamcrest.Matchers.*;

@QuarkusTest
class JobTemplateResourceTest {

    @Test
    void createAndGetTemplate() {
        int id = given()
            .contentType(ContentType.JSON)
            .body("""
                {
                  "name": "test-template-create",
                  "description": "A test template",
                  "dockerImage": "registry.example.com/spark:3.5",
                  "changelog": "Initial version"
                }
                """)
            .when().post("/api/v1/templates")
            .then()
            .statusCode(201)
            .body("name", equalTo("test-template-create"))
            .body("latestVersionNumber", equalTo(1))
            .extract().path("id");

        given()
            .when().get("/api/v1/templates/" + id)
            .then()
            .statusCode(200)
            .body("id", equalTo(id))
            .body("name", equalTo("test-template-create"));
    }

    @Test
    void listTemplates() {
        given()
            .when().get("/api/v1/templates")
            .then()
            .statusCode(200)
            .body("items", notNullValue())
            .body("total", greaterThanOrEqualTo(0));
    }

    @Test
    void duplicateNameReturns409() {
        String body = """
            {
              "name": "duplicate-template",
              "dockerImage": "registry.example.com/test:1.0"
            }
            """;

        given().contentType(ContentType.JSON).body(body)
            .when().post("/api/v1/templates")
            .then().statusCode(201);

        given().contentType(ContentType.JSON).body(body)
            .when().post("/api/v1/templates")
            .then().statusCode(409);
    }

    @Test
    void missingRequiredFieldReturns400() {
        given()
            .contentType(ContentType.JSON)
            .body("{\"description\": \"no image or name\"}")
            .when().post("/api/v1/templates")
            .then()
            .statusCode(400);
    }

    @Test
    void notFoundReturns404() {
        given()
            .when().get("/api/v1/templates/999999")
            .then()
            .statusCode(404);
    }

    @Test
    void publishVersionAndRetrieve() {
        int templateId = given()
            .contentType(ContentType.JSON)
            .body("""
                {
                  "name": "versioned-template",
                  "dockerImage": "registry.example.com/base:1.0"
                }
                """)
            .when().post("/api/v1/templates")
            .then().statusCode(201)
            .extract().path("id");

        given()
            .contentType(ContentType.JSON)
            .body("""
                {
                  "dockerImage": "registry.example.com/base:2.0",
                  "changelog": "Upgraded base image"
                }
                """)
            .when().post("/api/v1/templates/" + templateId + "/versions")
            .then()
            .statusCode(201)
            .body("versionNumber", equalTo(2));

        given()
            .when().get("/api/v1/templates/" + templateId + "/versions")
            .then()
            .statusCode(200)
            .body("size()", equalTo(2));

        given()
            .when().get("/api/v1/templates/" + templateId + "/versions/2")
            .then()
            .statusCode(200)
            .body("dockerImage", equalTo("registry.example.com/base:2.0"));
    }

    @Test
    void updateTemplate() {
        int id = given()
            .contentType(ContentType.JSON)
            .body("""
                {
                  "name": "update-me-template",
                  "dockerImage": "registry.example.com/base:1.0"
                }
                """)
            .when().post("/api/v1/templates")
            .then().statusCode(201)
            .extract().path("id");

        given()
            .contentType(ContentType.JSON)
            .body("{\"description\": \"updated description\"}")
            .when().put("/api/v1/templates/" + id)
            .then()
            .statusCode(200)
            .body("description", equalTo("updated description"));
    }

    @Test
    void deleteTemplate() {
        int id = given()
            .contentType(ContentType.JSON)
            .body("""
                {
                  "name": "delete-me-template",
                  "dockerImage": "registry.example.com/base:1.0"
                }
                """)
            .when().post("/api/v1/templates")
            .then().statusCode(201)
            .extract().path("id");

        given().when().delete("/api/v1/templates/" + id).then().statusCode(204);
        given().when().get("/api/v1/templates/" + id).then().statusCode(404);
    }
}
