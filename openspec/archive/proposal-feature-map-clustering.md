# Proposal: Map Performance & Clustering (feature-map-clustering)

## Why
When rendering a large number of events (e.g., thousands of wildfires across the continent) on the map, the browser will attempt to render thousands of individual DOM elements or WebGL points. This can cause severe performance degradation, battery drain, and browser lockups, especially on mobile devices.

## What Changes
We will enable MapLibre's native GeoJSON clustering feature. Instead of drawing 5,000 points, the map will group dense points into clusters with a numerical count, splitting apart dynamically as the user zooms in.

## Out of Scope
- Backend clustering logic (e.g., PostGIS clustering). We will rely entirely on the frontend MapLibre client-side clustering for v1.0.
- Custom cluster shapes beyond basic styled circles.

## User Impact
Massively improved interactive map performance when viewing large datasets. The map will look cleaner at low zoom levels, preventing the UI from being overwhelmed by overlapping event markers.
