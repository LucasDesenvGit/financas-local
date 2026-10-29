# O padrão de um assunto em Go (#77)

Cada assunto do épico #70 é um pacote em `internal/<assunto>`, com a mesma forma. O modelo é o `internal/categorias`, com as regras em `internal/categorizacao`, `regrasdescricao` e `transferencias`.

## As três camadas

| Camada | Onde | O que faz | No .NET |
|---|---|---|---|
| HTTP | `internal/<assunto>`, uma função por rota | lê a entrada como o model binding do ASP.NET, chama a regra e responde o mesmo JSON | o controller |
| Regra | o mesmo pacote, ou um pacote próprio quando outro assunto também usa (`categorizacao`, que a sincronização #83 reaproveita) | a regra de negócio, sem nada de HTTP | o `XxxService` |
| Banco | SQL com `?` (ou o código do `sqlc`) dentro da regra | ler e gravar no formato do EF Core | o `FinancasDbContext` |

- **`Registrar(mux, servico)`** põe as rotas do assunto no `ServeMux`; o `internal/api` tem uma linha por assunto.
- **`Servico`** recebe as dependências no construtor (`NovoServico(db, relogio, log)`): sem container de injeção.
- **Interface só onde há duas implementações de verdade** (o cliente da PTAX de verdade e o falso dos testes), declarada por quem usa ([Effective Go](https://go.dev/doc/effective_go#interfaces)). Não há um `IXxxService` ao lado de cada `XxxService`.

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
- **Contrato:** `testes/contrato/casos/NN-assunto.json`, gravado do .NET.
- **Comportamento do C# que é difícil de reproduzir** (normalização de texto, decimal): um gabarito gravado por uma sonda que chama o próprio C# (`internal/texto/testdata`, `internal/tipos/testdata`).
