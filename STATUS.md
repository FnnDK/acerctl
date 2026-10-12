# NitroSense para Linux — STATUS (ponto de retomada)

Última atualização: 2026-10-12. Leia este arquivo primeiro; a referência técnica está em `docs/firmware-wmi.md` (citada abaixo como §N).

## Objetivo e decisão de arquitetura

Reproduzir no Linux as funções do Acer NitroSense/PredatorSense **coexistindo com o `acer_wmi` in-tree**, sem substituí-lo: o que faltar entra por (a) patches upstream no `acer_wmi` e (b) módulos companheiros fora da árvore + um daemon + uma GUI. Sem flag de marca: as capacidades vêm do firmware (máscaras WMI, DMI); tabela por modelo só para o que o firmware não informa. Linuwu-Sense e Div-Acer-Manager-Max são referência, não base (§10).

**Pilha decidida (2026-10-12):** módulos de kernel em **C** (`drivers/`), daemon em **Go** (serviço D-Bus no system bus, polkit, systemd), GUI 1:1 em **Go + Wails** (frontend web em WebView/WebKitGTK), CLI e tray como clientes da mesma API D-Bus. Arte própria na GUI (ícones, fontes, animações): nada de material da Acer.

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
- [x] Módulo de pesquisa `acer-wmi-lab` (sysfs) + 3 lotes de probes: mapa de capacidades do firmware (§4–§7); formatos de Set do APGe validados (§5).
- [x] Módulo companheiro `acer-wmi-extras` v1 escrito, compilado e **validado no ANV15-52** (`test.sh`, 2026-10-12): `kbd_backlight_timeout`, `kbd_backlight_state`, `usb_charging`, `supported_profiles(_mask)`, hwmon `acer_battery` (temp/tensão/corrente).
- [x] Conferido que teclas de rádio, teclado, eventos e áudio já têm driver in-tree (§1).
- [x] **AC/bateria por uevent** (2026-10-12): `internal/hw/uevent.go` escuta `NETLINK_KOBJECT_UEVENT` (grupo 1, sem libudev) e filtra `power_supply`; releitura de segurança a cada 30 s. Validado: daemon iniciado na bateria, cabo recolocado → `power source changed from=battery to=ac` imediato, `Allowed` recalculado. Tomada → bateria também validado (sem fallback: `balanced` vale nas duas fontes).
- [x] **Tecla Nitro por evdev** (2026-10-12): `internal/hw/nitrokey.go` lê `KEY_PROG1` do teclado interno sem grab e reabre se o dispositivo sumir; ação pela opção `--nitro-key` (`cycle`, padrão sem GUI; `none` quando a GUI/tray assumir a tecla, já que no app original ela abre o app). Validado: 3 toques → `balanced-performance`, `quiet`, `balanced`.
- [x] **Daemon instalado no host e validado no system bus** (2026-10-12): `install-daemon.sh` (build no container, install no host; polkit reiniciado para ler `/usr/local/share/polkit-1/actions`; `DeviceAllow` para NVIDIA), polkit `control` sem senha na sessão ativa, ida e volta com tuned-ppd sem laço, `pwm=0` → piso de ~1 955 RPM. **Achado:** chamadas WMI concorrentes de métodos diferentes corrompem respostas (§9 do firmware-wmi.md) → mutex no daemon, cadência reduzida (6 % → 1,7 % CPU), `poll()` no `platform_profile`, debounce de mudanças externas. Candidato upstream (4): serializar WMI.
- [x] **Esqueleto do daemon Go** (`daemon/`, 2026-10-12): módulo `nitrosense/daemon` (godbus v5.2.3), backends reais (`internal/hw/real.go`) e simulado (`sim.go`), regras puras com testes (`internal/rules`), serviço D-Bus completo da API v1 (propriedades + `PropertiesChanged`, métodos com polkit, erros, Monitor com `Subscribe` unicast, PPD sync, estado em JSON), unit + policy D-Bus + polkit + `packaging/install-daemon.sh`. Validado no bus de sessão: simulado (toda a API, erros `NotAllowed`/`NotSupported`/`InvalidArgument`) e hardware real em `--dry-run` (capacidades do ANV15-52 detectadas do firmware, fans, bateria, monitor). **Ainda não instalado no host.**
- [x] **GPU por NVML instalada e validada** (2026-10-12): `internal/hw/gpu_nvml.go` (go-nvml, `libnvidia-ml.so` em runtime; `nvidia-smi` só como reserva). Na unit: `NVML ready gpu="NVIDIA GeForce RTX 4060 Laptop GPU"`, `gpu.*` presente já na 1ª amostra e igual ao `nvidia-smi` (P8 ocioso: 210/405 MHz, ~1,6 W). **Ganho:** a build anterior, que consultava via `nvidia-smi`, mantinha a GPU acordada (2010/7001 MHz, ~10 W); com NVML a GPU volta a `runtime_status=suspended` entre amostras. Sandbox da unit não atrapalha.
- [x] **Suspend/resume** (2026-10-12): `internal/service/sleep.go` ouve `PrepareForSleep` do logind; reaplica perfil e fans no resume e restaura fans na partida (manual → auto). Achados no ANV15-52: o firmware volta o perfil a `balanced` no resume (o daemon ignora essa escrita e a do tuned até terminar), os fans (max/manual + PWM) sobrevivem ao S3. Validado: `quiet` + max e `quiet` + manual 40 % → resume restaura `quiet`; restart com manual → `auto` (api-dbus.md §11).
- [x] **Persistência validada em reboot** (2026-10-12): `packaging/install-modules.sh` instala os `.ko` assinados em `/var/lib/acer-wmi-modules/` (rótulo `modules_object_t`) e habilita `acer-wmi-modules.service`; `packaging/check.sh` confirma a camada inteira após o boot.

