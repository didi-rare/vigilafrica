export type EventCategory = 'floods' | 'wildfires'
export type EventStatus = 'open' | 'closed'

export interface GeoLocation {
  country: string;
  state: string;
  lat: number;
  lng: number;
}

export interface ContextResponse {
  location: GeoLocation | null;
  nearby_events: VigilEvent[];
}

export interface VigilEvent {
  id: string;
  source_id: string;
  source: string;
  title: string;
  category: EventCategory;
  status: EventStatus;
  geometry_type: string | null;
  latitude: number | null;
  longitude: number | null;
  country_name: string | null;
  state_name: string | null;
  event_date: string | null;
  source_url: string | null;
  ingested_at: string;
  enriched_at: string | null;
}

export interface EventsResponse {
  data: VigilEvent[];
  meta: {
    total: number;
    limit: number;
    offset: number;
  };
}

export async function fetchEvents(category?: EventCategory, stateName?: string): Promise<EventsResponse> {
  // Use absolute URL since Vite proxy might not be configured yet, but usually we use relative
  // Assuming API runs on 8080 locally if proxy fails, for now use relative URL assuming Vite proxy is configured.
  const url = new URL('/v1/events', window.location.origin)
  
  if (category) url.searchParams.set('category', category)
  if (stateName) url.searchParams.set('state', stateName)

  const res = await fetch(url.toString())
  if (!res.ok) {
    throw new Error('Failed to fetch events from VigilAfrica API')
  }

  return res.json()
}

export async function fetchEventById(id: string): Promise<VigilEvent> {
  const res = await fetch(`/v1/events/${id}`)
  if (!res.ok) {
    throw new Error(`Failed to fetch event ${id}`)
  }
  return res.json()
}

export async function fetchContext(): Promise<ContextResponse> {
  const res = await fetch('/v1/context')
  if (!res.ok) {
    throw new Error('Failed to fetch user context')
  }
  return res.json()
}

// ── Health / ingestion freshness (v0.5 — ADR-011) ────────────────────────────

export interface LastIngestion {
  status: 'success' | 'failure' | 'running' | null
  started_at: string | null
  completed_at: string | null
  events_fetched: number | null
  events_stored: number | null
  error: string | null
}

export interface HealthResponse {
  status: 'ok' | 'degraded'
  version: string
  last_ingestion: LastIngestion | null
}

export async function fetchHealth(): Promise<HealthResponse> {
  const res = await fetch('/health')
  if (!res.ok) {
    throw new Error('Failed to fetch health status')
  }
  return res.json()
}
