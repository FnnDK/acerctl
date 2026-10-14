import { App, act, caps, el, fanIds, num, type Dict } from '../core'
import { t } from '../i18n'
import { icon } from '../icons'
import { paintRange as paint, type Page } from '../widgets'

const fanArt = `<svg viewBox="0 0 100 100">
  <circle cx="50" cy="50" r="47" fill="#141117" stroke="#2e2833" stroke-width="2"/>
  <circle cx="50" cy="50" r="42" fill="none" stroke="#221e26" stroke-width="1.2" stroke-dasharray="2 3"/>
  <g class="blades">${[0, 1, 2, 3, 4, 5, 6].map(i =>
    `<path transform="rotate(${(i * 360) / 7} 50 50)" d="M50 41 C 50 30, 55 17, 67 11 C 74 17, 72 31, 58 44 Z" fill="#2c2731" stroke="url(#ggrad)" stroke-width="1.4" stroke-linejoin="round"/>`).join('')}
    <circle cx="50" cy="50" r="11" fill="#1f1b23" stroke="url(#ggrad)" stroke-width="2"/>
    <circle cx="50" cy="50" r="3" style="fill:var(--g3)"/>
  </g>
</svg>`

export function fans(): Page {
  const root = el(`<section>
    <div class="page-head"><div><h1>${t('fans.title')}</h1><p>${t('fans.sub')}</p></div>
      <div class="row"><span class="muted small">${t('fans.all')}</span>
        <div class="seg" data-all><button data-m="auto">${t('fan.auto')}</button><button data-m="max">${t('fan.max')}</button></div></div>
    </div>
    <div class="grid fan-grid" data-cards></div>
    <div class="note mt hidden" data-nocurve>${icon('info')}<span>${t('fans.nocurve')} ${t('fans.floor')}</span></div>
  </section>`)
  const cards = root.querySelector('[data-cards]') as HTMLElement
  let built = ''
  const dragging = new Set<string>()

  root.querySelector('[data-all]')!.addEventListener('click', e => {
    const b = (e.target as HTMLElement).closest('button') as HTMLButtonElement | null
    if (b) act(App.SetFanMode('all', b.dataset.m!))
  })

  function build() {
    const ids = fanIds()
    const modes: string[] = caps().fans?.modes ?? []
    cards.innerHTML = ids.map(id => {
      const [lo, hi] = caps().fans?.range?.[id] ?? [0, 100]
      return `<div class="card fan-card" data-fan="${id}">
        <div class="fan-visual">${fanArt}</div>
        <div class="stack" style="gap:6px">
          <h3>${t('fan.' + id) === 'fan.' + id ? id.toUpperCase() : t('fan.' + id)}</h3>
          <div class="fan-rpm"><span data-rpm>–</span> <small>RPM</small></div>
        </div>
        <div class="fan-controls stack">
          <div class="seg fill" data-modes>${modes.map(m => `<button data-m="${m}">${t('fan.' + m)}</button>`).join('')}</div>
          <div class="stack" data-manual style="gap:6px">
            <span class="muted small">${t('fan.speed')}</span>
            <div class="slider-row"><input type="range" min="${lo}" max="${hi}" step="1" value="${lo}"><output class="num">–</output></div>
          </div>
        </div>
      </div>`
    }).join('')
    for (const card of cards.querySelectorAll<HTMLElement>('[data-fan]')) {
      const id = card.dataset.fan!
      const range = card.querySelector('input[type=range]') as HTMLInputElement
      const out = card.querySelector('output')!
      card.querySelector('[data-modes]')!.addEventListener('click', e => {
        const b = (e.target as HTMLElement).closest('button') as HTMLButtonElement | null
        if (!b || b.classList.contains('on')) return
        act(App.SetFanMode(id, b.dataset.m!)) // manual keeps the current PWM
      })
      range.addEventListener('pointerdown', () => dragging.add(id))
      range.addEventListener('input', () => { out.textContent = range.value + ' %'; paint(range) })
      range.addEventListener('change', () => {
        dragging.delete(id)
        act(App.SetFanSpeed(id, Number(range.value)))
      })
    }
  }

  function spin(id: string, rpm: number) {
    const g = cards.querySelector(`[data-fan="${id}"] .blades`) as SVGGElement | null
    if (!g) return
    if (!rpm) { g.style.animationPlayState = 'paused'; return }
    g.style.animationDuration = `${Math.max(0.18, 900 / rpm).toFixed(2)}s`
    g.style.animationPlayState = 'running'
  }

  return {
    el: root,
    live: true,
    update(s) {
      const key = JSON.stringify([fanIds(), caps().fans])
      if (key !== built) { built = key; build() }
      root.querySelector('[data-nocurve]')!.classList.toggle('hidden', !fanIds().length || caps().fans?.curve)
      const st: Dict = s.fans?.State ?? {}
      for (const id of fanIds()) {
        const f = st[id] ?? {}
        const card = cards.querySelector(`[data-fan="${id}"]`)!
        card.querySelectorAll<HTMLButtonElement>('[data-modes] button').forEach(b => b.classList.toggle('on', b.dataset.m === f.mode))
        card.querySelector('[data-manual]')!.classList.toggle('hidden', f.mode !== 'manual')
        if (!dragging.has(id) && f.mode === 'manual') {
          const range = card.querySelector('input[type=range]') as HTMLInputElement
          range.value = String(f.speed ?? 0)
          paint(range)
          card.querySelector('output')!.textContent = `${f.speed ?? 0} %`
        }
        if (f.rpm != null) { card.querySelector('[data-rpm]')!.textContent = num(f.rpm); spin(id, f.rpm) }
      }
      const modes = fanIds().map(id => st[id]?.mode)
      root.querySelectorAll<HTMLButtonElement>('[data-all] button').forEach(b => b.classList.toggle('on', modes.length > 0 && modes.every(m => m === b.dataset.m)))
    },
    sample(x) {
      for (const [id, rpm] of Object.entries<number>(x.fans ?? {})) {
        const n = cards.querySelector(`[data-fan="${id}"] [data-rpm]`)
        if (n) { n.textContent = num(rpm); spin(id, rpm) }
      }
    },
  }
}
