# API D-Bus do daemon `nitrosensed` (rascunho v1)

Revisado em 2026-10-12 a partir do v0, contra `docs/firmware-wmi.md` (§1, §4–§8) e os atributos reais de `acer_wmi`, `acer-wmi-extras` e `acer-wmi-battery` no ANV15-52. Mudanças em relação ao v0 e pontos em aberto no fim do arquivo.

## 1. Visão geral

- **Bus:** system bus. Nome `org.nitrosense.Daemon1`, objeto `/org/nitrosense/Daemon1`, um objeto só com várias interfaces.
- **Leituras são propriedades** D-Bus (`org.freedesktop.DBus.Properties`, com `PropertiesChanged`), para que `busctl`, GUI, CLI e tray usem o mecanismo padrão sem código extra. **Escritas são métodos**, porque precisam validar, aplicar regras e devolver erro com motivo.
- **Capacidades vêm do firmware** (§3 abaixo). Toda interface expõe só o que `Capabilities` listar; o cliente nunca assume uma função pela marca ou modelo. Uma chamada a algo não listado devolve `NotSupported`.
- **Autorização** por polkit (§9). O daemon roda como root (precisa escrever em sysfs e ler evdev/RAPL); nunca executa nada vindo do cliente além dos valores validados.
- Todos os valores numéricos usam unidades naturais em tipos simples: temperatura `d` (°C), frequência `u` (MHz), potência `d` (W), tensão `u` (mV), corrente `i` (mA), RPM `u`, percentuais `u` (0–100), bytes `t`.

### Erros

| Nome | Quando |
|---|---|
| `org.nitrosense.Daemon1.Error.NotSupported` | função ausente em `Capabilities` neste hardware |
| `org.nitrosense.Daemon1.Error.NotAllowed` | regra do daemon (ex.: `performance` na bateria); a mensagem traz o motivo em texto para a GUI exibir |
| `org.nitrosense.Daemon1.Error.InvalidArgument` | valor fora do conjunto aceito (ex.: USB 15 %) |
| `org.nitrosense.Daemon1.Error.Hardware` | sysfs/WMI devolveu erro; mensagem com o `errno` |
| `org.freedesktop.DBus.Error.AccessDenied` | polkit negou |

### Fontes de cada dado (host)

| Dado | Fonte | Observação |
|---|---|---|
| Modo (perfil térmico) | `/sys/class/platform-profile/platform-profile-0/profile` (`acer_wmi predator_v4`) | mudanças externas detectadas por `poll()` no atributo (`sysfs_notify`) |
| Perfis suportados | `/sys/devices/platform/acer-wmi-extras/supported_profiles` | o `choices` do `platform-profile` lista os 5 fixos do driver e **não** serve (§8 do firmware-wmi) |
| RPM, temperaturas CPU/GPU/externa | hwmon `acer` (`fan1`, `fan2`, `temp1..3`) | GPU lê 0 com a dGPU desligada |
| Modo e velocidade dos fans | hwmon `acer` `pwmN_enable` (0 máx, 1 manual, 2 auto) e `pwmN` (0–255) | só existem com a quirk `.pwm` ou `force_caps` |
| Timeout e estado do backlight | `acer-wmi-extras/kbd_backlight_timeout` (RW, só 0 ou 30), `kbd_backlight_state` (RO) | ligar/desligar é pela tecla Fn (`acer_wmi`); não há Set de estado |
| USB charging | `acer-wmi-extras/usb_charging` (0/10/20/30) | — |
| Limite 80 %, calibração | `/sys/bus/wmi/drivers/acer-wmi-battery/{health_mode,calibration_mode}` | atributos de driver, não de device; se um dia `BAT1/charge_control_end_threshold` existir (patch upstream), preferir ele |
| Temperatura, tensão, corrente da bateria | hwmon `acer_battery` (`temp1`, `in0`, `curr1`) | — |
| Carga, status, ciclos, capacidade | `/sys/class/power_supply/BAT1` (`capacity`, `status`, `cycle_count`, `energy_full`, `energy_full_design`) | fonte de `health` |
| AC | `/sys/class/power_supply/ACAD/online` + udev (`power_supply` uevent) | sem depender do UPower |
| GPU (temp, uso, clocks, potência) | NVML (go-nvml); fallback `nvidia-smi --query-gpu` | ausente ⇒ `gpu.nvml=false` e campos omitidos |
| CPU (uso, freq, potência) | `/proc/stat`, `/sys/devices/system/cpu/*/cpufreq/scaling_cur_freq`, RAPL `/sys/class/powercap/intel-rapl:0/energy_uj` (só root) | — |
| RAM, rede | `/proc/meminfo`, `/proc/net/dev` (soma das interfaces físicas) | — |
| Tecla Nitro | evdev `KEY_PROG1` (148) no teclado interno (`AT Translated Set 2 keyboard`) | sem `EVIOCGRAB`: o desktop continua recebendo a tecla |

