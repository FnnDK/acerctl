<p align="center"><img src="assets/logo/a-icon-cut.svg" width="112" alt="">&nbsp;&nbsp;<img src="assets/logo/n-icon-cut.svg" width="112" alt="">&nbsp;&nbsp;<img src="assets/logo/p-icon-cut.svg" width="112" alt=""></p>

<h1 align="center">acerctl</h1>

<p align="center">All-in-one Linux alternative to Acer's AcerSense, NitroSense and PredatorSense: one daemon, CLI and app for operating modes, fans, battery, keyboard and sensors, showing only the features the laptop's firmware offers.</p>

<p align="center"><b>Download:</b> <a href="#acer">Acer</a> · <a href="#nitro">Nitro</a> · <a href="#predator">Predator</a></p>

**Contents:** [Download](#download) ([Acer](#acer), [Nitro](#nitro), [Predator](#predator)) · [Uninstalling](#uninstalling) · [What it does](#what-it-does) · [Screenshots](#screenshots) · [Compared with other tools](#compared-with-other-tools) · [GNOME integration](#gnome-integration) · [Components](#components) · [Documentation](#documentation) · [Building from source](#building-from-source) · [License](#license)

Acer ships a different Windows-only app per product line: **AcerSense** (Aspire, Swift and the other non-gaming lines), **NitroSense** (Nitro) and **PredatorSense** (Predator). The laptops themselves share the same Acer WMI firmware interfaces (the ones the kernel's `acer_wmi` driver already uses across product lines), so acerctl covers all three with a single stack: each feature shows up only if the machine's firmware reports it, with no per-brand switch and no hard-coded model list.

| Windows app | Laptops | Status in acerctl |
|---|---|---|
| AcerSense | Aspire, Swift, TravelMate, Extensa | in progress (Aspire AG15-71P) |
| NitroSense | Nitro | working (reference machine: Nitro ANV15-52) |
| PredatorSense | Predator | to do |

Your model is not on the list? Run the read-only [hardware report](docs/hardware-reports.md) and open a *Hardware report* issue: that is all we need to start supporting it.

acerctl works **by complementing the in-tree `acer_wmi` driver** instead of replacing it: what the kernel lacks comes from small companion modules, a daemon applies the rules, and an app and a CLI (`acerctl`) talk to it over D-Bus. The app comes in three flavors built from the same code, differing only in name and colors: `Acer Control Center` (green `[a]`) for Aspire and the other lines, `Nitro Control Center` (red `[N]`) for Nitro and `Predator Control Center` (blue-grey `[P]`) for Predator; the installer picks one from the product name.

> Independent open-source project, not affiliated with Acer. "Acer", "Aspire", "Nitro", "Predator", "AcerSense", "NitroSense" and "PredatorSense" are trademarks of Acer Inc., mentioned only to identify the hardware.

## Download

Latest: **[0.1.1](https://github.com/FnnDK/acerctl/releases/tag/v0.1.1)** (pre-release), ready-built packages for **Fedora 44** (Workstation, Silverblue, Kinoite). Pick your laptop's product line: each one gets the app with its own name and colors, on top of the same daemon and kernel modules. The kernel modules come as an akmod, rebuilt for every new kernel and signed with the local akmods key when Secure Boot needs it.

Then see [After installing](#after-installing). Upgrading from an install made with the scripts (including the older `nitrod`/`nitroctl` ones)? Remove it first, as [Uninstalling](#uninstalling) shows: a unit left in `/etc/systemd/system` overrides the packaged one. Another distribution? See [Building from source](#building-from-source).

### Acer

Aspire, Swift, TravelMate, Extensa and the other non-gaming lines: **Acer Control Center**. Tested on the Aspire AG15-71P (charge limit, Sense key, app and tray).

```bash
v=0.1.1; app=acer-control-center
mkdir acerctl-rpms && cd acerctl-rpms
for p in acerctl-$v-1.fc44.x86_64 acerctl-tray-$v-1.fc44.x86_64 $app-$v-1.fc44.x86_64 \
         akmod-acerctl-$v-1.fc44.x86_64 acerctl-kmod-common-$v-1.fc44.noarch; do
  curl -LO https://github.com/FnnDK/acerctl/releases/download/v$v/$p.rpm
done
sudo dnf install ./*.rpm                   # Silverblue/Kinoite: rpm-ostree install ./*.rpm, then reboot
sudo systemctl enable --now acerd
```

### Nitro

**Nitro Control Center**. Tested on the Nitro ANV15-52 (modes, fans, sensors, keyboard, USB charging, battery, scenarios).

```bash
v=0.1.1; app=nitro-control-center
mkdir acerctl-rpms && cd acerctl-rpms
for p in acerctl-$v-1.fc44.x86_64 acerctl-tray-$v-1.fc44.x86_64 $app-$v-1.fc44.x86_64 \
         akmod-acerctl-$v-1.fc44.x86_64 acerctl-kmod-common-$v-1.fc44.noarch; do
  curl -LO https://github.com/FnnDK/acerctl/releases/download/v$v/$p.rpm
done
sudo dnf install ./*.rpm                   # Silverblue/Kinoite: rpm-ostree install ./*.rpm, then reboot
sudo systemctl enable --now acerd
```

### Predator

**Predator Control Center**. Not tested on a Predator yet: the features come from the firmware, and a [hardware report](docs/hardware-reports.md) from your machine helps a lot.

```bash
v=0.1.1; app=predator-control-center
mkdir acerctl-rpms && cd acerctl-rpms
for p in acerctl-$v-1.fc44.x86_64 acerctl-tray-$v-1.fc44.x86_64 $app-$v-1.fc44.x86_64 \
         akmod-acerctl-$v-1.fc44.x86_64 acerctl-kmod-common-$v-1.fc44.noarch; do
  curl -LO https://github.com/FnnDK/acerctl/releases/download/v$v/$p.rpm
done
sudo dnf install ./*.rpm                   # Silverblue/Kinoite: rpm-ostree install ./*.rpm, then reboot
sudo systemctl enable --now acerd
```

### After installing

Nitro and Predator models need `acer_wmi` with `predator_v4` for operating modes and fans, e.g. on the Nitro ANV15-52 (`/etc/modprobe.d/acer-wmi.conf`, then reboot):

```
options acer_wmi predator_v4=1 force_caps=0x1C26
```

Before enabling `platform-profile`, check the `tuned`/`power-profiles-daemon` profile: Linux's `performance` mode maps to Acer's Turbo, which not every model has. Then `acerctl status` shows what the daemon found and `acerctl help` lists the commands.

The kernel decides part of it: `acer_wmi` exposes the modes from Linux 6.9 and fan control from 6.19 (Debian 13 has the modes only; Ubuntu 24.04 HWE, 26.04 and Fedora have both). The companion modules build from 5.15 on.

From the next release on, the app does this for you: when it opens, it lists what is left (the `acer_wmi` option, enrolling the akmods key for Secure Boot) and applies it with one password and one restart.

### Uninstalling

**Packages:** remove the ones you installed (Silverblue/Kinoite: `rpm-ostree uninstall` with the same names, then reboot):

```bash
sudo dnf remove acerctl acerctl-tray nitro-control-center akmod-acerctl acerctl-kmod-common   # or predator-/acer-control-center
```

**Script installs** (from source or the binary archive), including the ones made under the project's earlier names (`nitrod`/`nitroctl`/`nitro-tray`, `nitrosensed`). Run it from the repository or the archive directory, or download it on its own:

```bash
curl -LO https://raw.githubusercontent.com/FnnDK/acerctl/main/packaging/uninstall.sh
sudo sh uninstall.sh            # keeps your settings (/var/lib/acerd, ~/.config/acerctl)
sudo sh uninstall.sh --purge    # also removes them, the app data and /etc/modprobe.d/acerctl.conf
```

It stops and removes the daemon, CLI, app, tray, kernel modules and their units. Packages, if any, are left in place with the command to remove them, and so is an `acer_wmi` option you wrote yourself in `/etc/modprobe.d`.

## What it does

- **Operating modes** (Eco, Quiet, Balanced, Performance, Turbo), limited to the profiles the firmware supports, with the original app's AC/battery rules and sync with the desktop's `power-profiles-daemon`/`tuned`.
- **CPU and GPU fans** in auto, max or manual speed.
- **Monitor** for temperatures, usage, clocks, power, fan speed, RAM and network, without waking the discrete GPU when it is powered off.
- **Battery:** 80 % charge limit, calibration, health, cycles, temperature, voltage and current.
- **Keyboard:** backlight auto-off timeout. **USB:** charge devices while the laptop is off.
- **Scenarios** combining mode, fans and keyboard, and **App Sync**, which applies a scenario when a game or program starts (including through Wine/Proton) and restores the previous state when it exits.
- **Sense key** (the AcerSense/NitroSense/PredatorSense button) opening the app or cycling the modes; state restored at boot and after suspend.

## Screenshots

### Acer Control Center

![Dashboard](docs/screenshots/acer/home.png)

| | |
|---|---|
| ![Monitor](docs/screenshots/acer/monitor.png) | ![Battery](docs/screenshots/acer/battery.png) |
| ![Settings](docs/screenshots/acer/settings.png) | |

<sub>Aspire AG15-71P, real machine with the daemon in read-only mode (`acerd --dry-run`). Its firmware has no operating modes, fans or keyboard backlight, so those screens and cards do not show up.</sub>

### Nitro Control Center

![Dashboard](docs/screenshots/nitro/home.png)

| | |
|---|---|
| ![Monitor](docs/screenshots/nitro/monitor.png) | ![Fans](docs/screenshots/nitro/fans.png) |
| ![Battery](docs/screenshots/nitro/battery.png) | ![Scenarios](docs/screenshots/nitro/scenarios.png) |
| ![Lighting](docs/screenshots/nitro/lighting.png) | ![Settings](docs/screenshots/nitro/settings.png) |

<sub>Daemon in simulated mode (`acerd --simulate ANV15-52`).</sub>

### Predator Control Center

![Dashboard](docs/screenshots/predator/home.png)

| | |
|---|---|
| ![Monitor](docs/screenshots/predator/monitor.png) | ![Fans](docs/screenshots/predator/fans.png) |
| ![Battery](docs/screenshots/predator/battery.png) | ![Scenarios](docs/screenshots/predator/scenarios.png) |
| ![Lighting](docs/screenshots/predator/lighting.png) | ![Settings](docs/screenshots/predator/settings.png) |

<sub>Daemon in simulated mode (`acerd --simulate PHN16-72`: a Predator with 5 modes, a 4-zone RGB keyboard and an RGB logo, modelled on public reports; no Predator has been tested yet).</sub>

The app uses its own dark theme, with an integrated title bar that follows GNOME's button order and icons, and texts in Portuguese and English.

## Compared with other tools

| Project | Features |
|---|---|
| `acer_wmi` (in-tree kernel driver) | Thermal profiles in `platform-profile` (fixed list of 5, without reading the firmware mask), fan RPM and temperatures in hwmon, max/manual fans via `pwm` (only with the model quirk or `force_caps`), Fn hotkeys and keyboard backlight events, wireless/rfkill. No battery, USB charging or backlight timeout; no user-space rules (AC/battery). |
| [acer-wmi-battery](https://github.com/frederik-h/acer-wmi-battery) (kernel module) | 80 % charge limit (health mode), battery calibration, battery temperature. Vendored here with a fix and the `charge_control_end_threshold` hook that GNOME/UPower use. |
| [Linuwu-Sense](https://github.com/0x7375646F/Linuwu-Sense) (kernel module, replaces `acer_wmi`) | Thermal profiles, CPU/GPU fan speed, 4-zone RGB keyboard with effects, 80 % charge limit, battery calibration, backlight timeout, boot sound, LCD overdrive, USB charging while off. |
| [Div-Acer-Manager-Max](https://github.com/PXDiv/Div-Acer-Manager-Max) (GUI + daemon on top of Linuwu-Sense) | Thermal profiles adjusted to AC/battery, auto/manual fans, LCD overdrive, backlight timeout, boot animation and sound toggle, live system info. |
| [predator-sense](https://github.com/wobqqq/predator-sense) (CLI + DKMS on top of Linuwu-Sense) | Thermal profiles, Turbo button as an on/off toggle, auto/max/manual fans per fan, temperature and RPM readout, 80 % charge limit and calibration, RGB keyboard (zones, effects, brightness), USB charging while off, backlight timeout, Fn+F4 fix. |
| Acer NitroSense / PredatorSense (Windows only) | Operating modes with AC/battery rules, auto/max/custom fans, system monitor and widget, battery health (80 % limit, calibration), RGB/keyboard lighting where present, USB charging while off, boot sound, LCD overdrive, CPU/GPU overclocking on some models, scenarios and App Sync, Nitro/Predator key. |
| **acerctl** (this project: companion modules + `acerd` + Acer/Nitro/Predator Control Center + CLI, alongside `acer_wmi`) | Operating modes limited to what the firmware reports, with the original app's AC/battery rules and `power-profiles-daemon`/`tuned` sync; auto/max/manual fans per fan; monitor (temperatures, usage, clocks, power, RPM, RAM, network) without waking the dGPU; 80 % charge limit (shared with GNOME), calibration, health; backlight timeout; USB charging while off; scenarios and App Sync (Wine/Proton aware); configurable Sense key; tray; CLI; state restored at boot and after suspend. Not yet: RGB, boot sound, LCD overdrive, overclocking. |

<sub>Third-party features as listed in each project's README (October 2026).</sub>

## GNOME integration

acerctl doesn't replace GNOME's own power controls. It stays in sync with them, so you can use either side.

### Power mode

GNOME's power mode menu (quick settings and Settings → Power) talks to `power-profiles-daemon` or `tuned-ppd`. `acerd` keeps that profile in sync with the Acer mode in both directions. The app has 5 modes and GNOME has 3, so the mapping is many-to-one going to GNOME. Coming back, the power source decides the mode. A mode the firmware doesn't report (e.g. Turbo on the ANV15-52) is never picked.

App → GNOME:

| App mode | GNOME power mode |
|---|---|
| Eco | Power Saver |
| Quiet | Power Saver |
| Balanced | Balanced |
| Performance | Performance |
| Turbo | Performance |

GNOME → app:

| GNOME power mode | On AC | On battery |
|---|---|---|
| Power Saver | Quiet | Eco |
| Balanced | Balanced | Balanced |
| Performance | Turbo when allowed (supported, battery ≥ 40 %), otherwise Performance | Performance (flagged as extra) when battery ≥ 40 %, otherwise Balanced |

When the app changes the mode, it sets the GNOME profile first, waits for `tuned` to apply it, and then writes the real Acer mode. This avoids loops. Details are in `docs/api-dbus.md` §11.

### Battery health

`acer-wmi-battery` exposes the 80 % limit through the standard `charge_control_end_threshold` attribute (kernel ≥ 6.15). UPower therefore sees it, and GNOME shows its own **Settings → Power → Preserve Battery Health** switch. That switch and the app's 80 % limit are the same setting:

- Turning on "Preserve Battery Health" in GNOME sets the 80 % limit, and the app shows it on the next refresh.
- The app's limit goes through UPower when it's available, so GNOME's switch follows it and UPower doesn't revert it at startup. Without UPower, `acerd` writes the firmware directly.
- The limit is either 80 % or off (100 %). The firmware has no other values and no start threshold.

Tested with UPower 1.91.5 on Fedora 44.

## Components

| Directory | What it is |
|---|---|
| `gui/` | **Acer / Nitro / Predator Control Center** (Go + Wails, WebKitGTK): the graphical app, one code in three flavors. Details in `gui/README.md`. |
| `daemon/` | **`acerd`** (Go): D-Bus service `io.github.fnndk.Acerd1` on the system bus, with polkit; power, fan, lighting, battery, monitor, scenario and App Sync rules. API in `docs/api-dbus.md`. Includes the **`acerctl`** CLI. |
| `drivers/acer-wmi-extras/` | Companion module (GPL): keyboard backlight timeout, USB charging while the laptop is off, the thermal profiles the firmware actually supports, battery sensors. |
| `drivers/acer-wmi-battery/` | Charge limit (80 %) and calibration, vendored from Frederik Himpe's project with one fix (`UPSTREAM.md`). |
| `drivers/acer-wmi-lab/` | Research module that calls WMI methods through `sysfs`; only for mapping firmware. |
| `packaging/` | systemd units, D-Bus policy, polkit actions and install scripts. |

Mainline layer used: `acer_wmi` with `predator_v4=1` (profiles in `platform-profile`, fans and temperatures in hwmon) and, until the model's quirk lands upstream, `force_caps=0x1C26` to unlock manual fan control.

## Documentation

- `STATUS.md`: project state, what has been done and next steps.
- `docs/firmware-wmi.md`: technical reference for the firmware (WMI classes, formats, capabilities).
- `docs/api-dbus.md`: `acerd` D-Bus API.

## Building from source

For other distributions, or to build from the repository. The scripts install to `/usr/local`, `/etc` and `/var/lib` (writable on Silverblue too); the reference machine is an Acer Nitro ANV15-52 with Fedora Silverblue 44.

**Prebuilt binaries for other distributions** (glibc ≥ 2.34, x86_64): [`acerctl-0.1.1-linux-x86_64.tar.gz`](https://github.com/FnnDK/acerctl/releases/download/v0.1.1/acerctl-0.1.1-linux-x86_64.tar.gz) holds the daemon, CLI, tray and app already built, these scripts and the kernel modules as source; its `INSTALL.md` has the steps (no Go, Node or Wails needed). The app is picked from the laptop's product name.

**From the repository:**

```bash
# 1. kernel modules (builds; signs with the akmods MOK when Secure Boot/lockdown
#    requires signed modules, otherwise they are used unsigned)
sudo sh packaging/build-sign.sh
sudo sh packaging/install-modules.sh          # or name the modules, e.g. acer-wmi-battery

# 2. acerd daemon + acerctl CLI
sh packaging/install-daemon.sh build          # needs Go
sudo sh packaging/install-daemon.sh install

# 3. the app (Acer, Nitro or Predator Control Center, after the product line)
sh packaging/install-gui.sh build             # needs Go, Node, Wails and libwebkit2gtk-4.1-dev
sudo sh packaging/install-gui.sh              # FLAVOR=nitro|predator|acer overrides; 'uninstall' removes it

# to remove all of it: sudo sh packaging/uninstall.sh (see Uninstalling)
```

The RPM specs are in `packaging/rpm/`; `acerctl-<version>.tar.gz` in each release is the source with vendored Go modules and the built frontend, for building without network.

### Building a module by hand

Requires `kernel-devel` for the running kernel, `gcc` and `make`. With Secure Boot, sign with an enrolled MOK (the akmods one on Fedora):

```bash
make -C drivers/acer-wmi-extras
sudo /usr/src/kernels/$(uname -r)/scripts/sign-file sha256 \
  /etc/pki/akmods/private/private_key.priv /etc/pki/akmods/certs/public_key.der \
  drivers/acer-wmi-extras/acer-wmi-extras.ko
sudo insmod drivers/acer-wmi-extras/acer-wmi-extras.ko
ls /sys/devices/platform/acer-wmi-extras/
```

## License

acerctl is free and open-source software under the GNU General Public License:

- **Daemon, CLI, tray, GUI and scripts** (`daemon/`, `gui/`, `packaging/`): [GPL-3.0-or-later](LICENSE).
- **Our kernel modules** (`drivers/acer-wmi-extras/`, `drivers/acer-wmi-lab/`): [GPL-2.0](drivers/LICENSE), as required for code that links against the Linux kernel.
- **`drivers/acer-wmi-battery/`**: GPL-2.0-or-later, the license of the upstream project ([its LICENSE](drivers/acer-wmi-battery/LICENSE)).

You can use, study, change and redistribute it; modified versions you distribute must stay under the same license and ship their source code.
