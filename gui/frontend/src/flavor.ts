// Branding flavor, set by the Go side on <html> before this code runs
// (gui/flavor.go): nitro (red [N], Nitro), predator (blue-grey [P], Predator)
// or acer (green [a], the other lines). Only names, marks and colours change;
// style.css switches the palette on :root[data-flavor].
const root = document.documentElement

export type Flavor = 'nitro' | 'predator' | 'acer'
const names: Record<Flavor, string> = { nitro: 'Nitro Control Center', predator: 'Predator Control Center', acer: 'Acer Control Center' }

export const flavor: Flavor = (root.dataset.flavor ?? '') in names ? root.dataset.flavor as Flavor : 'nitro'
export const appName = root.dataset.appName || names[flavor]
/** First word of the name, shown big in the sidebar ("Nitro", "Predator", "Acer"). */
export const brand = appName.split(' ')[0]

/** A palette colour as a hex string, for canvas drawing (uPlot). */
export const cssColor = (name: string, fallback: string) =>
  getComputedStyle(root).getPropertyValue(name).trim() || fallback