## 2. Interface `org.nitrosense.Daemon1.System`

| Membro | Tipo | Descrição |
|---|---|---|
| prop `Version` | `s` | versão do daemon |
| prop `Capabilities` | `a{sv}` | ver §3; muda só em recarga de módulos (sinal `PropertiesChanged`) |
| prop `Model` | `a{sv}` | `vendor`, `product`, `bios` (DMI), `kernel`, `modules: a{ss}` (nome → versão/“absent”), `acer_wmi_caps: u` (`force_caps` efetivo, se legível) |
| método `Reload()` | | reprobe de módulos e capacidades (após `insmod`/update); polkit `control` |

## 3. Capacidades (`System.Capabilities`)

Cada chave só aparece quando o backend foi detectado. Detecção é por **presença e leitura com sucesso** do arquivo, nunca por DMI.

| Chave | Tipo | Detecção | ANV15-52 |
|---|---|---|---|
| `profiles` | `as` | `acer-wmi-extras/supported_profiles`; se o módulo faltar, `platform-profile/choices` **com** `profiles_from_firmware=false` | `low-power quiet balanced balanced-performance` |
| `profiles_from_firmware` | `b` | ver acima | `true` |
| `fans` | `a{sv}` | hwmon `acer` com `pwmN_enable`; `ids: as` (`cpu`, `gpu`), `modes: as` (`auto max manual`), `range: a{s(uu)}` (mín/máx em % por fan; 0–100 quando não há tabela por modelo), `curve: b` (sempre `false` até existir módulo para `GetGamingFanTable`) | `cpu gpu`, `0–100`, sem curva |
| `kbd_backlight` | `a{sv}` | `acer-wmi-extras/kbd_backlight_timeout`; `timeouts: au` (`0 30`), `rgb: b`, `zones: u` | timeout, `rgb=false` |
| `usb_charging` | `au` | `acer-wmi-extras/usb_charging`; valores aceitos | `0 10 20 30` |
| `battery` | `a{sv}` | `charge_limit: b` (`health_mode` ≥ 0), `calibration: b` (`calibration_mode` ≥ 0), `sensors: b` (hwmon `acer_battery`) | tudo `true` |
| `gpu` | `a{sv}` | `nvml: b` (NVML inicializou), `oc: b` (sempre `false` na v1) | `nvml=true` |
| `cpu_power_limit` | `b` | RAPL gravável; `false` na v1 | `false` |
| `nitro_key` | `b` | evdev encontrou o teclado interno | `true` |

Sem Turbo (`performance`) em `profiles` ⇒ a GUI nem mostra o botão. Isso vale para qualquer modelo: a lista é a do firmware.

**Convenção para funções extras.** Tudo o que o daemon permite além do comportamento do app original (hoje só Performance na bateria) é exposto separadamente (`Mode.Extras`) para a GUI marcar como extra, nunca misturado em silêncio com o comportamento 1:1.

## 4. Interface `org.nitrosense.Daemon1.Mode`

Nomes são os do `platform_profile` (o que o daemon escreve). Mapeamento Acer: `quiet`=Quiet, `balanced`=Balanced, `balanced-performance`=Performance, `performance`=Turbo, `low-power`=Eco.

| Membro | Tipo | Descrição |
|---|---|---|
| prop `Current` | `s` | perfil atual; reflete também mudanças externas (tuned, outro cliente) |
| prop `PowerSource` | `s` | `ac` / `battery` |
| prop `BatteryPercent` | `u` | — |
| prop `Allowed` | `as` | perfis permitidos **agora** (capacidades ∩ regras de energia); é o que a GUI habilita |
| prop `Extras` | `as` | itens de `Allowed` que só existem por regra estendida do daemon (hoje: `balanced-performance` na bateria); a GUI sinaliza como função extra |
| método `Set(s profile)` | | `NotSupported` se fora de `profiles`; `NotAllowed` se fora de `Allowed` |
| método `Cycle()` | `→ s` | próximo em `Allowed`, na ordem de `profiles`; usado pela tecla Nitro |

