import { main } from '../../wailsjs/go/models'
import { App, act, caps, confirmDialog, el, esc, fanIds, profileName, profiles, state, switchHtml, toast, type Dict } from '../core'
import { t } from '../i18n'
import { icon } from '../icons'
import type { Page } from '../widgets'

// Profiles that can ever apply on each source (the daemon has the full rules).
const acProfiles = () => profiles().filter(p => p !== 'low-power')
const batProfiles = () => profiles().filter(p => p !== 'quiet' && p !== 'performance')

function fanText(f: Dict): string {
  return f.mode === 'manual' ? `${f.speed ?? 0} %` : t('fan.' + f.mode)
}
function timeoutText(s: number): string { return s === 0 ? t('light.always') : t('light.after', s) }

function card(sc: Dict, active: boolean): string {
  const chips: string[] = []
  if (sc.mode_ac) chips.push(`<span class="chip">${icon('plug').replace('<svg', '<svg style="width:12px;height:12px;vertical-align:-2px"')} ${t('scn.ac')}: <b>${profileName(sc.mode_ac)}</b></span>`)
  if (sc.mode_battery) chips.push(`<span class="chip">${t('scn.battery')}: <b>${profileName(sc.mode_battery)}</b></span>`)
  for (const [id, f] of Object.entries<Dict>(sc.fans ?? {})) chips.push(`<span class="chip">${id.toUpperCase()}: <b>${fanText(f)}</b></span>`)
  if (sc.kbd_timeout_s != null) chips.push(`<span class="chip">${t('scn.kbd')}: <b>${timeoutText(sc.kbd_timeout_s)}</b></span>`)
  const apps: string[] = sc.apps ?? []
  return `<div class="card scn ${active ? 'on' : ''}" data-name="${esc(sc.name)}">
    <div class="row between"><h3>${esc(sc.name)}</h3>${active ? `<span class="badge active">${t('scn.active')}</span>` : ''}</div>
    <div class="chips">${chips.join('')}</div>
    ${apps.length ? `<div class="small muted">${icon('apps').replace('<svg', '<svg style="width:13px;height:13px;vertical-align:-2px"')} ${apps.map(esc).join(', ')}</div>` : ''}
    <div class="actions">
      <button class="btn sm primary" data-act="apply" ${active ? 'disabled' : ''}>${icon('check')}${t('scn.apply')}</button>
      <button class="btn sm" data-act="edit">${icon('edit')}${t('scn.edit')}</button>
      <button class="btn sm ghost danger" data-act="delete" style="margin-left:auto">${icon('trash')}</button>
    </div>
  </div>`
}

function opts(list: string[], cur: string | undefined, label: (v: string) => string): string {
  return `<option value="">${t('scn.keep')}</option>` + list.map(v => `<option value="${v}" ${v === cur ? 'selected' : ''}>${label(v)}</option>`).join('')
}

function editor(sc: Dict | null, title: string) {
  const fans: Dict = sc?.fans ?? {}
  const tos: number[] = caps().kbd_backlight?.timeouts ?? []
  const fanModes: string[] = caps().fans?.modes ?? []
  const b = el(`<div class="backdrop"><div class="dialog">
    <h2>${esc(title)}</h2>
    <label class="field"><span>${t('scn.name')}</span><input type="text" data-f="name" value="${esc(sc?.name ?? '')}" maxlength="60"></label>
    <div class="grid g2">
      <label class="field"><span>${t('scn.mode_ac')}</span><select data-f="mode_ac">${opts(acProfiles(), sc?.mode_ac, profileName)}</select></label>
      <label class="field"><span>${t('scn.mode_battery')}</span><select data-f="mode_battery">${opts(batProfiles(), sc?.mode_battery, profileName)}</select></label>
    </div>
    ${fanIds().length ? `<div class="field"><span>${t('scn.fans')}</span>${fanIds().map(id => `
      <div class="fan-edit" data-fan="${id}"><span>${t('fan.' + id)}</span>
        <select>${opts(fanModes, fans[id]?.mode, m => t('fan.' + m))}</select>
        <input type="number" min="0" max="100" value="${fans[id]?.speed ?? 50}" title="%">
      </div>`).join('')}</div>` : ''}
    ${tos.length ? `<label class="field"><span>${t('scn.kbd_timeout')}</span><select data-f="kbd">${opts(tos.map(String), sc?.kbd_timeout_s != null ? String(sc.kbd_timeout_s) : undefined, v => timeoutText(Number(v)))}</select></label>` : ''}
    <label class="field"><span>${t('scn.apps')} — <span class="faint">${t('scn.apps_hint')}</span></span><textarea data-f="apps" spellcheck="false">${esc((sc?.apps ?? []).join('\n'))}</textarea></label>
    <div class="foot"><button class="btn" data-x>${t('scn.cancel')}</button><button class="btn primary" data-ok>${icon('save')}${t('scn.save')}</button></div>
  </div></div>`)
  const q = <T extends HTMLElement>(s: string) => b.querySelector(s) as T
  const syncSpeed = () => b.querySelectorAll<HTMLElement>('[data-fan]').forEach(r => {
    (r.querySelector('input') as HTMLInputElement).style.visibility = (r.querySelector('select') as HTMLSelectElement).value === 'manual' ? 'visible' : 'hidden'
  })
  b.addEventListener('change', syncSpeed)
  syncSpeed()
  const close = () => b.remove()
  q('[data-x]').onclick = close
  b.onclick = e => { if (e.target === b) close() }
  q('[data-ok]').onclick = async () => {
    const name = q<HTMLInputElement>('[data-f=name]').value.trim()
    if (!name) { toast(t('scn.name_needed'), '', 'error'); return }
    const out = new main.Scenario({ name })
    out.mode_ac = q<HTMLSelectElement>('[data-f=mode_ac]').value || undefined
    out.mode_battery = q<HTMLSelectElement>('[data-f=mode_battery]').value || undefined
    const f: Record<string, main.FanSetting> = {}
    b.querySelectorAll<HTMLElement>('[data-fan]').forEach(r => {
      const mode = (r.querySelector('select') as HTMLSelectElement).value
      if (mode) f[r.dataset.fan!] = new main.FanSetting({ mode, speed: Number((r.querySelector('input') as HTMLInputElement).value) })
    })
    if (Object.keys(f).length) out.fans = f
    const kbd = b.querySelector('[data-f=kbd]') as HTMLSelectElement | null
    if (kbd?.value) out.kbd_timeout_s = Number(kbd.value)
    const apps = q<HTMLTextAreaElement>('[data-f=apps]').value.split('\n').map(s => s.trim()).filter(Boolean)
    if (apps.length) out.apps = apps
    if (!(await act(App.SaveScenario(out), t('scn.saved')))) return
    if (sc?.name && sc.name !== name) await act(App.DeleteScenario(sc.name))
    close()
  }
  document.body.append(b)
  q<HTMLInputElement>('[data-f=name]').focus()
}

