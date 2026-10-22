# Back-end em Go

A API do Finanças Local em Go, que substitui o .NET aos poucos ([épico #70](https://github.com/LucasDesenvGit/financas-local/issues/70), [ADR 0049](../../docs/adr/0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md)). Até a convivência (#231) entrar, ele **não roda no site**: o .NET continua sendo o que atende.

| Pasta | O que tem |
|---|---|
| `cmd/financas` | o ponto de entrada (o `Program.cs` do Go) |
| `internal/config` | leitura dos mesmos `appsettings*.json` do .NET |
| `internal/servidor` | HTTPS local, cabeçalhos de segurança, erros, front-end |
| `internal/banco` | SQLite no formato do EF Core, transação, consultas do `sqlc` ([README](internal/banco/README.md)) |
| `internal/tipos` | `Decimal`, `Data`, `DataHora`, `Guid` no formato do EF Core |
| `internal/relogio` | fuso de Brasília, injetável |

## Rodar

Precisa do Go 1.27.3 ou mais novo (as anteriores têm falhas de segurança na biblioteca padrão). Ao lado do .NET, que fica na 5443:

```bash
cd src/backend-go
go run ./cmd/financas -endereco 127.0.0.1:5444 -wwwroot ../frontend/dist/financas-web/browser -certificado-temporario
```

- O `-wwwroot` aponta para o build do Angular (`npm run build` em `src/frontend`).
- O `-certificado-temporario` cria um certificado só em memória, e o navegador vai avisar que não confia nele.
- Para usar o `.pfx` e os caminhos de uma instalação, aponte `-pasta` para a pasta da publicação (onde está o `appsettings.Production.json`).

## Antes de subir (push)

```bash
gofmt -l .                 # não pode listar nada
go vet ./...
go test -race ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
GOOS=windows go build ./...   # o executável do Windows compila sem cgo
```

A CI ainda não roda esses comandos: o job do Go entra na #231.

Dois testes manuais, que só rodam com as variáveis definidas:

```bash
# Ler uma CÓPIA do banco real inteira, sem mostrar valores (#74)
FINANCAS_COPIA_BANCO=/tmp/copia.db go test ./internal/banco -run CopiaDoBancoReal -v
# Comparar o Go com o .NET rodando, rota por rota: status, cabeçalhos e corpo (#73)
FINANCAS_DOTNET_URL=https://127.0.0.1:5443 FINANCAS_WWWROOT=$PWD/../../publicado/api/wwwroot \
  go test ./internal/servidor -run IgualAoDotnet -v
```

Todo PR do épico traz a seção "🎓 Aula rápida" na descrição.
