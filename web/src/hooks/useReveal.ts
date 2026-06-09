import { useEffect, useRef, useState } from 'react'

// useReveal — one-shot scroll-into-view reveal. Returns a ref to attach to an
// element (paired with the `.reveal` CSS class) and a `revealed` flag; toggle
// `.is-revealed` from it to drive the fade-up transition. Respects
// prefers-reduced-motion (reveals immediately, no transition) and degrades to
// "always visible" where IntersectionObserver is unavailable. Ground Truth
// motion layer (ADR-015).
export function useReveal<T extends HTMLElement = HTMLElement>() {
  const ref = useRef<T>(null)
  // Start "revealed" when motion is reduced or IntersectionObserver is missing,
  // so the effect never has to set state synchronously (no cascading render).
  const [revealed, setRevealed] = useState(() => {
    // Reveal immediately (no transition) when there's no DOM, no
    // IntersectionObserver, or no matchMedia (e.g. jsdom) — never crash, and
    // never leave content stuck hidden where the observer can't fire.
    if (typeof window === 'undefined' || !('IntersectionObserver' in window)) return true
    return (
      typeof window.matchMedia === 'function' &&
      window.matchMedia('(prefers-reduced-motion: reduce)').matches
    )
  })

  useEffect(() => {
    if (revealed) return
    const node = ref.current
    if (!node) return

    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            setRevealed(true)
            observer.disconnect()
          }
        }
      },
      { rootMargin: '0px 0px -10% 0px', threshold: 0.12 },
    )
    observer.observe(node)
    return () => observer.disconnect()
  }, [revealed])

  return { ref, revealed }
}
