# acerd

Go daemon that exposes `io.github.fnndk.Acerd1` on the system bus and applies the rules of `docs/api-dbus.md` on top of `platform-profile`, hwmon `acer`, `acer-wmi-extras`, `acer-wmi-battery` and `power_supply`. Clients (GUI, CLI, tray) only speak D-Bus.

## Layout

```
cmd/acerd/      main: flags, probe or simulation, stop signal
cmd/acerctl/   CLI: status, caps, monitor, mode, fan, kbd, battery, usb-charging,
                      scenario, appsync, setup, reload (--json on every read)
client/               public D-Bus client (CLI now, GUI/tray later): constants,
                      typed errors (NotAllowed etc.), Monitor Subscribe
internal/hw/          sysfs read/write backends (real.go) and simulated (sim.go);
                      Capabilities() is built only from what was detected
internal/rules/       pure, tested policy: Allowed/Extras, Cycle, PPD mapping
internal/service/     D-Bus object: properties (godbus/prop), methods, polkit,
                      watchers (sysfs, AC/battery, PPD), Monitor with unicast signals
internal/monitor/     CPU/GPU/fans/RAM/network/battery sample
internal/ppd/         net.hadess.PowerProfiles client (tuned-ppd)
internal/state/       /var/lib/acerd/state.json (mode per source, fans, active scenario,
                      App Sync and snapshot to revert)
internal/scenarios/   scenarios: format, validation, scenarios.json, app index
internal/appsync/     proc connector (exec/exit), matching by comm/argv, active user
```

## Developing without root

```bash
cd daemon && go test ./... && go build ./cmd/acerd
./acerd --session --simulate ANV15-52 --state /tmp/ns-state.json   # no hardware
./acerd --session --dry-run --state /tmp/ns-state.json             # real hardware, read-only
./acerd --session --root internal/hw/testdata/models/AG15-71P/root --state /tmp/ns-state.json   # a known model's fixture
busctl --user introspect io.github.fnndk.Acerd1 /io/github/fnndk/Acerd1
busctl --user call io.github.fnndk.Acerd1 /io/github/fnndk/Acerd1 io.github.fnndk.Acerd1.Mode Set s quiet
```

`--session` uses the session bus and disables polkit; `--dry-run` logs the writes instead of performing them; `--root DIR` reads and writes the kernel interfaces under a fixture tree instead of `/` (writes go into that tree, so use a copy; no GPU, UPower or Sense key device there). Without PPD on the session bus, syncing is disabled (it shows up in the log).

## Known models

`internal/hw/testdata/models` holds a fixture per known model (Nitro ANV15-52, Aspire AG15-71P, Predator PH16-71; its README says where each comes from). `go test ./internal/hw -run TestModels` runs the hardware layer on each; `sh tools/models-test.sh DIST` runs the installed packages on each in Fedora 44 and Ubuntu 24.04 containers (6 combinations, part of `tools/ci.sh` and the release workflow). After an intended change, `sh tools/models-test.sh --update` rewrites the expected results; review the diff before committing.

## CLI

```bash
acerctl                       # status
acerctl monitor -i 1000       # live sensors
acerctl mode quiet            # eco|low-power, quiet, balanced, balanced-performance, turbo|performance
acerctl fan gpu 60            # manual; auto|max too; id all
acerctl scenario save Game --from-current --mode-ac balanced-performance --fan cpu=max --app eldenring.exe
acerctl appsync on
acerctl setup                 # what is left: acer_wmi option, Secure Boot key, modules
acerctl setup apply           # make it (one password), then restart
acerctl --json status         # for scripts
```

The names are the `platform_profile` ones; Acer's "Performance" mode is `balanced-performance` and "Turbo" is `performance`. Daemon refusals exit with the daemon's reason and code 1; usage errors exit with code 2. `--session` talks to a development daemon on the session bus.

## Installing on the host (Silverblue)

```bash
sh packaging/install-daemon.sh build                      # in the container (has Go)
distrobox-host-exec sudo sh packaging/install-daemon.sh   # on the host (has root)
```

Installs `acerd` and `acerctl` into `/usr/local/bin`, `/etc/dbus-1/system.d`, `/usr/local/share/polkit-1/actions` and `/etc/systemd/system`, and enables `acerd.service` (ordered after `acer-wmi-modules.service` and tuned).

## Still missing (see STATUS.md)

App Sync tested with a real Proton game, integration tests with tuned-ppd.
