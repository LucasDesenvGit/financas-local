# Back-end em Go

A API do Finanças Local em Go, que substitui o .NET aos poucos ([épico #70](https://github.com/LucasDesenvGit/financas-local/issues/70), [ADR 0049](../../docs/adr/0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md)). Desde a #231, ele **roda no site atrás do .NET**: o .NET cuida do login, do CSRF, dos segredos e do esquema do banco, sobe o Go como processo filho e repassa a ele só os prefixos de rota de `Go:Prefixos`.

| Pasta | O que tem |
|---|---|
| `cmd/financas` | o ponto de entrada (o `Program.cs` do Go): `-atras-do-dotnet` no site, ou sozinho (esqueleto da #73) |
| `internal/api` | o `ServeMux` com as rotas de todos os assuntos (o `MapControllers` do Go) |
| `internal/saude` | `GET /api/go/saude`, o exemplo que passa pelo caminho todo (front-end → .NET → Go → banco) |
| `internal/servidor` | HTTPS local, cabeçalhos de segurança, erros, front-end e o modo atrás do .NET (segredo interno, usuário) |
| `internal/sessao` | quem está logado, no contexto da requisição |
| `internal/config` | leitura dos mesmos `appsettings*.json` do .NET |
| `internal/banco` | SQLite no formato do EF Core, transação, consultas do `sqlc` ([README](internal/banco/README.md)) |
| `internal/tipos` | `Decimal`, `Data`, `DataHora`, `Guid` no formato do EF Core |
| `internal/relogio` | fuso de Brasília, injetável |
| `internal/versao` | a versão, gravada na compilação pelo `-ldflags` |

## Como roda no site (atrás do .NET)

```
navegador ──HTTPS──► .NET (login, CSRF) ──HTTP 127.0.0.1:<porta>──► Go ──► mesmo SQLite (WAL)
                       │  X-Financas-Segredo: segredo desta partida
                       │  X-Financas-Usuario: Id de quem está logado
```

- **Quem sobe:** o .NET (`Servicos/Go/ProcessoGo.cs`) executa `financas-go -atras-do-dotnet -banco <caminho> -pasta <pasta da API>` depois do `EsquemaBanco`. O executável fica ao lado da API (`Go:Executavel` muda o caminho).
- **Partida:** o Go lê o segredo interno da primeira linha da entrada padrão, escuta em `127.0.0.1` numa porta livre e escreve `{"porta":N}` na saída padrão.
- **Confiança:** sem o segredo, 401 (e um aviso no log). O Id do usuário vem em `X-Financas-Usuario`, que o .NET apaga quando vem de fora; a rota o lê com `sessao.Usuario(ctx)`.
- **Parada:** quando a entrada padrão fecha (o .NET parou, ou caiu), o Go para com elegância. SIGINT e SIGTERM também. No Windows, o .NET o põe num Job Object que o encerra junto.
- **Log:** JSON na saída de erro. O .NET registra cada linha na categoria `Financas.Go`; avisos e erros vão para Configurações › Erros. Para cair na origem certa da tela, ponha o atributo `"origem"` (`"API"`, `"Open Finance"`, `"Investimentos"`, `"Câmbio"` ou `"Sistema"`).
- **Banco:** o mesmo arquivo do .NET, em WAL (quem passa para WAL é o `EsquemaBanco`). O Go não cria nem muda tabela.

### Rota nova

1. O handler no pacote do assunto (`internal/<assunto>`), com as dependências pelo construtor.
2. O registro em `internal/api/api.go`: `mux.Handle("GET /api/<assunto>/{id}", ...)`.
3. O prefixo em `Go:Prefixos` no `src/backend/Financas.Api/appsettings.json` (separados por `;`). Enquanto o prefixo não estiver lá, a rota não recebe nada e o .NET continua atendendo.
4. Tarefa em segundo plano que passou para o Go: o nome dela em `Go:Tarefas` (`Sincronizacao`, `Cambio` ou `Cripto`), para o .NET não rodar a dele.

Voltar o assunto para o .NET é tirar o prefixo (e a tarefa) da lista e reiniciar o serviço; não precisa reinstalar.

## Rodar

Precisa do Go 1.27.3 ou mais novo (as anteriores têm falhas de segurança na biblioteca padrão).

**Atrás do .NET, em desenvolvimento:** compile o Go para a pasta de saída da API e suba a API como sempre (Visual Studio ou `dotnet run`). Sem o executável, as rotas de `Go:Prefixos` respondem 503 e o .NET registra o erro.

```bash
cd src/backend-go
go build -o ../backend/Financas.Api/bin/Debug/net10.0/financas-go ./cmd/financas        # Linux
go build -o ../backend/Financas.Api/bin/Debug/net10.0/financas-go.exe ./cmd/financas    # Windows
```

**Sozinho** (o esqueleto da #73: front-end e cabeçalhos, sem rota de `/api`, porque o Go ainda não tem login), ao lado do .NET, que fica na 5443:

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
# De ponta a ponta, com o .NET compilado (dotnet build FinancasLocal.sln -c Release, na raiz):
bash ../../scripts/ci/testar-go-atras-do-dotnet.sh Release
```

A CI roda os cinco primeiros no job "Go" e o de ponta a ponta no job "API e banco".

Dois testes manuais, que só rodam com as variáveis definidas:

```bash
# Ler uma CÓPIA do banco real inteira, sem mostrar valores (#74)
FINANCAS_COPIA_BANCO=/tmp/copia.db go test ./internal/banco -run CopiaDoBancoReal -v
# Comparar o Go com o .NET rodando, rota por rota: status, cabeçalhos e corpo (#73)
FINANCAS_DOTNET_URL=https://127.0.0.1:5443 FINANCAS_WWWROOT=$PWD/../../publicado/api/wwwroot \
  go test ./internal/servidor -run IgualAoDotnet -v
```

Todo PR do épico traz a seção "🎓 Aula rápida" na descrição.
