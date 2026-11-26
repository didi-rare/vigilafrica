package models

import "time"

// Boundary levels a manifest entry can load (feature-continental-coverage §1).
const (
	// BoundaryLevelsADM1 loads named states (ADM1) and derives the country
	// outline (ADM0) as their union.
	BoundaryLevelsADM1 = "adm1"
	// BoundaryLevelsADM0 loads a country outline only, so events in a country
	// without named states are still named by country.
	BoundaryLevelsADM0 = "adm0"
)

// BoundaryUnit is one administrative unit parsed from a boundary source file,
// ready to be written to admin_boundaries.
type BoundaryUnit struct {
	// Name is the unit's display name after name_overrides; empty for an ADM0
	// outline (the country name is used).
	Name string
	// PCode is the source's stable P-code for the unit, when it has one.
	PCode *string
	// GeometryJSON is the unit's GeoJSON geometry object, verbatim from the
	// source file. It is made valid and coerced to a MultiPolygon in SQL.
	GeometryJSON string
}

// BoundaryReplacement is everything the boundary swap needs for one country.
type BoundaryReplacement struct {
	CountryCode string
	CountryName string
	// Levels is BoundaryLevelsADM1 or BoundaryLevelsADM0.
	Levels string
	Units  []BoundaryUnit
}

// BoundaryReplaceResult reports what the swap did.
type BoundaryReplaceResult struct {
	// Units is the number of rows inserted at the loaded level (ADM1 rows for an
	// adm1 entry, 1 for an adm0 outline).
	Units int
	// PreviousCountryNames are the distinct country_name values of the rows the
	// swap deleted. Events still carrying one of them are re-enriched even if
	// they lie outside the new box, so no event keeps a name from boundaries
	// that no longer exist.
	PreviousCountryNames []string
}

// BoundaryLoadRecord is the provenance written to boundary_sources when a
// country's boundaries load.
type BoundaryLoadRecord struct {
	CountryCode  string
	CountryName  string
	Levels       string
	Source       string
	ArchiveURL   string
	Member       *string
	MemberSHA256 string
	EntrySHA256  string
	Vintage      time.Time
	UnitCount    int
}

// BoundarySource is one boundary_sources row: the last successful load of a
// manifest country and, separately, its last failed attempt. A row can carry
// either or both; every field but CountryCode may be nil.
type BoundarySource struct {
	CountryCode       string
	CountryName       *string
	Levels            *string
	Source            *string
	ArchiveURL        *string
	Member            *string
	MemberSHA256      *string
	EntrySHA256       *string
	Vintage           *time.Time
	UnitCount         *int
	LoadedAt          *time.Time
	ReenrichedAt      *time.Time
	FailedEntrySHA256 *string
	FailedAt          *time.Time
	LastError         *string
}

// BoundaryStatus is the /health `boundaries` object.
type BoundaryStatus struct {
	// Loaded counts manifest countries whose boundaries are loaded and whose
	// events were re-enriched (by the current entry or an earlier one).
	Loaded int `json:"loaded"`
	// Failed lists manifest countries whose CURRENT entry failed its last load
	// attempt. A country can be loaded (on an earlier entry) and failed.
	Failed []string `json:"failed"`
	// ManifestSHA256 identifies the embedded manifest the server was built with.
	ManifestSHA256 string `json:"manifest_sha256"`
}
