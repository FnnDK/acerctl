# Hardware reports

acerctl shows each feature only when the laptop's firmware offers it, so supporting a new model mostly means learning what its firmware exposes. `tools/survey.sh` collects exactly that, without access to the machine: you run it, review the result if you like, and attach it to a [Hardware report](https://github.com/FnnDK/acerctl/issues/new?template=hardware-report.yml) issue.

## Running it

```bash
git clone https://github.com/FnnDK/acerctl.git
cd acerctl
sudo sh tools/survey.sh
```

This needs nothing beyond a standard Linux install and takes a few seconds. It prints a summary, writes `acer-report-<model>-<date>.tar.gz` in the current directory and prints a link to a new *Hardware report* issue with the model and the summary already filled in (also saved as `issue-url.txt`): open it and drag the `.tar.gz` into the form.

### Optional: firmware probes

```bash
sudo sh tools/survey.sh --probes
```

With `--probes` the script also asks the firmware what it supports, by calling about 90 **read-only** ("Get") WMI methods through the research module in `drivers/acer-wmi-lab`. The module is built, loaded for the run and unloaded at the end. This tells us much more (supported fans, sensors, modes, lighting, battery functions), but it needs:

- `make`, a C compiler and the headers of the running kernel (`kernel-devel` on Fedora, `linux-headers-$(uname -r)` on Debian/Ubuntu, `linux-headers` on Arch);
- **Secure Boot off**, or a module signed with a key you have enrolled (MOK). With Secure Boot on, the script says so and skips the probes; the rest of the report is still useful;
- the acerctl daemon stopped, if you have it installed (`sudo systemctl stop acerd`), and no other Acer WMI module loaded (`linuwu_sense`, `facer`): concurrent WMI calls can corrupt each other's results on some firmware. The script checks both and skips the probes otherwise.

On image-based systems without a compiler on the host (Fedora Silverblue, Bluefin, Kinoite…), build the module in a toolbox and pass it to the script:

```bash
toolbox run sh -c 'sudo dnf install -y gcc make kernel-devel-$(uname -r) && make -C drivers/acer-wmi-lab KDIR=/usr/src/kernels/$(uname -r)'
sudo sh tools/survey.sh --lab-module drivers/acer-wmi-lab/acer-wmi-lab.ko
```

## What is collected

| File | Contents |
|---|---|
| `summary.txt` | Model, BIOS/EC version, CPU/GPU, distribution, Secure Boot, what the kernel already exposes, other Acer tools found (Linuwu-Sense, Div-Acer-Manager-Max, acer-predator-turbo), list of WMI devices, probe result |
| `dmi.txt` | Model and BIOS identification, including the Acer OEM DMI tables (types 170–172) |
| `wmi.txt`, `bmof-*.bin` | WMI devices and the firmware's own WMI class definitions (binary MOF) |
| `acpi/*.dat` | DSDT and SSDT tables (where the WMI methods are implemented) |
| `acer_wmi.txt`, `platform_profile.txt`, `hwmon.txt`, `power_supply.txt`, `leds.txt`, `input.txt`, `misc.txt`, `dmesg.txt` | What the kernel and its drivers already show for this laptop |
| `hid.txt` | HID devices (vendor:product, name) and their report descriptors (some models switch modes or lighting through the embedded controller's HID device) |
| `system.txt` | Kernel version, distribution, PCI/USB device list, loaded modules |
| `probes.txt` | Raw answers to the read-only firmware calls (only with `--probes`) |
| `issue-url.txt` | The prefilled issue link |

**Not collected:** hostname, user name, serial numbers, UUIDs (including the root filesystem UUID in the kernel command line), MAC addresses, Bluetooth addresses, and the ACPI `MSDM` table (which holds the Windows product key). Everything except the raw firmware tables is plain text you can open and check before sending.

## Safety

- The script never writes to the firmware and never changes a setting. Without `--probes` it only reads files under `/sys` and `/proc` and runs standard tools (`dmidecode`, `lspci`, `lsusb`).
- `--probes` only calls methods whose names start with "Get", using the method IDs that `acer_wmi` and the public Acer Linux projects already use on many models. The research module stays loaded only during the run.
- As with any software here, there is no warranty (see `LICENSE`).

## What happens next

We decode the firmware tables, compare them with the models already known and reply in the issue: which features can work right away, which need driver changes, and, if needed, a specific test to run. A test that changes a setting is only ever proposed with a format already validated on another model and a way to undo it.
