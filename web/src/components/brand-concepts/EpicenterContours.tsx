import type { ConceptMarkProps } from './types'

// C4 — Epicenter Contours: an event epicentre read as topographic contour
// rings — the outermost broken like a dashed map contour, the inner ring
// carrying the accent. The most "data/terrain" concept: events are phenomena
// on a mapped landscape.
export function EpicenterContours({ size = 32, className, title = 'Epicenter Contours' }: ConceptMarkProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 32 32" role="img" aria-label={title} className={className} fill="none">
      <circle cx="16" cy="16" r="14" className="cmark-soft" strokeWidth="1.2" strokeDasharray="5 4" />
      <circle cx="16" cy="16" r="10" className="cmark-line" strokeWidth="1.6" />
      <circle cx="16" cy="16" r="6" className="cmark-accent" strokeWidth="1.8" />
      <circle cx="16" cy="16" r="2.4" className="cmark-fill" />
    </svg>
  )
}
