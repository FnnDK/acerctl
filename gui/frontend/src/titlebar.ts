// Title bar of the frameless window: drag to move, double click to maximize,
// window buttons in the desktop's order with the icon theme's own symbolic
// icons (both come from titlebar.go); own icons only as a fallback.

import { EventsOn, WindowSetTitle } from '../wailsjs/runtime/runtime'
import * as App from '../wailsjs/go/main/App'
import { el, esc } from './core'
import { t } from './i18n'

type Bar = { left: string[], right: string[], icons: Record<string, string> }

const fallback: Record<string, string> = {
  'window-minimize': '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16"><rect x="4" y="10" width="8" height="2" rx="1"/></svg>',
  'window-maximize': '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16"><path d="M4 3h8a1 1 0 0 1 1 1v8a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1zm1 2v6h6V5z"/></svg>',
  'window-restore': '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16"><path d="M5 5h6v6H5z" fill="none" stroke="#000" stroke-width="2"/></svg>',
  'window-close': '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16"><path d="m4 4 8 8m0-8-8 8" stroke="#000" stroke-width="2" stroke-linecap="round"/></svg>',
}

let bar: Bar = { left: [], right: ['minimize', 'maximize', 'close'], icons: {} }
let maximized = false
const WIN_MARGIN = 18 // --win-margin in style.css, winMargin in main.go

// The icon is used as a mask so it takes the button's text color, whatever
// fills the theme's SVG uses.
function mask(name: string): string {
  const svg = bar.icons[name] ?? fallback[name]
  return `url("data:image/svg+xml;utf8,${encodeURIComponent(svg)}")`
}

function buttons(list: string[]): string {
  return list.map(b => {
    const icon = b === 'maximize' && maximized ? 'window-restore' : 'window-' + b
    return `<button class="tb-btn" data-win="${b}" title="${esc(t('win.' + b))}"><i style='-webkit-mask-image:${mask(icon)};mask-image:${mask(icon)}'></i></button>`
  }).join('')
}

export function titlebar(app: HTMLElement, brand: string) {
  const side = el(`<div class="tb tb-side"><div class="tb-btns" data-left></div>${brand}</div>`)
  const main = el(`<div class="tb tb-main"><div class="tb-btns" data-right></div></div>`)
  app.prepend(side, main)

  const render = () => {
    side.querySelector('[data-left]')!.innerHTML = buttons(bar.left)
    main.querySelector('[data-right]')!.innerHTML = buttons(bar.right)
  }
  const onClick = (e: MouseEvent) => {
    const b = (e.target as HTMLElement).closest('[data-win]') as HTMLElement | null
    if (!b) return
    if (b.dataset.win === 'minimize') App.Minimise()
    if (b.dataset.win === 'maximize') App.ToggleMaximise()
    if (b.dataset.win === 'close') App.Close()
  }
  const onDouble = (e: MouseEvent) => { if (!(e.target as HTMLElement).closest('button')) App.ToggleMaximise() }
  for (const n of [side, main]) { n.addEventListener('click', onClick); n.addEventListener('dblclick', onDouble) }

  const sync = async () => {
    try { maximized = await App.IsMaximised() } catch { maximized = false }
    document.documentElement.classList.toggle('maximized', maximized)
    // Resize handles over the whole shadow plus a few pixels of the window.
    const w = (window as any).wails
    if (w?.flags) w.flags.borderThickness = maximized ? 0 : WIN_MARGIN + 4
    render()
  }
  window.addEventListener('blur', () => document.documentElement.classList.add('unfocused'))
  window.addEventListener('focus', () => document.documentElement.classList.remove('unfocused'))
  let pending = 0
  window.addEventListener('resize', () => { clearTimeout(pending); pending = window.setTimeout(sync, 120) })

  App.Titlebar().then(b => { bar = b as Bar; render() }).catch(render)
  EventsOn('titlebar', (b: Bar) => { bar = b; render() })
  WindowSetTitle('Nitro Control Center')
  sync()
}
