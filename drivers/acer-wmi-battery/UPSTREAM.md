Vendored from https://github.com/frederik-h/acer-wmi-battery
- upstream: 9f90d75cc9237aeed7964622d10dbdf4d2c7b518 2026-01-25 Update MODELS.md
- plus the fix from https://github.com/bwz-kk/acer-wmi-battery commit 722822c2 ("fix: OOB heap read in health-control status/set functions"), applied locally as: a6ac733d54eb203657a58ec6a0d577a6b8d6bea9 fix: OOB heap read in health-control status/set functions
License: GPL-2.0 (see LICENSE).
- local (2026-10-13): binds the 79772EC5 WMI device and, on kernels ≥ 6.15, adds `charge_control_end_threshold` (80/100) to the ACPI battery through a battery hook + power_supply extension, so UPower/GNOME can toggle health mode; set paths serialized by a mutex.
- local (2026-10-17): WMI GUID in upper case, so the `wmi:` alias matches the uevent and the module autoloads (packages rely on it; the script install uses `insmod`).
