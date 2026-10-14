// Gauges and the page contract.

import type { Dict, State } from './core'

export interface Page {
  el: HTMLElement
  live: boolean // wants Monitor samples while shown
  update(s: State): void
  sample?(s: Dict): void
  shown?(): void
}

// paintRange fills a range input up to its value with the flavor gradient.
export function paintRange(range: HTMLInputElement) {
  const lo = Number(range.min), hi = Number(range.max)
  const p = hi > lo ? ((Number(range.value) - lo) * 100) / (hi - lo) : 0
  range.style.background = `linear-gradient(90deg, var(--g1), var(--g3) ${p}%, var(--surface-3) ${p}%)`
}

const R = 62
const C = 2 * Math.PI * R
const ARC = 0.75 * C

export function gaugeHtml(key: string, label: string): string {
  return `<div class="gauge" data-g="${key}">
    <svg viewBox="0 0 160 150">
      <g transform="rotate(135 80 80)">
        <circle class="track" cx="80" cy="80" r="${R}" fill="none" stroke-width="11" stroke-linecap="round" stroke-dasharray="${ARC} ${C}"/>
        <circle class="bar" cx="80" cy="80" r="${R}" fill="none" stroke="url(#ggrad)" stroke-width="11" stroke-linecap="round" stroke-dasharray="${ARC} ${C}" stroke-dashoffset="${ARC}"/>
      </g>
      <text class="val" x="80" y="86" text-anchor="middle">–</text>
      <text class="unit" x="80" y="108" text-anchor="middle"></text>
    </svg>
    <div class="label">${label}</div>
    <div class="sub num">&nbsp;</div>
  </div>`
}

export function setGauge(root: ParentNode, key: string, frac: number | null, text: string, unit: string, sub = '', hot = false) {
  const g = root.querySelector(`[data-g="${key}"]`)
  if (!g) return
  const bar = g.querySelector('.bar') as SVGCircleElement
  const f = frac == null || !isFinite(frac) ? 0 : Math.max(0, Math.min(1, frac))
  bar.setAttribute('stroke-dashoffset', String(ARC * (1 - f)))
  bar.setAttribute('stroke', hot ? 'var(--danger)' : 'url(#ggrad)')
  g.querySelector('.val')!.textContent = text
  g.querySelector('.unit')!.textContent = unit
  g.querySelector('.sub')!.textContent = sub || ' '
}
