# D-Bus API of the `acerd` daemon (v1 draft)

Revised on 2026-10-12 from v0, against `docs/firmware-wmi.md` (§1, §4–§8) and the real attributes of `acer_wmi`, `acer-wmi-extras` and `acer-wmi-battery` on the ANV15-52. Changes from v0 and open points at the end of the file.

## 1. Overview

- **Bus:** system bus. Name `io.github.fnndk.Acerd1`, object `/io/github/fnndk/Acerd1`, a single object with several interfaces.
- **Reads are D-Bus properties** (`org.freedesktop.DBus.Properties`, with `PropertiesChanged`), so that `busctl`, GUI, CLI and tray use the standard mechanism with no extra code. **Writes are methods**, because they need to validate, apply rules and return an error with a reason.
- **Capabilities come from the firmware** (§3 below). Every interface exposes only what `Capabilities` lists; the client never assumes a feature from the brand or model. A call to something not listed returns `NotSupported`.
- **Authorization** through polkit (§10). The daemon runs as root (it needs to write to sysfs and read evdev/RAPL); it never executes anything coming from the client beyond the validated values.
- All numeric values use natural units in simple types: temperature `d` (°C), frequency `u` (MHz), power `d` (W), voltage `u` (mV), current `i` (mA), RPM `u`, percentages `u` (0–100), bytes `t`.

### Errors

| Name | When |
|---|---|
| `io.github.fnndk.Acerd1.Error.NotSupported` | feature missing from `Capabilities` on this hardware |
| `io.github.fnndk.Acerd1.Error.NotAllowed` | daemon rule (e.g. `performance` on battery); the message carries the reason as text for the GUI to show |
| `io.github.fnndk.Acerd1.Error.InvalidArgument` | value outside the accepted set (e.g. USB 15 %) |
| `io.github.fnndk.Acerd1.Error.Hardware` | sysfs/WMI returned an error; message with the `errno` |
| `org.freedesktop.DBus.Error.AccessDenied` | polkit denied |

### Source of each piece of data (host)

| Data | Source | Notes |
|---|---|---|
| Mode (thermal profile) | `/sys/class/platform-profile/platform-profile-0/profile` (`acer_wmi predator_v4`) | external changes detected by `poll()` on the attribute (`sysfs_notify`) |
| Supported profiles | `/sys/devices/platform/acer-wmi-extras/supported_profiles` | the `platform-profile` `choices` lists the driver's 5 fixed ones and is **not** usable (§8 of firmware-wmi) |
| RPM, CPU/GPU/external temperatures | hwmon `acer` (`fan1`, `fan2`, `temp1..3`) | GPU reads 0 while the dGPU is powered off |
| Fan mode and speed | hwmon `acer` `pwmN_enable` (0 max, 1 manual, 2 auto) and `pwmN` (0–255) | only exist with the `.pwm` quirk or `force_caps` |
| Backlight timeout and state | `acer-wmi-extras/kbd_backlight_timeout` (RW, only 0 or 30), `kbd_backlight_state` (RO) | on/off is via the Fn key (`acer_wmi`); there is no state Set |
| USB charging | `acer-wmi-extras/usb_charging` (0/10/20/30) | — |
| 80 % limit, calibration | `/sys/bus/wmi/drivers/acer-wmi-battery/{health_mode,calibration_mode}` | driver attributes, not device ones; if `BAT1/charge_control_end_threshold` ever exists (upstream patch), prefer it |
| Battery temperature, voltage, current | hwmon `acer_battery` (`temp1`, `in0`, `curr1`) | — |
| Charge, status, cycles, capacity | `/sys/class/power_supply/BAT1` (`capacity`, `status`, `cycle_count`, `energy_full`, `energy_full_design`) | source of `health` |
| AC | `/sys/class/power_supply/ACAD/online` + udev (`power_supply` uevent) | without depending on UPower |
| GPU (temp, usage, clocks, power) | NVML (go-nvml); fallback `nvidia-smi --query-gpu` | missing ⇒ `gpu.nvml=false` and fields omitted |
| CPU (usage, freq, power) | `/proc/stat`, `/sys/devices/system/cpu/*/cpufreq/scaling_cur_freq`, RAPL `/sys/class/powercap/intel-rapl:0/energy_uj` (root only) | — |
| RAM, network | `/proc/meminfo`, `/proc/net/dev` (sum of the physical interfaces) | — |
| Sense key | evdev `KEY_PROG1` (148) on the internal keyboard (`AT Translated Set 2 keyboard`), present when scancode `0xF5` maps to it (systemd hwdb, or `packaging/61-acerctl-keyboard.hwdb` for models not listed there yet) | no `EVIOCGRAB`: the desktop still receives the key |

