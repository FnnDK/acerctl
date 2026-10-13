# nitroctl — STATUS (ponto de retomada)

Última atualização: 2026-10-13. Leia este arquivo primeiro; a referência técnica está em `docs/firmware-wmi.md` (citada abaixo como §N).

## Objetivo e decisão de arquitetura

Reproduzir no Linux as funções do Acer NitroSense/PredatorSense **coexistindo com o `acer_wmi` in-tree**, sem substituí-lo: o que faltar entra por (a) patches upstream no `acer_wmi` e (b) módulos companheiros fora da árvore + um daemon + uma GUI. Sem flag de marca: as capacidades vêm do firmware (máscaras WMI, DMI); tabela por modelo só para o que o firmware não informa. Linuwu-Sense e Div-Acer-Manager-Max são referência, não base (§10).

**Pilha decidida (2026-10-12):** módulos de kernel em **C** (`drivers/`), daemon em **Go** (serviço D-Bus no system bus, polkit, systemd), GUI 1:1 em **Go + Wails** (frontend web em WebView/WebKitGTK), CLI e tray como clientes da mesma API D-Bus. Arte própria na GUI (ícones, fontes, animações): nada de material da Acer.

**Nomes (2026-10-13), no esquema do asusctl:** projeto e CLI `nitroctl`, daemon `nitrod`, GUI `nitro-control-center`; prefixo reverse-DNS `io.github.fnndk` (D-Bus `io.github.fnndk.Nitrod1` em `/io/github/fnndk/Nitrod1`, polkit `io.github.fnndk.nitrod.{control,settings}`, estado em `/var/lib/nitrod`). "Nitro" só como referência ao hardware; README deve dizer que não há vínculo com a Acer.

Máquina de referência: Acer **Nitro ANV15-52** (i7-13620H, RTX 4060, BIOS V1.31), Fedora Silverblue 44, kernel 7.2.9.

## Ambiente de trabalho

- O terminal roda num **distrobox** Debian (`deb`). Comandos no host: `distrobox-host-exec <cmd>`; root no host: `distrobox-host-exec sudo ...` (pede senha).
- **Secure Boot ativo** (`sig_enforce=Y`, lockdown `integrity`): todo `.ko` fora da árvore precisa ser assinado com a MOK do akmods já registrada:
  `sudo /usr/src/kernels/$(uname -r)/scripts/sign-file sha256 /etc/pki/akmods/private/private_key.priv /etc/pki/akmods/certs/public_key.der modulo.ko`
  (a assinatura fica no arquivo; só re-assinar após recompilar). Lockdown bloqueia debugfs gravável → interfaces em **sysfs**.
- Host tem `gcc`, `make`, `kernel-devel`: compilar com `distrobox-host-exec make -C <dir do módulo>`.
- `tuned` + `tuned-ppd` escrevem em `platform_profile`; manter `tuned` em `balanced` (Linux `performance` = Acer Turbo, que este modelo não tem).

## Estado atual da máquina (2026-10-12)

| Item | Estado | Persistente? |
|---|---|---|
| `acer_wmi predator_v4=1 force_caps=0x1C26` | ativo: `platform-profile`, hwmon `acer` com fan1/fan2 + pwm1/pwm2 | ✅ `/etc/modprobe.d/acer-wmi.conf` |
| `acer-wmi-battery` (`drivers/acer-wmi-battery/`) | carregado e validado (`health_mode=1`, `calibration_mode`, `temperature`) | ✅ unit `acer-wmi-modules.service` (validada em reboot 2026-10-12) |
| `acer-wmi-lab` (`drivers/acer-wmi-lab/`) | ferramenta de pesquisa, carregar sob demanda (`insmod` do `.ko` assinado) | — (de propósito) |
| `acer-wmi-extras` (`drivers/acer-wmi-extras/`) | carregado e **validado** (2026-10-12): detecção `mask 0x53`, timeout 0↔30, USB 20↔30, valor inválido rejeitado, hwmon `acer_battery` (25,6 °C, 16 440 mV, 0 mA) | ✅ unit `acer-wmi-modules.service` (validada em reboot 2026-10-12) |
| `tuned` | `balanced` | ✅ |

