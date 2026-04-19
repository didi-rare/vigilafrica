package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

var openAPISpecPaths = []string{
	filepath.Join("openspec", "specs", "vigilafrica", "openapi.yaml"),
	filepath.Join("..", "openspec", "specs", "vigilafrica", "openapi.yaml"),
	filepath.Join("..", "..", "openspec", "specs", "vigilafrica", "openapi.yaml"),
}

func loadOpenAPISpec() ([]byte, error) {
	for _, candidate := range openAPISpecPaths {
		data, err := os.ReadFile(candidate)
		if err == nil {
			return data, nil
		}
	}

	return nil, fmt.Errorf("openapi spec not found in known local paths")
}

func scalarHTML(specURL string) string {
	return fmt.Sprintf(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>VigilAfrica API Docs</title>
    <style>
      body { margin: 0; }
    </style>
  </head>
  <body>
    <script
      id="api-reference"
      data-url="%s"
      data-configuration='{
        "theme": "saturn",
        "darkMode": true,
        "defaultHttpClient": { "targetKey": "shell", "clientKey": "curl" },
        "defaultOpenAllTags": false,
        "metaData": {
          "title": "VigilAfrica API Docs",
          "description": "Enriched NASA EONET natural event data for Africa"
        },
        "customCss": ":root, .dark-mode { --scalar-color-accent: #38bdf8; --scalar-background-1: #050714; --scalar-background-2: #090b1f; --scalar-background-3: rgba(15,23,42,0.7); } .sidebar { border-right: 1px solid rgba(255,255,255,0.08); }"
      }'
    ></script>
    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
  </body>
</html>`, specURL)
}

func OpenAPISpecHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		spec, err := loadOpenAPISpec()
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "failed to load OpenAPI spec")
			return
		}

		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(spec)
	}
}

func SwaggerUIHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(scalarHTML("/openapi.yaml")))
	}
}
