package normalizer_test

import (
	"encoding/json"
	"testing"

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
			{ "id": "GDACS", "url": "http://example.com/source" }
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
	if event.SourceURL == nil || *event.SourceURL != "http://example.com/source" {
		t.Errorf("Expected SourceURL http://example.com/source, got %v", event.SourceURL)
	}

	expectedGeoJSON := `{"type":"Point","coordinates":[8.13,7.33]}`
	if geoJSON != expectedGeoJSON {
		t.Errorf("Expected GeoJSON %s, got %s", expectedGeoJSON, geoJSON)
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
