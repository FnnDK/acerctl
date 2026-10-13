# Firmware Acer "Sense": interfaces WMI, EC e capacidades

Referência técnica do que a BIOS de um Acer Nitro/Predator expõe ao sistema operacional e de como o Linux já cobre parte disso. Máquina de referência: **Acer Nitro ANV15-52** (BIOS V1.31, i7-13620H, RTX 4060), kernel 7.2.9. Fontes: tabelas ACPI e blobs BMOF da própria máquina (`DSDT`, `SSDT10`, decodificados com `iasl` e `bmf2mof`), fonte do kernel (`drivers/platform/x86/acer-wmi.c`, `drivers/platform/wmi/core.c`, v7.2), os projetos [Linuwu-Sense](https://github.com/0x7375646F/Linuwu-Sense) e [acer-wmi-battery](https://github.com/frederik-h/acer-wmi-battery), e chamadas feitas com o módulo de pesquisa `drivers/acer-wmi-lab`.

## 1. O que o Linux já cobre nesta máquina

| Função | Caminho | Observação |
|---|---|---|
| Modos térmicos | `acer_wmi predator_v4=1` → `/sys/class/platform-profile/` | O driver lista 5 perfis fixos (`low-power quiet balanced balanced-performance performance`) sem ler a máscara do firmware; este modelo só tem 4 (ver §6). Linux `performance` = Acer Turbo. |
| RPM dos fans, temperaturas | hwmon `acer` (`fan1`=CPU, `fan2`=GPU, `temp1` CPU, `temp2` GPU, `temp3` externa) | GPU lê 0 com a dGPU desligada. |
| Fan manual/máximo | hwmon `acer` `pwm1`/`pwm2` + `pwm*_enable` (0 máx, 1 manual, 2 auto) | Só com a quirk `.pwm` ou `force_caps=0x1C26` (bits wireless, bluetooth, function-mode, profile, hwmon, pwm). O modelo não está na tabela DMI do driver. |
| Tecla de modo avião | `acer_wireless` (ACPI `10251229`) + rfkill do `acer_wmi` | — |
| Tecla Nitro | teclado PS/2, scancode `0xF5` → `KEY_PROG1` (148) via hwdb do systemd | **Não** é evento WMI neste modelo; cabe ao user space. |
| Hotkeys Fn | EC `_Qxx` → `HKEV` → evento WMI `676AA15E…` tratado pelo `acer_wmi` (0x84 = toggle do backlight do teclado, 0x61/0x62/0x63 brilho) | — |
| Limite de carga 80 %, calibração, temperatura da bateria | `acer-wmi-battery` (GUID de bateria, métodos 19/20/21) | Módulo fora da árvore, coexiste com o `acer_wmi`. |
| Áudio, Wi-Fi, GPU | `sof-hda-dsp` + Realtek, `mt7921`, `nvidia`/`i915` | — |

## 2. Classes WMI da BIOS (BMOF, "Version 2.93")

| Classe | GUID | Método ACPI | Métodos |
|---|---|---|---|
| **AcerGamingFunction** | `7A4DDFE7-5B5D-40B4-8595-4408E0CC7F56` | `WMBH` | 1 SetGamingProfile(u64) · 2 SetGamingLED(u8[16]) · 3 GetGamingProfile(u32→u64) · 4 GetGamingLED · 5 GetGamingSysInfo(u32→u64) · 6/7 Set/GetGamingRgbKb · 8/9 Set/GetGamingProfileSetting · 10/11 Set/GetGamingLEDBehavior · 12/13 Set/GetGamingLEDColor · 14/15 Set/GetGamingFanBehavior · 16/17 Set/GetGamingFanSpeed · 18/19 Set/GetGamingFanTable · 20/21 Set/GetGamingKBBacklight(u8[16]) · 22/23 Set/GetGamingMiscSetting · 24/25 Set/GetCPUOverclockingProfile(u8 + u8[512]) |
| **BatteryControl** | `79772EC5-04B1-4BFD-843C-61E7F77B6CC9` | `WMBE` | 19 GetBattInfoInterface(índice SBS, bateria→u32) · 20 GetBatteryHealthControlStatus · 21 SetBatteryHealthControl · 22/23 Get/SetBatteryFunctionData (carga adaptativa, "BAC") |
| **APGeAction** | `61EF69EA-865C-4BC3-A502-A0DEBA0CB531` | `WMAA` | 1 SetFunction(u64→u32) · 2 GetFunction(u32→u64). É o `WMID_GUID3` que o `acer_wmi` usa para status de Wi-Fi/Bluetooth em qualquer Acer. |
| AcerGenericMethod | `1EA0BC57-521E-461B-A7A9-5D36F141A82A` | `WMAE` | Set/GetFunction, GetStorageInformation |
| UtilityFunction | `FE1DBBDA-3014-4856-870C-5B3A744BF341` | `WMBL` | senhas, BIOS settings, 11/12 Get/SetLedIndicator, diagnóstico |
| BIOSSetting / AcerBiosConfigurationTool / MethodForPowerShell | `F75F5666…` / `4BB53443…` / `48E699DB…` | `WMBK`/`WMAC`/`WMBP` | configurações de BIOS (fora do escopo) |
| Eventos | `676AA15E…` (APGeEvent), `A9D77EF6…`, `2E6B3CE2…` | `_WED` | `EventDetail[8]` |

## 3. Como o firmware implementa

- **Quase tudo é SMM.** `WMBH` copia o buffer de entrada para `WMIB`, o id do método para `MTID` e dispara uma SMI (porta `PSSP`, valor 0xD0). A lógica de fans, teclado e configurações está no firmware, não na AML — os formatos dos buffers só se obtêm por experimento.
- **Exceção: sensores.** O método 5 (`GetGamingSysInfo`) chama `EC0.GGSI`: cmd 0 → máscara de sensores suportados (`0x0227` = ids 1, 2, 3, 6, 10), cmd 1 → leitura (índice no byte 1: 1 temp CPU `CTMP`, 2 fan CPU `CPUF`, 3 temp externa `ETP2`, 6 fan GPU `GPUF`, 10 temp GPU `GTMP`), cmd 2 → status AC (`BTST`). É o caminho do hwmon do `acer_wmi`.
- **Perfil térmico também reprograma a GPU.** `SetGamingMiscSetting` com índice 0x0B ajusta em AML o DPTF (`ODV0`, `Notify(\_SB.IETM, 0x88)`) e o controlador NVIDIA `NPCF` (`AMAT`/`ATPP`/`ACBT` a partir das tabelas `M1*` quiet, `M2*` balanced, `M3*` performance, `M5*` eco; `NVWM`=1 em quiet/eco). Não há tabela para Turbo (5) neste modelo.
- **EC RAM mapeada em memória** (`OperationRegion (ERAM, SystemMemory, 0xFE708500, 0x100)`): `KBBL` 0x48.0 (backlight do teclado ligado), `BTST` 0x48.5 (AC), `ETP2` 0x52, `FANT` 0x54 (4 bits), `CTMP` 0x58, `GTMP` 0x59, `CPUF` 0x5C (16 bits), `KBST` 0x69.1, `GPUF` 0x6A, `ACTP` 0x6D, `BTMD` 0x70, `NPWM` 0xF5. Acesso direto exigiria `ioremap`; preferir WMI.
- **Convenção das respostas:** buffers little-endian com **byte 0 = status** (0 ok, 1 não suportado, 2 parâmetro inválido) e o valor nos bytes seguintes.

## 4. AcerGamingFunction — formatos conhecidos

| Método | Entrada | Saída / semântica |
|---|---|---|
| 5 GetGamingSysInfo | byte 0 = cmd (0/1/2), byte 1 = índice do sensor | cmd 0: máscara nos bytes 3-4; cmd 1: valor nos bytes 1-2 (temp °C; RPM 16 bits); cmd 2: AC no byte 5 |
| 23 GetGamingMiscSetting | byte 0 = índice | byte 1 = valor. Índices com resposta neste modelo: 0x07 (0xFF), 0x08 (1), **0x0A máscara de perfis**, **0x0B perfil atual**. 1-6, 9, 12, 13, 15, 16 → não suportado; 0, 14, 17+ → inválido. (0x05/0x07 = OC no `acer-wmi`; 0x06 = som/animação de boot no Linuwu — não suportado aqui.) |
| 22 SetGamingMiscSetting | byte 0 = índice, byte 1 = valor | perfil: 0 quiet, 1 balanced, 4 performance, 5 turbo, 6 eco |
| 15 GetGamingFanBehavior | u16 bitmap de fans (**id 1 = bit 0, id 4 = bit 3**) | byte 1: modo de 2 bits por índice de fan (bitmap 0x09 → `0x41` = ambos auto) |
| 14 SetGamingFanBehavior | u16 bitmap + modo 2 bits em `16 + 2·i` (1 auto, 2 máx, 3 manual) | ex.: `0x410009` auto ambos, `0x820009` máx ambos, `0xC30009` manual ambos |
| 17 GetGamingFanSpeed / 16 Set | byte 0 = id do fan (1 CPU, 4 GPU), byte 1 = velocidade (`pct·256/100`) | só ids 1 e 4 respondem |
| 19 GetGamingFanTable | — | não suportado neste modelo (status 1) |
| 3 GetGamingProfile(0) | — | `00 00 00 01 00 00 ff 00`; no Linuwu o byte 6 = 1 indica LCD overdrive ligado → aqui 0xFF (não disponível) |
| 21 GetGamingKBBacklight, 7 GetGamingRgbKb | — | status 1 para qualquer entrada: sem API de teclado RGB/zonas (backlight de cor única) |
| 4/11/13 LEDs | — | status 1/2: sem LED de tecla Turbo, light bar ou logo |
| 25 GetCPUOverclockingProfile | — | ReturnCode 1: sem OC de CPU |

## 5. APGeAction — decodificação (validada com escritas reversíveis)

Buffer: byte 0 = função, bytes 1-2 = seletor/payload, byte 5 = valor. Get = método 2 com u32; Set = método 1 com u64.

| Função | Get | Resposta | Set |
|---|---|---|---|
| Backlight do teclado | `01 84 08 00` (0x088401) | `00 [00] [08 = ligado] 00 00 [timeout em s]` | `02 84 08 00 00 [s]`: `0x088402` desliga o timeout, `0x1E0000088402` = 30 s |
| USB charging com o notebook desligado | `04 00 00 00` | `00 [0f ligado / 1f desligado] [limite %]` | `04 0f 1e` 30 %, `04 0f 14` 20 %, `04 0f 0a` 10 %, `04 1f 0a` desligado |
| fn 3 / fn 7 / fn 10 | `03..` / `07 02..` / `0a..` | 0x11C0 / 0 / 1 | significado desconhecido |

O bit 0x08 da resposta do teclado é o mesmo que `WMID.GOTS(0x84)` devolve quando `KBBL == 1`. Códigos de erro do SMM: 0xE1 (não suportado), 0xE2 (parâmetro inválido), 0xE4.

## 6. BatteryControl

- 20/21 (health control): `uFunctionList` bit 0 = limite de 80 %, bit 1 = calibração; estruturas no `acer-wmi-battery`. O estado fica no EC e sobrevive a reboots.
- 19 `GetBattInfoInterface(índice, bateria=1)`: o índice é o **comando Smart Battery (SBS)**: 0x08 temperatura (0,1 K), 0x09 tensão (mV), 0x0A corrente (mA, com sinal); 0x03 BatteryMode. Índices não suportados devolvem 1.
- 22/23 (carga adaptativa): ReturnCode 1 neste modelo.

## 7. Capacidades do ANV15-52 (lidas do firmware)

- Perfis: máscara 0x0A = **0x53** → Quiet (0), Balanced (1), Performance (4), Eco (6). **Sem Turbo (5)**; coerente com a ausência da tabela `M4*` no `NPCF`.
- Fans: auto / máximo / manual por fan (ids 1 e 4); sem curva.
- Teclado: cor única; timeout de 30 s configurável (APGe).
- USB charging off-state: 0/10/20/30 %.
- Bateria: limite 80 %, calibração, tensão/corrente/temperatura; sem carga adaptativa.
- Sem: RGB/zonas, LEDs extras, OC de CPU, fan table; som de boot e LCD overdrive sem caminho conhecido.

### Validação do módulo `acer-wmi-extras` (ANV15-52, 2026-10-12)

Detecção no probe: `kbd timeout: yes, usb charging: yes, profiles: yes (mask 0x53), battery: yes`. Leituras: timeout 30 s, backlight ligado, USB 30 %, perfis `low-power quiet balanced balanced-performance`; hwmon `acer_battery`: 25,6 °C, 16 440 mV, 0 mA. Escritas: timeout 30→0→30, USB 30→20→30 com leitura coerente; 15 % rejeitado com `-EINVAL`.

## 8. `acer-wmi` mainline: fatos e patches candidatos

- `acer_predator_v4_platform_profile_probe` faz `set_bit` dos 5 perfis sem consultar `ACER_WMID_MISC_SETTING_SUPPORTED_PROFILES` (0x0A), constante definida e não usada.
- Mapeamento: `performance`=Turbo 5, `balanced-performance`=Performance 4, `balanced`=1, `quiet`=0, `low-power`=Eco 6.
- O driver ignora `WMID_AC_EVENT`; `platform_profile_cycle()` percorre todas as escolhas (Eco na tomada, Turbo na bateria). A regra AC/bateria fica no user space.
- Candidatos upstream: (1) entrada DMI `Nitro ANV15-52` → `predator_v4` + `pwm`; (2) probe de perfis lendo 0x0A; (3) battery hook `charge_control_end_threshold`; (4) **serializar as chamadas WMI** (mutex no `acer_wmi` em torno de `WMI_gaming_execute_u64`/`WMID_gaming_*`, ou melhor, no núcleo WMI por dispositivo `PNP0C14`), porque chamadas concorrentes de métodos diferentes corrompem as respostas (§9). Sem flags de marca: a variação é por modelo.
- Atenção com `tuned`/`tuned-ppd`: `throughput-performance` escreve `performance` (= Turbo); manter `balanced` em modelos sem Turbo.

## 9. Regras de coexistência para módulos fora da árvore

- Usar só a API legada por GUID (`wmi_evaluate_method`, `wmi_has_guid`; `EXPORT_SYMBOL_GPL` → módulo GPL). O núcleo WMI 7.2 não impede que vários módulos chamem o mesmo GUID; o `acer_wmi` usa a mesma API e não vincula driver ao GUID gaming. Exceção: o `acer-wmi-battery` vincula ao `79772EC5…` (nenhum outro driver o usa) só para ter um `struct device` para a extensão `power_supply`; as chamadas continuam pela API legada.
- Não vincular ao GUID de eventos `676AA15E…` nem instalar handler legado nele (já ocupado pelo `acer_wmi`, `AE_ALREADY_ACQUIRED`).
- Serializar as próprias chamadas (mutex): não há lock entre módulos ao invocar `WMAB`/`WMBH`.
- **Medido (2026-10-12): chamadas WMI concorrentes corrompem as respostas.** Os métodos AML (`WMAA`, `WMAB`, `WMAE`, `WMBE`, `WMBH`) são `Serialized`, o que só impede duas execuções do *mesmo* método; métodos diferentes rodam em paralelo e compartilham o caminho SMM. Lendo `platform-profile` a cada 5 ms enquanto outro processo lia `kbd_backlight_state` (APGe, `WMAA`), 30 de 578 leituras vieram erradas (`quiet` = valor 0, ou `-EOPNOTSUPP` por valor fora da tabela); com `pwm1` (`WMBH`, mesmo método) ao lado, 8 de 654; sem concorrência explícita, 2 de 333 (colisões com o daemon). Consequências: (a) o daemon serializa todos os seus acessos a sysfs com um mutex e só aceita como "mudança externa" um valor conhecido lido duas vezes; (b) qualquer leitor externo (`sensors`, `cat` no sysfs) pode receber lixo até o patch (4) do §8; (c) `acer-wmi-extras` já tem mutex próprio, mas ele não cobre o `acer_wmi`.
- Secure Boot: assinar os `.ko` com a MOK do akmods; lockdown `integrity` bloqueia debugfs gravável → interfaces em sysfs.

## 10. Projetos relacionados

- [Linuwu-Sense](https://github.com/0x7375646F/Linuwu-Sense) (GPL-3): substitui o `acer_wmi`; referência de constantes (fans, APGe, RGB 4 zonas, bateria).
- [acer-wmi-battery](https://github.com/frederik-h/acer-wmi-battery): módulo independente de bateria (vendorado em `drivers/`).
- [Div-Acer-Manager-Max](https://github.com/PXDiv/Div-Acer-Manager-Max) (GPL-3): GUI Avalonia + daemon Python sobre o Linuwu.
- [wobqqq/predator-sense](https://github.com/wobqqq/predator-sense): CLI + DKMS sobre o Linuwu.
