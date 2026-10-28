# Gerenciador em Go

O gerenciador do site em Go, com o [Wails v3](https://pkg.go.dev/github.com/wailsapp/wails/v3) ([ADR 0052](../../docs/adr/0052-gerenciador-em-go-com-bandeja-e-pagina-local.md), épico #70). Vai substituir o de C# (`src/gerenciador/`) quando estiver completo. **Ainda não é instalado** pelo `.msi` nem pelos scripts.

| Pronto | Falta |
|---|---|
| Janela igual à do `FormPrincipal.cs`, ícone na bandeja, uma cópia só, `--bandeja`, fechar destrói a janela (#418) | Versão nova pelo GitHub e pelo develop (#420) |
| Situação do site (de 5 em 5 s, na janela e no ícone da bandeja), ligar, desligar e reiniciar pelo UAC, cópia do banco, preferências (#419) | Aviso na bandeja quando o site cai com a janela fechada |
| | Linux: compilar com cgo, GTK 4 e WebKitGTK, Orca (#417) |
| | Ícone no `.exe` (go-winres) |

## Organização

- `main.go`: a entrada, e as ações que o próprio gerenciador roda como administrador (`--servico ligar|desligar|reiniciar`, `--backup "destino"`), como o `Program.cs`.
- `gerenciador.go`: o Wails (janela, bandeja, instância única), a consulta da situação e as rotas da página, servidas por dentro do programa, **sem porta aberta** (`AssetOptions.Handler`).
- `internal/tela`: a página (`pagina.html`, `estilo.css`) e o que ela mostra em cada situação. Não depende do Wails: os testes rodam sem janela.
- `internal/instalacao`: onde estão a API, o banco, os dados e o endereço (o `Instalacao.cs`), pelos mesmos `appsettings`.
- `internal/situacao`: o serviço e o `GET /api/auth/estado` (o `VerificadorSituacao`).
- `internal/servico`, `internal/elevacao`: o serviço do Windows e o UAC só para a ação.
- `internal/copia`: a cópia do banco pelo `VACUUM INTO` (`modernc.org/sqlite`, o mesmo da API).
- `internal/icones`: o ícone da bandeja com o ponto da cor da situação.
- `internal/preferencias`: o tema do Windows e as preferências do C# (`HKCU`).

É um módulo à parte (`go.mod` próprio), para a API não levar o Wails nem precisar de cgo no Linux.

## Compilar e testar (Windows)

```bash
cd src/gerenciador-go
go vet ./... && go test ./...
CGO_ENABLED=0 go build -tags production -trimpath -ldflags "-s -w -H=windowsgui -X main.versao=X.Y.Z" -o gerenciador-go.exe .
```

- `-tags production` tira o F12 e o menu do botão direito; `-H=windowsgui`, a janela do console.
- Para comparar com o C# sem mexer no serviço: `gerenciador-go.exe -mostrar=desligado -mais-opcoes` (ou `no-ar`, `ligando`, `sem-resposta`, `nao-instalado`). Sem `-mostrar`, a situação é a de verdade.
- **"Iniciar com o Windows" usa a mesma entrada do C#:** ligar aqui faz o Windows abrir o gerenciador em Go no lugar do de C#.
- O cache do WebView2 fica em `%LOCALAPPDATA%\FinancasLocal\Gerenciador\WebView2`.
