import type { ConceptMarkProps } from './types'

// C2 — Sentinel Pin: a map pin (the place) under rising pulse arcs (the
// watch). The most literal "monitored location" of the set — instantly
// readable as both "map" and "alert" even at favicon size.
export function SentinelPin({ size = 32, className, title = 'Sentinel Pin' }: ConceptMarkProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 32 32" role="img" aria-label={title} className={className} fill="none">
      {/* pin body */}
      <path
        className="cmark-line"
        strokeWidth="1.8"
        strokeLinejoin="round"
        d="M16 28.5C12.4 23.6 9.6 20 9.6 15.4a6.4 6.4 0 1 1 12.8 0c0 4.6-2.8 8.2-6.4 13.1Z"
      />
      {/* live point in the pin head */}
      <circle cx="16" cy="15.2" r="2.4" className="cmark-fill" />
      {/* pulse arcs */}
      <path className="cmark-accent" strokeWidth="1.8" strokeLinecap="round" d="M10.6 5.6a8.6 8.6 0 0 1 10.8 0" />
      <path className="cmark-accent" strokeWidth="1.8" strokeLinecap="round" d="M13 8.4a4.8 4.8 0 0 1 6 0" />
    </svg>
  )
}
