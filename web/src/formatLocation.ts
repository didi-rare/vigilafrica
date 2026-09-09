/**
 * Formats the place labels shown on event cards and the detail page.
 *
 * ⚠️ A missing STATE does not mean a missing PLACE. Events reached through the
 * NG/GH bounding-box overhang are enriched by the ADM0 fallback added in
 * migration 000012: they carry a country but no state, because we hold ADM1
 * boundaries only for Nigeria and Ghana and will not invent a state for a
 * country whose states we do not have.
 *
 * Both call sites previously branched on `state_name` alone, which produced two
 * different wrong answers for the same row:
 *
 *   EventsDashboard  showed raw coordinates ("4.6027, 9.4139") while
 *                    country_name said "Cameroon"
 *   EventDetail      rendered an empty <strong> and a stray leading comma
 *                    (", Cameroon")
 *
 * Neither is acceptable on a warning product: a Nigerian reader is not told the
 * event is outside their country, and a Cameroonian reader is not told it is
 * inside theirs. Coordinates are a last resort, not a substitute for a country
 * we already know.
 */
export interface LocationLabel {
  /** The emphasised part — a state where we have one, else the country. */
  primary: string
  /** The trailing part, or empty when there is nothing further to add. */
  secondary: string
  /** True when no place name exists at all and coordinates are the only option. */
  isCoordinatesOnly: boolean
}

export function formatLocation(
  stateName: string | null | undefined,
  countryName: string | null | undefined,
  lat: number | null | undefined,
  lng: number | null | undefined,
): LocationLabel {
  const state = stateName?.trim()
  const country = countryName?.trim()

  if (state && country) {
    return { primary: state, secondary: country, isCoordinatesOnly: false }
  }
  // A state without a country should not occur, but rendering the state alone
  // beats rendering a dangling separator if it ever does.
  if (state) {
    return { primary: state, secondary: '', isCoordinatesOnly: false }
  }
  if (country) {
    return { primary: country, secondary: '', isCoordinatesOnly: false }
  }
  if (typeof lat === 'number' && typeof lng === 'number') {
    return { primary: `${lat.toFixed(4)}, ${lng.toFixed(4)}`, secondary: '', isCoordinatesOnly: true }
  }
  // Geometry exists but yielded no point and no enrichment. Say so plainly
  // rather than printing "undefined, undefined".
  return { primary: 'Location unavailable', secondary: '', isCoordinatesOnly: true }
}
