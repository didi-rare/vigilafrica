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

	log.Println("Starting VigilAfrica NASA EONET Ingestor run...")

	repo, err := database.NewRepository(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to initialize database repository: %v", err)
	}
	defer repo.Close()

	result, err := ingestor.Ingest(ctx, repo)
	if err != nil {
		log.Fatalf("Ingestion run failed: %v", err)
	}

	log.Printf("Ingestion run completed successfully. Fetched: %d, Stored: %d",
		result.EventsFetched, result.EventsStored,
	)
}
