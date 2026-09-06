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

const (
	maxNormalizedTitleRunes = 512
	maxSourceURLLength      = 2048
)

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
		Title:      truncateRunes(raw.Title, maxNormalizedTitleRunes),
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
			if ok1 && ok2 && validLonLat(lon, lat) {
				evt.Longitude = &lon
				evt.Latitude = &lat
				// Construct simple GeoJSON
				geoJSON = fmt.Sprintf(`{"type":"Point","coordinates":[%f,%f]}`, lon, lat)
			}
		} else if geom.Type == "Polygon" {
			// ⚠️ Reject a polygon whose coordinates cannot be [lon, lat] at all.
			//
			// EONET republishes GDACS polygons with the pair order reversed, so the
			// "latitude" is really a longitude. Where that longitude exceeds 90 the
			// result is not merely wrong, it is impossible — 14 of 40 sampled flood
			// events declared latitudes such as 136.77 (Japan) or 168.12 (New
			// Zealand). See openspec/changes/fix-eonet-polygon-transposition.
			//
			// This is a definitional constraint, not a heuristic, so it can never
			// reject valid data. It does NOT catch every transposed polygon —
			// anything whose true longitude is within +/-90 stays plausible-looking,
			// which is exactly why the ingestor resolves GDACS geometry from GDACS
			// rather than relying on this guard alone.
			if !polygonCoordinatesPlausible(geom.Coordinates) {
				return evt, "", nil
			}
			// Construct GeoJSON from the raw coordinates interface array
			coordsBytes, _ := json.Marshal(geom.Coordinates)
			geoJSON = fmt.Sprintf(`{"type":"Polygon","coordinates":%s}`, string(coordsBytes))
			// lon/lat stay nil here. The ingestor resolves an authoritative point for
			// GDACS-sourced polygons; until it does, containment is unverifiable.
		}
	}

	return evt, geoJSON, nil
}

// validLonLat applies the definitional bounds of a WGS-84 coordinate pair.
// Anything outside them is not a coordinate, whatever the feed claims.
func validLonLat(lon, lat float64) bool {
	return lon >= -180 && lon <= 180 && lat >= -90 && lat <= 90
}

// polygonCoordinatesPlausible walks every vertex of an arbitrarily nested GeoJSON
// coordinate array and reports whether all of them could be [lon, lat] pairs.
//
// Every vertex is checked rather than the first, because a partially corrupted
// ring would be harder to spot than a uniformly transposed one and must not slip
// through on the strength of a valid opening vertex.
func polygonCoordinatesPlausible(coords []interface{}) bool {
	valid := true
	var walk func(v interface{})
	walk = func(v interface{}) {
		if !valid {
			return
		}
		arr, ok := v.([]interface{})
		if !ok || len(arr) == 0 {
			return
		}
		// A coordinate pair is a flat array whose first element is a number.
		if first, isNum := arr[0].(float64); isNum {
			if len(arr) < 2 {
				valid = false
				return
			}
			second, isNum2 := arr[1].(float64)
			if !isNum2 || !validLonLat(first, second) {
				valid = false
			}
			return
		}
		for _, item := range arr {
			walk(item)
		}
	}
	walk(coords)
	return valid
}

func validatedSourceURL(rawURL string) (string, bool) {
	if len(rawURL) > maxSourceURLLength {
		return "", false
	}
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

func truncateRunes(value string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	for index := range value {
		if maxRunes == 0 {
			return value[:index]
		}
		maxRunes--
	}
	return value
}
