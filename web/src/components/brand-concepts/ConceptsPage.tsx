import type { ComponentType } from 'react'
import type { ConceptMarkProps } from './types'
import { VigilReticle } from './VigilReticle'
import { SentinelPin } from './SentinelPin'
import { MeridianV } from './MeridianV'
import { EpicenterContours } from './EpicenterContours'
import { RadarSweep } from './RadarSweep'
import './ConceptsPage.css'

type Concept = {
  id: string
  name: string
  thesis: string
  dfii: string
  Mark: ComponentType<ConceptMarkProps>
}

// DFII = impact + fit + feasibility + performance − consistency-risk (max 15).
const CONCEPTS: Concept[] = [
  {
    id: 'C1',
    name: 'Vigil Reticle (incumbent, refined)',
    thesis: 'A targeting graticule framing a live point — "watching a place". Continuity with everything shipped.',
    dfii: '13 · impact 4 / fit 5 / feas 5 / perf 5 − risk 1 (lowest risk: already proven in the nav)',
    Mark: VigilReticle,
  },
  {
    id: 'C2',
    name: 'Sentinel Pin',
    thesis: 'A map pin under rising pulse arcs — the most literal "monitored location"; instantly legible to non-technical partners.',
    dfii: '12 · impact 4 / fit 5 / feas 4 / perf 5 − risk 2 (pin shapes are a crowded field)',
    Mark: SentinelPin,
  },
  {
    id: 'C3',
    name: 'Meridian V',
    thesis: 'The V of Vigil built from converging meridians over a latitude rule — the only letterform; strongest wordmark pairing.',
    dfii: '12 · impact 5 / fit 4 / feas 4 / perf 5 − risk 2 (letterforms read abstract at 16px)',
    Mark: MeridianV,
  },
  {
    id: 'C4',
    name: 'Epicenter Contours',
    thesis: 'An event epicentre as topographic contour rings — events as phenomena on mapped terrain; calm, data-first.',
    dfii: '13 · impact 4 / fit 5 / feas 5 / perf 5 − risk 1 (concentric rings scale perfectly)',
    Mark: EpicenterContours,
  },
  {
    id: 'C5',
    name: 'Radar Sweep',
    thesis: 'A station display mid-scan with a detected blip — the most operational tone; leans mission-control.',
    dfii: '11 · impact 5 / fit 4 / feas 4 / perf 5 − risk 3 (sweep wedge muddies at 16px; tone may read military)',
    Mark: RadarSweep,
  },
]

// Local-only review page (DEV route, Phase A of feat-ground-truth-identity).
// Renders every candidate at favicon (16), nav (32) and display (120) sizes
// plus a nav-context lockup. Deleted after the maintainer selects a winner.
export function ConceptsPage() {
  return (
    <div className="container concepts-page">
      <span className="section-label">Phase A · Brand mark candidates</span>
      <h1 className="section-title">Five concepts, one winner</h1>
      <ul className="concepts-list" role="list">
        {CONCEPTS.map(({ id, name, thesis, dfii, Mark }) => (
          <li key={id} className="concept-card glass-effect">
            <div className="concept-card__sizes">
              <span className="concept-card__size"><Mark size={16} /><code>16</code></span>
              <span className="concept-card__size"><Mark size={32} /><code>32</code></span>
              <span className="concept-card__size concept-card__size--display"><Mark size={120} /><code>120</code></span>
            </div>
            <div className="concept-card__meta">
              <h2 className="concept-card__name"><span className="concept-card__id">{id}</span> {name}</h2>
              <p className="concept-card__thesis">{thesis}</p>
              <p className="concept-card__dfii">DFII {dfii}</p>
              <span className="concept-card__lockup">
                <Mark size={30} />
                <span className="logo-text">VigilAfrica</span>
                <span className="nav-station" aria-hidden="true">NG·GH</span>
              </span>
            </div>
          </li>
        ))}
      </ul>
    </div>
  )
}
