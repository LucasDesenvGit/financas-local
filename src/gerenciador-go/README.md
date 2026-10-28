# Gerenciador em Go

O gerenciador do site em Go, com o [Wails v3](https://pkg.go.dev/github.com/wailsapp/wails/v3) ([ADR 0052](../../docs/adr/0052-gerenciador-em-go-com-bandeja-e-pagina-local.md), épico #70). Vai substituir o de C# (`src/gerenciador/`) quando estiver completo. **Ainda não é instalado** pelo `.msi` nem pelos scripts.

| Pronto | Falta |
|---|---|
| Janela igual à do `FormPrincipal.cs`, ícone na bandeja, uma cópia só, `--bandeja`, fechar destrói a janela (#418) | Situação do site, ligar e desligar, cópia do banco, gravar as preferências (#419) |
| | Versão nova pelo GitHub e pelo develop (#420) |
| | Linux: compilar com cgo, GTK 4 e WebKitGTK, Orca (#417) |
| | Ícone no `.exe` (go-winres) |

## Organização

- `main.go`: o Wails (janela, bandeja, instância única) e as rotas da página, servidas por dentro do programa, **sem porta aberta** (`AssetOptions.Handler`).
- `internal/tela`: a página (`pagina.html`, `estilo.css`) e o que ela mostra em cada situação. Não depende do Wails: os testes rodam sem janela.
- `internal/preferencias`: o tema do Windows e as preferências do C# (`HKCU`), só leitura por enquanto.

É um módulo à parte (`go.mod` próprio), para a API não levar o Wails nem precisar de cgo no Linux.

## Compilar e testar (Windows)

```bash
cd src/gerenciador-go
go vet ./... && go test ./...
CGO_ENABLED=0 go build -tags production -trimpath -ldflags "-s -w -H=windowsgui -X main.versao=X.Y.Z" -o gerenciador-go.exe .
```

- `-tags production` tira o F12 e o menu do botão direito; `-H=windowsgui`, a janela do console.
- Para comparar com o C# enquanto a situação não é lida de verdade: `gerenciador-go.exe -mostrar=desligado -mais-opcoes` (ou `no-ar`, `ligando`, `sem-resposta`, `nao-instalado`).
- O cache do WebView2 fica em `%LOCALAPPDATA%\FinancasLocal\Gerenciador\WebView2`.
