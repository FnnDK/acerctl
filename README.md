<p align="center"><img src="assets/logo/n-icon-cut.svg" width="112" alt=""></p>

<h1 align="center">nitroctl</h1>

<p align="center">Central de controle para notebooks Acer Nitro e Predator no Linux: modos de operação, ventoinhas, bateria, teclado e sensores, com app gráfico, CLI e serviço de sistema.</p>

O Acer NitroSense/PredatorSense só existe para Windows. O nitroctl traz as mesmas funções para o Linux **complementando o driver in-tree `acer_wmi`** em vez de substituí-lo: o que falta no kernel entra por módulos companheiros pequenos, um daemon aplica as regras e um app (`Nitro Control Center`) e uma CLI (`nitroctl`) falam com ele por D-Bus. Nada depende de marca ou modelo fixos: cada função só aparece se o firmware da máquina a oferecer.

> Projeto independente e de código aberto, sem vínculo com a Acer. "Nitro", "Predator", "NitroSense" e "PredatorSense" são marcas da Acer Inc., citadas só para identificar o hardware.

## O que faz

- **Modos de operação** (Eco, Silencioso, Equilibrado, Desempenho, Turbo) só com os perfis que o firmware suporta, com as regras de tomada/bateria do app original e sincronia com o `power-profiles-daemon`/`tuned` do desktop.
- **Ventoinhas** da CPU e da GPU em automático, máximo ou velocidade manual.
- **Monitor** de temperaturas, uso, clocks, potência, rotação, RAM e rede, sem acordar a GPU dedicada quando ela está desligada.
- **Bateria:** limite de carga em 80 %, calibração, saúde, ciclos, temperatura, tensão e corrente.
- **Teclado:** tempo para apagar a luz. **USB:** carregar dispositivos com o notebook desligado.
- **Cenários** que combinam modo, ventoinhas e teclado, e **App Sync**, que aplica um cenário quando um jogo ou programa abre (inclusive via Wine/Proton) e volta ao estado anterior quando ele fecha.
- **Tecla Nitro** alternando os modos; estado restaurado no boot e após suspender.

## Nitro Control Center

![Painel](docs/screenshots/home.png)

| | |
|---|---|
| ![Monitor](docs/screenshots/monitor.png) | ![Ventoinhas](docs/screenshots/fans.png) |
| ![Bateria](docs/screenshots/battery.png) | ![Cenários](docs/screenshots/scenarios.png) |
| ![Iluminação](docs/screenshots/lighting.png) | ![Configurações](docs/screenshots/settings.png) |

<sub>Capturas com o daemon em modo simulado (`nitrod --simulate ANV15-52`).</sub>

O app segue o tema escuro próprio, com barra de título integrada que usa a ordem dos botões e os ícones do GNOME, e textos em português e inglês.

## Componentes

| Diretório | O que é |
|---|---|
| `gui/` | **Nitro Control Center** (Go + Wails, WebKitGTK): o app gráfico. Detalhes em `gui/README.md`. |
| `daemon/` | **`nitrod`** (Go): serviço D-Bus `io.github.fnndk.Nitrod1` no system bus, com polkit; regras de energia, ventoinhas, iluminação, bateria, monitor, cenários e App Sync. API em `docs/api-dbus.md`. Inclui a CLI **`nitroctl`**. |
| `drivers/acer-wmi-extras/` | Módulo companheiro (GPL): timeout do backlight do teclado, carregamento USB com o notebook desligado, perfis térmicos que o firmware realmente suporta, sensores da bateria. |
| `drivers/acer-wmi-battery/` | Limite de carga (80 %) e calibração, vendorado do projeto de Frederik Himpe com uma correção (`UPSTREAM.md`). |
| `drivers/acer-wmi-lab/` | Módulo de pesquisa que chama métodos WMI por `sysfs`; só para mapear firmware. |
| `packaging/` | Units do systemd, política D-Bus, ações polkit e scripts de instalação. |

Camada mainline usada: `acer_wmi` com `predator_v4=1` (perfis em `platform-profile`, ventoinhas e temperaturas em hwmon) e, até a quirk do modelo chegar upstream, `force_caps=0x1C26` para liberar o controle manual das ventoinhas.

## Instalação

Máquina de referência: Acer Nitro ANV15-52 com Fedora Silverblue 44. Os scripts instalam em `/usr/local`, `/etc` e `/var/lib` (graváveis no Silverblue).

```bash
# 1. módulos de kernel (compila e assina com a MOK do akmods; Secure Boot)
sudo sh packaging/build-sign.sh
sudo sh packaging/install-modules.sh

# 2. daemon nitrod + CLI nitroctl
sh packaging/install-daemon.sh build          # precisa de Go
sudo sh packaging/install-daemon.sh install

# 3. app Nitro Control Center
sh packaging/install-gui.sh build             # precisa de Go, Node, Wails e libwebkit2gtk-4.1-dev
sudo sh packaging/install-gui.sh              # 'uninstall' remove
```

Opções do `acer_wmi` na máquina de referência (`/etc/modprobe.d/acer-wmi.conf`):

```
options acer_wmi predator_v4=1 force_caps=0x1C26
```

Antes de ativar `platform-profile`, confira o perfil do `tuned`/`power-profiles-daemon`: o modo `performance` do Linux corresponde ao Turbo da Acer, que nem todo modelo tem.

Depois de instalado: `nitroctl status` mostra o estado; `nitroctl help` lista os comandos.

### Compilar um módulo à mão

Requer `kernel-devel` do kernel em uso, `gcc` e `make`. Com Secure Boot, assine com uma MOK registrada (a do akmods no Fedora):

```bash
make -C drivers/acer-wmi-extras
sudo /usr/src/kernels/$(uname -r)/scripts/sign-file sha256 \
  /etc/pki/akmods/private/private_key.priv /etc/pki/akmods/certs/public_key.der \
  drivers/acer-wmi-extras/acer-wmi-extras.ko
sudo insmod drivers/acer-wmi-extras/acer-wmi-extras.ko
ls /sys/devices/platform/acer-wmi-extras/
```

## Documentação

- `STATUS.md`: estado do projeto, o que foi feito e próximos passos.
- `docs/firmware-wmi.md`: referência técnica do firmware (classes WMI, formatos, capacidades).
- `docs/api-dbus.md`: API D-Bus do `nitrod`.

## Licença

Módulos de kernel: GPL-2.0. `drivers/acer-wmi-battery/` mantém a licença do projeto de origem.
