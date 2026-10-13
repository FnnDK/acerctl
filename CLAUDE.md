# acerctl — notes for the assistant

Project: Acer NitroSense/PredatorSense features on Linux, coexisting with the in-tree `acer_wmi`. **Read `STATUS.md` first** (state, done, next step, backlog, commands), then the sections of `docs/firmware-wmi.md` it cites.

Environment:
- The shell runs in a Debian distrobox (`deb`); the host is Fedora Silverblue. Host commands: `distrobox-host-exec ...`; root on the host needs the user's password → hand them `distrobox-host-exec sudo ...` commands to run in their terminal.
- Secure Boot is on: sign `.ko` files with the akmods MOK (command in `STATUS.md`). Lockdown blocks writable debugfs → use sysfs.
- Build modules on the host: `distrobox-host-exec make -C drivers/<module>`.
- The above is the Nitro ANV15-52 (reference machine). On the **Aspire AG15-71P** (Bluefin 44): Secure Boot is off (no signing), the host has no `kernel-devel` → build in a Fedora toolbox with `make -C drivers/<module> KDIR=/usr/src/kernels/$(uname -r)` and `insmod` on the host. Check `/sys/class/dmi/id/product_name` to know which machine the session is on.

Project rules:
- **English only in the repository.** Everything tracked by git is written in English: code, identifiers, comments, log and error messages, docs (`README.md`, `STATUS.md`, `docs/`, per-directory READMEs), scripts, packaging files and commit messages. The only exception is user-facing translations: the GUI (`gui/frontend/src/i18n.ts`) and the tray (`daemon/cmd/acer-tray`) keep their Portuguese texts, and desktop/polkit files may carry `[pt_BR]` translations next to the English strings. Conversation with the user can stay in Portuguese.
- Do not replace `acer_wmi`; no brand flags (capabilities come from the firmware); probes only with "Get" methods; "Set" only with a validated, reversible format; keep `tuned` on `balanced`.
- Update `STATUS.md` at the end of each block of work. Commit only when the user asks.
