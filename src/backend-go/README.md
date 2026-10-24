# Back-end em Go

A API do Finanças Local em Go, que substitui o .NET aos poucos ([épico #70](https://github.com/LucasDesenvGit/financas-local/issues/70), [ADR 0049](../../docs/adr/0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md)). Desde a #231, ele **roda no site atrás do .NET**: o .NET cuida do login, do CSRF, dos segredos e do esquema do banco, sobe o Go como processo filho e repassa a ele só os prefixos de rota de `Go:Prefixos`.

| Pasta | O que tem |
|---|---|
| `cmd/financas` | o ponto de entrada (o `Program.cs` do Go): `-atras-do-dotnet` no site, ou sozinho, com o login do Go (#75) |
| `internal/api` | o `ServeMux` com as rotas de todos os assuntos (o `MapControllers` do Go): uma linha por assunto |
| `internal/saude` | `GET /api/go/saude`, o exemplo que passa pelo caminho todo (front-end → .NET → Go → banco) |
| `internal/cambio` | câmbio pela PTAX (#81, ADR 0007): cliente do Banco Central, conversor para reais, `/api/cambio` e a tarefa de 15 em 15 minutos |
| `internal/cripto` | criptomoedas pelo Mercado Bitcoin (#81, ADR 0048): cliente, `/api/cripto` e a tarefa de 30 em 30 minutos |
| `internal/tarefas` | tarefas em segundo plano (o `BackgroundService` do .NET): só as de `Go:Tarefas`, com `context`, espera ao parar e `recover` por ciclo |
| `internal/servidor` | HTTPS local, cabeçalhos de segurança, erros, front-end e o modo atrás do .NET (segredo interno, usuário) |
| `internal/sessao` | quem está logado, no contexto da requisição |
| `internal/autenticacao` | o `AuthController` em Go (#75): login, sessão em cookie assinado, CSRF, limite por IP, bloqueio e auditoria; só com o Go sozinho, até a inversão (#85) |
| `internal/respostas` | as respostas que o ASP.NET monta sozinho (`ProblemDetails`, `ValidationProblemDetails` com as mensagens do `System.Text.Json`, 415, `{"mensagem"}` do `BaseController`), para todos os assuntos |
| `internal/auditoria` | o `AuditoriaService`: grava em `AuditoriaAcesso` o evento, o login, o IP e o navegador |
| `internal/configuracao` | o `ConfiguracaoService` (#76): ler e gravar a `Configuracao` com as regras do `int.TryParse`, `bool.TryParse` e `DateTime.TryParse` do C# |
| `internal/parametros` | o `ParametrosController` e o `ParametroService` (#76): o catálogo, a gravação que valida tudo antes e o diagnóstico; o prefixo entra no repasse na inversão (#85) |
| `internal/segredos` | o esquema novo dos segredos (#76): AES-256-GCM com prefixo `v2:`, chave protegida por DPAPI no Windows e `0600` no Linux; desligado até a #85 |
| `internal/config` | leitura dos mesmos `appsettings*.json` do .NET |
| `internal/banco` | SQLite no formato do EF Core, transação, consultas do `sqlc`, esquema ao iniciar ([README](internal/banco/README.md)) |
| `internal/tipos` | `Decimal`, `Data`, `DataHora`, `Guid` no formato do EF Core |
| `internal/relogio` | fuso de Brasília, injetável; o relógio de teste do contrato (`APartirDe`, #363) |
| `internal/versao` | a versão, gravada na compilação pelo `-ldflags` |
| `internal/testeapoio` | só para testes (`_test.go`): banco temporário com o esquema real, fábrica de dados inventados, relógio parado em Brasília e handler chamado com o usuário logado ([README](internal/testeapoio/README.md), #369) |
| `testes/contrato` | testes de contrato: o que o .NET responde e grava, caso a caso, para o Go responder igual ([README](testes/contrato/README.md)) |

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

1. O handler no pacote do assunto (`internal/<assunto>`), com as dependências pelo construtor. O pacote exporta `Registrar(mux, <só o que ele usa>)`, que põe as rotas dele no mux (`mux.Handle("GET /api/<assunto>/{id}", ...)`); o exemplo é o `internal/saude`.
2. **Uma linha** em `internal/api/api.go`: `<assunto>.Registrar(mux, d.DB, d.Log, d.Relogio)`. É só isso que os assuntos feitos em paralelo dividem; no merge, conflito ali é sempre "fique com as duas linhas". Hora vem do `d.Relogio`, nunca do `time.Now()`.
3. Os casos de contrato (no número reservado do assunto) e a rodada contra o Go: `PREFIXOS_GO` para rodar aqui e a linha do assunto em `testes/contrato/assuntos-no-go.txt` para a CI, como no [README do contrato](testes/contrato/README.md#como-cada-assunto-liga-o-contrato-contra-o-go).
4. Os testes do assunto (ADR 0051), com o apoio comum de `internal/testeapoio` ([README](internal/testeapoio/README.md)): banco temporário, fábrica de dados e handler chamado com o usuário logado.
5. Para ligar no site (passo 2 da ADR 0049, PR à parte): os prefixos na entrada do assunto em `Go:Prefixos`, no `src/backend/Financas.Api/appsettings.json`. É uma entrada por assunto, já reservada e vazia, com os prefixos separados por `;` (`"cambio": "/api/cambio;/api/cripto"`), e uma linha em branco entre as entradas, para dois PRs não mexerem em linhas vizinhas. Enquanto a entrada estiver vazia, a rota não recebe nada e o .NET continua atendendo.
6. Tarefa em segundo plano que passou para o Go: o nome dela em `Go:Tarefas` (`Sincronizacao`, `Cambio` ou `Cripto`), para o .NET não rodar a dele.

Voltar o assunto para o .NET é esvaziar a entrada dele (e tirar a tarefa) e reiniciar o serviço; não precisa reinstalar. Num `appsettings` de cima, `"Go": {"Prefixos": {"cambio": ""}}` desliga só aquele assunto, e `"Prefixos": ""` (texto) desliga todos.

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
- O `-aplicar-esquema` cria e atualiza o banco antes de atender, como o .NET faz ao iniciar (#297). Fica desligado por padrão: até a inversão (#85), quem faz isso é o .NET. Use só com uma cópia do banco.
- O `-inicio-do-relogio 2026-10-15T01:00:00Z` é **só para teste** (#363): o relógio começa nesse instante e anda dali, como a libfaketime faz com o .NET no contrato. Sem ele, o relógio do sistema. Atrás do .NET, quem passa a opção é o .NET, quando `Go:InicioDoRelogio` (vazio por padrão) tem um instante com fuso; ele passa o instante já validado, mais o tempo desde a partida dele. Ligado, o Go deixa um aviso no log (Configurações › Erros).

## Antes de subir (push)

```bash
gofmt -l .                 # não pode listar nada
go vet ./...
go test -race ./...
go generate ./... && git diff --exit-code -- .   # código gerado e cópia dos scripts do banco em dia
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
GOOS=windows go build ./...   # o executável do Windows compila sem cgo
# De ponta a ponta, com o .NET compilado (dotnet build FinancasLocal.sln -c Release, na raiz):
bash ../../scripts/ci/testar-go-atras-do-dotnet.sh Release
```

A CI roda os cinco primeiros no job "Go" (também quando muda o `banco/` ou o `2-criar-banco.sh`) e o de ponta a ponta no job "API e banco".

**Testes de contrato (#72):** mudou o back-end (.NET ou Go)? Rode `testes/contrato/rodar-dotnet.sh` (Linux, com a libfaketime; detalhes no [README](testes/contrato/README.md)). Ele compila o Go e o sobe atrás do .NET, como no site. Assunto novo grava os casos dele com `--atualizar`, contra o .NET, antes do código Go, e confere o Go com `PREFIXOS_GO="<prefixos do assunto>" testes/contrato/rodar-dotnet.sh` (os mesmos esperados). A CI roda o contrato no job "contrato (.NET)".

Dois testes manuais, que só rodam com as variáveis definidas:

```bash
# Ler uma CÓPIA do banco real inteira, sem mostrar valores (#74)
FINANCAS_COPIA_BANCO=/tmp/copia.db go test ./internal/banco -run CopiaDoBancoReal -v
# Comparar o Go com o .NET rodando, rota por rota: status, cabeçalhos e corpo (#73)
FINANCAS_DOTNET_URL=https://127.0.0.1:5443 FINANCAS_WWWROOT=$PWD/../../publicado/api/wwwroot \
  go test ./internal/servidor -run IgualAoDotnet -v
```

Todo PR do épico traz a seção "🎓 Aula rápida" na descrição.
