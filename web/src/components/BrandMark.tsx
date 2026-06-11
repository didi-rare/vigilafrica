type Props = {
  size?: number
  className?: string
  title?: string
}

// BrandMark — "Epicenter Contours" (ADR-016): an event epicentre rendered as
// concentric topographic contour rings — dashed outer contour, neutral mid
// ring, amber accent ring, and the amber live point at the core. Events as
// phenomena on mapped terrain, which is the product. Parts are themed via the
// brand-mark__* CSS classes so the mark inherits the token palette anywhere it
// renders. Selected from five Phase-A candidates (feat-ground-truth-identity).
export function BrandMark({ size = 30, className, title = 'VigilAfrica' }: Props) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 32 32"
      role="img"
      aria-label={title}
      className={className}
      fill="none"
    >
      {/* outer contour — broken, like a dashed map contour line */}
      <circle
        cx="16"
        cy="16"
        r="14"
        className="brand-mark__graticule"
        strokeWidth="1.2"
        strokeDasharray="5 4"
      />
      {/* mid contour */}
      <circle cx="16" cy="16" r="10" className="brand-mark__ring" strokeWidth="1.6" />
      {/* accent contour */}
      <circle cx="16" cy="16" r="6" className="brand-mark__ticks" strokeWidth="1.8" />
      {/* the live point — the epicentre */}
      <circle cx="16" cy="16" r="2.4" className="brand-mark__point" />
    </svg>
  )
}
