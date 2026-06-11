import type { ConceptMarkProps } from './types'

// C5 — Radar Sweep: a station display mid-scan — ring, amber sweep sector
// with a hard leading edge, and a detected blip inside the swept area. The
// most operational/mission-control tone of the set.
export function RadarSweep({ size = 32, className, title = 'Radar Sweep' }: ConceptMarkProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 32 32" role="img" aria-label={title} className={className} fill="none">
      <circle cx="16" cy="16" r="12.5" className="cmark-line" strokeWidth="1.8" />
      {/* swept sector (translucent wash) + leading edge */}
      <path className="cmark-wash" d="M16 16V3.5A12.5 12.5 0 0 1 26.8 9.75Z" />
      <path className="cmark-accent" strokeWidth="1.8" strokeLinecap="round" d="M16 16 26.8 9.75" />
      {/* detected blip inside the swept area */}
      <circle cx="20.5" cy="10.5" r="2" className="cmark-fill" />
      {/* cardinal ticks */}
      <path className="cmark-soft" strokeWidth="1.4" strokeLinecap="round" d="M16 1.6v2.2 M16 28.2v2.2 M1.6 16h2.2 M28.2 16h2.2" />
    </svg>
  )
}
