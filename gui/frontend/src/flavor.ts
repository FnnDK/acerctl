// Branding flavor, set by the Go side on <html> before this code runs
// (gui/flavor.go): nitro (red [N], Nitro/Predator) or acer (green [a], the
// other lines). Only names, marks and colours change; style.css switches the
// palette on :root[data-flavor].
const root = document.documentElement

export const flavor: 'nitro' | 'acer' = root.dataset.flavor === 'acer' ? 'acer' : 'nitro'
export const appName = root.dataset.appName || (flavor === 'acer' ? 'Acer Control Center' : 'Nitro Control Center')
/** First word of the name, shown big in the sidebar ("Nitro", "Acer"). */
export const brand = appName.split(' ')[0]

/** A palette colour as a hex string, for canvas drawing (uPlot). */
export const cssColor = (name: string, fallback: string) =>
  getComputedStyle(root).getPropertyValue(name).trim() || fallback
