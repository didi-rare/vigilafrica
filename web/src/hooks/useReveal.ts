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
    if (typeof window === 'undefined') return true
    return (
      window.matchMedia('(prefers-reduced-motion: reduce)').matches ||
      !('IntersectionObserver' in window)
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
