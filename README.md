<p align="center"><img src="assets/logo/a-icon-cut.svg" width="112" alt="">&nbsp;&nbsp;<img src="assets/logo/n-icon-cut.svg" width="112" alt="">&nbsp;&nbsp;<img src="assets/logo/p-icon-cut.svg" width="112" alt=""></p>

<h1 align="center">acerctl</h1>

<p align="center">All-in-one Linux alternative to Acer's AcerSense, NitroSense and PredatorSense: one daemon, CLI and app for operating modes, fans, battery, keyboard and sensors, showing only the features the laptop's firmware offers.</p>

<p align="center"><b>Download:</b> <a href="#acer">Acer</a> · <a href="#nitro">Nitro</a> · <a href="#predator">Predator</a></p>

**Contents:** [Download](#download) ([Acer](#acer), [Nitro](#nitro), [Predator](#predator)) · [Upgrading](#upgrading) · [Uninstalling](#uninstalling) · [What it does](#what-it-does) · [Screenshots](#screenshots) · [Tested models](#tested-models) · [Compared with other tools](#compared-with-other-tools) · [GNOME integration](#gnome-integration) · [Components](#components) · [Documentation](#documentation) · [Building from source](#building-from-source) · [License](#license)

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

Latest: **[0.1.3](https://github.com/FnnDK/acerctl/releases/tag/v0.1.3)** (pre-release), ready-built packages for **Fedora 44** (Workstation and the Atomic desktops: Silverblue, Kinoite, Bluefin...) and **Debian 13, Ubuntu 24.04 and 26.04** (the .deb packages are new and so far tested in containers only). Run the commands on the host, not inside a toolbox/distrobox. Pick your laptop's product line: each one gets the app with its own name and colors, on top of the same daemon and kernel modules. The kernel modules are rebuilt for every new kernel (akmod on Fedora, DKMS on Debian/Ubuntu) and signed with the local key when Secure Boot needs it. The daemon starts by itself when the app or the tray first needs it.

Then see [After installing](#after-installing). Coming from an earlier version or from an install made with the scripts? See [Upgrading](#upgrading). Another distribution? See [Building from source](#building-from-source).

### <img src="assets/logo/a-icon-cut.svg" width="28" height="28" alt="">&nbsp;Acer

Aspire, Swift, TravelMate, Extensa and the other non-gaming lines: **Acer Control Center**. Tested on the Aspire AG15-71P (charge limit, Sense key, app and tray).

**Fedora 44:**

```bash
v=0.1.3; app=acer-control-center
mkdir -p acerctl-$v && cd acerctl-$v
for p in acerctl-$v-1.fc44.x86_64 acerctl-tray-$v-1.fc44.x86_64 $app-$v-1.fc44.x86_64 \
         akmod-acerctl-$v-1.fc44.x86_64 acerctl-kmod-common-$v-1.fc44.noarch; do
  curl -LO https://github.com/FnnDK/acerctl/releases/download/v$v/$p.rpm
done
if [ -e /run/ostree-booted ]; then           # Silverblue, Kinoite, Bluefin...
  sudo rpm-ostree install ./*.rpm && echo "installed: reboot to start using it"
else
  sudo dnf install ./*.rpm
fi
```

**Debian 13, Ubuntu 24.04 and 26.04:**

```bash
v=0.1.3; app=acer-control-center
mkdir -p acerctl-$v && cd acerctl-$v
for p in acerctl_${v}_amd64 acerctl-tray_${v}_amd64 ${app}_${v}_amd64 acerctl-dkms_${v}_all; do
  wget -nv -O $p.deb https://github.com/FnnDK/acerctl/releases/download/v$v/$p.deb
done
sudo apt install ./*.deb
```

### <img src="assets/logo/n-icon-cut.svg" width="28" height="28" alt="">&nbsp;Nitro

**Nitro Control Center**. Tested on the Nitro ANV15-52 (modes, fans, sensors, keyboard, USB charging, battery, scenarios).

**Fedora 44:**

```bash
v=0.1.3; app=nitro-control-center
mkdir -p acerctl-$v && cd acerctl-$v
for p in acerctl-$v-1.fc44.x86_64 acerctl-tray-$v-1.fc44.x86_64 $app-$v-1.fc44.x86_64 \
         akmod-acerctl-$v-1.fc44.x86_64 acerctl-kmod-common-$v-1.fc44.noarch; do
  curl -LO https://github.com/FnnDK/acerctl/releases/download/v$v/$p.rpm
done
if [ -e /run/ostree-booted ]; then           # Silverblue, Kinoite, Bluefin...
  sudo rpm-ostree install ./*.rpm && echo "installed: reboot to start using it"
else
  sudo dnf install ./*.rpm
fi
```

**Debian 13, Ubuntu 24.04 and 26.04:**

```bash
v=0.1.3; app=nitro-control-center
mkdir -p acerctl-$v && cd acerctl-$v
for p in acerctl_${v}_amd64 acerctl-tray_${v}_amd64 ${app}_${v}_amd64 acerctl-dkms_${v}_all; do
  wget -nv -O $p.deb https://github.com/FnnDK/acerctl/releases/download/v$v/$p.deb
done
sudo apt install ./*.deb
```

### <img src="assets/logo/p-icon-cut.svg" width="28" height="28" alt="">&nbsp;Predator

**Predator Control Center**. Not tested on a Predator yet: the features come from the firmware, and a [hardware report](docs/hardware-reports.md) from your machine helps a lot.

**Fedora 44:**

```bash
v=0.1.3; app=predator-control-center
mkdir -p acerctl-$v && cd acerctl-$v
for p in acerctl-$v-1.fc44.x86_64 acerctl-tray-$v-1.fc44.x86_64 $app-$v-1.fc44.x86_64 \
         akmod-acerctl-$v-1.fc44.x86_64 acerctl-kmod-common-$v-1.fc44.noarch; do
  curl -LO https://github.com/FnnDK/acerctl/releases/download/v$v/$p.rpm
done
if [ -e /run/ostree-booted ]; then           # Silverblue, Kinoite, Bluefin...
  sudo rpm-ostree install ./*.rpm && echo "installed: reboot to start using it"
else
  sudo dnf install ./*.rpm
fi
```

**Debian 13, Ubuntu 24.04 and 26.04:**

```bash
v=0.1.3; app=predator-control-center
mkdir -p acerctl-$v && cd acerctl-$v
for p in acerctl_${v}_amd64 acerctl-tray_${v}_amd64 ${app}_${v}_amd64 acerctl-dkms_${v}_all; do
  wget -nv -O $p.deb https://github.com/FnnDK/acerctl/releases/download/v$v/$p.deb
done
sudo apt install ./*.deb
```

### After installing

Open the app: on its first start it lists what is still missing and applies it with one password and one restart. That is the `acer_wmi` option Nitro and Predator models need for operating modes and fans, and, with Secure Boot, enrolling the key that signs the kernel modules (akmods on Fedora, DKMS on Debian/Ubuntu). `acerctl status` shows what the daemon found and `acerctl help` lists the commands.

By hand, the `acer_wmi` option for the Nitro ANV15-52 is (`/etc/modprobe.d/acer-wmi.conf`, then reboot):

```
options acer_wmi predator_v4=1 force_caps=0x1C26
```

Before enabling `platform-profile`, check the `tuned`/`power-profiles-daemon` profile: Linux's `performance` mode maps to Acer's Turbo, which not every model has.

The kernel decides part of it: `acer_wmi` exposes the modes from Linux 6.9 and fan control from 6.19 (Debian 13 has the modes only; Ubuntu 24.04 with the HWE kernel, 26.04 and Fedora have both; Ubuntu 24.04's original 6.8 kernel has neither). The companion modules build from 5.15 on.

### Upgrading

- **Fedora Workstation, Debian, Ubuntu:** run the block of your product line again; `dnf`/`apt` upgrade in place.
- **Silverblue, Kinoite, Bluefin:** remove the installed packages first, then run the block and reboot once:
  ```bash
  sudo rpm-ostree uninstall acerctl acerctl-tray nitro-control-center akmod-acerctl acerctl-kmod-common   # your app's name
  ```
- **From an install made with the scripts** (including the older `nitrod`/`nitroctl` ones): remove it first, as [Uninstalling](#uninstalling) shows. A unit left in `/etc/systemd/system` overrides the packaged one.

### Uninstalling

**Packages:** remove the ones you installed (Silverblue, Kinoite, Bluefin: `rpm-ostree uninstall` with the same names, then reboot):

```bash
sudo dnf remove acerctl acerctl-tray nitro-control-center akmod-acerctl acerctl-kmod-common   # or predator-/acer-control-center
sudo apt purge acerctl acerctl-tray nitro-control-center acerctl-dkms                          # Debian/Ubuntu
```

**Script installs** (from source or the binary archive), including the ones made under the project's earlier names (`nitrod`/`nitroctl`/`nitro-tray`, `nitrosensed`). Run it from the repository or the archive directory, or download it on its own:

```bash
curl -LO https://raw.githubusercontent.com/FnnDK/acerctl/main/packaging/uninstall.sh
sudo sh uninstall.sh            # keeps your settings (/var/lib/acerd, ~/.config/acerctl)
sudo sh uninstall.sh --purge    # also removes them, the app data and /etc/modprobe.d/acerctl.conf
```

Run it on the host: inside a toolbox/distrobox shell, `sudo` only reaches the container, so the script refuses to run there (use `distrobox-host-exec sudo sh uninstall.sh`). It stops and removes the daemon, CLI, app, tray, kernel modules and their units. Packages, if any, are left in place with the command to remove them, and so is an `acer_wmi` option you wrote yourself in `/etc/modprobe.d`.

## What it does

- **Operating modes** (Eco, Quiet, Balanced, Performance, Turbo), limited to the profiles the firmware supports, with the original app's AC/battery rules and sync with the desktop's `power-profiles-daemon`/`tuned`.
- **CPU and GPU fans** in auto, max or manual speed.
- **Monitor** for temperatures, usage, clocks, power, fan speed, RAM and network, without waking the discrete GPU when it is powered off.
- **Battery:** 80 % charge limit, calibration, health, cycles, temperature, voltage and current.
- **Keyboard:** backlight auto-off timeout. **USB:** charge devices while the laptop is off.
- **Scenarios** combining mode, fans and keyboard, and **App Sync**, which applies a scenario when a game or program starts (including through Wine/Proton) and restores the previous state when it exits.
- **Sense key** (the AcerSense/NitroSense/PredatorSense button) opening the app or cycling the modes; state restored at boot and after suspend.

## Screenshots

### <img src="assets/logo/a-icon-cut.svg" width="28" height="28" alt="">&nbsp;Acer Control Center

![Dashboard](docs/screenshots/acer/home.png)

| | |
|---|---|
| ![Monitor](docs/screenshots/acer/monitor.png) | ![Battery](docs/screenshots/acer/battery.png) |
| ![Settings](docs/screenshots/acer/settings.png) | |

<sub>Aspire AG15-71P, real machine with the daemon in read-only mode (`acerd --dry-run`). Its firmware has no operating modes, fans or keyboard backlight, so those screens and cards do not show up.</sub>

### <img src="assets/logo/n-icon-cut.svg" width="28" height="28" alt="">&nbsp;Nitro Control Center

![Dashboard](docs/screenshots/nitro/home.png)

| | |
|---|---|
| ![Monitor](docs/screenshots/nitro/monitor.png) | ![Fans](docs/screenshots/nitro/fans.png) |
| ![Battery](docs/screenshots/nitro/battery.png) | ![Scenarios](docs/screenshots/nitro/scenarios.png) |
| ![Lighting](docs/screenshots/nitro/lighting.png) | ![Settings](docs/screenshots/nitro/settings.png) |

<sub>Daemon in simulated mode (`acerd --simulate ANV15-52`).</sub>

### <img src="assets/logo/p-icon-cut.svg" width="28" height="28" alt="">&nbsp;Predator Control Center

![Dashboard](docs/screenshots/predator/home.png)

| | |
|---|---|
| ![Monitor](docs/screenshots/predator/monitor.png) | ![Fans](docs/screenshots/predator/fans.png) |
| ![Battery](docs/screenshots/predator/battery.png) | ![Scenarios](docs/screenshots/predator/scenarios.png) |
| ![Lighting](docs/screenshots/predator/lighting.png) | ![Settings](docs/screenshots/predator/settings.png) |

<sub>Daemon in simulated mode (`acerd --simulate PHN16-72`: a Predator with 5 modes, a 4-zone RGB keyboard and an RGB logo, modelled on public reports; no Predator has been tested yet).</sub>

The app uses its own dark theme, with an integrated title bar that follows GNOME's button order and icons, and texts in Portuguese and English.

## Tested models

On the laptops acerctl has been run on: <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> works, <img src="assets/readme/no.svg" width="16" height="16" alt="no"> the laptop has it but acerctl does not handle it yet, <img src="assets/readme/na.svg" width="16" height="16" alt="n/a"> the firmware does not offer it.

| Model | Modes | AC/battery rules | Fans | Sensors | 80 % limit | Calibration | Kbd timeout | RGB | USB charging off | Boot sound | LCD overdrive | Overclock | Scenarios | Sense key | GUI |
|---|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|
| **Nitro ANV15-52** (BIOS V1.31, Fedora Silverblue 44) | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes">¹ | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes">² | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/na.svg" width="16" height="16" alt="n/a"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/na.svg" width="16" height="16" alt="n/a"> | <img src="assets/readme/na.svg" width="16" height="16" alt="n/a"> | <img src="assets/readme/na.svg" width="16" height="16" alt="n/a"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> |
| **Aspire AG15-71P** (BIOS V1.13, Bluefin 44) | <img src="assets/readme/na.svg" width="16" height="16" alt="n/a"> | <img src="assets/readme/na.svg" width="16" height="16" alt="n/a"> | <img src="assets/readme/na.svg" width="16" height="16" alt="n/a"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes">³ | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes">¹ | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes">² | <img src="assets/readme/na.svg" width="16" height="16" alt="n/a"> | <img src="assets/readme/na.svg" width="16" height="16" alt="n/a"> | <img src="assets/readme/na.svg" width="16" height="16" alt="n/a"> | <img src="assets/readme/na.svg" width="16" height="16" alt="n/a"> | <img src="assets/readme/na.svg" width="16" height="16" alt="n/a"> | <img src="assets/readme/na.svg" width="16" height="16" alt="n/a"> | <img src="assets/readme/na.svg" width="16" height="16" alt="n/a">⁴ | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> |

¹ Also in GNOME's Power settings ("Preserve Battery Health"). ² The firmware offers it and acerctl exposes it; a full calibration cycle has not been run yet. ³ CPU temperature and the battery sensors; this model has no fan readings and no discrete GPU. ⁴ Nothing for a scenario to set without modes, fans or keyboard backlight.

Both, plus a Predator PH16-71 built from public reports, are also checked automatically on every change: the packages installed in Fedora 44 and Ubuntu 24.04 run against a copy of what each laptop exposes (`daemon/README.md`, "Known models"). A [hardware report](docs/hardware-reports.md) from another model is how it gets added.

## Compared with other tools

| Project | Modes | AC/battery rules | Fans | Sensors | 80 % limit | Calibration | Kbd timeout | RGB | USB charging off | Boot sound | LCD overdrive | Overclock | Scenarios | Sense key | GUI | Keeps `acer_wmi` |
|---|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|
| `acer_wmi` (in-tree kernel driver) | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes">¹ | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes">¹ | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> |
| [acer-wmi-battery](https://github.com/frederik-h/acer-wmi-battery) (kernel module) | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> |
| [Linuwu-Sense](https://github.com/0x7375646F/Linuwu-Sense) (kernel module) | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> |
| [Div-Acer-Manager-Max](https://github.com/PXDiv/Div-Acer-Manager-Max) (GUI + daemon on Linuwu-Sense) | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> |
| [predator-sense](https://github.com/wobqqq/predator-sense) (CLI + DKMS on Linuwu-Sense) | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> |
| Acer NitroSense / PredatorSense (Windows only) | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes">² | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes">² | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes">² | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/na.svg" width="16" height="16" alt="n/a"> |
| **acerctl** (this project) | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes">³ | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes">⁴ | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/no.svg" width="16" height="16" alt="no"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> | <img src="assets/readme/yes.svg" width="16" height="16" alt="yes"> |

¹ A fixed list of 5 profiles, without reading the firmware mask; fan control only with the model quirk or `force_caps`. ² Where the model has it. ³ Only the profiles the firmware reports, synced with `power-profiles-daemon`/`tuned`. ⁴ Shared with GNOME/UPower.

Sensors: CPU/GPU temperatures and fan speed. acerctl also has a CLI, a tray, a monitor that does not wake the dGPU and the state restored at boot and after suspend; RGB, boot sound, LCD overdrive and overclocking are not there yet.

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

**Prebuilt binaries for other distributions** (glibc ≥ 2.34, x86_64): [`acerctl-0.1.3-linux-x86_64.tar.gz`](https://github.com/FnnDK/acerctl/releases/download/v0.1.3/acerctl-0.1.3-linux-x86_64.tar.gz) holds the daemon, CLI, tray and app already built, these scripts and the kernel modules as source; its `INSTALL.md` has the steps (no Go, Node or Wails needed). The app is picked from the laptop's product name.

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
