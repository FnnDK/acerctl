import { App, act, caps, confirmDialog, el, fmtTemp, num, sample, setText, state, statusName, switchHtml, type Dict } from '../core'
import { t } from '../i18n'
import { icon } from '../icons'
import type { Page } from '../widgets'

export function battery(): Page {
  const root = el(`<section>
    <div class="page-head"><div><h1>${t('batt.title')}</h1><p>${t('batt.sub')}</p></div></div>
    <div class="grid" style="grid-template-columns: minmax(0,1.1fr) minmax(0,1fr)">
      <div class="card">
        <div class="batt-big">
          <div class="batt-shape"><i data-fill style="width:0"></i></div>
          <div class="stack" style="gap:4px">
            <div class="batt-pct"><span data-pct>–</span><small>%</small></div>
            <div class="row" style="gap:8px"><span class="muted" data-status>–</span><span class="badge" data-src></span></div>
          </div>
        </div>
        <div class="grid g4 mt">
          <div class="stat"><span class="k">${t('batt.voltage')}</span><span class="v" data-volt>–</span></div>
          <div class="stat"><span class="k">${t('batt.current')}</span><span class="v" data-curr>–</span></div>
          <div class="stat"><span class="k">${t('batt.power')}</span><span class="v" data-pow>–</span></div>
          <div class="stat"><span class="k">${t('batt.temp')}</span><span class="v" data-temp>–</span></div>
        </div>
      </div>
      <div class="card">
        <div class="tile-head">${icon('heart')}<b>${t('batt.health')}</b><span class="badge" data-health-badge></span></div>
        <div class="meter"><i data-health-bar style="width:0"></i></div>
        <div class="grid g2 mt">
          <div class="stat"><span class="k">${t('batt.capacity')}</span><span class="v" data-cap>–</span><span class="small faint" data-design></span></div>
          <div class="stat"><span class="k">${t('batt.cycles')}</span><span class="v" data-cycles>–</span></div>
        </div>
      </div>
    </div>
    <div class="card mt" data-care>
      <h2>${t('batt.care')}</h2>
      <div class="setting" data-limit-row><div class="label"><b>${t('batt.limit')}</b><span>${t('batt.limit_hint')}</span></div>${switchHtml('data-limit', false)}</div>
      <div class="setting" data-calib-row><div class="label"><b>${t('batt.calib')}</b><span>${t('batt.calib_hint')}</span><span class="small" style="display:block;color:var(--warn)" data-calib-ac></span></div>
        <button class="btn" data-calib></button></div>
    </div>
  </section>`)

  const limit = root.querySelector('[data-limit]') as HTMLInputElement
  limit.onchange = async () => {
    const want = limit.checked
    if (!(await act(App.SetChargeLimit(want)))) limit.checked = !want
  }
  const calib = root.querySelector('[data-calib]') as HTMLButtonElement
  calib.onclick = async () => {
    if (state.battery?.Calibrating) act(App.StopCalibration())
    else if (await confirmDialog(t('batt.calib_confirm'), t('batt.calib_start'))) act(App.StartCalibration())
  }

  function live(b: Dict) {
    if (b.voltage != null) setText(root, '[data-volt]', `${(b.voltage / 1000).toFixed(2)} V`)
    if (b.current != null) setText(root, '[data-curr]', `${b.current} mA`)
    if (b.voltage != null && b.current != null) setText(root, '[data-pow]', `${(Math.abs(b.voltage * b.current) / 1e6).toFixed(1)} W`)
    if (b.temp != null) setText(root, '[data-temp]', fmtTemp(b.temp))
    if (b.status) setText(root, '[data-status]', statusName(b.status))
  }

  return {
    el: root,
    live: true,
    update(s) {
      const m = s.mode ?? {}, bat = s.battery ?? {}, h: Dict = bat.Health ?? {}
      const pct = sample?.battery?.percent ?? m.BatteryPercent
      setText(root, '[data-pct]', num(pct))
      const fill = root.querySelector('[data-fill]') as HTMLElement
      fill.style.width = `${pct ?? 0}%`
      const status = sample?.battery?.status ?? h.status
      fill.className = status === 'Charging' ? 'charging' : pct != null && pct < 20 ? 'low' : ''
      const src = root.querySelector('[data-src]')!
      src.innerHTML = `${m.PowerSource === 'ac' ? t('power.ac') : t('power.battery')}`
      src.className = 'badge ' + (m.PowerSource === 'ac' ? 'ok' : '')
      live({ ...h, ...(sample?.battery ?? {}) })

      setText(root, '[data-health-badge]', h.health_pct != null ? `${h.health_pct} %` : '–')
      ;(root.querySelector('[data-health-bar]') as HTMLElement).style.width = `${h.health_pct ?? 0}%`
      setText(root, '[data-cap]', h.full ? `${(h.full / 1e6).toFixed(1)} Wh` : '–')
      setText(root, '[data-design]', h.design ? `${t('home.of')} ${(h.design / 1e6).toFixed(1)} Wh ${t('batt.design')}` : '')
      setText(root, '[data-cycles]', num(h.cycles))

      const bc = caps().battery ?? {}
      root.querySelector('[data-limit-row]')!.classList.toggle('hidden', !bc.charge_limit)
      root.querySelector('[data-calib-row]')!.classList.toggle('hidden', !bc.calibration)
      // nothing for the card to hold when the firmware offers neither
      root.querySelector('[data-care]')!.classList.toggle('hidden', !bc.charge_limit && !bc.calibration)
      limit.checked = !!bat.ChargeLimit
      calib.innerHTML = bat.Calibrating ? `${icon('stop')}${t('batt.calib_stop')}` : `${icon('play')}${t('batt.calib_start')}`
      calib.classList.toggle('danger', !!bat.Calibrating)
      const noAC = m.PowerSource !== 'ac' && !bat.Calibrating
      calib.disabled = noAC
      setText(root, '[data-calib-ac]', bat.Calibrating ? t('batt.calibrating') : noAC ? t('batt.calib_needs_ac') : '')
    },
    sample(x) { if (x.battery) { live(x.battery); setText(root, '[data-pct]', num(x.battery.percent)) } },
  }
}
