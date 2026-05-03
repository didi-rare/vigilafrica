package database

import (
	"strings"
	"testing"
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
