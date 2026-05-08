# Design: v0.4 Useful Prototype (Sentinel Maps & Near-Me Context)

## Architecture
**1. GeoIP Sidecar (Docker)**
We will use the `maxmindinc/geoipupdate` container mounted with a shared volume `/var/opt/maxmind`. The Go application container will mount the same volume as Read-Only.
Environment variables for the maxmind container (`GEOIPUPDATE_ACCOUNT_ID` and `GEOIPUPDATE_LICENSE_KEY`) will be mocked in local `.env.example`.

**2. Context API Endpoint (`GET /v1/context`)**
Inside `api/internal/geoip/`, we implement the `Reader` logic.
Inside `api/internal/handlers/context.go`, we parse the `X-Forwarded-For` HTTP header (safe behind Caddy/Vercel) to obtain the IP.
We then execute the GeoIP lookup, convert it to PostGIS `Point(longitude, latitude)`, and use `ST_DWithin` to find events clustered ~200km near that center. Return JSON:
```json
{
  "location": {"country": "Nigeria", "state": "Lagos", "lat": 6.52, "lng": 3.37},
  "nearby_events": [...]
}
```

**3. Frontend Sentinel Dashboard**
- `react-map-gl` or pure `maplibre-gl-js` standard implementation. 
- Map style: MapTiler Satellite or equivalent dark terrain view. 
- A translucent `.dashboard-overlay` element covering the UI frame that enhances the neon text elements.
- CSS classes for `.pulse-flood` and `.pulse-fire`.
- Main view transitions to a 50/50 horizontal split (List on Left, Map on Right).

## Security & Privacy
No user tracking or cookies required. The IP address is mapped entirely locally by the `mmdb` database; no third-party HTTP requests are sent to maxmind during the user's API transaction.
