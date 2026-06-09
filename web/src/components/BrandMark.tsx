type Props = {
  size?: number
  className?: string
  title?: string
}

// BrandMark — the VigilAfrica "vigil reticle": a cartographic targeting
// graticule (the watch) framing a single live point (the place). The amber
// crosshair ticks + centre point read as "locating / monitoring", which is the
// product. Parts are themed via CSS classes (currentColor-free) so the mark
// inherits the token palette and works on any surface. Ground Truth rebrand.
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
      {/* outer graticule ring */}
      <circle cx="16" cy="16" r="12.5" className="brand-mark__ring" strokeWidth="1.5" />
      {/* faint graticule parallels (lat lines) */}
      <path className="brand-mark__graticule" strokeWidth="1" d="M5 12h22 M5 20h22" />
      {/* reticle crosshair ticks at N / E / S / W */}
      <path
        className="brand-mark__ticks"
        strokeWidth="1.75"
        strokeLinecap="round"
        d="M16 2.5V7.5 M16 24.5v5 M2.5 16h5 M24.5 16h5"
      />
      {/* live point */}
      <circle cx="16" cy="16" r="3" className="brand-mark__point" />
    </svg>
  )
}
