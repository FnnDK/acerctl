# Acer "Sense" firmware: WMI interfaces, EC and capabilities

Technical reference for what the BIOS of an Acer Nitro/Predator exposes to the operating system and how much of it Linux already covers. Reference machine: **Acer Nitro ANV15-52** (BIOS V1.31, i7-13620H, RTX 4060), kernel 7.2.9. Sources: the machine's own ACPI tables and BMOF blobs (`DSDT`, `SSDT10`, decoded with `iasl` and `bmf2mof`), kernel source (`drivers/platform/x86/acer-wmi.c`, `drivers/platform/wmi/core.c`, v7.2), the [Linuwu-Sense](https://github.com/0x7375646F/Linuwu-Sense) and [acer-wmi-battery](https://github.com/frederik-h/acer-wmi-battery) projects, and calls made with the `drivers/acer-wmi-lab` research module.

## 1. What Linux already covers on this machine

| Feature | Path | Notes |
|---|---|---|
| Thermal modes | `acer_wmi predator_v4=1` → `/sys/class/platform-profile/` | The driver lists 5 fixed profiles (`low-power quiet balanced balanced-performance performance`) without reading the firmware mask; this model has only 4 (see §7). Linux `performance` = Acer Turbo. |
| Fan RPM, temperatures | hwmon `acer` (`fan1`=CPU, `fan2`=GPU, `temp1` CPU, `temp2` GPU, `temp3` external) | GPU reads 0 while the dGPU is powered off. |
| Manual/max fan | hwmon `acer` `pwm1`/`pwm2` + `pwm*_enable` (0 max, 1 manual, 2 auto) | Only with the `.pwm` quirk or `force_caps=0x1C26` (bits wireless, bluetooth, function-mode, profile, hwmon, pwm). The model is not in the driver's DMI table. |
| Airplane mode key | `acer_wireless` (ACPI `10251229`) + `acer_wmi` rfkill | — |
| Sense key (NitroSense button) | PS/2 keyboard, scancode `0xF5` → `KEY_PROG1` (148) via systemd hwdb | **Not** a WMI event on this model; up to user space. |
| Fn hotkeys | EC `_Qxx` → `HKEV` → WMI event `676AA15E…` handled by `acer_wmi` (0x84 = keyboard backlight toggle, 0x61/0x62/0x63 brightness) | — |
| 80 % charge limit, calibration, battery temperature | `acer-wmi-battery` (battery GUID, methods 19/20/21) | Out-of-tree module, coexists with `acer_wmi`. |
| Audio, Wi-Fi, GPU | `sof-hda-dsp` + Realtek, `mt7921`, `nvidia`/`i915` | — |

## 2. BIOS WMI classes (BMOF, "Version 2.93")

| Class | GUID | ACPI method | Methods |
|---|---|---|---|
| **AcerGamingFunction** | `7A4DDFE7-5B5D-40B4-8595-4408E0CC7F56` | `WMBH` | 1 SetGamingProfile(u64) · 2 SetGamingLED(u8[16]) · 3 GetGamingProfile(u32→u64) · 4 GetGamingLED · 5 GetGamingSysInfo(u32→u64) · 6/7 Set/GetGamingRgbKb · 8/9 Set/GetGamingProfileSetting · 10/11 Set/GetGamingLEDBehavior · 12/13 Set/GetGamingLEDColor · 14/15 Set/GetGamingFanBehavior · 16/17 Set/GetGamingFanSpeed · 18/19 Set/GetGamingFanTable · 20/21 Set/GetGamingKBBacklight(u8[16]) · 22/23 Set/GetGamingMiscSetting · 24/25 Set/GetCPUOverclockingProfile(u8 + u8[512]) |
| **BatteryControl** | `79772EC5-04B1-4BFD-843C-61E7F77B6CC9` | `WMBE` | 19 GetBattInfoInterface(SBS index, battery→u32) · 20 GetBatteryHealthControlStatus · 21 SetBatteryHealthControl · 22/23 Get/SetBatteryFunctionData (adaptive charging, "BAC") |
| **APGeAction** | `61EF69EA-865C-4BC3-A502-A0DEBA0CB531` | `WMAA` | 1 SetFunction(u64→u32) · 2 GetFunction(u32→u64). This is the `WMID_GUID3` that `acer_wmi` uses for Wi-Fi/Bluetooth status on every Acer. |
| AcerGenericMethod | `1EA0BC57-521E-461B-A7A9-5D36F141A82A` | `WMAE` | Set/GetFunction, GetStorageInformation |
| UtilityFunction | `FE1DBBDA-3014-4856-870C-5B3A744BF341` | `WMBL` | passwords, BIOS settings, 11/12 Get/SetLedIndicator, diagnostics |
| BIOSSetting / AcerBiosConfigurationTool / MethodForPowerShell | `F75F5666…` / `4BB53443…` / `48E699DB…` | `WMBK`/`WMAC`/`WMBP` | BIOS settings (out of scope) |
| Events | `676AA15E…` (APGeEvent), `A9D77EF6…`, `2E6B3CE2…` | `_WED` | `EventDetail[8]` |

