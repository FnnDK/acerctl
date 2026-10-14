import { flavor } from './flavor'

// Own line icons (24×24, stroke = currentColor).

const P: Record<string, string> = {
  home: '<rect x="3" y="3" width="7.5" height="9" rx="2"/><rect x="13.5" y="3" width="7.5" height="5.5" rx="2"/><rect x="13.5" y="11.5" width="7.5" height="9.5" rx="2"/><rect x="3" y="15" width="7.5" height="6" rx="2"/>',
  monitor: '<path d="M3 12h4l2.5-6 4.5 12 2.5-6H21"/>',
  fan: '<circle cx="12" cy="12" r="1.8"/><path d="M12 10.2C11 6 12.5 3 15 3c2.2 0 3 2.6.8 4.6l-2.7 2.6"/><path d="M13.8 12.6c4.1 1.3 6.2 3.9 5 6.1-1.1 1.9-3.8 1.3-4.6-1.6l-.9-3.6"/><path d="M10.6 13.3c-2.9 3.2-6.2 3.7-7.4 1.5-1.1-1.9.8-3.9 3.7-3.1l3.6 1"/>',
  light: '<rect x="2.5" y="7" width="19" height="11" rx="2.2"/><path d="M6 10.5h1M9.5 10.5h1M13 10.5h1M16.5 10.5h1M6 14.5h12"/><path d="M12 2.5v2M7 3.5l1 1.5M17 3.5l-1 1.5"/>',
  battery: '<rect x="2.5" y="7" width="17" height="10" rx="2.2"/><path d="M21.5 10.5v3"/><path d="M6 10v4M9 10v4M12 10v4"/>',
  scenarios: '<path d="M12 3 3 7.5l9 4.5 9-4.5z"/><path d="m3 12 9 4.5 9-4.5"/><path d="m3 16.5 9 4.5 9-4.5"/>',
  settings: '<path d="M4 7h9M17 7h3M4 17h3M11 17h9"/><circle cx="15" cy="7" r="2.2"/><circle cx="9" cy="17" r="2.2"/><path d="M4 12h3M11 12h9"/><circle cx="9" cy="12" r="0.01"/>',
  plug: '<path d="M8 3v5M16 3v5"/><path d="M6 8h12v3a6 6 0 0 1-12 0z"/><path d="M12 17v4"/>',
  bolt: '<path d="M13 2.5 5 13.5h6l-1 8 8-11h-6z"/>',
  leaf: '<path d="M5 19c0-8 5-14 15-14 0 10-6 15-14 15"/><path d="M5 19c3-4 6-6.5 9.5-8.5"/>',
  moon: '<path d="M20 14.5A8 8 0 1 1 9.5 4a6.5 6.5 0 0 0 10.5 10.5z"/>',
  balance: '<path d="M12 3v18M7 21h10M5 7h14"/><path d="m5 7-3 7a3 3 0 0 0 6 0zM19 7l-3 7a3 3 0 0 0 6 0z"/>',
  gauge: '<path d="M4.5 18a9 9 0 1 1 15 0"/><path d="m12 13 4.5-5"/><circle cx="12" cy="13" r="1.4"/>',
  flame: '<path d="M12 21c-4 0-7-2.8-7-6.6 0-3.4 2.4-5.4 3.8-8.4.7 2 1.7 3 2.9 3.6C12 6.7 13 4.2 15 3c0 3.6 4 5.6 4 11.4 0 3.8-3 6.6-7 6.6z"/>',
  cpu: '<rect x="6" y="6" width="12" height="12" rx="2"/><rect x="9.5" y="9.5" width="5" height="5" rx="1"/><path d="M9 2.5V6M15 2.5V6M9 18v3.5M15 18v3.5M2.5 9H6M2.5 15H6M18 9h3.5M18 15h3.5"/>',
  gpu: '<rect x="2.5" y="6" width="19" height="11" rx="2"/><circle cx="9" cy="11.5" r="3"/><path d="M15 9.5h3.5M15 13.5h3.5M5 17v3"/>',
  ram: '<rect x="2.5" y="7" width="19" height="9" rx="1.6"/><path d="M6 10v3M10 10v3M14 10v3M18 10v3M5 16v2.5M19 16v2.5"/>',
  net: '<path d="M7 20V5M3.5 8.5 7 5l3.5 3.5"/><path d="M17 4v15M13.5 15.5 17 19l3.5-3.5"/>',
  info: '<circle cx="12" cy="12" r="9"/><path d="M12 11v5.5M12 7.5v.01"/>',
  play: '<path d="M7 4.5v15l12-7.5z"/>',
  stop: '<rect x="6" y="6" width="12" height="12" rx="2"/>',
  edit: '<path d="M4 20h4L19 9l-4-4L4 16z"/><path d="m13.5 6.5 4 4"/>',
  trash: '<path d="M4 7h16M10 3.5h4M6 7l1 13h10l1-13"/><path d="M10 11v5.5M14 11v5.5"/>',
  plus: '<path d="M12 5v14M5 12h14"/>',
  save: '<path d="M5 3.5h11l3.5 3.5v13H5z"/><path d="M8 3.5v5h7v-5M8 20.5v-6h8v6"/>',
  refresh: '<path d="M20 11a8 8 0 0 0-14.5-4.5L3.5 9M4 13a8 8 0 0 0 14.5 4.5l2-2.5"/><path d="M3.5 4v5h5M20.5 20v-5h-5"/>',
  check: '<path d="m4.5 12.5 5 5 10-11"/>',
  thermo: '<path d="M10 14.5V5a2 2 0 0 1 4 0v9.5a4 4 0 1 1-4 0z"/><path d="M12 9v8"/>',
  heart: '<path d="M12 20s-7.5-4.6-7.5-10A4.3 4.3 0 0 1 12 7.4 4.3 4.3 0 0 1 19.5 10c0 5.4-7.5 10-7.5 10z"/>',
  apps: '<rect x="3.5" y="3.5" width="7" height="7" rx="1.6"/><rect x="13.5" y="3.5" width="7" height="7" rx="1.6"/><rect x="3.5" y="13.5" width="7" height="7" rx="1.6"/><rect x="13.5" y="13.5" width="7" height="7" rx="1.6"/>',
  usb: '<path d="M12 21V4M9 6.5 12 3.5l3 3"/><path d="M12 16.5 7 13.5V10M12 14l5-2.5V9"/><circle cx="7" cy="9" r="1.2"/><rect x="15.8" y="7" width="2.4" height="2.4"/><circle cx="12" cy="20" r="1.4"/>',
  globe: '<circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3c3 3.2 3 14.8 0 18M12 3c-3 3.2-3 14.8 0 18"/>',
}

