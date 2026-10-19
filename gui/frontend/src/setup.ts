// System setup dialog: what acerd reports in System.Setup (the acer_wmi option,
// the Secure Boot key, missing modules), applied with one password and one
// restart. Opens by itself for each pending item not shown yet since the app
// started, and from Settings; an open list follows System.Setup.

import { App, el, errorText, esc, state, toast, type Dict } from './core'
import { t } from './i18n'
import { icon } from './icons'

export const setupItems = (): Dict[] => state.system?.Setup ?? []
// what the user has to do now (not only waiting for a restart), as
// id.state.detail, to tell a new item from those already shown
export const setupPendingKeys = () => setupItems()
  .filter(it => it.state === 'needed' || it.state === 'manual')
  .map(it => `${it.id}.${it.state}.${it.detail ?? ''}`)
export const setupPending = () => setupPendingKeys().length > 0

let dialog: HTMLElement | null = null
// redraws the open dialog's list; null while it is closed, applying or
// showing the result of ApplySetup
let relist: (() => void) | null = null
let listed = '' // the items the list shows, as JSON

// updateSetup redraws an open list when System.Setup changed (acerd re-reads
// it every 30 s: DKMS may have built the modules, a key been enrolled).
export function updateSetup() {
  if (relist && JSON.stringify(setupItems()) !== listed) relist()
}

function itemText(it: Dict): string {
  const keys = [`setup.${it.id}.${it.state}.${it.detail}`, `setup.${it.id}.${it.state}`]
  for (const k of keys) if (t(k) !== k) return t(k, esc(it.detail ?? ''))
  return esc(`${it.state} ${it.detail ?? ''}`)
}

function itemsHtml(items: Dict[]): string {
  return items.map(it => {
    const ic = it.state === 'reboot' ? icon('check') : icon('info')
    return `<div class="setup-item ${it.state}">${ic}<div><b>${t('setup.' + it.id)}</b><p>${itemText(it)}</p></div></div>`
  }).join('')
}

export function openSetup() {
  if (dialog) return
  const b = el(`<div class="backdrop"><div class="dialog setup">
    <h2>${t('setup.title')}</h2>
    <div data-body></div>
    <div class="foot"><button class="btn" data-x></button><button class="btn hidden" data-alt></button><button class="btn primary hidden" data-go></button></div>
  </div></div>`)
  dialog = b
  const body = b.querySelector('[data-body]') as HTMLElement
  const x = b.querySelector('[data-x]') as HTMLButtonElement
  const alt = b.querySelector('[data-alt]') as HTMLButtonElement
  const go = b.querySelector('[data-go]') as HTMLButtonElement
  const close = () => { b.remove(); dialog = null; relist = null }
  x.onclick = close
  b.onclick = e => { if (e.target === b) close() }

  const reboot = async () => {
    try { await App.Reboot() } catch (e) { const { title, msg } = errorText(e); toast(title, msg, 'error') }
  }

  // after ApplySetup: the code for the firmware's screen, and the restart
  const done = (code: string) => {
    body.innerHTML = `<p class="muted">${t('setup.done')}</p>` + (code ? `
      <div class="setup-code"><span>${t('setup.code')}</span><b class="num">${esc(code)}</b></div>
      <ol class="setup-steps"><li>${t('setup.step1')}</li><li>${t('setup.step2')}</li><li>${t('setup.step3')}</li></ol>` : '')
    x.textContent = t('setup.later')
    alt.classList.add('hidden')
    go.textContent = t('setup.reboot_now')
    go.classList.remove('hidden')
    go.onclick = reboot
  }

  const apply = async (ids: string[]) => {
    go.disabled = alt.disabled = true
    relist = null
    try {
      done(await App.ApplySetup(ids))
    } catch (e) {
      const { title, msg } = errorText(e)
      toast(title, msg, 'error')
      relist = list
      list()
    } finally {
      go.disabled = alt.disabled = false
    }
  }

  const list = () => {
    const items = setupItems()
    listed = JSON.stringify(items)
    const needed = items.filter(it => it.state === 'needed').map(it => it.id)
    const key = items.find(it => it.id === 'signing_key')
    body.innerHTML = `<p class="muted">${items.length ? t('setup.sub') : t('setup.nothing')}</p>
      <div class="setup-items">${itemsHtml(items)}</div>
      ${needed.length ? `<div class="note mt">${icon('info')}<span>${needed.includes('signing_key') ? t('setup.note_key') : t('setup.note_reboot')}</span></div>` : ''}`
    x.textContent = needed.length ? t('setup.later') : t('setup.close')
    go.classList.add('hidden')
    alt.classList.add('hidden')
    if (needed.length) {
      go.textContent = t('setup.apply')
      go.classList.remove('hidden')
      go.onclick = () => apply(needed)
    } else if (items.some(it => it.state === 'reboot')) {
      go.textContent = t('setup.reboot_now')
      go.classList.remove('hidden')
      go.onclick = reboot
    }
    // the code is shown once; a lost code needs a new enrollment request
    if (key?.state === 'reboot' && !needed.length) {
      alt.textContent = t('setup.newcode')
      alt.classList.remove('hidden')
      alt.onclick = () => apply(['signing_key'])
    }
  }
  relist = list
  list()
  document.body.append(b)
}
