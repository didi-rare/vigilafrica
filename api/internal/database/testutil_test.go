//go:build integration

package database_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"vigilafrica/api/internal/database"
)

// testRepo is the shared repository instance for the entire test suite.
// Initialized once in TestMain; the PostGIS container lives for the duration of the run.
var testRepo database.Repository

func TestMain(m *testing.M) {
	ctx := context.Background()

	// PostGIS emits the ready log twice: once for template DB init and once for the
	// actual database. Waiting for 2 occurrences avoids the "EOF" race on startup.
	ctr, err := tcpostgres.Run(ctx, "postgis/postgis:15-3.4@sha256:3f4a5d48e0be9580ed70ed618cd039ce57bbb2dd113053d3836e28513f1f87cd",
		tcpostgres.WithDatabase("vigilafrica_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(120*time.Second),
		),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start PostGIS container: %v\n", err)
		os.Exit(1)
	}

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = ctr.Terminate(ctx) // best-effort cleanup; already in fatal exit path
		fmt.Fprintf(os.Stderr, "failed to get PostGIS connection string: %v\n", err)
		os.Exit(1)
	}

	repo, err := database.NewRepository(ctx, dsn)
	if err != nil {
		_ = ctr.Terminate(ctx) // best-effort cleanup; already in fatal exit path
		fmt.Fprintf(os.Stderr, "failed to initialize repository (migrations may have failed): %v\n", err)
		os.Exit(1)
	}

	testRepo = repo
	code := m.Run()

	repo.Close()
	_ = ctr.Terminate(ctx) // best-effort cleanup; exit code already captured
	os.Exit(code)
}

// ptrStr returns a pointer to a string literal.
func ptrStr(s string) *string { return &s }

// ptrF64 returns a pointer to a float64 literal.
func ptrF64(f float64) *float64 { return &f }
