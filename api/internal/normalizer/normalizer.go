package normalizer

import (
	"encoding/json"
	"fmt"
	"time"

	"vigilafrica/api/internal/models"
)

// RawEONETEvent structures the incoming JSON from NASA's API.
type RawEONETEvent struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Closed     *string `json:"closed"`
	Categories []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"categories"`
	Sources []struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	} `json:"sources"`
	Geometries []RawGeometry `json:"geometry"`
}

type RawGeometry struct {
	Date        string        `json:"date"`
	Type        string        `json:"type"`
	Coordinates []interface{} `json:"coordinates"` // Usually [lon, lat] or nested for polygons
}

// Normalize takes a raw API payload and transforms it into the canonical models.Event.
// It returns the transformed Event and a GeoJSON string representing the geometry, or an error.
func Normalize(raw RawEONETEvent, rawPayload []byte) (models.Event, string, error) {
	evt := models.Event{
		SourceID:   raw.ID,
		Source:     "eonet",
		Title:      raw.Title,
		RawPayload: rawPayload,
		IngestedAt: time.Now().UTC(),
	}

	// Determine category (defaulting to floods if unknown but this shouldn't happen with strict API filtering)
	evt.Category = models.CategoryFloods
	for _, c := range raw.Categories {
		if c.ID == "wildfires" {
			evt.Category = models.CategoryWildfires
			break
		}
	}

	// Determine status based on the "closed" field
	if raw.Closed != nil && *raw.Closed != "" {
		evt.Status = models.StatusClosed
	} else {
		evt.Status = models.StatusOpen
	}

	// Determine source URL if available
	if len(raw.Sources) > 0 {
		url := raw.Sources[0].URL
		evt.SourceURL = &url
	}

	geoJSON := ""
	// Handle Geometry. We take the most recent geometry (last in the array usually, or first if only one).
	// For simplicity, we just use the first geometry block provided.
	if len(raw.Geometries) > 0 {
		geom := raw.Geometries[0]
		evt.GeomType = &geom.Type

		if geom.Date != "" {
			if t, err := time.Parse(time.RFC3339, geom.Date); err == nil {
				evt.EventDate = &t
			}
		}

		// If it's a Point, coordinates are [lon, lat]
		if geom.Type == "Point" && len(geom.Coordinates) == 2 {
			lon, ok1 := geom.Coordinates[0].(float64)
			lat, ok2 := geom.Coordinates[1].(float64)
			if ok1 && ok2 {
				evt.Longitude = &lon
				evt.Latitude = &lat
				// Construct simple GeoJSON
				geoJSON = fmt.Sprintf(`{"type":"Point","coordinates":[%f,%f]}`, lon, lat)
			}
		} else if geom.Type == "Polygon" {
			// Construct GeoJSON from the raw coordinates interface array
			coordsBytes, _ := json.Marshal(geom.Coordinates)
			geoJSON = fmt.Sprintf(`{"type":"Polygon","coordinates":%s}`, string(coordsBytes))
			// Extract centroid? Deferring complex GIS parsing to DB, leaving lon/lat nil for polygons now.
		}
	}

	return evt, geoJSON, nil
}
