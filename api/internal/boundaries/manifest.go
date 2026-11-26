// Package boundaries owns the country manifest and the boundary loader
// (feature-continental-coverage spec §1 and §2).
//
// The manifest (manifest.json, embedded at build time) is the single source of
// truth for covered countries. The loader downloads each entry's source file,
// verifies its checksum, and swaps the country's admin_boundaries rows in one
// transaction with the re-enrichment of its stored events.
package boundaries

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"vigilafrica/api/internal/models"
)

//go:embed manifest.json
var embeddedManifest []byte

// Source values a manifest entry may carry.
const (
	SourceHDXCODAB      = "hdx-cod-ab"
	SourceGeoBoundaries = "geoboundaries"
)

// AfricaRect is the rectangle every entry's bbox must lie inside,
// [min_lon, min_lat, max_lon, max_lat]. Longitude and the northern edge are the
// density scripts' Africa rectangle (scripts/eonet-density/measure.mjs). The
// southern edge is widened from -35 to -47.5 so a South African file that
// includes the Prince Edward Islands (about 46.9°S) still validates.
// The check exists to catch swapped or mistyped coordinates, not to define
// Africa.
var AfricaRect = [4]float64{-26.0, -47.5, 64.0, 38.0}

var (
	alpha2Re = regexp.MustCompile(`^[A-Z]{2}$`)
	sha256Re = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// Entry is one manifest country. Field meanings are in spec §1.
type Entry struct {
	Code          string            `json:"code"`
	Name          string            `json:"name"`
	Levels        string            `json:"levels"`
	Source        string            `json:"source"`
	Enabled       *bool             `json:"enabled"`
	ArchiveURL    string            `json:"archive_url"`
	Member        string            `json:"member,omitempty"`
	MemberSHA256  string            `json:"member_sha256"`
	Vintage       string            `json:"vintage"`
	NameField     string            `json:"name_field,omitempty"`
	PCodeField    string            `json:"pcode_field,omitempty"`
	NameOverrides map[string]string `json:"name_overrides,omitempty"`
	BBoxValues    []float64         `json:"bbox"`

	hash    string
	vintage time.Time
}

// Hash is the SHA-256 of the canonicalised entry: its JSON object with keys
// sorted, no insignificant whitespace, and `enabled` removed (enabling or
// disabling a country changes nothing the loader stores). Any other change —
// a name override, a field, the box, the member — changes the hash, and the
// loader reloads the country.
func (e Entry) Hash() string { return e.hash }

// IsEnabled reports whether the country is ingested and offered.
func (e Entry) IsEnabled() bool { return e.Enabled != nil && *e.Enabled }

// BBox returns the entry's [min_lon, min_lat, max_lon, max_lat].
func (e Entry) BBox() [4]float64 {
	var b [4]float64
	copy(b[:], e.BBoxValues)
	return b
}

// VintageDate is the parsed vintage.
func (e Entry) VintageDate() time.Time { return e.vintage }

// Manifest is the parsed, validated manifest.
type Manifest struct {
	SchemaVersion int
	Entries       []Entry
	checksum      string
}

// Checksum is the SHA-256 of the canonicalised manifest (every entry,
// `enabled` included, keys sorted, no whitespace). It identifies the manifest
// independently of line endings in a checkout.
func (m *Manifest) Checksum() string { return m.checksum }

// Entry returns the entry for code, if any.
func (m *Manifest) Entry(code string) (Entry, bool) {
	for _, e := range m.Entries {
		if e.Code == code {
			return e, true
		}
	}
	return Entry{}, false
}

// ErrInvalidManifest wraps every validation failure.
var ErrInvalidManifest = errors.New("invalid boundary manifest")

// Embedded parses and validates the manifest compiled into the binary.
func Embedded() (*Manifest, error) {
	return Parse(embeddedManifest)
}

type rawManifest struct {
	SchemaVersion int               `json:"schema_version"`
	Countries     []json.RawMessage `json:"countries"`
}

// Parse decodes and validates a manifest. Unknown fields are rejected, so a
// misspelt key fails here instead of being silently ignored.
func Parse(data []byte) (*Manifest, error) {
	var raw rawManifest
	if err := strictDecode(data, &raw); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidManifest, err)
	}
	if raw.SchemaVersion != 1 {
		return nil, fmt.Errorf("%w: schema_version %d, want 1", ErrInvalidManifest, raw.SchemaVersion)
	}

	m := &Manifest{SchemaVersion: raw.SchemaVersion}
	canonicalAll := make([]json.RawMessage, 0, len(raw.Countries))
	for i, rc := range raw.Countries {
		var e Entry
		if err := strictDecode(rc, &e); err != nil {
			return nil, fmt.Errorf("%w: entry %d: %w", ErrInvalidManifest, i, err)
		}
		withEnabled, withoutEnabled, err := canonicalEntry(rc)
		if err != nil {
			return nil, fmt.Errorf("%w: entry %d: %w", ErrInvalidManifest, i, err)
		}
		sum := sha256.Sum256(withoutEnabled)
		e.hash = hex.EncodeToString(sum[:])
		canonicalAll = append(canonicalAll, withEnabled)
		m.Entries = append(m.Entries, e)
	}
	if err := m.validate(); err != nil {
		return nil, err
	}

	all, err := json.Marshal(struct {
		SchemaVersion int               `json:"schema_version"`
		Countries     []json.RawMessage `json:"countries"`
	}{raw.SchemaVersion, canonicalAll})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidManifest, err)
	}
	sum := sha256.Sum256(all)
	m.checksum = hex.EncodeToString(sum[:])
	return m, nil
}

