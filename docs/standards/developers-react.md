# React Coding Standards — VigilAfrica

**Scope:** all frontend code under `web/` in this repository.
**Stack:** React 19, TypeScript, Vite, TanStack Query v5, React Router v7, MapLibre GL, plain CSS.
**Audience:** contributors writing React/TS code, and reviewers enforcing standards via `/openspec-review`.
**Status:** living document. Any contributor may open a PR proposing changes; maintainer approval merges.

Each rule: **statement → why → example (where useful) → anti-pattern callout**. Rules are numbered (`§4.2`) so reviewers can cite them directly.

Cross-references:
- ADR-012 — TanStack Query as server-state layer.
- ADR-013 — Plain CSS over CSS-in-JS.
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

**§1.2 — Folder structure: `src/api/` (typed API clients), `src/components/` (shared UI), `src/pages/` (route-level components), `src/data/` (static data, constants), `src/assets/` (images, fonts).**
*Why:* Separation of concerns. `api/` knows about network; `components/` knows about rendering; `pages/` composes them into routes.
❌ A flat `src/` with every `.tsx` mixed together.
✅ Current layout (`src/api/events.ts`, `src/components/EventsDashboard.tsx`, `src/pages/`).

**§1.3 — Component files are `PascalCase.tsx`; hooks are `useThing.ts`; utilities are `camelCase.ts`. Consistent per folder.**
*Why:* Filename telegraphs what the file exports. `EventsDashboard.tsx` obviously exports a component; `useEvents.ts` a hook.

**§1.4 — One default-exported component per file. The file name equals the component name.**
*Why:* `grep` for the name finds the file; IDE auto-imports work predictably.
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

**§2.1 — `strict: true` in `tsconfig.json`. Do not disable individual strict sub-flags.**
*Why:* Strict mode catches an order of magnitude more bugs at compile time. Disabling a sub-flag is a permanent tax on every future contributor.

**§2.2 — No `any`. If you genuinely need "any shape", use `unknown` and narrow.**
*Why:* `any` disables type checking for everything it touches. `unknown` forces a narrowing check.
❌ `function parse(raw: any) { return raw.data; }`
✅ `function parse(raw: unknown) { if (isEventList(raw)) return raw.data; throw new Error("..."); }`
**Anti-pattern:** reaching for `any` to silence a type error. The error is the code telling you something; fix the shape.

**§2.3 — No non-null assertions (`!`) without an inline comment explaining why the value cannot be null at that point.**
*Why:* `!` is a runtime crash waiting to happen.
❌ `const map = mapRef.current!;`
✅ `const map = mapRef.current!; // set in onLoad, only read after map loaded`

**§2.4 — Prefer `type` aliases for unions, primitives, and function signatures. Use `interface` only when declaration merging is needed.**
*Why:* `type` is more flexible. Pick one; the project default is `type`.

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

**§3.2 — One default-exported component per file. Internal helpers live below the component, unexported.**

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
Cross-ref: ADR (TanStack Query as server-state layer).

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

**§5.3 — One `QueryClient` instance, created at app root with project defaults. Never instantiate inside a component.**
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

**§6.2 — Route definitions live in one place: `src/main.tsx` or a dedicated `src/routes.tsx`.**

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

> Cross-ref: ADR (Plain CSS over CSS-in-JS — to be formalised).

**§7.1 — Plain CSS files co-located with components. No CSS-in-JS, no Tailwind, no CSS Modules.**
*Why:* Zero runtime cost, no build transformation, readable by anyone.
**Anti-pattern:** installing `styled-components` or `@emotion/react` without an ADR.

**§7.2 — Each component imports exactly one CSS file: its own. Cross-component sharing via CSS custom properties.**
```css
/* index.css */
:root { --color-primary: #1a6b3c; --spacing-md: 1rem; }
```

**§7.3 — CSS class names are `kebab-case`, prefixed with the component root name.**
✅ `.events-dashboard__filter`, `.events-dashboard__list`
❌ `.filter`, `.list` — collides globally.

**§7.4 — Layout lives in CSS. Inline `style` props are reserved for truly dynamic values (map marker position, data-driven widths).**
❌ `<div style={{ display: 'flex', gap: '1rem' }}>`
✅ `<div className="events-dashboard__row">`

**§7.5 — Colours, spacing, typography, and z-index are CSS custom properties defined in `index.css`. Never hardcode values in component CSS.**
❌ `color: #1a6b3c;`
✅ `color: var(--color-primary);`

**§7.6 — Responsive breakpoints are defined in `index.css`. Components do not hardcode pixel values for breakpoints.**

**§7.7 — No `!important`. Fix the selector hierarchy instead.**

**§7.8 — Animations use CSS `transition` and `@keyframes`. JS-driven animations are reserved for effects CSS cannot achieve.**

**§7.9 — Dark mode via `data-theme="dark"` on `<html>` with CSS custom property overrides.**
```css
[data-theme="dark"] { --color-primary: #4caf7d; --bg: #1a1a1a; }
```

**§7.10 — `z-index` values are named custom properties in `index.css`. Never hardcode a `z-index` value in a component file.**
```css
:root { --z-modal: 300; --z-nav: 200; --z-map-controls: 100; }
```

---

## 8. Performance

