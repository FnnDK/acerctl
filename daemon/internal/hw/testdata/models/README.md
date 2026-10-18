# Model fixtures

One directory per known model. `root/` is the part of a machine's file system that `acerd` reads and writes (`/sys`, `/proc/sys/kernel/osrelease`, `/run/udev/data`, `/dev/input`, `/etc/modprobe.d`) with the in-tree `acer_wmi` and both companion modules loaded, as the packages leave it. `acerd --root DIR` and the tests read it in place of `/`; the hardware code (`hw.Probe` and every backend) runs unchanged on top of it. Files the firmware does not offer on a model are absent, as on the machine.

| Model | Source |
|---|---|
| `ANV15-52` | Nitro ANV15-52, the reference machine: values read on it (BIOS V1.31, kernel 7.2.10, Fedora Silverblue 44), the 2026-10-13 hardware report |
| `AG15-71P` | Aspire AG15-71P: hardware report v3 (issue #1) for DMI, WMI devices, power supply and sensors; the companion modules' attributes derived from the firmware's answers in the report's probes (no profiles, no fans; USB charging answers 0xE1, so `acer-wmi-extras` hides it; the keyboard timeout call answers status 0, but DMI type 0xAA lists the keyboard backlight as absent, so `acer-wmi-extras` hides `kbd_backlight_*` too) |
| `PH16-71` | Predator PH16-71: no report yet. Follows `acerd --simulate PH16-71` and the public findings of `docs/firmware-wmi.md` §11 (USB keyboard, so no i8042 keyboard and no Sense key on evdev; no keyboard timeout); sensor and battery values are illustrative, and the battery reports `energy_*` to cover that path. Replace it with a real report when one comes in |

Per model, next to `root/`:

- `expected.json`: what `go test ./internal/hw -run TestModels` gets from the hardware layer: capabilities, model, setup items, the values read, one change through each backend and the files those changes wrote.
- `acerctl.txt`: the transcript of `tools/models-run.sh` (`acerd` on D-Bus, `acerctl --json status`, a fixed list of commands, the files written), checked by `tools/models-test.sh` on the packages installed in Fedora 44 and Ubuntu 24.04.
- `boot-modules.txt`: the companion modules that load at boot with the packages: through the WMI alias of one of the model's devices (`/sys/bus/wmi/devices/*/modalias`) or `modules-load.d`.
- `dmi-170.txt` (models with a real report only): the machine's DMI type 170 structure as `dmidecode -t 170 -u` prints it (the report's `dmi.txt`). `acerd` does not read it; `TestModelsKbdDMI` checks `root/` against it with the rule of `acer-wmi-extras`: when it lists the keyboard backlight as absent, `root/` has no `kbd_backlight_*`.

After an intended change, `sh tools/models-test.sh --update` rewrites `expected.json` and `acerctl.txt` from the code; review their diff before committing. `boot-modules.txt` and `dmi-170.txt` are written by hand.

A new model: copy the closest directory, change `root/` after its hardware report (`tools/survey.sh`), copy the type 170 block of its `dmi.txt` into `dmi-170.txt`, run `--update` and check that nothing changed for the other models. The companion modules' attributes come from the firmware's answers in the report's probes and, for the keyboard backlight, from the summary's "DMI type 170" line: a WMI answer with status 0 alone does not mean the laptop has one.
