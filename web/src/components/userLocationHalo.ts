// Size of the "you are here" marker's approximate-area halo
// (feature-current-location-marker, review fix P1).
//
// The halo stands for a fixed area on the ground, not a fixed size on screen. A
// fixed-pixel circle shrinks to a few hundred metres when the visitor zooms in
// and then reads like a GPS accuracy circle, which an IP-derived position must
// never do (proposal R1).

// ⚠️ ASSUMPTION, not a measurement: /v1/context sends no accuracy radius, and
// IP geolocation is city-level at best. 50 km is the stated radius of the
// approximate area. If real error is usually larger (region- or country-level
// lookups), the halo still understates it; if it is usually much smaller, the
// halo overstates the vagueness. Either way it never claims street precision.
export const APPROX_RADIUS_M = 50_000

// Never smaller than the marker's own 56px box (`.map-user-marker` in
// Map.css), so the halo is always visible around the dot. Keep the two equal.
export const MIN_HALO_DIAMETER_PX = 56

// Web Mercator ground resolution with MapLibre's 512px tiles: the world is
// 512 * 2^zoom pixels wide at the equator, and a pixel covers cos(lat) less
// ground toward the poles. Over a 50 km radius the scale change across the
// circle is negligible, so one value at the marker's latitude is enough.
const EARTH_CIRCUMFERENCE_M = 40_075_016.686
const TILE_SIZE_PX = 512

export function metresPerPixel(zoom: number, lat: number): number {
  return (EARTH_CIRCUMFERENCE_M * Math.cos((lat * Math.PI) / 180)) / (TILE_SIZE_PX * 2 ** zoom)
}

/**
 * On-screen diameter of the halo, in pixels.
 *
 * `maxPx` caps what gets painted (pass about twice the map's longest side);
 * once the cap applies the halo already covers the whole view, which is the
 * honest reading at that zoom. A non-positive `maxPx` means no cap (e.g. an
 * unmeasured container).
 */
export function haloDiameterPx({ zoom, lat, maxPx }: { zoom: number; lat: number; maxPx: number }): number {
  const ground = (2 * APPROX_RADIUS_M) / metresPerPixel(zoom, lat)
  const capped = maxPx > 0 ? Math.min(ground, maxPx) : ground
  return Math.max(MIN_HALO_DIAMETER_PX, capped)
}