**§8.1 — Measure before optimising. Use React DevTools Profiler to identify bottlenecks. Do not add `memo`, `useMemo`, or `useCallback` speculatively.**

**§8.2 — Route-level code splitting is mandatory (§6.3). Library-level splitting (MapLibre) is the next priority.**

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

**§9.12 — Accessibility is checked with a keyboard walkthrough and `axe-core` browser extension before each PR.**

---

## 10. Error Handling & Boundaries

**§10.1 — Every route has an `errorElement`. Unhandled render errors show a fallback, not a blank page.**
```tsx
<Route path="/events" element={<EventsPage />} errorElement={<PageError />} />
```

**§10.2 — Use `useRouteError` inside `<PageError>` to access the thrown value.**
```tsx
function PageError() {
  const error = useRouteError();
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
  if (error instanceof APIError) return error.userMessage;
  return "Something went wrong. Please try again.";
}
```

**§10.5 — API functions normalise errors into typed `APIError` instances before throwing. Components handle `APIError`, not raw `Response` objects.**

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

> Vitest + React Testing Library are not yet installed. Rules apply once the framework lands.

**§13.1 — Vitest is the test runner. React Testing Library (RTL) is the component testing layer. Do not add Jest.**
*Why:* Vitest is Vite-native — shares the transform pipeline, no duplicate TS/babel setup.

**§13.2 — Test files are co-located: `EventsDashboard.test.tsx` next to `EventsDashboard.tsx`.**

**§13.3 — Test behaviour, not implementation. Query by accessible role, label, or text — not class names or internals.**
❌ `container.querySelector(".events-dashboard__list")`
✅ `screen.getByRole("list", { name: /events/i })`
**Anti-pattern:** testing that `useState` was called or a specific child rendered.

**§13.4 — Wrap components in required providers via a `renderWithProviders` helper.**
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

**§13.9 — Accessibility tested with `vitest-axe`. Run `expect(await axe(container)).toHaveNoViolations()` on every component.**

**§13.10 — `userEvent` from `@testing-library/user-event` over `fireEvent` for simulating interactions.**

**§13.11 — No snapshot tests. Delete on sight.**
**Anti-pattern:** `expect(container).toMatchSnapshot()`.

---

## 14. Dependencies

**§14.1 — Browser APIs and React built-ins first. Add a package only when the native equivalent is insufficient.**

**§14.2 — New dependencies require a line in an OpenSpec change record: problem solved, alternatives considered, why native isn't sufficient.**

**§14.3 — Current approved production dependencies: `react`, `react-dom`, `react-router-dom`, `@tanstack/react-query`, `maplibre-gl`, `lucide-react`. Adding requires §14.2.**

**§14.4 — Bundle impact checked with `rollup-plugin-visualizer` before merging a new dep. Before/after size in the change record.**

**§14.5 — Pin exact versions in `package.json`: `npm install --save-exact`.**
❌ `"maplibre-gl": "^5.23.0"`
✅ `"maplibre-gl": "5.23.0"`

**§14.6 — `package-lock.json` is committed. Never `.gitignore` it.**

**§14.7 — Dev dependencies stay in `devDependencies`. Build tools, linters, and type packages do not go in `dependencies`.**

**§14.8 — `npm audit` runs in CI. High and critical vulnerabilities block the build.**

**§14.9 — One package per concern. The approved icon library is `lucide-react`. Adding `react-icons` alongside requires an ADR.**

**§14.10 — Unused dependencies are removed. Run `npx depcheck` before significant PRs.**

**§14.11 — UI component libraries (MUI, Chakra, Radix) require an ADR. Plain CSS + semantic HTML is the current standard (§7).**

---

## 15. Build & Env

**§15.1 — Environment variables exposed to the frontend are prefixed `VITE_`. Non-prefixed vars are not available in the browser bundle.**

**§15.2 — No secrets, API keys, or credentials in the frontend bundle. Anything sensitive belongs in the Go API.**
**Anti-pattern:** `VITE_RESEND_API_KEY` — email sending belongs in the API, not callable from the browser.

**§15.3 — Environment files: `.env` (all), `.env.development` (local), `.env.production` (prod overrides). Only `.env.example` is committed.**
```
# .env.example
VITE_API_BASE_URL=http://localhost:8080
```

**§15.4 — `VITE_API_BASE_URL` is the single configuration point for the API origin. Never hardcode URLs in source.**
❌ `fetch("http://localhost:8080/v1/events")`
✅ `` fetch(`${import.meta.env.VITE_API_BASE_URL}/v1/events`) ``

**§15.5 — Production build is always `tsc -b && vite build`. TypeScript errors fail the build. Never bypass the type check.**

**§15.6 — `vite preview` verifies the production build locally before deploying.**

**§15.7 — The `base` option in `vite.config.ts` is set explicitly if the app is served from a subpath.**

**§15.8 — Source maps are generated for production and kept private (uploaded to error reporter, not served publicly).**

**§15.9 — ESLint runs in CI with `npm run lint`. Lint errors fail the build. `// eslint-disable` requires an explanatory comment.**

**§15.10 — `vite.config.ts` is kept minimal. Each plugin requires a justification comment.**

**§15.11 — `npm run type-check` (`tsc --noEmit`) is available as a standalone script.**

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