**Regras de energia** (do daemon, aplicadas em `Set`, `Cycle` e em toda troca AC↔bateria):

- Na tomada: `quiet`, `balanced`, `balanced-performance`; `performance` (Turbo) só com carga ≥ 40 %.
- Na bateria: `balanced` e `low-power`; **extra (além do app original):** `balanced-performance` também, com carga ≥ 40 %. Ao cair abaixo de 40 % o daemon volta para `balanced` e emite `PropertiesChanged`.
- `low-power` nunca na tomada.
- O daemon guarda o último perfil escolhido **por fonte** (`mode_ac`, `mode_battery`) e reaplica na troca. Se o perfil atual ficar proibido (ex.: Turbo e o cabo sai), troca para o guardado da outra fonte, ou `balanced`.

## 5. Interface `org.nitrosense.Daemon1.Fans`

| Membro | Tipo | Descrição |
|---|---|---|
| prop `State` | `a{sa{sv}}` | por id (`cpu`, `gpu`): `rpm: u`, `mode: s` (`auto`/`max`/`manual`), `speed: u` (%; só significativo em `manual`) |
| método `SetMode(s fan, s mode)` | | `fan` = id ou `all`. `max` ⇒ `pwmN_enable=0`; `auto` ⇒ 2; `manual` ⇒ 1 mantendo o `pwmN` atual |
| método `SetSpeed(s fan, u percent)` | | grava `pwmN = percent·255/100`; muda para `manual` se preciso. Sem piso fixo no daemon: aceita a faixa de `capabilities.fans.range` (0–100 por padrão) e o firmware aplica o mínimo real da ventoinha; fora da faixa ⇒ `InvalidArgument` |

`State.rpm` é atualizado a cada amostra do Monitor (§6); `mode`/`speed` são lidos após cada escrita e a cada 5 s (outro processo pode escrever no hwmon).

## 6. Interface `org.nitrosense.Daemon1.Monitor`

| Membro | Tipo | Descrição |
|---|---|---|
| método `Get()` | `→ a{sv}` | uma amostra, formato abaixo |
| método `Subscribe(u interval_ms)` | | registra o chamador (nome único do sender); mínimo 500 ms |
| método `Unsubscribe()` | | — |
| sinal `Sample` | `a{sv}` | emitido **a cada assinante** (sinal unicast, `destination` = sender) no intervalo pedido; o daemon amostra no menor intervalo ativo. Assinantes que saem do bus (`NameOwnerChanged`) são removidos |

Formato da amostra (chaves ausentes quando o backend não existe):

```
cpu:     {temp: d, usage: u, freq: u, power: d}
gpu:     {temp: d, usage: u, clock: u, mem_clock: u, power: d}
fans:    {cpu: u, gpu: u}                     # RPM, mesma leitura de Fans.State
ram:     {used: t, total: t}
net:     {rx: t, tx: t}                       # bytes/s desde a amostra anterior
battery: {percent: u, status: s, voltage: u, current: i, temp: d}
ts:      t                                    # CLOCK_MONOTONIC em ms
```

Sem assinantes o daemon **não** amostra (NVML e RAPL custam CPU).

## 7. Interface `org.nitrosense.Daemon1.Lighting`

| Membro | Tipo | Descrição |
|---|---|---|
| prop `Keyboard` | `a{sv}` | `on: b` (RO, estado do firmware), `timeout_s: u`; em modelos RGB (futuro): `zones`, `effect`, `brightness`, `speed` |
| método `SetKeyboardTimeout(u seconds)` | | só valores de `capabilities.kbd_backlight.timeouts` (hoje 0 ou 30); outros ⇒ `InvalidArgument` |
| método `SetKeyboardZones(...)` / `SetKeyboardEffect(...)` | | assinatura a definir quando houver módulo RGB; `NotSupported` até lá |

`on` muda pela tecla Fn; o daemon relê `kbd_backlight_state` a cada 2 s enquanto a GUI está na tela de iluminação (o cliente chama `Subscribe` do Monitor) e emite `PropertiesChanged`.

## 8. Interface `org.nitrosense.Daemon1.Battery`

