package normalizer

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"vigilafrica/api/internal/models"
)

var allowedSourceURLHostSuffixes = []string{
	"nasa.gov",
	"gdacs.org",
	"usgs.gov",
	"noaa.gov",
	"copernicus.eu",
	"europa.eu",
}

// RawEONETEvent structures the incoming JSON from NASA's API.
type RawEONETEvent struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
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

func parseGeometryDate(rawDate string) (time.Time, bool) {
	if rawDate == "" {
		return time.Time{}, false
	}

	parsed, err := time.Parse(time.RFC3339, rawDate)
	if err != nil {
		return time.Time{}, false
	}

	return parsed, true
}

func selectMostRecentGeometry(geometries []RawGeometry) RawGeometry {
	selected := geometries[len(geometries)-1]
	selectedDate, hasSelectedDate := parseGeometryDate(selected.Date)

	for _, geom := range geometries[:len(geometries)-1] {
		geomDate, ok := parseGeometryDate(geom.Date)
		if !ok {
			continue
		}

		if !hasSelectedDate || geomDate.After(selectedDate) {
			selected = geom
			selectedDate = geomDate
			hasSelectedDate = true
		}
	}

	return selected
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

	// Determine source URL if available. Upstream URLs are treated as untrusted
	// data; only HTTPS links from known public data providers are retained.
	if len(raw.Sources) > 0 {
		if sourceURL, ok := validatedSourceURL(raw.Sources[0].URL); ok {
			evt.SourceURL = &sourceURL
		}
	}

	geoJSON := ""
	// Handle Geometry. Prefer the geometry with the most recent timestamp, falling
	// back to the last snapshot when upstream dates are missing or malformed.
	if len(raw.Geometries) > 0 {
		geom := selectMostRecentGeometry(raw.Geometries)
		evt.GeomType = &geom.Type

		if parsedDate, ok := parseGeometryDate(geom.Date); ok {
			evt.EventDate = &parsedDate
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

func validatedSourceURL(rawURL string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return "", false
	}
	host := strings.ToLower(parsed.Hostname())
	for _, suffix := range allowedSourceURLHostSuffixes {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return parsed.String(), true
		}
	}
	return "", false
}
