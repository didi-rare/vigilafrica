package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestScalarHTML_PointsToLocalSpec(t *testing.T) {
	html := scalarHTML("/openapi.yaml")

	if !strings.Contains(html, "@scalar/api-reference") {
		t.Fatal("expected Scalar CDN script in HTML")
	}
	if !strings.Contains(html, "/openapi.yaml") {
		t.Fatal("expected Scalar HTML to reference the local spec endpoint")
	}
}

func TestLoadOpenAPISpec_ReturnsEmbeddedSpec(t *testing.T) {
	spec, err := loadOpenAPISpec()
	if err != nil {
		t.Fatalf("expected embedded spec to load, got error: %v", err)
	}
	if len(spec) == 0 {
		t.Fatal("expected non-empty embedded spec")
	}
	if !strings.Contains(string(spec), "openapi:") {
		t.Fatal("expected embedded spec to contain openapi key")
	}
}

func TestOpenAPISpecHandler_ServesEmbeddedSpec(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil)
	rec := httptest.NewRecorder()

	OpenAPISpecHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/yaml") {
		t.Fatalf("expected yaml content type, got %q", ct)
	}
	if body := rec.Body.String(); !strings.Contains(body, "openapi:") {
		t.Fatalf("expected spec body, got %q", body)
	}
}