| Membro | Tipo | Descrição |
|---|---|---|
| prop `ChargeLimit` | `b` | limite de 80 % (`health_mode`); o estado vive no EC e **sobrevive a reboot** — o daemon lê, não restaura |
| prop `Calibrating` | `b` | `calibration_mode` |
| prop `UsbCharging` | `u` | 0 (desligado) / 10 / 20 / 30 |
| prop `Health` | `a{sv}` | `cycles: u`, `full: t`, `design: t` (µWh), `health_pct: u` (= full/design), `temp: d` |
| método `SetChargeLimit(b)` | | polkit `settings` |
| método `StartCalibration()` | | `NotAllowed` sem AC; polkit `settings`. Não há evento de fim na API legada (o `acer_wmi` ocupa o handler): o daemon faz **polling** de `calibration_mode` a cada 30 s enquanto `Calibrating` e emite `PropertiesChanged` ao terminar |
| método `StopCalibration()` | | — |
| método `SetUsbCharging(u percent)` | | só valores de `capabilities.usb_charging`; polkit `settings` |

## 9. Interface `org.nitrosense.Daemon1.Scenarios`

Renomeada de `Profiles` para não colidir com os perfis térmicos (`platform_profile`).

| Membro | Tipo | Descrição |
|---|---|---|
| prop `List` | `aa{sv}` | cenários salvos |
| prop `Active` | `s` | nome ou `""` |
| prop `AppSync` | `b` | watcher ligado |
| método `Apply(s name)` | | aplica cada campo pelos métodos acima (mesmas regras e erros) |
| método `Save(a{sv})` | | cria/substitui pelo `name` |
| método `Delete(s name)` | | — |
| método `SetAppSync(b)` | | — |

Formato de um cenário (todas as chaves opcionais exceto `name`):

```
name: s
mode_ac: s            mode_battery: s
fans: a{sa{sv}}       # id → {mode, speed}
kbd_timeout_s: u
apps: as              # basenames de executáveis (App Sync)
```

Armazenamento em `/var/lib/nitrosense/scenarios.json` (global ao sistema, já que o daemon é do sistema). App Sync: `proc` connector (netlink, o daemon é root) para `exec`/`exit`; casa `comm`/basename do exe com `apps`, só para processos do usuário da sessão ativa (logind). Ao sair o último processo casado, volta ao cenário anterior.

## 10. Autorização (polkit)

Duas ações, em `/usr/share/polkit-1/actions/org.nitrosense.daemon.policy`:

| Ação | Cobre | Padrão |
|---|---|---|
| `org.nitrosense.daemon.control` | `Mode.Set/Cycle`, `Fans.*`, `Lighting.*`, `Scenarios.*`, `System.Reload` | `allow_active=yes`, `allow_inactive=no`, `allow_any=auth_admin` |
| `org.nitrosense.daemon.settings` | `Battery.SetChargeLimit`, `Start/StopCalibration`, `SetUsbCharging` | `allow_active=auth_admin_keep`, demais `auth_admin` |

Motivo da divisão: trocar modo e fan é ação corriqueira (a tecla Nitro faz isso sem senha); limite de carga, calibração e USB charging persistem no EC e afetam a máquina desligada. Leituras e `Monitor.Subscribe` não passam por polkit. Política D-Bus em `/usr/share/dbus-1/system.d/org.nitrosense.Daemon1.conf`: só root possui o nome; qualquer usuário pode enviar.

## 11. Comportamento do daemon

