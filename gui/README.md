# nitro-control-center

GUI do nitroctl em Go + Wails v2: uma janela sobre a API D-Bus do `nitrod` (`docs/api-dbus.md`). Não tem lógica de hardware: cada ação é uma chamada ao daemon, que valida e explica a recusa (o erro aparece como aviso na tela).

## Layout

```
main.go             opções da janela, instância única, ícone; --session, --minimized
agent.go            agente da sessão (daemon/cmd/nitro-tray): opções "Ícone na bandeja" e
                    "Tecla Nitro", Reload do agente ou início dele quando passa a ser preciso
titlebar.go         janela sem moldura: ordem dos botões (button-layout do GNOME, pelo portal
                    de configurações ou gsettings) e ícones window-*-symbolic do tema, lidos do sistema
app.go              ponte D-Bus ↔ frontend: State() (todas as propriedades), métodos de escrita,
                    eventos "state" (PropertiesChanged, com reconexão) e "sample" (Monitor)
frontend/src/       TypeScript sem framework + CSS próprio; gráficos com uPlot
  core.ts           estado, histórico de amostras, chamadas, formatação, toasts/diálogos
  i18n.ts           textos pt/en (idioma do sistema ou escolhido em Configurações)
  icons.ts          ícones de linha próprios e a marca [N]
  titlebar.ts       barra de título própria (arrastar, duplo clique, minimizar/maximizar/fechar)
  pages/            home, monitor, fans, lighting, battery, scenarios, settings
frontend/wailsjs/   bindings gerados pelo Wails (não editar)
build/appicon.png   gerado de assets/logo/n-icon-cut.svg (rsvg-convert -w 512)
```

Telas e controles aparecem só para o que `System.Capabilities` lista (ex.: sem Turbo no ANV15-52). O Monitor só é assinado com a janela visível numa tela que mostra dados ao vivo; sem assinantes o daemon não amostra.

## Desenvolver (no container)

Precisa de `nodejs`, `npm`, `libwebkit2gtk-4.1-dev`, `libgtk-3-dev` e `go install github.com/wailsapp/wails/v2/cmd/wails@latest`. O Debian/Fedora só têm WebKitGTK 4.1: sempre `-tags webkit2_41`.

```bash
../daemon/nitrod --session --simulate ANV15-52 --state /tmp/ns.json &   # daemon falso
NITROCTL_SESSION=1 wails dev -tags webkit2_41    # janela + http://localhost:34115 no navegador
```

`wails dev` recusa flags desconhecidas, por isso o bus de sessão vem de `NITROCTL_SESSION=1` (equivale a `--session`).

## Instalar

```bash
sh packaging/install-gui.sh build                       # container
distrobox-host-exec sudo sh packaging/install-gui.sh    # host: /usr/local/bin, .desktop e ícone
distrobox-host-exec sudo sh packaging/install-gui.sh uninstall   # remove os três arquivos
```

O host só precisa do runtime `webkit2gtk4.1` (já presente no Fedora Silverblue 44).