## 3. How the firmware implements it

- **Almost everything is SMM.** `WMBH` copies the input buffer to `WMIB`, the method id to `MTID` and triggers an SMI (port `PSSP`, value 0xD0). The fan, keyboard and settings logic lives in the firmware, not in AML — buffer formats can only be found by experiment.
- **Exception: sensors.** Method 5 (`GetGamingSysInfo`) calls `EC0.GGSI`: cmd 0 → mask of supported sensors (`0x0227` = ids 1, 2, 3, 6, 10), cmd 1 → reading (index in byte 1: 1 CPU temp `CTMP`, 2 CPU fan `CPUF`, 3 external temp `ETP2`, 6 GPU fan `GPUF`, 10 GPU temp `GTMP`), cmd 2 → AC status (`BTST`). This is the path `acer_wmi`'s hwmon uses.
- **The thermal profile also reprograms the GPU.** `SetGamingMiscSetting` with index 0x0B adjusts DPTF in AML (`ODV0`, `Notify(\_SB.IETM, 0x88)`) and the NVIDIA `NPCF` controller (`AMAT`/`ATPP`/`ACBT` from the tables `M1*` quiet, `M2*` balanced, `M3*` performance, `M5*` eco; `NVWM`=1 in quiet/eco). There is no table for Turbo (5) on this model.
- **Memory-mapped EC RAM** (`OperationRegion (ERAM, SystemMemory, 0xFE708500, 0x100)`): `KBBL` 0x48.0 (keyboard backlight on), `BTST` 0x48.5 (AC), `ETP2` 0x52, `FANT` 0x54 (4 bits), `CTMP` 0x58, `GTMP` 0x59, `CPUF` 0x5C (16 bits), `KBST` 0x69.1, `GPUF` 0x6A, `ACTP` 0x6D, `BTMD` 0x70, `NPWM` 0xF5. Direct access would require `ioremap`; prefer WMI.
- **Response convention:** little-endian buffers with **byte 0 = status** (0 ok, 1 not supported, 2 invalid parameter) and the value in the following bytes.

## 4. AcerGamingFunction — known formats

| Method | Input | Output / semantics |
|---|---|---|
| 5 GetGamingSysInfo | byte 0 = cmd (0/1/2), byte 1 = sensor index | cmd 0: mask in bytes 3-4; cmd 1: value in bytes 1-2 (temp °C; 16-bit RPM); cmd 2: AC in byte 5 |
| 23 GetGamingMiscSetting | byte 0 = index | byte 1 = value. Indexes that respond on this model: 0x07 (0xFF), 0x08 (1), **0x0A profile mask**, **0x0B current profile**. 1-6, 9, 12, 13, 15, 16 → not supported; 0, 14, 17+ → invalid. (0x05/0x07 = OC in `acer-wmi`; 0x06 = boot sound/animation in Linuwu — not supported here.) |
| 22 SetGamingMiscSetting | byte 0 = index, byte 1 = value | profile: 0 quiet, 1 balanced, 4 performance, 5 turbo, 6 eco |
| 15 GetGamingFanBehavior | u16 fan bitmap (**id 1 = bit 0, id 4 = bit 3**) | byte 1: 2-bit mode per fan index (bitmap 0x09 → `0x41` = both auto) |
| 14 SetGamingFanBehavior | u16 bitmap + 2-bit mode at `16 + 2·i` (1 auto, 2 max, 3 manual) | e.g. `0x410009` both auto, `0x820009` both max, `0xC30009` both manual |
| 17 GetGamingFanSpeed / 16 Set | byte 0 = fan id (1 CPU, 4 GPU), byte 1 = speed (`pct·256/100`) | only ids 1 and 4 respond |
| 19 GetGamingFanTable | — | not supported on this model (status 1) |
| 3 GetGamingProfile(0) | — | `00 00 00 01 00 00 ff 00`; in Linuwu byte 6 = 1 means LCD overdrive on → here 0xFF (not available) |
| 21 GetGamingKBBacklight, 7 GetGamingRgbKb | — | status 1 for any input: no RGB/zone keyboard API (single-color backlight) |
| 4/11/13 LEDs | — | status 1/2: no Turbo key LED, light bar or logo |
| 25 GetCPUOverclockingProfile | — | ReturnCode 1: no CPU OC |