func strictDecode(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return errors.New("trailing data after JSON value")
	}
	return nil
}

// canonicalEntry re-encodes a raw entry with sorted keys (encoding/json sorts
// map keys, nested maps included) and numbers kept as written. It returns the
// encoding with and without `enabled`.
func canonicalEntry(raw json.RawMessage) (withEnabled, withoutEnabled []byte, err error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var obj map[string]any
	if err := dec.Decode(&obj); err != nil {
		return nil, nil, err
	}
	withEnabled, err = json.Marshal(obj)
	if err != nil {
		return nil, nil, err
	}
	delete(obj, "enabled")
	withoutEnabled, err = json.Marshal(obj)
	if err != nil {
		return nil, nil, err
	}
	return withEnabled, withoutEnabled, nil
}

// validate applies the spec §1 rules. It is also what the offline CI test runs
// against the embedded manifest.
func (m *Manifest) validate() error {
	var problems []string
	add := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }

	if len(m.Entries) == 0 {
		add("no countries")
	}
	seen := map[string]bool{}
	for i := range m.Entries {
		e := &m.Entries[i]
		id := fmt.Sprintf("entry %d (%s)", i, e.Code)

		if !alpha2Re.MatchString(e.Code) {
			add("%s: code must be ISO 3166-1 alpha-2 (two upper-case letters)", id)
		}
		if seen[e.Code] {
			add("%s: duplicate code", id)
		}
		seen[e.Code] = true
		if strings.TrimSpace(e.Name) == "" {
			add("%s: name is required", id)
		}
		if e.Enabled == nil {
			add("%s: enabled is required", id)
		}

		switch e.Levels {
		case models.BoundaryLevelsADM1:
			if e.Source != SourceHDXCODAB {
				add("%s: an adm1 entry must have source %q", id, SourceHDXCODAB)
			}
			if e.Member == "" {
				add("%s: an adm1 entry needs member", id)
			}
			if e.NameField == "" {
				add("%s: an adm1 entry needs name_field", id)
			}
		case models.BoundaryLevelsADM0:
			if e.Source != SourceHDXCODAB && e.Source != SourceGeoBoundaries {
				add("%s: unknown source %q", id, e.Source)
			}
		default:
			add("%s: levels must be %q or %q", id, models.BoundaryLevelsADM1, models.BoundaryLevelsADM0)
		}

		if e.ArchiveURL == "" {
			add("%s: archive_url is required", id)
		} else if !strings.HasPrefix(e.ArchiveURL, "https://") {
			add("%s: archive_url must be https", id)
		}
		if strings.HasSuffix(strings.ToLower(e.Member), "_em.geojson") {
			add("%s: member %q is an _em variant", id, e.Member)
		}
		if !sha256Re.MatchString(e.MemberSHA256) {
			add("%s: member_sha256 must be 64 lower-case hex characters", id)
		}
		if e.Vintage == "" {
			add("%s: vintage is required", id)
		} else if t, err := time.Parse("2006-01-02", e.Vintage); err != nil {
			add("%s: vintage %q is not YYYY-MM-DD", id, e.Vintage)
		} else {
			e.vintage = t
		}
		for from, to := range e.NameOverrides {
			if strings.TrimSpace(from) == "" || strings.TrimSpace(to) == "" {
				add("%s: name_overrides keys and values must be non-empty", id)
			}
		}

		if len(e.BBoxValues) != 4 {
			add("%s: bbox must be [min_lon, min_lat, max_lon, max_lat]", id)
			continue
		}
		b := e.BBox()
		if b[0] >= b[2] || b[1] >= b[3] {
			add("%s: bbox min must be below max", id)
		}
		if b[0] < AfricaRect[0] || b[1] < AfricaRect[1] || b[2] > AfricaRect[2] || b[3] > AfricaRect[3] {
			add("%s: bbox %v is outside the Africa rectangle %v", id, b, AfricaRect)
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("%w: %s", ErrInvalidManifest, strings.Join(problems, "; "))
	}
	return nil
}
