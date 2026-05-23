package handlers

import (
	"errors"
	"net/url"
	"strings"
)

// countryCodeToName maps ISO 3166-1 alpha-2 codes to the canonical country
// names stored in the events.country_name column. The list mirrors
// ingestor.DefaultCountries but is kept local to avoid a handlers→ingestor
// dependency (fix-api-country-filter D4). When a new country is onboarded,
// add it here AND in ingestor/eonet.go.
var countryCodeToName = map[string]string{
	"NG": "Nigeria",
	"GH": "Ghana",
}

// countryNameToCode is the reverse lookup, built once at init. Keys are
// lowercased so name matching is case-insensitive (D5).
var countryNameToCode = func() map[string]string {
	m := make(map[string]string, len(countryCodeToName))
	for code, name := range countryCodeToName {
		m[strings.ToLower(name)] = code
	}
	return m
}()

// errUnknownCountry is the package-level sentinel for unrecognised country
// inputs. The message doubles as the public 400 body — it lists supported
// values in both code and name form so callers learn the contract from the
// error itself (developers-go.md §4.8).
var errUnknownCountry = errors.New("unknown country: supported values are NG, GH (or Nigeria, Ghana)")

// resolveCountry inspects the `country` and `country_code` query params and
// returns the canonical country_name for filtering downstream queries.
//
// Precedence: if both are present, `country_code` wins (D2).
// Matching is case-insensitive for both code and name (D5).
//
// Returns:
//   - canonical name, true, nil  → caller should apply the filter
//   - "",             false, nil → no filter requested
//   - "",             false, err → unknown value; caller MUST respond 400
func resolveCountry(query url.Values) (string, bool, error) {
	code := strings.TrimSpace(query.Get("country_code"))
	name := strings.TrimSpace(query.Get("country"))

	if code != "" {
		canonical, ok := countryCodeToName[strings.ToUpper(code)]
		if !ok {
			return "", false, errUnknownCountry
		}
		return canonical, true, nil
	}

	if name != "" {
		resolvedCode, ok := countryNameToCode[strings.ToLower(name)]
		if !ok {
			return "", false, errUnknownCountry
		}
		return countryCodeToName[resolvedCode], true, nil
	}

	return "", false, nil
}
