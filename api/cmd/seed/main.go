package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type GeoBoundariesResponse struct {
	SimplifiedGeometryGeoJSON string `json:"simplifiedGeometryGeoJSON"`
}

type FeatureCollection struct {
	Features []Feature `json:"features"`
}

type Feature struct {
	Properties map[string]interface{} `json:"properties"`
	Geometry   json.RawMessage        `json:"geometry"`
}

// Download JSON from URL with a simple retry
func fetchURL(urlStr string) ([]byte, error) {
	var body []byte
	var err error
	
	client := &http.Client{Timeout: 30 * time.Second}
	
	for i := 0; i < 3; i++ {
		req, _ := http.NewRequest("GET", urlStr, nil)
		req.Header.Set("User-Agent", "VigilAfrica-Seeder/1.0")
		
		resp, reqErr := client.Do(req)
		if reqErr == nil && resp.StatusCode == 200 {
			body, err = io.ReadAll(resp.Body)
			resp.Body.Close()
			return body, err
		}
		if resp != nil {
			resp.Body.Close()
		}
		err = fmt.Errorf("failed to fetch %s (attempt %d)", urlStr, i+1)
		time.Sleep(2 * time.Second)
	}
	return nil, err
}

func main() {
	log.Println("Starting VigilAfrica Reference Data Seeder...")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	// 1. Get the actual download URL for the NGA ADM1 geojson
	log.Println("Fetching GeoBoundaries metadata for Nigeria ADM1...")
	metaData, err := fetchURL("https://www.geoboundaries.org/api/current/gbOpen/NGA/ADM1/")
	if err != nil {
		log.Fatalf("Failed to fetch API metadata: %v", err)
	}

	var meta GeoBoundariesResponse
	if err := json.Unmarshal(metaData, &meta); err != nil {
		log.Fatalf("Failed to decode metadata: %v", err)
	}

	// 2. Download the GeoJSON features
	log.Printf("Downloading GeoJSON from: %s\n", meta.SimplifiedGeometryGeoJSON)
	geoData, err := fetchURL(meta.SimplifiedGeometryGeoJSON)
	if err != nil {
		log.Fatalf("Failed to download geojson: %v", err)
	}

	var fc FeatureCollection
	if err := json.Unmarshal(geoData, &fc); err != nil {
		log.Fatalf("Failed to parse GeoJSON: %v", err)
	}

	log.Printf("Successfully loaded %d administrative boundaries. Inserting to DB...\n", len(fc.Features))

	// 3. Clear existing Nigeria boundaries to make this script idempotent
	_, err = pool.Exec(ctx, "DELETE FROM admin_boundaries WHERE country_code = $1", "NGA")
	if err != nil {
		log.Printf("Warning: failed to clear existing boundaries: %v\n", err)
	}

	// 4. Insert each feature
	successCount := 0
	for _, feature := range fc.Features {
		// GeoBoundaries standardizes the name property as "shapeName"
		stateNameRaw, ok := feature.Properties["shapeName"]
		if !ok {
			log.Println("Warning: Feature missing shapeName property, skipping.")
			continue
		}
		
		stateName := stateNameRaw.(string)
		countryName := "Nigeria"
		countryCode := "NGA"
		admLevel := 1

		geometryJSON := string(feature.Geometry)

		// PostGIS provides ST_GeomFromGeoJSON for easy insertion
		query := `
			INSERT INTO admin_boundaries 
			(country_code, country_name, adm_level, adm_name, geom)
			VALUES ($1, $2, $3, $4, ST_Multi(ST_GeomFromGeoJSON($5)))
		`

		_, err := pool.Exec(ctx, query, countryCode, countryName, admLevel, stateName, geometryJSON)
		if err != nil {
			log.Printf("Failed to insert %s: %v\n", stateName, err)
			continue
		}
		successCount++
	}

	log.Printf("Seeding complete. Inserted %d / %d records.\n", successCount, len(fc.Features))
}