## 5. APGeAction — decoding (validated with reversible writes)

Buffer: byte 0 = function, bytes 1-2 = selector/payload, byte 5 = value. Get = method 2 with u32; Set = method 1 with u64.

| Function | Get | Response | Set |
|---|---|---|---|
| Keyboard backlight | `01 84 08 00` (0x088401) | `00 [00] [08 = on] 00 00 [timeout in s]` | `02 84 08 00 00 [s]`: `0x088402` disables the timeout, `0x1E0000088402` = 30 s |
| USB charging while the laptop is off | `04 00 00 00` | `00 [0f on / 1f off] [limit %]` | `04 0f 1e` 30 %, `04 0f 14` 20 %, `04 0f 0a` 10 %, `04 1f 0a` off |
| AC adapter status | `0a 00 00 00` | `00 [01 on AC / 00 on battery]` (measured 2026-10-20: 1 → 0 → 1 unplugging and replugging the charger, matching `power_supply` `online`) | — |
| fn 3 / fn 7 | `03..` / `07 02..` | 0x11C0 / 0 | meaning unknown |

Bit 0x08 of the keyboard response is the same one `WMID.GOTS(0x84)` returns when `KBBL == 1`. SMM error codes: 0xE1 (not supported), 0xE2 (invalid parameter), 0xE4.

**Keyboard backlight presence comes from DMI, not from this call.** It answers status 0 on laptops without a keyboard backlight too: on the Aspire AG15-71P `00 00 00 00 32 1e 00 00` (bit 0x08 clear, byte 4 = 0x32 of unknown meaning, timeout 30 s), and its AML `GOTS` has no 0x84 case at all, only 0x83 (touchpad, `TOUP`). Bit 0x08 is most likely the on/off state rather than presence, since `GOTS` is also the payload of WMI event 0xBC (`_WED` → `EVBC`) sent on these toggles, so it cannot tell a missing backlight from one switched off. DMI OEM type 0xAA (the hotkey function table, `struct hotkey_function_type_aa` in `acer_wmi`) lists the functions present: after the 4-byte header, five u16 bitmaps (communication, application, media, display, others), then 4-byte entries `{ code, 2, u16 bit }` giving each function's bit. Code 0x84 is the keyboard backlight (the same code as its hotkey event and the APGe selector), 0x83 the touchpad, 0x61–0x64 display. Aspire (82 bytes, 17 entries): display `0x000B` with 61/62/63 present; others `0x0002`, so 83 is present and **84 → `0x0008` absent**. Nitro ANV15-52 (58 bytes, 11 entries, all present): others `0x100A` with 82 → `0x0002`, **84 → `0x0008`** and 8D → `0x1000`. The bit of a code differs between models (0x41 is `0x0001` on the Nitro, `0x0004` on the Aspire), so it is read from the entry, not assumed. `acer-wmi-extras` hides the timeout and state only when the table lists 0x84 as absent; without the table or the entry it keeps the WMI answer.

## 6. BatteryControl

- 20/21 (health control): `uFunctionList` bit 0 = 80 % limit, bit 1 = calibration; structures in `acer-wmi-battery`. The state lives in the EC and survives reboots.
- 19 `GetBattInfoInterface(index, battery=1)`: the index is the **Smart Battery (SBS) command**: 0x08 temperature (0.1 K), 0x09 voltage (mV), 0x0A current (mA, signed); 0x03 BatteryMode. Unsupported indexes return 1.
- 22/23 (adaptive charging): ReturnCode 1 on this model.

## 7. ANV15-52 capabilities (read from the firmware)

- Profiles: mask 0x0A = **0x53** → Quiet (0), Balanced (1), Performance (4), Eco (6). **No Turbo (5)**; consistent with the missing `M4*` table in `NPCF`.
- Fans: auto / max / manual per fan (ids 1 and 4); no curve.
- Keyboard: single color; configurable 30 s timeout (APGe).
- Off-state USB charging: 0/10/20/30 %.
- Battery: 80 % limit, calibration, voltage/current/temperature; no adaptive charging.
- None: RGB/zones, extra LEDs, CPU OC, fan table; boot sound and LCD overdrive have no known path.

### Validation of the `acer-wmi-extras` module (ANV15-52, 2026-10-12)

