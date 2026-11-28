# Gerenciador em Go

A janela e o ícone na bandeja que mostram a situação do site, ligam, desligam e reiniciam o serviço, copiam e restauram o banco e atualizam o sistema, no Windows e no Linux, com o [Wails v3](https://pkg.go.dev/github.com/wailsapp/wails/v3) ([ADR 0052](../../docs/adr/0052-gerenciador-em-go-com-bandeja-e-pagina-local.md)). Onde ele fica: [docs/ARQUITETURA.md, "Gerenciador"](../../docs/ARQUITETURA.md#gerenciador--srcgerenciador-go); o que faz para quem usa: [docs/INSTALADOR-E-GERENCIADOR.md](../../docs/INSTALADOR-E-GERENCIADOR.md#o-gerenciador).

É um módulo à parte (`go.mod` próprio), para a API não levar o Wails nem precisar de cgo. O `main.go` é a entrada e as ações que o gerenciador roda como administrador (`--servico`, `--backup`, `--restaurar`, `--celular`, `--primeiro-acesso`, `--instalar-pelos-scripts`) ou sem janela (`--situacao`, que a CI do Linux usa: 0 = no ar); o `gerenciador.go`, o Wails, com a página servida por dentro, **sem porta aberta**; os `*_windows.go` e `*_linux.go`, o que muda entre os dois; e um pacote por assunto em `internal/` (a página em `internal/tela`, testada sem janela).

## Compilar e testar

No Linux, pede cgo, o GTK 4 e o WebKitGTK 6.0 (no Ubuntu, `libgtk-4-dev libwebkitgtk-6.0-dev`; no Arch, `webkitgtk-6.0`). No Windows, compila sem cgo:

```bash
cd src/gerenciador-go
go vet ./... && go test -race ./...    # no Windows, sem o -race
CGO_ENABLED=1 go build -tags production -trimpath -ldflags "-s -w -X main.versao=X.Y.Z" -o gerenciador-go .   # Linux
CGO_ENABLED=0 go build -tags production -trimpath -ldflags "-s -w -H=windowsgui -X main.versao=X.Y.Z" -o gerenciador-go.exe .   # Windows
./gerenciador-go -mostrar=desligado -mais-opcoes
```

- `-tags production` tira o F12 e o menu do botão direito; `-H=windowsgui`, a janela do console. `-mostrar` abre numa situação de exemplo, sem consultar nem mexer no serviço (`-h` lista as situações).
- **"Iniciar com o sistema" é a mesma entrada do gerenciador instalado:** ligá-lo num compilado à mão troca o que abre com a sessão.
- No Linux, o ícone da janela vem do atalho `br.financaslocal.Gerenciador.desktop` (em `~/.local/share/applications`); sem ele, aparece o do Wails.
- **Ícone e manifesto do `.exe`:** o `rsrc_windows_amd64.syso`, que o `go build` junta sozinho, é gerado pelo go-winres na CI (job "gerenciador em Go"), que confere se o do repositório está em dia. Mudou o `icone.ico`? Baixe o artefato `icone-gerenciador` da CI e suba o `.syso` novo; ninguém roda o go-winres na própria máquina.

## Instalação

O `.msi` (`instalador/gerar-msi.ps1`) e os scripts (`scripts/windows/instalar-gerenciador.ps1` e `scripts/linux/instalar-gerenciador.sh`, no [índice dos scripts](../../scripts/README.md)) compilam este módulo: no Windows, sem cgo e com o nome `Financas.Gerenciador.exe`; no Linux, com cgo, e sem o compilador de C ou as bibliotecas o script diz o que falta e segue sem gerenciador. Sem o WebView2, o `.msi` instala do mesmo jeito, e a tela final manda para a página da Microsoft.