## Próximo passo imediato

1. **Instalado (build com mutex/poll/debounce, 2026-10-12): CPU ociosa 0,5 %**; GPU agora por NVML (go-nvml, `libnvidia-ml.so` carregada em runtime; `nvidia-smi` só como reserva). Ação `settings` confirmada: o gnome-shell abre o diálogo de senha (chamadas via `distrobox-host-exec` caem na sessão gráfica 5 do logind). 
2. **Completar o daemon** (ordem sugerida): cenários + App Sync (`internal/scenarios`, `internal/appsync`); CLI `nitrosense-cli`.
3. Manutenção da camada de kernel: após atualização de kernel, `distrobox-host-exec sudo sh packaging/build-sign.sh`; `sh packaging/check.sh` para conferir. Depois, akmods/DKMS.

## Backlog (ordem sugerida)

1. **Patches upstream `acer_wmi`** (§8): entrada DMI `Nitro ANV15-52` (`predator_v4` + `pwm`); `platform_profile_probe` lendo a máscara 0x0A; battery hook `charge_control_end_threshold`; **serialização das chamadas WMI** (respostas corrompidas sob concorrência, §9).
2. **Persistência/empacotamento** dos módulos (akmods ou DKMS + unit systemd), script único de build+sign.
3. **Daemon `nitrosensed` (Go)** — `daemon/` (esqueleto feito 2026-10-12; itens abaixo são o alvo completo)
   - D-Bus system bus `org.nitrosense.Daemon1` (godbus), polkit para ações privilegiadas, unit systemd `nitrosensed.service`.
   - Backends: `platform-profile` (modo), hwmon `acer` (RPM, temps, `pwm*`), `acer-wmi-extras` (timeout, USB, perfis suportados, hwmon bateria), `acer-wmi-battery` (limite 80 %, calibração), NVML (`nvidia-smi`/go-nvml: temp, clocks, uso, potência), `/proc` + `/sys` (CPU, RAM, rede), UPower/udev (AC, bateria), evdev (tecla Nitro `KEY_PROG1`).
   - API espelhando o app original: `GetCapabilities`, `Get/SetOperatingMode`, `Get/SetFanMode` (auto/máx/manual por fan), `GetFanSpeeds`, `GetTemperatures`, `Get/SetBacklightTimeout`, `Get/SetUsbCharging`, `Get/SetChargeLimit`, `StartCalibration`, `GetMonitor` (stream por sinal), perfis/cenários (`List/Apply/Save/Delete`), App Sync (watcher de processos), sinais de mudança.
   - Regras: Eco só na bateria; Turbo/OC só com AC e ≥ 40 %; restaurar último estado no boot e no resume; ciclo de modos pela tecla Nitro com a lista de perfis vinda do firmware.
   - CLI `nitrosense-cli` sobre a mesma API.
4. **GUI `nitrosense` (Go + Wails v2)** — `gui/` (a criar)
   - Frontend web (TS + CSS; gráficos com uPlot; animações Lottie; opcional three.js), bindings Go → D-Bus.
   - Telas 1:1: Home (modo + monitor), Fan Control (auto/máx/manual, sem curva onde o firmware não tem), Lighting (timeout; zonas/efeitos só em modelos com RGB), Battery (limite 80 %, calibração, saúde), Settings (USB charging, unidades, inicialização), Scenario Manager + App Sync, widget de monitor, tray (StatusNotifierItem) com modos rápidos.
   - A GUI esconde o que `GetCapabilities` não listar (ex.: Turbo no ANV15-52).
   - Empacotamento: Flatpak (fala D-Bus com o daemon do host); RPM/akmods para módulos + daemon.