Detection at probe: `kbd timeout: yes, usb charging: yes, profiles: yes (mask 0x53), battery: yes`. Reads: timeout 30 s, backlight on, USB 30 %, profiles `low-power quiet balanced balanced-performance`; hwmon `acer_battery`: 25.6 °C, 16 440 mV, 0 mA. Writes: timeout 30→0→30, USB 30→20→30 with consistent reads; 15 % rejected with `-EINVAL`.

## 8. Mainline `acer-wmi`: facts and candidate patches

- **By kernel version** (checked in the torvalds tree, 2026-10-14): `predator_v4` and its `platform_profile` from **6.9** (absent in 6.8); fan control (`ACER_CAP_PWM`, hwmon `pwmN`) from **6.19** (absent in 6.18). Older kernels still load `acer_wmi` (hotkeys, rfkill) and our companion modules, which build from 5.15 on (`tools/modules-matrix.sh`). What that means per distribution: Debian 12 (6.1) and Ubuntu 22.04 (5.15) and 24.04 GA (6.8) have neither; Debian 13 (6.12) has the modes but not fan control; Ubuntu 24.04 HWE and 26.04 (7.0) and Fedora 44 (7.2) have both.
- `acer_predator_v4_platform_profile_probe` does `set_bit` on all 5 profiles without querying `ACER_WMID_MISC_SETTING_SUPPORTED_PROFILES` (0x0A), a constant that is defined but unused.
- Mapping: `performance`=Turbo 5, `balanced-performance`=Performance 4, `balanced`=1, `quiet`=0, `low-power`=Eco 6.
- The driver ignores `WMID_AC_EVENT`; `platform_profile_cycle()` goes through every choice (Eco on AC, Turbo on battery). The AC/battery rule belongs in user space.
- Upstream candidates: (1) DMI entry `Nitro ANV15-52` → `predator_v4` + `pwm`; (2) profile probe reading 0x0A; (3) battery hook `charge_control_end_threshold`; (4) **serialize WMI calls** (a mutex in `acer_wmi` around `WMI_gaming_execute_u64`/`WMID_gaming_*`, or better, in the WMI core per `PNP0C14` device), because concurrent calls to different methods corrupt the responses (§9). No brand flags: the variation is per model.
- Watch out for `tuned`/`tuned-ppd`: `throughput-performance` writes `performance` (= Turbo); keep `balanced` on models without Turbo.

## 9. Coexistence rules for out-of-tree modules

- Use only the legacy GUID-based API (`wmi_evaluate_method`, `wmi_has_guid`; `EXPORT_SYMBOL_GPL` → GPL module). The 7.2 WMI core does not prevent several modules from calling the same GUID; `acer_wmi` uses the same API and does not bind a driver to the gaming GUID. Exception: `acer-wmi-battery` binds to `79772EC5…` (no other driver uses it) only to get a `struct device` for the `power_supply` extension; the calls still go through the legacy API.
- Do not bind to the event GUID `676AA15E…` or install a legacy handler on it (already taken by `acer_wmi`, `AE_ALREADY_ACQUIRED`).
- Serialize your own calls (mutex): there is no lock between modules when invoking `WMAB`/`WMBH`.
- **Measured (2026-10-12): concurrent WMI calls corrupt the responses.** The AML methods (`WMAA`, `WMAB`, `WMAE`, `WMBE`, `WMBH`) are `Serialized`, which only prevents two executions of the *same* method; different methods run in parallel and share the SMM path. Reading `platform-profile` every 5 ms while another process read `kbd_backlight_state` (APGe, `WMAA`), 30 of 578 reads came back wrong (`quiet` = value 0, or `-EOPNOTSUPP` for a value outside the table); with `pwm1` (`WMBH`, same method) alongside, 8 of 654; with no explicit concurrency, 2 of 333 (collisions with the daemon). Consequences: (a) the daemon serializes all its sysfs accesses with a mutex and only accepts as an "external change" a known value read twice; (b) any external reader (`sensors`, `cat` on sysfs) may get garbage until patch (4) of §8; (c) `acer-wmi-extras` already has its own mutex, but it does not cover `acer_wmi`.
- Secure Boot: sign the `.ko` files with the akmods MOK; lockdown `integrity` blocks writable debugfs → interfaces in sysfs.

## 10. Related projects