- **Boot/resume.** Restaura só o que não vive no EC: perfil térmico (por fonte de energia) e o modo dos fans. Timeout, USB charging e limite de carga ficam no EC e são apenas lidos. Fans em `manual` **não** são restaurados no boot (risco de ventoinha baixa sob carga); voltam a `auto` e o daemon registra no log. No resume, reaplica o perfil e relê tudo. Estado em `/var/lib/nitrosense/state.json`.
- **AC/bateria.** udev em `power_supply`; a cada evento recalcula `Allowed`, reaplica o perfil guardado da fonte e emite `PropertiesChanged` em `Mode`.
- **Tecla Nitro.** evdev `KEY_PROG1` do teclado interno (sem grab) → `Mode.Cycle()` internamente, sem polkit. A GUI pode registrar um atalho adicional via desktop.
- **Convivência com `tuned-ppd`/`power-profiles-daemon` (decidido 2026-10-12).** O daemon escreve **direto** em `platform_profile` e mantém o PPD sincronizado nos dois sentidos. A seleção completa (5 modos) fica no nosso app; o PPD só tem 3 perfis, então o mapeamento é muitos-para-um na ida e resolvido pela fonte de energia na volta.

  Fatos do tuned-ppd 2.28 (Fedora 44): PPD `power-saver` → tuned `powersave` → `platform_profile=low-power|quiet` (o primeiro que existir em `choices`); `balanced` → `balanced`; `performance` → `throughput-performance` → `performance` (= Turbo) e ainda governor etc. Na bateria o PPD `balanced` vira `balanced-battery`, sem seção `[acpi]`; ao trocar de perfil o tuned **restaura** o valor lido ao aplicar o anterior. Com `sysfs_acpi_monitor=true` ele observa o arquivo por inotify e, se outro processo escrever `low-power`/`balanced`/`performance`, troca o `ActiveProfile` para acompanhar; `quiet` e `balanced-performance` ele ignora.

  **Ida (nosso app → PPD).** Após `Mode.Set`/`Cycle`, o daemon ajusta `net.hadess.PowerProfiles.ActiveProfile` para o mais próximo:

  | Modo escolhido | PPD |
  |---|---|
  | `low-power`, `quiet` | `power-saver` |
  | `balanced` | `balanced` |
  | `balanced-performance`, `performance` | `performance` |

  Ordem: primeiro o PPD (o tuned vai escrever o `platform_profile` dele), o daemon marca a transição como **esperada** e ignora essa escrita no `poll()`, espera o tuned terminar (`PropertiesChanged` de `ActiveProfile` ou até 2 s) e só então escreve o modo real no sysfs. O tuned-ppd ignora `quiet`/`balanced-performance` e, para os outros, só confirma o `ActiveProfile` já definido: sem laço. Se o PPD estiver igual, não toca nele. Sem PPD no bus, pula a sincronização.

  **Volta (GNOME/PPD → nosso app).** O daemon assina `PropertiesChanged` de `ActiveProfile`; uma mudança que não foi iniciada por ele é intenção do usuário e vira o melhor modo para a fonte atual:

  | PPD | Na tomada | Na bateria |
  |---|---|---|
  | `power-saver` | `quiet` | `low-power` |
  | `balanced` | `balanced` | `balanced` |
  | `performance` | `performance` se em `Allowed` (Turbo, ≥ 40 %), senão `balanced-performance` | `balanced-performance` se ≥ 40 % (extra), senão `balanced` |

  O daemon deixa o tuned escrever, depois grava o modo escolhido e atualiza `mode_ac`/`mode_battery`. Escritas diretas de terceiros no sysfs (fora do PPD) seguem a mesma regra: aplicar as regras de energia e sincronizar o PPD na ida.

  **Eventos de energia e resume.** O daemon aplica o perfil guardado da fonte com atraso de 1–2 s, depois de o tuned ter trocado/restaurado o seu, e refaz a ida. Unit: `After=tuned.service power-profiles-daemon.service`; o daemon nunca desliga nem reconfigura esses serviços.

  **Ressalva a validar:** ao pedir PPD `performance` num modelo sem Turbo, o tuned escreve `performance` (5) no firmware antes de o daemon corrigir; conferir se o firmware rejeita (status 1/2, o `acer_wmi` devolve erro e o tuned só loga) ou aceita um Turbo transitório. Se aceitar, a ida para `balanced-performance` passa a escrever o sysfs primeiro e o PPD depois.

- **Cadência de leitura (medida 2026-10-12).** Cada leitura via WMI custa 5–10 ms de CPU (`pwm1` 11 ms, `pwm1_enable` 8 ms, `usb_charging` 8 ms, `kbd_backlight_*` 6 ms, `platform_profile` 6 ms); `fan*_input`, `temp*_input`, `power_supply` e `acer-wmi-battery` custam ~0. Por isso: AC/carga a cada 2 s; `Fans.State` a cada 5 s; teclado, USB, limite, calibração e saúde a cada 30 s (e logo após cada escrita); `platform_profile` por `poll()` (sysfs_notify) nos dois arquivos (classe e `/sys/firmware/acpi`) com releitura de segurança a cada 10 s. Resultado: ~1,7 % de CPU ocioso, contra 6 % lendo tudo a cada 2 s.
- **Leituras corrompidas.** Chamadas WMI concorrentes de processos diferentes corrompem respostas (ver `firmware-wmi.md` §9). O daemon serializa as suas com um mutex e, numa mudança externa de perfil, só age se o valor for um nome conhecido e se repetir numa segunda leitura 150 ms depois.
- **Mudanças externas** nos fans (outro programa no hwmon) aparecem em `Fans.State` em até 5 s.
- **Log** em journald; `--dry-run` (não escreve) e `--simulate <modelo>` (backends falsos com capacidades fixas, para desenvolver GUI sem hardware).
- Unit `nitrosensed.service`: `Type=dbus`, `BusName=org.nitrosense.Daemon1`, `After=acer-wmi-modules.service`, `ProtectSystem=strict` com `ReadWritePaths=/sys /var/lib/nitrosense`, `DeviceAllow=/dev/input/event* r`, `CapabilityBoundingSet=CAP_NET_ADMIN` (connector) — a refinar quando o esqueleto existir.

