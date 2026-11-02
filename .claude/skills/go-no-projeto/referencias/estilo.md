# Estilo do Go no projeto

O que se espera de Go idiomático aqui, com a fonte de cada ponto. O que é regra de ouro do projeto (dinheiro, SQL, relógio, bibliotecas) está na ADR 0049 ("Convenções do código Go" e a tabela de bibliotecas) e, o dinheiro, também no `CLAUDE.md`; não se repete aqui.

## O que seguimos

| Ponto | Como fica aqui | Fonte |
|---|---|---|
| Erro é valor | `if err != nil { return fmt.Errorf("salvar conta %d: %w", id, err) }`, caminho feliz sem `else` | [Go 1.13 errors](https://go.dev/blog/go1.13-errors), [CodeReviewComments: indent error flow](https://go.dev/wiki/CodeReviewComments#indent-error-flow) |
| Status HTTP a partir do erro | `errors.Is`/`errors.As` nas rotas do assunto, com erro exportado pela regra (`internal/reservas/rotas.go` com o `ErrContaInvalida` e o `ErrSemCotacao` de `reservas.go`; `internal/contas/rotas.go` com o `sql.ErrNoRows`); o corpo do erro pelo `internal/respostas` | ADR 0056 (regra sem HTTP) |
| Texto do erro | minúsculo e sem ponto final, porque é encadeado | [CodeReviewComments: error strings](https://go.dev/wiki/CodeReviewComments#error-strings) |
| `panic` | só para bug de programação; o `recover` do `internal/servidor` responde 500 com a mensagem genérica, e o de `internal/tarefas` segura cada ciclo | [CodeReviewComments: don't panic](https://go.dev/wiki/CodeReviewComments#dont-panic) |
| `context.Context` | primeiro parâmetro de tudo que faz I/O, e respeitado (`QueryContext`, `NewRequestWithContext`); em teste, `t.Context()` | [CodeReviewComments: contexts](https://go.dev/wiki/CodeReviewComments#contexts), [Go blog: context](https://go.dev/blog/context) |
| Dependências | pelo construtor (`NovoServico(db, relogio, log)`), sem container de injeção | ADR 0056 |
| Interface | pequena, declarada por quem usa, só com duas implementações de verdade (a real e a falsa do teste) | [CodeReviewComments: interfaces](https://go.dev/wiki/CodeReviewComments#interfaces), ADR 0056 |
| Goroutine | só com motivo, com dono que a encerra pelo `context`, e `go test -race` limpo; tarefa em segundo plano pelo `internal/tarefas` | [CodeReviewComments: goroutine lifetimes](https://go.dev/wiki/CodeReviewComments#goroutine-lifetimes), [race detector](https://go.dev/doc/articles/race_detector) |
| Hora | `relogio.Relogio` injetado; `relogio.Hoje(r)` e `relogio.AgoraLocal(r)` para data de negócio em Brasília; carimbo (`CriadoEm`) em UTC | ADR 0049, `internal/relogio/relogio.go` |
| Tipos do banco | `tipos.Decimal`, `Data`, `DataHora`, `Guid`: `sql.Scanner`, `driver.Valuer` e JSON, com teste de ida e volta e fuzz. Objeto de valor novo (`Dinheiro`, `Mes`, `Ticker`) só quando um assunto precisar | ADR 0056 ("DDD tático, só onde paga") |
| Ordem que é contrato | `slices.SortStableFunc` com o desempate explícito (`sort.Slice` não é estável) | [pacote slices](https://pkg.go.dev/slices) |
| Log | `log/slog` em JSON, com o atributo `"origem"` pelas constantes de `internal/erros` (`erros.OrigemAPI`…), para cair no lugar certo de Configurações › Erros | README do Go, "Como roda no site" |
| Nome de pacote | curto, minúsculo, sem `_` nem plural genérico (`contasagendadas`, `regrasdescricao`), nada de `util`/`comum` | [CodeReviewComments: package names](https://go.dev/wiki/CodeReviewComments#package-names) |
| Receptor | uma ou duas letras, o mesmo em todos os métodos do tipo (`func (s *Servico)`) | [CodeReviewComments: receiver names](https://go.dev/wiki/CodeReviewComments#receiver-names) |
| Comentário de documentação | frase completa, em português, começando pelo nome (`// Hoje devolve...`) | [Go Doc Comments](https://go.dev/doc/comment) |
| Teste | tabela com `t.Run`, mensagem `Função(entrada) = obtido, esperado X` | [Go Test Comments](https://go.dev/wiki/TestComments#identify-the-function), ADR 0051 |

## Onde divergimos dos guias de fora, e por quê

Os guias conhecidos (Effective Go, CodeReviewComments, Google Go Style Guide, Uber Go Style Guide) e o hábito de quem vem de .NET ou de Clean Architecture sugerem coisas que o projeto decidiu **não** fazer. Ao ver uma sugestão destas num guia ou num revisor, a decisão do projeto vale.

| O guia ou o hábito diz | Aqui | Por quê |
|---|---|---|
| Identificadores em inglês | português (`ContaServico`, `SalvarContaEntrada`) | `CLAUDE.md`: o projeto inteiro é em português |
| Iniciais em maiúsculas: `ID`, `URL` ([initialisms](https://go.dev/wiki/CodeReviewComments#initialisms)) | `Id` nos campos que vêm do banco (`CategoriaId`, `ContaId`: centenas de usos) | *Inferido:* o mesmo nome da coluna e do C#. Não renomeie sem combinar; código novo segue o pacote em que está |
| Preferir a fatia `nil` ([declaring empty slices](https://go.dev/wiki/CodeReviewComments#declaring-empty-slices)) | `[]T{}` em toda saída JSON | `nil` sai `null`, e o contrato (e o front-end) espera `[]`. O próprio guia cita a exceção do JSON |
| Biblioteca de asserção ou mock (`testify`, `gomock`, `go-sqlmock`) | só a biblioteca padrão; `testeapoio.ConferirJSON` | ADR 0051 |
| Framework HTTP (gin, chi, echo) | `net/http` com os padrões de rota do Go 1.22+ | ADR 0049; [routing enhancements](https://go.dev/blog/routing-enhancements) |
| ORM (gorm, ent) | `sqlc` + `database/sql`, ou SQL com `?` | ADR 0049; [SQL injection](https://go.dev/doc/database/sql-injection) |
| Biblioteca de log de terceiros | `log/slog` | ADR 0049 |
| `float64` para número | `tipos.Decimal` em todo dinheiro | `CLAUDE.md` |
| Pacote por camada (`handlers/`, `services/`, `repositories/`) | camadas **dentro** do assunto (`internal/<assunto>`) | ADR 0056 |
| Interface de repositório para poder trocar o banco (Clean, Hexagonal) | sem repositório: a regra fica perto do `sqlc` | ADR 0056, consequência aceita: trocar o SQLite não está nos planos (ADR 0002) |
| Interface "para o caso de precisar" | só com duas implementações de verdade | ADR 0056 |
