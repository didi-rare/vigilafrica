# Spec: Map Performance & Clustering (feature-map-clustering)

## Context
Our React frontend uses MapLibre GL JS to display event locations as GeoJSON data. Currently, every point is added to the map layer directly. To handle continent-scale datasets, we must group overlapping points.

## Components to Touch
1. `web/src/components/Map.tsx`

## Implementation Plan
1.  **Configure Source:** Update the `addSource` call for the events GeoJSON in `Map.tsx` to include `cluster: true`, `clusterMaxZoom: 14`, and `clusterRadius: 50`.
2.  **Add Cluster Layers:**
    *   Add a new circle layer for the clusters themselves (filtered by `has: point_count`). Color the circles based on point density (e.g., small=yellow, medium=orange, large=red using `step` expressions).
    *   Add a new symbol layer to display the `point_count` text inside the cluster circles.
3.  **Update Unclustered Layer:** Modify the existing event point layer to be filtered by `!has: point_count` so individual events only show when they aren't part of a cluster.
4.  **Interaction:** Add a click event handler to the cluster layer that automatically zooms the map in to expand the clicked cluster.

## Acceptance Criteria
- [ ] Map displays clustered circles with numerical counts when zoomed out.
- [ ] Clusters expand into smaller clusters or individual points when zooming in or clicking.
- [ ] Individual points are rendered when sufficiently zoomed in.
- [ ] The browser does not freeze when 5,000+ points are loaded.

## Verification Plan
1.  Use the `npm run api:seed` functionality (or modify the DB) to insert 2,000+ mock events clustered around a single city.
2.  Load the frontend dashboard and confirm the map renders smoothly and displays a single large cluster.
3.  Zoom in and verify smooth separation of the clusters.
