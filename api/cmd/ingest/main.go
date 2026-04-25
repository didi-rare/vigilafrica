package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"vigilafrica/api/internal/database"
	"vigilafrica/api/internal/ingestor"
)

func main() {
	// Setup context with timeout and cancellation on interrupt
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Received termination signal, shutting down ingestor...")
		cancel()
	}()

	// Database URL from environment — required, no fallback (matches cmd/server behaviour)
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	log.Printf("Starting VigilAfrica NASA EONET Ingestor — %d countries", len(ingestor.DefaultCountries))

	repo, err := database.NewRepository(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to initialize database repository: %v", err)
	}
	defer repo.Close()

	var totalFetched, totalStored int
	for _, country := range ingestor.DefaultCountries {
		result, err := ingestor.Ingest(ctx, repo, country)
		if err != nil {
			log.Printf("Ingestion failed for %s: %v", country.Name, err)
			continue
		}
		log.Printf("%s: fetched=%d stored=%d", country.Name, result.EventsFetched, result.EventsStored)
		totalFetched += result.EventsFetched
		totalStored += result.EventsStored
	}

	log.Printf("Ingestion run complete. Total fetched: %d, stored: %d", totalFetched, totalStored)
}
