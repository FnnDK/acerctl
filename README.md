# nitrosense-linux

Ferramentas para trazer ao Linux as funções do Acer NitroSense/PredatorSense em notebooks Nitro e Predator (e, no que o firmware permitir, outros Acer), **complementando o driver in-tree `acer_wmi`** em vez de substituí-lo.

Estado e próximos passos: `STATUS.md`. Referência técnica do firmware (classes WMI, formatos, capacidades): `docs/firmware-wmi.md`.

## Componentes

| Diretório | O que é |
|---|---|
| `drivers/acer-wmi-extras/` | Módulo companheiro (GPL): timeout do backlight do teclado, carregador USB com o notebook desligado, perfis térmicos que o firmware realmente suporta, hwmon da bateria (temperatura, tensão, corrente). Cada função só aparece se o firmware responder. |
| `drivers/acer-wmi-battery/` | Driver de limite de carga (80 %) e calibração, vendorado do projeto de Frederik Himpe com uma correção de leitura fora do buffer (`UPSTREAM.md`). |
| `drivers/acer-wmi-lab/` | Módulo de pesquisa: chama qualquer método WMI por `sysfs` (`/sys/devices/platform/acer-wmi-lab/call`). Só para mapear firmware; não é para uso normal. |

Camada mainline usada: `acer_wmi` com `predator_v4=1` (perfis térmicos em `platform-profile`, fans e temperaturas em hwmon) e, até a quirk DMI chegar upstream, `force_caps=0x1C26` para liberar o controle manual dos fans.

## Compilar, assinar, carregar

Requer `kernel-devel` do kernel em uso, `gcc` e `make`. Com Secure Boot, assine com uma MOK registrada (a do akmods no Fedora):

```bash
make -C drivers/acer-wmi-extras
sudo /usr/src/kernels/$(uname -r)/scripts/sign-file sha256 \
  /etc/pki/akmods/private/private_key.priv /etc/pki/akmods/certs/public_key.der \
  drivers/acer-wmi-extras/acer-wmi-extras.ko
sudo insmod drivers/acer-wmi-extras/acer-wmi-extras.ko
ls /sys/devices/platform/acer-wmi-extras/
```

Opções do `acer_wmi` usadas na máquina de referência (`/etc/modprobe.d/acer-wmi.conf`):

```
options acer_wmi predator_v4=1 force_caps=0x1C26
```

Antes de ativar `platform-profile`, confira o perfil do `tuned`/`power-profiles-daemon`: o modo `performance` do Linux corresponde ao Turbo da Acer, que nem todo modelo tem.

## Licença

Módulos de kernel: GPL-2.0. `drivers/acer-wmi-battery/` mantém a licença do projeto de origem.
