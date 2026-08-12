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
//
// ⚠️ Syntax validity is not enough, and the first version of this gate proved
// it: the handler had begun returning 500 while the contract declared only
// 200 and 400, and this test passed anyway. Independent review caught the lie
// the gate was written to catch. It now asserts that every status the handler
// can produce is declared, and that the parameters are real parameters with
// locations, types and bounds — not prose that happens to parse.
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

	// --- parameters must be structured, not prose ---
	wantParams := map[string]struct {
		min, max float64
	}{
		"lat": {-90, 90},
		"lng": {-180, 180},
	}
	params, _ := get["parameters"].([]any)
	seen := map[string]bool{}
	for _, raw := range params {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name, _ := m["name"].(string)
		want, tracked := wantParams[name]
		if !tracked {
			continue
		}
		seen[name] = true

		if in, _ := m["in"].(string); in != "query" {
			t.Errorf("parameter %q: in = %q, want \"query\"", name, in)
		}
		schema, ok := m["schema"].(map[string]any)
		if !ok {
			t.Errorf("parameter %q has no schema", name)
			continue
		}
		if typ, _ := schema["type"].(string); typ != "number" {
			t.Errorf("parameter %q: type = %q, want \"number\"", name, typ)
		}
		// Bounds must match the handler's validation, or the contract lies about
		// what is accepted.
		for key, want := range map[string]float64{"minimum": want.min, "maximum": want.max} {
			got, ok := toFloat(schema[key])
			if !ok {
				t.Errorf("parameter %q has no %s", name, key)
				continue
			}
			if got != want {
				t.Errorf("parameter %q: %s = %v, want %v (must match parseCoordinate)", name, key, got, want)
			}
		}
	}
	for name := range wantParams {
		if !seen[name] {
			t.Errorf("query parameter %q missing from /v1/context (found %v)", name, seen)
		}
	}

	// --- every status the handler can return must be declared ---
	//
	// Keep this list in step with GetContext. It currently returns:
	//   200 success
	//   400 invalid or non-finite coordinates
	//   500 response could not be encoded, or the nearby-events query failed
	responses, _ := get["responses"].(map[string]any)
	for _, code := range []string{"200", "400", "500"} {
		if _, ok := responses[code]; !ok {
			t.Errorf("/v1/context does not declare a %s response, but the handler returns one", code)
		}
	}

	// --- location_source must be required ---
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

	// The enum must cover every LocationSource constant, or a client switching
	// on it has an unhandled case.
	props := ctxResp["properties"].(map[string]any)
	ls, ok := props["location_source"].(map[string]any)
	if !ok {
		t.Fatal("ContextResponse.location_source property missing")
	}
	declared := map[string]bool{}
	for _, v := range ls["enum"].([]any) {
		if s, ok := v.(string); ok {
			declared[s] = true
		}
	}
	for _, want := range []LocationSource{
		LocationSourceExplicit, LocationSourceDevOverride,
		LocationSourceIP, LocationSourceUnavailable,
	} {
		if !declared[string(want)] {
			t.Errorf("location_source enum omits %q, which the handler can emit", want)
		}
	}
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case float64:
		return n, true
	}
	return 0, false
}
