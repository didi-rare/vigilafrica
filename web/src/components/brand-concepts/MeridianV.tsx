import type { ConceptMarkProps } from './types'

// C3 — Meridian V: a letterform mark — the V of Vigil constructed from two
// converging meridian curves crossing a latitude rule, with the live point at
// the vertex. The only typographic concept of the set; pairs naturally with
// the Space Grotesk wordmark.
export function MeridianV({ size = 32, className, title = 'Meridian V' }: ConceptMarkProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 32 32" role="img" aria-label={title} className={className} fill="none">
      {/* latitude rule */}
      <path className="cmark-soft" strokeWidth="1.2" d="M4 9.5h24" />
      {/* converging meridians forming the V */}
      <path className="cmark-line" strokeWidth="2" strokeLinecap="round" d="M8 5.5c1.4 7.4 4.4 14.2 8 19.5" />
      <path className="cmark-line" strokeWidth="2" strokeLinecap="round" d="M24 5.5c-1.4 7.4-4.4 14.2-8 19.5" />
      {/* live point at the vertex */}
      <circle cx="16" cy="25" r="2.6" className="cmark-fill" />
    </svg>
  )
}
