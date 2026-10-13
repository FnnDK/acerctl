# nitrosensed

Daemon Go que expõe `org.nitrosense.Daemon1` no system bus e aplica as regras de `docs/api-dbus.md` sobre `platform-profile`, hwmon `acer`, `acer-wmi-extras`, `acer-wmi-battery` e `power_supply`. Clientes (GUI, CLI, tray) só falam D-Bus.

## Layout

```
cmd/nitrosensed/      main: flags, probe ou simulação, sinal de parada
cmd/nitrosense-cli/   CLI: status, caps, monitor, mode, fan, kbd, battery, usb-charging,
                      scenario, appsync, reload (--json em todas as leituras)
client/               cliente D-Bus público (CLI agora, GUI/tray depois): constantes,
                      erros tipados (NotAllowed etc.), Subscribe do Monitor
internal/hw/          backends de leitura/escrita em sysfs (real.go) e simulado (sim.go);
                      Capabilities() é montado só do que foi detectado
internal/rules/       política pura e testada: Allowed/Extras, Cycle, mapeamento PPD
internal/service/     objeto D-Bus: propriedades (godbus/prop), métodos, polkit,
                      watchers (sysfs, AC/bateria, PPD), Monitor com sinais unicast
internal/monitor/     amostra de CPU/GPU/fans/RAM/rede/bateria
internal/ppd/         cliente de net.hadess.PowerProfiles (tuned-ppd)
internal/state/       /var/lib/nitrosense/state.json (modo por fonte, fans, cenário ativo,
                      App Sync e retrato para reverter)
internal/scenarios/   cenários: formato, validação, scenarios.json, índice de apps
internal/appsync/     proc connector (exec/exit), casamento por comm/argv, usuário ativo
```

## Desenvolver sem root

```bash
cd daemon && go test ./... && go build ./cmd/nitrosensed
./nitrosensed --session --simulate ANV15-52 --state /tmp/ns-state.json   # sem hardware
./nitrosensed --session --dry-run --state /tmp/ns-state.json             # hardware real, só leitura
busctl --user introspect org.nitrosense.Daemon1 /org/nitrosense/Daemon1
busctl --user call org.nitrosense.Daemon1 /org/nitrosense/Daemon1 org.nitrosense.Daemon1.Mode Set s quiet
```

`--session` usa o bus de sessão e desliga o polkit; `--dry-run` loga as escritas em vez de fazê-las. Sem PPD no bus de sessão a sincronização fica desligada (aparece no log).

## CLI

```bash
nitrosense-cli                       # status
nitrosense-cli monitor -i 1000       # sensores ao vivo
nitrosense-cli mode quiet            # eco|low-power, quiet, balanced, balanced-performance, turbo|performance
nitrosense-cli fan gpu 60            # manual; auto|max também; id all
nitrosense-cli scenario save Jogo --from-current --mode-ac balanced-performance --fan cpu=max --app eldenring.exe
nitrosense-cli appsync on
nitrosense-cli --json status         # para scripts
```

Os nomes são os do `platform_profile`; o modo "Performance" da Acer é `balanced-performance` e o "Turbo" é `performance`. Recusas do daemon saem com o motivo dele e código 1; erro de uso sai com código 2. `--session` fala com um daemon de desenvolvimento no bus de sessão.

## Instalar no host (Silverblue)

```bash
sh packaging/install-daemon.sh build                      # no container (tem Go)
distrobox-host-exec sudo sh packaging/install-daemon.sh   # no host (tem root)
```

Instala `nitrosensed` e `nitrosense-cli` em `/usr/local/bin`, `/etc/dbus-1/system.d`, `/usr/local/share/polkit-1/actions` e `/etc/systemd/system`, e habilita `nitrosensed.service` (ordenado depois de `acer-wmi-modules.service` e do tuned).

## O que ainda falta (ver STATUS.md)

App Sync testado com jogo Proton real, cadência dos fans conforme assinantes, testes de integração com tuned-ppd.
