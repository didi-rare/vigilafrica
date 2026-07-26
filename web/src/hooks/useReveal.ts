import { useEffect, useRef, useState } from 'react'

// useReveal — one-shot scroll-into-view reveal with a load-time backstop.
// Returns a ref to attach to an element (paired with the `.reveal` CSS class)
// and a `revealed` flag; toggle `.is-revealed` from it to drive the fade-up
// transition. Reveals on first intersection, OR once the page has loaded and
// settled (so content below the fold on landing is never left invisible —
// see the backstop note below), whichever comes first. Respects
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

    // Backstop — never leave content invisible on load. The observer only
    // reveals on intersection, so a section below the fold at load (common on
    // laptop viewports, and made worse by the layout shift when the lazy
    // dashboard + map mount) would sit at opacity:0 until the user happens to
    // scroll to it — a blank void on landing. Once the page has loaded and
    // laid out, reveal anything still hidden. It still fades via the CSS
    // transition; sections the user scrolls to first are already handled above.
    let rafId = 0
    const revealNow = () => {
      rafId = requestAnimationFrame(() => setRevealed(true))
    }
    if (document.readyState === 'complete') {
      revealNow()
    } else {
      window.addEventListener('load', revealNow, { once: true })
    }
    // Absolute floor, in case `load` already fired and was missed or never fires.
    const timerId = window.setTimeout(() => setRevealed(true), 1500)

    return () => {
      observer.disconnect()
      window.removeEventListener('load', revealNow)
      if (rafId) cancelAnimationFrame(rafId)
      clearTimeout(timerId)
    }
  }, [revealed])

  return { ref, revealed }
}
