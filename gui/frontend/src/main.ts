import './style.css'
import { App, el, esc, onSample, onState, profileName, start, state, type State } from './core'
import { lang, setLang, t } from './i18n'
import { icon, mark } from './icons'
import type { Page } from './widgets'
import { home } from './pages/home'
import { monitor } from './pages/monitor'
import { fans } from './pages/fans'
import { lighting } from './pages/lighting'
import { battery } from './pages/battery'
import { scenarios } from './pages/scenarios'
import { settings } from './pages/settings'

setLang((() => { try { return (localStorage.getItem('lang') ?? '') as never } catch { return '' as never } })())

const NAV: { id: string, icon: string, make: () => Page, show?: () => boolean }[] = [
  { id: 'home', icon: 'home', make: home },
  { id: 'monitor', icon: 'monitor', make: monitor },
  { id: 'fans', icon: 'fan', make: fans, show: () => !!state.system?.Capabilities?.fans },
  { id: 'lighting', icon: 'light', make: lighting, show: () => !!state.system?.Capabilities?.kbd_backlight },
  { id: 'battery', icon: 'battery', make: battery },
  { id: 'scenarios', icon: 'scenarios', make: scenarios },
  { id: 'settings', icon: 'settings', make: settings },
]

const app = document.getElementById('app')!
app.append(el(`<nav class="sidebar">
  <div class="brand">${mark}<div><b>Nitro</b><span>${t('app.tagline')}</span></div></div>
  ${NAV.map(n => `<button class="nav-item" data-nav="${n.id}">${icon(n.icon)}<span>${t('nav.' + n.id)}</span></button>`).join('')}
  <div class="sidebar-foot">
    <div class="pill" data-mode-pill></div>
    <div class="pill" data-power-pill></div>
  </div>
</nav>`))
const main = el('<main></main>')
app.append(main)

const pages = new Map<string, Page>()
NAV.forEach(n => pages.set(n.id, n.make()))
let current = ''

function go(id: string) {
  if (!pages.has(id)) id = 'home'
  current = id
  try { localStorage.setItem('page', id) } catch { /* no storage */ }
  document.querySelectorAll<HTMLElement>('[data-nav]').forEach(b => b.classList.toggle('active', b.dataset.nav === id))
  main.replaceChildren(pages.get(id)!.el)
  main.scrollTop = 0
  pages.get(id)!.update(state)
  pages.get(id)!.shown?.()
  monitorPolicy()
}

document.querySelector('.sidebar')!.addEventListener('click', e => {
  const b = (e.target as HTMLElement).closest('[data-nav]') as HTMLElement | null
  if (b) go(b.dataset.nav!)
})

// Samples only while a live page is on screen: without subscribers the
// daemon does not sample at all.
let monWant = -1
function monitorPolicy() {
  const want = state.connected && !document.hidden && pages.get(current)?.live ? 1000 : 0
  if (want !== monWant) { monWant = want; App.Monitor(want) }
}
document.addEventListener('visibilitychange', monitorPolicy)

// ---- connection overlay ----
const overlay = el(`<div class="overlay hidden"><div class="box">
  <svg class="mark" viewBox="0 0 900 990">${mark.replace(/^<svg[^>]*>|<\/svg>$/g, '')}</svg>
  <h2 style="margin:6px 0 0">${t('conn.title')}</h2>
  <p class="muted" style="margin:0">${t('conn.hint')} <code>systemctl status nitrod</code></p>
  <p class="small faint" data-err style="margin:0"></p>
  <p class="small muted" style="margin:0">${t('conn.retry')}</p>
</div></div>`)
document.body.append(overlay)

function chrome(s: State) {
  overlay.classList.toggle('hidden', s.connected)
  overlay.querySelector('[data-err]')!.textContent = s.error ?? ''
  for (const n of NAV) {
    const b = document.querySelector(`[data-nav="${n.id}"]`) as HTMLElement
    b.classList.toggle('hidden', !!n.show && !n.show())
  }
  const m = s.mode ?? {}
  const mp = document.querySelector('[data-mode-pill]') as HTMLElement
  mp.innerHTML = m.Current ? `${icon('gauge')}<b>${esc(profileName(m.Current))}</b>${s.scenarios?.Active ? `<span>· ${esc(s.scenarios.Active)}</span>` : ''}` : ''
  mp.classList.toggle('hidden', !m.Current)
  const pp = document.querySelector('[data-power-pill]') as HTMLElement
  pp.innerHTML = m.PowerSource ? `${icon(m.PowerSource === 'ac' ? 'plug' : 'battery')}<b class="num">${m.BatteryPercent ?? '–'} %</b><span>${m.PowerSource === 'ac' ? t('power.ac') : t('power.battery')}</span>` : ''
  pp.classList.toggle('hidden', !m.PowerSource)
}

onState(s => {
  chrome(s)
  if (current) {
    const nav = NAV.find(n => n.id === current)
    if (nav?.show && !nav.show()) go('home')
    else pages.get(current)!.update(s)
  }
  monitorPolicy()
})
onSample(x => { pages.get(current)?.sample?.(x) })

document.documentElement.lang = lang() === 'pt' ? 'pt-BR' : 'en'
start()
go((() => { try { return localStorage.getItem('page') ?? 'home' } catch { return 'home' } })())
