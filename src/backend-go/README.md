# Back-end em Go

A API do Finanças Local em Go, que substitui o .NET aos poucos ([épico #70](https://github.com/LucasDesenvGit/financas-local/issues/70), [ADR 0049](../../docs/adr/0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md)). Desde a #231, ele **roda no site atrás do .NET**: o .NET cuida do login, do CSRF, dos segredos e do esquema do banco, sobe o Go como processo filho e repassa a ele só os prefixos de rota de `Go:Prefixos`.

| Pasta | O que tem |
|---|---|
| `cmd/financas` | o ponto de entrada (o `Program.cs` do Go): `-atras-do-dotnet` no site, ou sozinho, com o login do Go (#75) |
| `internal/api` | o `ServeMux` com as rotas de todos os assuntos (o `MapControllers` do Go): uma linha por assunto |
| `internal/saude` | `GET /api/go/saude`, o exemplo que passa pelo caminho todo (front-end → .NET → Go → banco) |
| `internal/cambio` | câmbio pela PTAX (#81, ADR 0007): cliente do Banco Central, conversor para reais, `/api/cambio` e a tarefa de 15 em 15 minutos |
| `internal/cripto` | criptomoedas pelo Mercado Bitcoin (#81, ADR 0048): cliente, `/api/cripto` e a tarefa de 30 em 30 minutos |
| `internal/categorias` | categorias, regras de categorização e regras de descrição (#77): as 14 rotas de `/api/categorias`, `/api/regras` (com o "reaplicar") e `/api/regras-descricao` |
| `internal/contas` | contas (#77): as 9 rotas de `/api/contas` e o saldo de cada conta (o `CalcularSaldosAsync`, com a escala do EF Core) |
| `internal/faturas` | a fatura do cartão do `ContaService` (#77): o ciclo (#144), o mês de cobrança de cada lançamento (#167), o pago e a situação (#251) e a fatura do mês de cada cartão (#323); o painel (#80) usa o mesmo |
| `internal/bancos` | o `BancoService` (#77, ADR 0024): o catálogo e o banco de cada conta (escolhido, da conta do cartão ou detectado pelo nome) |
| `internal/parcelas` | o que reconhece as parcelas da mesma compra (o `ParcelasFuturas.Compra`, com a regex do .NET escrita à mão) |
| `internal/extrato` | transações (#78): as rotas de `/api/transacoes` (as 18: a lista com os filtros, a ordem e os totais, obter, o lançamento manual, as sugestões de categoria, alterar, ajustar, excluir, as alterações em lote, a transferência, o renomear e a categoria em lote com o Desfazer); o modelo da transação fica em `internal/transacoes` |
| `internal/sugestoes` | as sugestões de categoria (#78, o `SugestaoCategoriaService`): pelo histórico e pelas palavras-chave, os grupos, o texto da regra e o aviso dela (#254) |
| `internal/poucoinformativa` | a descrição pouco informativa (#9): as letras úteis, conferidas contra o C# por um gabarito; o painel (#80) usa a mesma |
| `internal/categorizacao` | o `CategorizacaoService` (#77): a categoria pelas regras e pelo mapa da Pluggy, e o tipo pela categoria e pelo valor (ADR 0036); a sincronização (#83) usa o mesmo |
| `internal/regrasdescricao` | o `RegraDescricaoService` (#77, ADR 0030): nome pela descrição do banco, em cadeia (#250) e com faixa de valor (#241), e o texto sugerido para a regra (#123) |
| `internal/desfazer` | o estado de antes de uma alteração em lote, na memória e por 10 minutos, para o "Desfazer" (#78, o `IMemoryCache` do C#) |
| `internal/reservas` | o `ReservaService` (#82, ADRs 0010, 0037 e 0038): as reservas pelo extrato, pela conferência e pelo que o banco informa, e o Méli Dólar pela PTAX; as metas (#79), o painel (#80) e a carteira usam |
| `internal/carteira` | o `CarteiraService` (#82): preço médio, posições, proventos, comparação com a Selic e os investimentos do Open Finance; `GET /api/carteira` e as operações de um ativo |
| `internal/cotacoes` | o `CotacaoService` (#82): cotações, proventos e históricos mensais pelo Yahoo Finance e pela brapi, e o IFIX; o token da brapi só vale depois da troca dos segredos (#85) |
| `internal/selic` | o `SelicService` (#82): a Selic diária e a meta pelo Banco Central, guardadas no banco, e o fator para comparar rentabilidades (em `float64`, como no C#); a carteira usa |
| `internal/transferencias` | o `TransferenciasService` (#77, ADR 0031): transferência entre contas suas fica fora dos relatórios; pagamento de fatura pareado no cartão |
| `internal/tipolancamento` | o `TipoLancamentoService` (#77, ADR 0025): como o dinheiro entrou ou saiu (Pix, fatura, estorno…) |
| `internal/transacoes` | a `Transacao` como as regras a leem e mudam; `Gravar` manda só as colunas que mudaram, como o `SaveChanges` do EF Core |
| `internal/relatorios` | a base do `RelatorioService` (#447, ADRs 0031 e 0036): as transações que entram nos relatórios, o total por categoria, o orçamento do mês contra o gasto, as contas a pagar e o próximo vencimento, a janela dos últimos meses completos e a renda; sem rota, a #79 e a #80 usam |
| `internal/emprestimos` | os totais do `EmprestimoService` (#447, ADR 0014): saldo devedor e parcela mensal de financiamentos e de empréstimos; sem rota |
| `internal/texto` | as normalizações de texto das regras do C# (sem acento, maiúsculas, palavras inteiras) e a ordem de texto da cultura pt-BR, conferidas contra o C# por gabaritos |
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
- **Log:** JSON na saída de erro. O .NET registra cada linha na categoria `Financas.Go`; avisos e erros vão para Configurações › Erros. Para cair na origem certa da tela, ponha o atributo `"origem"` (`"API"`, `"Open Finance"`, `"Investimentos"`, `"Câmbio"`, `"Cripto"` ou `"Sistema"`). Com o Go sozinho (#298), quem grava é o próprio Go (`internal/erros`), com as mesmas origens.
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
- O `-rotinas-de-partida` roda as seis rotinas que acertam dados ao subir (`internal/partida`, #444): regras padrão novas, entradas nos cartões, compras parceladas estornadas, transferências, contas a pagar cobradas sozinhas e trocos sem dúvida, nesta ordem, uma transação por rotina. Também desligado por padrão, e só vale sozinho (não atrás do .NET, que as roda por conta própria: rodar nos dois acertaria duas vezes). Use junto de `-aplicar-esquema` e só com uma cópia do banco. Erro numa rotina vira aviso (Configurações › Erros), a partida segue para as seguintes e o Go sobe do mesmo jeito (o `try/catch` do `Program.cs` parava na primeira; ADR 0049).
- O `-inicio-do-relogio 2026-10-15T01:00:00Z` é **só para teste** (#363): o relógio começa nesse instante e anda dali, como a libfaketime faz com o .NET no contrato. Sem ele, o relógio do sistema. Atrás do .NET, quem passa a opção é o .NET, quando `Go:InicioDoRelogio` (vazio por padrão) tem um instante com fuso; ele passa o instante já validado, mais o tempo desde a partida dele. Ligado, o Go deixa um aviso no log (Configurações › Erros).

## Antes de subir (push)

```bash
gofmt -l .                 # não pode listar nada
go vet ./...
python3 ../../scripts/ci/verificar-regras-go.py   # float64, time.Now() e time.Sleep de teste só com a exceção escrita (#457)
go test -race ./...
go generate ./... && git diff --exit-code -- .   # código gerado e cópia dos scripts do banco em dia
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
# e com GOOS=windows (os _windows.go, #449): instale antes, porque "GOOS=windows go run" compila o govulncheck para Windows
go install golang.org/x/vuln/cmd/govulncheck@v1.9.0 && GOOS=windows "$(go env GOPATH)/bin/govulncheck" ./...
GOOS=windows go build ./...   # o executável do Windows compila sem cgo
# De ponta a ponta, com o .NET compilado (dotnet build FinancasLocal.sln -c Release, na raiz):
bash ../../scripts/ci/testar-go-atras-do-dotnet.sh Release
```

Teste de mutação (#373, #456, ADR 0051): no PR, o workflow "Teste de mutação (Go)" mede só os pacotes que o PR mudou; toda segunda, todos. À mão, antes de subir um pacote com regra:

```bash
python3 ../../scripts/ci/mutacao-go.py internal/parametros   # um pacote (~1 min); sem argumento, todos de mutacao-go.txt
```

A CI roda os seis primeiros no job "Go" (também quando muda o `banco/` ou o `2-criar-banco.sh`) e o de ponta a ponta no job "API e banco". No job "Go", o `go test` também confere a cobertura de cada pacote (o piso de `cobertura-minima.txt`, que só sobe) e o tempo dos testes. Como ver localmente: `CONTRIBUINDO.md`, "Testes do back-end (Go)".

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
