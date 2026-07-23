# React Coding Standards — VigilAfrica

**Scope:** all frontend code under `web/` in this repository.
**Stack:** React 19, TypeScript, Vite, TanStack Query v5, React Router v7, MapLibre GL, plain CSS.
**Audience:** contributors writing React/TS code, and reviewers enforcing standards via `/openspec-review`.
**Status:** living document. Any contributor may open a PR proposing changes; maintainer approval merges.

Each rule: **statement → why → example (where useful) → anti-pattern callout**. Rules are numbered (`§4.2`) so reviewers can cite them directly.

Cross-references (all ACCEPTED in [`openspec/specs/vigilafrica/decisions.md`](../../openspec/specs/vigilafrica/decisions.md)):
- ADR-012 — Frontend Server State: TanStack Query.
- ADR-013 — Frontend Styling: Plain CSS over CSS-in-JS.
- ADR-015 — Visual Identity & Type System: Ground Truth (§7.11, §14.3).
- ADR-001 — MapLibre GL JS (map library choice, existing).
- `docs/standards/developers-go.md` — Go-side standards for the API layer.

---

## Table of Contents

1. [Project Layout](#1-project-layout)
2. [TypeScript](#2-typescript)
3. [Component Design](#3-component-design)
4. [State Management](#4-state-management)
5. [Data Fetching](#5-data-fetching)
6. [Routing](#6-routing)
7. [Styling](#7-styling)
8. [Performance](#8-performance)
9. [Accessibility](#9-accessibility)
10. [Error Handling & Boundaries](#10-error-handling--boundaries)
11. [Forms & Validation](#11-forms--validation)
12. [Map/Geo Rendering](#12-mapgeo-rendering)
13. [Testing](#13-testing)
14. [Dependencies](#14-dependencies)
15. [Build & Env](#15-build--env)
16. [Appendix — Decision Log](#appendix--decision-log)

---

## 1. Project Layout

**§1.1 — All frontend source lives under `web/src/`. The build artefact is `web/dist/` (gitignored). No code outside `web/` is part of the bundle.**
*Why:* Clear boundary between frontend and the Go API. Deployment scripts know exactly what to ship.

**§1.2 — Folder structure: `src/api/` (typed API clients), `src/components/` (shared UI), `src/pages/` (route-level components), `src/data/` (static data, constants), `src/assets/` (images, fonts), `src/hooks/` (shared hooks), `src/styles/` (design tokens — see §7).**
Root-level `src/` files are limited to entry points and cross-cutting setup: `main.tsx`, `App.tsx`, `analytics.ts`, `setupTests.ts`, and the `.d.ts` shims.
*Why:* Separation of concerns. `api/` knows about network; `components/` knows about rendering; `pages/` composes them into routes.
❌ A flat `src/` with every `.tsx` mixed together.
✅ Current layout (`src/api/events.ts`, `src/components/EventsDashboard.tsx`, `src/pages/`).

**§1.3 — Component files are `PascalCase.tsx`; hooks are `useThing.ts`; utilities are `camelCase.ts`. Consistent per folder.**
*Why:* Filename telegraphs what the file exports. `EventsDashboard.tsx` obviously exports a component; `useEvents.ts` a hook.

**§1.4 — One `named`-exported component per file. The file name equals the component name.**
*Why:* `grep` for the name finds the file; named exports survive renames without silently binding to the wrong symbol.
*Project reality:* `App` is the sole default export (`src/App.tsx`); every other component — `EventsDashboard`, `EventDetail`, `ForPartners`, `BrandMark` — is a named export. Route components are lazy-loaded by remapping the named export to `default` at the `lazy()` call site:
```tsx
const EventsDashboard = lazy(async () => {
  const module = await import('./components/EventsDashboard')
  return { default: module.EventsDashboard }
})
```
**Anti-pattern:** a `components.tsx` exporting `<Header>`, `<Footer>`, `<Sidebar>`. Split them.

**§1.5 — Co-locate a component's CSS next to it: `EventsDashboard.tsx` + `EventsDashboard.css`, imported at the top of the `.tsx`.**
*Why:* Moving or deleting a component takes its styles with it. No orphaned CSS.

**§1.6 — `src/pages/` components are route entry points; they orchestrate data and compose components. They do not contain reusable UI primitives.**
*Why:* Pages are the route surface; components are the building blocks.

**§1.7 — Shared types live next to the API client that produces them (`src/api/events.ts` exports `Event`). No `src/types/` grab-bag folder.**
*Why:* Types stay close to the code that owns them.

**§1.8 — `src/assets/` holds static imports referenced by code. Raw public files (favicons, `robots.txt`) go in `web/public/`.**
*Why:* Vite handles these differently — `assets/` gets hashed and bundled; `public/` is copied verbatim.

**§1.9 — No barrel files (`index.ts` re-exports). Import directly from the source file.**
*Why:* Barrel files pull the entire module into every importer's bundle — Vite cannot tree-shake across them.
❌ `import { fetchEvents } from "../api"` (via `api/index.ts`)
✅ `import { fetchEvents } from "../api/events"`

---

## 2. TypeScript

**§2.1 — `strict: true` in both `web/tsconfig.app.json` and `web/tsconfig.node.json`. Do not disable individual strict sub-flags.**
*Why:* Strict mode catches an order of magnitude more bugs at compile time. Disabling a sub-flag is a permanent tax on every future contributor.
*Enforced by* `npm run type-check` (§15.11), which runs as its own CI step and covers **`src/` including test files** — `tsconfig.app.json` has no `exclude`, so tests are held to the same strict settings as the code they exercise.
*Why tests specifically:* Vitest transpiles via esbuild, which strips types without checking them. If the type checker skipped tests too, a mock whose shape had drifted from the real API type would still compile and still pass — a green test asserting against a shape production never produces.

**§2.2 — No `any`. If you genuinely need "any shape", use `unknown` and narrow.**
*Why:* `any` disables type checking for everything it touches. `unknown` forces a narrowing check.
❌ `function parse(raw: any) { return raw.data; }`
✅ `function parse(raw: unknown) { if (isEventList(raw)) return raw.data; throw new Error("..."); }`
**Anti-pattern:** reaching for `any` to silence a type error. The error is the code telling you something; fix the shape.

**§2.3 — No non-null assertions (`!`) without an inline comment explaining why the value cannot be null at that point.**
*Why:* `!` is a runtime crash waiting to happen.
❌ `const map = mapRef.current!;`
✅ `const map = mapRef.current!; // set in onLoad, only read after map loaded`

**§2.4 — `interface` for object shapes; `type` for unions, primitives, and function signatures.**
*Why:* Matches the codebase — `src/api/events.ts` declares `GeoLocation`, `ContextResponse`, `VigilEvent`, `EventsResponse` and `LastIngestion` as `interface`, and `EventCategory` / `EventStatus` as `type` unions. Consistency beats the abstract `type`-vs-`interface` argument; this is the split already in force.

**§2.5 — Component prop types are defined as `type Props = { ... }` immediately above the component. Do not export prop types unless a consumer needs them.**
*Why:* Keeps the component self-contained. Exported prop types become a public API.
```tsx
type Props = { events: Event[]; onSelect: (id: string) => void };
export default function EventsList({ events, onSelect }: Props) { ... }
```

**§2.6 — Do not type-cast with `as` when a type guard or explicit conversion would work.**
*Why:* `as` is a lie to the compiler.
❌ `const event = data as Event;`
✅ Use a type guard: `function isEvent(v: unknown): v is Event { ... }`
Acceptable `as`: DOM queries with a comment (`document.getElementById("x") as HTMLCanvasElement // verified in template`).

**§2.7 — Use `satisfies` to validate a literal against a type without widening it.**
```ts
const countries = { NG: "Nigeria", GH: "Ghana" } satisfies Record<string, string>;
```

**§2.8 — API response types in `src/api/` are the single source of truth. Components import and consume them; never redeclare a subset.**
*Why:* Drift between the API client's type and a component's idea of the shape causes silent breakage.

**§2.9 — `readonly` on props and state that must not be mutated in place. Do not mutate arrays or objects inside state setters.**
*Why:* React re-renders on reference change; in-place mutation prevents the re-render.
❌ `state.events.push(newEvent); setState(state);`
✅ `setState(prev => ({ ...prev, events: [...prev.events, newEvent] }));`

**§2.10 — Generics carry meaningful names when they have a role (`TData`, `TError`), single-letter only when the role is obvious (`T`).**

---

## 3. Component Design

**§3.1 — Function components only. No class components.**
*Why:* Hooks cover every use case class components had.

**§3.2 — One named-exported component per file (§1.4). Internal helpers live below the component, unexported.**

**§3.3 — Props are destructured at the signature.**
❌ `function EventsList(props: Props) { return props.events.map(...); }`
✅ `function EventsList({ events, onSelect }: Props) { return events.map(...); }`

**§3.4 — Components are composable. More than ~8 props signals a split or a `children`-based composition.**
**Anti-pattern:** `<Dashboard showHeader showFilters showMap filterBy={...} sortBy={...} onSelect={...} />` — reach for `<Outlet>` or `children` instead.

**§3.5 — Containers fetch and orchestrate; presentational components render props. A component calling `useQuery` should not also be a leaf visual primitive.**
*Why:* The visual layer stays reusable and testable without a query client.

**§3.6 — Lift state only when two siblings need it. Do not hoist preemptively.**
*Why:* Premature hoisting causes unnecessary re-renders. YAGNI.
**Anti-pattern:** `App` holding `selectedEventId` that only the map and list care about.

**§3.7 — `children` is the right API for slot composition. Named render props (`renderHeader`) are acceptable for multi-slot cases.**

**§3.8 — Refs (`useRef`, ref-as-prop in React 19) are escape hatches for DOM access, focus, and imperative integrations (maps, video). Do not use refs to work around unidirectional data flow.**
✅ `mapContainerRef` for MapLibre (§12).
❌ `const countRef = useRef(0); countRef.current++;` rendered in JSX.

**§3.9 — Custom hooks are named `useX` and return a consistent shape per hook (tuple or named object — not mixed).**

**§3.10 — Hooks are called unconditionally at the top of the component. No `if (cond) useThing()`.**
*Why:* Breaks React's hook ordering. Enforced by `eslint-plugin-react-hooks`.

**§3.11 — Event handlers: `handleX` when defined locally; `onX` when passed as props.**
✅ Child prop: `onSelect`. Local handler: `handleSelect`.

**§3.12 — Keys on list items are stable IDs, never array indices.**
❌ `events.map((e, i) => <Row key={i} />)`
✅ `events.map(e => <Row key={e.id} />)`

**§3.13 — Conditional rendering uses a ternary or explicit boolean check — never `&&` with a non-boolean left side.**
*Why:* `{count && <Component />}` renders `0` when `count` is `0`. Use `{count > 0 && <Component />}` or `{count ? <Component /> : null}`.
**Anti-pattern:** `{items.length && <List items={items} />}` — renders `0` when the list is empty.

**§3.14 — `React.memo`, `useMemo`, `useCallback` are performance tools applied after profiling — not default practice. See §8.**

---

## 4. State Management

**§4.1 — State falls into four categories. Use the right bucket:**

| Category | Tool | Examples |
|---|---|---|
| Server state | TanStack Query | events list, ingestion run status |
| URL state | React Router `useSearchParams` | active filters, selected country |
| Local UI state | `useState` / `useReducer` | modal open, hover state |
| Global client state | Context (or Zustand if justified) | user preferences, theme |

*Why:* Mixing categories is the root cause of most React state bugs.

**§4.2 — Server state lives exclusively in TanStack Query. Do not shadow it with `useState`.**
❌
```tsx
const [events, setEvents] = useState([]);
useEffect(() => { fetchEvents().then(setEvents); }, []);
```
✅ `const { data: events } = useQuery(eventKeys.list(filters));`
**Anti-pattern:** copying query data into `useState` to "edit locally" — use `useOptimistic` instead.

**§4.3 — Filterable, shareable, or bookmarkable state lives in the URL via `useSearchParams`.**
*Why:* URL state survives refresh, back-navigation, and link-sharing.

**§4.4 — `useState` is for transient UI state with no value outside the current session.**

**§4.5 — `useReducer` when local state has more than two fields that change together, or transitions must be explicit.**

**§4.6 — Context is for low-frequency global values (theme, locale). Do not put high-frequency state in Context.**
*Why:* Every Context consumer re-renders on every context change.

**§4.7 — Derive, don't sync. Compute derived values with `useMemo` during render. Do not `useEffect` + `setState` to keep a derived value in sync.**
❌
```tsx
const [filtered, setFiltered] = useState([]);
useEffect(() => { setFiltered(events.filter(...)); }, [events, filter]);
```
✅ `const filtered = useMemo(() => events.filter(...), [events, filter]);`
**Anti-pattern:** `useEffect` watching state to update other state — almost always a derivation in disguise.

**§4.8 — `useEffect` is for synchronising React state with external systems (DOM, MapLibre, subscriptions). Not for data fetching, not for derived state.**

**§4.9 — Every `useEffect` that sets up a subscription, interval, or listener returns a cleanup function.**
*Why:* Missing cleanup leaks memory and event handlers on every remount.
```tsx
useEffect(() => {
  const id = setInterval(refetch, 30_000);
  return () => clearInterval(id);
}, [refetch]);
```

**§4.10 — Expensive initial state uses the lazy initialiser form of `useState`.**
*Why:* `useState(expensiveCalc())` runs `expensiveCalc` on every render. The function form runs once.
❌ `useState(buildInitialState(rawData))`
✅ `useState(() => buildInitialState(rawData))`

**§4.11 — Global client state beyond Context requires justification in an OpenSpec change record. Zustand is the approved escalation path.**

---

## 5. Data Fetching

**§5.1 — All server data fetching uses TanStack Query. Do not fetch in `useEffect`.**
*Why:* TanStack Query provides loading/error states, caching, deduplication, and background refetch. `useEffect` provides none of it.
Cross-ref: ADR-012 (ACCEPTED).

**§5.2 — Query keys are hierarchical arrays built via a key factory. Never write inline string keys.**
```ts
export const eventKeys = {
  all: ['events'] as const,
  lists: () => [...eventKeys.all, 'list'] as const,
  list: (filters: EventFilters) => [...eventKeys.lists(), filters] as const,
  detail: (id: string) => [...eventKeys.all, 'detail', id] as const,
};
```
**Anti-pattern:** `useQuery({ queryKey: ['events-list-nigeria'] })` — unfilterable, un-invalidatable by prefix.
*Enforced by* `@tanstack/query/exhaustive-deps` (CI, via `npm run lint`). **The bug it prevents:** if a value the `queryFn` reads is missing from the `queryKey`, two different filter states share one cache entry — a user sees another country's events. No error, no crash, just wrong data. That is why this rule is worth more than its noise.

**§5.3 — One `QueryClient` instance, created at app root. Never instantiate inside a component.**
*Enforced by* `@tanstack/query/stable-query-client` (CI). A `QueryClient` built inside a component is discarded and rebuilt on every render, silently throwing away the entire cache.
The single instance lives in `src/main.tsx` ✅. It is currently constructed **bare** — `new QueryClient()` — so every query runs on library defaults (`staleTime: 0`, `retry: 3`).
*Consequence to know:* with `staleTime: 0` every remount refetches, and a failing endpoint is retried three times with backoff before `isError` shows. If you want different behaviour, set it per-query today, or propose project-wide defaults:
```tsx
const queryClient = new QueryClient({
  defaultOptions: { queries: { staleTime: 60_000, retry: 1 } }
});
```

**§5.4 — Data fetching functions live in `src/api/`. Components call hooks; hooks call `src/api/`. Components never call `fetch` directly.**

**§5.5 — Loading and error states are handled explicitly. Use `isPending` (not `isLoading`) for new queries with no cached data.**
*Why:* TanStack Query v5: `isPending` = no data yet; `isFetching` = background refetch in progress.
```tsx
if (isPending) return <Spinner />;
if (isError) return <ErrorMessage error={error} onRetry={refetch} />;
return <EventsList events={data} />;
```
**Anti-pattern:** `data?.map(...)` with no error branch — blank list with no user feedback.

**§5.6 — `useSuspenseQuery` is preferred when a parent `<Suspense>` boundary exists. `useQuery` with manual `isPending` check is the fallback.**

**§5.7 — Suspense boundaries are placed at route level by default. Add a nested boundary only when a section should load independently.**
```tsx
<Route path="/events" element={
  <Suspense fallback={<PageSpinner />}>
    <EventsPage />
  </Suspense>
} />
```

**§5.8 — Cache invalidation uses `queryClient.invalidateQueries({ queryKey: eventKeys.lists() })`. Invalidate by the most specific prefix that captures all affected queries.**

**§5.9 — Background refetch intervals use `refetchInterval` on the query, not a `setInterval` in `useEffect`.**
✅ `useQuery({ ...eventKeys.list(filters), refetchInterval: 5 * 60_000 })`

**§5.10 — API functions accept typed filter objects and return typed response objects. No untyped `fetch` with `as` casts on the response.**

**§5.11 — Optimistic UI uses React 19's `useOptimistic`. Do not manually mirror server state into local state for optimistic updates.**
```tsx
const [optimisticEvents, addOptimistic] = useOptimistic(events);
```

**§5.12 — Error objects from failed queries are normalised in `src/api/` before throwing. Components display a user message, not the raw `Error`.**

---

## 6. Routing

**§6.1 — React Router v7 is the routing layer. Do not navigate with `window.location` or `history` directly.**

**§6.2 — Route definitions live in one place. Today that place is `src/App.tsx` — a single `<Routes>` block inside `App`; `main.tsx` only mounts `<App />`.**
*Why one place:* the route table is the app's public surface; scattering it across files makes the surface unauditable. Extracting to `src/routes.tsx` is fine if the block outgrows `App.tsx`, but do not split routes across several files.

**§6.3 — Route-level components are lazy-loaded with `React.lazy` + `Suspense`.**
*Why:* MapLibre GL is ~250kb gzipped. Lazy-loading the map route means non-map users never pay that cost.
```tsx
const MapPage = React.lazy(() => import("./pages/MapPage"));
<Route path="/map" element={
  <Suspense fallback={<PageSpinner />}><MapPage /></Suspense>
} />
```
**Anti-pattern:** eagerly importing all page components at the top of `routes.tsx`.

**§6.4 — URL parameters via `useParams`; search params via `useSearchParams`. Never parse `window.location.search` manually.**

**§6.5 — Filterable state that should survive navigation lives in the URL (§4.3). Do not reset filters on remount.**

**§6.6 — Programmatic navigation uses `useNavigate` with a `URLSearchParams` builder.**
```tsx
const params = new URLSearchParams({ country: "Nigeria" });
navigate({ pathname: "/events", search: params.toString() });
```

**§6.7 — `<Link>` for internal navigation. `<a target="_blank" rel="noopener noreferrer">` for external links.**

**§6.8 — A catch-all `<Route path="*">` renders a 404 page. No unmatched path renders blank.**

**§6.9 — Layouts use `<Outlet>`-based wrapper routes. Do not import layout components inside every page.**
```tsx
<Route element={<AppShell />}>
  <Route path="/events" element={<EventsPage />} />
  <Route path="/map" element={<MapPage />} />
</Route>
```

---

## 7. Styling

> Cross-ref: ADR-013 (Plain CSS over CSS-in-JS — ACCEPTED 2026-04-18), ADR-015 (type system).
>
> **Enforcement:** stylelint is the only CI-enforced style gate in the repo (`npm run lint:styles`, run by the "Run Frontend Style Lint" job). `web/.stylelintrc.json` wires `stylelint-declaration-strict-value` to reject any **colour-ish** literal that isn't a `var(--…)` — `color`, `background`, `background-color`, `fill`, `stroke` and the `border-*-color` family — with `src/styles/tokens.css` exempted via an override. **Spacing, typography, and z-index are NOT machine-checked**; those halves of §7.5/§7.10/§7.11 are review-enforced only.

**§7.1 — Plain CSS files co-located with components. No CSS-in-JS, no Tailwind, no CSS Modules.**
*Why:* Zero runtime cost, no build transformation, readable by anyone.
**Anti-pattern:** installing `styled-components` or `@emotion/react` without an ADR.

**§7.2 — Each component imports exactly one CSS file: its own. Cross-component sharing via CSS custom properties defined in `src/styles/tokens.css`.**
`tokens.css` is a two-layer system — primitives, then semantic aliases — imported once in `src/main.tsx` ahead of `index.css`. Its owning spec is `openspec/archive/spec-chore-css-tokens.md`.
```css
/* src/styles/tokens.css */
:root { --color-primary: #1a6b3c; --spacing-md: 1rem; }
```

**§7.3 — CSS class names are `kebab-case`, prefixed with the component root name.**
✅ `.events-dashboard__filter`, `.events-dashboard__list`
❌ `.filter`, `.list` — collides globally.

**§7.4 — Layout lives in CSS. Inline `style` props are reserved for truly dynamic values (map marker position, data-driven widths).**
❌ `<div style={{ display: 'flex', gap: '1rem' }}>`
✅ `<div className="events-dashboard__row">`

**§7.5 — Colours, spacing, typography, and z-index are CSS custom properties defined in `src/styles/tokens.css`. Never hardcode values in component CSS.**
❌ `color: #1a6b3c;`
✅ `color: var(--color-primary);`
*Enforcement is uneven:* colours are mechanically blocked by stylelint (see the section header). Spacing, type and z-index are not — the migration is partial and new hardcoded values will pass CI. Don't add them anyway.

**§7.6 — Responsive breakpoints are defined centrally. Components do not hardcode pixel values for breakpoints.**

**§7.7 — No `!important`. Fix the selector hierarchy instead.**

**§7.8 — Animations use CSS `transition` and `@keyframes`. JS-driven animations are reserved for effects CSS cannot achieve.**

**§7.9 — Dark mode via `data-theme="dark"` on `<html>` with CSS custom property overrides.**
```css
[data-theme="dark"] { --color-primary: #4caf7d; --bg: #1a1a1a; }
```

**§7.10 — `z-index` values are named custom properties. Never hardcode a `z-index` in a component file.**
```css
:root { --z-modal: 300; --z-nav: 200; --z-map-controls: 100; }
```
⚠️ **Partially migrated — this rule does not yet describe the codebase.** The `--z-*` properties live ad-hoc in `src/App.css` (`--z-nav`, `--z-dropdown`, `--z-skip-link`) and `src/index.css` (`--z-map-hud`) rather than in `tokens.css`, and hardcoded literals survive in `App.css` and `components/Map.css`. Nothing checks this — stylelint's strict-value rule covers colours only. Consolidating the scale into `tokens.css` is an open chore (`chore-z-index-tokens`); until then, reuse an existing `--z-*` rather than inventing a number, and don't add new ad-hoc properties.

**§7.11 — Font families come from the type tokens in `tokens.css` — `--font-display` (Space Grotesk), `--font-body` (IBM Plex Sans), `--font-mono` (IBM Plex Mono). Never hardcode a font name in component CSS (ADR-015 "Ground Truth").**
*Why:* The three-family system is the brand voice; routing through tokens keeps it swappable in one place and prevents a stray `font-family: Inter` regressing the identity. Fonts are self-hosted via `@fontsource` (no runtime CDN call) — see §15.
❌ `font-family: 'Space Grotesk', sans-serif;`
✅ `font-family: var(--font-display);`

---

## 8. Performance

**§8.1 — Measure before optimising. Use React DevTools Profiler to identify bottlenecks. Do not add `memo`, `useMemo`, or `useCallback` speculatively.**

**§8.2 — Route-level code splitting is mandatory (§6.3). Library-level splitting is done: `vite.config.ts` `manualChunks` isolates `map-vendor` (MapLibre) and `react-vendor`.**
*Keep it that way:* a new heavyweight dependency should either land in an existing vendor chunk or get its own — don't let it fall into the main bundle by default.

**§8.3 — `React.memo` wraps a component only when it receives stable-reference props and a profiler shows unnecessary re-renders.**
**Anti-pattern:** wrapping every component in `memo` "just in case".

**§8.4 — `useMemo` is for expensive computations and stable object references needed in dependency arrays.**
✅ GeoJSON transformation of 5000 features.
❌ `useMemo` on a two-item array.

**§8.5 — `useCallback` is for functions passed to memoised children or used in `useEffect` dependency arrays. Not for every handler.**

**§8.6 — Lists over 100 items are virtualised with `react-window` or `react-virtual`.**
**Anti-pattern:** paginating to page size 200 to avoid virtualisation.

**§8.7 — Images have explicit `width` and `height` or `aspect-ratio` to prevent cumulative layout shift.**

**§8.8 — Avoid creating new objects or arrays in JSX props — they break `memo` and trigger unnecessary re-renders.**
❌ `<EventsList filters={{ country, category }} />`
✅ `const filters = useMemo(() => ({ country, category }), [country, category]);`

**§8.9 — `useDeferredValue` wraps expensive derived values updated at high frequency (search input, slider).**
```tsx
const deferred = useDeferredValue(searchQuery);
const results = useMemo(() => search(events, deferred), [events, deferred]);
```

**§8.10 — `useTransition` wraps non-urgent state updates that trigger expensive re-renders.**

**§8.11 — Bundle size is checked with `rollup-plugin-visualizer` before adding significant dependencies. Before/after size is included in the change record.**

---

## 9. Accessibility

**§9.1 — Semantic HTML first. Use the correct element before reaching for ARIA.**
❌ `<div onClick={handleClick}>Submit</div>`
✅ `<button onClick={handleClick}>Submit</button>`
**Anti-pattern:** `<div role="button">` gives the role but not keyboard behaviour.

**§9.2 — Every interactive element is reachable and operable by keyboard. Tab order follows visual order.**

**§9.3 — Every form input has a visible `<label>` via `htmlFor`/`id`. Placeholder is not a label.**
❌ `<input placeholder="Country" />`
✅ `<label htmlFor="country">Country</label><input id="country" />`

**§9.4 — Colour is never the sole conveyor of meaning. Pair with text, icon, or pattern.**

**§9.5 — WCAG AA contrast: 4.5:1 for normal text, 3:1 for large text and UI components.**

**§9.6 — Focus is always visible. Do not `outline: none` without a custom focus indicator.**
✅ `:focus-visible { outline: 2px solid var(--color-primary); outline-offset: 2px; }`

**§9.7 — Dynamic content updates are announced via `aria-live` or focus movement.**
✅ `<p aria-live="polite">{total} events found</p>`

**§9.8 — Loading states use `aria-busy="true"` and a `role="status"` spinner with `aria-label`.**
```tsx
<div aria-busy={isPending}>
  {isPending && <span role="status" aria-label="Loading events" className="spinner" />}
</div>
```

**§9.9 — Modals trap focus while open and return focus to the trigger on close.**

**§9.10 — The map canvas has a non-map accessible alternative (visible event list or table). See §12.11.**

**§9.11 — Icon-only buttons have `aria-label`.**
✅ `<button aria-label="Close filter panel"><XIcon /></button>`

**§9.12 — Accessibility is checked two ways: automated `vitest-axe` assertions in the component tests (§13.9, runs in CI), plus a manual keyboard walkthrough before each PR.**
*Why both:* axe catches roughly the machine-checkable half — it cannot tell you tab order is illogical or that focus vanished after a modal closed.

---

## 10. Error Handling & Boundaries

**§10.1 — The router tree is wrapped in a `react-error-boundary` `<ErrorBoundary>` with a `FallbackComponent`. Unhandled render errors show a fallback, not a blank page.**
```tsx
<ErrorBoundary FallbackComponent={PageError}>
  <Routes>…</Routes>
</ErrorBoundary>
```
⚠️ **Do not use `errorElement`.** It is a data-router (`createBrowserRouter`) API and is **silently ignored** under the JSX `<Routes>`/`<Route>` setup this app uses — you get a blank page and no warning. `App.tsx` carries a comment recording this. `errorElement` and `useRouteError` become available only if we migrate to `createBrowserRouter`, which would be an ADR-level change.

**§10.2 — The fallback receives the thrown value via `FallbackProps`, not `useRouteError`.**
```tsx
function PageError({ error }: FallbackProps) {
  return <ErrorMessage message={getErrorMessage(error)} />;
}
```

**§10.3 — TanStack Query errors are handled at the callsite via `isError` + `error`. Never silently swallow.**
```tsx
if (isError) return <ErrorMessage error={error} onRetry={refetch} />;
```
**Anti-pattern:** `data?.map(...)` with no error branch.

**§10.4 — User-facing error messages are human-readable. Never surface raw `Error.message`, API codes, or stack traces.**
```ts
function getErrorMessage(error: unknown): string {
  if (error instanceof ApiError) return friendlyFor(error.status);
  return "Something went wrong. Please try again.";
}
```
⚠️ The class is `ApiError` (`src/api/events.ts`) and it carries `message` + optional `status` — there is **no** `userMessage` field. Its thrown messages currently embed the HTTP code (`"…(HTTP 500)"`) and the fallback renders them, so this rule is partially violated today. Prefer mapping `status` to a human sentence over surfacing `error.message`.

**§10.5 — API functions normalise errors into typed `ApiError` instances before throwing. Components handle `ApiError`, not raw `Response` objects.**

**§10.6 — Production error reporting requires a proper integration (e.g. Sentry) added via ADR — not ad-hoc `console.error` in components.**

**§10.7 — `<ErrorMessage>` accepts an optional `onRetry` callback. Queries expose `refetch`; pass it through.**

**§10.8 — `try/catch` in async event handlers. Do not let unhandled promise rejections reach the window.**
```tsx
async function handleSubmit() {
  try { await mutateAsync(data); }
  catch (err) { setError(getErrorMessage(err)); }
}
```

**§10.9 — Error boundaries do not catch async errors or event handler errors. Handle those at the callsite (§10.8).**

---

## 11. Forms & Validation

> Note: the current codebase has no forms yet. Rules apply when forms land.

**§11.1 — Controlled inputs by default. `value` + `onChange` on every input.**
**Anti-pattern:** mixing controlled and uncontrolled on the same input.

**§11.2 — Every input has an associated `<label>` (§9.3) and a `name` attribute.**

**§11.3 — Validation runs on submit, then stays live as the user corrects. Not on every keystroke.**
```tsx
const [submitted, setSubmitted] = useState(false);
const emailError = submitted && !isValidEmail(email) ? "Invalid email" : null;
```

**§11.4 — Validation logic is a pure function outside the component.**
```ts
function validateFilter(f: EventFilters): Record<string, string> {
  const errors: Record<string, string> = {};
  if (!f.country) errors.country = "Country is required";
  return errors;
}
```

**§11.5 — Error messages are linked via `aria-describedby`.**
```tsx
<input id="email" aria-describedby="email-error" />
<span id="email-error" role="alert">{emailError}</span>
```

**§11.6 — Submit buttons are disabled during async submission.**
```tsx
<button type="submit" disabled={isSubmitting}>
  {isSubmitting ? "Saving…" : "Save"}
</button>
```

**§11.7 — React 19 `useActionState` is the preferred pattern for form actions needing pending + error state.**
```tsx
const [state, submitAction, isPending] = useActionState(saveFilter, null);
<form action={submitAction}>...</form>
```

**§11.8 — For complex multi-field forms, use `useReducer` (§4.5). Do not add a form library without an ADR.**

---

## 12. Map/Geo Rendering

> Cross-ref: ADR-001 (MapLibre GL JS), ADR-012 (TanStack Query), ADR-013 (Plain CSS).

**§12.1 — MapLibre GL is the mapping library. Do not add Mapbox GL JS, Leaflet, or Google Maps without an ADR.**

**§12.2 — The `Map` instance is created once per mount in `useEffect` and destroyed in cleanup via `map.remove()`.**
```tsx
useEffect(() => {
  const map = new maplibregl.Map({
    container: mapContainerRef.current!,
    style: STYLE_URL,
    center: DEFAULT_CENTER,
    zoom: DEFAULT_ZOOM,
  });
  mapRef.current = map;
  return () => { map.remove(); };
}, []);
```
**Anti-pattern:** creating the map in the component body — runs on every render.

**§12.3 — Guard against React StrictMode double-invoke with an initialised flag.**
```tsx
useEffect(() => {
  if (mapRef.current) return;
  mapRef.current = new maplibregl.Map({ ... });
  return () => { mapRef.current?.remove(); mapRef.current = null; };
}, []);
```

**§12.4 — Layers and sources are added in the `map.on("load")` callback, not immediately after construction.**
*Why:* The style may not be loaded when the constructor returns.

**§12.5 — Layer and source IDs are prefixed with the component name.**
✅ `"events-dashboard-points"`, `"events-dashboard-source"`
❌ `"data"`, `"points"` — collide globally.

**§12.6 — Remove layers before removing their source: `removeLayer` → `removeSource`.**
```tsx
if (map.getLayer("events-dashboard-points")) map.removeLayer("events-dashboard-points");
if (map.getSource("events-dashboard-source")) map.removeSource("events-dashboard-source");
```

**§12.7 — Update map data via `source.setData()`, not remove/re-add.**
```tsx
(map.getSource("events-dashboard-source") as maplibregl.GeoJSONSource).setData(newGeoJSON);
```

**§12.8 — GeoJSON construction from event data is memoised.**
```tsx
const geojson = useMemo(() => eventsToGeoJSON(events), [events]);
```

**§12.9 — The map container `<div>` has an explicit height in CSS. Zero-height renders an invisible map silently.**
✅ `.map-container { height: 500px; width: 100%; }`

**§12.10 — Marker instances are tracked in a ref and removed before replacement.**
```tsx
markersRef.current.forEach(m => m.remove());
markersRef.current = events.map(e =>
  new maplibregl.Marker().setLngLat([e.lng, e.lat]).addTo(map)
);
```

**§12.11 — The map canvas has a non-map accessible alternative — a visible event list or summary table alongside the canvas.**
✅ `<canvas aria-label="Event locations map" role="img" />` paired with a visible list.

**§12.12 — Map interaction handlers are registered in the `load` callback and removed in `useEffect` cleanup.**

**§12.13 — Call `map.resize()` via a `ResizeObserver` when the map container's dimensions change (sidebar collapse, panel toggle).**
*Why:* MapLibre sizes itself to the container at construction. A container resize without `map.resize()` leaves a stale canvas.
*Status: not yet implemented* — there is no `ResizeObserver` in `src/components/Map.tsx`. The rule stands for new resizable layouts; it does not describe current behaviour.
```tsx
useEffect(() => {
  if (!mapRef.current) return;
  const observer = new ResizeObserver(() => mapRef.current?.resize());
  observer.observe(mapContainerRef.current!);
  return () => observer.disconnect();
}, []);
```

---

## 13. Testing

> **Installed and enforced.** Vitest 4 + React Testing Library are the test stack, `npm run test` (`vitest run`) is a CI step, and components across `src/` have tests. Setup lives in `src/setupTests.ts` (jest-dom matchers + RTL cleanup) and the `test` block of `vite.config.ts` (jsdom, with the jsdom URL pinned so `window.location` is stable).
>
> **Test files are type-checked** at full strict, same as `src/` (§2.1). Three consequences: a drifted mock is a compile error rather than a silently-passing test; `noUnusedLocals` applies, so an unused import in a test fails `type-check`; and because `npm run build` is `tsc -b && vite build`, **a type error in a test file also fails the production build (and therefore a Vercel deploy)** — a red deploy can originate in a test, not in `src/`.

**§13.0 — Import test globals explicitly: `import { describe, it, expect, vi } from 'vitest'`. Do not enable `vitest/globals`.**
*Why:* every test file in the project already does this, and it keeps the global type surface honest — with `globals: true`, a missing import in *non-test* code can resolve against the injected globals and compile anyway.

**§13.1 — Vitest is the test runner. React Testing Library (RTL) is the component testing layer. Do not add Jest.**
*Why:* Vitest is Vite-native — shares the transform pipeline, no duplicate TS/babel setup.

**§13.2 — Test files are co-located: `EventsDashboard.test.tsx` next to `EventsDashboard.tsx`.**

**§13.3 — Test behaviour, not implementation. Query by accessible role, label, or text — not class names or internals.**
❌ `container.querySelector(".events-dashboard__list")`
✅ `screen.getByRole("list", { name: /events/i })`
**Anti-pattern:** testing that `useState` was called or a specific child rendered.

**§13.4 — Wrap components in required providers. A shared `renderWithProviders` helper is the target; today each test wires `QueryClientProvider` inline, so no such helper exists yet — add one rather than copying the inline block again.**
```tsx
function renderWithProviders(ui: ReactElement) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>
  );
}
```

**§13.5 — Mock `src/api/` fetch functions, not `useQuery`. Use a real `QueryClient` (retry: false).**
✅ `vi.mock("../api/events", () => ({ fetchEvents: vi.fn().mockResolvedValue(mockData) }));`

**§13.6 — Async rendering awaited with `await screen.findBy*` or `waitFor`. No arbitrary `setTimeout`.**

**§13.7 — MapLibre is mocked at module level in tests.**
```ts
vi.mock("maplibre-gl", () => ({
  Map: vi.fn(() => ({ on: vi.fn(), remove: vi.fn(), addLayer: vi.fn(), resize: vi.fn() }))
}));
```

**§13.8 — Every component with user interaction has: a render test, an interaction test, and an error state test.**

**§13.9 — Accessibility tested with `vitest-axe`. Run axe over the rendered container on every component and assert zero violations.**
```tsx
const { container } = render(<FeedbackPrompt eventId="evt-1" />)
const results = await axe(container)
expect(results.violations).toHaveLength(0)
```
*Why this form:* the `toHaveNoViolations()` custom matcher requires `expect.extend` wiring in `setupTests.ts` (vitest-axe ships an empty `extend-expect` and a legacy `Vi`-namespace type augmentation that does not type-check cleanly under Vitest 4). Asserting on `results.violations` needs no matcher registration and is the project convention used across all component tests.

**§13.10 — `userEvent` from `@testing-library/user-event` over `fireEvent` for simulating interactions.**

**§13.11 — No snapshot tests. Delete on sight.**
**Anti-pattern:** `expect(container).toMatchSnapshot()`.

---

## 14. Dependencies

**§14.1 — Browser APIs and React built-ins first. Add a package only when the native equivalent is insufficient.**

**§14.2 — New dependencies require a line in an OpenSpec change record: problem solved, alternatives considered, why native isn't sufficient.**

**§14.3 — Current approved production dependencies: `react`, `react-dom`, `react-router-dom`, `@tanstack/react-query`, `maplibre-gl`, `lucide-react`, `react-error-boundary` (the app-wide fallback, §10.1), and the self-hosted type families `@fontsource/space-grotesk`, `@fontsource/ibm-plex-sans`, `@fontsource/ibm-plex-mono` (Ground Truth visual identity, ADR-015). Adding requires §14.2.**
⚠️ `@types/react-router-dom` is currently in `dependencies` and should not be: it is a v5-era types package, redundant under React Router v7 (which ships its own types), and a type package belongs in `devDependencies` regardless (§14.7). Removing it is a pending cleanup.

**§14.4 — Bundle impact checked with `rollup-plugin-visualizer` before merging a new dep. Before/after size in the change record.**

**§14.5 — Pin exact versions in `package.json`: `npm install --save-exact`.**
❌ `"maplibre-gl": "^5.23.0"`
✅ `"maplibre-gl": "5.23.0"`

**§14.6 — `package-lock.json` is committed. Never `.gitignore` it.**

**§14.7 — Dev dependencies stay in `devDependencies`. Build tools, linters, and type packages do not go in `dependencies`.**

**§14.8 — `npm audit` runs in CI at `--audit-level=moderate` — moderate and above block the build, for both the root and `web/` dependency trees.**
*Note:* this is stricter than the "high and critical" this rule used to claim; a moderate advisory anywhere in the tree turns CI red on every open PR. Fix by bumping the dependency, never by suppressing the finding (same convention as the Go side, ADR-008).

**§14.9 — One package per concern. The approved icon library is `lucide-react`. Adding `react-icons` alongside requires an ADR.**

**§14.10 — Unused dependencies are removed. Run `npx depcheck` before significant PRs.**

**§14.11 — UI component libraries (MUI, Chakra, Radix) require an ADR. Plain CSS + semantic HTML is the current standard (§7).**

---

## 15. Build & Env

**§15.1 — Environment variables exposed to the frontend are prefixed `VITE_`. Non-prefixed vars are not available in the browser bundle.**

**§15.2 — No secrets, API keys, or credentials in the frontend bundle. Anything sensitive belongs in the Go API.**
**Anti-pattern:** `VITE_RESEND_API_KEY` — email sending belongs in the API, not callable from the browser.

**§15.3 — Environment files: the committed template is the repository-root `.env.example` (it carries the `VITE_` section alongside the API vars). Local overrides go in `web/.env.local` (untracked). Deploy-time values are set in Vercel, not in a file.**
```
# .env.example (repo root)
VITE_API_BASE_URL=http://localhost:8080
```
There is no `.env.development` / `.env.production` in this project and no committed example under `web/`. A build gate (`assertDeploymentEnvVars` in `web/vite.config.ts`) hard-fails a staging/production build when a required `VITE_` var is missing, so a misconfigured deploy fails loudly at build rather than silently at runtime.

**§15.3a — The frontend `VITE_` surface: `VITE_API_BASE_URL`, `VITE_ENV` (drives the staging banner, `noindex` meta, and whether `robots.txt`/`sitemap.xml` are emitted), `VITE_ANALYTICS_URL` + `VITE_ANALYTICS_WEBSITE_ID` (Umami; the build gate fails without them), `VITE_SHOW_ERROR_DETAIL`.**

**§15.4 — `VITE_API_BASE_URL` is the single configuration point for the API origin. Never hardcode URLs in source.**
*One deliberate fallback:* `getApiBaseUrl()` in `src/api/events.ts` falls back to `window.location.origin` when the var is unset, which is what makes the local dev proxy and same-origin serving work. Deploys are protected by the §15.3 build gate, so the fallback can't silently ship to prod.
❌ `fetch("http://localhost:8080/v1/events")`
✅ `` fetch(`${import.meta.env.VITE_API_BASE_URL}/v1/events`) ``

**§15.5 — Production build is always `tsc -b && vite build`. TypeScript errors fail the build. Never bypass the type check.**

**§15.6 — `vite preview` verifies the production build locally before deploying.**

**§15.7 — The `base` option in `vite.config.ts` is set explicitly if the app is served from a subpath.**

**§15.8 — Source maps are generated for production and kept private (uploaded to error reporter, not served publicly).**

**§15.9 — Both linters run in CI and failures block the build: ESLint (`npm run lint`, "Run Frontend Lint") and stylelint (`npm run lint:styles`, "Run Frontend Style Lint"). `// eslint-disable` requires an explanatory comment.**
*Why both:* ESLint covers `eslint-plugin-react-hooks` (§3.10) and unused-symbol errors; stylelint covers the colour-token rule (§7). Neither subsumes the other.
ESLint's config extends `@tanstack/eslint-plugin-query`'s `flat/recommended`, which is what makes §5.2 and §5.3 machine-checked rather than review-only.

**§15.10 — Every `vite.config.ts` plugin carries a justification comment.**
*The config is not minimal, by design:* it holds two custom plugins (`robotsMetaPlugin`, `seoFilesPlugin` — per-environment `robots.txt`/`sitemap.xml`), the `assertDeploymentEnvVars` gate, `manualChunks` vendor splitting, the dev proxy, and the vitest block. Each is commented; keep that up. When adding a stable public route, add it to `SITEMAP_ROUTES` — but never `/events/:id`, whose upstream IDs expire.

**§15.11 — `npm run type-check` (`tsc -b --force`) is the standalone type check, and runs as its own CI step ("Run Frontend Type Check"). Type checking also happens inside `npm run build` (§15.5).**
*Why `--force`:* `tsc -b` is incremental and will report success from a stale `.tsbuildinfo`; `--force` makes the check unconditional, which is what you want in CI and before pushing.

---

## Appendix — Decision Log

| # | Decision | Alternatives | Rationale |
|---|---|---|---|
| 1 | Dual purpose: enforcement + onboarding | Two separate docs | Single doc avoids drift; same rules for both audiences |
| 2 | All 15 areas | Subset | Full coverage from v1; gaps create ambiguity in review |
| 3 | Rule + rationale + example + anti-pattern callout | Rule-only | Anti-patterns address common React ecosystem mistakes directly |
| 4 | Cross-ref ADRs; promote 3 new ADRs alongside | No cross-refs | ADRs capture the *why*; rules capture the *how to apply* |
| 5 | Living document, contributor PRs | Maintainer-only | React ecosystem evolves; contributor input keeps the doc current |
| 6 | Flat sections, §N.M numbering | Grouped by lifecycle or severity | Citable in reviews; scannable for onboarding |
| 7 | React 19 features folded into relevant sections | Separate React 19 section | Version silos date quickly; anchoring to topics is more durable |
| 8 | Testing section aspirational (Vitest + RTL) | Defer until framework installed | Better to have the standard ready when tests land than scramble at that point |
| 9 | Added §1.9 barrel file rule (post-review) | Not covered | /react-best-practices review flagged bundle-barrel-imports as a real Vite tree-shaking hazard |
| 10 | Added §3.13 `&&` conditional rendering bug (post-review) | Not covered | One of the most common silent React bugs; warranted an explicit rule |
| 11 | Added §4.9 `useEffect` cleanup mandatory (post-review) | Implicit | /frontend-developer flagged missing cleanup as a frequent memory leak source |
| 12 | Added §12.13 `map.resize()` via `ResizeObserver` (post-review) | Not covered | /frontend-developer flagged stale canvas on container resize as a common MapLibre bug |
| 13 | Fixed `isPending` over `isLoading` in §5.5 (post-review) | | TanStack Query v5 renamed the field; factual correction |
| 14 | Fixed §13.9 axe example to `results.violations` assertion (post-review) | Wire `toHaveNoViolations()` matcher | The matcher example didn't run — vitest-axe's `extend-expect` is empty and its `Vi`-namespace types don't hold under Vitest 4; documented the matcher-free convention already used in every component test (chore-analytics-and-feedback review) |
| 15 | 2026-07-22 accuracy pass: rewrite every rule that misdescribed the codebase, and label unenforced rules as unenforced | Leave as aspirational; or change the code to match | An external review verified each checkable claim against the tree. Two were flatly false — §2.1 claimed `strict: true` (absent from all three tsconfigs) and §13 claimed Vitest/RTL "not yet installed" (installed, and a CI step). §10.1 prescribed `errorElement`, which the app had already abandoned because it is silently ignored under JSX `<Routes>`. §1.4/§3.2 had the export convention backwards. The doc also never mentioned stylelint — the one style gate CI actually runs |
| 16 | Enforcement status stated inline per rule (CI-enforced / local-only / review-only / not implemented) | A single "what CI checks" section | A reviewer reads the rule they are citing, not a table elsewhere. Colour tokens are machine-checked while spacing and z-index are not — that asymmetry is invisible unless it sits next to the rule |
| 17 | `strict: true` deferred to its own PR rather than bundled with the doc corrections | Flip it in the docs PR | Kept the docs PR docs-only and reviewable. The expected fallout in `web/src/` (and the OpenSpec record it would have needed) turned out not to exist — see row 18 |
| 18 | 2026-07-22: `strict: true` enabled in `tsconfig.app.json` **and** `tsconfig.node.json` | Enable only for `app`; or stage it behind individual sub-flags | Measured first: enabling it produces **zero** type errors. A canary (`string \| null` assigned to `string`) confirmed `strictNullChecks` really is live, so the clean result is real and not a config no-op. With no fallout there was no reason to stage it, and `tsconfig.node.json` (which type-checks `vite.config.ts` — the deploy env-var gate and SEO plugins live there) had the same gap |
| 19 | 2026-07-22: test files brought under the type checker by dropping `tsconfig.app.json`'s `exclude`, at full strict | A separate `tsconfig.test.json` with a strictness ratchet | Measured first: 8 test files join the program and produce **0** errors, and a canary (a mock with `id: 42` and `category: 'earthquakes'`) is correctly rejected — so the gate bites. The ratchet existed to absorb a mock-drift backlog that turned out not to exist, because tests already followed §2.8 and typed their mocks against the real API types |
| 20 | Added §13.0 — import vitest helpers explicitly, do not enable `vitest/globals` | Turn on `globals: true` for brevity | All 8 test files already import explicitly, and `globals: true` would let a missing import in *non-test* code resolve against injected globals and compile anyway. Codifying the existing practice costs nothing and keeps the global type surface honest |
| 21 | 2026-07-23: wired `@tanstack/eslint-plugin-query` (`flat/recommended`) into `eslint.config.js`, making §5.2 and §5.3 machine-checked | Drop the unused dependency instead | It was already installed (depcheck flagged it unused), so the supply-chain cost was paid with no benefit. Zero violations on the current tree — a canary (a `queryFn` reading `country` with `country` absent from the `queryKey`) is correctly rejected, so it's live regression protection, not cleanup. `exhaustive-deps` guards against the cache-collision bug where two filter states silently share one entry. The decision-log row was deliberately deferred from PR #173 to avoid a merge conflict with rows 19–20 (PR #172), then added in the archive batch |