## 2. Interface `io.github.fnndk.Acerd1.System`

| Member | Type | Description |
|---|---|---|
| prop `Version` | `s` | daemon version |
| prop `Capabilities` | `a{sv}` | see §3; changes only on a module reload (`PropertiesChanged` signal) |
| prop `Model` | `a{sv}` | `vendor`, `product`, `bios` (DMI), `kernel`, `modules: a{ss}` (name → version/“absent”), `acer_wmi_caps: u` (effective `force_caps`, if readable) |
| prop `SenseKey` | `s` | Sense key action: `app` (the session agent opens the app, as in the original) or `cycle` (the daemon cycles the profile); persisted in `state.json`, default from the `--sense-key` option (a `nitro_key` saved before the rename is moved over) |
| method `SetSenseKey(s action)` | | `app` or `cycle`; `NotSupported` without `capabilities.sense_key`, and for `cycle` on models without thermal profiles (there the effective action is always `app`); polkit `control` |
| signal `SenseKeyPressed` | `s` | on every press, with the action in effect; the session agent (`acer-tray`) opens the app when it is `app` and only accepts the signal from the owner of the name `io.github.fnndk.Acerd1` |
| method `Reload()` | | reprobe of modules and capabilities (after `insmod`/update); polkit `control` |
| prop `Setup` | `aa{sv}` | system changes still to make, read from the system on each start, every 30 s, after a resume, on `Reload` and `ApplySetup` (DKMS may build the modules while `acerd` runs): `id: s` (`acer_wmi`: the firmware reports thermal profiles through `acer-wmi-extras` but `acer_wmi` exposes none; `signing_key`: Secure Boot refuses the modules and the key they are signed with is not enrolled (akmods key on Fedora; with the modules under `updates/dkms`, DKMS's `/var/lib/dkms/mok.pub` or Ubuntu's `/var/lib/shim-signed/mok/MOK.der`); `modules`: `acer-wmi-extras` not loaded for another reason), `state: s` (`needed`: `ApplySetup` does it; `reboot`: done, takes effect at the next boot; `manual`: the user has to act, `detail` says why: `oldkernel` (this kernel's `acer_wmi` has no `predator_v4`), `missing`, `noheaders` (`acerctl-dkms` installed, no headers for the running kernel, so DKMS skipped the build), `notbuilt` (`acerctl-dkms` installed with headers, but no module for the running kernel: the build failed), `notloaded`, `nokey`, `nomokutil`), `detail: s` (for `acer_wmi`, the options it writes: `predator_v4=1`, plus `force_caps` on models where a value was validated). Empty when nothing is left |
| method `ApplySetup(as ids)` | `→ s` | applies items in the `needed` state (`signing_key` also in `reboot`, replacing the pending request): `acer_wmi` writes `/etc/modprobe.d/acerctl.conf`, `signing_key` runs `mokutil --import` on that existing key (none is ever created) with a fresh two-digit code and returns it (the firmware asks for it at the next boot; `""` otherwise). Nothing is loaded or reloaded: both take effect after a restart. polkit `setup` (password every time) |

## 3. Capabilities (`System.Capabilities`)

Each key appears only when the backend was detected. Detection is by **presence and a successful read** of the file, never by DMI.

| Key | Type | Detection | ANV15-52 |
|---|---|---|---|
| `profiles` | `as` | `acer-wmi-extras/supported_profiles`; if the module is missing, `platform-profile/choices` **with** `profiles_from_firmware=false` | `low-power quiet balanced balanced-performance` |
| `profiles_from_firmware` | `b` | see above | `true` |
| `fans` | `a{sv}` | hwmon `acer` with `pwmN_enable`; `ids: as` (`cpu`, `gpu`), `modes: as` (`auto max manual`), `range: a{s(uu)}` (min/max in % per fan; 0–100 when there is no per-model table), `curve: b` (always `false` until there is a module for `GetGamingFanTable`) | `cpu gpu`, `0–100`, no curve |
| `kbd_backlight` | `a{sv}` | `acer-wmi-extras/kbd_backlight_timeout`; `timeouts: au` (`0 30`), `rgb: b` and `zones: u` (copied from `lighting.keyboard` when there is one) | timeout, `rgb=false` |
| `lighting` | `a{sa{sv}}` | RGB lighting devices (`keyboard`, `lightbar`, `logo`), each with `effects: as` (display order), `with_color`/`with_speed`/`with_direction: as` (effects that use each field), `zones: u` (colours `static` takes; 0 = one), `speed: (uu)` (firmware steps), `directions: as`, `readback: b`. Today only the simulated Predators have it (§7, firmware-wmi §11) | absent |
| `usb_charging` | `au` | `acer-wmi-extras/usb_charging`; accepted values | `0 10 20 30` |
| `battery` | `a{sv}` | `charge_limit: b` (`health_mode` ≥ 0), `calibration: b` (`calibration_mode` ≥ 0), `sensors: b` (hwmon `acer_battery`) | all `true` |
| `gpu` | `a{sv}` | `nvml: b` (NVML initialized), `oc: b` (always `false` in v1) | `nvml=true` |
| `cpu_power_limit` | `b` | writable RAPL; `false` in v1 | `false` |
| `sense_key` | `b` | the internal keyboard maps scancode `0xF5` to `KEY_PROG1` | `true` |

No Turbo (`performance`) in `profiles` ⇒ the GUI does not even show the button. This holds for any model: the list is the firmware's.

**Convention for extra features.** Everything the daemon allows beyond the original app's behavior (today only Performance on battery) is exposed separately (`Mode.Extras`) so the GUI can mark it as extra, never silently mixed with the 1:1 behavior.

## 4. Interface `io.github.fnndk.Acerd1.Mode`

Names are the `platform_profile` ones (what the daemon writes). Acer mapping: `quiet`=Quiet, `balanced`=Balanced, `balanced-performance`=Performance, `performance`=Turbo, `low-power`=Eco.

| Member | Type | Description |
|---|---|---|
| prop `Current` | `s` | current profile; also reflects external changes (tuned, another client) |
| prop `PowerSource` | `s` | `ac` / `battery` |
| prop `BatteryPercent` | `u` | — |
| prop `Allowed` | `as` | profiles allowed **now** (capabilities ∩ power rules); this is what the GUI enables |
| prop `Extras` | `as` | items of `Allowed` that exist only because of an extended daemon rule (today: `balanced-performance` on battery); the GUI flags them as an extra feature |
| method `Set(s profile)` | | `NotSupported` if outside `profiles`; `NotAllowed` if outside `Allowed` |
| method `Cycle()` | `→ s` | next one in `Allowed`, in `profiles` order; used by the Sense key |

**Power rules** (from the daemon, applied in `Set`, `Cycle` and on every AC↔battery switch):

- On AC: `quiet`, `balanced`, `balanced-performance`; `performance` (Turbo) only with charge ≥ 40 %.
- On battery: `balanced` and `low-power`; **extra (beyond the original app):** `balanced-performance` too, with charge ≥ 40 %. When it drops below 40 % the daemon goes back to `balanced` and emits `PropertiesChanged`.
- `low-power` never on AC.
- The daemon keeps the last chosen profile **per source** (`mode_ac`, `mode_battery`) and reapplies it on a switch. If the current profile becomes forbidden (e.g. Turbo and the cable is unplugged), it switches to the one saved for the other source, or `balanced`.

## 5. Interface `io.github.fnndk.Acerd1.Fans`

| Member | Type | Description |
|---|---|---|
| prop `State` | `a{sa{sv}}` | per id (`cpu`, `gpu`): `rpm: u`, `mode: s` (`auto`/`max`/`manual`), `speed: u` (%; only meaningful in `manual`) |
| method `SetMode(s fan, s mode)` | | `fan` = id or `all`. `max` ⇒ `pwmN_enable=0`; `auto` ⇒ 2; `manual` ⇒ 1 keeping the current `pwmN` |
| method `SetSpeed(s fan, u percent)` | | writes `pwmN = percent·255/100`; switches to `manual` if needed. No fixed floor in the daemon: accepts the range from `capabilities.fans.range` (0–100 by default) and the firmware enforces the fan's real minimum; outside the range ⇒ `InvalidArgument` |

`rpm` is reread every 5 s (EC read, ~0.2 ms). `mode`/`speed` cost WMI calls (`pwmN_enable` ~16 ms, `pwmN` ~7 ms per fan on the ANV15-52), so they are reread only after each write, on resume and every 60 s (to catch another process writing to hwmon). Monitor samples read only the RPM.

## 6. Interface `io.github.fnndk.Acerd1.Monitor`

| Member | Type | Description |
|---|---|---|
| method `Get()` | `→ a{sv}` | one sample, format below |
| method `Subscribe(u interval_ms)` | | registers the caller (sender's unique name); minimum 500 ms |
| method `Unsubscribe()` | | — |
| signal `Sample` | `a{sv}` | emitted **to each subscriber** (unicast signal, `destination` = sender) at the requested interval; the daemon samples at the shortest active interval. Subscribers that leave the bus (`NameOwnerChanged`) are removed |

Sample format (keys missing when the backend does not exist):

```
cpu:     {temp: d, usage: u, freq: u, power: d}
gpu:     {active: b, temp: d, usage: u, clock: u, mem_clock: u, power: d}  # only active=false while the dGPU is suspended
fans:    {cpu: u, gpu: u}                     # RPM, same read as Fans.State
ram:     {used: t, total: t}
net:     {rx: t, tx: t}                       # bytes/s since the previous sample
battery: {percent: u, status: s, voltage: u, current: i, temp: d}
ts:      t                                    # CLOCK_MONOTONIC in ms
```

With no subscribers the daemon does **not** sample (NVML and RAPL cost CPU).

**The dGPU is never woken up to be monitored.** With runtime PM (Optimus), any NVML/`nvidia-smi` query takes the GPU out of D3cold: on the ANV15-52 this takes ~2 s, spends ~1.4 s of kernel CPU in the driver and leaves the card on (1–10 W) for as long as the queries continue. The daemon reads `power/runtime_status` of the NVIDIA PCI device before each sample; outside `active`, the sample carries only `gpu: {active: false}`. With the GPU already woken by another process, it queries normally. Fans and battery in the samples: only RPM (EC) and `power_supply`; the battery temperature (WMI) is reread at most every 30 s.

## 7. Interface `io.github.fnndk.Acerd1.Lighting`

| Member | Type | Description |
|---|---|---|
| prop `Keyboard` | `a{sv}` | single-colour backlight: `on: b` (RO, firmware state), `timeout_s: u` |
| method `SetKeyboardTimeout(u seconds)` | | only values from `capabilities.kbd_backlight.timeouts` (today 0 or 30); others ⇒ `InvalidArgument` |
| prop `Devices` | `a{sa{sv}}` | RGB devices of `capabilities.lighting` whose state is known: `effect: s`, `brightness: u` (%), `speed: u` and `direction: s` (when the effect uses them), `colors: as` (`#rrggbb`; one per zone for `static` on a zoned device), `readback: b` (false: this is what the daemon last applied, the device cannot be read). A device that cannot be read and was never set is missing |
| method `SetLight(s device, a{sv} fields)` | | changes only the given fields (`effect`, `speed`, `brightness`, `direction`, `colors`); the rest is kept and fitted to the effect: one colour is spread over the zones of a zoned `static`, other effects keep the first colour, unused fields are dropped. Unknown device/field or a value outside the capabilities ⇒ `InvalidArgument`; polkit `control` |

`on` changes via the Fn key; the daemon rereads `kbd_backlight_state` every 2 s while the GUI is on the lighting screen (the client calls the Monitor's `Subscribe`) and emits `PropertiesChanged`. `Devices` is reread with the slow refresh (30 s) and after each `SetLight`. Every `SetLight` is remembered (`state.json`, `lights`) and reapplied at startup and after a resume, skipping readable devices that already show it: some firmware forgets the lighting (firmware-wmi §11).

## 8. Interface `io.github.fnndk.Acerd1.Battery`

| Member | Type | Description |
|---|---|---|
| prop `ChargeLimit` | `b` | 80 % limit (`health_mode`); the state lives in the EC and **survives a reboot** — the daemon reads it, does not restore it |
| prop `Calibrating` | `b` | `calibration_mode` |
| prop `UsbCharging` | `u` | 0 (off) / 10 / 20 / 30 |
| prop `Health` | `a{sv}` | `cycles: u`, `full: t`, `design: t` (µWh), `health_pct: u` (= full/design), `temp: d` |
| method `SetChargeLimit(b)` | with the `acer-wmi-battery` `power_supply` extension, through UPower's `EnableChargeThreshold` (keeps GNOME in agreement); otherwise, or if UPower fails, writes `health_mode` | polkit `settings` |
| method `StartCalibration()` | | `NotAllowed` without AC; polkit `settings`. There is no end event in the legacy API (`acer_wmi` holds the handler): the daemon **polls** `calibration_mode` every 30 s (the module rereads the firmware on each read) and emits `PropertiesChanged` when it ends. While `Calibrating`, `acerd` holds a logind `block` inhibitor on `sleep:handle-lid-switch`: a suspend stalls the discharge and a shutdown in the middle leaves the EC reporting no modes at the next boot |
| method `StopCalibration()` | | — |
| method `SetUsbCharging(u percent)` | | only values from `capabilities.usb_charging`; polkit `settings` |

## 9. Interface `io.github.fnndk.Acerd1.Scenarios`

Renamed from `Profiles` to avoid clashing with the thermal profiles (`platform_profile`).

| Member | Type | Description |
|---|---|---|
| prop `List` | `aa{sv}` | saved scenarios |
| prop `Active` | `s` | name or `""` |
| prop `AppSync` | `b` | watcher on |
| method `Apply(s name)` | | applies each field through the methods above (same rules and errors) |
| method `Save(a{sv})` | | creates/replaces by `name` |
| method `Delete(s name)` | | — |
| method `SetAppSync(b)` | | — |

Scenario format (all keys optional except `name`):

```
name: s
mode_ac: s            mode_battery: s
fans: a{sa{sv}}       # id → {mode, speed}
kbd_timeout_s: u
apps: as              # executable basenames (App Sync)
```

Stored in `/var/lib/acerd/scenarios.json` (system-wide, since the daemon is a system one). `Save` validates against the hardware (firmware profiles, fan ids and range, keyboard timeouts; unknown key ⇒ `InvalidArgument`) and normalizes `apps` to lowercase basenames (`C:\Games\Foo.exe` → `foo.exe`).

**Semantics (implemented 2026-10-12).**
- `Apply` writes the scenario's modes and fans into the saved state (§11), so source switches, resume and boot follow the scenario; the mode applied now is the one for the current source (with `Fallback` if it is not allowed). `Active` becomes the name.
- Any manual change to something a scenario controls (`Mode.Set/Cycle`, Sense key, external PPD/sysfs, `Fans.*`, `SetKeyboardTimeout`) clears `Active`. So does `Delete` of the active scenario.
- **App Sync** (`SetAppSync`, persisted; resumed at startup): proc connector (`NETLINK_CONNECTOR`, `CAP_NET_ADMIN`; on kernel ≥ 6.6 with an `exec`/`exit`-only filter). On each `exec` it reads `/proc/<pid>/comm`; `cmdline` only when `comm` is truncated (15 characters) or is Wine/Proton (`argv[0]` and, after a Wine loader, `argv[1]`). Wine processes that did not match are reread after 2 s (Wine changes `comm`/argv after the `exec`). Case-insensitive matching, optional `.exe`, truncated `comm` matched by prefix. Only processes of seat0's active user (`ACTIVE_UID` in `/run/systemd/seats/seat0`). `/proc/<pid>/exe` is not used: it would require `CAP_SYS_PTRACE`.
- The most recently started app decides the scenario. On the first one, the daemon saves a snapshot of the state (modes, fans, timeout, `Active`) and applies the scenario; when the last one exits, it applies the snapshot back — **unless the user changed something in the meantime** (`Active` ≠ App Sync scenario), in which case it just discards the snapshot. Turning App Sync off also reverts. The snapshot lives in `state.json`: if the daemon crashes with the game open, at startup it reverts when no matched app is running.
- Cost: ~0.1–0.2 ms of daemon CPU per `exec` on the system (measured with 2 000 `exec`s); nothing without `exec`s.

## 10. Authorization (polkit)

Three actions, in `/usr/share/polkit-1/actions/io.github.fnndk.acerd.policy`:

| Action | Covers | Default |
|---|---|---|
| `io.github.fnndk.acerd.control` | `Mode.Set/Cycle`, `Fans.*`, `Lighting.*`, `Scenarios.*`, `System.Reload` | `allow_active=yes`, `allow_inactive=no`, `allow_any=auth_admin` |
| `io.github.fnndk.acerd.setup` | `System.ApplySetup` | `auth_admin` everywhere (never kept) |
| `io.github.fnndk.acerd.settings` | `Battery.SetChargeLimit`, `Start/StopCalibration`, `SetUsbCharging` | `allow_active=auth_admin_keep`, others `auth_admin` |

Reason for the split: switching mode and fan is an everyday action (the Sense key does it without a password); charge limit, calibration and USB charging persist in the EC and affect the machine while it is off. Reads and `Monitor.Subscribe` do not go through polkit. D-Bus policy in `/usr/share/dbus-1/system.d/io.github.fnndk.Acerd1.conf`: only root owns the name; any user can send.

## 11. Daemon behavior

- **Boot/resume.** Restores only what does not live in the EC: thermal profile (per power source), fan mode and RGB lighting (§7). Timeout, USB charging and charge limit live in the EC and are only read. Fans in `manual` are **not** restored at boot (risk of a slow fan under load); they go back to `auto` and the daemon logs it. On resume, it reapplies the profile and rereads everything. State in `/var/lib/acerd/state.json`.
- **Start on demand (D-Bus activation).** `packaging/io.github.fnndk.Acerd1.service` (in `/usr/share/dbus-1/system-services`, or `/usr/local/share/...` for the script install) points the bus name at `acerd.service`; when the name has no owner, the client library (`daemon/client`, used by the app, the tray and the CLI) calls `StartServiceByName`, so the first use starts the daemon even if nobody enabled it. `ServiceUnknown` (no activation file) is reported as "acerd is not running". The packages also ship a systemd preset (`80-acerctl.preset`) that enables the unit at install, so the profile, fans and lighting are restored at boot before anyone logs in.
- **AC/battery.** Kernel uevents via netlink (`NETLINK_KOBJECT_UEVENT`, group 1, filter `SUBSYSTEM=power_supply`; no libudev) with a safety reread every 30 s; on each event it recalculates `Allowed`, reapplies the profile saved for the source and emits `PropertiesChanged` on `Mode`.
- **Sense key** (the NitroSense / PredatorSense / AcerSense button; renamed from "Nitro key" on 2026-10-13, since the key is not Nitro-only). In the original app the key **opens the app**; cycling modes belongs to the "Mode Switch Key" of other models. The daemon reads `KEY_PROG1` via evdev (internal keyboard, no grab, reopens if the device disappears), emits `System.SenseKeyPressed` and, in `cycle`, calls `Mode.Cycle()` without polkit. Opening an app is not a system service's job: in `app`, the session agent `acer-tray` opens it (XDG autostart; the GUI is single-instance, so opening it again only brings the window forward). The choice is `System.SetSenseKey` (GUI: Settings → Sense key); `--sense-key` only sets the default (`cycle`; `none` is a synonym for `app`).
- **Session agent (`acer-tray`).** Tray icon (StatusNotifierItem; on GNOME, the AppIndicator extension) with quick modes, fans at max, scenarios and App Sync, and the Sense key's "open the app". It starts at login and exits on its own when the icon is off (`~/.config/acerctl/settings.json`, `tray_hidden`) and the key is set to `cycle`; the GUI notifies it of changes (`io.github.fnndk.AcerTray.Reload` on the session bus) and starts it when it becomes needed.
- **Coexistence with `tuned-ppd`/`power-profiles-daemon` (decided 2026-10-12).** The daemon writes **directly** to `platform_profile` and keeps PPD in sync both ways. The full selection (5 modes) lives in our app; PPD has only 3 profiles, so the mapping is many-to-one going out and resolved by the power source coming back.

  Facts about tuned-ppd 2.28 (Fedora 44): PPD `power-saver` → tuned `powersave` → `platform_profile=low-power|quiet` (the first one present in `choices`); `balanced` → `balanced`; `performance` → `throughput-performance` → `performance` (= Turbo) plus governor etc. On battery, PPD `balanced` becomes `balanced-battery`, with no `[acpi]` section; when switching profiles tuned **restores** the value it read when applying the previous one. With `sysfs_acpi_monitor=true` it watches the file through inotify and, if another process writes `low-power`/`balanced`/`performance`, switches `ActiveProfile` to follow; it ignores `quiet` and `balanced-performance`.

  **Outgoing (our app → PPD).** After `Mode.Set`/`Cycle`, the daemon sets `net.hadess.PowerProfiles.ActiveProfile` to the closest one:

  | Chosen mode | PPD |
  |---|---|
  | `low-power`, `quiet` | `power-saver` |
  | `balanced` | `balanced` |
  | `balanced-performance`, `performance` | `performance` |

  Order: PPD first (tuned will write its own `platform_profile`), the daemon marks the transition as **expected** and ignores that write in `poll()`, waits for tuned to finish (`PropertiesChanged` on `ActiveProfile` or up to 2 s) and only then writes the real mode to sysfs. tuned-ppd ignores `quiet`/`balanced-performance` and, for the others, only confirms the `ActiveProfile` already set: no loop. If PPD already matches, it is left alone. With no PPD on the bus, syncing is skipped.

  **Incoming (GNOME/PPD → our app).** The daemon subscribes to `PropertiesChanged` on `ActiveProfile`; a change it did not initiate is the user's intent and becomes the best mode for the current source:

  | PPD | On AC | On battery |
  |---|---|---|
  | `power-saver` | `quiet` | `low-power` |
  | `balanced` | `balanced` | `balanced` |
  | `performance` | `performance` if in `Allowed` (Turbo, ≥ 40 %), otherwise `balanced-performance` | `balanced-performance` if ≥ 40 % (extra), otherwise `balanced` |

  The daemon lets tuned write, then writes the chosen mode and updates `mode_ac`/`mode_battery`. Direct writes to sysfs by third parties (outside PPD) follow the same rule: apply the power rules and sync PPD outgoing.

  **Power events and resume.** The daemon applies the profile saved for the source with a 1–2 s delay, after tuned has switched/restored its own, and redoes the outgoing sync. Unit: `After=tuned.service power-profiles-daemon.service`; the daemon never stops or reconfigures those services.

  **Caveat to validate:** when PPD `performance` is requested on a model without Turbo, tuned writes `performance` (5) to the firmware before the daemon corrects it; check whether the firmware rejects it (status 1/2, `acer_wmi` returns an error and tuned only logs) or accepts a transient Turbo. If it accepts it, the outgoing sync for `balanced-performance` switches to writing sysfs first and PPD afterwards.

- **Read cadence (measured 2026-10-12).** Each WMI read costs 5–10 ms of CPU (`pwm1` 11 ms, `pwm1_enable` 8 ms, `usb_charging` 8 ms, `kbd_backlight_*` 6 ms, `platform_profile` 6 ms); `fan*_input`, `temp*_input`, `power_supply` cost ~0, and each `acer-wmi-battery` mode read ~10 ms (one WMI call, since 2026-10-19). Hence: AC/charge via uevent (2 s only without netlink); `Fans.State` every 5 s; keyboard, USB, limit, calibration and health every 30 s (and right after each write); `platform_profile` via `poll()` (sysfs_notify) on both files (class and `/sys/firmware/acpi`) with a safety reread every 10 s; AC/charge via uevent with a reread every 30 s. Result: ~1.7 % CPU idle, versus 6 % reading everything every 2 s.
- **Corrupted reads.** Concurrent WMI calls from different processes corrupt responses (see `firmware-wmi.md` §9). The daemon serializes its own with a mutex and, on an external profile change, only acts if the value is a known name and repeats in a second read 150 ms later.
- **External changes** to the fans (another program on hwmon) show up in `Fans.State` within 5 s.
- **Logging** to journald; `--dry-run` (does not write) and `--simulate <model>` (fake backends with fixed capabilities, for developing the GUI without hardware).
- Unit `acerd.service`: `Type=dbus`, `BusName=io.github.fnndk.Acerd1`, `After=acer-wmi-modules.service`, `ProtectSystem=strict` with `ReadWritePaths=/sys /var/lib/acerd`, `DeviceAllow=/dev/input/event* r`, `CapabilityBoundingSet=CAP_NET_ADMIN` (connector) — to be refined once the skeleton exists.

## 12. Changes from v0

1. Reads became **properties** with `PropertiesChanged`; custom `Changed` signals removed. Methods are only for writes.
2. **Power rules** completed: on battery `balanced`, `low-power` and, as a flagged extension, `balanced-performance` with charge ≥ 40 %; `quiet` now requires AC. v0 only forbade `low-power` on AC and `performance` on battery. New `Allowed` and `Extras` properties keep each client from reimplementing the rule.
3. `profiles` in `Capabilities` comes from `acer-wmi-extras/supported_profiles`, not from the `platform-profile` `choices` (which lists Turbo even without support).
4. **Detection table** for each capability (sysfs → key), instead of a loose description.
5. Set of **errors** defined (v0 only had `NotAllowed`).
6. **Monitor:** units/types fixed; `Subscribe` specified (unicast per subscriber, cleanup on `NameOwnerChanged`, no sampling without subscribers).
7. **Lighting:** `on` is read-only (there is no state Set in APGe); timeout limited to the values the firmware accepts.
8. **Battery:** limit, calibration and USB are not "restored" (they live in the EC); calibration ends by polling, since there is no way to catch the WMI event without taking the handler away from `acer_wmi`.
9. `Profiles` → **`Scenarios`**; scenario format and storage defined; App Sync restricted to the active session's user.
10. **polkit** split into `control` (no password in the active session) and `settings` (`auth_admin_keep`).
11. `manual` fans are not restored at boot.
12. Coexistence with `tuned-ppd` defined from its code: direct sysfs write, `ActiveProfile` sync both ways (many-to-one outgoing, choice by power source incoming), delayed reapply after power events.
13. Manual fan with no fixed floor: per-fan range in `capabilities.fans.range` (0–100 by default), the firmware is the arbiter.

## 13. Open points

- **Per-user scenarios** (`~/.config`) in addition to the global ones: possible by having the GUI store the JSON and call `Apply` with the dictionary; decide whether the daemon needs to know about it.
- ~~Validate `pwm=0` in manual~~ **Done (2026-10-12):** with `pwm1_enable=1` and `pwm1=0` the CPU fan stayed at ~1 955 RPM for 8 s; the firmware enforces a floor and does not stop it. The 0–100 range stays as is.
- **Validation with tuned-ppd (2026-10-12), on AC:** `Set quiet` → PPD `power-saver`; `Set balanced-performance` → PPD `performance`; `Set balanced` → `balanced`; stable 5 s later, no loop. Through PPD: `power-saver` → `quiet`, `performance` → `balanced-performance`, `balanced` → `balanced`. Watching `platform_profile` every 20 ms during the outgoing switch to `performance`, the value never showed `performance`: tuned writes (there is a `sysfs_notify`), but the value read stays the previous one, i.e. the firmware/driver rejects the nonexistent Turbo. The "PPD first, sysfs after" order stays. Battery (unplugging/plugging the cable) validated later (see STATUS).
- **Suspend/resume (S3) on the ANV15-52, validated 2026-10-12.** The firmware **resets the profile**: ~0.8 s after `suspend exit` `platform_profile` goes back to `balanced` and tuned follows (PPD → `balanced`). The **fans survive**: `pwm*_enable` 0 (max) and 1 with the PWM (40 % = 102) stay as they were. The daemon listens to logind's `PrepareForSleep` (no inhibitor) and, from `true` until the resume finishes, ignores writes to `platform_profile` and PPD switches (otherwise it would save `balanced` as the user's choice). On resume it waits 3 s, rereads source and profile, reapplies the profile saved for the source and the saved fans (manual included) and rereads the rest. At daemon startup, a fan saved as `manual` goes back to `auto` and the state becomes `auto`.
- **GPU:** the first `nvidia-smi` read after the dGPU wakes up (D3cold) returns `power.draw` = 590.01 W; the next one, ~10 W. The daemon omits `power` when the value exceeds 400 W. The initial probe (`nvidia-smi`) wakes the GPU and takes ~1.5 s; with NVML (go-nvml) both problems can be avoided.
- **Sense key on GNOME:** `KEY_PROG1` may be bound to a desktop shortcut; check whether it needs its own `hwdb` or whether the desktop ignores it.
