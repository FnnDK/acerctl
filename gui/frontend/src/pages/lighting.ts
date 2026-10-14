import { App, act, caps, el, type Dict } from '../core'
import { t } from '../i18n'
import { paintRange, type Page } from '../widgets'

// Schematic keyboard: rows of key widths (in key units).
const ROWS = [
  Array(15).fill(0.93),
  [1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 2],
  [1.5, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1.5],
  [1.8, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 2.2],
  [2.3, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 2.7],
  [1.2, 1.2, 1.2, 1.2, 6, 1.2, 1.2, 1, 1, 1],
]

// Each key carries its horizontal position (0-1) in data-x: zones are
// vertical slices of the keyboard, and colourless effects paint a rainbow.
function keyboardSvg(): string {
  const U = 34, G = 4, W = 15 * U, rows: string[] = []
  ROWS.forEach((row, r) => {
    const total = row.reduce((a, b) => a + b, 0)
    const scale = (W - G * (row.length - 1)) / (total * U)
    let x = 14
    for (const w of row) {
      const kw = w * U * scale
      rows.push(`<rect class="key" data-x="${((x - 14 + kw / 2) / W).toFixed(3)}" x="${x.toFixed(1)}" y="${14 + r * (U + G) - (r === 0 ? 0 : 8)}" width="${kw.toFixed(1)}" height="${r === 0 ? U - 10 : U}" rx="5"/>`)
      x += kw + G
    }
  })
  return `<svg class="kbd-art" viewBox="0 0 ${W + 28} ${6 * (U + G) + 12}"><rect class="body" x="1" y="1" width="${W + 26}" height="${6 * (U + G) + 10}" rx="14"/><g class="keys">${rows.join('')}</g></svg>`
}

// A lighting device as System.Capabilities describes it.
interface Dev {
  effects: string[]
  with_color: string[]
  with_speed: string[]
  with_direction: string[]
  zones: number
  speed: [number, number]
  directions: string[]
  readback: boolean
}

const devName = (id: string) => (t('light.dev.' + id) === 'light.dev.' + id ? id : t('light.dev.' + id))

// What the daemon shows before a device was ever set (hw.LightDevice.Default).
function fallback(d: Dev): Dict {
  return { effect: d.effects[0], brightness: 50, colors: ['#ffffff'], speed: Math.round((d.speed[0] + d.speed[1]) / 2) }
}

// colourCount is how many colours an effect takes on this device.
function colourCount(d: Dev, effect: string): number {
  if (!d.with_color.includes(effect)) return 0
  return effect === 'static' && d.zones > 0 ? d.zones : 1
}

// fxStyle sets the preview animation of an element from a device state:
// data-fx picks the keyframes, --fx-dur follows the speed.
function fxStyle(node: HTMLElement | SVGElement, st: Dict | undefined, d: Dev | undefined) {
  node.dataset.fx = st?.effect ?? ''
  if (st?.speed && d) {
    const [lo, hi] = d.speed
    const f = hi > lo ? (st.speed - lo) / (hi - lo) : 0.5
    node.style.setProperty('--fx-dur', `${(4 - 3.2 * f).toFixed(2)}s`)
  }
}

function paintKeyboard(art: SVGElement, st: Dict | undefined, d: Dev) {
  const colors: string[] = st?.colors ?? []
  const b = (st?.brightness ?? 0) / 100
  art.classList.add('rgb')
  fxStyle(art, st, d)
  art.querySelectorAll<SVGRectElement>('.key').forEach(k => {
    const x = Number(k.dataset.x)
    let c = ''
    if (st && b > 0) {
      c = colors.length === 0 ? `hsl(${Math.round(x * 300)} 90% 60%)`
        : colors[Math.min(colors.length - 1, Math.floor(x * colors.length))]
    }
    k.style.fill = c ? `color-mix(in srgb, ${c} ${Math.round(12 + 60 * b)}%, #26212b)` : ''
    k.style.stroke = c
  })
}

function paintSwatch(sw: HTMLElement, st: Dict | undefined, d: Dev) {
  const colors: string[] = st?.colors ?? []
  const b = (st?.brightness ?? 0) / 100
  sw.style.background = !st || b === 0 ? '' : colors.length === 0
    ? 'linear-gradient(90deg, hsl(0 90% 60%), hsl(120 90% 60%), hsl(240 90% 60%), hsl(300 90% 60%))'
    : colors.length > 1 ? `linear-gradient(90deg, ${colors.join(', ')})` : colors[0]
  sw.style.opacity = st ? String(0.3 + 0.7 * b) : ''
  fxStyle(sw, st, d)
}

