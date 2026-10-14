import { App, act, caps, el, esc, setText, switchHtml, useF } from '../core'
import { savedLang, setLang, t, type Lang } from '../i18n'
// App icon, same file the installer puts in the icon theme.
import iconNitro from '../../../../assets/logo/n-icon-cut.svg'
import iconAcer from '../../../../assets/logo/a-icon-cut.svg'
import { appName, flavor } from '../flavor'

const appIcon = flavor === 'acer' ? iconAcer : iconNitro
import type { Page } from '../widgets'

export function settings(): Page {
  const root = el(`<section>
    <div class="page-head"><div><h1>${t('set.title')}</h1><p>${t('set.sub')}</p></div></div>
    <div class="card" data-machine>
      <h2>${t('set.machine')}</h2>
      <div class="setting" data-usb-row><div class="label"><b>${t('set.usb')}</b><span>${t('set.usb_hint')}</span></div><div class="seg" data-usb></div></div>
      <div class="setting" data-key-row><div class="label"><b>${t('set.key')}</b><span>${t('set.key_hint')}</span></div>
        <div class="seg" data-key><button data-k="app">${t('set.key_app')}</button><button data-k="cycle">${t('set.key_cycle')}</button></div></div>
      <div class="setting"><div class="label"><b>${t('set.reload')}</b><span>${t('set.reload_hint')}</span></div><button class="btn" data-reload>${t('set.reload')}</button></div>
    </div>
    <div class="card mt">
      <h2>${t('set.app')}</h2>
      <div class="setting"><div class="label"><b>${t('set.units')}</b></div><div class="seg" data-units><button data-u="C">°C</button><button data-u="F">°F</button></div></div>
      <div class="setting"><div class="label"><b>${t('set.lang')}</b></div><div class="seg" data-lang><button data-l="">${t('set.lang_auto')}</button><button data-l="pt">Português</button><button data-l="en">English</button></div></div>
      <div class="setting"><div class="label"><b>${t('set.tray')}</b><span>${t('set.tray_hint')}</span></div>${switchHtml('data-tray', false)}</div>
      <div class="setting"><div class="label"><b>${t('set.autostart')}</b><span>${t('set.autostart_hint')}</span></div>${switchHtml('data-autostart', false)}</div>
    </div>
    <div class="card mt">
      <h2>${t('set.about')}</h2>
      <div class="row" style="gap:16px;align-items:flex-start">
        <img src="${appIcon}" alt="" style="width:56px;height:56px;flex:none">
        <div class="stack" style="gap:6px">
          <div><b style="font-size:16px">${appName}</b> <span class="faint small" data-version></span></div>
          <table class="small" style="border-spacing:14px 3px;margin-left:-14px"><tbody data-about></tbody></table>
          <p class="small faint" style="margin:6px 0 0;max-width:640px">${t('set.disclaimer')}</p>
        </div>
      </div>
    </div>
  </section>`)

  const usb = root.querySelector('[data-usb]') as HTMLElement
  let usbKey = ''
  usb.onclick = e => {
    const b = (e.target as HTMLElement).closest('button') as HTMLButtonElement | null
    if (b && !b.classList.contains('on')) act(App.SetUsbCharging(Number(b.dataset.p)))
  }
  ;(root.querySelector('[data-reload]') as HTMLElement).onclick = () => act(App.Reload(), t('done'))

  const units = root.querySelector('[data-units]') as HTMLElement
  const markUnits = () => units.querySelectorAll<HTMLElement>('button').forEach(b => b.classList.toggle('on', (b.dataset.u === 'F') === useF()))
  units.onclick = e => {
    const b = (e.target as HTMLElement).closest('button') as HTMLButtonElement | null
    if (!b) return
    try { localStorage.setItem('units', b.dataset.u!) } catch { /* no storage */ }
    markUnits()
  }
  markUnits()

  const langSeg = root.querySelector('[data-lang]') as HTMLElement
  langSeg.querySelectorAll<HTMLElement>('button').forEach(b => b.classList.toggle('on', b.dataset.l === savedLang()))
  langSeg.onclick = e => {
    const b = (e.target as HTMLElement).closest('button') as HTMLButtonElement | null
    if (!b || b.dataset.l === savedLang()) return
    const l = b.dataset.l as Lang | ''
    // settings.json carries it to the tray; localStorage is the startup cache
    App.SetLang(l).catch(() => {}).finally(() => { setLang(l); location.reload() })
  }

  const key = root.querySelector('[data-key]') as HTMLElement
  key.onclick = e => {
    const b = (e.target as HTMLElement).closest('button') as HTMLButtonElement | null
    if (b && !b.classList.contains('on')) act(App.SetSenseKey(b.dataset.k!))
  }
  const tray = root.querySelector('[data-tray]') as HTMLInputElement
  App.TrayShown().then(v => { tray.checked = v })
  tray.onchange = async () => { const want = tray.checked; if (!(await act(App.SetTrayShown(want)))) tray.checked = !want }

  const auto = root.querySelector('[data-autostart]') as HTMLInputElement
  App.Autostart().then(v => { auto.checked = v })
  auto.onchange = async () => { const want = auto.checked; if (!(await act(App.SetAutostart(want)))) auto.checked = !want }

  return {
    el: root,
    live: false,
    update(s) {
      const vals: number[] = caps().usb_charging ?? []
      root.querySelector('[data-usb-row]')!.classList.toggle('hidden', !vals.length)
      const k = JSON.stringify(vals)
      if (k !== usbKey) {
        usbKey = k
        usb.innerHTML = vals.map(v => `<button data-p="${v}">${v === 0 ? t('set.usb_off') : t('set.usb_until', v)}</button>`).join('')
      }
      usb.querySelectorAll<HTMLElement>('button').forEach(b => b.classList.toggle('on', Number(b.dataset.p) === s.battery?.UsbCharging))

      root.querySelector('[data-key-row]')!.classList.toggle('hidden', !caps().sense_key)
      // without thermal profiles the key can only open the app
      key.querySelector('[data-k="cycle"]')!.classList.toggle('hidden', !caps().profiles?.length)
      key.querySelectorAll<HTMLElement>('button').forEach(b => b.classList.toggle('on', b.dataset.k === s.system?.SenseKey))

      const sys = s.system ?? {}, md = sys.Model ?? {}
      setText(root, '[data-version]', sys.Version ? `acerd ${sys.Version}` : '')
      // acerd reports "absent", "loaded" or the module's version (loaded too)
      const modState = (v: string) => v === 'absent'
        ? `<span class="badge">${t('set.absent')}</span>`
        : `<span class="badge ok">${t('set.loaded')}</span>${v === 'loaded' ? '' : `<span class="faint num">${esc(v)}</span>`}`
      const mods = `<div class="mods">${Object.entries<string>(md.modules ?? {}).map(([n, v]) => `<span>${esc(n)} ${modState(v)}</span>`).join('')}</div>`
      const rows: [string, string][] = [
        [t('set.model'), esc([md.vendor, md.product].filter(Boolean).join(' '))],
        [t('set.bios'), esc(md.bios ?? '')],
        [t('set.kernel'), esc(md.kernel ?? '')],
        [t('set.modules'), mods],
      ]
      const html = rows.map(([k, v]) => `<tr><td class="muted">${k}</td><td>${v || '–'}</td></tr>`).join('')
      const tb = root.querySelector('[data-about]')!
      if (tb.innerHTML !== html) tb.innerHTML = html
    },
  }
}
