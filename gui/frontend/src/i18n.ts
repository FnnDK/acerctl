// Interface texts (own wording). Language: saved choice, else the system's.

const pt = {
  'app.tagline': 'Control Center',
  'nav.home': 'Painel', 'nav.monitor': 'Monitor', 'nav.fans': 'Ventoinhas', 'nav.lighting': 'Iluminação',
  'nav.battery': 'Bateria', 'nav.scenarios': 'Cenários', 'nav.settings': 'Configurações',

  'profile.low-power': 'Eco', 'profile.quiet': 'Silencioso', 'profile.balanced': 'Equilibrado',
  'profile.balanced-performance': 'Desempenho', 'profile.performance': 'Turbo',
  'profile.desc.low-power': 'Máxima autonomia', 'profile.desc.quiet': 'Ventoinhas baixas',
  'profile.desc.balanced': 'Uso geral', 'profile.desc.balanced-performance': 'Jogos e criação',
  'profile.desc.performance': 'Tudo no máximo',
  'why.battery_only': 'Só na bateria', 'why.ac_only': 'Só na tomada', 'why.charge': 'Requer carga ≥ 40 %',
  'extra': 'extra', 'extra.hint': 'Função além do app original',

  'power.ac': 'Na tomada', 'power.battery': 'Na bateria',
  'home.title': 'Painel', 'home.mode': 'Modo de operação',
  'home.cpu_temp': 'CPU', 'home.gpu_temp': 'GPU', 'home.cpu_fan': 'Ventoinha CPU', 'home.gpu_fan': 'Ventoinha GPU',
  'home.usage': 'Uso', 'home.freq': 'Frequência', 'home.power': 'Potência', 'home.clock': 'Clock', 'home.memclock': 'Memória',
  'home.gpu_sleep': 'Em repouso', 'home.gpu_sleep_hint': 'A dGPU está desligada para economizar energia; nada a medir.',
  'home.ram': 'Memória RAM', 'home.net': 'Rede', 'home.down': 'Recebendo', 'home.up': 'Enviando',
  'home.battery': 'Bateria', 'home.used': 'em uso', 'home.of': 'de',

  'monitor.title': 'Monitor', 'monitor.sub': 'Sensores ao vivo, atualizados a cada segundo',
  'monitor.temps': 'Temperaturas', 'monitor.usage': 'Uso', 'monitor.fans': 'Ventoinhas', 'monitor.power': 'Potência',
  'monitor.window': 'Janela', 'monitor.batt': 'Bateria',

  'fans.title': 'Ventoinhas', 'fans.sub': 'Controle de cada ventoinha pelo firmware',
  'fan.cpu': 'Ventoinha da CPU', 'fan.gpu': 'Ventoinha da GPU',
  'fan.auto': 'Automático', 'fan.max': 'Máximo', 'fan.manual': 'Manual',
  'fan.speed': 'Velocidade', 'fans.all': 'Todas as ventoinhas',
  'fans.nocurve': 'Este firmware não oferece curva de ventoinha: em Automático quem decide é o próprio firmware conforme o modo de operação.',
  'fans.floor': 'O firmware impõe uma rotação mínima: valores baixos podem não parar a ventoinha.',

  'light.title': 'Iluminação', 'light.sub': 'Teclado',
  'light.state': 'Luz do teclado', 'light.on': 'Ligada', 'light.off': 'Desligada',
  'light.state_hint': 'Liga e desliga pela tecla de iluminação do teclado (Fn).',
  'light.timeout': 'Desligar sozinha', 'light.timeout_hint': 'Apaga a luz após um tempo sem digitar',
  'light.always': 'Nunca', 'light.after': 'Após {0} s',
  'light.sub_rgb': 'Teclado e LEDs',
  'light.dev.keyboard': 'Teclado RGB', 'light.dev.lightbar': 'Barra de luz', 'light.dev.logo': 'Logo',
  'light.effect': 'Efeito', 'light.color': 'Cor', 'light.zone': 'Zona {0}', 'light.zones_hint': 'Uma cor para cada uma das {0} zonas',
  'light.speed': 'Velocidade', 'light.brightness': 'Brilho', 'light.direction': 'Direção',
  'light.dir.left': '← Esquerda', 'light.dir.right': 'Direita →',
  'light.noreadback': 'Este dispositivo não informa o próprio estado: aqui aparece o último efeito aplicado pelo acerctl.',
  'fx.static': 'Estático', 'fx.breathing': 'Respiração', 'fx.neon': 'Neon', 'fx.wave': 'Onda', 'fx.shifting': 'Deslizante',
  'fx.zoom': 'Zoom', 'fx.meteor': 'Meteoro', 'fx.twinkling': 'Cintilante', 'fx.snake': 'Serpente', 'fx.ripple': 'Ondulação',
  'fx.rainbow': 'Arco-íris', 'fx.rain': 'Chuva', 'fx.lightning': 'Relâmpago', 'fx.spot': 'Holofote', 'fx.stars': 'Estrelas',
  'fx.fireball': 'Bola de fogo', 'fx.snow': 'Neve', 'fx.heartbeat': 'Batimento',

  'batt.title': 'Bateria', 'batt.sub': 'Carga, saúde e cuidados',
  'batt.status.Charging': 'Carregando', 'batt.status.Discharging': 'Descarregando', 'batt.status.Full': 'Carregada',
  'batt.status.Not charging': 'Sem carregar', 'batt.status.Unknown': 'Desconhecido',
  'batt.health': 'Saúde', 'batt.cycles': 'Ciclos', 'batt.capacity': 'Capacidade', 'batt.design': 'de projeto',
  'batt.temp': 'Temperatura', 'batt.voltage': 'Tensão', 'batt.current': 'Corrente', 'batt.power': 'Potência',
  'batt.limit': 'Limitar carga a 80 %', 'batt.limit_hint': 'Prolonga a vida útil quando o notebook fica muito tempo na tomada. Gravado no firmware: vale mesmo desligado.',
  'batt.calib': 'Calibração da bateria',
  'batt.calib_hint': 'Descarrega e recarrega a bateria por completo para corrigir a leitura da carga. Leva horas; mantenha o carregador ligado e evite usar o notebook.',
  'batt.calib_start': 'Iniciar calibração', 'batt.calib_stop': 'Parar calibração', 'batt.calibrating': 'Calibrando…',
  'batt.calib_needs_ac': 'Conecte o carregador para calibrar.',
  'batt.calib_confirm': 'Iniciar a calibração? A bateria será descarregada e recarregada por completo.',
  'batt.care': 'Cuidados',

  'scn.title': 'Cenários', 'scn.sub': 'Combinações de modo, ventoinhas e teclado — aplicadas na hora ou ao abrir um app',
  'scn.appsync': 'App Sync', 'scn.appsync_hint': 'Aplica o cenário quando um dos apps dele é aberto e volta ao estado anterior quando ele fecha.',
  'scn.new': 'Novo cenário', 'scn.from_current': 'Salvar estado atual',
  'scn.apply': 'Aplicar', 'scn.edit': 'Editar', 'scn.delete': 'Excluir', 'scn.active': 'ativo',
  'scn.empty': 'Nenhum cenário salvo ainda.',
  'scn.ac': 'Tomada', 'scn.battery': 'Bateria', 'scn.kbd': 'Teclado', 'scn.apps': 'Apps',
  'scn.name': 'Nome', 'scn.mode_ac': 'Modo na tomada', 'scn.mode_battery': 'Modo na bateria',
  'scn.fans': 'Ventoinhas', 'scn.kbd_timeout': 'Luz do teclado desliga', 'scn.apps_hint': 'Um executável por linha (ex.: jogo.exe). Usado pelo App Sync.',
  'scn.keep': 'Não alterar', 'scn.save': 'Salvar', 'scn.cancel': 'Cancelar',
  'scn.delete_confirm': 'Excluir o cenário “{0}”?', 'scn.name_needed': 'Dê um nome ao cenário.',
  'scn.saved': 'Cenário salvo', 'scn.applied': 'Cenário aplicado',

  'set.title': 'Configurações', 'set.sub': 'Preferências do app e do notebook',
  'set.usb': 'Carregar USB com o notebook desligado', 'set.usb_hint': 'Mantém a porta USB alimentada até a bateria chegar ao nível escolhido',
  'set.usb_off': 'Desligado', 'set.usb_until': 'Até {0} %',
  'set.units': 'Temperatura', 'set.lang': 'Idioma', 'set.lang_auto': 'Do sistema',
  'set.key': 'Tecla Sense', 'set.key_hint': 'O que acontece ao apertar a tecla Sense do teclado',
  'set.key_app': 'Abrir o app', 'set.key_cycle': 'Alternar os modos',
  'set.tray': 'Ícone na bandeja', 'set.tray_hint': 'Status e atalhos no painel (no GNOME, precisa da extensão AppIndicator)',
  'set.autostart': 'Iniciar com a sessão', 'set.autostart_hint': 'Abre minimizado ao entrar no desktop',
  'set.app': 'App', 'set.machine': 'Notebook', 'set.about': 'Sobre',
  'set.model': 'Modelo', 'set.bios': 'BIOS', 'set.kernel': 'Kernel', 'set.daemon': 'Daemon', 'set.modules': 'Módulos',
  'set.reload': 'Detectar de novo', 'set.reload_hint': 'Reconsulta módulos e capacidades do firmware (após atualizar o kernel)',
  'set.disclaimer': 'Projeto independente e de código aberto. Sem vínculo com a Acer; “Acer”, “Aspire”, “Nitro” e “Predator” são marcas da Acer Inc., citadas só para identificar o hardware.',
  'set.absent': 'ausente', 'set.loaded': 'carregado',

  'conn.title': 'Sem conexão com o acerd', 'conn.retry': 'Tentando de novo…',
  'conn.hint': 'O serviço do sistema precisa estar rodando:',

  'err.NotSupported': 'Não suportado neste notebook', 'err.NotAllowed': 'Não permitido agora',
  'err.InvalidArgument': 'Valor inválido', 'err.Hardware': 'Erro do hardware', 'err.AccessDenied': 'Não autorizado',
  'err.Disconnected': 'Sem conexão com o acerd', 'err.Other': 'Erro',
  'ok': 'OK', 'cancel': 'Cancelar', 'done': 'Feito',
  'win.minimize': 'Minimizar', 'win.maximize': 'Maximizar', 'win.close': 'Fechar',
}

