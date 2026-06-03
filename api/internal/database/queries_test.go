package database

import (
	"strings"
	"testing"
	"time"
)

func TestBuildEventFilterClauseKeepsUserValuesParameterized(t *testing.T) {
	filters := EventFilters{
		Category: "floods",
		Country:  "Nigeria' OR '1'='1",
		State:    "Lagos'); DROP TABLE events; --",
		Status:   "open",
	}

	whereClause, args, nextArgID := buildEventFilterClause(filters)

	if strings.Contains(whereClause, filters.Country) {
		t.Fatalf("country value leaked into SQL clause: %s", whereClause)
	}
	if strings.Contains(whereClause, filters.State) {
		t.Fatalf("state value leaked into SQL clause: %s", whereClause)
	}
	if strings.Contains(whereClause, "DROP TABLE") || strings.Contains(whereClause, "OR '1'='1") {
		t.Fatalf("SQLi-looking input was interpolated into clause: %s", whereClause)
	}
	if want := "WHERE category = $1 AND country_name ILIKE $2 AND state_name ILIKE $3 AND status = $4"; whereClause != want {
		t.Fatalf("expected clause %q, got %q", want, whereClause)
	}
	if nextArgID != 5 {
		t.Fatalf("expected next arg id 5, got %d", nextArgID)
	}
	if len(args) != 4 {
		t.Fatalf("expected 4 args, got %d", len(args))
	}
	if args[1] != filters.Country {
		t.Fatalf("expected country to remain a query arg, got %v", args[1])
	}
	if args[2] != filters.State {
		t.Fatalf("expected state to remain a query arg, got %v", args[2])
	}
}

// TestBuildEventFilterClauseDateRange covers the event_date half-open interval
// filtering added for the daily flood digest (feature-daily-flood-digest).
func TestBuildEventFilterClauseDateRange(t *testing.T) {
	from := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)

	whereClause, args, nextArgID := buildEventFilterClause(EventFilters{
		Category: "floods",
		DateFrom: &from,
		DateTo:   &to,
	})

	if want := "WHERE category = $1 AND event_date >= $2 AND event_date < $3"; whereClause != want {
		t.Fatalf("expected clause %q, got %q", want, whereClause)
	}
	if nextArgID != 4 {
		t.Fatalf("expected next arg id 4, got %d", nextArgID)
	}
	if len(args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(args))
	}
	if args[1] != from {
		t.Fatalf("expected DateFrom as arg[1], got %v", args[1])
	}
	if args[2] != to {
		t.Fatalf("expected DateTo as arg[2], got %v", args[2])
	}
}

// TestBuildEventFilterClauseNilDatesOmitted verifies nil date bounds add no
// clause — existing callers (which never set them) are unaffected.
func TestBuildEventFilterClauseNilDatesOmitted(t *testing.T) {
	whereClause, args, _ := buildEventFilterClause(EventFilters{})
	if whereClause != "" {
		t.Fatalf("expected empty clause, got %q", whereClause)
	}
	if len(args) != 0 {
		t.Fatalf("expected 0 args, got %d", len(args))
	}
}
