# acerctl — STATUS (resume point)

Last updated: 2026-10-13. Read this file first; the technical reference is `docs/firmware-wmi.md` (cited below as §N).

## Goal and architecture decision

All-in-one Linux alternative to Acer's NitroSense (working), AcerSense (in progress, Aspire AG15-71P) and PredatorSense (to do), showing each feature only when the firmware offers it. Reproduce their features on Linux **coexisting with the in-tree `acer_wmi`**, without replacing it: whatever is missing comes from (a) upstream patches to `acer_wmi` and (b) out-of-tree companion modules + a daemon + a GUI. No brand flag: capabilities come from the firmware (WMI masks, DMI); a per-model table only for what the firmware does not report. Linuwu-Sense and Div-Acer-Manager-Max are references, not a base (§10).

**Stack decided (2026-10-12):** kernel modules in **C** (`drivers/`), daemon in **Go** (D-Bus service on the system bus, polkit, systemd), 1:1 GUI in **Go + Wails** (web frontend in WebView/WebKitGTK), CLI and tray as clients of the same D-Bus API. Own artwork in the GUI (icons, fonts, animations): no Acer material.

**Names (2026-10-13, renamed from nitroctl/nitrod the same day), following the asusctl scheme:** project, repository and CLI `acerctl`, daemon `acerd`, tray agent `acer-tray`; GUI in two flavors from the same `gui/` code, chosen per product line at install time (branding only, capabilities still come from the firmware): `nitro-control-center` (red, Nitro/Predator) and `acer-control-center` (green, other lines such as Aspire — to do); reverse-DNS prefix `io.github.fnndk` (D-Bus `io.github.fnndk.Acerd1` at `/io/github/fnndk/Acerd1`, polkit `io.github.fnndk.acerd.{control,settings}`, state in `/var/lib/acerd`). "Nitro"/"Acer" only as references to the hardware; the README must say there is no affiliation with Acer.

Reference machine: Acer **Nitro ANV15-52** (i7-13620H, RTX 4060, BIOS V1.31), Fedora Silverblue 44, kernel 7.2.9.

## Work environment

- The terminal runs in a Debian **distrobox** (`deb`). Host commands: `distrobox-host-exec <cmd>`; root on the host: `distrobox-host-exec sudo ...` (asks for a password).
- **Secure Boot on** (`sig_enforce=Y`, lockdown `integrity`): every out-of-tree `.ko` must be signed with the already enrolled akmods MOK:
  `sudo /usr/src/kernels/$(uname -r)/scripts/sign-file sha256 /etc/pki/akmods/private/private_key.priv /etc/pki/akmods/certs/public_key.der module.ko`
  (the signature stays in the file; only re-sign after rebuilding). Lockdown blocks writable debugfs → interfaces in **sysfs**.
- The host has `gcc`, `make`, `kernel-devel`: build with `distrobox-host-exec make -C <module dir>`.
- `tuned` + `tuned-ppd` write to `platform_profile`; keep `tuned` on `balanced` (Linux `performance` = Acer Turbo, which this model does not have).

## Current machine state (2026-10-12)

| Item | State | Persistent? |
|---|---|---|
| `acer_wmi predator_v4=1 force_caps=0x1C26` | active: `platform-profile`, hwmon `acer` with fan1/fan2 + pwm1/pwm2 | ✅ `/etc/modprobe.d/acer-wmi.conf` |
| `acer-wmi-battery` (`drivers/acer-wmi-battery/`) | loaded and validated (`health_mode=1`, `calibration_mode`, `temperature`) | ✅ unit `acer-wmi-modules.service` (validated across a reboot 2026-10-12) |
| `acer-wmi-lab` (`drivers/acer-wmi-lab/`) | research tool, loaded on demand (`insmod` of the signed `.ko`) | — (on purpose) |
| `acer-wmi-extras` (`drivers/acer-wmi-extras/`) | loaded and **validated** (2026-10-12): detection `mask 0x53`, timeout 0↔30, USB 20↔30, invalid value rejected, hwmon `acer_battery` (25.6 °C, 16 440 mV, 0 mA) | ✅ unit `acer-wmi-modules.service` (validated across a reboot 2026-10-12) |
| `tuned` | `balanced` | ✅ |