// The current state as a scenario draft (mode for the current source).
function fromCurrent(): Dict {
  const m = state.mode ?? {}
  const sc: Dict = { name: '' }
  sc[m.PowerSource === 'battery' ? 'mode_battery' : 'mode_ac'] = m.Current
  const fans: Dict = {}
  for (const [id, f] of Object.entries<Dict>(state.fans?.State ?? {})) fans[id] = { mode: f.mode, speed: f.speed }
  sc.fans = fans
  const k = state.lighting?.Keyboard
  if (k?.timeout_s != null) sc.kbd_timeout_s = k.timeout_s
  return sc
}

export function scenarios(): Page {
  const root = el(`<section>
    <div class="page-head"><div><h1>${t('scn.title')}</h1><p>${t('scn.sub')}</p></div>
      <div class="row"><button class="btn" data-current>${icon('save')}${t('scn.from_current')}</button><button class="btn primary solid" data-new>${icon('plus')}${t('scn.new')}</button></div>
    </div>
    <div class="card"><div class="setting"><div class="label"><b>${t('scn.appsync')}</b><span>${t('scn.appsync_hint')}</span></div>${switchHtml('data-appsync', false)}</div></div>
    <div class="scn-grid mt" data-list></div>
  </section>`)
  const list = root.querySelector('[data-list]') as HTMLElement
  const sync = root.querySelector('[data-appsync]') as HTMLInputElement
  let key = ''

  sync.onchange = async () => { const want = sync.checked; if (!(await act(App.SetAppSync(want)))) sync.checked = !want }
  ;(root.querySelector('[data-new]') as HTMLElement).onclick = () => editor(null, t('scn.new'))
  ;(root.querySelector('[data-current]') as HTMLElement).onclick = () => editor(fromCurrent(), t('scn.from_current'))
  list.onclick = async e => {
    const b = (e.target as HTMLElement).closest('[data-act]') as HTMLButtonElement | null
    if (!b) return
    const name = (b.closest('[data-name]') as HTMLElement).dataset.name!
    const sc = (state.scenarios?.List ?? []).find((x: Dict) => x.name === name)
    if (b.dataset.act === 'apply') act(App.ApplyScenario(name), t('scn.applied'))
    if (b.dataset.act === 'edit') editor(sc, t('scn.edit'))
    if (b.dataset.act === 'delete' && (await confirmDialog(t('scn.delete_confirm', name), t('scn.delete')))) act(App.DeleteScenario(name))
  }

  return {
    el: root,
    live: false,
    update(s) {
      const sc = s.scenarios ?? {}
      sync.checked = !!sc.AppSync
      const k = JSON.stringify([sc.List, sc.Active, document.documentElement.lang])
      if (k === key) return
      key = k
      const items: Dict[] = sc.List ?? []
      list.innerHTML = items.length
        ? items.map(x => card(x, x.name === sc.Active)).join('')
        : `<div class="card empty" style="grid-column:1/-1">${icon('scenarios')}<p>${t('scn.empty')}</p></div>`
    },
  }
}
