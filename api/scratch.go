package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type GbResponse struct {
	SimplifiedGeometryGeoJSON string `json:"simplifiedGeometryGeoJSON"`
}

func main() {
	resp, err := http.Get("https://www.geoboundaries.org/api/current/gbOpen/NGA/ADM1/")
	if err != nil {
		log.Fatalf("GeoBoundaries API Error: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var gbResp GbResponse
	json.Unmarshal(body, &gbResp)

	fmt.Printf("GeoJSON URL: %s\n", gbResp.SimplifiedGeometryGeoJSON)

	// Fetch GeoJSON
	geojsonResp, err := http.Get(gbResp.SimplifiedGeometryGeoJSON)
	if err != nil {
		log.Fatalf("Error fetching geojson: %v", err)
	}
	defer geojsonResp.Body.Close()
	gb, _ := io.ReadAll(geojsonResp.Body)

	// Print just the first few bytes to see the structure and some properties
	fmt.Printf("GeoJSON Snippet: %s\n", string(gb[:1000]))
}
