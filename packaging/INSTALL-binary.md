# Installing the acerctl binaries

This archive holds `acerd` (daemon), `acerctl` (CLI), `acer-tray` (tray agent) and the app (Acer, Nitro or Predator Control Center, picked from the laptop's product line), built for x86_64 Linux, plus the install scripts and the kernel modules as source. Everything goes to `/usr/local`, `/etc` and `/var/lib`, which are writable on Fedora Silverblue and other image-based systems too.

Read `README.md` first: support depends on what the laptop's firmware offers, and only some models have been tested.

## Requirements

- systemd, polkit and D-Bus (any mainstream desktop distribution);
- the app needs the WebKitGTK 4.1 runtime: `webkit2gtk4.1` (Fedora), `libwebkit2gtk-4.1-0` (Debian/Ubuntu), `webkit2gtk-4.1` (Arch);
- the kernel modules need `make`, `gcc` and the headers of the running kernel (`kernel-devel`, `linux-headers-$(uname -r)` or `linux-headers`). With Secure Boot, `packaging/build-sign.sh` signs them with the akmods key (Fedora, already enrolled when you use akmods); elsewhere, sign them with your own enrolled MOK (README, "Building a module by hand").

## Steps

Run from this directory:

```bash
# 1. kernel modules (optional but recommended: keyboard timeout, USB charging,
#    supported profiles, 80 % charge limit)
sudo sh packaging/build-sign.sh
sudo sh packaging/install-modules.sh          # or name them, e.g. acer-wmi-battery

# 2. daemon and CLI
sudo sh packaging/install-daemon.sh install

# 3. the app and the tray agent
sudo sh packaging/install-gui.sh              # FLAVOR=nitro|predator|acer overrides the choice
```

Then `acerctl status` shows what the daemon found. After a kernel update, run `sudo sh packaging/build-sign.sh` again. To remove the app: `sudo sh packaging/install-gui.sh uninstall`.

On Nitro and Predator models, `platform_profile` and the fans also need `acer_wmi` with `predator_v4` (see README, "Installation").

## Hardware reports

`sudo sh tools/survey.sh --probes` writes a read-only report of the firmware and prints a link to open an issue with it (`docs/hardware-reports.md` in the repository). This is how support for new models is added.