## Done

- [x] Dump and decoding of the machine's firmware: BMOF (real WMI classes, 25 gaming methods), DSDT/SSDT (`WMBH` → SMM; sensors in AML; memory-mapped EC), DMI (§2, §3).
- [x] Mainline `acer_wmi` tested on the ANV15-52: profiles, fans, temps, PWM via `force_caps` (§1, §8).
- [x] Nitro key: keyboard scancode (keycode 148 = `KEY_PROG1`), no WMI event; hwdb already maps it (§1).
- [x] `acer-wmi-battery` vendored with the OOB fix, signed, validated (§6).
- [x] **Charge limit in GNOME** (2026-10-13): `acer-wmi-battery` now binds to the WMI device `79772EC5…` and registers an ACPI battery hook with a `power_supply` extension (kernel ≥ 6.15): `BAT1/charge_control_end_threshold` (80 = health mode, 100 = off, other values → `-EINVAL`), `power_supply_changed` when `health_mode` changes through the old attribute, a mutex between the write paths. UPower 1.91.5: `ChargeThresholdSupported=true` (end only, `SettingsSupported=2`); `EnableChargeThreshold true/false/true` → 80/100/80 with `health_mode` 1/0/1, no error from the missing start threshold. It shows up in Settings → Power → "Preserve Battery Health". Since UPower keeps its own state and reapplies it at startup, `acerd` (`internal/hw/upower.go`) routes `SetChargeLimit` through UPower's `EnableChargeThreshold` when the battery has the extension (device found by `NativePath`, a dedicated connection per call, root authorized by polkit); if UPower fails, it writes `health_mode`. Reading still uses `health_mode` (reread in the slow cycle, so changes made from GNOME show up). Validated with a manual test: off/on → `health_mode` 0/1, `ChargeThresholdEnabled` follows. Fast sync: on every limit change the driver emits `power_supply_changed` (the daemon rereads `ChargeLimit` on the uevent, cached value without WMI). The "charging" state in GNOME only changes on UPower's next poll (the firmware does not notify); accepted, no extra code.
- [x] Research module `acer-wmi-lab` (sysfs) + 3 batches of probes: map of firmware capabilities (§4–§7); APGe Set formats validated (§5).
- [x] Companion module `acer-wmi-extras` v1 written, built and **validated on the ANV15-52** (`test.sh`, 2026-10-12): `kbd_backlight_timeout`, `kbd_backlight_state`, `usb_charging`, `supported_profiles(_mask)`, hwmon `acer_battery` (temp/voltage/current).
- [x] Checked that the radio, keyboard, event and audio keys already have in-tree drivers (§1).
- [x] **AC/battery via uevent** (2026-10-12): `internal/hw/uevent.go` listens on `NETLINK_KOBJECT_UEVENT` (group 1, no libudev) and filters `power_supply`; safety reread every 30 s. Validated: daemon started on battery, cable plugged back in → immediate `power source changed from=battery to=ac`, `Allowed` recalculated. AC → battery also validated (no fallback: `balanced` is valid on both sources).
- [x] **Nitro key via evdev** (2026-10-12): `internal/hw/nitrokey.go` reads `KEY_PROG1` from the internal keyboard without a grab and reopens it if the device disappears; action set by the `--nitro-key` option (`cycle`, the default without a GUI; `none` once the GUI/tray takes over the key, since in the original app it opens the app). Validated: 3 presses → `balanced-performance`, `quiet`, `balanced`.
- [x] **Daemon installed on the host and validated on the system bus** (2026-10-12): `install-daemon.sh` (build in the container, install on the host; polkit restarted to read `/usr/local/share/polkit-1/actions`; `DeviceAllow` for NVIDIA), polkit `control` without a password in the active session, round trip with tuned-ppd without a loop, `pwm=0` → floor of ~1 955 RPM. **Finding:** concurrent WMI calls to different methods corrupt responses (§9 of firmware-wmi.md) → mutex in the daemon, reduced polling rate (6 % → 1.7 % CPU), `poll()` on `platform_profile`, debounce of external changes. Upstream candidate (4): serialize WMI.
- [x] **Go daemon skeleton** (`daemon/`, 2026-10-12): module `acerctl/daemon` (godbus v5.2.3), real (`internal/hw/real.go`) and simulated (`sim.go`) backends, pure rules with tests (`internal/rules`), complete D-Bus service for API v1 (properties + `PropertiesChanged`, methods with polkit, errors, Monitor with unicast `Subscribe`, PPD sync, state in JSON), unit + D-Bus policy + polkit + `packaging/install-daemon.sh`. Validated on the session bus: simulated (the whole API, errors `NotAllowed`/`NotSupported`/`InvalidArgument`) and real hardware in `--dry-run` (ANV15-52 capabilities detected from the firmware, fans, battery, monitor). **Not yet installed on the host.**
- [x] **GPU via NVML installed and validated** (2026-10-12): `internal/hw/gpu_nvml.go` (go-nvml, `libnvidia-ml.so` at runtime; `nvidia-smi` only as a fallback). In the unit: `NVML ready gpu="NVIDIA GeForce RTX 4060 Laptop GPU"`, `gpu.*` present from the 1st sample and matching `nvidia-smi` (P8 idle: 210/405 MHz, ~1.6 W). **Gain:** the previous build, which queried through `nvidia-smi`, kept the GPU awake (2010/7001 MHz, ~10 W); with NVML the GPU goes back to `runtime_status=suspended` between samples. The unit's sandbox does not get in the way.
- [x] **Daemon CPU cost** (2026-10-12): per-attribute measurements on the ANV15-52 — `pwmN_enable` ~16 ms, `pwmN` ~7 ms, `platform_profile`/`kbd_*`/`usb_charging`/hwmon `acer_battery` ~5–15 ms (WMI/SMM); `fanN_input`, temps from hwmon `acer` and `power_supply` ≤ 0.2 ms. Changes: fans with a cheap `RPM()` every 5 s and mode/speed cached (reread after a write, on resume and every 60 s); Monitor samples only with RPM and `power_supply` (battery temperature at most every 30 s); **the dGPU is never woken up to be monitored** (`runtime_status` before the NVML query → `gpu.active=false`). Measured: idle **1.7 % → 0.5 %**; Monitor at 1 s **10.4 % → 1.0 %**, and the GPU stays in D3cold (before, each sample kept it on at 1–10 W; waking it cost ~2 s and 1.4 s of kernel CPU). With the GPU woken by another process, the Monitor shows the data and it goes back to sleep on its own ~20 s later.
- [x] **`acerctl` CLI** (2026-10-12): `daemon/cmd/acerctl` on top of the public package `daemon/client` (which the GUI will reuse; hence in the same Go module as the daemon and not in `cli/`). Commands: `status` (default), `caps`, `monitor`, `mode` (aliases `eco`/`turbo`), `fan`, `kbd timeout`, `battery limit|calibrate`, `usb-charging`, `scenario list|save|apply|delete` (`save --from-current`), `appsync`, `reload`, `version`; `--json` on every read. Installed by `install-daemon.sh` into `/usr/local/bin`. Tested on the host against the real daemon (reads, writes, daemon and usage errors). Also: `Health.full/design` fixed — the ANV15-52's BAT1 only has `charge_*` (µAh) and the daemon published µAh as µWh; it now converts using `voltage_min_design` (3.733 Ah × 15.4 V = 57.5 Wh).
- [x] **Scenarios + App Sync** (2026-10-12): `internal/scenarios` (format, validation, `scenarios.json`), `internal/appsync` (proc connector with a kernel filter, matching by `comm`/argv with Wine/Proton support, active seat0 user, tests) and `internal/service/scenarios.go` (Apply/Save/Delete/SetAppSync, `Active` cleared by a manual change, snapshot + revert). Validated on the ANV15-52: scenario with `balanced-performance` + CPU fan max tied to a test executable → applied on start, reverted on exit; with a manual change in between, it does not revert; App Sync resumed after a restart. Cost ~0.1–0.2 ms per `exec` (it was 0.4 ms before the kernel filter, raw `comm` reading and processing without a goroutine switch). Semantics in api-dbus.md §9. **Still to do:** test with a real Proton game.
- [x] **Suspend/resume** (2026-10-12): `internal/service/sleep.go` listens to logind's `PrepareForSleep`; reapplies profile and fans on resume and restores fans at startup (manual → auto). Findings on the ANV15-52: the firmware resets the profile to `balanced` on resume (the daemon ignores that write and tuned's until it finishes), the fans (max/manual + PWM) survive S3. Validated: `quiet` + max and `quiet` + manual 40 % → resume restores `quiet`; restart with manual → `auto` (api-dbus.md §11).
- [x] **`[N]` mark and icon** (2026-10-13): `assets/logo/` holds only the ones in use: `n-icon-cut.svg` (app icon: dark rounded square with a #FFA000 base showing at the bottom, a 3D effect in the style of GNOME icons, `[N]` in bas-relief) and `n-mark-cut.svg` (the same `[N]` without the background: N in `currentColor`, brackets in `var(--accent, #ff1b19)`). Source Code Pro Black letters (SIL OFL 1.1) as outlines. The discarded variants (including the gradient octagon, too similar to the Acer icon) went to `../referencias/logo/`, outside the repository.
- [x] **GUI `nitro-control-center` v1** (2026-10-13): `gui/` in Go + Wails v2 (WebKitGTK 4.1, tag `webkit2_41`), framework-less TS frontend, own CSS and icons, system font (`system-ui`), `[N]` mark in the sidebar and `n-icon-cut.svg` as the app icon, no 3D/Lottie. Screens: Dashboard (firmware modes with `Allowed`/`Extras`, temperature and RPM gauges, CPU/GPU/RAM/network), Monitor (uPlot, 2/5/10 min), Fans (auto/max/manual per fan and for all; manual keeps the current PWM), Lighting (state and timeout), Battery (charge, health, 80 % limit, calibration), Scenarios + App Sync (editor, save current state, rename), Settings (USB charging, °C/°F, pt/en language, start with the session via XDG autostart, about/modules, `Reload`). Bridge `gui/app.go`: `State()` with the properties of every interface, `state` event on each `PropertiesChanged` (batched over 80 ms), automatic reconnection when the daemon restarts ("no connection" screen), Monitor only while the window is visible on a live screen, daemon errors as translated `{kind,msg}`. Single instance (opening it again brings the window forward). `daemon/client.Plain` is now shared by the CLI and the GUI. Tested in the `wails dev` browser against the simulated daemon (every screen, errors `NotSupported`/`NotAllowed`/`InvalidArgument`, daemon going down and coming back) and opened on the host against the real `acerd` (connects and subscribes to the Monitor). Packaging: `packaging/install-gui.sh` + `io.github.fnndk.NitroControlCenter.desktop`.
- [x] **GUI: integrated window** (2026-10-13): no system frame (`Frameless` + transparent background); own themed bar (`gui/titlebar.go`, `frontend/src/titlebar.ts`) with the button order from GNOME's `button-layout` (settings portal, `gsettings` fallback, updates on change) and the icon theme's `window-*-symbolic` icons read from the system at runtime; 12 px corners and a shadow in an 18 px transparent margin (`winMargin`/`--win-margin`, the resize area covers the shadow), all gone when maximized. Limitation: GNOME sees the margin as part of the window (side-by-side tiling leaves a gap). **Per desktop** (2026-10-13): on KDE/LXQt (`XDG_CURRENT_DESKTOP`) the window keeps the compositor's frame (`Frameless` off, opaque background, no margin or own buttons; `ssd` class in the frontend), overridable with `ACERCTL_DECORATIONS=client|server`. Builds; **still to test in a Plasma session**. `[N]` mark with the font's brackets, as in the icon, without the background: `assets/logo/n-mark-cut.svg` (sidebar and connection screen).
- [x] **Tray and configurable Nitro key** (2026-10-13): session agent `acer-tray` (`daemon/cmd/acer-tray`, `fyne.io/systray`, pure Go over D-Bus) with status, open the app, modes (current one checked, disallowed ones disabled, extras flagged), fans at max, scenarios submenu + App Sync and "hide until next login"; daemon refusals become notifications. Daemon: `System.NitroKey`/`SetNitroKey` (`app` | `cycle`, persisted) and the `NitroKeyPressed` signal; in `app` the agent opens the app. GUI settings: "Tray icon" (`~/.config/acerctl/settings.json`, package `daemon/usercfg`) and "Nitro key" (open the app / cycle the modes). The agent starts at login (`/etc/xdg/autostart`) and exits when it has nothing to do; the GUI reloads or starts it. Tested with the simulated daemon on GNOME (AppIndicator): menu read and clicked through `com.canonical.dbusmenu`, lifecycle without an icon, signal accepted only from the name owner. Icon (2026-10-13): white `[N]` on a rounded black square (readable on light and dark panels); a symbolic icon in the theme color would require `IconName`, which `fyne.io/systray` does not expose. **Still to do:** validate a real key press on the host.
- [x] **Daemon fix** (2026-10-13): `conn.Send` returns `*dbus.Call`; the Monitor treated the return value as an error and logged `signal send failed` on every sample for every subscriber. It now checks `call.Err`. **Reinstall the daemon** to get rid of the journal warning.
- [x] **Repository in English** (2026-10-13): README, STATUS, `docs/` and the per-directory READMEs translated (mode and screen names match the GUI's English strings); code, scripts and packaging were already English. Portuguese only in the GUI (`i18n.ts`), the tray's text table and `[pt_BR]` entries in desktop/polkit files. Commit messages before `9106bdd` stay in Portuguese (no history rewrite).
- [x] **Persistence validated across a reboot** (2026-10-12): `packaging/install-modules.sh` installs the signed `.ko` files into `/var/lib/acer-wmi-modules/` (label `modules_object_t`) and enables `acer-wmi-modules.service`; `packaging/check.sh` checks the whole layer after boot.

## Immediate next step

**Rename to acerctl (2026-10-13, not yet installed):** `nitrod`/`nitroctl`/`nitro-tray` → `acerd`/`acerctl`/`acer-tray`, D-Bus `io.github.fnndk.Acerd1`, polkit `io.github.fnndk.acerd.*`, state `/var/lib/acerd`, user settings `~/.config/acerctl`, env `ACERCTL_*`, Go modules `acerctl/{daemon,gui}`, repository directory `acerctl`. `install-daemon.sh` removes the old `nitrod` install and moves `/var/lib/nitrod`; `install-gui.sh` removes `nitro-tray` and moves `~/.config/nitroctl`. The GitHub repository still has to be renamed (FnnDK/nitroctl → FnnDK/acerctl) and `origin` updated. API names that refer to the physical key (`NitroKey`, `--nitro-key`) were kept.

**Aspire AG15-71P:** `drivers/acer-wmi-lab/survey.sh` (read-only) to run on that machine; then compare its firmware with §2–§7 and plan the `acer-control-center` flavor.

0. **Install the GUI and the fixed daemon** (still pending as of 2026-10-13): in the container `sh packaging/install-daemon.sh build && sh packaging/install-gui.sh build`; on the host `distrobox-host-exec sudo sh packaging/install-daemon.sh install && distrobox-host-exec sudo sh packaging/install-gui.sh`. Open "Nitro Control Center" from the menu and check the writes on the real hardware (mode, fans, timeout, limit, USB, scenarios); polkit `settings` should ask for a password.
1. **GUI, what is left:** validate on the host the tray and the Nitro key set to "open the app" (reinstall daemon and GUI); monitor widget; Flatpak.
2. **Finish the daemon:** App Sync with a real Proton game.
3. Kernel layer maintenance: after a kernel update, `distrobox-host-exec sudo sh packaging/build-sign.sh`; `sh packaging/check.sh` to verify. Later, akmods/DKMS.

## Backlog (suggested order)

1. **Upstream `acer_wmi` patches** (§8): DMI entry `Nitro ANV15-52` (`predator_v4` + `pwm`); `platform_profile_probe` reading mask 0x0A; battery hook `charge_control_end_threshold`; **serialization of WMI calls** (corrupted responses under concurrency, §9).
2. **Persistence/packaging** of the modules (akmods or DKMS + systemd unit), a single build+sign script.
3. **`acerd` daemon (Go)** — `daemon/` (skeleton done 2026-10-12; the items below are the full target)
   - D-Bus system bus `io.github.fnndk.Acerd1` (godbus), polkit for privileged actions, systemd unit `acerd.service`.
   - Backends: `platform-profile` (mode), hwmon `acer` (RPM, temps, `pwm*`), `acer-wmi-extras` (timeout, USB, supported profiles, battery hwmon), `acer-wmi-battery` (80 % limit, calibration), NVML (`nvidia-smi`/go-nvml: temp, clocks, usage, power), `/proc` + `/sys` (CPU, RAM, network), UPower/udev (AC, battery), evdev (Nitro key `KEY_PROG1`).
   - API mirroring the original app: `GetCapabilities`, `Get/SetOperatingMode`, `Get/SetFanMode` (auto/max/manual per fan), `GetFanSpeeds`, `GetTemperatures`, `Get/SetBacklightTimeout`, `Get/SetUsbCharging`, `Get/SetChargeLimit`, `StartCalibration`, `GetMonitor` (stream via signal), profiles/scenarios (`List/Apply/Save/Delete`), App Sync (process watcher), change signals.
   - Rules: Eco only on battery; Turbo/OC only on AC and ≥ 40 %; restore the last state at boot and on resume; mode cycling by the Nitro key with the profile list coming from the firmware.
   - `acerctl` CLI on top of the same API.
4. **GUI `nitro-control-center` (Go + Wails v2)** — `gui/` (v1 done 2026-10-13; tray, Nitro key, widget, Flatpak still missing)
   - Web frontend (TS + CSS; charts with uPlot; Lottie animations; optional three.js), Go → D-Bus bindings.
   - 1:1 screens: Home (mode + monitor), Fan Control (auto/max/manual, no curve where the firmware has none), Lighting (timeout; zones/effects only on models with RGB), Battery (80 % limit, calibration, health), Settings (USB charging, units, startup), Scenario Manager + App Sync, monitor widget, tray (StatusNotifierItem) with quick modes.
   - The GUI hides whatever `GetCapabilities` does not list (e.g. Turbo on the ANV15-52).
   - **Own artwork and texts:** no brand fonts (use a free font, e.g. Noto/Inter), icons, backgrounds, Lottie animations, 3D models, sounds, videos or interface texts from the original app; app icon without the gradient octagon. Material Icons (Apache-2.0) and Noto (OFL) are free. The names "Nitro", "NitroSense", "Predator", "PredatorSense" are Acer trademarks: decide the public name before publishing.
   - Packaging: Flatpak (talks D-Bus to the host daemon); RPM/akmods for modules + daemon.
5. **Extras for other models** (auto-detected from the firmware): 4-zone/per-key RGB, LEDs, boot sound, LCD overdrive, fan table, CPU/GPU OC (NVML), CPU power limit (RAPL).
6. Out of scope: Acer online services, BIOS/driver updates, boot logo, DTS, MagKey, mobile app.

### Suggested repository skeleton

```
drivers/            C modules (extras, battery, lab)
daemon/             Go: cmd/{acerd,acerctl}, client/, internal/{hw,rules,service,monitor,ppd,state,scenarios,appsync}
gui/                Go + Wails: app.go, frontend/ (TS, CSS, own assets)
packaging/          systemd (modules, daemon), polkit, dbus, install-daemon.sh; akmods/dkms, flatpak to do
docs/               firmware-wmi.md, api-dbus.md
```

## How to resume (commands)

```bash
# build a module on the host (e.g. extras)
distrobox-host-exec make -C /var/home/klumpp/work/nitro-sense/acerctl/drivers/acer-wmi-extras
# sign + load (acer-wmi-battery, acer-wmi-lab or acer-wmi-extras)
distrobox-host-exec sudo sh -c 'K=/usr/src/kernels/$(uname -r); M=/var/home/klumpp/work/nitro-sense/acerctl/drivers/acer-wmi-extras/acer-wmi-extras.ko; "$K/scripts/sign-file" sha256 /etc/pki/akmods/private/private_key.priv /etc/pki/akmods/certs/public_key.der "$M" && insmod "$M"'
# arbitrary WMI calls (research), with acer-wmi-lab loaded:
#   echo "<GUID> <method> <hex bytes>" | sudo tee /sys/devices/platform/acer-wmi-lab/call; cat /sys/devices/platform/acer-wmi-lab/call
# ready-made batches: drivers/acer-wmi-lab/probe.sh, probe2.sh, probe3.sh; key: keywatch.py
# daemon without root (session bus): simulated or real hardware read-only
(cd daemon && go test ./... && go build ./cmd/acerd && ./acerd --session --simulate ANV15-52 --state /tmp/ns.json)
(cd daemon && ./acerd --session --dry-run --state /tmp/ns.json)
busctl --user introspect io.github.fnndk.Acerd1 /io/github/fnndk/Acerd1
# install: sh packaging/install-daemon.sh build  (container)  →  distrobox-host-exec sudo sh packaging/install-daemon.sh  (host)
```

## File map

- `README.md` — overview and build/signing instructions.
- `docs/firmware-wmi.md` — technical reference (WMI classes, formats, EC, capabilities, candidate patches).
- `drivers/acer-wmi-battery/` — vendored (see `UPSTREAM.md`), signed `.ko`.
- `drivers/acer-wmi-lab/` — research module + `probe*.sh`, `keywatch.py`, results `probe*-<date>.txt`; `survey.sh` = read-only survey of another Acer model (DMI, WMI/BMOF, ACPI tables, existing drivers, probes if the lab module is loaded).
- `drivers/acer-wmi-extras/` — companion v1 + `test.sh`.
- `packaging/` — modules: `acer-wmi-modules.service`, `install-modules.sh`, `build-sign.sh`, `check.sh`; daemon: `acerd.service`, `io.github.fnndk.Acerd1.conf` (D-Bus), `io.github.fnndk.acerd.policy` (polkit), `install-daemon.sh`.
- `daemon/` — `acerd` in Go (see `daemon/README.md`).
- `docs/api-dbus.md` — the daemon's D-Bus API (v1 draft, revised 2026-10-12).
- `gui/` — `nitro-control-center` (see `gui/README.md`); `packaging/install-gui.sh`, `packaging/io.github.fnndk.NitroControlCenter.desktop`.

## Agreed rules

- Do not replace `acer_wmi`; do not create brand flags; capabilities from the firmware.
- English only in the repository (code, docs, scripts, commit messages); Portuguese only in the GUI/tray translations and `[pt_BR]` entries.
- Companion name: **`acer-wmi-extras`** (applies from Aspire to Predator).
- Languages: C (kernel), Go (daemon, CLI), Go + Wails (GUI). GUI with own artwork.
- Only "Get" methods in exploratory probes; "Set" only with a validated, reversible format.
- Names: `acerctl` (project/CLI), `acerd` (daemon), `nitro-control-center` (GUI), namespace `io.github.fnndk`.
- Features beyond the original app (e.g. Performance on battery) are allowed, but always **flagged as extra** in the API (`Mode.Extras`) and in the GUI.
- Before persisting options that create `platform_profile`, check the `tuned` profile.
