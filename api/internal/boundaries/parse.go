package boundaries

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"vigilafrica/api/internal/models"
)

// wgs84CRSRe accepts the CRS names GeoJSON files use for WGS84 lon/lat. HDX
// writes urn:ogc:def:crs:OGC:1.3:CRS84; a file with no crs member is WGS84 by
// RFC 7946.
var wgs84CRSRe = regexp.MustCompile(`(CRS84|EPSG::?4326)$`)

type featureCollection struct {
	Type string `json:"type"`
	CRS  *struct {
		Properties struct {
			Name string `json:"name"`
		} `json:"properties"`
	} `json:"crs"`
	Features []struct {
		Properties map[string]any  `json:"properties"`
		Geometry   json.RawMessage `json:"geometry"`
	} `json:"features"`
}

// parseUnits turns a verified member into rows for admin_boundaries: for an
// adm1 entry one unit per feature, named from name_field with name_overrides
// applied and keyed by pcode_field; for an adm0 entry the features' geometries
// (unioned in SQL into one outline).
func parseUnits(data []byte, e Entry, log *slog.Logger) ([]models.BoundaryUnit, error) {
	var fc featureCollection
	if err := json.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadGeoJSON, err)
	}
	if fc.Type != "FeatureCollection" {
		return nil, fmt.Errorf("%w: type %q", ErrBadGeoJSON, fc.Type)
	}
	if fc.CRS != nil && fc.CRS.Properties.Name != "" && !wgs84CRSRe.MatchString(fc.CRS.Properties.Name) {
		return nil, fmt.Errorf("%w: CRS %q is not WGS84 lon/lat", ErrBadGeoJSON, fc.CRS.Properties.Name)
	}
	if len(fc.Features) == 0 {
		return nil, ErrNoFeatures
	}

	usedOverride := map[string]bool{}
	units := make([]models.BoundaryUnit, 0, len(fc.Features))
	for i, f := range fc.Features {
		var g struct {
			Type string `json:"type"`
		}
		if len(f.Geometry) == 0 || string(f.Geometry) == "null" {
			return nil, fmt.Errorf("%w: feature %d has no geometry", ErrBadGeoJSON, i)
		}
		if err := json.Unmarshal(f.Geometry, &g); err != nil {
			return nil, fmt.Errorf("%w: feature %d geometry: %w", ErrBadGeoJSON, i, err)
		}
		if g.Type != "Polygon" && g.Type != "MultiPolygon" {
			return nil, fmt.Errorf("%w: feature %d geometry is %q, want Polygon or MultiPolygon", ErrBadGeoJSON, i, g.Type)
		}

		u := models.BoundaryUnit{GeometryJSON: string(f.Geometry)}
		if e.Levels == models.BoundaryLevelsADM1 {
			name, _ := f.Properties[e.NameField].(string)
			name = strings.TrimSpace(name)
			if name == "" {
				return nil, fmt.Errorf("%w: feature %d has no %q", ErrFeatureWithoutName, i, e.NameField)
			}
			if fixed, ok := e.NameOverrides[name]; ok {
				usedOverride[name] = true
				name = fixed
			}
			u.Name = name
			if e.PCodeField != "" {
				if p, ok := f.Properties[e.PCodeField].(string); ok && strings.TrimSpace(p) != "" {
					p = strings.TrimSpace(p)
					u.PCode = &p
				}
			}
		}
		units = append(units, u)
	}

	for from := range e.NameOverrides {
		if !usedOverride[from] {
			log.Warn("name override matched no feature; the source may have fixed it", "override_from", from)
		}
	}
	return units, nil
}
