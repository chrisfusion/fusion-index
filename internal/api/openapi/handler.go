package openapi

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

//go:embed openapi.yaml
var specYAML []byte

var specJSON []byte

func init() {
	var doc any
	if err := yaml.Unmarshal(specYAML, &doc); err != nil {
		panic("openapi: failed to parse openapi.yaml: " + err.Error())
	}
	// yaml.v3 may produce map[any]any for non-string-keyed maps (e.g. bare integer
	// status-code keys). encoding/json cannot serialise map[any]any, so we
	// recursively normalise all map keys to strings before marshalling.
	b, err := json.Marshal(normaliseYAML(doc))
	if err != nil {
		panic("openapi: failed to marshal spec to JSON: " + err.Error())
	}
	specJSON = b
}

// normaliseYAML converts map[any]any produced by yaml.v3 into map[string]any
// so that encoding/json can serialise it without error.
func normaliseYAML(v any) any {
	switch m := v.(type) {
	case map[any]any:
		out := make(map[string]any, len(m))
		for k, val := range m {
			out[fmt.Sprintf("%v", k)] = normaliseYAML(val)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(m))
		for k, val := range m {
			out[k] = normaliseYAML(val)
		}
		return out
	case []any:
		for i, val := range m {
			m[i] = normaliseYAML(val)
		}
	}
	return v
}

// ServeSpec handles GET /api/openapi.json — returns the OpenAPI 3.1 spec as JSON.
func ServeSpec(c *gin.Context) {
	c.Data(http.StatusOK, "application/json", specJSON)
}

// swaggerUI is the embedded Swagger UI page. Swagger UI assets are loaded from CDN;
// the HTML itself is compiled into the binary.
const swaggerUI = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>fusion-index API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
<script>
  SwaggerUIBundle({
    url: "/api/openapi.json",
    dom_id: "#swagger-ui",
    deepLinking: true,
    presets: [SwaggerUIBundle.presets.apis],
    layout: "BaseLayout"
  });
</script>
</body>
</html>`

// ServeUI handles GET /swagger/ — returns the embedded Swagger UI page.
func ServeUI(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(swaggerUI))
}
