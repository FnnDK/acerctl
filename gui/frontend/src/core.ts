// Shared state, daemon calls, formatting and small DOM helpers.

import { EventsOn } from '../wailsjs/runtime/runtime'
import * as App from '../wailsjs/go/main/App'
import { t } from './i18n'

export type Dict = Record<string, any>

export interface State {
  connected: boolean
  error?: string
  system?: Dict
  mode?: Dict
  fans?: Dict
  lighting?: Dict
  battery?: Dict
  scenarios?: Dict
}

export let state: State = { connected: false }
export let sample: Dict | null = null

type Fn<T> = (v: T) => void
const stateFns: Fn<State>[] = []
const sampleFns: Fn<Dict>[] = []
export const onState = (f: Fn<State>) => stateFns.push(f)
export const onSample = (f: Fn<Dict>) => sampleFns.push(f)

// ---- sample history (Monitor page) ----
export const HISTORY = 600
export const hist = {
  ts: [] as number[],
  cpuTemp: [] as (number | null)[], gpuTemp: [] as (number | null)[], batTemp: [] as (number | null)[],
  cpuUse: [] as (number | null)[], gpuUse: [] as (number | null)[],
  fanCpu: [] as (number | null)[], fanGpu: [] as (number | null)[],
  cpuPow: [] as (number | null)[], gpuPow: [] as (number | null)[], batPow: [] as (number | null)[],
}
type Series = Exclude<keyof typeof hist, 'ts'>

function record(s: Dict) {
  const now = Date.now() / 1000
  const last = hist.ts[hist.ts.length - 1]
  const push = (vals: Partial<Record<Series, number | null>>, at: number) => {
    hist.ts.push(at)
    for (const k of Object.keys(hist) as (keyof typeof hist)[]) {
      if (k !== 'ts') (hist[k] as (number | null)[]).push(vals[k as Series] ?? null)
    }
  }
  // A gap (window hidden, other page) becomes a break in the lines.
  if (last && now - last > 4) push({}, last + 1)
  const gpuOn = s.gpu?.active !== false
  const bat = s.battery
  push({
    cpuTemp: s.cpu?.temp ?? null,
    gpuTemp: gpuOn ? s.gpu?.temp ?? null : null,
    batTemp: bat?.temp ?? null,
    cpuUse: s.cpu?.usage ?? null,
    gpuUse: gpuOn ? s.gpu?.usage ?? null : null,
    fanCpu: s.fans?.cpu ?? null,
    fanGpu: s.fans?.gpu ?? null,
    cpuPow: s.cpu?.power ?? null,
    gpuPow: gpuOn ? s.gpu?.power ?? null : null,
    batPow: bat?.voltage != null && bat?.current != null ? Math.abs(bat.voltage * bat.current) / 1e6 : null,
  }, now)
  const extra = hist.ts.length - HISTORY
  if (extra > 0) for (const k of Object.keys(hist) as (keyof typeof hist)[]) hist[k].splice(0, extra)
}

export function start() {
  EventsOn('state', (s: State) => { state = s; stateFns.forEach(f => f(s)) })
  EventsOn('sample', (s: Dict) => { sample = s; record(s); sampleFns.forEach(f => f(s)) })
  App.State().then(s => { state = s as State; stateFns.forEach(f => f(state)) })
}

// ---- capabilities ----
export const caps = (): Dict => state.system?.Capabilities ?? {}
export const profiles = (): string[] => caps().profiles ?? []
export const fanIds = (): string[] => caps().fans?.ids ?? []

// ---- calls ----
export function errorText(e: unknown): { title: string, msg: string } {
  // Wails rejects with the Go error text (a string; an Error in the browser dev server).
  let kind = 'Other', msg = e instanceof Error ? e.message : String(e)
  try { const j = JSON.parse(msg); kind = j.kind; msg = j.msg } catch { /* plain text */ }
  return { title: t('err.' + kind) === 'err.' + kind ? kind : t('err.' + kind), msg }
}

export async function act(p: Promise<unknown>, okMsg?: string): Promise<boolean> {
  try {
    await p
    if (okMsg) toast(okMsg, '', 'ok')
    return true
  } catch (e) {
    const { title, msg } = errorText(e)
    toast(title, msg, 'error')
    return false
  }
}