5. **Extras para outros modelos** (auto-detectados pelo firmware): RGB 4 zonas/per-key, LEDs, som de boot, LCD overdrive, fan table, OC de CPU/GPU (NVML), CPU power limit (RAPL).
6. Fora do escopo: serviços online da Acer, atualizações de BIOS/drivers, boot logo, DTS, MagKey, app mobile.

### Esqueleto sugerido do repositório

```
drivers/            módulos C (extras, battery, lab)
daemon/             Go: cmd/nitrosensed, internal/{hw,rules,service,monitor,ppd,state} (+ scenarios, appsync, nvml a fazer)
cli/                Go: cmd/nitrosense-cli
gui/                Go + Wails: app.go, frontend/ (TS, CSS, assets próprios)
packaging/          systemd (módulos, daemon), polkit, dbus, install-daemon.sh; akmods/dkms, flatpak a fazer
docs/               firmware-wmi.md, api-dbus.md
```

## Como retomar (comandos)

```bash
# compilar um módulo no host (ex.: extras)
distrobox-host-exec make -C /var/home/klumpp/work/nitro-sense/nitrosense-linux/drivers/acer-wmi-extras
# assinar + carregar (acer-wmi-battery, acer-wmi-lab ou acer-wmi-extras)
distrobox-host-exec sudo sh -c 'K=/usr/src/kernels/$(uname -r); M=/var/home/klumpp/work/nitro-sense/nitrosense-linux/drivers/acer-wmi-extras/acer-wmi-extras.ko; "$K/scripts/sign-file" sha256 /etc/pki/akmods/private/private_key.priv /etc/pki/akmods/certs/public_key.der "$M" && insmod "$M"'
# chamadas WMI arbitrárias (pesquisa), com acer-wmi-lab carregado:
#   echo "<GUID> <método> <bytes hex>" | sudo tee /sys/devices/platform/acer-wmi-lab/call; cat /sys/devices/platform/acer-wmi-lab/call
# lotes prontos: drivers/acer-wmi-lab/probe.sh, probe2.sh, probe3.sh; tecla: keywatch.py
# daemon sem root (bus de sessão): simulado ou hardware real só leitura
(cd daemon && go test ./... && go build ./cmd/nitrosensed && ./nitrosensed --session --simulate ANV15-52 --state /tmp/ns.json)
(cd daemon && ./nitrosensed --session --dry-run --state /tmp/ns.json)
busctl --user introspect org.nitrosense.Daemon1 /org/nitrosense/Daemon1
# instalar: sh packaging/install-daemon.sh build  (container)  →  distrobox-host-exec sudo sh packaging/install-daemon.sh  (host)
```

## Mapa de arquivos

- `README.md` — visão geral e instruções de build/assinatura.
- `docs/firmware-wmi.md` — referência técnica (classes WMI, formatos, EC, capacidades, patches candidatos).
- `drivers/acer-wmi-battery/` — vendorado (ver `UPSTREAM.md`), `.ko` assinado.
- `drivers/acer-wmi-lab/` — módulo de pesquisa + `probe*.sh`, `keywatch.py`, resultados `probe*-<data>.txt`.
- `drivers/acer-wmi-extras/` — companheiro v1 + `test.sh`.
- `packaging/` — módulos: `acer-wmi-modules.service`, `install-modules.sh`, `build-sign.sh`, `check.sh`; daemon: `nitrosensed.service`, `org.nitrosense.Daemon1.conf` (D-Bus), `org.nitrosense.daemon.policy` (polkit), `install-daemon.sh`.
- `daemon/` — `nitrosensed` em Go (ver `daemon/README.md`).
- `docs/api-dbus.md` — API D-Bus do daemon (rascunho v1, revisado 2026-10-12).

## Regras acordadas

- Não substituir o `acer_wmi`; não criar flags de marca; capacidades pelo firmware.
- Nome do companheiro: **`acer-wmi-extras`** (vale de Aspire a Predator).
- Linguagens: C (kernel), Go (daemon, CLI), Go + Wails (GUI). GUI com arte própria.
- Só métodos "Get" em probes exploratórios; "Set" só com formato validado e reversível.
- Funções além do app original (ex.: Performance na bateria) são permitidas, mas sempre **sinalizadas como extra** na API (`Mode.Extras`) e na GUI.
- Antes de persistir opções que criam `platform_profile`, conferir o perfil do `tuned`.
