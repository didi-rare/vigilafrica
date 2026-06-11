// Shared prop contract for the five candidate brand marks (Phase A,
// feat-ground-truth-identity). Mirrors BrandMark's API so the winner can be
// swapped in without touching the nav. This folder is temporary — deleted
// after the maintainer selects a mark.
export type ConceptMarkProps = {
  size?: number
  className?: string
  title?: string
}