function slider(key: string, lo: number, hi: number): string {
  return `<div class="slider-row light-slider"><input type="range" data-${key} min="${lo}" max="${hi}" step="1"><output class="num"></output></div>`
}

function deviceCard(id: string, d: Dev): string {
  return `<div class="card mt" data-dev="${id}">
    <h2>${devName(id)}<span class="spacer"></span><span class="swatch ${id === 'logo' ? 'dot' : 'bar'}" data-swatch></span></h2>
    <div class="setting"><div class="label"><b>${t('light.effect')}</b></div>
      <select data-effect>${d.effects.map(e => `<option value="${e}">${t('fx.' + e)}</option>`).join('')}</select></div>
    <div class="setting" data-row="color"><div class="label"><b>${t('light.color')}</b><span data-colorhint></span></div><div class="colors" data-colors></div></div>
    <div class="setting" data-row="speed"><div class="label"><b>${t('light.speed')}</b></div>${slider('speed', d.speed[0], d.speed[1])}</div>
    <div class="setting" data-row="direction"><div class="label"><b>${t('light.direction')}</b></div>
      <div class="seg" data-dirs>${d.directions.map(v => `<button data-d="${v}">${t('light.dir.' + v)}</button>`).join('')}</div></div>
    <div class="setting"><div class="label"><b>${t('light.brightness')}</b></div>${slider('bright', 0, 100)}</div>
    ${d.readback ? '' : `<p class="muted small" data-noreadback>${t('light.noreadback')}</p>`}
  </div>`
}