type Key = keyof typeof pt

const en: Record<Key, string> = {
  'app.tagline': 'Control Center',
  'nav.home': 'Dashboard', 'nav.monitor': 'Monitor', 'nav.fans': 'Fans', 'nav.lighting': 'Lighting',
  'nav.battery': 'Battery', 'nav.scenarios': 'Scenarios', 'nav.settings': 'Settings',

  'profile.low-power': 'Eco', 'profile.quiet': 'Quiet', 'profile.balanced': 'Balanced',
  'profile.balanced-performance': 'Performance', 'profile.performance': 'Turbo',
  'profile.desc.low-power': 'Longest battery life', 'profile.desc.quiet': 'Low fan noise',
  'profile.desc.balanced': 'Everyday use', 'profile.desc.balanced-performance': 'Gaming and creation',
  'profile.desc.performance': 'Everything maxed out',
  'why.battery_only': 'Battery only', 'why.ac_only': 'AC only', 'why.charge': 'Needs ≥ 40 % charge',
  'extra': 'extra', 'extra.hint': 'Beyond the original app',

  'power.ac': 'On AC', 'power.battery': 'On battery',
  'home.title': 'Dashboard', 'home.mode': 'Operating mode',
  'home.cpu_temp': 'CPU', 'home.gpu_temp': 'GPU', 'home.cpu_fan': 'CPU fan', 'home.gpu_fan': 'GPU fan',
  'home.usage': 'Usage', 'home.freq': 'Frequency', 'home.power': 'Power', 'home.clock': 'Clock', 'home.memclock': 'Memory',
  'home.gpu_sleep': 'Sleeping', 'home.gpu_sleep_hint': 'The dGPU is powered off to save energy; nothing to measure.',
  'home.ram': 'RAM', 'home.net': 'Network', 'home.down': 'Down', 'home.up': 'Up',
  'home.battery': 'Battery', 'home.used': 'used', 'home.of': 'of',

  'monitor.title': 'Monitor', 'monitor.sub': 'Live sensors, updated every second',
  'monitor.temps': 'Temperatures', 'monitor.usage': 'Usage', 'monitor.fans': 'Fans', 'monitor.power': 'Power',
  'monitor.window': 'Window', 'monitor.batt': 'Battery',

  'fans.title': 'Fans', 'fans.sub': 'Firmware control of each fan',
  'fan.cpu': 'CPU fan', 'fan.gpu': 'GPU fan',
  'fan.auto': 'Auto', 'fan.max': 'Max', 'fan.manual': 'Manual',
  'fan.speed': 'Speed', 'fans.all': 'All fans',
  'fans.nocurve': 'This firmware has no fan curve: in Auto the firmware itself decides, following the operating mode.',
  'fans.floor': 'The firmware enforces a minimum speed: low values may not stop the fan.',

  'light.title': 'Lighting', 'light.sub': 'Keyboard',
  'light.state': 'Keyboard light', 'light.on': 'On', 'light.off': 'Off',
  'light.state_hint': 'Toggled by the keyboard light key (Fn).',
  'light.timeout': 'Turn off by itself', 'light.timeout_hint': 'Turns the light off after a while without typing',
  'light.always': 'Never', 'light.after': 'After {0} s',
  'light.sub_rgb': 'Keyboard and LEDs',
  'light.dev.keyboard': 'RGB keyboard', 'light.dev.lightbar': 'Light bar', 'light.dev.logo': 'Logo',
  'light.effect': 'Effect', 'light.color': 'Colour', 'light.zone': 'Zone {0}', 'light.zones_hint': 'One colour for each of the {0} zones',
  'light.speed': 'Speed', 'light.brightness': 'Brightness', 'light.direction': 'Direction',
  'light.dir.left': '← Left', 'light.dir.right': 'Right →',
  'light.noreadback': 'This device cannot report its state: shown here is the last effect acerctl applied.',
  'fx.static': 'Static', 'fx.breathing': 'Breathing', 'fx.neon': 'Neon', 'fx.wave': 'Wave', 'fx.shifting': 'Shifting',
  'fx.zoom': 'Zoom', 'fx.meteor': 'Meteor', 'fx.twinkling': 'Twinkling', 'fx.snake': 'Snake', 'fx.ripple': 'Ripple',
  'fx.rainbow': 'Rainbow', 'fx.rain': 'Rain', 'fx.lightning': 'Lightning', 'fx.spot': 'Spotlight', 'fx.stars': 'Stars',
  'fx.fireball': 'Fireball', 'fx.snow': 'Snow', 'fx.heartbeat': 'Heartbeat',

  'batt.title': 'Battery', 'batt.sub': 'Charge, health and care',
  'batt.status.Charging': 'Charging', 'batt.status.Discharging': 'Discharging', 'batt.status.Full': 'Full',
  'batt.status.Not charging': 'Not charging', 'batt.status.Unknown': 'Unknown',
  'batt.health': 'Health', 'batt.cycles': 'Cycles', 'batt.capacity': 'Capacity', 'batt.design': 'design',
  'batt.temp': 'Temperature', 'batt.voltage': 'Voltage', 'batt.current': 'Current', 'batt.power': 'Power',
  'batt.limit': 'Limit charge to 80 %', 'batt.limit_hint': 'Extends battery life when the laptop stays plugged in. Stored in firmware: applies even when off.',
  'batt.calib': 'Battery calibration',
  'batt.calib_hint': 'Fully discharges and recharges the battery to fix the charge reading. Takes hours; keep the charger plugged in and avoid using the laptop.',
  'batt.calib_start': 'Start calibration', 'batt.calib_stop': 'Stop calibration', 'batt.calibrating': 'Calibrating…',
  'batt.calib_needs_ac': 'Plug in the charger to calibrate.',
  'batt.calib_confirm': 'Start calibration? The battery will be fully discharged and recharged.',
  'batt.care': 'Care',

  'scn.title': 'Scenarios', 'scn.sub': 'Mode, fan and keyboard presets — applied now or when an app starts',
  'scn.appsync': 'App Sync', 'scn.appsync_hint': 'Applies a scenario when one of its apps starts and restores the previous state when it exits.',
  'scn.new': 'New scenario', 'scn.from_current': 'Save current state',
  'scn.apply': 'Apply', 'scn.edit': 'Edit', 'scn.delete': 'Delete', 'scn.active': 'active',
  'scn.empty': 'No scenarios saved yet.',
  'scn.ac': 'AC', 'scn.battery': 'Battery', 'scn.kbd': 'Keyboard', 'scn.apps': 'Apps',
  'scn.name': 'Name', 'scn.mode_ac': 'Mode on AC', 'scn.mode_battery': 'Mode on battery',
  'scn.fans': 'Fans', 'scn.kbd_timeout': 'Keyboard light turns off', 'scn.apps_hint': 'One executable per line (e.g. game.exe). Used by App Sync.',
  'scn.keep': 'Leave as is', 'scn.save': 'Save', 'scn.cancel': 'Cancel',
  'scn.delete_confirm': 'Delete scenario “{0}”?', 'scn.name_needed': 'Give the scenario a name.',
  'scn.saved': 'Scenario saved', 'scn.applied': 'Scenario applied',

  'set.title': 'Settings', 'set.sub': 'App and laptop preferences',
  'set.usb': 'Charge USB while the laptop is off', 'set.usb_hint': 'Keeps the USB port powered until the battery reaches the chosen level',
  'set.usb_off': 'Off', 'set.usb_until': 'Down to {0} %',
  'set.units': 'Temperature', 'set.lang': 'Language', 'set.lang_auto': 'System',
  'set.key': 'Sense key', 'set.key_hint': 'What pressing the Sense key on the keyboard does',
  'set.key_app': 'Open the app', 'set.key_cycle': 'Cycle the modes',
  'set.tray': 'Tray icon', 'set.tray_hint': 'Status and shortcuts in the panel (GNOME needs the AppIndicator extension)',
  'set.autostart': 'Start with the session', 'set.autostart_hint': 'Opens minimized when you log in',
  'set.app': 'App', 'set.machine': 'Laptop', 'set.about': 'About',
  'set.model': 'Model', 'set.bios': 'BIOS', 'set.kernel': 'Kernel', 'set.daemon': 'Daemon', 'set.modules': 'Modules',
  'set.reload': 'Detect again', 'set.reload_hint': 'Re-probes kernel modules and firmware capabilities (after a kernel update)',
  'set.disclaimer': 'Independent open-source project. Not affiliated with Acer; “Acer”, “Aspire”, “Nitro” and “Predator” are trademarks of Acer Inc., mentioned only to identify the hardware.',
  'set.absent': 'absent', 'set.loaded': 'loaded',

  'conn.title': 'Cannot reach acerd', 'conn.retry': 'Retrying…',
  'conn.hint': 'The system service must be running:',

  'err.NotSupported': 'Not supported on this laptop', 'err.NotAllowed': 'Not allowed right now',
  'err.InvalidArgument': 'Invalid value', 'err.Hardware': 'Hardware error', 'err.AccessDenied': 'Not authorized',
  'err.Disconnected': 'Cannot reach acerd', 'err.Other': 'Error',
  'ok': 'OK', 'cancel': 'Cancel', 'done': 'Done',
  'win.minimize': 'Minimize', 'win.maximize': 'Maximize', 'win.close': 'Close',
}

export const langs = { pt, en } as const
export type Lang = keyof typeof langs

function pick(): Lang {
  let saved = ''
  try { saved = localStorage.getItem('lang') ?? '' } catch { /* no storage */ }
  if (saved in langs) return saved as Lang
  return navigator.language.toLowerCase().startsWith('pt') ? 'pt' : 'en'
}

let current: Lang = pick()

export function lang(): Lang { return current }

export function setLang(l: Lang | '') {
  try { l ? localStorage.setItem('lang', l) : localStorage.removeItem('lang') } catch { /* no storage */ }
  current = pick()
  document.documentElement.lang = current === 'pt' ? 'pt-BR' : 'en'
}

export function t(key: string, ...args: (string | number)[]): string {
  const dict = langs[current] as Record<string, string>
  let s = dict[key] ?? (pt as Record<string, string>)[key] ?? key
  args.forEach((a, i) => { s = s.replace(`{${i}}`, String(a)) })
  return s
}

export function savedLang(): string {
  try { return localStorage.getItem('lang') ?? '' } catch { return '' }
}