## Feito

- [x] Dump e decodificação do firmware da máquina: BMOF (classes WMI reais, 25 métodos gaming), DSDT/SSDT (`WMBH` → SMM; sensores em AML; EC mapeada em memória), DMI (§2, §3).
- [x] `acer_wmi` mainline testado no ANV15-52: perfis, fans, temps, PWM via `force_caps` (§1, §8).
- [x] Tecla Nitro: scancode de teclado (keycode 148 = `KEY_PROG1`), sem evento WMI; hwdb já mapeia (§1).
- [x] `acer-wmi-battery` vendorado com fix OOB, assinado, validado (§6).
- [x] **Limite de carga no GNOME** (2026-10-13): `acer-wmi-battery` agora vincula ao dispositivo WMI `79772EC5…` e registra um battery hook ACPI com extensão `power_supply` (kernel ≥ 6.15): `BAT1/charge_control_end_threshold` (80 = health mode, 100 = desligado, outros valores → `-EINVAL`), `power_supply_changed` quando `health_mode` muda pelo atributo antigo, mutex entre os caminhos de escrita. UPower 1.91.5: `ChargeThresholdSupported=true` (só fim, `SettingsSupported=2`); `EnableChargeThreshold true/false/true` → 80/100/80 com `health_mode` 1/0/1, sem erro pela falta do limite inicial. Aparece em Configurações → Energia → "Preservar a saúde da bateria". Como o UPower guarda o próprio estado e o reaplica ao iniciar, o `nitrod` (`internal/hw/upower.go`) passa o `SetChargeLimit` pelo `EnableChargeThreshold` do UPower quando a bateria tem a extensão (dispositivo achado por `NativePath`, conexão própria por chamada, root autorizado pelo polkit); se o UPower falhar, escreve `health_mode`. A leitura continua em `health_mode` (relida no ciclo lento, então mudanças feitas pelo GNOME aparecem). Validado com teste manual: desligar/ligar → `health_mode` 0/1, `ChargeThresholdEnabled` acompanha. Sincronia rápida: a cada troca do limite o driver emite `power_supply_changed` (o daemon relê `ChargeLimit` no uevent, valor em cache sem WMI). O estado "carregando" no GNOME só muda no próximo poll do UPower (o firmware não avisa); aceito, sem código extra.
- [x] Módulo de pesquisa `acer-wmi-lab` (sysfs) + 3 lotes de probes: mapa de capacidades do firmware (§4–§7); formatos de Set do APGe validados (§5).
- [x] Módulo companheiro `acer-wmi-extras` v1 escrito, compilado e **validado no ANV15-52** (`test.sh`, 2026-10-12): `kbd_backlight_timeout`, `kbd_backlight_state`, `usb_charging`, `supported_profiles(_mask)`, hwmon `acer_battery` (temp/tensão/corrente).
- [x] Conferido que teclas de rádio, teclado, eventos e áudio já têm driver in-tree (§1).
- [x] **AC/bateria por uevent** (2026-10-12): `internal/hw/uevent.go` escuta `NETLINK_KOBJECT_UEVENT` (grupo 1, sem libudev) e filtra `power_supply`; releitura de segurança a cada 30 s. Validado: daemon iniciado na bateria, cabo recolocado → `power source changed from=battery to=ac` imediato, `Allowed` recalculado. Tomada → bateria também validado (sem fallback: `balanced` vale nas duas fontes).
- [x] **Tecla Nitro por evdev** (2026-10-12): `internal/hw/nitrokey.go` lê `KEY_PROG1` do teclado interno sem grab e reabre se o dispositivo sumir; ação pela opção `--nitro-key` (`cycle`, padrão sem GUI; `none` quando a GUI/tray assumir a tecla, já que no app original ela abre o app). Validado: 3 toques → `balanced-performance`, `quiet`, `balanced`.
- [x] **Daemon instalado no host e validado no system bus** (2026-10-12): `install-daemon.sh` (build no container, install no host; polkit reiniciado para ler `/usr/local/share/polkit-1/actions`; `DeviceAllow` para NVIDIA), polkit `control` sem senha na sessão ativa, ida e volta com tuned-ppd sem laço, `pwm=0` → piso de ~1 955 RPM. **Achado:** chamadas WMI concorrentes de métodos diferentes corrompem respostas (§9 do firmware-wmi.md) → mutex no daemon, cadência reduzida (6 % → 1,7 % CPU), `poll()` no `platform_profile`, debounce de mudanças externas. Candidato upstream (4): serializar WMI.
- [x] **Esqueleto do daemon Go** (`daemon/`, 2026-10-12): módulo `nitroctl/daemon` (godbus v5.2.3), backends reais (`internal/hw/real.go`) e simulado (`sim.go`), regras puras com testes (`internal/rules`), serviço D-Bus completo da API v1 (propriedades + `PropertiesChanged`, métodos com polkit, erros, Monitor com `Subscribe` unicast, PPD sync, estado em JSON), unit + policy D-Bus + polkit + `packaging/install-daemon.sh`. Validado no bus de sessão: simulado (toda a API, erros `NotAllowed`/`NotSupported`/`InvalidArgument`) e hardware real em `--dry-run` (capacidades do ANV15-52 detectadas do firmware, fans, bateria, monitor). **Ainda não instalado no host.**
- [x] **GPU por NVML instalada e validada** (2026-10-12): `internal/hw/gpu_nvml.go` (go-nvml, `libnvidia-ml.so` em runtime; `nvidia-smi` só como reserva). Na unit: `NVML ready gpu="NVIDIA GeForce RTX 4060 Laptop GPU"`, `gpu.*` presente já na 1ª amostra e igual ao `nvidia-smi` (P8 ocioso: 210/405 MHz, ~1,6 W). **Ganho:** a build anterior, que consultava via `nvidia-smi`, mantinha a GPU acordada (2010/7001 MHz, ~10 W); com NVML a GPU volta a `runtime_status=suspended` entre amostras. Sandbox da unit não atrapalha.
- [x] **Custo de CPU do daemon** (2026-10-12): medidas por atributo no ANV15-52 — `pwmN_enable` ~16 ms, `pwmN` ~7 ms, `platform_profile`/`kbd_*`/`usb_charging`/hwmon `acer_battery` ~5–15 ms (WMI/SMM); `fanN_input`, temps do hwmon `acer` e `power_supply` ≤ 0,2 ms. Mudanças: fans com `RPM()` barato a cada 5 s e modo/velocidade em cache (relidos após escrita, no resume e a cada 60 s); amostras do Monitor só com RPM e `power_supply` (temperatura da bateria no máximo a cada 30 s); **a dGPU nunca é acordada para ser monitorada** (`runtime_status` antes da consulta NVML → `gpu.active=false`). Medido: ocioso **1,7 % → 0,5 %**; Monitor a 1 s **10,4 % → 1,0 %**, e a GPU fica em D3cold (antes cada amostra a mantinha ligada a 1–10 W; acordar custava ~2 s e 1,4 s de CPU de kernel). Com a GPU acordada por outro processo, o Monitor mostra os dados e ela volta a dormir sozinha ~20 s depois.
- [x] **CLI `nitroctl`** (2026-10-12): `daemon/cmd/nitroctl` sobre o pacote público `daemon/client` (que a GUI vai reusar; por isso no mesmo módulo Go do daemon e não em `cli/`). Comandos: `status` (padrão), `caps`, `monitor`, `mode` (aliases `eco`/`turbo`), `fan`, `kbd timeout`, `battery limit|calibrate`, `usb-charging`, `scenario list|save|apply|delete` (`save --from-current`), `appsync`, `reload`, `version`; `--json` em todas as leituras. Instalada pelo `install-daemon.sh` em `/usr/local/bin`. Testada no host contra o daemon real (leituras, escritas, erros do daemon e de uso). Junto: `Health.full/design` corrigido — o BAT1 do ANV15-52 só tem `charge_*` (µAh) e o daemon publicava µAh como µWh; agora converte pela `voltage_min_design` (3,733 Ah × 15,4 V = 57,5 Wh).
- [x] **Cenários + App Sync** (2026-10-12): `internal/scenarios` (formato, validação, `scenarios.json`), `internal/appsync` (proc connector com filtro do kernel, casamento por `comm`/argv com suporte a Wine/Proton, usuário ativo do seat0, testes) e `internal/service/scenarios.go` (Apply/Save/Delete/SetAppSync, `Active` zerado por mudança manual, retrato + reversão). Validado no ANV15-52: cenário com `balanced-performance` + fan CPU max ligado a um executável de teste → aplicado ao abrir, revertido ao fechar; com mudança manual no meio, não reverte; App Sync retomado após restart. Custo ~0,1–0,2 ms por `exec` (era 0,4 ms antes de filtro no kernel, leitura crua do `comm` e processamento sem troca de goroutine). Semântica em api-dbus.md §9. **Falta** testar com um jogo Proton real.
- [x] **Suspend/resume** (2026-10-12): `internal/service/sleep.go` ouve `PrepareForSleep` do logind; reaplica perfil e fans no resume e restaura fans na partida (manual → auto). Achados no ANV15-52: o firmware volta o perfil a `balanced` no resume (o daemon ignora essa escrita e a do tuned até terminar), os fans (max/manual + PWM) sobrevivem ao S3. Validado: `quiet` + max e `quiet` + manual 40 % → resume restaura `quiet`; restart com manual → `auto` (api-dbus.md §11).
- [x] **Marca `[N]` e ícone** (2026-10-13): em `assets/logo/` ficam só os em uso: `n-icon-cut.svg` (ícone do app: quadrado arredondado escuro com base #FFA000 aparecendo embaixo, efeito 3D no estilo dos ícones do GNOME, `[N]` em baixo-relevo) e `n-mark-cut.svg` (o mesmo `[N]` sem o fundo: N em `currentColor`, colchetes em `var(--accent, #ff1b19)`). Letras da Source Code Pro Black (SIL OFL 1.1) em contorno. As variantes descartadas (incluindo o octógono com degradê, parecido demais com o ícone da Acer) foram para `../referencias/logo/`, fora do repositório.
- [x] **GUI `nitro-control-center` v1** (2026-10-13): `gui/` em Go + Wails v2 (WebKitGTK 4.1, tag `webkit2_41`), frontend TS sem framework, CSS e ícones próprios, fonte do sistema (`system-ui`), marca `[N]` na barra lateral e `n-icon-cut.svg` como ícone do app, sem 3D/Lottie. Telas: Painel (modos do firmware com `Allowed`/`Extras`, medidores de temperatura e RPM, CPU/GPU/RAM/rede), Monitor (uPlot, 2/5/10 min), Ventoinhas (auto/máx/manual por fan e para todas; manual preserva o PWM atual), Iluminação (estado e timeout), Bateria (carga, saúde, limite 80 %, calibração), Cenários + App Sync (editor, salvar estado atual, renomear), Configurações (USB charging, °C/°F, idioma pt/en, iniciar com a sessão via XDG autostart, sobre/módulos, `Reload`). Ponte `gui/app.go`: `State()` com as propriedades de todas as interfaces, evento `state` a cada `PropertiesChanged` (agrupado em 80 ms), reconexão automática quando o daemon reinicia (tela "sem conexão"), Monitor só com janela visível numa tela ao vivo, erros do daemon como `{kind,msg}` traduzidos. Instância única (abrir de novo traz a janela). `daemon/client.Plain` passou a ser compartilhado pela CLI e pela GUI. Testado no navegador do `wails dev` contra o daemon simulado (todas as telas, erros `NotSupported`/`NotAllowed`/`InvalidArgument`, queda e volta do daemon) e aberto no host contra o `nitrod` real (conecta e assina o Monitor). Empacotamento: `packaging/install-gui.sh` + `io.github.fnndk.NitroControlCenter.desktop`.
- [x] **GUI: janela integrada** (2026-10-13): sem moldura do sistema (`Frameless` + fundo transparente); barra própria no tema (`gui/titlebar.go`, `frontend/src/titlebar.ts`) com a ordem dos botões do `button-layout` do GNOME (portal de configurações, fallback `gsettings`, atualiza ao mudar) e os ícones `window-*-symbolic` do tema de ícones lidos do sistema em runtime; cantos de 12 px e sombra numa margem transparente de 18 px (`winMargin`/`--win-margin`, área de redimensionar cobre a sombra), tudo some ao maximizar. Limitação: o GNOME vê a margem como parte da janela (encaixe lado a lado deixa um vão). **Por desktop** (2026-10-13): no KDE/LXQt (`XDG_CURRENT_DESKTOP`) a janela fica com a moldura do compositor (`Frameless` off, fundo opaco, sem margem nem botões próprios; classe `ssd` no frontend), sobreposição com `NITROCTL_DECORATIONS=client|server`. Compilado; **falta testar numa sessão Plasma**. Marca `[N]` com os colchetes da fonte, como no ícone, sem o fundo: `assets/logo/n-mark-cut.svg` (barra lateral e tela de conexão).
- [x] **Bandeja e tecla Nitro configurável** (2026-10-13): agente da sessão `nitro-tray` (`daemon/cmd/nitro-tray`, `fyne.io/systray`, Go puro sobre D-Bus) com status, abrir o app, modos (marcado o atual, desabilitados os não permitidos, extras sinalizados), ventoinhas no máximo, submenu de cenários + App Sync e "ocultar até o próximo login"; recusas do daemon viram notificação. Daemon: `System.NitroKey`/`SetNitroKey` (`app` | `cycle`, persistido) e sinal `NitroKeyPressed`; em `app` o agente abre o app. Configurações da GUI: "Ícone na bandeja" (`~/.config/nitroctl/settings.json`, pacote `daemon/usercfg`) e "Tecla Nitro" (abrir o app / alternar os modos). O agente inicia no login (`/etc/xdg/autostart`) e sai quando não tem função; a GUI o recarrega ou inicia. Testado com o daemon simulado no GNOME (AppIndicator): menu lido e clicado por `com.canonical.dbusmenu`, ciclo de vida sem ícone, sinal aceito só do dono do nome. **Falta** validar o toque real da tecla no host.
- [x] **Fix no daemon** (2026-10-13): `conn.Send` devolve `*dbus.Call`; o Monitor tratava o retorno como erro e logava `signal send failed` a cada amostra para cada assinante. Agora checa `call.Err`. **Reinstalar o daemon** para sumir o aviso do journal.
- [x] **Persistência validada em reboot** (2026-10-12): `packaging/install-modules.sh` instala os `.ko` assinados em `/var/lib/acer-wmi-modules/` (rótulo `modules_object_t`) e habilita `acer-wmi-modules.service`; `packaging/check.sh` confirma a camada inteira após o boot.

## Próximo passo imediato

0. **Instalar GUI e daemon corrigido** (2026-10-13): no container `sh packaging/install-daemon.sh build && sh packaging/install-gui.sh build`; no host `distrobox-host-exec sudo sh packaging/install-daemon.sh install && distrobox-host-exec sudo sh packaging/install-gui.sh`. Abrir "Nitro Control Center" pelo menu e conferir no hardware real as escritas (modo, fans, timeout, limite, USB, cenários); polkit `settings` deve pedir senha.
1. **GUI, o que falta:** validar no host a bandeja e a tecla Nitro em "abrir o app" (reinstalar daemon e GUI); widget de monitor; Flatpak.
2. **Completar o daemon:** App Sync com um jogo Proton real.
3. Manutenção da camada de kernel: após atualização de kernel, `distrobox-host-exec sudo sh packaging/build-sign.sh`; `sh packaging/check.sh` para conferir. Depois, akmods/DKMS.

## Backlog (ordem sugerida)

1. **Patches upstream `acer_wmi`** (§8): entrada DMI `Nitro ANV15-52` (`predator_v4` + `pwm`); `platform_profile_probe` lendo a máscara 0x0A; battery hook `charge_control_end_threshold`; **serialização das chamadas WMI** (respostas corrompidas sob concorrência, §9).
2. **Persistência/empacotamento** dos módulos (akmods ou DKMS + unit systemd), script único de build+sign.
3. **Daemon `nitrod` (Go)** — `daemon/` (esqueleto feito 2026-10-12; itens abaixo são o alvo completo)
   - D-Bus system bus `io.github.fnndk.Nitrod1` (godbus), polkit para ações privilegiadas, unit systemd `nitrod.service`.
   - Backends: `platform-profile` (modo), hwmon `acer` (RPM, temps, `pwm*`), `acer-wmi-extras` (timeout, USB, perfis suportados, hwmon bateria), `acer-wmi-battery` (limite 80 %, calibração), NVML (`nvidia-smi`/go-nvml: temp, clocks, uso, potência), `/proc` + `/sys` (CPU, RAM, rede), UPower/udev (AC, bateria), evdev (tecla Nitro `KEY_PROG1`).
   - API espelhando o app original: `GetCapabilities`, `Get/SetOperatingMode`, `Get/SetFanMode` (auto/máx/manual por fan), `GetFanSpeeds`, `GetTemperatures`, `Get/SetBacklightTimeout`, `Get/SetUsbCharging`, `Get/SetChargeLimit`, `StartCalibration`, `GetMonitor` (stream por sinal), perfis/cenários (`List/Apply/Save/Delete`), App Sync (watcher de processos), sinais de mudança.
   - Regras: Eco só na bateria; Turbo/OC só com AC e ≥ 40 %; restaurar último estado no boot e no resume; ciclo de modos pela tecla Nitro com a lista de perfis vinda do firmware.
   - CLI `nitroctl` sobre a mesma API.
4. **GUI `nitro-control-center` (Go + Wails v2)** — `gui/` (v1 feita 2026-10-13; faltam tray, tecla Nitro, widget, Flatpak)
   - Frontend web (TS + CSS; gráficos com uPlot; animações Lottie; opcional three.js), bindings Go → D-Bus.
   - Telas 1:1: Home (modo + monitor), Fan Control (auto/máx/manual, sem curva onde o firmware não tem), Lighting (timeout; zonas/efeitos só em modelos com RGB), Battery (limite 80 %, calibração, saúde), Settings (USB charging, unidades, inicialização), Scenario Manager + App Sync, widget de monitor, tray (StatusNotifierItem) com modos rápidos.
   - A GUI esconde o que `GetCapabilities` não listar (ex.: Turbo no ANV15-52).
   - **Arte e textos próprios:** nada de fontes da marca (usar fonte livre, ex. Noto/Inter), ícones, fundos, animações Lottie, modelos 3D, sons, vídeos ou textos de interface do app original; ícone do app sem o octógono com degradê. Material Icons (Apache-2.0) e Noto (OFL) são livres. Nomes "Nitro", "NitroSense", "Predator", "PredatorSense" são marcas da Acer: decidir o nome público antes de publicar.
   - Empacotamento: Flatpak (fala D-Bus com o daemon do host); RPM/akmods para módulos + daemon.
5. **Extras para outros modelos** (auto-detectados pelo firmware): RGB 4 zonas/per-key, LEDs, som de boot, LCD overdrive, fan table, OC de CPU/GPU (NVML), CPU power limit (RAPL).
6. Fora do escopo: serviços online da Acer, atualizações de BIOS/drivers, boot logo, DTS, MagKey, app mobile.

### Esqueleto sugerido do repositório

```
drivers/            módulos C (extras, battery, lab)
daemon/             Go: cmd/{nitrod,nitroctl}, client/, internal/{hw,rules,service,monitor,ppd,state,scenarios,appsync}
gui/                Go + Wails: app.go, frontend/ (TS, CSS, assets próprios)
packaging/          systemd (módulos, daemon), polkit, dbus, install-daemon.sh; akmods/dkms, flatpak a fazer
docs/               firmware-wmi.md, api-dbus.md
```

## Como retomar (comandos)

```bash
# compilar um módulo no host (ex.: extras)
distrobox-host-exec make -C /var/home/klumpp/work/nitro-sense/nitroctl/drivers/acer-wmi-extras
# assinar + carregar (acer-wmi-battery, acer-wmi-lab ou acer-wmi-extras)
distrobox-host-exec sudo sh -c 'K=/usr/src/kernels/$(uname -r); M=/var/home/klumpp/work/nitro-sense/nitroctl/drivers/acer-wmi-extras/acer-wmi-extras.ko; "$K/scripts/sign-file" sha256 /etc/pki/akmods/private/private_key.priv /etc/pki/akmods/certs/public_key.der "$M" && insmod "$M"'
# chamadas WMI arbitrárias (pesquisa), com acer-wmi-lab carregado:
#   echo "<GUID> <método> <bytes hex>" | sudo tee /sys/devices/platform/acer-wmi-lab/call; cat /sys/devices/platform/acer-wmi-lab/call
# lotes prontos: drivers/acer-wmi-lab/probe.sh, probe2.sh, probe3.sh; tecla: keywatch.py
# daemon sem root (bus de sessão): simulado ou hardware real só leitura
(cd daemon && go test ./... && go build ./cmd/nitrod && ./nitrod --session --simulate ANV15-52 --state /tmp/ns.json)
(cd daemon && ./nitrod --session --dry-run --state /tmp/ns.json)
busctl --user introspect io.github.fnndk.Nitrod1 /io/github/fnndk/Nitrod1
# instalar: sh packaging/install-daemon.sh build  (container)  →  distrobox-host-exec sudo sh packaging/install-daemon.sh  (host)
```

## Mapa de arquivos

- `README.md` — visão geral e instruções de build/assinatura.
- `docs/firmware-wmi.md` — referência técnica (classes WMI, formatos, EC, capacidades, patches candidatos).
- `drivers/acer-wmi-battery/` — vendorado (ver `UPSTREAM.md`), `.ko` assinado.
- `drivers/acer-wmi-lab/` — módulo de pesquisa + `probe*.sh`, `keywatch.py`, resultados `probe*-<data>.txt`.
- `drivers/acer-wmi-extras/` — companheiro v1 + `test.sh`.
- `packaging/` — módulos: `acer-wmi-modules.service`, `install-modules.sh`, `build-sign.sh`, `check.sh`; daemon: `nitrod.service`, `io.github.fnndk.Nitrod1.conf` (D-Bus), `io.github.fnndk.nitrod.policy` (polkit), `install-daemon.sh`.
- `daemon/` — `nitrod` em Go (ver `daemon/README.md`).
- `docs/api-dbus.md` — API D-Bus do daemon (rascunho v1, revisado 2026-10-12).
- `gui/` — `nitro-control-center` (ver `gui/README.md`); `packaging/install-gui.sh`, `packaging/io.github.fnndk.NitroControlCenter.desktop`.

## Regras acordadas

- Não substituir o `acer_wmi`; não criar flags de marca; capacidades pelo firmware.
- Nome do companheiro: **`acer-wmi-extras`** (vale de Aspire a Predator).
- Linguagens: C (kernel), Go (daemon, CLI), Go + Wails (GUI). GUI com arte própria.
- Só métodos "Get" em probes exploratórios; "Set" só com formato validado e reversível.
- Nomes: `nitroctl` (projeto/CLI), `nitrod` (daemon), `nitro-control-center` (GUI), namespace `io.github.fnndk`.
- Funções além do app original (ex.: Performance na bateria) são permitidas, mas sempre **sinalizadas como extra** na API (`Mode.Extras`) e na GUI.
- Antes de persistir opções que criam `platform_profile`, conferir o perfil do `tuned`.
