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
- [x] **Persistência validada em reboot** (2026-10-12): `packaging/install-modules.sh` instala os `.ko` assinados em `/var/lib/acer-wmi-modules/` (rótulo `modules_object_t`) e habilita `acer-wmi-modules.service`; `packaging/check.sh` confirma a camada inteira após o boot.

## Próximo passo imediato

1. **Daemon Go** (`daemon/`): `docs/api-dbus.md` revisado para **v1** (2026-10-12: propriedades + `PropertiesChanged`, regras de energia completas, tabela de detecção de capacidades, erros, polkit `control`/`settings`, `Scenarios`); decidido: Performance na bateria com ≥ 40 % como função **extra** sinalizada (`Mode.Extras`); fan manual sem piso fixo (faixa 0–100 em `capabilities.fans.range`, firmware decide); convivência com tuned-ppd decidida (escrita direta no sysfs + `ActiveProfile` sincronizado nos dois sentidos, ver §11 do doc); pendentes só os testes de validação do §13 (`pwm=0` em manual; laço com tuned-ppd) e criar o esqueleto: módulo Go, serviço D-Bus (godbus), `GetCapabilities`, backends de leitura (modo, fans, temps, extras, bateria), unit + policy D-Bus + polkit em `packaging/`. Toolchain: Go 1.27 instalado no container; Node/Wails ficam para a fase da GUI (host tem `webkit2gtk4.1` e `gtk3`).
2. Manutenção da camada de kernel: após atualização de kernel, `distrobox-host-exec sudo sh packaging/build-sign.sh` (recompila, assina e atualiza a cópia instalada); `sh packaging/check.sh` para conferir. Depois, akmods/DKMS.

## Backlog (ordem sugerida)

1. **Patches upstream `acer_wmi`** (§8): entrada DMI `Nitro ANV15-52` (`predator_v4` + `pwm`); `platform_profile_probe` lendo a máscara 0x0A; battery hook `charge_control_end_threshold`.
2. **Persistência/empacotamento** dos módulos (akmods ou DKMS + unit systemd), script único de build+sign.
3. **Daemon `nitrosensed` (Go)** — `daemon/` (a criar)
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
daemon/             Go: cmd/nitrosensed, internal/{dbus,hw,profiles,appsync,nvml}
cli/                Go: cmd/nitrosense-cli
gui/                Go + Wails: app.go, frontend/ (TS, CSS, assets próprios)
packaging/          systemd, polkit, dbus, akmods/dkms, flatpak
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
```

## Mapa de arquivos

- `README.md` — visão geral e instruções de build/assinatura.
- `docs/firmware-wmi.md` — referência técnica (classes WMI, formatos, EC, capacidades, patches candidatos).
- `drivers/acer-wmi-battery/` — vendorado (ver `UPSTREAM.md`), `.ko` assinado.
- `drivers/acer-wmi-lab/` — módulo de pesquisa + `probe*.sh`, `keywatch.py`, resultados `probe*-<data>.txt`.
- `drivers/acer-wmi-extras/` — companheiro v1 + `test.sh`.
- `packaging/` — `acer-wmi-modules.service`, `install-modules.sh`, `build-sign.sh`.
- `docs/api-dbus.md` — API D-Bus do daemon (rascunho v1, revisado 2026-10-12).

## Regras acordadas

- Não substituir o `acer_wmi`; não criar flags de marca; capacidades pelo firmware.
- Nome do companheiro: **`acer-wmi-extras`** (vale de Aspire a Predator).
- Linguagens: C (kernel), Go (daemon, CLI), Go + Wails (GUI). GUI com arte própria.
- Só métodos "Get" em probes exploratórios; "Set" só com formato validado e reversível.
- Funções além do app original (ex.: Performance na bateria) são permitidas, mas sempre **sinalizadas como extra** na API (`Mode.Extras`) e na GUI.
- Antes de persistir opções que criam `platform_profile`, conferir o perfil do `tuned`.