## 12. Mudanças em relação ao v0

1. Leituras viraram **propriedades** com `PropertiesChanged`; sinais `Changed` próprios removidos. Métodos ficaram só para escritas.
2. **Regras de energia** completadas: na bateria `balanced`, `low-power` e, como extensão sinalizada, `balanced-performance` com carga ≥ 40 %; `quiet` passa a exigir AC. O v0 só proibia `low-power` na tomada e `performance` na bateria. Novas propriedades `Allowed` e `Extras` evitam que cada cliente reimplemente a regra.
3. `profiles` em `Capabilities` vem de `acer-wmi-extras/supported_profiles`, não do `choices` do `platform-profile` (que lista Turbo mesmo sem suporte).
4. **Tabela de detecção** de cada capacidade (sysfs → chave), em vez de descrição solta.
5. Conjunto de **erros** definido (v0 só tinha `NotAllowed`).
6. **Monitor:** unidades/tipos fixados; `Subscribe` especificado (unicast por assinante, limpeza por `NameOwnerChanged`, sem amostragem sem assinantes).
7. **Lighting:** `on` é só leitura (não há Set de estado no APGe); timeout limitado aos valores que o firmware aceita.
8. **Battery:** limite, calibração e USB não são "restaurados" (vivem no EC); calibração termina por polling, pois não há como capturar o evento WMI sem tirar o handler do `acer_wmi`.
9. `Profiles` → **`Scenarios`**; formato do cenário e armazenamento definidos; App Sync restrito ao usuário da sessão ativa.
10. **polkit** dividido em `control` (sem senha na sessão ativa) e `settings` (`auth_admin_keep`).
11. Fans `manual` não são restaurados no boot.
12. Coexistência com `tuned-ppd` definida a partir do código dele: escrita direta no sysfs, sincronização do `ActiveProfile` nos dois sentidos (muitos-para-um na ida, escolha pela fonte de energia na volta), reaplicação com atraso após eventos de energia.
13. Fan manual sem piso fixo: faixa por fan em `capabilities.fans.range` (0–100 por padrão), o firmware é o árbitro.

## 13. Pontos em aberto

- **Cenários por usuário** (`~/.config`) além dos globais: possível pela GUI guardar o JSON e chamar `Apply` com o dicionário; decidir se o daemon precisa saber disso.
- ~~Validar `pwm=0` em manual~~ **Feito (2026-10-12):** com `pwm1_enable=1` e `pwm1=0` a ventoinha da CPU ficou em ~1 955 RPM por 8 s; o firmware impõe um piso e não a para. A faixa 0–100 fica como está.
- **Validação com tuned-ppd (2026-10-12), tomada:** `Set quiet` → PPD `power-saver`; `Set balanced-performance` → PPD `performance`; `Set balanced` → `balanced`; estável 5 s depois, sem laço. Pelo PPD: `power-saver` → `quiet`, `performance` → `balanced-performance`, `balanced` → `balanced`. Observando `platform_profile` a cada 20 ms durante a ida para `performance`, o valor nunca mostrou `performance`: o tuned escreve (há `sysfs_notify`), mas o valor lido continua o anterior, ou seja, o firmware/driver recusa o Turbo inexistente. A ordem "PPD primeiro, sysfs depois" fica. **Pendente:** bateria (tirar/colocar o cabo) e suspend/resume.
- **GPU:** a primeira leitura do `nvidia-smi` depois de a dGPU acordar (D3cold) devolve `power.draw` = 590,01 W; a seguinte, ~10 W. O daemon omite `power` quando o valor passa de 400 W. O probe inicial (`nvidia-smi`) acorda a GPU e leva ~1,5 s; com NVML (go-nvml) dá para evitar os dois problemas.
- **Tecla Nitro no GNOME:** `KEY_PROG1` pode estar ligada a um atalho do desktop; checar se precisa de `hwdb` próprio ou se o desktop ignora.
