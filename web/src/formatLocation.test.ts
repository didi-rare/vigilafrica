import { describe, expect, it } from 'vitest'

import { formatLocation } from './formatLocation'

describe('formatLocation', () => {
  it('shows state and country when both are known', () => {
    const label = formatLocation('Jigawa', 'Nigeria', 12.5287, 9.8956)
    expect(label).toEqual({ primary: 'Jigawa', secondary: 'Nigeria', isCoordinatesOnly: false })
  })

  // ⚠️ The case observed on staging 2026-09-09. EONET_22248 is a Cameroonian
  // flood reached through the Nigeria bbox overhang: the ADM0 fallback gives it
  // a country but no state, because we hold ADM1 boundaries only for NG and GH.
  //
  // The dashboard used to print raw coordinates here and the detail page used to
  // print ", Cameroon" with an empty <strong>. Both discarded a country we knew.
  it('falls back to the COUNTRY, not coordinates, when the state is unknown', () => {
    const label = formatLocation(null, 'Cameroon', 4.6027, 9.4139)
    expect(label.primary).toBe('Cameroon')
    expect(label.secondary).toBe('')
    expect(label.isCoordinatesOnly).toBe(false)
    expect(label.primary).not.toContain('4.6027')
  })

  it('treats blank strings as absent rather than rendering a dangling separator', () => {
    // A stray leading comma is exactly what the old detail page produced.
    const label = formatLocation('   ', 'Cameroon', 4.6027, 9.4139)
    expect(label.primary).toBe('Cameroon')
    expect(label.secondary).toBe('')
  })

  it('uses coordinates only when no place name exists at all', () => {
    const label = formatLocation(null, null, 6.4551, 3.3942)
    expect(label.primary).toBe('6.4551, 3.3942')
    expect(label.isCoordinatesOnly).toBe(true)
  })

  it('says so plainly when there is neither a place nor a point', () => {
    // A polygon event whose enrichment found nothing. Printing
    // "undefined, undefined" would be worse than admitting we do not know.
    const label = formatLocation(null, null, null, null)
    expect(label.primary).toBe('Location unavailable')
    expect(label.isCoordinatesOnly).toBe(true)
  })

  it('renders a state without a country rather than a dangling separator', () => {
    const label = formatLocation('Lagos', null, 6.4551, 3.3942)
    expect(label.primary).toBe('Lagos')
    expect(label.secondary).toBe('')
  })
})
