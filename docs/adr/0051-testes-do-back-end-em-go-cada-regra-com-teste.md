# 0051. Testes do back-end em Go: cada regra com teste unitário, além do contrato

- **Status:** Aceita
- **Data:** 2026-10-23 (issue #368, épico de testes #374)
- **Relacionadas:** [0049](0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md) (back-end em Go, bibliotecas e testes de contrato da #72); [0013](0013-testes-do-front-end-com-jest.md) e [0041](0041-testes-no-navegador-com-playwright.md) (testes do front-end, de onde vêm "um comportamento testado num lugar só" e "bug corrigido ganha teste"); [0047](0047-ci-paralela-so-testes-afetados-no-pr.md) (o job "Go" da CI); [0054](0054-analise-de-seguranca-do-codigo-com-o-codeql.md) (o CodeQL, que confere o SQL montado com texto); épico #70

**Como testar, no dia a dia:** [CONTRIBUINDO.md, "Testes do back-end (Go)"](../../CONTRIBUINDO.md#testes-do-back-end-go). Esta ADR guarda a decisão, as alternativas e o custo; os passos, os comandos, os limites e as listas ficam no guia.

## Contexto

**O C# não tinha teste nenhum.** O back-end .NET chegou a 128 rotas e cerca de 15,3 mil linhas sem teste automatizado ([0049](0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md), Contexto). Por isso a migração para Go começou pelos testes de contrato (#72), só para saber o que o sistema fazia.

**O contrato não basta sozinho:**
- ele olha de fora: status, JSON e as linhas que mudaram no banco, comparados com o que o .NET respondeu e gravou;
- cada situação é um caso inteiro (banco fictício e requisição), gravado com o .NET de pé. Caso-limite de cálculo (arredondamento, dia 31, virada do ano, `NULL`, valor negativo) custa pouco num teste unitário e muito num caso de contrato;
- depois da #86 o .NET sai, e regra nova ou mudada não tem mais de onde gravar um esperado. Daí em diante, só o teste do próprio Go protege a regra.

Em 23/10/2026 os pacotes com lógica já tinham teste (78,6% de cobertura em `internal/`), e o PR #360 tinha provado o porte do esquema com 12 mutações à mão. Mas **nenhuma regra obrigava** o teste, e três devs iam começar #75, #77 e #81 em paralelo: a regra precisava existir antes do código.

## Alternativas consideradas

- **Só o teste de contrato** (a regra que já existia). Não chega aos casos-limite sem um caso inteiro para cada um; quando falha, diz que o JSON mudou, e não qual regra quebrou; pedia o .NET de pé, no Linux; e some como referência depois da #86. **Descartada como regra única:** o contrato continua obrigatório, ao lado do unitário.
- **Mock do banco** (repositório atrás de interface com implementação falsa, ou o `go-sqlmock`). Boa parte do que pode errar está no banco (o formato do EF Core em `TEXT`, a cascata, o `LIKE` com acento, a ordenação, a chave estrangeira), e um mock devolve o que o teste mandou. O SQLite de verdade é rápido (~0,02 s um banco novo com o esquema inteiro), o guia do Go pede não criar interface "for mocking" ([Go Code Review Comments](https://go.dev/wiki/CodeReviewComments#interfaces)), e seria mais uma dependência fora da [0049](0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md).

  **O `:memory:` também fica de fora:** cada conexão a `:memory:` abre um banco diferente ([sqlite.org](https://www.sqlite.org/inmemorydb.html)), o `*sql.DB` é um pool ([database/sql](https://pkg.go.dev/database/sql#DB)), e o WAL e o `busy_timeout` só existem com arquivo.
- **`testify` (ou outra biblioteca de asserção).** Mensagens prontas e menos linhas. Contra: a [0049](0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md) fecha a lista de bibliotecas; a documentação do Go desaconselha ("Avoid the use of 'assert' libraries", [Go Test Comments](https://go.dev/wiki/TestComments#assert-libraries); [FAQ](https://go.dev/doc/faq#testing_framework)); o `require` encerra na primeira falha, e numa tabela queremos ver todos os casos que falharam ([Keep going](https://go.dev/wiki/TestComments#keep-going)); e quem lê o teste aprende uma linguagem só.

  **O `go-cmp`** (`cmp.Diff`) também fica de fora por ora: o JSON é comparado pelo comparador do contrato, e o resto com `==`, `slices.Equal`, `maps.Equal` ou `reflect.DeepEqual`. Se um assunto precisar de diff de estrutura grande, ele entra atualizando a 0049. *Inferido.*
- **Uma pasta separada para os testes** (como no front-end). O `go test` compila cada pacote junto com os `*_test.go` da mesma pasta ([cmd/go](https://pkg.go.dev/cmd/go#hdr-Test_packages)), e o teste ali enxerga o que não é exportado. A exceção é o contrato, que testa o servidor de fora.
- **Ferramenta de mutação** (25/10/2026, #373): o [gremlins](https://github.com/go-gremlins/gremlins) trabalha numa cópia do módulo e não entra no `go.mod`. O [go-mutesting da Avito](https://github.com/avito-tech/go-mutesting) troca o arquivo original (interrompido, deixa o código estragado); o [ooze](https://github.com/gtramontina/ooze) entraria no `go.mod`; o [go-mutesting original](https://github.com/zimmski/go-mutesting) não tem versão desde 2021.
- **Regras de ouro num analisador do `go vet`** (#457): o `go/analysis` é do `golang.org/x/tools`, fora da 0049, e com só o `go/ast` seria um programa à parte. Ficou um script em Python, como os outros da CI.

## Decisão

**Toda regra do back-end em Go nasce com teste unitário**, além do caso de contrato do endpoint. Um comportamento é testado num lugar só, na camada mais barata que o alcança, como no front-end (#204). Só a biblioteca padrão (`testing`, `net/http/httptest`, `testing/synctest`), sem asserção nem mock de terceiros.

- **As camadas:** unitário em tabela no pacote do assunto; integração com o SQLite **de verdade**, num arquivo do `t.TempDir()`, com o esquema real; HTTP com `httptest`, sem porta aberta; contrato em `testes/contrato`; dependência externa por um servidor HTTPS de teste com respostas gravadas; o que é só do Windows num `_windows_test.go`. O apoio comum fica no `internal/plataforma/testeapoio`, importado só por `_test.go`.
- **O gabarito vem da referência, nunca do código que ele testa** (o .NET, ou o EF Core pela sonda), e só é regravado por opção explícita, olhando a diferença.
- **Uma etapa do #70 só está pronta** com a lista "pronto quando" do guia: a tabela regra → teste no PR, os riscos de equivalência, o contrato (inclusive de erro), o SQLite real nas consultas, os casos-limite e o teste novo que provou que falha, estragando o código de propósito, com as mutações no PR.
- **Bug corrigido ganha teste de regressão**, que falha antes da correção. **Bug do C# achado no porte não é corrigido em silêncio:** o teste fixa o comportamento de hoje e aponta a issue aberta.
- **O que a CI confere** (as histórias do épico #374): cobertura com piso por pacote que só sobe (#370), trava de tempo (#371), fuzz nos tipos de dinheiro e data, fora do PR (#372), teste de mutação automático com piso por pacote (#373, #456, #673) e as regras de ouro, `float64`, `time.Now` e `time.Sleep` de teste (#457).
- **Teste de arquitetura** (#476), no estilo do ArchUnit, com só a biblioteca padrão: **só local** (decisão de 29/10/2026), fora da CI.
- **Proibido em teste:** `time.Now()`, `time.Sleep` para esperar, rede de verdade, dado real e biblioteca de asserção ou de mock, cada um com a exceção aceita escrita no guia.
- Benchmark só para código quente, antes de otimizar (skill `go-no-projeto`, "Desempenho").

## Consequências

- **Positivas:**
  - cada regra tem um teste que diz **qual** regra quebrou, e o Go continua protegido sem o .NET como referência;
  - os casos-limite ficam baratos, e os testes rodam em Windows e Linux;
  - o revisor tem uma lista para conferir (tabela regra → teste, mutações);
  - sem dependência nova: quem lê o teste lê Go puro.
- **Negativas:**
  - **mais tempo por etapa:** tabela de casos, testes de integração e mutações à mão;
  - **a regra depende de revisão:** a CI não sabe se "cada regra tem teste". A cobertura e a mutação automática são só indicadores;
  - **a mutação automática não vê `case` nem remove comandos** (limites do gremlins). No PR, ela só mede os pacotes mudados: o teste fraco num pacote que o PR não tocou aparece só na rodada da semana;
  - **as regras de ouro são conferidas por texto:** um `import t "time"` escapa, e `time.Since`/`time.Until` não são conferidos;
  - **teste mais verboso** sem asserção pronta, e **sobreposição** entre unitário e contrato, que pede julgamento a cada PR;
  - **o gabarito precisa de disciplina:** regravar sem olhar a diferença esconde uma regressão;
  - **`t.Parallel()` mal usado** dá teste instável (variável de pacote, banco dividido). O `-race` só pega parte disso;
  - **a cobertura cai sozinha, mas não sobe sozinha:** subir o piso é trabalho de quem subiu a cobertura, no mesmo PR;
  - as dívidas de espera por `time.Sleep` foram pagas na #458; sobram só as exceções escritas, com o motivo, em `scripts/ci/go/regras-go-excecoes.txt`.
- **Na prática:** o passo a passo está no [CONTRIBUINDO.md](../../CONTRIBUINDO.md#testes-do-back-end-go). A regra também está no `CLAUDE.md`, na skill `go-no-projeto`, no agente `dev-go` e no modelo de PR. Como cada assunto do C# foi portado está na tag `v-ultimo-dotnet`.

## Atualização de 2026-11-03: o contrato vira a regressão do Go (issue #86, parte 3)

Sem o .NET, não há mais referência para gravar os esperados. **Decisão:** os esperados de `testes/contrato/esperado/`, gravados do .NET até aqui, passam a ser a **regressão do Go**.

- Antes da troca, os 883 casos passaram contra o Go sozinho (03/11/2026), menos uma diferença conhecida, com o motivo no caso.
- **Um esperado que já existe muda só de propósito**, com o motivo no PR (bug corrigido ou mudança pedida). Diferença que aparece sem querer é regressão: o código volta, e o esperado fica. Endpoint novo ganha caso gravado do Go, mais o teste HTTP dos ramos de validação.
- Os outros gabaritos que vieram do C# (as sondas do EF Core e do esquema) continuam valendo: o formato no banco que eles fixam não muda.
- Saíram da CI o teste de ponta a ponta do .NET com o Go atrás dele e a comparação da CSP com o `Seguranca.cs`; o `TestCabecalhosDeSeguranca` fixa os valores.
- **Consequência negativa:** o contrato deixa de provar "igual ao .NET" e passa a provar "igual a antes". Um defeito que já estava no Go e no esperado continua lá até alguém o ver. *Julgamento:* aceitável, porque o ensaio da #85 e os 883 casos mostraram o Go igual ao .NET no que o contrato cobre.

Como rodar o contrato e gravar um caso novo: [CONTRIBUINDO.md](../../CONTRIBUINDO.md#testes-do-back-end-go).
