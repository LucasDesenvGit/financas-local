# Gerenciador em Go

O gerenciador do site em Go, com o [Wails v3](https://pkg.go.dev/github.com/wailsapp/wails/v3) ([ADR 0052](../../docs/adr/0052-gerenciador-em-go-com-bandeja-e-pagina-local.md), épico #70). Vai substituir o de C# (`src/gerenciador/`) quando estiver completo. **Ainda não é instalado** pelo `.msi` nem pelos scripts.

| Pronto | Falta |
|---|---|
| Janela igual à do `FormPrincipal.cs`, ícone na bandeja, uma cópia só, `--bandeja`, fechar destrói a janela (#418) | Atualizar o gerenciador e reabrir (#430, depois da #277) |
| Situação do site (de 5 em 5 s, na janela e no ícone da bandeja), ligar, desligar e reiniciar pelo UAC, cópia do banco, preferências (#419) | |
| Versão nova pelo GitHub Releases: procurar, "O que mudou", baixar conferindo o SHA-256 e abrir o instalador; "Receber versões de desenvolvimento" (#420, ADR 0053) | Instalar no Linux pelos scripts: o WebKitGTK 6.0 e o atalho `br.financaslocal.Gerenciador.desktop` (#277) |
| "Atualizar para a última do develop" na instalação de desenvolvimento pelos scripts, com a saída ao vivo (#428, ADR 0040), e "O que mudou" depois, pelo `CHANGELOG.md` do clone (#432) | |
| Linux (#417): `systemctl --user` sem senha, tema pelo portal, "Iniciar com o sistema" (`~/.config/autostart`), registro do serviço, comando para atualizar, "sem login" (linger), ícones em SVG | |
| Aviso na bandeja com a janela fechada: o site caiu, versão nova (#448) | |

## Organização

- `main.go`: a entrada, e as ações que o próprio gerenciador roda como administrador (`--servico ligar|desligar|reiniciar`, `--backup "destino"`, `--instalar-pelos-scripts`), como o `Program.cs`.
- `gerenciador.go`: o Wails (janela, bandeja, instância única), a consulta da situação e as rotas da página, servidas por dentro do programa, **sem porta aberta** (`AssetOptions.Handler`).
- `sistema_windows.go`, `sistema_linux.go`: o que muda entre os dois. No Windows, ligar, desligar, copiar o banco e reinstalar pedem o UAC; no Linux, rodam no próprio gerenciador, e fechar a janela só a esconde (ADR 0052, #417).
- `internal/tela`: a página (`pagina.html`, `estilo.css`) e o que ela mostra em cada situação. Não depende do Wails: os testes rodam sem janela.
- `internal/instalacao`: onde estão a API, o banco, os dados e o endereço (o `Instalacao.cs`), pelos mesmos `appsettings`.
- `internal/situacao`: o serviço e o `GET /api/auth/estado` (o `VerificadorSituacao`).
- `internal/servico`, `internal/elevacao`: o serviço do Windows e o UAC só para a ação; no Linux, a unidade `financaslocal.service` do `systemd --user`, o linger e o registro.
- `internal/comando`: roda os programas do sistema (`systemctl`, `loginctl`, `journalctl`, `busctl`), como o `Comando.cs`.
- `internal/copia`: a cópia do banco pelo `VACUUM INTO` (`modernc.org/sqlite`, o mesmo da API).
- `internal/atualizacao`: a versão nova pelo GitHub Releases (o `Atualizacoes.cs`): procurar, baixar conferindo o SHA-256, abrir o `.msi` e as notas separadas e em HTML seguro. `versao_nova.go` liga isso à janela.
- `internal/desenvolvimento`: o "Atualizar para a última do develop" (ADR 0040): reconhecer o clone na `develop`, rodar os scripts com a saída ao vivo e acompanhar o log do instalador. `develop.go` liga isso à janela.
- `internal/icones`: o ícone da bandeja com o ponto da cor da situação.
- `internal/preferencias`: o tema do Windows e as preferências do C# (`HKCU`); no Linux, o atalho em `~/.config/autostart` e o tema pelo portal do freedesktop.

É um módulo à parte (`go.mod` próprio), para a API não levar o Wails nem precisar de cgo no Linux.

## Compilar e testar no Linux

Pede cgo, o GTK 4 e o WebKitGTK 6.0 (os `-dev` no Ubuntu: `libgtk-4-dev libwebkitgtk-6.0-dev`; no Arch, `webkitgtk-6.0`):

```bash
cd src/gerenciador-go
go vet ./... && go test -race ./...
CGO_ENABLED=1 go build -tags production -trimpath -ldflags "-s -w -X main.versao=X.Y.Z" -o gerenciador-go .
./gerenciador-go -mostrar=desligado -mais-opcoes
```

- O ícone da janela vem do atalho `br.financaslocal.Gerenciador.desktop` (em `~/.local/share/applications`); sem ele, aparece o do Wails.
- **"Iniciar com o sistema" usa o mesmo atalho do C#** (`~/.config/autostart/financaslocal-gerenciador.desktop`): ligar aqui troca o que abre com a sessão.

## Compilar e testar (Windows)

```bash
cd src/gerenciador-go
go vet ./... && go test ./...
CGO_ENABLED=0 go build -tags production -trimpath -ldflags "-s -w -H=windowsgui -X main.versao=X.Y.Z" -o gerenciador-go.exe .
```

- `-tags production` tira o F12 e o menu do botão direito; `-H=windowsgui`, a janela do console.
- Para comparar com o C# sem mexer no serviço: `gerenciador-go.exe -mostrar=desligado -mais-opcoes` (ou `no-ar`, `ligando`, `sem-resposta`, `nao-instalado`). Sem `-mostrar`, a situação é a de verdade.
- **"Iniciar com o Windows" usa a mesma entrada do C#:** ligar aqui faz o Windows abrir o gerenciador em Go no lugar do de C#.
- **Ícone e manifesto do `.exe`:** o `rsrc_windows_amd64.syso` (o cofrinho do `icone.ico`, o manifesto "gui": controles comuns v6 e DPI por monitor, e o nome "Finanças Local", o do topo das notificações), que o `go build` junta sozinho. Ele é gerado pelo [go-winres](https://github.com/tc-hib/go-winres) na CI (job "gerenciador em Go"), que confere se o do repositório está em dia; mudou o `icone.ico`, baixe o artefato `icone-gerenciador` da CI e suba o `.syso` novo.
- O cache do WebView2 fica em `%LOCALAPPDATA%\FinancasLocal\Gerenciador\WebView2`.
