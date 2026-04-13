export type EventCategory = 'floods' | 'wildfires'
export type EventStatus = 'open' | 'closed'

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
