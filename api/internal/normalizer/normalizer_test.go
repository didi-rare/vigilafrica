package normalizer_test

import (
	"encoding/json"
	"testing"
	"time"

	"vigilafrica/api/internal/models"
	"vigilafrica/api/internal/normalizer"
)

func TestNormalizeEvent_PointGeometry(t *testing.T) {
	rawPayload := []byte(`{
		"id": "EONET_123",
		"title": "Severe Flooding along River XYZ",
		"categories": [
			{ "id": "floods", "title": "Floods" }
		],
		"sources": [
			{ "id": "NASA", "url": "https://eonet.gsfc.nasa.gov/source" }
		],
		"geometry": [
			{
				"magnitudeValue": null,
				"magnitudeUnit": null,
				"date": "2026-04-10T12:00:00Z",
				"type": "Point",
				"coordinates": [8.13, 7.33]
			}
		],
		"closed": null
	}`)

	var rawEvent normalizer.RawEONETEvent
	if err := json.Unmarshal(rawPayload, &rawEvent); err != nil {
		t.Fatalf("Failed to unmarshal raw payload: %v", err)
	}

	event, geoJSON, err := normalizer.Normalize(rawEvent, rawPayload)
	if err != nil {
		t.Fatalf("Normalize() returned error: %v", err)
	}

	if event.SourceID != "EONET_123" {
		t.Errorf("Expected SourceID 'EONET_123', got %s", event.SourceID)
	}
	if event.Category != models.CategoryFloods {
		t.Errorf("Expected Category floods, got %s", event.Category)
	}
	if event.Status != models.StatusOpen {
		t.Errorf("Expected Status open, got %s", event.Status)
	}
	if event.GeomType == nil || *event.GeomType != "Point" {
		t.Errorf("Expected GeomType Point, got %v", event.GeomType)
	}
	if event.Longitude == nil || *event.Longitude != 8.13 {
		t.Errorf("Expected Longitude 8.13, got %v", event.Longitude)
	}
	if event.Latitude == nil || *event.Latitude != 7.33 {
		t.Errorf("Expected Latitude 7.33, got %v", event.Latitude)
	}
	if event.SourceURL == nil || *event.SourceURL != "https://eonet.gsfc.nasa.gov/source" {
		t.Errorf("Expected SourceURL https://eonet.gsfc.nasa.gov/source, got %v", event.SourceURL)
	}

	// Verify GeoJSON structure generically (fields already checked numerically above)
	if geoJSON == "" {
		t.Error("Expected non-empty GeoJSON string")
	}
}

func TestNormalizeEvent_BlocksUnsafeSourceURLs(t *testing.T) {
	tests := []struct {
		name      string
		sourceURL string
	}{
		{name: "javascript scheme", sourceURL: "javascript:alert(1)"},
		{name: "data scheme", sourceURL: "data:text/html,<script>alert(1)</script>"},
		{name: "non allowlisted domain", sourceURL: "https://evil.example/source"},
		{name: "http scheme", sourceURL: "http://eonet.gsfc.nasa.gov/source"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rawPayload := []byte(`{
				"id": "EONET_UNSAFE",
				"title": "Unsafe source URL",
				"categories": [{ "id": "floods", "title": "Floods" }],
				"sources": [{ "id": "BAD", "url": "` + tt.sourceURL + `" }],
				"geometry": [
					{ "type": "Point", "coordinates": [8.13, 7.33], "date": "2026-04-10T12:00:00Z" }
				]
			}`)

			var rawEvent normalizer.RawEONETEvent
			if err := json.Unmarshal(rawPayload, &rawEvent); err != nil {
				t.Fatalf("Failed to unmarshal raw payload: %v", err)
			}

			event, _, err := normalizer.Normalize(rawEvent, rawPayload)
			if err != nil {
				t.Fatalf("Normalize() returned error: %v", err)
			}
			if event.SourceURL != nil {
				t.Fatalf("expected unsafe source URL to be blocked, got %q", *event.SourceURL)
			}
		})
	}
}

func TestNormalizeEvent_ClosedStatus(t *testing.T) {
	rawPayload := []byte(`{
		"id": "EONET_456",
		"title": "Old Wildfire",
		"categories": [{ "id": "wildfires", "title": "Wildfires" }],
		"geometry": [
			{ "type": "Point", "coordinates": [0.0, 0.0], "date": "2026-03-01T00:00:00Z" }
		],
		"closed": "2026-03-05T00:00:00Z"
	}`)

	var rawEvent normalizer.RawEONETEvent
	json.Unmarshal(rawPayload, &rawEvent)

	event, _, err := normalizer.Normalize(rawEvent, rawPayload)
	if err != nil {
		t.Fatalf("Normalize() returned error: %v", err)
	}

	if event.Status != models.StatusClosed {
		t.Errorf("Expected Status closed, got %s", event.Status)
	}
}

func TestNormalizeEvent_MissingGeometry(t *testing.T) {
	rawPayload := []byte(`{
		"id": "EONET_789",
		"title": "No Geo Event",
		"categories": [{ "id": "floods" }],
		"geometry": []
	}`)

	var rawEvent normalizer.RawEONETEvent
	json.Unmarshal(rawPayload, &rawEvent)

	event, geoJSON, err := normalizer.Normalize(rawEvent, rawPayload)
	if err != nil {
		t.Fatalf("Normalize() returned error: %v", err)
	}

	if event.GeomType != nil {
		t.Errorf("Expected GeomType nil, got %v", *event.GeomType)
	}
	if geoJSON != "" {
		t.Errorf("Expected empty GeoJSON, got %s", geoJSON)
	}
}

func TestNormalizeEvent_UsesMostRecentGeometryByDate(t *testing.T) {
	rawPayload := []byte(`{
		"id": "EONET_999",
		"title": "Multi-snapshot flood event",
		"categories": [{ "id": "floods", "title": "Floods" }],
		"geometry": [
			{ "type": "Point", "coordinates": [1.0, 1.0], "date": "2026-04-01T00:00:00Z" },
			{ "type": "Point", "coordinates": [2.0, 2.0], "date": "2026-04-12T00:00:00Z" },
			{ "type": "Point", "coordinates": [3.0, 3.0], "date": "2026-04-05T00:00:00Z" }
		]
	}`)

	var rawEvent normalizer.RawEONETEvent
	if err := json.Unmarshal(rawPayload, &rawEvent); err != nil {
		t.Fatalf("Failed to unmarshal raw payload: %v", err)
	}

	event, _, err := normalizer.Normalize(rawEvent, rawPayload)
	if err != nil {
		t.Fatalf("Normalize() returned error: %v", err)
	}

	if event.Longitude == nil || *event.Longitude != 2.0 {
		t.Errorf("Expected Longitude 2.0 from most recent geometry, got %v", event.Longitude)
	}
	if event.Latitude == nil || *event.Latitude != 2.0 {
		t.Errorf("Expected Latitude 2.0 from most recent geometry, got %v", event.Latitude)
	}
	if event.EventDate == nil || !event.EventDate.Equal(mustParseTime(t, "2026-04-12T00:00:00Z")) {
		t.Errorf("Expected EventDate from most recent geometry, got %v", event.EventDate)
	}
}

func mustParseTime(t *testing.T, value string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("Failed to parse time %q: %v", value, err)
	}

	return parsed
}
