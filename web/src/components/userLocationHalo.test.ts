import { describe, expect, it } from 'vitest'

import { APPROX_RADIUS_M, MIN_HALO_DIAMETER_PX, haloDiameterPx, metresPerPixel } from './userLocationHalo'

const LAGOS_LAT = 6.5244

describe('userLocationHalo', () => {
  it('uses MapLibre 512px-tile ground resolution', () => {
    // Equator, zoom 0: the whole circumference across 512px.
    expect(metresPerPixel(0, 0)).toBeCloseTo(40_075_016.686 / 512, 6)
    // Each zoom level halves it; latitude shrinks it by cos(lat).
    expect(metresPerPixel(8, 0)).toBeCloseTo(metresPerPixel(7, 0) / 2, 9)
    expect(metresPerPixel(7, 60)).toBeCloseTo(metresPerPixel(7, 0) * 0.5, 9)
  })

  it('covers the same ground at every zoom, so the circle grows as the map zooms in', () => {
    const z7 = haloDiameterPx({ zoom: 7, lat: LAGOS_LAT, maxPx: 0 })
    const z8 = haloDiameterPx({ zoom: 8, lat: LAGOS_LAT, maxPx: 0 })
    const z10 = haloDiameterPx({ zoom: 10, lat: LAGOS_LAT, maxPx: 0 })

    // 100 km across at ~607 m/px.
    expect(z7).toBeCloseTo((2 * APPROX_RADIUS_M) / metresPerPixel(7, LAGOS_LAT), 6)
    expect(z7).toBeGreaterThan(160)
    expect(z7).toBeLessThan(170)
    expect(z8).toBeCloseTo(z7 * 2, 6)
    expect(z10).toBeCloseTo(z7 * 8, 6)
  })

  it('never shrinks below the marker box when zoomed out', () => {
    for (const zoom of [0, 2, 4, 5]) {
      expect(haloDiameterPx({ zoom, lat: LAGOS_LAT, maxPx: 1360 })).toBe(MIN_HALO_DIAMETER_PX)
    }
  })

  it('caps the painted size once the halo is larger than the view', () => {
    expect(haloDiameterPx({ zoom: 16, lat: LAGOS_LAT, maxPx: 1360 })).toBe(1360)
    // A cap below the minimum never wins over the minimum.
    expect(haloDiameterPx({ zoom: 16, lat: LAGOS_LAT, maxPx: 10 })).toBe(MIN_HALO_DIAMETER_PX)
    // No measurable container: no cap.
    expect(haloDiameterPx({ zoom: 12, lat: LAGOS_LAT, maxPx: 0 })).toBeGreaterThan(5000)
  })
})
