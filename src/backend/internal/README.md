# O padrão de um assunto em Go (#77)

A decisão por trás deste padrão está na [ADR 0056](../../../docs/adr/0056-arquitetura-do-codigo-go-camadas-por-assunto-e-ddd-estrategico.md): camadas dentro de cada assunto, regra sem HTTP, mapa de contextos e o que **não** se adota (um pacote por camada, Clean Architecture completa, N-tier). O teste de arquitetura local confere (`go test -tags arquitetura ./internal/arquitetura/`, #476).

Cada assunto do épico #70 é um pacote em `internal/<area>/<assunto>`, com a mesma forma. O modelo é o `internal/ajustes/categorias`, com as regras em `internal/ajustes/categorizacao`, `regrasdescricao` e `internal/movimentacao/transferencias`.

## As áreas

As áreas são as mesmas das telas do front-end (`src/frontend/src/app/paginas/`), para a mesma palavra levar à mesma pasta nos dois lados (#656). A área só organiza: o pacote continua com o nome do assunto (`transacoes.NovoServico`), e quem importa quem fica na tabela `dependencias` do teste de arquitetura.

| Pasta | O que tem | Pacotes |
|---|---|---|
| `movimentacao/` | o dinheiro que entra e sai: transações, contas, cartões, contas a pagar | `transacoes`, `extrato`, `transferencias`, `parcelas`, `faturas`, `contas`, `contasagendadas`, `cobrancas`, `conferenciabanco`, `sugestoes`, `bancos`, `tipolancamento`, `poucoinformativa` |
| `planejamento/` | orçamentos e metas | `orcamentos`, `metas` |
| `patrimonio/` | investimentos, reservas, empréstimos e as cotações | `carteira`, `investimentos`, `reservas`, `cambio`, `cotacoes`, `cripto`, `selic`, `emprestimos` |
| `analises/` | relatórios e painel | `relatorios`, `painel` |
| `ajustes/` | os cadastros e a parametrização | `categorias`, `categorizacao`, `regrasdescricao`, `parametros` |
| `openfinance/` | a Pluggy e a sincronização (no front-end, a tela fica em `ajustes`) | `openfinance`, `pluggy`, `sincronizacao` |
| `plataforma/` | o que todos usam: banco, tipos, HTTP comum, login, erros do sistema, auditoria, segredos, tarefas | `banco`, `tipos`, `texto`, `relogio`, `config`, `respostas`, `servidor`, `servicosistema`, `sessao`, `autenticacao`, `auditoria`, `configuracao`, `erros`, `segredos`, `tarefas`, `versao`, `saude`, `desfazer`, `testeapoio` |
| raiz de `internal/` | quem junta ou confere todos os outros | `api`, `partida`, `arquitetura` |

**Assunto novo:** vai para a área da tela dele. Se ninguém o vê como tela e todos os assuntos o usam, é `plataforma`. Em dúvida, a ADR 0056 decide pelo mapa de contextos, não pelo nome.

## As três camadas

| Camada | Onde | O que faz | No .NET |
|---|---|---|---|
| HTTP | `internal/<area>/<assunto>`, uma função por rota | lê a entrada como o model binding do ASP.NET, chama a regra e responde o mesmo JSON | o controller |
| Regra | o mesmo pacote, ou um pacote próprio quando outro assunto também usa (`categorizacao`, que a sincronização #83 reaproveita) | a regra de negócio, sem nada de HTTP | o `XxxService` |
| Banco | SQL com `?` (ou o código do `sqlc`) dentro da regra | ler e gravar no formato do EF Core | o `FinancasDbContext` |

- **`Registrar(mux, servico)`** põe as rotas do assunto no `ServeMux`; o `internal/api` tem uma linha por assunto.
- **`Servico`** recebe as dependências no construtor (`NovoServico(db, relogio, log)`): sem container de injeção.
- **Interface só onde há duas implementações de verdade** (o cliente da PTAX de verdade e o falso dos testes), declarada por quem usa ([Effective Go](https://go.dev/doc/effective_go#interfaces)). Não há um `IXxxService` ao lado de cada `XxxService`.

## Assunto novo (ADR 0056)

- **Uma linha em `dependencias`** (`internal/arquitetura/arquitetura_test.go`): com quais assuntos ele conversa e por quê. É o mapa de contextos.
- **Regra que outro assunto usa** vai para um pacote próprio, na lista `regraSemHTTP`: ele não importa `net/http`, `respostas` nem `servidor`.
- **Uma linha no diagrama de Componentes** ([`docs/arquitetura/componentes.md`](../../../docs/arquitetura/componentes.md), #464): o pacote entra na tabela, na linha do componente dele. O mesmo teste de arquitetura confere.
- **Termo novo do domínio** entra nos [termos para quem desenvolve](../../../docs/arquitetura/dominio.md#termos-para-quem-desenvolve), com o mesmo nome do código.
- **Interface** só quando há duas implementações de verdade, declarada por quem usa. Objetos de valor e agregados, só onde pagam (ADR 0056).
- **Testes que a CI cobra do pacote novo:** 80% de cobertura já no primeiro PR (`scripts/ci/go/verificar-cobertura-go.py`) e uma linha em `scripts/ci/go/mutacao-go.txt` (com regra de negócio ou dinheiro) **ou** em `scripts/ci/go/mutacao-go-fora.txt`, com o motivo. Pacote que não está em nenhum dos dois faz a CI falhar (#456, ADR 0051).

## Entrada e saída iguais às do .NET

- **Corpo JSON:** `respostas.LerObjeto(w, r, "dto", "<tipo do C#>", campos...)`. Cada campo é o atributo do record:
  - `CampoTexto`: `[Required]`, `[StringLength]` e `[RegularExpression]`;
  - `CampoInteiro` e `CampoInteiroOpcional`: aceitam número em texto, com `[Range]`;
  - `CampoEnum`: só número, de 0 a 255;
  - `CampoDecimalOpcional`: com `[Range]` em cultura pt-BR;
  - `CampoBool`: o padrão do construtor vai no `Valor`.

  Ele responde sozinho o 400 com as mensagens do `System.Text.Json` e dos atributos.
- **Query:** o nome não diferencia maiúsculas e vale o primeiro valor. O que não vira o tipo do parâmetro é o 400 `The value 'x' is not valid.`
- **Rota `{id:int}`:** o que não é int de 32 bits cai no 404 sem corpo do fallback.
- **Erros do controller:** `respostas.Mensagem(w, status, "...")`, o `{"mensagem"}` do `BaseController`, com o texto que o front-end mostra. Erro inesperado: o 500 genérico (`servidor.ResponderErroInterno`), com o erro no log.
- **DTO de saída:** uma struct com as propriedades na ordem do record do C#, em camelCase. Lista vazia sai `[]`, e não `null`.

## Gravar como o EF Core

- **Uma requisição, uma transação:** `banco.EmTransacao`.
- **Mudança numa entidade lida:** só as colunas que mudaram (`transacoes.Gravar`). O `AtualizadoEm` muda quando o C# muda; o `ExecuteUpdate` do C# não mexe nele.
- **Dinheiro:** comparado em Go, com decimal, nunca no SQL (no SQLite ele é texto).

## Testes (ADR 0051)

- **Regra:** tabela com `t.Run`, num SQLite de verdade (`testeapoio.NovoBancoVazio`).
- **HTTP:** com `httptest`, para o que o contrato não cobre.
- **Contrato:** `testes/contrato/casos/NN-assunto.json` (gravado do .NET até a #86; hoje, com `rodar.sh --atualizar` e o `git diff` conferido).
- **Comportamento do C# que é difícil de reproduzir** (normalização de texto, decimal): um gabarito gravado por uma sonda que chamava o próprio C# (`internal/plataforma/texto/testdata`, `internal/plataforma/tipos/testdata`). As sondas saíram na #86 (estão na tag `v-ultimo-dotnet`); os gabaritos ficam, como regressão.
