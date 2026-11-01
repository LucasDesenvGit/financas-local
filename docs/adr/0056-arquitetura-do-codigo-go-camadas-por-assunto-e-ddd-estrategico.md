# 0056. Arquitetura do código Go: camadas dentro de cada assunto, regra sem HTTP e DDD estratégico

- **Status:** Aceita (#474, épico #70; o time de acordo em 01/11/2026, pelo @JulioCezarSP)
- **Data:** 2026-11-01
- **Relacionadas:** [0049](0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md) (o back-end em Go), [0051](0051-testes-do-back-end-em-go-cada-regra-com-teste.md) (testes, inclusive o de arquitetura local), [0020](0020-glossario-unico-com-balao-na-top-layer.md) (glossário de quem usa), [0055](0055-documentacao-de-arquitetura-em-diagramas-mermaid.md) (o mapa de contextos em diagrama)

## Contexto

O back-end em Go já tem 48 pacotes, escritos por três pessoas e por agentes, ao mesmo tempo. A forma do código existia só no `src/backend-go/internal/README.md` (o padrão da #77) e na prática:
- um pacote por assunto, com HTTP, regra e banco dentro;
- a regra vai para um pacote próprio quando outro assunto também a usa;
- interface só onde há duas implementações de verdade.

Nada conferia essas regras. A #476 criou o teste de arquitetura local (ADR 0051), com o que já valia. Faltava decidir:
- o que se adota de DDD e de Clean Architecture, que foram propostos para o projeto;
- o que **não** se adota, e por quê;
- qual é o mapa de contextos que o teste confere.

## Alternativas consideradas

- **Um pacote por camada** (`handlers/`, `services/`, `repositories/`), como no .NET de hoje:
  - **prós:** familiar para quem vem do C#;
  - **contras:** uma mudança num assunto mexe em três pacotes, e em Go isso costuma acabar em dependência circular (o compilador não aceita ciclo entre pacotes: [Go spec, Import declarations](https://go.dev/ref/spec#Import_declarations)). O resultado é um pacote `comum` que todo mundo importa.
- **Clean Architecture completa:** entidades, casos de uso e adaptadores, com uma interface de repositório para cada agregado:
  - **prós:** a regra fica testável com um banco falso;
  - **contras:** contraria a ADR 0051 (SQLite de verdade, sem mock, cerca de 0,02 s por banco) e o costume do Go de declarar a interface onde ela é usada, só quando há mais de uma implementação ([Effective Go, Interfaces](https://go.dev/doc/effective_go#interfaces)). Multiplica arquivos e cópias de dados entre camadas sem um problema concreto, num sistema local de uma pessoa.
- **N-tier físico** (camadas em processos ou máquinas separadas): contraria o "tudo local" e o SQLite (ADR 0002).
- **Ferramenta pronta de arquitetura** (`arch-go`, `go-arch-lint`, o `depguard` do `golangci-lint`): seria dependência nova, fora da lista das ADRs 0049 e 0051. As regras de "quem importa quem" cabem num teste em Go puro (#476). Reavaliar se as regras passarem disso.
- **Só o padrão da #77, sem escrever:** custo zero, mas a regra continua dependendo de quem revisa lembrar.
- **Camadas dentro de cada assunto, regra sem HTTP e DDD estratégico (escolhida).**

## Decisão

### Camadas dentro de cada assunto

Cada assunto é um pacote `internal/<assunto>` com **HTTP → regra → banco** dentro, o padrão da #77. Não há um pacote por camada.
- A regra vai para um pacote próprio só quando **outro assunto** também a usa (`categorizacao`, `transacoes`, `tipolancamento`).
- As dependências entram pelo construtor (`NovoServico(db, relogio, log)`), sem container de injeção.

### Regra de dependência

**Pacote de regra não conhece HTTP:** não chega a `net/http`, `internal/respostas` nem `internal/servidor`, nem por outro pacote. Quem fala HTTP são só as rotas do assunto e o `internal/api`.
- A lista dos pacotes de regra e as exceções, com o motivo, ficam no teste de arquitetura (`regraSemHTTP` e `regraComHTTPPorEnquanto` em `internal/arquitetura/arquitetura_test.go`).
- Função de texto ou de cálculo usada pela regra mora num pacote de regra ou de base, e não no `respostas`. Foi o que a #476 fez com o `TamanhoUtf16`, que foi para o `internal/texto`.

### DDD estratégico

- **Cada assunto é um contexto delimitado** (o conceito central do DDD: Evans, *Domain-Driven Design*, 2003).
- **O mapa de contextos é a tabela `dependencias` do teste de arquitetura:** quem importa quem, com o motivo. Ela é a fonte e o teste a confere. O diagrama do [mapa de contextos](../ARQUITETURA.md#mapa-de-contextos) agrupa os mesmos pares por assunto, para quem lê.
- Dependência nova entre assuntos é uma linha nova na tabela, com o motivo, **no mesmo PR**. O diagrama acompanha quando a seta entre grupos muda.
- **A linguagem ubíqua** é a lista de [termos para quem desenvolve](../arquitetura/dominio.md#termos-para-quem-desenvolve), ligada ao glossário de quem usa (ADR 0020). O código usa os mesmos nomes. Termo novo do domínio entra lá.

### Módulos (#190, #191)

O núcleo não importa módulo, e um módulo não importa outro. O teste de arquitetura já tem a regra, com a lista `modulos` vazia até o primeiro módulo (#191).

### DDD tático, só onde paga

- **Objetos de valor:** tipos que garantem a regra em vez de a revisão garantir. O que existe é o `internal/tipos`: `Decimal` (dinheiro e quantidade, sem `float64`), `Data`, `DataHora` e `Guid`, no formato do EF Core. Os próximos entram **aos poucos**, quando um assunto novo precisar:
  - `Dinheiro`, separado da quantidade, para a soma de dinheiro com quantidade não compilar;
  - `Mes`/`Periodo`, para os filtros de mês do Painel e dos relatórios (#80);
  - `Ticker`, para os ativos da carteira.
- **Agregados**, só onde há regra que precisa valer sempre:
  - **a carteira:** a posição e o preço médio são calculados das operações, e só o `internal/carteira` grava operação (`OperacaoAtivo`). O preço do ativo é do `internal/cotacoes`;
  - **a fatura do cartão:** a fatura do mês sai das parcelas e do ciclo, e só o `internal/faturas` a calcula (`faturas.DoMesDosCartoes`, que o `contas` e a `conferenciabanco` chamam).

  Os outros assuntos leem pelo que esses dois expõem. Não há agregado para cada uma das 32 tabelas.

### Interfaces

**Interface pequena, declarada por quem usa**, quando há duas implementações de verdade: a real e a falsa dos testes.
- É o que a `carteira`, a `conferenciabanco` e o `cambio` (`fonteDeCotacoes`) já fazem.
- **Fonte de cotação:**
  - hoje, só as ações e os FIIs têm duas fontes: o Yahoo e, se ele falhar, a brapi (`internal/cotacoes`). A troca já existe dentro do pacote;
  - o câmbio (PTAX) e a cripto (Mercado Bitcoin) têm uma fonte cada. Quando ganharem uma segunda, a fonte vira uma interface declarada pelo `cambio` ou pelo `cripto`, com a troca quando uma falha;
  - uma interface comum entre as três não entra: elas dão dados diferentes (moeda, cripto, ativo da B3).
- **A Pluggy não ganha interface** enquanto for a única fonte de transações.

### Vale daqui para a frente

Vale para assunto novo e para assunto que ainda vai migrar (#79, #80, #83, #84). Os já migrados mudam quando alguém mexer neles. O C# não muda, porque sai na #86.

## Consequências

- **Positivas:**
  - a forma do código está escrita, e o teste de arquitetura (#476) confere a regra de dependência e o mapa de contextos;
  - assunto novo segue uma receita: pacote, camadas, linha na tabela, termos na lista;
  - dependência entre assuntos aparece no PR, porque a tabela muda junto;
  - sem dependência nova e sem camadas a mais.
- **Negativas:**
  - **o teste é só local** (decisão de 29/10/2026): quem não roda antes de subir só descobre na revisão;
  - **a tabela e o diagrama são dois lugares:** a tabela vale e o teste confere; o diagrama pode ficar para trás e depende da revisão;
  - **objetos de valor aos poucos:** até o `Dinheiro` existir, misturar dinheiro com quantidade continua compilando. A #457 confere o `float64` na CI;
  - **a regra fica perto do banco:** sem repositório, trocar o SQLite exigiria mexer em cada assunto. *Julgamento:* não está nos planos (ADR 0002).
- **Na prática:**
  - **assunto novo:** `internal/<assunto>` com HTTP, regra e banco, uma linha em `dependencias` (com quem ele importa e por quê) e o termo novo na lista de termos;
  - **regra usada por outro assunto:** pacote próprio, na lista `regraSemHTTP`;
  - antes de subir, `go test -tags arquitetura ./internal/arquitetura/` (CLAUDE.md, skill `migrar-para-go`, agente `dev-go`).
