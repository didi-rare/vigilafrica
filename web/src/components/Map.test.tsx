import { StrictMode } from 'react'
import { act, render, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { Map } from './Map'

type LayerConfig = {
  id: string
  type: string
  source: string
  filter?: unknown
  paint?: Record<string, unknown>
  layout?: Record<string, unknown>
}

type SourceConfig = {
  type: string
  cluster?: boolean
  clusterMaxZoom?: number
  clusterRadius?: number
  data: GeoJSON.FeatureCollection
}

type SourceFeature = {
  properties: Record<string, unknown>
  geometry: GeoJSON.Point
}

type Handler = (...args: unknown[]) => void

type TestEvent = {
  id: string
  lat: number
  lng: number
  category: string
  title: string
}

const maplibreMock = vi.hoisted(() => {
  class MockGeoJSONSource {
    readonly setData = vi.fn()
    readonly getClusterExpansionZoom = vi.fn().mockResolvedValue(10)
  }

  class MockMap {
    readonly genericHandlers = new globalThis.Map<string, Set<Handler>>()
    readonly layerHandlers = new globalThis.Map<string, globalThis.Map<string, Set<Handler>>>()
    readonly sources = new globalThis.Map<string, MockGeoJSONSource>()
    readonly sourceConfigs = new globalThis.Map<string, SourceConfig>()
    readonly layers = new globalThis.Map<string, LayerConfig>()
    sourceFeatures: SourceFeature[] = []
    canvasStyle: { cursor: string } = { cursor: '' }

    readonly addSource = vi.fn((id: string, config: SourceConfig) => {
      this.sources.set(id, new MockGeoJSONSource())
      this.sourceConfigs.set(id, config)
      return this
    })
    readonly addLayer = vi.fn((config: LayerConfig) => {
      this.layers.set(config.id, config)
      return this
    })
    readonly removeLayer = vi.fn((id: string) => {
      this.layers.delete(id)
      return this
    })
    readonly removeSource = vi.fn((id: string) => {
      this.sources.delete(id)
      this.sourceConfigs.delete(id)
      return this
    })
    readonly getSource = vi.fn((id: string) => this.sources.get(id))
    readonly getLayer = vi.fn((id: string) => this.layers.get(id))
    readonly querySourceFeatures = vi.fn(() => this.sourceFeatures)
    readonly queryRenderedFeatures = vi.fn(() => this.sourceFeatures)
    readonly getCanvas = vi.fn(() => ({ style: this.canvasStyle } as HTMLCanvasElement))
    readonly flyTo = vi.fn()
    readonly easeTo = vi.fn()
    readonly remove = vi.fn()

    readonly on = vi.fn((...args: unknown[]) => {
      if (args.length === 2) {
        const [event, handler] = args as [string, Handler]
        if (!this.genericHandlers.has(event)) this.genericHandlers.set(event, new Set())
        this.genericHandlers.get(event)?.add(handler)
      } else {
        const [event, layer, handler] = args as [string, string, Handler]
        if (!this.layerHandlers.has(event)) this.layerHandlers.set(event, new globalThis.Map())
        const layerMap = this.layerHandlers.get(event)
        if (!layerMap) return this
        if (!layerMap.has(layer)) layerMap.set(layer, new Set())
        layerMap.get(layer)?.add(handler)
      }
      return this
    })

    readonly off = vi.fn((...args: unknown[]) => {
      if (args.length === 2) {
        const [event, handler] = args as [string, Handler]
        this.genericHandlers.get(event)?.delete(handler)
      } else {
        const [event, layer, handler] = args as [string, string, Handler]
        this.layerHandlers.get(event)?.get(layer)?.delete(handler)
      }
      return this
    })

    readonly options: unknown

    constructor(options: unknown) {
      this.options = options
      maplibreMock.instances.maps.push(this)
    }

    trigger(event: string, ...payload: unknown[]) {
      const handlers = Array.from(this.genericHandlers.get(event) ?? [])
      handlers.forEach((handler) => handler(...payload))
    }

    triggerLayer(event: string, layer: string, ...payload: unknown[]) {
      const handlers = Array.from(this.layerHandlers.get(event)?.get(layer) ?? [])
      handlers.forEach((handler) => handler(...payload))
    }
  }

  class MockPopup {
    readonly setDOMContent = vi.fn((content: HTMLElement) => {
      this.content = content
      return this
    })
    readonly options: unknown
    content: HTMLElement | null = null

    constructor(options: unknown) {
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
    readonly options: { element: HTMLElement; anchor: string }
    lngLat: [number, number] | null = null
    popup: MockPopup | null = null
    map: MockMap | null = null

    constructor(options: { element: HTMLElement; anchor: string }) {
      this.options = options
      maplibreMock.instances.markers.push(this)
    }
  }

  return {
    Map: vi.fn(function mapConstructor(options: unknown) {
      return new MockMap(options)
    }),
    Marker: vi.fn(function markerConstructor(options: { element: HTMLElement; anchor: string }) {
      return new MockMarker(options)
    }),
    Popup: vi.fn(function popupConstructor(options: unknown) {
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

const SOURCE_ID = 'events-map-source'
const CLUSTERS_LAYER_ID = 'events-map-clusters'
const CLUSTER_COUNT_LAYER_ID = 'events-map-cluster-count'

const events: TestEvent[] = [
  { id: 'lagos-flood', lat: 6.5244, lng: 3.3792, category: 'floods', title: 'Lagos Flood' },
  { id: 'accra-fire', lat: 5.6037, lng: -0.187, category: 'wildfires', title: 'Accra Wildfire' },
]

function unclusteredFeature(event: TestEvent): SourceFeature {
  return {
    properties: { id: event.id, title: event.title, category: event.category },
    geometry: { type: 'Point', coordinates: [event.lng, event.lat] },
  }
}

function clusterFeature(clusterId: number, pointCount: number, lng: number, lat: number): SourceFeature {
  return {
    properties: {
      cluster: true,
      cluster_id: clusterId,
      point_count: pointCount,
      point_count_abbreviated: String(pointCount),
    },
    geometry: { type: 'Point', coordinates: [lng, lat] },
  }
}

describe('Map', () => {
  afterEach(() => {
    vi.clearAllMocks()
    maplibreMock.instances.maps.length = 0
    maplibreMock.instances.markers.length = 0
    maplibreMock.instances.popups.length = 0
  })

  it('initializes MapLibre with a clustered GeoJSON source and cluster layers', async () => {
    render(<Map events={events} center={[3.3792, 6.5244]} zoom={6} />)

    expect(maplibreMock.setWorkerUrl).toHaveBeenCalledWith('mock-maplibre-worker.js')
    expect(maplibreMock.Map).toHaveBeenCalledTimes(1)

    const map = maplibreMock.instances.maps[0]
    map.sourceFeatures = []
    const options = map.options as { style: { glyphs?: string } }
    expect(options.style.glyphs).toBeUndefined()

    await act(async () => {
      map.trigger('load')
    })

    const sourceConfig = map.sourceConfigs.get(SOURCE_ID)
    expect(sourceConfig).toMatchObject({
      type: 'geojson',
      cluster: true,
      clusterMaxZoom: 14,
      clusterRadius: 50,
    })

    const clusters = map.layers.get(CLUSTERS_LAYER_ID)
    expect(clusters).toMatchObject({
      type: 'circle',
      source: SOURCE_ID,
      filter: ['has', 'point_count'],
    })

    const clusterCount = map.layers.get(CLUSTER_COUNT_LAYER_ID)
    expect(clusterCount).toMatchObject({
      type: 'symbol',
      source: SOURCE_ID,
      filter: ['has', 'point_count'],
    })
    expect(clusterCount?.layout).toMatchObject({
      'text-field': ['get', 'point_count_abbreviated'],
      'text-font': expect.arrayContaining([
        'Arial Unicode MS Bold',
        'Roboto Bold',
        'DejaVu Sans Bold',
      ]),
    })
    // Regression guard: a remote-glyph font ("Open Sans Bold") would re-introduce
    // the demotiles 404s — the stack must use locally-renderable fonts only.
    const textFont = clusterCount?.layout?.['text-font'] as string[] | undefined
    expect(textFont).not.toContain('Open Sans Bold')
  })

  it('creates accessible DOM markers only for unclustered features returned by the source', async () => {
    render(<Map events={events} />)

    const map = maplibreMock.instances.maps[0]
    map.sourceFeatures = events.map(unclusteredFeature)

    await act(async () => {
      map.trigger('load')
    })

    await waitFor(() => {
      expect(maplibreMock.Marker).toHaveBeenCalledTimes(2)
    })

    const source = map.sources.get(SOURCE_ID)
    expect(source?.setData).toHaveBeenCalledTimes(1)

    const [lagosMarker, accraMarker] = maplibreMock.instances.markers
    expect(lagosMarker.options.element).toHaveAccessibleName('Lagos Flood (floods)')
    expect(lagosMarker.lngLat).toEqual([3.3792, 6.5244])
    expect(lagosMarker.addTo).toHaveBeenCalledWith(map)
    expect(accraMarker.options.element).toHaveAccessibleName('Accra Wildfire (wildfires)')
    expect(accraMarker.lngLat).toEqual([-0.187, 5.6037])
  })

  it('does not inject event data into marker HTML', async () => {
    const maliciousEvent = {
      id: 'malicious-title',
      lat: 6.5244,
      lng: 3.3792,
      category: 'floods',
      title: '<img src=x onerror=alert(1)>',
    }
    render(<Map events={[maliciousEvent]} />)

    const map = maplibreMock.instances.maps[0]
    map.sourceFeatures = [unclusteredFeature(maliciousEvent)]

    await act(async () => {
      map.trigger('load')
    })

    await waitFor(() => {
      expect(maplibreMock.instances.markers).toHaveLength(1)
    })

    const markerElement = maplibreMock.instances.markers[0].options.element
    expect(markerElement).toHaveAccessibleName('<img src=x onerror=alert(1)> (floods)')
    expect(markerElement.querySelector('img')).toBeNull()
    expect(markerElement.innerHTML).not.toContain('onerror')
  })

  it('removes markers for points that become clustered after a viewport change', async () => {
    render(<Map events={events} />)

    const map = maplibreMock.instances.maps[0]
    map.sourceFeatures = events.map(unclusteredFeature)

    await act(async () => {
      map.trigger('load')
    })

    await waitFor(() => {
      expect(maplibreMock.instances.markers).toHaveLength(2)
    })

    const accraMarker = maplibreMock.instances.markers.find((m) => m.lngLat?.[0] === -0.187)
    expect(accraMarker).toBeDefined()

    map.sourceFeatures = [unclusteredFeature(events[0]), clusterFeature(99, 1, -0.187, 5.6037)]

    await act(async () => {
      map.trigger('zoomend')
    })

    expect(accraMarker?.remove).toHaveBeenCalledTimes(1)
  })

  it('replaces an existing marker when a visible event keeps its id but changes data', async () => {
    const initialEvent = events[0]
    const updatedEvent = {
      ...initialEvent,
      lat: 7.3775,
      lng: 3.947,
      category: 'wildfires',
      title: 'Ibadan Wildfire',
    }
    const { rerender } = render(<Map events={[initialEvent]} />)

    const map = maplibreMock.instances.maps[0]
    map.sourceFeatures = [unclusteredFeature(initialEvent)]

    await act(async () => {
      map.trigger('load')
    })

    await waitFor(() => {
      expect(maplibreMock.instances.markers).toHaveLength(1)
    })

    const initialMarker = maplibreMock.instances.markers[0]
    map.sourceFeatures = [unclusteredFeature(updatedEvent)]

    rerender(<Map events={[updatedEvent]} />)

    await waitFor(() => {
      expect(maplibreMock.instances.markers).toHaveLength(2)
    })

    const replacementMarker = maplibreMock.instances.markers[1]
    expect(initialMarker.remove).toHaveBeenCalledTimes(1)
    expect(replacementMarker.options.element).toHaveAccessibleName('Ibadan Wildfire (wildfires)')
    expect(replacementMarker.lngLat).toEqual([3.947, 7.3775])
  })

  it('expands a cluster on click using getClusterExpansionZoom + easeTo', async () => {
    render(<Map events={events} />)

    const map = maplibreMock.instances.maps[0]
    map.sourceFeatures = []

    await act(async () => {
      map.trigger('load')
    })

    const cluster = clusterFeature(42, 8, 3.3792, 6.5244)
    map.sourceFeatures = [cluster]

    await act(async () => {
      map.triggerLayer('click', CLUSTERS_LAYER_ID, { point: { x: 10, y: 10 } })
    })

    const source = map.sources.get(SOURCE_ID)
    expect(source?.getClusterExpansionZoom).toHaveBeenCalledWith(42)

    await waitFor(() => {
      expect(map.easeTo).toHaveBeenCalledWith({ center: [3.3792, 6.5244], zoom: 10 })
    })
  })

  it('flies to a new center when the center prop changes after load', async () => {
    const { rerender } = render(<Map events={events.slice(0, 1)} center={[3.3792, 6.5244]} />)

    const map = maplibreMock.instances.maps[0]
    map.sourceFeatures = [unclusteredFeature(events[0])]

    await act(async () => {
      map.trigger('load')
    })

    rerender(<Map events={events.slice(0, 1)} center={[-0.187, 5.6037]} />)

    await waitFor(() => {
      expect(map.flyTo).toHaveBeenLastCalledWith({
        center: [-0.187, 5.6037],
        zoom: 7,
        speed: 0.8,
      })
    })
  })

  it('removes cluster layers, source, and the MapLibre instance on unmount', async () => {
    const { unmount } = render(<Map events={events} />)

    const map = maplibreMock.instances.maps[0]
    map.sourceFeatures = []

    await act(async () => {
      map.trigger('load')
    })

    unmount()

    expect(map.removeLayer).toHaveBeenCalledWith(CLUSTER_COUNT_LAYER_ID)
    expect(map.removeLayer).toHaveBeenCalledWith(CLUSTERS_LAYER_ID)
    expect(map.removeSource).toHaveBeenCalledWith(SOURCE_ID)
    expect(map.remove).toHaveBeenCalledTimes(1)
  })

  it('reinitializes cleanly across the StrictMode effect cleanup cycle', () => {
    render(
      <StrictMode>
        <Map events={events} />
      </StrictMode>,
    )

    expect(maplibreMock.Map).toHaveBeenCalledTimes(2)
    expect(maplibreMock.instances.maps[0].remove).toHaveBeenCalledTimes(1)
    expect(maplibreMock.instances.maps[1].remove).not.toHaveBeenCalled()
  })
})
