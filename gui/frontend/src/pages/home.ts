import { App, act, blockedReason, el, esc, fmtRate, gib, num, profileName, profiles, setText, state, tempNum, tempUnit, type Dict } from '../core'
import { t } from '../i18n'
import { icon, profileIcon } from '../icons'
import { gaugeHtml, setGauge, type Page } from '../widgets'

// RPM scale of the fan gauges: grows with the highest reading seen.
let rpmMax = 5500

export function modesHtml(): string {
  const m = state.mode ?? {}
  const allowed: string[] = m.Allowed ?? []
  const extras: string[] = m.Extras ?? []
  return profiles().map(p => {
    const ok = allowed.includes(p)
    const on = m.Current === p
    const extra = extras.includes(p)
    return `<button class="mode ${on ? 'on' : ''}" data-mode="${p}" ${ok ? '' : 'disabled'} title="${extra ? esc(t('extra.hint')) : ''}">
      ${extra ? `<span class="badge extra">${t('extra')}</span>` : ''}
      ${icon(profileIcon[p] ?? 'gauge')}
      <b>${profileName(p)}</b>
      <small>${ok ? t('profile.desc.' + p) : blockedReason(p)}</small>
    </button>`
  }).join('')
}

export function bindModes(host: HTMLElement) {
  host.onclick = e => {
    const b = (e.target as HTMLElement).closest('[data-mode]') as HTMLButtonElement | null
    if (b && !b.disabled && !b.classList.contains('on')) act(App.SetMode(b.dataset.mode!))
  }
}

export function home(): Page {
  const root = el(`<section>
    <div class="page-head"><div><h1>${t('home.title')}</h1><p data-model></p></div></div>
    <div class="card"><h2>${t('home.mode')}</h2><div class="modes" data-modes></div></div>
    <div class="card mt"><div class="grid g4">
      ${gaugeHtml('cpuT', t('home.cpu_temp'))}${gaugeHtml('gpuT', t('home.gpu_temp'))}
      ${gaugeHtml('cpuF', t('home.cpu_fan'))}${gaugeHtml('gpuF', t('home.gpu_fan'))}
    </div></div>
    <div class="grid tiles mt">
      <div class="card"><div class="tile-head">${icon('cpu')}<b>CPU</b></div>
        <div class="stats">
          <div class="stat"><span class="k">${t('home.usage')}</span><span class="v" data-cpu-use>–</span></div>
          <div class="stat"><span class="k">${t('home.freq')}</span><span class="v" data-cpu-freq>–</span></div>
          <div class="stat"><span class="k">${t('home.power')}</span><span class="v" data-cpu-pow>–</span></div>
        </div></div>
      <div class="card"><div class="tile-head">${icon('gpu')}<b>GPU</b><span class="badge hidden" data-gpu-sleep>${t('home.gpu_sleep')}</span></div>
        <div class="stats" data-gpu-stats>
          <div class="stat"><span class="k">${t('home.usage')}</span><span class="v" data-gpu-use>–</span></div>
          <div class="stat"><span class="k">${t('home.clock')}</span><span class="v" data-gpu-clk>–</span></div>
          <div class="stat"><span class="k">${t('home.power')}</span><span class="v" data-gpu-pow>–</span></div>
        </div>
        <p class="small muted hidden" data-gpu-hint style="margin:0">${t('home.gpu_sleep_hint')}</p></div>
      <div class="card"><div class="tile-head">${icon('ram')}<b>${t('home.ram')}</b></div>
        <div class="stat"><span class="v" data-ram>–</span></div>
        <div class="meter" style="margin-top:10px"><i data-ram-bar style="width:0"></i></div></div>
      <div class="card"><div class="tile-head">${icon('net')}<b>${t('home.net')}</b></div>
        <div class="stats">
          <div class="stat"><span class="k">${t('home.down')}</span><span class="v" data-rx>–</span></div>
          <div class="stat"><span class="k">${t('home.up')}</span><span class="v" data-tx>–</span></div>
        </div></div>
    </div>
  </section>`)
  const modes = root.querySelector('[data-modes]') as HTMLElement
  bindModes(modes)
  let modesKey = ''

  const v = (n: string, unit: string) => (n === '–' ? n : `${n}<small>${unit}</small>`)
  const setHtml = (sel: string, html: string) => { const n = root.querySelector(sel)!; if (n.innerHTML !== html) n.innerHTML = html }

  return {
    el: root,
    live: true,
    update(s) {
      const md = s.system?.Model ?? {}
      setText(root, '[data-model]', [md.vendor, md.product].filter(Boolean).join(' '))
      const key = JSON.stringify([s.mode, profiles(), document.documentElement.lang])
      if (key !== modesKey) { modesKey = key; modes.innerHTML = modesHtml() }
    },
    sample(x: Dict) {
      const cpu = x.cpu ?? {}, gpu = x.gpu ?? {}, fans = x.fans ?? {}
      const gpuOn = gpu.active !== false
      setGauge(root, 'cpuT', (cpu.temp ?? 0) / 100, tempNum(cpu.temp), tempUnit(), '', cpu.temp >= 90)
      setGauge(root, 'gpuT', gpuOn ? (gpu.temp ?? 0) / 100 : 0, gpuOn ? tempNum(gpu.temp) : '–', gpuOn ? tempUnit() : t('home.gpu_sleep'), '', gpu.temp >= 87)
      rpmMax = Math.max(rpmMax, fans.cpu ?? 0, fans.gpu ?? 0)
      setGauge(root, 'cpuF', (fans.cpu ?? 0) / rpmMax, num(fans.cpu), 'RPM')
      setGauge(root, 'gpuF', (fans.gpu ?? 0) / rpmMax, num(fans.gpu), 'RPM')

      setHtml('[data-cpu-use]', v(num(cpu.usage), '%'))
      setHtml('[data-cpu-freq]', v(cpu.freq ? (cpu.freq / 1000).toFixed(2) : '–', 'GHz'))
      setHtml('[data-cpu-pow]', v(num(cpu.power, 1), 'W'))
      root.querySelector('[data-gpu-sleep]')!.classList.toggle('hidden', gpuOn)
      root.querySelector('[data-gpu-stats]')!.classList.toggle('hidden', !gpuOn)
      root.querySelector('[data-gpu-hint]')!.classList.toggle('hidden', gpuOn)
      setHtml('[data-gpu-use]', v(num(gpu.usage), '%'))
      setHtml('[data-gpu-clk]', v(num(gpu.clock), 'MHz'))
      setHtml('[data-gpu-pow]', v(num(gpu.power, 1), 'W'))

      const ram = x.ram ?? {}
      setHtml('[data-ram]', `${gib(ram.used)}<small>/ ${gib(ram.total)} GiB</small>`)
      ;(root.querySelector('[data-ram-bar]') as HTMLElement).style.width = ram.total ? `${(100 * ram.used) / ram.total}%` : '0'
      setText(root, '[data-rx]', fmtRate(x.net?.rx))
      setText(root, '[data-tx]', fmtRate(x.net?.tx))
    },
  }
}