- [Linuwu-Sense](https://github.com/0x7375646F/Linuwu-Sense) (GPL-3): replaces `acer_wmi`; reference for constants (fans, APGe, 4-zone RGB, battery).
- [acer-wmi-battery](https://github.com/frederik-h/acer-wmi-battery): standalone battery module (vendored in `drivers/`).
- [Div-Acer-Manager-Max](https://github.com/PXDiv/Div-Acer-Manager-Max) (GPL-3): Avalonia GUI + Python daemon on top of Linuwu.
- [wobqqq/predator-sense](https://github.com/wobqqq/predator-sense): CLI + DKMS on top of Linuwu.
- [cleyton1986/predator-sense](https://github.com/cleyton1986/predator-sense) (GPL-3.0): Rust GUI + `facer` module; its `PH16-71-FINDINGS.md` is the main source of §11.
- [nekro-sense](https://github.com/fredac100/nekro-sense) (GPL-3.0): Linuwu fork for the PHN16-72 (4-zone keyboard, logo).

## 11. Predator lighting (public sources; not verified here)

No Predator has been tested by this project. The simulated Predators (`acerd --simulate PHN16-72` or `PH16-71`) reproduce what these public projects report:

- [cleyton1986/predator-sense](https://github.com/cleyton1986/predator-sense) (GPL-3.0), `PH16-71-FINDINGS.md`, measured on a **Predator PH16-71** (sections cited as PS §n);
- [Linuwu-Sense](https://github.com/0x7375646F/Linuwu-Sense) `four_zone_mode` / `per_zone_mode`, and [nekro-sense](https://github.com/fredac100/nekro-sense) (GPL-3.0, its fork for the **PHN16-72**: 4-zone keyboard and RGB logo).

| Method | Layout | Notes |
|---|---|---|
| 20 SetGamingKBBacklight (u8[16]) | 0 effect, 1 speed, 2 brightness 0–100, 4 direction, 5–7 R G B, **9 target**: 1 keyboard, 2 light bar (logo on the PHN16-72) | PH16-71 (PS §4, §6): bytes 3, 8 and 10–15 are ignored; effects 0x08–0xFE are a silent no-op; brightness 0 is dark |
| 21 GetGamingKBBacklight | argument = target; answer: [0] status, [1] effect, [2] speed, [3] brightness, [5] direction, [6..8] R G B | keyboard and light bar keep separate states (PS §6); status 1 on the ANV15-52 (§4) |
| 6 SetGamingRgbKb (u64) / 7 Get | `[zone, R, G, B]`, zone 1/2/4/8 | Linuwu `per_zone_mode`: one colour per zone, brightness through method 20 static. PH16-71: stored and read back, but nothing lights (its keyboard is USB; PS §6, §9) |

- **Effects** (byte 0, Linuwu-Sense): 0 static, 1 breathing, 2 neon, 3 wave, 4 shifting, 5 zoom, 6 meteor, 7 twinkling. Speed 1–9 on the keyboard (unused by static), direction 1 = right to left, 2 = left to right; neon and wave carry no colour.
- **Light bar, PH16-71** (PS §4, §10): speed 1–5, brightness 0–100; effect 0 is a solid colour, not off. After boot or after being dark it stays off until **one breathing frame** is sent, although the readback already shows the new state. The bar has three physical zones but no per-zone path is known (PS §9, §11).
- **USB keyboard, PH16-71** (Chicony `04F2:0117`, PS §7): SET_REPORT session `B1 00 00 00 00 00 00 4E`, `14 00 00 RR GG BB 00 00`, `08 02 <effect> <speed 1-9> <brightness 0-0x32> 01 <dir> <checksum>`; 13 effects plus off (predator-sense `keyboard_rgb.rs`: only wave and rainbow ignore the colour, only wave takes a direction, static has no speed); **no readback**; its own ~30 s idle timer (PS §24), which the keyboard timeout of §5 does not reach.
- **Boot and resume** (PS §20, §22): the PH16-71 loses its lighting; `acerd` reapplies the last effect it set (`state.json`).
- **Keys** (PS §14, §15, §25): the mode key is WMI event 0x7 / key 5, cycled by `acer_wmi` with `predator_v4` (the profile indices are those of §4: 0, 1, 4, 5, 6). The PredatorSense key arrives as HID usage `0xFF81` on the keyboard's interface 2, with no evdev key: `acerd`'s Sense key detection (`KEY_PROG1`) does not see it yet.

How `acerd` maps this (`daemon/internal/hw/light.go`): devices `keyboard`, `lightbar`, `logo`; effect names as above, plus the USB keyboard's (`snake`, `ripple`, `rainbow`, `rain`, `lightning`, `spot`, `stars`, `fireball`, `snow`, `heartbeat`); brightness always in percent (scaling to 0–0x32 is a backend's job); direction `left` = 1, `right` = 2. There is no real backend yet: one needs a module or hidraw path tested on the hardware.
