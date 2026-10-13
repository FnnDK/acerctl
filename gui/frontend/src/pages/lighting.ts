import { App, act, caps, el } from '../core'
import { t } from '../i18n'
import { icon } from '../icons'
import type { Page } from '../widgets'

// Schematic keyboard: rows of key widths (in key units).
const ROWS = [
  Array(15).fill(0.93),
  [1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 2],
  [1.5, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1.5],
  [1.8, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 2.2],
  [2.3, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 2.7],
  [1.2, 1.2, 1.2, 1.2, 6, 1.2, 1.2, 1, 1, 1],
]

function keyboardSvg(): string {
  const U = 34, G = 4, W = 15 * U, rows: string[] = []
  ROWS.forEach((row, r) => {
    const total = row.reduce((a, b) => a + b, 0)
    const scale = (W - G * (row.length - 1)) / (total * U)
    let x = 14
    for (const w of row) {
      const kw = w * U * scale
      rows.push(`<rect class="key" x="${x.toFixed(1)}" y="${14 + r * (U + G) - (r === 0 ? 0 : 8)}" width="${kw.toFixed(1)}" height="${r === 0 ? U - 10 : U}" rx="5"/>`)
      x += kw + G
    }
  })
  return `<svg class="kbd-art" viewBox="0 0 ${W + 28} ${6 * (U + G) + 12}"><rect class="body" x="1" y="1" width="${W + 26}" height="${6 * (U + G) + 10}" rx="14"/>${rows.join('')}</svg>`
}

export function lighting(): Page {
  const root = el(`<section>
    <div class="page-head"><div><h1>${t('light.title')}</h1><p>${t('light.sub')}</p></div></div>
    <div class="card">${keyboardSvg()}</div>
    <div class="card mt">
      <div class="setting"><div class="label"><b>${t('light.state')}</b><span>${t('light.state_hint')}</span></div><span class="badge" data-on></span></div>
      <div class="setting"><div class="label"><b>${t('light.timeout')}</b><span>${t('light.timeout_hint')}</span></div><div class="seg" data-timeouts></div></div>
    </div>
    <div class="note mt hidden" data-rgb>${icon('info')}<span>${t('light.rgb')}</span></div>
  </section>`)
  const art = root.querySelector('.kbd-art')!
  const seg = root.querySelector('[data-timeouts]') as HTMLElement
  let built = ''
  seg.onclick = e => {
    const b = (e.target as HTMLElement).closest('button') as HTMLButtonElement | null
    if (b && !b.classList.contains('on')) act(App.SetKeyboardTimeout(Number(b.dataset.s)))
  }

  return {
    el: root,
    live: true, // the daemon refreshes the on/off state while someone watches the Monitor
    update(s) {
      const kb = caps().kbd_backlight ?? {}
      const tos: number[] = kb.timeouts ?? []
      const key = JSON.stringify(tos)
      if (key !== built) {
        built = key
        seg.innerHTML = tos.map(v => `<button data-s="${v}">${v === 0 ? t('light.always') : t('light.after', v)}</button>`).join('')
      }
      const k = s.lighting?.Keyboard ?? {}
      art.classList.toggle('lit', !!k.on)
      const on = root.querySelector('[data-on]')!
      on.textContent = k.on ? t('light.on') : t('light.off')
      on.className = 'badge ' + (k.on ? 'active' : '')
      seg.querySelectorAll<HTMLButtonElement>('button').forEach(b => b.classList.toggle('on', Number(b.dataset.s) === k.timeout_s))
      root.querySelector('[data-rgb]')!.classList.toggle('hidden', !kb.rgb)
    },
  }
}
