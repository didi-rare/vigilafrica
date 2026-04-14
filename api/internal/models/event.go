package models

import (
	"time"

	"github.com/google/uuid"
)

// EventCategory is the normalized event classification.
type EventCategory string

const (
	CategoryFloods    EventCategory = "floods"
	CategoryWildfires EventCategory = "wildfires"
)

// EventStatus reflects whether the EONET event is still active.
type EventStatus string

const (
	StatusOpen   EventStatus = "open"
	StatusClosed EventStatus = "closed"
)

// Event is the canonical internal event representation.
// All layers (ingestor, normalizer, enricher, API) use this type.
type Event struct {
	ID          uuid.UUID     `json:"id"            db:"id"`
	SourceID    string        `json:"source_id"     db:"source_id"`
	Source      string        `json:"source"        db:"source"`
	Title       string        `json:"title"         db:"title"`
	Category    EventCategory `json:"category"      db:"category"`
	Status      EventStatus   `json:"status"        db:"status"`
	GeomType    *string       `json:"geometry_type" db:"geom_type"`
	Latitude    *float64      `json:"latitude"      db:"latitude"`
	Longitude   *float64      `json:"longitude"     db:"longitude"`
	CountryName *string       `json:"country_name"  db:"country_name"`
	StateName   *string       `json:"state_name"    db:"state_name"`
	EventDate   *time.Time    `json:"event_date"    db:"event_date"`
	SourceURL   *string       `json:"source_url"    db:"source_url"`
	RawPayload  []byte        `json:"-"             db:"raw_payload"`
	IngestedAt  time.Time     `json:"ingested_at"   db:"ingested_at"`
	EnrichedAt  *time.Time    `json:"enriched_at"   db:"enriched_at"`
}
