package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestOpenAPISpecHandler_ServesSpecFile(t *testing.T) {
	originalPaths := openAPISpecPaths
	t.Cleanup(func() { openAPISpecPaths = originalPaths })

	tempDir := t.TempDir()
	specPath := filepath.Join(tempDir, "openapi.yaml")
	if err := os.WriteFile(specPath, []byte("openapi: 3.1.0\ninfo:\n  title: Test\n"), 0o644); err != nil {
		t.Fatalf("failed to write temp spec: %v", err)
	}
	openAPISpecPaths = []string{specPath}

	req := httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil)
	rec := httptest.NewRecorder()

	OpenAPISpecHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/yaml") {
		t.Fatalf("expected yaml content type, got %q", ct)
	}
	if body := rec.Body.String(); !strings.Contains(body, "openapi: 3.1.0") {
		t.Fatalf("expected served spec body, got %q", body)
	}
}
