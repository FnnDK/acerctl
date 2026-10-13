# nitrosensed

Daemon Go que expõe `org.nitrosense.Daemon1` no system bus e aplica as regras de `docs/api-dbus.md` sobre `platform-profile`, hwmon `acer`, `acer-wmi-extras`, `acer-wmi-battery` e `power_supply`. Clientes (GUI, CLI, tray) só falam D-Bus.

## Layout

```
cmd/nitrosensed/      main: flags, probe ou simulação, sinal de parada
internal/hw/          backends de leitura/escrita em sysfs (real.go) e simulado (sim.go);
                      Capabilities() é montado só do que foi detectado
internal/rules/       política pura e testada: Allowed/Extras, Cycle, mapeamento PPD
internal/service/     objeto D-Bus: propriedades (godbus/prop), métodos, polkit,
                      watchers (sysfs, AC/bateria, PPD), Monitor com sinais unicast
internal/monitor/     amostra de CPU/GPU/fans/RAM/rede/bateria
internal/ppd/         cliente de net.hadess.PowerProfiles (tuned-ppd)
internal/state/       /var/lib/nitrosense/state.json (modo por fonte, fans)
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

## Instalar no host (Silverblue)

```bash
sh packaging/install-daemon.sh build                      # no container (tem Go)
distrobox-host-exec sudo sh packaging/install-daemon.sh   # no host (tem root)
```

Instala em `/usr/local/bin`, `/etc/dbus-1/system.d`, `/usr/local/share/polkit-1/actions` e `/etc/systemd/system`, e habilita `nitrosensed.service` (ordenado depois de `acer-wmi-modules.service` e do tuned).

## O que ainda falta (ver STATUS.md)

CLI `nitrosense-cli`, App Sync testado com jogo Proton real, cadência dos fans conforme assinantes, testes de integração com tuned-ppd.
