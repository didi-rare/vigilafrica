import type { ConceptMarkProps } from './types'

// C1 — Vigil Reticle (the incumbent, optically refined): a targeting graticule
// framing a live point. "Watching a place." Refinements over the shipped
// BrandMark: heavier ring + ticks for 16px legibility, graticule parallels
// pulled inside the ring so they don't clip the silhouette.
export function VigilReticle({ size = 32, className, title = 'Vigil Reticle' }: ConceptMarkProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 32 32" role="img" aria-label={title} className={className} fill="none">
      <circle cx="16" cy="16" r="12" className="cmark-line" strokeWidth="1.8" />
      <path className="cmark-soft" strokeWidth="1" d="M6.5 12h19 M6.5 20h19" />
      <path
        className="cmark-accent"
        strokeWidth="2"
        strokeLinecap="round"
        d="M16 2.8v4.4 M16 24.8v4.4 M2.8 16h4.4 M24.8 16h4.4"
      />
      <circle cx="16" cy="16" r="3.2" className="cmark-fill" />
    </svg>
  )
}
