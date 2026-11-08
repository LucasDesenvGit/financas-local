# Back-end em Go

O site do Finanças Local: serve o Angular e a `/api`, num executável só (`financas-go`), como serviço do sistema. Um pacote por assunto em `internal/`, com HTTP, regra e banco dentro; o padrão de um assunto está no [`internal/README.md`](internal/README.md).

*Histórico:* o Go substituiu a API em .NET um assunto por vez ([épico #70](https://github.com/LucasDesenvGit/financas-local/issues/70), [ADR 0049](../../docs/adr/0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md)), e o .NET saiu na #86 (a última versão com ele está na tag `v-ultimo-dotnet`). Por isso alguns comentários e gabaritos ainda citam o C#.

| Pasta | O que tem |
|---|---|
| `cmd/financas` | o ponto de entrada: as opções, a ligação das peças e o subcomando `gerar-certificado` |
| `internal/api` | o `ServeMux` com as rotas de todos os assuntos : uma linha por assunto |
| `internal/plataforma/saude` | `GET /api/go/saude`, o exemplo que passa pelo caminho todo (login → rota → banco), com a versão e o relógio |
| `internal/patrimonio/cambio` | câmbio pela PTAX (#81, ADR 0007): cliente do Banco Central, conversor para reais, `/api/cambio` e a tarefa de 15 em 15 minutos |
| `internal/patrimonio/cripto` | criptomoedas pelo Mercado Bitcoin (#81, ADR 0048): cliente, `/api/cripto` e a tarefa de 30 em 30 minutos |
| `internal/ajustes/categorias` | categorias, regras de categorização e regras de descrição (#77): as 14 rotas de `/api/categorias`, `/api/regras` (com o "reaplicar") e `/api/regras-descricao` |
| `internal/movimentacao/contas` | contas (#77): as 9 rotas de `/api/contas` e o saldo de cada conta, com a escala de casas decimais que o banco guarda |
| `internal/movimentacao/faturas` | a fatura do cartão (#77): o ciclo (#144), o mês de cobrança de cada lançamento (#167), o pago e a situação (#251) e a fatura do mês de cada cartão (#323); o painel (#80) usa o mesmo |
| `internal/movimentacao/bancos` | os bancos (#77, ADR 0024): o catálogo e o banco de cada conta (escolhido, da conta do cartão ou detectado pelo nome) |
| `internal/movimentacao/parcelas` | o que reconhece as parcelas da mesma compra ("03/10", "PARC 3 DE 10"), com a expressão escrita à mão |
| `internal/movimentacao/extrato` | transações (#78): as rotas de `/api/transacoes` (as 18: a lista com os filtros, a ordem e os totais, obter, o lançamento manual, as sugestões de categoria, alterar, ajustar, excluir, as alterações em lote, a transferência, o renomear e a categoria em lote com o Desfazer); o modelo da transação fica em `internal/movimentacao/transacoes` |
| `internal/movimentacao/sugestoes` | as sugestões de categoria (#78): pelo histórico e pelas palavras-chave, os grupos, o texto da regra e o aviso dela (#254) |
| `internal/movimentacao/poucoinformativa` | a descrição pouco informativa (#9): as letras úteis, conferidas por um gabarito; o painel (#80) usa a mesma |
| `internal/ajustes/categorizacao` | a categorização (#77): a categoria pelas regras e pelo mapa da Pluggy, e o tipo pela categoria e pelo valor (ADR 0036); a sincronização (#83) usa o mesmo |
| `internal/ajustes/regrasdescricao` | as regras de descrição (#77, ADR 0030): nome pela descrição do banco, em cadeia (#250) e com faixa de valor (#241), e o texto sugerido para a regra (#123) |
| `internal/plataforma/desfazer` | o estado de antes de uma alteração em lote, na memória e por 10 minutos, para o "Desfazer" (#78); reiniciar o serviço perde o Desfazer |
| `internal/patrimonio/reservas` | as reservas (#82, ADRs 0010, 0037 e 0038), com as 3 rotas de `/api/reservas`: as reservas pelo extrato, pela conferência e pelo que o banco informa, e o Méli Dólar pela PTAX; as metas (#79), o painel (#80) e a carteira usam |
| `internal/patrimonio/carteira` | a carteira de investimentos (#82): preço médio, posições, proventos, comparação com a Selic e os investimentos do Open Finance; as 18 rotas de `/api/carteira` (a carteira, os ativos e as operações, a configuração, os FIIs, a importação do extrato, a atualização das cotações e a conferência, #299) |
| `internal/patrimonio/cotacoes` | as cotações (#82): cotações, proventos e históricos mensais pelo Yahoo Finance e pela brapi, e o IFIX |
| `internal/patrimonio/selic` | a Selic (#82): a Selic diária e a meta pelo Banco Central, guardadas no banco, e o fator para comparar rentabilidades (em `float64`: é um fator, não dinheiro); a carteira usa |
| `internal/movimentacao/transferencias` | as transferências (#77, ADR 0031): transferência entre contas suas fica fora dos relatórios; pagamento de fatura pareado no cartão |
| `internal/movimentacao/tipolancamento` | o tipo de lançamento (#77, ADR 0025): como o dinheiro entrou ou saiu (Pix, fatura, estorno…) |
| `internal/movimentacao/transacoes` | a `Transacao` como as regras a leem e mudam; `Gravar` manda só as colunas que mudaram |
| `internal/movimentacao/conferenciabanco` | o "Conferir com o banco" (#172, #445): por conta, fatura e investimento, o valor do site e as pistas de por que o app do banco mostra outro; `/api/conferencia-banco` |
| `internal/movimentacao/contasagendadas` | as contas a pagar (#79): as 9 rotas de `/api/contas-agendadas`, os avisos das recorrentes (#158, ADR 0043) e as sugestões do extrato (#62, #205, #241) |
| `internal/movimentacao/cobrancas` | a conciliação da conta a pagar com as cobranças do extrato (#158, ADR 0043); sem rota: as contas a pagar, a partida e a sincronização (#83) usam |
| `internal/planejamento/orcamentos` | os orçamentos do mês (#79): as 6 rotas de `/api/orcamentos` e a sugestão por categoria (#39) |
| `internal/planejamento/metas` | as metas de economia (#79): as 8 rotas de `/api/metas`, o progresso pelo saldo da conta ou do cofrinho (#89), a prioridade, os aportes (#318) e o caixa do plano (#90, #149) |
| `internal/analises/relatorios` | a base dos relatórios (#447, ADRs 0031 e 0036): as transações que entram nos relatórios, o total por categoria, o orçamento do mês contra o gasto, as contas a pagar e o próximo vencimento, a janela dos últimos meses completos e a renda; sem rota, a #79 e a #80 usam |
| `internal/patrimonio/emprestimos` | os totais dos empréstimos (#447, ADR 0014): saldo devedor e parcela mensal de financiamentos e de empréstimos; sem rota |
| `internal/plataforma/texto` | as normalizações de texto das regras (sem acento, maiúsculas, palavras inteiras) e a ordem alfabética do pt-BR, conferidas por gabaritos |
| `internal/partida` | as seis rotinas que acertam dados ao subir (#444): só a ordem, a transação de cada uma e o que fazer quando falha; a regra de cada uma fica no pacote do assunto. Ligadas por `-rotinas-de-partida`, na linha do serviço |
| `internal/openfinance/pluggy` | o cliente HTTP da Pluggy, sem SDK (#83, ADR 0006): sessão, paginação, novas tentativas e os modelos das respostas |
| `internal/openfinance/openfinance` | as rotas do Open Finance (#83): credenciais, conexões, status, histórico e o mapa de categorias; quem traz os dados é o `internal/openfinance/sincronizacao` |
| `internal/openfinance/sincronizacao` | a sincronização e a tarefa periódica (#83): contas, faturas, transações, investimentos, empréstimos e carteira, uma rodada por vez; sem rota |
| `internal/plataforma/erros` | os erros do sistema (#298, ADR 0022): o handler do `slog` que manda avisos e erros para a fila, o gravador em `ErroSistema` e as rotas de `/api/erros` |
| `internal/plataforma/servicosistema` | o que faz o executável rodar como serviço (#84): o gerenciador de serviços e o Log de Eventos no Windows; o SIGTERM do systemd no Linux |
| `internal/plataforma/tarefas` | tarefas em segundo plano (câmbio, cripto e a sincronização), com `context`, espera ao parar e `recover` por ciclo |
| `internal/plataforma/servidor` | HTTPS local, cabeçalhos de segurança (a CSP), erros, os arquivos do front-end e o certificado |
| `internal/plataforma/sessao` | quem está logado, no contexto da requisição |
| `internal/plataforma/autenticacao` | login, sessão em cookie assinado, CSRF, limite por IP, bloqueio e auditoria (#75) |
| `internal/plataforma/respostas` | as respostas comuns a todos os assuntos: ler o JSON e validar os campos, o 400 com as mensagens de validação (`ValidationProblemDetails`), o 415 e o `{"mensagem"}` que o front-end mostra |
| `internal/plataforma/auditoria` | a auditoria: grava em `AuditoriaAcesso` o evento, o login, o IP e o navegador |
| `internal/plataforma/configuracao` | ler e gravar a tabela `Configuracao` (#76), chave e valor em texto, convertidos para número, sim/não e data |
| `internal/ajustes/parametros` | a Parametrização do site (#76): o catálogo, a gravação que valida tudo antes e o diagnóstico |
| `internal/plataforma/segredos` | os segredos (#76): AES-256-GCM com prefixo `v2:`, chave protegida por DPAPI no Windows e `0600` no Linux |
| `internal/plataforma/config` | leitura do `appsettings.json` e do `appsettings.Production.json` da instalação |
| `internal/plataforma/banco` | SQLite no formato do banco (o de quando o EF Core gravava), transação, consultas do `sqlc`, esquema ao iniciar ([README](internal/plataforma/banco/README.md)) |
| `internal/plataforma/tipos` | `Decimal`, `Data`, `DataHora`, `Guid` no formato em que o banco os guarda (texto) |
| `internal/plataforma/relogio` | fuso de Brasília, injetável; o relógio de teste do contrato (`APartirDe`, #363) |
| `internal/plataforma/versao` | a versão, gravada na compilação pelo `-ldflags` |
| `internal/arquitetura` | o teste de arquitetura (ADR 0056, #476): regra sem HTTP, mapa de contextos e módulos. **Só local** (`go test -tags arquitetura ./internal/arquitetura/`), a CI não roda |
| `internal/plataforma/testeapoio` | só para testes (`_test.go`): banco temporário com o esquema real, fábrica de dados inventados, relógio parado em Brasília e handler chamado com o usuário logado ([README](internal/plataforma/testeapoio/README.md), #369) |
| `testes/contrato` | testes de contrato: o que o site responde e grava, caso a caso (a regressão do site; os primeiros esperados vieram do .NET) ([README](testes/contrato/README.md)) |

## Como roda no site

```
navegador ──HTTPS 127.0.0.1:5443──► financas-go (Angular do wwwroot e /api) ──► SQLite (WAL)
```

- **Quem sobe:** o serviço do sistema executa `financas-go -aplicar-esquema -rotinas-de-partida`. O executável fica em `publicado/api` (scripts) ou em `%ProgramFiles%\FinancasLocal` (`.msi`), com o `appsettings.json` deste módulo e o `appsettings.Production.json` gerado pela instalação.
- **Partida:** aplica o esquema do banco (cópia antes de mudar), roda as rotinas de partida, carrega o certificado e liga as tarefas em segundo plano (câmbio, cripto e a sincronização).
- **Login:** o do Go (`internal/plataforma/autenticacao`, #75), na frente de todas as rotas; a rota lê o usuário com `sessao.Usuario(ctx)`.
- **Parada:** SIGINT, SIGTERM ou o `Stop` do gerenciador de serviços do Windows: espera os pedidos em andamento, para as tarefas e fecha o banco.
- **Log:** texto na saída de erro (journald no Linux, Log de Eventos no Windows); avisos e erros vão também para Configurações › Erros (`internal/plataforma/erros`, #298). Para cair na origem certa da tela, ponha o atributo `"origem"` (`"API"`, `"Open Finance"`, `"Investimentos"`, `"Câmbio"`, `"Cripto"` ou `"Sistema"`).
- **Banco:** em WAL, com datas, dinheiro e booleanos no formato que o EF Core gravava quando o site era em .NET (o banco não mudou na troca). O Go só cria ou muda tabela pelos scripts de `banco/` (`-aplicar-esquema`).

### Rota nova

1. O handler no pacote do assunto (`internal/<area>/<assunto>`), com as dependências pelo construtor. O pacote exporta `Registrar(mux, <só o que ele usa>)`, que põe as rotas dele no mux (`mux.Handle("GET /api/<assunto>/{id}", ...)`); o exemplo é o `internal/plataforma/saude`.
2. **Uma linha** em `internal/api/api.go`: `<assunto>.Registrar(mux, d.DB, d.Log, d.Relogio)`. É só isso que os assuntos feitos em paralelo dividem; no merge, conflito ali é sempre "fique com as duas linhas". Hora vem do `d.Relogio`, nunca do `time.Now()`.
3. Os casos de contrato, num número livre de `20` a `89`, gravados com `testes/contrato/rodar.sh --atualizar` e conferidos no `git diff`, como no [README do contrato](testes/contrato/README.md#adicionar-um-caso).
4. Os testes do assunto (ADR 0051), com o apoio comum de `internal/plataforma/testeapoio` ([README](internal/plataforma/testeapoio/README.md)): banco temporário, fábrica de dados e handler chamado com o usuário logado.
5. Tarefa em segundo plano nova: entra em `tarefas.Conhecidas` e no `iniciarTarefas` do `cmd/financas`.

## Rodar como serviço

O mesmo executável roda como serviço nos dois sistemas (#84), sem mudar de opção: no Windows, ele percebe que quem o iniciou foi o gerenciador de serviços, responde a `Stop` e `Shutdown` e escreve avisos e erros no Log de Eventos (origem `FinancasLocal`); no Linux, para com o SIGTERM do systemd, e o log vai para o journald. Nos scripts, o que o serviço executa está num ponto só: `exe_servico` no `3-publicar-atualizar.sh` e `$binPath` no `instalar-windows.ps1`.

## Certificado

`financas-go gerar-certificado -pfx <caminho>`, com a senha na entrada padrão, é o passo do certificado da instalação: reaproveita o `.pfx` que abre com a senha e vale por mais de 30 dias, gera outro no caso contrário (1 ano, para `127.0.0.1`, `::1` e `localhost`) e grava a parte pública no `.pem` ao lado. Escreve `gerado` ou `reaproveitado`.

## Rodar

Precisa do Go 1.27.3 ou mais novo (as anteriores têm falhas de segurança na biblioteca padrão).

**Em desenvolvimento**, com uma **cópia** do banco, ao lado do site instalado (que fica na 5443):

```bash
cd src/backend
go run ./cmd/financas -endereco 127.0.0.1:5444 -wwwroot ../frontend/dist/financas-web/browser -certificado-temporario \
  -banco <cópia>/FinancasLocal.db -aplicar-esquema
```

- O `-wwwroot` aponta para o build do Angular (`npm run build` em `src/frontend`). Com o `npm start` (o Angular em `http://localhost:4200`), use `-endereco localhost:7443`, a porta do `proxy.conf.json`, e qualquer pasta no `-wwwroot` (`internal/plataforma/servidor/testdata/wwwroot`).
- O `-certificado-temporario` cria um certificado só em memória, e o navegador vai avisar que não confia nele.
- Para usar o `.pfx` e os caminhos de uma instalação, aponte `-pasta` para a pasta da publicação (onde está o `appsettings.Production.json`).
- O `-aplicar-esquema` cria e atualiza o banco antes de atender (#297). A linha do serviço instalado sempre liga; em desenvolvimento, só com uma cópia do banco.
- O `-rotinas-de-partida` roda as seis rotinas que acertam dados ao subir (`internal/partida`, #444): regras padrão novas, entradas nos cartões, compras parceladas estornadas, transferências, contas a pagar cobradas sozinhas e trocos sem dúvida, nesta ordem, uma transação por rotina. Use junto de `-aplicar-esquema` e só com uma cópia do banco. Erro numa rotina vira aviso (Configurações › Erros), a partida segue para as seguintes e o site sobe do mesmo jeito.
- O `-inicio-do-relogio 2026-10-15T01:00:00Z` é **só para teste** (#363): o relógio começa nesse instante e anda dali; o contrato usa. Sem ele, o relógio do sistema. Ligado, o Go deixa um aviso no log (Configurações › Erros).

## Antes de subir (push)

```bash
gofmt -l .                 # não pode listar nada
go vet ./...
python3 ../../scripts/ci/go/verificar-regras-go.py   # float64, time.Now() e time.Sleep de teste só com a exceção escrita (#457)
go test -race ./...
go generate ./... && git diff --exit-code -- .   # código gerado e cópia dos scripts do banco em dia
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
# e com GOOS=windows (os _windows.go, #449): instale antes, porque "GOOS=windows go run" compila o govulncheck para Windows
go install golang.org/x/vuln/cmd/govulncheck@v1.9.0 && GOOS=windows "$(go env GOPATH)/bin/govulncheck" ./...
GOOS=windows go build ./...   # o executável do Windows compila sem cgo
# De ponta a ponta: o contrato inteiro contra o Go (#72) e o site ligando como o serviço (#309)
bash testes/contrato/rodar.sh
bash ../../scripts/ci/ligar-api.sh
```

Teste de mutação (#373, #456, ADR 0051): no PR, o workflow "Teste de mutação (Go)" mede só os pacotes que o PR mudou; toda segunda, todos. À mão, antes de subir um pacote com regra:

```bash
python3 ../../scripts/ci/go/mutacao-go.py internal/ajustes/parametros   # um pacote (~1 min); sem argumento, todos de mutacao-go.txt
```

A CI roda todos menos os de ponta a ponta no job "Go" (também quando muda o `banco/` ou o `2-criar-banco.sh`); o contrato, no job "contrato", e o site ligando, no job "API e banco". O job "Go (Windows)" roda o `go vet` e o `go test` num Windows de verdade, para os arquivos `_windows.go` (DPAPI dos segredos, serviço do Windows; #459), sem o `-race`, que lá pede cgo. No job "Go", o `go test` também confere a cobertura de cada pacote (o piso de `cobertura-minima.txt`, que só sobe) e o tempo dos testes. Como ver localmente: `CONTRIBUINDO.md`, "Testes do back-end (Go)".

**Testes de contrato (#72):** mudou o back-end? Rode `testes/contrato/rodar.sh` (Linux ou Git Bash do Windows; detalhes no [README](testes/contrato/README.md)). Ele compila o Go e o sobe como o serviço instalado. Os esperados são a regressão do site (os primeiros vieram do .NET, até a #86): um esperado só muda com o motivo no PR (ADR 0051). A CI roda o contrato no job "contrato".

Um teste manual, que só roda com a variável definida:

```bash
# Ler uma CÓPIA do banco real inteira, sem mostrar valores (#74)
FINANCAS_COPIA_BANCO=/tmp/copia.db go test ./internal/plataforma/banco -run CopiaDoBancoReal -v
```

Todo PR com Go traz a seção "🎓 Aula rápida" na descrição.
