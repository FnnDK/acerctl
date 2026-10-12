# nitrosense-linux — notas para o assistente

Projeto: NitroSense/PredatorSense (Acer) para Linux, coexistindo com o `acer_wmi` in-tree. **Leia `STATUS.md` primeiro** (estado, feito, próximo passo, backlog, comandos) e depois as seções citadas de `docs/firmware-wmi.md`.

Ambiente:
- O shell roda num distrobox Debian (`deb`); o host é Fedora Silverblue. Comandos no host: `distrobox-host-exec ...`; root no host exige senha do usuário → entregue comandos `distrobox-host-exec sudo ...` para ele rodar no terminal.
- Secure Boot ativo: assinar `.ko` com a MOK do akmods (comando em `STATUS.md`). Lockdown bloqueia debugfs gravável → sysfs.
- Compilar módulos no host: `distrobox-host-exec make -C drivers/<módulo>`.

Regras do projeto: não substituir o `acer_wmi`; sem flags de marca (capacidades vêm do firmware); probes só com métodos "Get"; "Set" só com formato validado e reversível; manter `tuned` em `balanced`. Atualize `STATUS.md` ao fim de cada bloco de trabalho. Commits só quando o usuário pedir.