export function lighting(): Page {
  const root = el(`<section>
    <div class="page-head"><div><h1>${t('light.title')}</h1><p data-sub></p></div></div>
    <div class="card" data-art>${keyboardSvg()}</div>
    <div class="card mt" data-single>
      <div class="setting"><div class="label"><b>${t('light.state')}</b><span>${t('light.state_hint')}</span></div><span class="badge" data-on></span></div>
      <div class="setting"><div class="label"><b>${t('light.timeout')}</b><span>${t('light.timeout_hint')}</span></div><div class="seg" data-timeouts></div></div>
    </div>
    <div data-devs></div>
  </section>`)
  const art = root.querySelector('.kbd-art') as SVGElement
  const seg = root.querySelector('[data-timeouts]') as HTMLElement
  const devsBox = root.querySelector('[data-devs]') as HTMLElement
  let builtTimeouts = '', builtDevs = ''
  let devs: Record<string, Dev> = {}
  const busy = new Set<HTMLElement>() // sliders being dragged
  const zoneMem: Record<string, string[]> = {} // last per-zone colours, kept across effect changes
  seg.onclick = e => {
    const b = (e.target as HTMLElement).closest('button') as HTMLButtonElement | null
    if (b && !b.classList.contains('on')) act(App.SetKeyboardTimeout(Number(b.dataset.s)))
  }

  const send = (id: string, fields: Dict) => act(App.SetLight(id, fields))

  function buildDevices() {
    devsBox.innerHTML = Object.entries(devs).map(([id, d]) => deviceCard(id, d)).join('')
    for (const card of devsBox.querySelectorAll<HTMLElement>('[data-dev]')) {
      const id = card.dataset.dev!, d = devs[id]
      card.querySelector('[data-effect]')!.addEventListener('change', e => {
        const effect = (e.target as HTMLSelectElement).value
        const fields: Dict = { effect }
        if (colourCount(d, effect) > 1 && zoneMem[id]?.length === d.zones) fields.colors = zoneMem[id]
        send(id, fields)
      })
      card.querySelector('[data-colors]')!.addEventListener('change', () => {
        send(id, { colors: [...card.querySelectorAll<HTMLInputElement>('[data-colors] input')].map(i => i.value) })
      })
      card.querySelector('[data-dirs]')!.addEventListener('click', e => {
        const b = (e.target as HTMLElement).closest('button') as HTMLButtonElement | null
        if (b && !b.classList.contains('on')) send(id, { direction: b.dataset.d })
      })
      for (const [key, field] of [['speed', 'speed'], ['bright', 'brightness']]) {
        const range = card.querySelector(`[data-${key}]`) as HTMLInputElement
        const out = range.nextElementSibling as HTMLOutputElement
        range.addEventListener('pointerdown', () => busy.add(range))
        range.addEventListener('input', () => { out.textContent = range.value + (key === 'bright' ? ' %' : ''); paintRange(range) })
        range.addEventListener('change', () => { busy.delete(range); send(id, { [field]: Number(range.value) }) })
      }
    }
  }

  function updateDevice(card: HTMLElement, d: Dev, st: Dict | undefined) {
    const id = card.dataset.dev!
    const s = st ?? fallback(d)
    if (s.effect === 'static' && d.zones > 0 && s.colors?.length === d.zones) zoneMem[id] = s.colors
    const sel = card.querySelector('[data-effect]') as HTMLSelectElement
    if (document.activeElement !== sel) sel.value = s.effect

    const n = colourCount(d, s.effect)
    card.querySelector('[data-row=color]')!.classList.toggle('hidden', n === 0)
    const box = card.querySelector('[data-colors]') as HTMLElement
    if (box.children.length !== n) {
      box.innerHTML = Array.from({ length: n }, (_, i) =>
        `<label class="color"><input type="color">${n > 1 ? `<span>${t('light.zone', i + 1)}</span>` : ''}</label>`).join('')
    }
    card.querySelector('[data-colorhint]')!.textContent = n > 1 ? t('light.zones_hint', n) : ''
    box.querySelectorAll<HTMLInputElement>('input').forEach((inp, i) => {
      if (document.activeElement !== inp) inp.value = s.colors?.[i] ?? s.colors?.[0] ?? '#ffffff'
    })

    const animated = d.with_speed.includes(s.effect)
    card.querySelector('[data-row=speed]')!.classList.toggle('hidden', !animated)
    card.querySelector('[data-row=direction]')!.classList.toggle('hidden', !d.with_direction.includes(s.effect))
    card.querySelectorAll<HTMLButtonElement>('[data-dirs] button').forEach(b => b.classList.toggle('on', b.dataset.d === s.direction))
    for (const [key, v, unit] of [['speed', s.speed ?? d.speed[0], ''], ['bright', s.brightness ?? 0, ' %']] as const) {
      const range = card.querySelector(`[data-${key}]`) as HTMLInputElement
      if (busy.has(range)) continue
      range.value = String(v)
      paintRange(range)
      ;(range.nextElementSibling as HTMLOutputElement).textContent = `${v}${unit}`
    }
    paintSwatch(card.querySelector('[data-swatch]') as HTMLElement, st, d)
  }

  return {
    el: root,
    live: true, // the daemon refreshes the on/off state while someone watches the Monitor
    update(s) {
      const kb = caps().kbd_backlight
      const lc: Record<string, Dev> = caps().lighting ?? {}
      root.querySelector('[data-sub]')!.textContent = Object.keys(lc).some(id => id !== 'keyboard') ? t('light.sub_rgb') : t('light.sub')
      root.querySelector('[data-art]')!.classList.toggle('hidden', !kb && !lc.keyboard)
      root.querySelector('[data-single]')!.classList.toggle('hidden', !kb)

      const tos: number[] = kb?.timeouts ?? []
      const key = JSON.stringify(tos)
      if (key !== builtTimeouts) {
        builtTimeouts = key
        seg.innerHTML = tos.map(v => `<button data-s="${v}">${v === 0 ? t('light.always') : t('light.after', v)}</button>`).join('')
      }
      const k = s.lighting?.Keyboard ?? {}
      const on = root.querySelector('[data-on]')!
      on.textContent = k.on ? t('light.on') : t('light.off')
      on.className = 'badge ' + (k.on ? 'active' : '')
      seg.querySelectorAll<HTMLButtonElement>('button').forEach(b => b.classList.toggle('on', Number(b.dataset.s) === k.timeout_s))

      const dkey = JSON.stringify(lc)
      if (dkey !== builtDevs) { builtDevs = dkey; devs = lc; buildDevices() }
      const states: Dict = s.lighting?.Devices ?? {}
      for (const card of devsBox.querySelectorAll<HTMLElement>('[data-dev]')) {
        updateDevice(card, devs[card.dataset.dev!], states[card.dataset.dev!])
      }
      if (lc.keyboard) paintKeyboard(art, states.keyboard, lc.keyboard)
      else art.classList.toggle('lit', !!k.on)
    },
  }
}
