package boundaries

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"
)

// TestEmbeddedManifestIsValid is the offline CI gate for the checked-in
// manifest (task 2.5): Embedded() runs every spec §1 rule.
func TestEmbeddedManifestIsValid(t *testing.T) {
	m, err := Embedded()
	if err != nil {
		t.Fatalf("embedded manifest: %v", err)
	}
	codes := make([]string, 0, len(m.Entries))
	for _, e := range m.Entries {
		codes = append(codes, e.Code)
	}
	if !sort.StringsAreSorted(codes) {
		t.Errorf("entries must be sorted by code (scripts/boundaries writes them that way): %v", codes)
	}
	for _, code := range []string{"NG", "GH"} {
		e, ok := m.Entry(code)
		if !ok {
			t.Errorf("%s missing from the manifest", code)
			continue
		}
		if e.Levels != "adm1" || !e.IsEnabled() || e.PCodeField == "" {
			t.Errorf("%s: levels=%q enabled=%v pcode_field=%q, want adm1, enabled, a P-code field", code, e.Levels, e.IsEnabled(), e.PCodeField)
		}
	}
	if len(m.Checksum()) != 64 {
		t.Errorf("manifest checksum %q is not a SHA-256", m.Checksum())
	}
}

func validEntry() map[string]any {
	return map[string]any{
		"code": "XA", "name": "Exland", "levels": "adm1", "source": "hdx-cod-ab", "enabled": true,
		"archive_url":   "https://example.org/xaa_admin_boundaries.geojson.zip",
		"member":        "xaa_admin1.geojson",
		"member_sha256": strings.Repeat("a", 64),
		"vintage":       "2025-10-30",
		"name_field":    "adm1_name",
		"pcode_field":   "adm1_pcode",
		"bbox":          []float64{50, -30, 51, -29},
	}
}

func manifestJSON(t *testing.T, entries ...map[string]any) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{"schema_version": 1, "countries": entries})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseRejectsInvalidManifests(t *testing.T) {
	with := func(k string, v any) map[string]any {
		e := validEntry()
		if v == nil {
			delete(e, k)
		} else {
			e[k] = v
		}
		return e
	}
	tests := []struct {
		name    string
		entries []map[string]any
		want    string
	}{
		{"duplicate code", []map[string]any{validEntry(), validEntry()}, "duplicate code"},
		{"lower-case code", []map[string]any{with("code", "xa")}, "alpha-2"},
		{"alpha-3 code", []map[string]any{with("code", "XAA")}, "alpha-2"},
		{"no archive_url", []map[string]any{with("archive_url", nil)}, "archive_url is required"},
		{"http archive_url", []map[string]any{with("archive_url", "http://example.org/a.zip")}, "https"},
		{"no member_sha256", []map[string]any{with("member_sha256", nil)}, "member_sha256"},
		{"short member_sha256", []map[string]any{with("member_sha256", "abc")}, "member_sha256"},
		{"no vintage", []map[string]any{with("vintage", nil)}, "vintage is required"},
		{"bad vintage", []map[string]any{with("vintage", "2025-10")}, "YYYY-MM-DD"},
		{"no bbox", []map[string]any{with("bbox", nil)}, "bbox"},
		{"three-number bbox", []map[string]any{with("bbox", []float64{1, 2, 3})}, "bbox"},
		{"inverted bbox", []map[string]any{with("bbox", []float64{51, -30, 50, -29})}, "min must be below max"},
		{"bbox outside Africa", []map[string]any{with("bbox", []float64{100, 0, 101, 1})}, "Africa rectangle"},
		{"adm1 without member", []map[string]any{with("member", nil)}, "needs member"},
		{"adm1 without name_field", []map[string]any{with("name_field", nil)}, "needs name_field"},
		{"_em member", []map[string]any{with("member", "xaa_admin1_em.geojson")}, "_em variant"},
		{"adm1 from geoboundaries", []map[string]any{with("source", "geoboundaries")}, "must have source"},
		{"unknown levels", []map[string]any{with("levels", "adm2")}, "levels must be"},
		{"no enabled", []map[string]any{with("enabled", nil)}, "enabled is required"},
		{"empty override", []map[string]any{with("name_overrides", map[string]string{"A": ""})}, "name_overrides"},
		{"unknown field", []map[string]any{with("bbx", []float64{1, 2, 3, 4})}, "unknown field"},
		{"no countries", nil, "no countries"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(manifestJSON(t, tt.entries...))
			if err == nil {
				t.Fatalf("Parse accepted an invalid manifest")
			}
			if !errors.Is(err, ErrInvalidManifest) {
				t.Errorf("error does not wrap ErrInvalidManifest: %v", err)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not mention %q", err, tt.want)
			}
		})
	}
}

