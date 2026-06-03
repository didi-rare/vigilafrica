// Package digest builds VigilAfrica's daily flood digest — the same content
// served as JSON at GET /v1/digest/today.json and emailed once per day to a
// fixed recipient list. BuildTodayDigest is the single source of truth so the
// API view and the email can never drift (feature-daily-flood-digest).
package digest

import (
	"context"
	"fmt"
	"sort"
	"time"

	"vigilafrica/api/internal/database"
	"vigilafrica/api/internal/models"
)

// maxDigestEvents caps the events pulled for a single day. The repository
// clamps any Limit above 200 back to its default, and a realistic day of
// floods across the supported countries is far below this, so the cap never
// bites in practice — it is a safety bound, not a pagination scheme.
const maxDigestEvents = 200

const unknownGroup = "Unknown"

// EventLister is the subset of database.Repository the digest needs. Narrowing
// the dependency keeps the builder trivially testable with a fake.
type EventLister interface {
	ListEvents(ctx context.Context, filters database.EventFilters) ([]models.Event, int, error)
}

// Digest is the day's flood events grouped by country → state.
type Digest struct {
	Date        string         `json:"date"`         // YYYY-MM-DD (UTC)
	GeneratedAt time.Time      `json:"generated_at"` // UTC
	Total       int            `json:"total"`
	ByCountry   []CountryGroup `json:"by_country"`
}

type CountryGroup struct {
	CountryName string       `json:"country_name"`
	States      []StateGroup `json:"states"`
}

type StateGroup struct {
	StateName string        `json:"state_name"`
	Events    []DigestEvent `json:"events"`
}

// DigestEvent is the reduced event shape the digest exposes — enough for a
// reader to know what and where, with a link back to the source.
type DigestEvent struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	EventDate *time.Time `json:"event_date"`
	SourceURL *string    `json:"source_url"`
}

// BuildTodayDigest returns the flood events whose event_date falls on the
// current UTC calendar day, grouped by country → state. `now` is injected so
// callers (handler, scheduler) and tests control the clock. An empty day is a
// valid result: Total 0 and an empty ByCountry slice, never an error.
func BuildTodayDigest(ctx context.Context, repo EventLister, now time.Time) (Digest, error) {
	dayStart := startOfUTCDay(now)
	dayEnd := dayStart.Add(24 * time.Hour)

	filters := database.EventFilters{
		Category: string(models.CategoryFloods),
		DateFrom: &dayStart,
		DateTo:   &dayEnd,
		Limit:    maxDigestEvents,
	}

	events, _, err := repo.ListEvents(ctx, filters)
	if err != nil {
		return Digest{}, fmt.Errorf("build today digest: %w", err)
	}

	return Digest{
		Date:        dayStart.Format("2006-01-02"),
		GeneratedAt: now.UTC(),
		Total:       len(events),
		ByCountry:   groupByCountryState(events),
	}, nil
}

// groupByCountryState buckets events by country then state, both sorted
// alphabetically for deterministic JSON and email output. Events missing a
// country or state name fall under "Unknown". Event order within a state is
// preserved from the query (event_date DESC).
func groupByCountryState(events []models.Event) []CountryGroup {
	bucket := map[string]map[string][]DigestEvent{}
	for _, e := range events {
		country := valueOr(e.CountryName, unknownGroup)
		state := valueOr(e.StateName, unknownGroup)
		if bucket[country] == nil {
			bucket[country] = map[string][]DigestEvent{}
		}
		bucket[country][state] = append(bucket[country][state], DigestEvent{
			ID:        e.ID.String(),
			Title:     e.Title,
			EventDate: e.EventDate,
			SourceURL: e.SourceURL,
		})
	}

	groups := make([]CountryGroup, 0, len(bucket))
	for _, country := range sortedKeys(bucket) {
		states := bucket[country]
		stateGroups := make([]StateGroup, 0, len(states))
		for _, state := range sortedKeys(states) {
			stateGroups = append(stateGroups, StateGroup{StateName: state, Events: states[state]})
		}
		groups = append(groups, CountryGroup{CountryName: country, States: stateGroups})
	}
	return groups
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func valueOr(s *string, fallback string) string {
	if s == nil || *s == "" {
		return fallback
	}
	return *s
}

func startOfUTCDay(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
