# Estilo do Go no projeto

O que se espera de Go idiomático aqui, com a fonte de cada ponto. As bibliotecas permitidas estão na tabela da [ADR 0049](../../../../docs/adr/0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md); dependência fora dela pede atualizar a ADR.

## Regras de ouro

Esta lista é a fonte (até a #729, era a "Convenções do código Go" da ADR 0049); o `CLAUDE.md` guarda só a do dinheiro. O "como" de cada uma está nas tabelas abaixo ou no lugar indicado.

- nomes em português;
- erro como valor, embrulhado com `%w`;
- `context.Context` como primeiro parâmetro de quem faz I/O;
- dependências pelo construtor, sem container;
- relógio injetável no fuso de Brasília, nunca o `time.Now()`;
- SQL escrito à mão, só com `?`, nunca com texto do usuário concatenado ([README do banco](../../../../src/backend/internal/plataforma/banco/README.md));
- dinheiro só em decimal, e nada de `SUM`, `ORDER BY` ou `<`/`>` no SQL em coluna de dinheiro (o porquê nas "Armadilhas" do README do banco);
- `gofmt`, `go vet` e `go test -race` limpos (o "Antes de subir" do [README do Go](../../../../src/backend/README.md); o `verificar-regras-go.py` da CI confere `float64` e `time.Now()`).

## O que seguimos

| Ponto | Como fica aqui | Fonte |
|---|---|---|
| Erro é valor | `if err != nil { return fmt.Errorf("salvar conta %d: %w", id, err) }`, caminho feliz sem `else` | [Go 1.13 errors](https://go.dev/blog/go1.13-errors), [CodeReviewComments: indent error flow](https://go.dev/wiki/CodeReviewComments#indent-error-flow) |
| Status HTTP a partir do erro | `errors.Is`/`errors.As` nas rotas do assunto, com erro exportado pela regra (`internal/patrimonio/reservas/rotas.go` com o `ErrContaInvalida` e o `ErrSemCotacao` de `reservas.go`; `internal/movimentacao/contas/rotas.go` com o `sql.ErrNoRows`); o corpo do erro pelo `internal/plataforma/respostas` | ADR 0056 (regra sem HTTP) |
| Texto do erro | minúsculo e sem ponto final, porque é encadeado | [CodeReviewComments: error strings](https://go.dev/wiki/CodeReviewComments#error-strings) |
| `panic` | só para bug de programação; o `recover` do `internal/plataforma/servidor` responde 500 com a mensagem genérica, e o de `internal/plataforma/tarefas` segura cada ciclo | [CodeReviewComments: don't panic](https://go.dev/wiki/CodeReviewComments#dont-panic) |
| `context.Context` | primeiro parâmetro de tudo que faz I/O, e respeitado (`QueryContext`, `NewRequestWithContext`); em teste, `t.Context()` | [CodeReviewComments: contexts](https://go.dev/wiki/CodeReviewComments#contexts), [Go blog: context](https://go.dev/blog/context) |
| Dependências | pelo construtor (`NovoServico(db, relogio, log)`), sem container de injeção | ADR 0056 |
| Interface | pequena, declarada por quem usa, só com duas implementações de verdade (a real e a falsa do teste) | [CodeReviewComments: interfaces](https://go.dev/wiki/CodeReviewComments#interfaces), ADR 0056 |
| Goroutine | só com motivo, com dono que a encerra pelo `context`, e `go test -race` limpo; tarefa em segundo plano pelo `internal/plataforma/tarefas` | [CodeReviewComments: goroutine lifetimes](https://go.dev/wiki/CodeReviewComments#goroutine-lifetimes), [race detector](https://go.dev/doc/articles/race_detector) |
| Hora | `relogio.Relogio` injetado; `relogio.Hoje(r)` e `relogio.AgoraLocal(r)` para data de negócio em Brasília; carimbo (`CriadoEm`) em UTC | Regras de ouro, `internal/plataforma/relogio/relogio.go` |
| Tipos do banco | `tipos.Decimal`, `Data`, `DataHora`, `Guid`: `sql.Scanner`, `driver.Valuer` e JSON, com teste de ida e volta e fuzz. Objeto de valor novo (`Dinheiro`, `Mes`, `Ticker`) só quando um assunto precisar | ADR 0056 ("DDD tático, só onde paga") |
| Ordem que é contrato | `slices.SortStableFunc` com o desempate explícito (`sort.Slice` não é estável) | [pacote slices](https://pkg.go.dev/slices) |
| Log | `log/slog` em JSON, com o atributo `"origem"` pelas constantes de `internal/plataforma/erros` (`erros.OrigemAPI`…), para cair no lugar certo de Configurações › Erros | README do Go, "Como roda no site" |
| Nome de pacote | curto, minúsculo, sem `_` nem plural genérico (`contasagendadas`, `regrasdescricao`), nada de `util`/`comum` | [CodeReviewComments: package names](https://go.dev/wiki/CodeReviewComments#package-names) |
| Receptor | uma ou duas letras, o mesmo em todos os métodos do tipo (`func (s *Servico)`) | [CodeReviewComments: receiver names](https://go.dev/wiki/CodeReviewComments#receiver-names) |
| Comentário de documentação | frase completa, em português, começando pelo nome (`// Hoje devolve...`) | [Go Doc Comments](https://go.dev/doc/comment) |
| Teste | tabela com `t.Run`, mensagem `Função(entrada) = obtido, esperado X` | [Go Test Comments](https://go.dev/wiki/TestComments#identify-the-function), ADR 0051 |

## Contas, texto e JSON: o que o C# deixou

O banco e o JSON continuam no formato do .NET (ADR 0049), e estas pegadinhas vieram da migração (a skill dela está na tag `v-ultimo-dotnet`, seção 4). O código de `internal/plataforma/tipos` e `texto` é a fonte; o comentário de cada função diz o detalhe.

| Ponto | Como fica aqui | Por quê |
|---|---|---|
| Arredondar | `tipos.Arredondar(x, casas)` (bancário, e só mexe quando o valor tem mais casas: `250.0` continua `250.0`); `tipos.ArredondarLongeDoZero` só onde a regra pede o empate longe do zero. Nunca o `Round` do `shopspring/decimal` direto | o `Math.Round` do C# é bancário; o `Round` da biblioteca arredonda o .5 para cima e muda a escala do JSON |
| Dividir e as outras contas | `tipos.Dividir` (devolve `ErrDivisaoPorZero` e `ErrEstouro`), `Multiplicar`, `Somar`, `Subtrair`, `Maior`, `Menor` | guardam a escala como o C# (gabarito em `internal/plataforma/tipos/testdata`); a biblioteca divide com 16 casas |
| Ordem e busca em texto | a ordem pt-BR e as normalizações do `internal/plataforma/texto`; teste com "Ação" e "acao" | Go compara bytes, e o `LIKE` do SQLite só ignora caixa em ASCII |
| Data | `time.Time` sempre com local explícito; `tipos.Data` só com ano, mês e dia | fuso trocado muda o mês do lançamento |
| Expressão regular | o `regexp` do Go é RE2: sem lookbehind nem lookahead (`(?<!...)`), e o `\b` e o `\d` só conhecem ASCII; confira as pontas no código, como em `internal/patrimonio/carteira/extrato.go` | o C# aceitava os dois e conhece Unicode; o mesmo padrão copiado casa outra coisa ou nem compila |
| `enum` | tipo `int` nomeado, que sai **número** no JSON; não trocar por texto | é o contrato com o front-end |
| Validação de entrada | na entrada, com o 400 do `internal/plataforma/respostas` (`type`, `title`, `status` e `errors` por campo, mensagens em português, #710; `LerObjeto`, `LerTextos`, `Validacao`) | a mesma mensagem e o mesmo status em todas as rotas |
| Rota e resposta | `{id}` que não é int de 32 bits é o 404 sem corpo; o DTO de saída tem as propriedades em camelCase, na ordem do contrato; o erro da regra sai por `respostas.Mensagem` (o `{"mensagem"}` que o front-end mostra), e o inesperado pelo `servidor.ResponderErroInterno` (500 genérico, com o erro no log) | era o que o ASP.NET fazia, e os casos de contrato conferem |
| Apagar com dependentes | o que o `ON DELETE` de `banco/01-estrutura.sql` não apaga, o Go apaga explícito, em `banco.EmTransacao` | parte da cascata era feita pelo EF Core, não pela chave, e há tabela que nenhum dos dois apagava (o `HistoricoInvestimento`, #575) |

## Onde divergimos dos guias de fora, e por quê

Os guias conhecidos (Effective Go, CodeReviewComments, Google Go Style Guide, Uber Go Style Guide) e o hábito de quem vem de .NET ou de Clean Architecture sugerem coisas que o projeto decidiu **não** fazer. Ao ver uma sugestão destas num guia ou num revisor, a decisão do projeto vale.

| O guia ou o hábito diz | Aqui | Por quê |
|---|---|---|
| Identificadores em inglês | português (`ContaServico`, `SalvarContaEntrada`) | `CLAUDE.md`: o projeto inteiro é em português |
| Iniciais em maiúsculas: `ID`, `URL` ([initialisms](https://go.dev/wiki/CodeReviewComments#initialisms)) | `Id` nos campos que vêm do banco (`CategoriaId`, `ContaId`: centenas de usos) | *Inferido:* o mesmo nome da coluna e do C#. Não renomeie sem combinar; código novo segue o pacote em que está |
| Preferir a fatia `nil` ([declaring empty slices](https://go.dev/wiki/CodeReviewComments#declaring-empty-slices)) | `[]T{}` em toda saída JSON | `nil` sai `null`, e o contrato (e o front-end) espera `[]`. O próprio guia cita a exceção do JSON |
| Biblioteca de asserção ou mock (`testify`, `gomock`, `go-sqlmock`) | só a biblioteca padrão; `testeapoio.ConferirJSON` | ADR 0051 |
| Framework HTTP (gin, chi, echo) | `net/http` com os padrões de rota do Go 1.22+ | ADR 0049; [routing enhancements](https://go.dev/blog/routing-enhancements) |
| ORM (gorm, ent) | `database/sql` com SQL à mão e `?` | ADR 0049 e 0074; [SQL injection](https://go.dev/doc/database/sql-injection) |
| Biblioteca de log de terceiros | `log/slog` | ADR 0049 |
| `float64` para número | `tipos.Decimal` em todo dinheiro | `CLAUDE.md` |
| Pacote por camada (`handlers/`, `services/`, `repositories/`) | camadas **dentro** do assunto (`internal/<area>/<assunto>`) | ADR 0056 |
| Interface de repositório para poder trocar o banco (Clean, Hexagonal) | sem repositório: a regra fica perto do SQL | ADR 0056, consequência aceita: trocar o SQLite não está nos planos (ADR 0002) |
| Interface "para o caso de precisar" | só com duas implementações de verdade | ADR 0056 |