func TestParseAcceptsADM0OutlineEntry(t *testing.T) {
	e := validEntry()
	e["levels"], e["source"] = "adm0", "geoboundaries"
	delete(e, "member")
	delete(e, "name_field")
	delete(e, "pcode_field")
	if _, err := Parse(manifestJSON(t, e)); err != nil {
		t.Fatalf("adm0 outline entry rejected: %v", err)
	}
}

func TestEntryHash(t *testing.T) {
	hashOf := func(t *testing.T, raw string) string {
		t.Helper()
		m, err := Parse([]byte(`{"schema_version":1,"countries":[` + raw + `]}`))
		if err != nil {
			t.Fatal(err)
		}
		return m.Entries[0].Hash()
	}
	base := `{"code":"XA","name":"Exland","levels":"adm1","source":"hdx-cod-ab","enabled":true,` +
		`"archive_url":"https://e.org/a.zip","member":"a_admin1.geojson","member_sha256":"` + strings.Repeat("a", 64) + `",` +
		`"vintage":"2025-10-30","name_field":"adm1_name","name_overrides":{"B":"Bb","A":"Aa"},"bbox":[50,-30,51,-29]}`
	reordered := `{"bbox":[50,-30,51,-29],"name_overrides":{"A":"Aa","B":"Bb"},"name_field":"adm1_name","vintage":"2025-10-30",` +
		`"member_sha256":"` + strings.Repeat("a", 64) + `","member":"a_admin1.geojson","archive_url":"https://e.org/a.zip",` +
		`"enabled":true,"source":"hdx-cod-ab","levels":"adm1","name":"Exland","code":"XA"}`

	h := hashOf(t, base)
	if len(h) != 64 {
		t.Fatalf("hash %q", h)
	}
	if got := hashOf(t, reordered); got != h {
		t.Errorf("key order changed the hash: %s vs %s", got, h)
	}
	if got := hashOf(t, strings.Replace(base, `"enabled":true`, `"enabled":false`, 1)); got != h {
		t.Errorf("enabled changed the hash; it must not (it changes nothing stored)")
	}
	for name, changed := range map[string]string{
		"override": strings.Replace(base, `"A":"Aa"`, `"A":"Ab"`, 1),
		"bbox":     strings.Replace(base, `[50,-30,51,-29]`, `[50,-30,51.1,-29]`, 1),
		"field":    strings.Replace(base, `"adm1_name"`, `"ADM1_EN"`, 1),
		"name":     strings.Replace(base, `"Exland"`, `"Exlandia"`, 1),
	} {
		if got := hashOf(t, changed); got == h {
			t.Errorf("changing the %s did not change the hash", name)
		}
	}
}

func TestManifestChecksumIgnoresWhitespace(t *testing.T) {
	compact := manifestJSON(t, validEntry())
	var pretty strings.Builder
	pretty.WriteString("{\r\n  \"schema_version\": 1,\r\n  \"countries\": [\r\n    ")
	b, _ := json.Marshal(validEntry())
	pretty.Write(b)
	pretty.WriteString("\r\n  ]\r\n}\r\n")

	a, err := Parse(compact)
	if err != nil {
		t.Fatal(err)
	}
	c, err := Parse([]byte(pretty.String()))
	if err != nil {
		t.Fatal(err)
	}
	if a.Checksum() != c.Checksum() {
		t.Errorf("CRLF/whitespace changed the manifest checksum")
	}
}