export function icon(name: string, cls = ''): string {
  return `<svg class="${cls}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${P[name] ?? ''}</svg>`
}

// The flavor's mark as in its app icon (assets/logo/n-mark-cut.svg,
// a-mark-cut.svg): font brackets beside the N or the slanted a, the letter in
// currentColor, the brackets in the accent.
const markSvg = (letter: string) => `<svg class="mark" viewBox="190 -708 1420 860" aria-hidden="true"><g transform="scale(1 -1)">
<path fill="currentColor" d="${letter}"/>
<path style="fill:var(--accent)" d="M190,-152 L494,-152 L494,-62 L320,-62 L320,618 L494,618 L494,708 L190,708Z M1306,-152 L1610,-152 L1610,708 L1306,708 L1306,618 L1480,618 L1480,-62 L1306,-62Z"/>
</g></svg>`
export const mark = markSvg(flavor === 'acer'
  ? 'M772.9 -12C834.9 -12 899.3 16 955.3 52H959.3L961.2 0H1101.2L1155.3 278C1187.2 442 1108.8 512 966.8 512C882.8 512 800.1 488 715.9 446L754.2 334C819.6 362 864.3 376 906.3 376C962.3 376 993.2 360 992.2 324C761.9 312 662.6 254 640.8 142C624.9 60 668.9 -12 772.9 -12ZM854.6 120C818.6 120 803.1 133 807.6 156C814.2 190 849.6 218 972.4 222L960 158C929.1 138 894.6 120 854.6 120Z'
  : 'M 660,0 L 824,0 L 824,188 C 824,268 808,362 800,436 L 804,436 L 857,291 L 966,0 L 1140,0 L 1140,650 L 976,650 L 976,462 C 976,382 992,284 1000,214 L 996,214 L 943,360 L 834,650 L 660,650 Z')

export const profileIcon: Record<string, string> = {
  'low-power': 'leaf', quiet: 'moon', balanced: 'balance', 'balanced-performance': 'gauge', performance: 'flame',
}
