package geoip

import (
	"errors"
	"net"

	"github.com/oschwald/geoip2-golang"
)

// Reader wraps the MaxMind GeoIP2 database for context lookups.
type Reader struct {
	db *geoip2.Reader
}

// Location represents the geospatial data resolved from an IP address.
type Location struct {
	Country string  `json:"country"`
	State   string  `json:"state"`
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
}

// NewReader initializes a new GeoIP reader from the provided mmdb file path.
// The file is opened in memory-mapped mode under the hood by geoip2-golang.
func NewReader(dbPath string) (*Reader, error) {
	db, err := geoip2.Open(dbPath)
	if err != nil {
		return nil, err
	}
	return &Reader{db: db}, nil
}

// Close closes the underlying GeoIP database file.
func (r *Reader) Close() {
	if r.db != nil {
		r.db.Close()
	}
}

// Lookup resolves an IP address to a physical location.
// Returns an error if the IP is invalid or cannot be found in the database.
func (r *Reader) Lookup(ipStr string) (*Location, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, errors.New("invalid IP address format")
	}

	record, err := r.db.City(ip)
	if err != nil {
		return nil, err
	}

	if record.Country.IsoCode == "" {
		return nil, errors.New("no country found for IP")
	}

	loc := &Location{
		Country: record.Country.Names["en"],
		Lat:     record.Location.Latitude,
		Lng:     record.Location.Longitude,
	}

	if len(record.Subdivisions) > 0 {
		loc.State = record.Subdivisions[0].Names["en"]
	}

	return loc, nil
}