export { App }

// ---- formatting ----
export function useF(): boolean {
  try { return localStorage.getItem('units') === 'F' } catch { return false }
}
export function tempNum(c: number | null | undefined): string {
  if (c == null || !isFinite(c)) return '–'
  return String(Math.round(useF() ? c * 9 / 5 + 32 : c))
}
export const tempUnit = () => (useF() ? '°F' : '°C')
export const fmtTemp = (c: number | null | undefined) => (c == null ? '–' : `${tempNum(c)} ${tempUnit()}`)

export function fmtRate(bps: number | undefined): string {
  if (bps == null) return '–'
  const u = ['B/s', 'KB/s', 'MB/s', 'GB/s']
  let i = 0, v = bps
  while (v >= 1000 && i < u.length - 1) { v /= 1000; i++ }
  return `${v < 10 && i > 0 ? v.toFixed(1) : Math.round(v)} ${u[i]}`
}
export const gib = (b: number | undefined) => (b == null ? '–' : (b / 2 ** 30).toFixed(1))
export const num = (v: number | undefined | null, d = 0) => (v == null || !isFinite(v) ? '–' : v.toFixed(d))

// ---- DOM ----
export function el<T extends HTMLElement = HTMLElement>(html: string): T {
  const tpl = document.createElement('template')
  tpl.innerHTML = html.trim()
  return tpl.content.firstElementChild as T
}
export const $ = <T extends HTMLElement = HTMLElement>(root: ParentNode, sel: string) => root.querySelector(sel) as T
export const $$ = <T extends HTMLElement = HTMLElement>(root: ParentNode, sel: string) => Array.from(root.querySelectorAll(sel)) as T[]
export function setText(root: ParentNode, sel: string, text: string) {
  const n = root.querySelector(sel)
  if (n && n.textContent !== text) n.textContent = text
}
export const esc = (s: string) => s.replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]!))

let toastHost: HTMLElement | null = null
export function toast(title: string, msg = '', kind: 'error' | 'ok' | 'info' = 'info') {
  if (!toastHost) { toastHost = el('<div class="toasts"></div>'); document.body.append(toastHost) }
  const n = el(`<div class="toast ${kind}"><b>${esc(title)}</b>${msg ? `<span>${esc(msg)}</span>` : ''}</div>`)
  toastHost.append(n)
  setTimeout(() => n.remove(), kind === 'error' ? 6500 : 3000)
}

export function confirmDialog(text: string, okLabel = t('ok')): Promise<boolean> {
  return new Promise(res => {
    const b = el(`<div class="backdrop"><div class="dialog" style="width:min(440px,92vw)">
      <p style="margin:0 0 6px;font-size:15px">${esc(text)}</p>
      <div class="foot"><button class="btn" data-x>${t('cancel')}</button><button class="btn primary" data-ok>${esc(okLabel)}</button></div>
    </div></div>`)
    const close = (v: boolean) => { b.remove(); res(v) }
    $(b, '[data-x]').onclick = () => close(false)
    $(b, '[data-ok]').onclick = () => close(true)
    b.onclick = e => { if (e.target === b) close(false) }
    document.body.append(b)
  })
}

export function switchHtml(attr: string, checked: boolean, disabled = false): string {
  return `<label class="switch"><input type="checkbox" ${attr} ${checked ? 'checked' : ''} ${disabled ? 'disabled' : ''}><span></span></label>`
}

// Why a profile is not allowed now: only a hint; the daemon has the rules
// and its refusal message is what the user sees on click.
export function blockedReason(p: string): string {
  const m = state.mode ?? {}
  if (p === 'low-power' && m.PowerSource === 'ac') return t('why.battery_only')
  if ((p === 'quiet' || p === 'performance') && m.PowerSource === 'battery') return t('why.ac_only')
  return t('why.charge')
}

export function profileName(p: string) { return t('profile.' + p) }

export function statusName(s: string) {
  const k = 'batt.status.' + s
  return t(k) === k ? s : t(k)
}
