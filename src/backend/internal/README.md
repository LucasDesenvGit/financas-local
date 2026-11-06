# O padrão de um assunto em Go (#77)

A decisão por trás deste padrão está na [ADR 0056](../../../docs/adr/0056-arquitetura-do-codigo-go-camadas-por-assunto-e-ddd-estrategico.md): camadas dentro de cada assunto, regra sem HTTP, mapa de contextos e o que **não** se adota (um pacote por camada, Clean Architecture completa, N-tier). O teste de arquitetura local confere (`go test -tags arquitetura ./internal/arquitetura/`, #476).

Cada assunto do épico #70 é um pacote em `internal/<assunto>`, com a mesma forma. O modelo é o `internal/categorias`, com as regras em `internal/categorizacao`, `regrasdescricao` e `transferencias`.

## As três camadas

| Camada | Onde | O que faz |
|---|---|---|
| HTTP | `internal/<assunto>`, uma função por rota (`rotas*.go`) | lê e valida a entrada (pacote `respostas`), chama a regra e responde o JSON |
| Regra | o mesmo pacote, ou um pacote próprio quando outro assunto também usa (`categorizacao`, que a sincronização #83 reaproveita) | a regra de negócio, sem nada de HTTP |
| Banco | SQL com `?` (ou o código do `sqlc`) dentro da regra | ler e gravar no formato do banco: dinheiro, datas e booleanos como texto e inteiro (pacote `tipos`) |

- **`Registrar(mux, servico)`** põe as rotas do assunto no `ServeMux`; o `internal/api` tem uma linha por assunto.
- **`Servico`** recebe as dependências no construtor (`NovoServico(db, relogio, log)`): sem container de injeção.
- **Interface só onde há duas implementações de verdade** (o cliente da PTAX de verdade e o falso dos testes), declarada por quem usa ([Effective Go](https://go.dev/doc/effective_go#interfaces)). Não há uma interface ao lado de cada `Servico`.

## Assunto novo (ADR 0056)

- **Uma linha em `dependencias`** (`internal/arquitetura/arquitetura_test.go`): com quais assuntos ele conversa e por quê. É o mapa de contextos.
- **Regra que outro assunto usa** vai para um pacote próprio, na lista `regraSemHTTP`: ele não importa `net/http`, `respostas` nem `servidor`.
- **Uma linha no diagrama de Componentes** ([`docs/arquitetura/componentes.md`](../../../docs/arquitetura/componentes.md), #464): o pacote entra na tabela, na linha do componente dele. O mesmo teste de arquitetura confere.
- **Termo novo do domínio** entra nos [termos para quem desenvolve](../../../docs/arquitetura/dominio.md#termos-para-quem-desenvolve), com o mesmo nome do código.
- **Interface** só quando há duas implementações de verdade, declarada por quem usa. Objetos de valor e agregados, só onde pagam (ADR 0056).
- **Testes que a CI cobra do pacote novo:** 80% de cobertura já no primeiro PR (`scripts/ci/verificar-cobertura-go.py`) e uma linha em `scripts/ci/mutacao-go.txt` (com regra de negócio ou dinheiro) **ou** em `scripts/ci/mutacao-go-fora.txt`, com o motivo. Pacote que não está em nenhum dos dois faz a CI falhar (#456, ADR 0051).

## Entrada e saída

- **Corpo JSON:** `respostas.LerObjeto(w, r, "dto", "<nome do tipo>", campos...)`, com um campo por propriedade:
  - `CampoTexto`: obrigatório, tamanho máximo e expressão regular;
  - `CampoInteiro` e `CampoInteiroOpcional`: aceitam número em texto, com faixa de valores;
  - `CampoEnum`: só número, de 0 a 255;
  - `CampoDecimalOpcional`: com faixa de valores, em formato pt-BR;
  - `CampoBool`: o valor quando a propriedade não vem vai no `Valor`.

  Ele responde sozinho o 400 com as mensagens de validação. O nome do tipo aparece na mensagem de erro de conversão, e os testes de contrato a conferem: copie o de um assunto vizinho.
- **Query:** o nome não diferencia maiúsculas e vale o primeiro valor. O que não converte para o tipo do parâmetro é o 400 `The value 'x' is not valid.`
- **Rota `{id:int}`:** o que não é int de 32 bits cai no 404 sem corpo.
- **Erro de regra:** `respostas.Mensagem(w, status, "...")`, o `{"mensagem"}` com o texto que o front-end mostra. Erro inesperado: o 500 genérico (`servidor.ResponderErroInterno`), com o erro no log.
- **DTO de saída:** uma struct com as propriedades em camelCase, na ordem que o contrato espera. Lista vazia sai `[]`, e não `null`.

## Gravar

- **Uma requisição, uma transação:** `banco.EmTransacao`.
- **Mudança numa entidade lida:** só as colunas que mudaram (`transacoes.Gravar`). O `AtualizadoEm` não muda sozinho: vai para o banco só quando a regra o muda, e o contrato confere em quais.
- **Dinheiro:** comparado em Go, com decimal, nunca no SQL (no SQLite ele é texto).

## Testes (ADR 0051)

- **Regra:** tabela com `t.Run`, num SQLite de verdade (`testeapoio.NovoBancoVazio`).
- **HTTP:** com `httptest`, para o que o contrato não cobre.
- **Contrato:** `testes/contrato/casos/NN-assunto.json` (com `rodar.sh --atualizar` e o `git diff` conferido).
- **Comportamento difícil de acertar** (normalização de texto, arredondamento do decimal): um gabarito em `testdata/` (`internal/texto/testdata`, `internal/tipos/testdata`). *Histórico:* esses gabaritos foram gravados chamando o C#, antes de o .NET sair na #86; ficam como regressão.
