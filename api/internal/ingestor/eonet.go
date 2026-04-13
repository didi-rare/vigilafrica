package ingestor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"vigilafrica/api/internal/database"
	"vigilafrica/api/internal/normalizer"
)

const eonetURL = "https://eonet.gsfc.nasa.gov/api/v3/events"

// Default query parameters for Nigeria bounds and categories
// bbox: [min_lon, min_lat, max_lon, max_lat] for Nigeria
const queryParams = "?bbox=2.0,4.0,15.0,14.0&category=floods,wildfires&status=open,closed"

// Ingest pulls events from NASA EONET and upserts them into the provided database repository.
func Ingest(ctx context.Context, repo database.Repository) error {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	reqURL := eonetURL + queryParams
	log.Printf("Fetching events from NASA EONET: %s", reqURL)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "VigilAfrica-Ingestor/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d from EONET", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	var root struct {
		Title  string                     `json:"title"`
		Events []normalizer.RawEONETEvent `json:"events"`
	}

	if err := json.Unmarshal(body, &root); err != nil {
		return fmt.Errorf("failed to decode JSON response: %w", err)
	}

	log.Printf("Fetched %d raw events from EONET. Normalizing and inserting...", len(root.Events))

	successCount := 0
	for _, rawEvt := range root.Events {
		// Isolate individual raw payload for storage
		rawEvtBytes, _ := json.Marshal(rawEvt)

		event, geoJSON, err := normalizer.Normalize(rawEvt, rawEvtBytes)
		if err != nil {
			log.Printf("Warning: failed to normalize event %s: %v", rawEvt.ID, err)
			continue
		}

		// Ensure we don't insert events without geometry (unless configured to do so)
		if geoJSON == "" {
			log.Printf("Skipping event %s: no geometry extracted", event.SourceID)
			continue
		}

		err = repo.UpsertEvent(ctx, event, geoJSON)
		if err != nil {
			log.Printf("Error: failed to upsert event %s: %v", event.SourceID, err)
			continue
		}

		successCount++
	}

	log.Printf("Successfully ingested %d events.", successCount)
	return nil
}
