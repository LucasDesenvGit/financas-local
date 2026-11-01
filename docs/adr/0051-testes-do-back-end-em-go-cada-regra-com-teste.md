# 0051. Testes do back-end em Go: cada regra com teste unitário, além do contrato

- **Status:** Aceita
- **Data:** 2026-10-23 (issue #368, épico de testes #374)
- **Relacionadas:** [0049](0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md) (back-end em Go, bibliotecas e testes de contrato da #72); [0013](0013-testes-do-front-end-com-jest.md) e [0041](0041-testes-no-navegador-com-playwright.md) (testes do front-end, de onde vêm "um comportamento testado num lugar só" e "bug corrigido ganha teste"); [0047](0047-ci-paralela-so-testes-afetados-no-pr.md) (o job "Go" da CI); épico #70

## Contexto

**O C# não tem teste nenhum.** O back-end .NET chegou a 128 rotas e cerca de 15,3 mil linhas sem teste automatizado ([0049](0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md), Contexto). Por isso a migração para Go teve de começar pelos testes de contrato (#72), só para saber o que o sistema faz hoje.

**O contrato não basta sozinho:**
- ele olha de fora: status, JSON e as linhas que mudaram no banco, comparados com o que o .NET respondeu e gravou;
- cada situação é um caso inteiro (banco fictício e requisição), gravado com o .NET de pé (`testes/contrato/rodar-dotnet.sh`: Linux, libfaketime, sem rede). Caso-limite de cálculo (arredondamento, dia 31, virada do ano, `NULL`, valor negativo) custa pouco num teste unitário e muito num caso de contrato;
- depois da #86 o .NET sai, e regra nova ou mudada não tem mais de onde gravar um esperado. Daí em diante, só o teste do próprio Go protege a regra.

**O que já existe** (conferido em 23/10/2026, no `develop`):
- os pacotes com lógica têm teste. `api`, `saude`, `sessao` e `versao` não têm arquivo de teste próprio, e os três primeiros passam pelos testes de `cmd/financas` e `internal/servidor`. A cobertura em `internal/` é de 78,6%, somando todos os testes com `-coverpkg` (épico #374);
- a CI roda `go test -race ./...` no job "Go" ([0047](0047-ci-paralela-so-testes-afetados-no-pr.md)) e o contrato contra o .NET no job "contrato (.NET)";
- padrões que esta ADR transforma em regra:
  - tabela com `t.Run`: `internal/relogio/relogio_test.go`, `internal/tipos/tipos_test.go`;
  - relógio parado: `relogio.Fixo`, como o `instanteDosTestes` de `internal/banco/esquema_banco_test.go:100`;
  - banco num arquivo do `t.TempDir()`, com a estrutura real: `bancoDeTeste`, em `internal/banco/banco_test.go:22`;
  - handler chamado com `httptest`: `internal/servidor/atras_do_dotnet_test.go`;
  - gabaritos gravados pela referência, e não pelo Go: as sondas que rodam o próprio EF Core (`internal/tipos/testdata/sonda-efcore`, `internal/banco/testdata/sonda-esquema`) e os esperados do contrato (`testes/contrato/esperado/`);
- **prova por mutação, à mão:** o PR #360 estragou o porte do esquema de propósito 12 vezes, e cada mudança foi pega por pelo menos um teste;
- **ainda não existe:** fuzz, benchmark, `t.Parallel()`, cobertura medida na CI, trava de tempo. E **nenhuma regra obriga** o teste: uma etapa do #70 pode entrar só com o contrato.

Três devs vão começar #75, #77 e #81 em paralelo. A regra precisa existir antes do código.

## Alternativas consideradas

- **Só o teste de contrato.** É a regra que já existe ("endpoint sem caso de contrato não está pronto"). Contra:
  - não chega aos casos-limite de cálculo sem um caso inteiro para cada um;
  - quando falha, diz que o JSON mudou, e não qual regra quebrou;
  - gravar e conferir contra o .NET pede o .NET de pé, no Linux (libfaketime e `unshare`);
  - some como referência depois da #86.

  Descartada **como regra única**: o contrato continua obrigatório, ao lado do unitário.
- **Mock do banco**: o repositório atrás de uma interface com uma implementação falsa, ou um driver que finge o SQL (como o `go-sqlmock`). Contra:
  - **boa parte do que pode errar na migração está no banco** (pegadinhas da skill `migrar-para-go`, seção 4): o formato do EF Core em `TEXT`, a cascata que o EF fazia sozinho, o `LIKE` com acento, a ordenação, a chave estrangeira. Um mock devolve o que o teste mandou e não pega nada disso;
  - **o SQLite de verdade é rápido.** Medido aqui: um banco novo num arquivo temporário, com o esquema inteiro dos scripts embutidos (`TestAplicarEsquemaComOsScriptsEmbutidos`), leva cerca de 0,02 s, e o pacote `internal/banco` inteiro, 0,1 s;
  - **o guia do Go pede o contrário:** "Do not define interfaces on the implementor side of an API 'for mocking'", e sim testar pela API pública da implementação real ([Go Code Review Comments, Interfaces](https://go.dev/wiki/CodeReviewComments#interfaces));
  - seria mais uma dependência fora da lista da [0049](0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md).

  **O `:memory:` do SQLite também fica de fora.** Cada conexão a `:memory:` abre um banco diferente ([sqlite.org](https://www.sqlite.org/inmemorydb.html)), e o `*sql.DB` é um pool de conexões ([database/sql](https://pkg.go.dev/database/sql#DB)): uma consulta pode cair numa conexão sem as tabelas. O WAL e o `busy_timeout` também só existem com arquivo.
- **`testify` (ou outra biblioteca de asserção) × só a biblioteca padrão.** O `testify` traz `assert`, `require`, `mock` e `suite` ([repositório](https://github.com/stretchr/testify)): mensagens prontas e menos linhas. Contra:
  - **a [0049](0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md) fecha a lista de bibliotecas.** Ela diz que "dependência fora desta lista precisa de atualização desta ADR", prefere a biblioteca padrão ("sem framework, menos dependência", nas rotas), e o `CLAUDE.md` repete ("só as da ADR");
  - **a documentação do Go desaconselha.** O FAQ explica por que o `testing` não tem asserção: deixar os outros casos rodarem depois de uma falha e escrever uma boa mensagem de erro ([FAQ](https://go.dev/doc/faq#testing_framework)). O guia de testes diz "Avoid the use of 'assert' libraries", que "make it too easy to write imprecise tests" ([Go Test Comments](https://go.dev/wiki/TestComments#assert-libraries));
  - o `require` encerra o teste na primeira falha ([repositório](https://github.com/stretchr/testify)). Numa tabela, queremos ver todos os casos que falharam ([Keep going](https://go.dev/wiki/TestComments#keep-going));
  - quem lê o teste aprende uma linguagem só, e aprender Go é um dos objetivos do épico.

  **O `go-cmp`** (`cmp.Diff`), que o guia de testes recomenda para comparar estruturas ([Go Test Comments](https://go.dev/wiki/TestComments#equality-comparison-and-diffs)), **também fica de fora por ora.** O JSON é comparado pelo comparador do contrato (sem `float64`), e o resto com `==`, `slices.Equal`, `maps.Equal` ou `reflect.DeepEqual`. Se um assunto precisar de diff de estrutura grande, ele entra atualizando a 0049. *Inferido.*
- **Uma pasta separada para os testes** (como `testes/` no front-end). Contra: o `go test` compila cada pacote junto com os `*_test.go` da mesma pasta ([cmd/go, Test packages](https://pkg.go.dev/cmd/go#hdr-Test_packages)), e o teste na pasta do pacote enxerga o que não é exportado. A exceção continua sendo o contrato, que testa o servidor de fora.

## Decisão

**Toda regra do back-end em Go nasce com teste unitário**, além do caso de contrato do endpoint. Um comportamento é testado num lugar só, na camada mais barata que o alcança, como no front-end (#204). Só a biblioteca padrão (`testing`, `net/http/httptest`, `testing/synctest`), sem asserção nem mock de terceiros.

### Onde cada teste mora

| Camada | Testa | Onde e como |
|---|---|---|
| **Unitário** | regra de negócio, cálculo, conversão, validação | `*_test.go` no pacote do assunto (`internal/<assunto>`), em tabela (*table-driven*): um `[]struct` de casos e um `t.Run(caso.nome, ...)` para cada ([Go Wiki](https://go.dev/wiki/TableDrivenTests)) |
| **Integração** | consulta SQL, transação, cascata, formato gravado | SQLite **de verdade**, num arquivo do `t.TempDir()`, com o **esquema real** (`banco.AplicarEsquema` e `banco.Abrir`, #297), fechado no `t.Cleanup`. Sem mock de banco, sem `:memory:`, sem `CREATE TABLE` escrito no teste |
| **HTTP** | handler: status, validação, mensagem de erro, usuário ausente | o handler chamado direto com `httptest.NewRequestWithContext` e `httptest.NewRecorder` ([httptest](https://pkg.go.dev/net/http/httptest)), com o usuário no contexto (`sessao.ComUsuario`, #231). Sem porta aberta |
| **Contrato** | o mesmo status, o mesmo JSON e o mesmo banco do .NET | `testes/contrato` (#72), casos em `casos/*.json` e esperados gravados do .NET |
| **Dependência externa** | Pluggy, Yahoo/brapi, BCB, Mercado Bitcoin | o cliente recebe a URL base e o `http.Client` pelo construtor, e o teste usa o **`testeapoio.ServicoDeFora`** (#460): um servidor HTTPS de teste (`httptest.NewTLSServer`) com as rotas cadastradas, que responde com arquivos de `testdata/` ou com os erros prontos (tempo esgotado, 401, 429, 500, JSON quebrado, página vazia), confere os cabeçalhos que o .NET manda e guarda os pedidos, com o valor dos cabeçalhos de segredo trocado por `<segredo>`. Pedido sem rota cadastrada é erro do teste |
| **Só do Windows** | o que está num `_windows.go` (DPAPI dos segredos, descrição do sistema, registro do gerenciador; depois, o serviço do Windows da #84) | teste num `_windows_test.go` do mesmo pacote (o `go` só o compila no Windows: [Build constraints](https://pkg.go.dev/cmd/go#hdr-Build_constraints)). Roda no job **"Go (Windows)"** (back-end, #459) e no **"gerenciador em Go"**, os dois em `windows-latest`, sem o `-race` (no Windows ele pede cgo). O que grava no registro usa uma chave **só do teste**, apagada no `t.Cleanup` (roda na máquina de quem desenvolve). Fica de fora o que pede administrador (UAC, instalar serviço), com o porquê no teste |

- **O apoio comum** (banco temporário, fábrica de dados inventados, ajudantes de relógio e de HTTP) é da #369: um pacote dentro de `internal/`, importado só por `_test.go`. O `go build` ignora os `_test.go` ([cmd/go](https://pkg.go.dev/cmd/go#hdr-Compile_packages_and_dependencies)), então o apoio não entra no executável. O pacote é o `internal/testeapoio` ([README](../../src/backend-go/internal/testeapoio/README.md), #369): `NovoBanco`, a `Fabrica`, os relógios parados em Brasília e o `Chamar` com o usuário logado, que confere o JSON pelo comparador do contrato. Os pacotes que ele importa (`banco`, `relogio`, `tipos`, `sessao`) não podem importá-lo nos próprios testes (ciclo), e continuam com um ajudante local, como o `bancoDeTeste`.
- **Não repita** num lugar o que outro já confere. O contrato prova a equivalência com o .NET, com pelo menos um caso de sucesso e um por status de erro. O unitário cobre as variações da regra. O HTTP cobre cada ramo de validação.
- **Gabarito** (*golden file*) fica em `testdata/` (o `go` ignora essa pasta: [cmd/go, Test packages](https://pkg.go.dev/cmd/go#hdr-Test_packages)) e é regravado só por uma opção explícita (`-atualizar`, como no contrato; a biblioteca padrão faz o mesmo com `-update`, no [`go/printer`](https://github.com/golang/go/blob/master/src/go/printer/printer_test.go)).
  - **O gabarito vem da referência, nunca do código que ele testa.** A referência é o .NET, ou o EF Core pela sonda. Senão o teste só confirma o próprio Go.
  - Depois da #86, sem o .NET, o gabarito do Go é revisado linha a linha no PR que o grava.

### Pronto quando (toda etapa do #70)

1. **Cada regra do levantamento** (lista "Regras de negócio", seção 2 da skill `migrar-para-go`) tem pelo menos uma linha na tabela de algum teste unitário. A descrição do PR traz a tabela **regra → teste** (nome do teste e do caso). *Inferido: o formato da tabela.*
2. **Cada ponto de risco de equivalência** do levantamento (arredondamento, ordenação, texto, fuso, `null`, cascata) tem um teste que o exercita: unitário, de integração ou de contrato.
3. **Cada endpoint tem caso de contrato**, inclusive os de erro (400, 401, 404, 409). É a regra que já existe ([0049](0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md)).
4. **Cada consulta que grava, apaga em cascata, filtra texto ou ordena** tem teste de integração com o SQLite real.
5. **Os casos-limite que se aplicam** (lista abaixo) estão na tabela. O PR diz quais não se aplicam.
6. **O teste novo provou que falha** (abaixo), com a lista das mutações no PR.
7. `gofmt`, `go vet` e `go test -race` limpos, dentro dos pisos de cobertura (#370) e de tempo (#371), que a CI confere.

O detalhe de cada etapa está num comentário do épico #70.

### Casos-limite obrigatórios, quando se aplicam

- **Valor:** negativo, zero, 8 casas decimais, meio centavo (`0,125`: o `Math.Round` do C# arredonda para o par por padrão, [Microsoft Learn](https://learn.microsoft.com/dotnet/api/system.math.round); em Go, o `RoundBank`).
- **Ausência:** `NULL` no banco e no JSON, e lista vazia, que sai `[]` e nunca `null`.
- **Hora:** meia-noite em Brasília × UTC (23h30 em Brasília já é o dia seguinte em UTC), e o horário de verão antigo (até 2019).
- **Calendário:** virada de mês e de ano; dias 29, 30 e 31, inclusive fevereiro de ano bissexto e o "mesmo dia no mês seguinte" a partir do dia 31.
- **Texto:** acento e maiúsculas ("Ação" × "acao"), porque o `LIKE` do SQLite só ignora a caixa em ASCII ([sqlite.org](https://www.sqlite.org/lang_expr.html#the_like_glob_regexp_match_and_extract_operators)).

### Bug corrigido ganha teste de regressão

- Vale a mesma regra do front-end ([0013](0013-testes-do-front-end-com-jest.md)): o teste fica no pacote do assunto, com o número da issue no comentário (`// #NNN: ...`), e **falha antes da correção**. Rode-o antes de corrigir e veja falhar.
- **Bug do C# achado no porte não é corrigido em silêncio** (skill `migrar-para-go`). O teste do Go fixa o comportamento de hoje, igual ao .NET, com um comentário que aponta a issue aberta. Quando a issue for decidida, o teste muda junto.

### Teste novo prova que falha

- Estrague o **código** (não o teste) de propósito, rode, veja o teste falhar e desfaça. Exemplos: trocar `RoundBank` por `Round`, tirar um filtro, inverter uma comparação, devolver `nil` no lugar de `[]`.
- Foi assim no PR #360: 12 mutações, todas pegas. A ADR [0041](0041-testes-no-navegador-com-playwright.md) faz o mesmo com o CSS do front-end.
- **Teste que nenhuma mutação derruba é refeito.** Em geral, os dados do teste não exercitam o caso.
- Pelo menos uma mutação por regra de cálculo, listada no PR com o teste que a pegou. *Inferido: a quantidade.*

### Teste de mutação automático (#373)

A mutação à mão continua no PR. A ferramenta mede, toda semana, se os testes de cada pacote continuam pegando erro.

- **Ferramenta: [gremlins](https://github.com/go-gremlins/gremlins) v0.6.0** (Apache-2.0), rodada com `go run …@v0.6.0`, como o `govulncheck`. Não entra no `go.mod`.
  - Trabalha numa cópia do módulo: o código dele diz que a cópia "allows Gremlins not to work in the actual source directory messing up with the source code files" ([`workdir.go`](https://github.com/go-gremlins/gremlins/blob/main/internal/engine/workdir/workdir.go)).
  - Só testa a mutação que algum teste do pacote cobre: mede antes a cobertura.
  - Roda em paralelo, tem limiar e grava o resultado em JSON.
- **Alternativas consideradas** (25/10/2026, pela página de cada uma no GitHub):
  - [go-mutesting da Avito](https://github.com/avito-tech/go-mutesting) (v2.3.1, dez/2025): tem remoção de comando, que o gremlins não tem. Mas o README diz que ele faz "Replace the original file with the mutation": interrompido no meio, deixa o código estragado.
  - [ooze](https://github.com/gtramontina/ooze) (sem versão publicada): é biblioteca, entra no `go.mod` e roda como teste com build tag. Dependência nova no módulo só para medir.
  - [go-mutesting original](https://github.com/zimmski/go-mutesting): sem versão desde 2021.
- **Onde e quando:** `scripts/ci/mutacao-go.py`, nos pacotes de `scripts/ci/mutacao-go.txt` (regra de negócio e dinheiro), pelo workflow "Teste de mutação (Go)":
  - **no PR que mexe no back-end (#456): só os pacotes da lista que o PR mudou** (código, teste ou `testdata/` deles), comparando com a base como o `testes-afetados.py` (ADR 0047). Falha abaixo do piso, antes do merge. Se o PR só muda a própria mutação (script, listas, workflow), mede o `internal/tipos`, para provar que ela ainda roda. *Julgamento:* medir também quem importa o pacote mudado pegaria mais, mas multiplicaria o tempo (~1 min por pacote);
  - toda segunda e à mão: a lista inteira;
  - todos os operadores ligados;
  - 4 workers, com folga de 30× no tempo de cada teste (no padrão de 5×, a compilação em paralelo estourava o tempo).
- **Piso por pacote, que só sobe:** eficácia = mortas / (mortas + vivas). Assunto novo com regra entra na lista no PR dele.
- **Nenhum pacote fica de fora sem ninguém perceber (#456):** todo pacote de `internal/` com código fora de teste está em `scripts/ci/mutacao-go.txt` ou em `scripts/ci/mutacao-go-fora.txt`, com o motivo (código gerado, só montagem de rotas, apoio a teste). O script confere antes de medir, e a CI falha com o pacote que não está em nenhum dos dois, dizendo como medir e onde pôr. Na primeira conferência, 7 pacotes com regra entraram na lista (todos com 100%, depois de 3 mutações vivas mortas por teste) e 8 ficaram de fora, cada um com o porquê.
- **Equivalentes conhecidas** (a mutação que não muda o resultado, como `100 * time.Nanosecond` por `100 / time.Nanosecond`): ficam em `scripts/ci/mutacao-go-equivalentes.txt`, pelo texto da linha, com o porquê, e saem da conta. Só entra o que foi conferido à mão.
- **Ponto cego:** a cobertura do Go conta comandos, e a condição de um `case` não é comando. Por isso, mutação dentro de `case` sai como "NOT COVERED" e o gremlins nunca a testa. No nosso estilo (`switch { case …: }`), isso cobre boa parte das regras. Nelas, a mutação à mão no PR é o que vale. *Conferido na primeira medição: o `validar` dos parâmetros tem teste, e as mutações dos `case` dele saíram como "NOT COVERED".*

### Teste de arquitetura local (#476)

Quem pode importar quem fica num teste no estilo do ArchUnit do Java, em `internal/arquitetura/arquitetura_test.go`.
- **Só local** (decisão de 29/10/2026): a build tag `arquitetura` o tira do `go test ./...` de todo dia e da CI. Roda com `go test -tags arquitetura ./internal/arquitetura/` antes de subir um PR que mexe em import ou pacote novo.
- **Só a biblioteca padrão:** o grafo vem do `go list -json ./...`. As ferramentas prontas (`arch-go`, `go-arch-lint`, `depguard`) seriam dependência nova, e as regras cabem em poucas centenas de linhas.
- **As regras, cada uma com o motivo e as exceções explicadas:**
  1. só o `internal/banco` (e o contrato) importam o driver do SQLite;
  2. o `internal/testeapoio` só entra em `_test.go`;
  3. pacote de regra (a lista `regraSemHTTP`) não chega a `net/http`, `internal/respostas` nem `internal/servidor`, nem por outro pacote;
  4. todo pacote está num grupo (base, transversal, montador ou assunto), e um assunto só importa outro se o par estiver na tabela `dependencias`. Essa tabela é a semente do mapa de contextos (#474).
- **A lista não envelhece:** exceção que deixou de acontecer, pacote que sumiu e pacote novo sem grupo também fazem o teste falhar, com o que fazer.
- O teste se testa: um grafo de mentira com um import proibido faz cada regra falhar.

### Proibido em teste

| Proibido | Use | Exceção aceita |
|---|---|---|
| `time.Now()`, no código testado ou para montar dado | `relogio.Fixo` (o código de regra recebe um `relogio.Relogio`) | medir o próprio tempo real, quando ele é o assunto (o teste de WAL entre dois processos, `internal/banco/wal_test.go`) |
| `time.Sleep` para **esperar** algo acontecer | um sinal: canal, `sync.WaitGroup`, a linha `{"porta":N}` lida da saída (como `cmd/financas/main_test.go`); código com temporizador (tarefas em segundo plano), [`testing/synctest`](https://pkg.go.dev/testing/synctest), que roda o teste com um relógio falso e adianta o tempo quando todos estão parados (Go 1.25+; [blog do Go](https://go.dev/blog/testing-time)) | um teto, para o teste não travar: `time.After` num `select`, ou um contexto com prazo |
| rede de verdade (internet, outro serviço) | o `testeapoio.ServicoDeFora` com as respostas de `testdata/` | o próprio teste ouvindo em `127.0.0.1` (o servidor do Go, o `httptest`) |
| dado real (o repositório é compartilhado) | dado inventado; resposta gravada de API externa com os valores trocados à mão antes de entrar no `testdata` (nunca token, conta, documento ou saldo de verdade) | os testes manuais que já existem, que leem uma **cópia** do banco (`FINANCAS_COPIA_BANCO`) ou comparam com o .NET rodando (`FINANCAS_DOTNET_URL`): só leem, não mostram valores, pulam sozinhos sem a variável e nada deles vai para o repositório |
| biblioteca de asserção ou de mock | `if obtido != esperado { t.Errorf(...) }` e um *fake* escrito à mão, pequeno, onde há interface | — |

- **Dívidas que já existem** (conferidas; ficam até alguém mexer no arquivo, ou numa história do épico #374. *Inferido.*):
  - `internal/servidor/servidor_test.go:273-278`: tenta o servidor a cada 20 ms, até 50 vezes, esperando ele subir;
  - `internal/banco/esquema_banco_test.go:234`: dorme 20 ms para a data do arquivo mudar;
  - ~~`internal/banco/banco_test.go:89`: `time.Now()` para preencher uma data que o teste não confere~~: resolvida na #369, com o `instanteDosTestes`.

### Como escrever

- **Mensagem:** a função, a entrada, o obtido e o esperado, nessa ordem, em português ("`Hoje = 2026-10-16, esperado 2026-10-15`"; [Got before want](https://go.dev/wiki/TestComments#got-before-want), [Useful test failures](https://go.dev/wiki/CodeReviewComments#useful-test-failures)).
- **Nome do caso:** uma frase que diz a regra ("meia-noite em Brasília", "virada do ano"; [nomes legíveis](https://go.dev/wiki/TestComments#choose-human-readable-subtest-names)).
- **`t.Error` para continuar**, e `t.Fatal` só quando o resto não faz sentido, como uma preparação que falhou ([Keep going](https://go.dev/wiki/TestComments#keep-going)).
- **Ajudante de teste chama `t.Helper()`**, para a falha apontar a linha de quem chamou ([`T.Helper`](https://pkg.go.dev/testing#T.Helper)).
- **Contexto do teste:** `t.Context()` (Go 1.24+), que é cancelado antes dos `t.Cleanup` ([`T.Context`](https://pkg.go.dev/testing#T.Context)), no lugar de `context.Background()`.
- **Teste que precisa de algo de fora** (o .NET de pé, uma cópia do banco) pula com `t.Skip` e uma mensagem que diz o que definir, e **nunca é o único teste de uma regra.**

### `t.Parallel()`: quando usar

- **Use** nos testes de integração e de HTTP que têm o próprio banco no próprio `t.TempDir()`: é onde está o tempo. Chame no começo do teste e de cada subteste da tabela.
  - O teste paralelo espera os não paralelos do pacote terminarem e roda só com os outros paralelos ([`T.Parallel`](https://pkg.go.dev/testing#T.Parallel)).
  - Desde o Go 1.22, cada volta do `for` tem a própria variável, e o subteste paralelo não pega o caso errado da tabela ([blog do Go](https://go.dev/blog/loopvar-preview)). O `go.mod` está no 1.27.
- **Não use** quando o teste:
  - chama `t.Setenv` ou `t.Chdir`: eles afetam o processo inteiro, e o Go recusa os dois em teste paralelo ([`T.Setenv`](https://pkg.go.dev/testing#T.Setenv), [`T.Chdir`](https://pkg.go.dev/testing#T.Chdir)). Ex.: `internal/config/config_test.go:83`;
  - muda uma variável do pacote;
  - divide um arquivo de banco com outro teste;
  - mede tempo de verdade ou sobe outro processo (o teste de WAL);
  - é de contrato: os casos rodam em ordem, no mesmo banco, e um caso usa o que o anterior capturou.
- **Unitário puro em tabela:** pode, mas não precisa. Ele leva microssegundos, e o ganho é pequeno. *Julgamento.*
- O `go test ./...` já roda pacotes diferentes ao mesmo tempo, em processos separados (`-p`, que por padrão é o número de CPUs: [cmd/go](https://pkg.go.dev/cmd/go#hdr-Compile_packages_and_dependencies)). Por isso **nenhum teste usa um caminho de banco fixo**: sempre o `t.TempDir()`.
- O `-race` só acha a corrida que acontece de fato durante o teste ([Race Detector](https://go.dev/doc/articles/race_detector)). Por isso, teste paralelo sobre código com goroutine ajuda a corrida a aparecer. *Julgamento.*

### O que fica para as outras histórias do épico #374

Os princípios estão aqui. Os números e as ferramentas, nelas (*inferido*, até cada uma decidir):

- **#369:** o pacote de apoio (banco temporário, fábrica de dados, relógio, HTTP com usuário);
- **#370:** a cobertura na CI, com piso por pacote que só sobe. **Feito:**
  - **como mede:** `go test -race -coverpkg=./internal/...`, em que cada pacote conta as linhas que qualquer teste roda;
  - **os pisos:** ficam em `src/backend-go/cobertura-minima.txt` e começaram no valor de 24/10/2026, medido na CI;
  - **pacote novo:** precisa de **80%**. *Julgamento*, sugerido na #370: é o que já tem a maioria dos pacotes escritos com teste desde o começo;
  - **descer um piso falha**, comparando com o arquivo do `develop`. Subir a cobertura 1 ponto gera um aviso para subir o piso;
  - **"fora":** só para código gerado (sqlc, gerador do `go generate`), com o motivo na linha;
  - **o que dá trabalho:** a cobertura cai sozinha quando entra código sem teste num pacote antigo, mas não sobe sozinha. Subir o piso é trabalho de quem subiu a cobertura, no mesmo PR;
- **#371:** a trava de tempo dos testes do Go. **Feito:**
  - **onde:** `scripts/ci/verificar-tempo-testes-go.py`, sobre o `go test -json` do mesmo comando;
  - **limites:** teste com aviso em 5 s e falha em 20 s, pacote em 60 s e suíte em 120 s;
  - **por que esses números:** cerca de 3 vezes o que se tem hoje (*julgamento*), com referência de ~22 s para a suíte e 20 s para o pacote mais lento, na CI e com `-race`;
- **#372:** fuzz nos tipos de dinheiro e data ([Go Fuzzing](https://go.dev/doc/security/fuzz/)). **Feito:**
  - **onde:** um alvo por tipo, em `internal/tipos/fuzz_test.go` e `internal/relogio/fuzz_test.go`;
  - **o que confere:** só propriedades que valem para qualquer entrada: ida e volta, escala, formato do EF Core e o dia de Brasília. As sementes vêm do gabarito do EF Core;
  - **regressão:** a entrada que achou bug fica em `testdata/fuzz/<alvo>/` e roda no `go test` comum;
  - **onde roda o fuzz de verdade:** o `-fuzz` não roda no PR. Roda à mão e uma vez por semana, num workflow à parte (PR `ci(go):`). *Julgamento:* o achado do fuzz quase nunca é culpa do PR aberto, e travá-lo por isso atrasaria um trabalho sem relação. A semana é *inferida*;
  - **primeira rodada:** achou 2 bugs, um no começo do dia no horário de verão e outro no GUID com hífen fora do lugar. O mesmo começo do dia lança exceção no C# (#393);
- **#373:** o teste de mutação automático nas regras de negócio (feito: seção acima).

Benchmark só para código quente, antes de otimizar (skill `migrar-para-go`, seção 5).

## Consequências

- **Positivas:**
  - nenhuma etapa do #70 entra só com o contrato; cada regra tem um teste que diz **qual** regra quebrou;
  - os casos-limite ficam baratos, e os testes rodam sem o .NET, em Windows e Linux;
  - depois da #86, o Go continua protegido sem o .NET como referência;
  - os três devs seguem o mesmo padrão, e o revisor tem uma lista para conferir (tabela regra → teste, mutações);
  - sem dependência nova, e quem lê o teste lê Go puro.
- **Negativas:**
  - **mais tempo por etapa:** a tabela de casos, os testes de integração e as mutações à mão vêm antes de o PR estar pronto;
  - **a regra depende de revisão:** a CI não sabe se "cada regra tem teste". A cobertura da #370 é só um indicador, e a mutação da #373 também;
  - **a mutação automática não vê `case` nem remove comandos** (limites do gremlins). No PR, ela só mede os pacotes mudados (#456): o teste mais fraco num pacote que o PR não tocou continua aparecendo só na semana seguinte;
  - **teste mais verboso** sem asserção pronta: cada `if` escreve a própria mensagem. A tabela dilui esse custo;
  - **sobreposição:** um mesmo comportamento pode acabar no unitário e no contrato. A regra "num lugar só" pede julgamento a cada PR;
  - **o gabarito precisa de disciplina:** regravar sem olhar a diferença esconde uma regressão;
  - **`t.Parallel()` mal usado** dá teste instável (variável de pacote, banco dividido). O `-race` só pega parte disso;
  - as três dívidas listadas continuam até alguém mexer nos arquivos.
- **Na prática:**
  - antes de subir: `gofmt -l .`, `go vet ./...` e `go test -race ./...` em `src/backend-go` (o job "Go" da CI roda os mesmos);
  - **regra nova:** linha na tabela do teste unitário do pacote; **consulta nova:** teste com o SQLite real; **endpoint novo:** caso de contrato gravado do .NET, mais o teste HTTP dos ramos de validação;
  - **no PR:** a tabela regra → teste, os casos-limite que não se aplicam e a lista de mutações;
  - o C# continua sem teste automatizado, só com o checklist do PR: ele sai aos poucos, e cada assunto ganha teste ao vir para o Go;
  - a regra está também no `CLAUDE.md`, no `CONTRIBUINDO.md` ("Testes do back-end (Go)"), na skill `migrar-para-go`, no agente `dev-go` e no modelo de PR.
