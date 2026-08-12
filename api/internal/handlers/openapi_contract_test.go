package handlers

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// This file exists because of a real escape, not as belt-and-braces.
//
// ⚠️ A multi-line `description:` block in openapi.yaml lost its indentation and
// BOTH copies became invalid YAML — and every gate passed anyway:
//
//	npm run sync:openapi        copies bytes; never parses
//	git diff --exit-code        compares the two copies; both equally broken
//	openspec validate --specs   validates the OpenSpec docs, not embedded YAML
//	go build / go vet / go test nothing parsed the file
//
// The spec is served to clients at /openapi.yaml and rendered by /docs, so a
// syntax error ships a contract no tooling can consume. Independent review
// caught it; nothing automated did. These tests parse the EMBEDDED copy — the
// bytes actually served — so the gap cannot reopen silently.

func loadEmbeddedSpec(t *testing.T) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := yaml.Unmarshal(embeddedOpenAPISpec, &doc); err != nil {
		t.Fatalf("embedded openapi.yaml is not valid YAML: %v", err)
	}
	return doc
}

// TestEmbeddedOpenAPIIsValidYAML is the guard that was missing.
func TestEmbeddedOpenAPIIsValidYAML(t *testing.T) {
	doc := loadEmbeddedSpec(t)

	if doc["openapi"] == nil {
		t.Error("spec has no `openapi` version key")
	}
	paths, ok := doc["paths"].(map[string]any)
	if !ok || len(paths) == 0 {
		t.Fatal("spec has no paths")
	}
}

// TestEmbeddedOpenAPIDescribesContextContract pins the parts this change added.
// Syntax validity alone would not catch a description block that swallowed the
// parameters into prose — which is exactly what the indentation bug did.
func TestEmbeddedOpenAPIDescribesContextContract(t *testing.T) {
	doc := loadEmbeddedSpec(t)

	paths := doc["paths"].(map[string]any)
	ctx, ok := paths["/v1/context"].(map[string]any)
	if !ok {
		t.Fatal("/v1/context missing from spec")
	}
	get, ok := ctx["get"].(map[string]any)
	if !ok {
		t.Fatal("/v1/context has no get operation")
	}

	// lat and lng must survive as real parameters, not prose.
	params, _ := get["parameters"].([]any)
	seen := map[string]bool{}
	for _, p := range params {
		if m, ok := p.(map[string]any); ok {
			if name, ok := m["name"].(string); ok {
				seen[name] = true
			}
		}
	}
	for _, want := range []string{"lat", "lng"} {
		if !seen[want] {
			t.Errorf("query parameter %q missing from /v1/context (found %v)", want, seen)
		}
	}

	// The 400 contract must be declared, since the handler returns one.
	responses, _ := get["responses"].(map[string]any)
	if _, ok := responses["400"]; !ok {
		t.Error("/v1/context does not declare a 400 response, but the handler returns one")
	}

	// location_source must be required, or clients may treat it as optional.
	schemas := doc["components"].(map[string]any)["schemas"].(map[string]any)
	ctxResp, ok := schemas["ContextResponse"].(map[string]any)
	if !ok {
		t.Fatal("ContextResponse schema missing")
	}
	required, _ := ctxResp["required"].([]any)
	found := false
	for _, r := range required {
		if s, ok := r.(string); ok && s == "location_source" {
			found = true
		}
	}
	if !found {
		t.Errorf("location_source is not in ContextResponse.required (got %v)", required)
	}
}
