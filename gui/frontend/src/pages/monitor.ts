import uPlot from 'uplot'
import 'uplot/dist/uPlot.min.css'
import { caps, el, fanIds, hist, tempUnit, useF } from '../core'
import { t } from '../i18n'
import type { Page } from '../widgets'

const CPU = '#ff6a0c', GPU = '#4aa8ff', BAT = '#3ecf8e', CPU2 = '#ffa000'

// Axis labels: seconds only when the ticks are less than a minute apart.
const fmtTicks = (ticks: number[]) => {
  const secs = ticks.length > 1 && ticks[1] - ticks[0] < 60
  return ticks.map(s => new Date(s * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', ...(secs ? { second: '2-digit' } : {}) }))
}

type Key = Exclude<keyof typeof hist, 'ts'>
// when: the backend exists; a series with data is shown anyway (read-only fans)
interface Series { key: Key, label: string, color: string, when?: () => boolean }
interface Chart { title: string, unit: () => string, series: Series[], conv?: (v: number) => number, max?: number }

const gpu = () => !!caps().gpu?.nvml
const visible = (c: Chart) => c.series.filter(s => !s.when || s.when() || hist[s.key].some(v => v != null))

export function monitor(): Page {
  const charts: Chart[] = [
    {
      title: t('monitor.temps'), unit: tempUnit, conv: v => (useF() ? v * 9 / 5 + 32 : v),
      series: [{ key: 'cpuTemp', label: 'CPU', color: CPU }, { key: 'gpuTemp', label: 'GPU', color: GPU, when: gpu }, { key: 'batTemp', label: t('monitor.batt'), color: BAT, when: () => !!caps().battery?.sensors }],
    },
    { title: t('monitor.usage'), unit: () => '%', max: 100, series: [{ key: 'cpuUse', label: 'CPU', color: CPU }, { key: 'gpuUse', label: 'GPU', color: GPU, when: gpu }] },
    { title: t('monitor.fans'), unit: () => 'RPM', series: [{ key: 'fanCpu', label: 'CPU', color: CPU2, when: () => fanIds().includes('cpu') }, { key: 'fanGpu', label: 'GPU', color: GPU, when: () => fanIds().includes('gpu') }] },
    {
      title: t('monitor.power'), unit: () => 'W',
      series: [{ key: 'cpuPow', label: 'CPU', color: CPU }, { key: 'gpuPow', label: 'GPU', color: GPU, when: gpu }, { key: 'batPow', label: t('monitor.batt'), color: BAT }],
    },
  ]
  const root = el(`<section>
    <div class="page-head"><div><h1>${t('monitor.title')}</h1><p>${t('monitor.sub')}</p></div>
      <div class="row"><span class="muted small">${t('monitor.window')}</span>
        <div class="seg" data-win><button data-w="120">2 min</button><button data-w="300" class="on">5 min</button><button data-w="600">10 min</button></div></div>
    </div>
    <div class="grid g2">${charts.map((c, i) => `<div class="card chart-card" data-card="${i}"><h2>${c.title}<span class="spacer"></span><span class="faint" data-unit="${i}"></span></h2><div class="chart-host" data-c="${i}"></div></div>`).join('')}</div>
  </section>`)

  let win = 300
  const plots: (uPlot | null)[] = charts.map(() => null)
  const shown: string[] = charts.map(() => '') // series keys each plot was built with

  root.querySelector('[data-win]')!.addEventListener('click', e => {
    const b = (e.target as HTMLElement).closest('button') as HTMLButtonElement | null
    if (!b) return
    root.querySelectorAll('[data-win] button').forEach(x => x.classList.toggle('on', x === b))
    win = Number(b.dataset.w)
    draw()
  })

  const axis = { stroke: '#9b94a2', grid: { stroke: '#2a252e', width: 1 }, ticks: { stroke: '#2a252e', width: 1 }, font: '11px system-ui, sans-serif' }

  function build(i: number, vis: Series[]) {
    const host = root.querySelector(`[data-c="${i}"]`) as HTMLElement
    const c = charts[i]
    const width = host.clientWidth
    if (width < 50) return
    host.innerHTML = ''
    shown[i] = vis.map(s => s.key).join()
    plots[i] = new uPlot({
      width, height: 200,
      cursor: { drag: { x: false, y: false }, points: { size: 6 } },
      legend: { show: true, live: false },
      scales: { x: { time: true }, y: c.max ? { range: [0, c.max] } : { range: (_u, _min, max) => [0, Math.max(10, Math.ceil((max * 1.15) / 10) * 10)] } },
      axes: [{ ...axis, space: 70, values: (_u, ticks) => fmtTicks(ticks) }, { ...axis, size: 48 }],
      series: [{}, ...vis.map(s => ({
        label: s.label, stroke: s.color, width: 2, spanGaps: false, points: { show: false },
        fill: s.color + '18', value: (_u: uPlot, v: number | null) => (v == null ? '–' : `${Math.round(v)} ${c.unit()}`),
      }))],
    }, data(i, vis), host)
  }

  function data(i: number, vis: Series[]): uPlot.AlignedData {
    const c = charts[i]
    const n = hist.ts.length
    const from = Math.max(0, n - win)
    const ts = hist.ts.slice(from)
    return [ts, ...vis.map(s => hist[s.key].slice(from).map(v => (v == null ? null : c.conv ? c.conv(v) : v)))] as uPlot.AlignedData
  }

  function draw() {
    charts.forEach((c, i) => {
      const vis = visible(c)
      root.querySelector(`[data-card="${i}"]`)!.classList.toggle('hidden', !vis.length)
      if (!vis.length) return
      root.querySelector(`[data-unit="${i}"]`)!.textContent = c.unit()
      if (plots[i] && shown[i] !== vis.map(s => s.key).join()) { plots[i]!.destroy(); plots[i] = null }
      if (!plots[i]) build(i, vis)
      else plots[i]!.setData(data(i, vis))
    })
  }

  new ResizeObserver(() => {
    plots.forEach((p, i) => {
      const host = root.querySelector(`[data-c="${i}"]`) as HTMLElement
      if (p && host.clientWidth > 50) p.setSize({ width: host.clientWidth, height: 200 })
    })
  }).observe(root)

  return {
    el: root,
    live: true,
    update() {},
    sample() { if (root.isConnected) draw() },
    shown() { requestAnimationFrame(draw) },
  }
}
