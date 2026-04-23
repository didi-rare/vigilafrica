import { act, render, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { Map } from './Map'

type MapOptions = {
  container: HTMLElement
  center?: [number, number]
  zoom?: number
}

type FlyToOptions = {
  center: [number, number]
  zoom: number
  speed: number
}

type MarkerOptions = {
  element: HTMLElement
  anchor: string
}

type PopupOptions = {
  offset: number
}

type LoadHandler = () => void

type TestEvent = {
  id: string
  lat: number
  lng: number
  category: string
  title: string
}

const maplibreMock = vi.hoisted(() => {
  class MockMap {
    readonly handlers = new globalThis.Map<string, LoadHandler>()
    readonly flyTo = vi.fn()
    readonly remove = vi.fn()
    readonly on = vi.fn((event: string, handler: LoadHandler) => {
      this.handlers.set(event, handler)
      return this
    })
    readonly options: MapOptions

    constructor(options: MapOptions) {
      this.options = options
      maplibreMock.instances.maps.push(this)
    }

    trigger(event: string) {
      this.handlers.get(event)?.()
    }
  }

  class MockPopup {
    readonly setDOMContent = vi.fn((content: HTMLElement) => {
      this.content = content
      return this
    })
    readonly options: PopupOptions
    content: HTMLElement | null = null

    constructor(options: PopupOptions) {
      this.options = options
      maplibreMock.instances.popups.push(this)
    }
  }

  class MockMarker {
    readonly setLngLat = vi.fn((lngLat: [number, number]) => {
      this.lngLat = lngLat
      return this
    })
    readonly setPopup = vi.fn((popup: MockPopup) => {
      this.popup = popup
      return this
    })
    readonly addTo = vi.fn((map: MockMap) => {
      this.map = map
      return this
    })
    readonly remove = vi.fn()
    readonly options: MarkerOptions
    lngLat: [number, number] | null = null
    popup: MockPopup | null = null
    map: MockMap | null = null

    constructor(options: MarkerOptions) {
      this.options = options
      maplibreMock.instances.markers.push(this)
    }
  }

  return {
    Map: vi.fn(function mapConstructor(options: MapOptions) {
      return new MockMap(options)
    }),
    Marker: vi.fn(function markerConstructor(options: MarkerOptions) {
      return new MockMarker(options)
    }),
    Popup: vi.fn(function popupConstructor(options: PopupOptions) {
      return new MockPopup(options)
    }),
    setWorkerUrl: vi.fn(),
    instances: {
      maps: [] as MockMap[],
      markers: [] as MockMarker[],
      popups: [] as MockPopup[],
    },
  }
})

vi.mock('maplibre-gl/dist/maplibre-gl-csp', () => ({
  default: {
    Map: maplibreMock.Map,
    Marker: maplibreMock.Marker,
    Popup: maplibreMock.Popup,
  },
  Map: maplibreMock.Map,
  Marker: maplibreMock.Marker,
  Popup: maplibreMock.Popup,
  setWorkerUrl: maplibreMock.setWorkerUrl,
}))

vi.mock('maplibre-gl/dist/maplibre-gl-csp-worker.js?url', () => ({
  default: 'mock-maplibre-worker.js',
}))

const events: TestEvent[] = [
  {
    id: 'lagos-flood',
    lat: 6.5244,
    lng: 3.3792,
    category: 'floods',
    title: 'Lagos Flood',
  },
  {
    id: 'accra-fire',
    lat: 5.6037,
    lng: -0.187,
    category: 'wildfires',
    title: 'Accra Wildfire',
  },
]

describe('Map', () => {
  afterEach(() => {
    vi.clearAllMocks()
    maplibreMock.instances.maps.length = 0
    maplibreMock.instances.markers.length = 0
    maplibreMock.instances.popups.length = 0
  })

  it('initializes MapLibre with the configured container, center, zoom, and worker', () => {
    render(<Map events={events} center={[3.3792, 6.5244]} zoom={6} />)

    expect(maplibreMock.setWorkerUrl).toHaveBeenCalledWith('mock-maplibre-worker.js')
    expect(maplibreMock.Map).toHaveBeenCalledTimes(1)

    const map = maplibreMock.instances.maps[0]
    expect(map.options.container).toBeInstanceOf(HTMLDivElement)
    expect(map.options.center).toEqual([3.3792, 6.5244])
    expect(map.options.zoom).toBe(6)
    expect(map.on).toHaveBeenCalledWith('load', expect.any(Function))
  })

  it('creates accessible markers after the map load event', async () => {
    render(<Map events={events} />)

    act(() => {
      maplibreMock.instances.maps[0].trigger('load')
    })

    await waitFor(() => {
      expect(maplibreMock.Marker).toHaveBeenCalledTimes(2)
    })

    const [lagosMarker, accraMarker] = maplibreMock.instances.markers
    expect(lagosMarker.options.element).toHaveAccessibleName('Lagos Flood (floods)')
    expect(lagosMarker.lngLat).toEqual([3.3792, 6.5244])
    expect(lagosMarker.addTo).toHaveBeenCalledWith(maplibreMock.instances.maps[0])
    expect(accraMarker.options.element).toHaveAccessibleName('Accra Wildfire (wildfires)')
    expect(accraMarker.lngLat).toEqual([-0.187, 5.6037])
  })

  it('flies to a new center and replaces markers when props change', async () => {
    const { rerender } = render(<Map events={events.slice(0, 1)} center={[3.3792, 6.5244]} />)

    act(() => {
      maplibreMock.instances.maps[0].trigger('load')
    })

    await waitFor(() => {
      expect(maplibreMock.Marker).toHaveBeenCalledTimes(1)
    })

    const initialMarker = maplibreMock.instances.markers[0]
    rerender(<Map events={events} center={[-0.187, 5.6037]} />)

    await waitFor(() => {
      expect(maplibreMock.Marker).toHaveBeenCalledTimes(3)
    })

    expect(maplibreMock.instances.maps[0].flyTo).toHaveBeenLastCalledWith({
      center: [-0.187, 5.6037],
      zoom: 7,
      speed: 0.8,
    } satisfies FlyToOptions)
    expect(initialMarker.remove).toHaveBeenCalledTimes(1)
  })

  it('removes the MapLibre instance on unmount', () => {
    const { unmount } = render(<Map events={events} />)

    unmount()

    expect(maplibreMock.instances.maps[0].remove).toHaveBeenCalledTimes(1)
  })
})
