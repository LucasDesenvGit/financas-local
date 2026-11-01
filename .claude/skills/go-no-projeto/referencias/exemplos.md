# Exemplos no código

Antes de criar algo, abra o exemplo e siga a forma dele. Estes são **ponteiros**, não cópias: o código é a fonte e não envelhece aqui. Se um exemplo deixar de seguir a ADR 0056 ou a 0051, troque o ponteiro no mesmo PR.

| Preciso de… | Olhe | O que copiar |
|---|---|---|
| A rota mais simples, ponta a ponta | `internal/saude` | um arquivo, `Registrar(mux, ...)`, front-end → .NET → Go → banco |
| Um assunto com o HTTP num arquivo e a regra noutro | `internal/reservas` (`rotas.go` com `net/http`; `reservas.go` e `texto.go` sem) | a rota só traduz HTTP; a regra exporta o erro tipado (`ErrContaInvalida`, `ErrSemCotacao`) e a rota escolhe o status com `errors.Is` |
| Um assunto grande, com o construtor e as rotas | `internal/contas` (`NovoServico`, `Registrar`, `rotas.go`, `testdata/`) | dependências pelo construtor; leitura com `consultas.DBTX`, que aceita `*sql.DB` e `*sql.Tx` |
| Regra usada por vários assuntos, sem rota | `internal/relatorios`, `internal/categorizacao` | pacote em `regraSemHTTP` no teste de arquitetura; nada de `net/http` |
| Cliente de serviço de fora, com tarefa periódica | `internal/cambio` (`ptax.go`, `servico.go`, `tarefa.go`, `testdata/`) | URL base e `http.Client` pelo construtor; respostas gravadas e fictícias em `testdata/`; tarefa pelo `internal/tarefas` |
| Consulta pelo `sqlc` | `internal/banco/consultas/sql` e quem usa (`internal/configuracao`, `internal/auditoria`) | SQL no arquivo `.sql`, `go generate ./...`, código gerado em dia (a CI confere) |
| Gravação em transação | `banco.EmTransacao` (`internal/banco/banco.go`) e o `Gravar` de `internal/transacoes` | só as colunas que mudaram, como o `SaveChanges` do EF Core |
| Gabarito contra o C# | `internal/texto` (`testdata/ordem.json` e a sonda em `testdata/sonda-csharp`), `internal/poucoinformativa/testdata` | o esperado sai de um programa C# pequeno (a sonda), não escrito à mão; o teste diz de onde veio |
| Testes que faltavam, achados pela mutação | `internal/cambio/mutacoes_test.go`, `internal/reservas/mutacoes_test.go` | um teste por mutação viva, com o comentário `// Regra: ...` dizendo o que ele garante |
| Tarefa em segundo plano com temporizador, sem `time.Sleep` | `internal/tarefas/tarefas_test.go` | `testing/synctest` |
| Fuzz de um tipo | `internal/tipos/fuzz_test.go`, `internal/relogio/fuzz_test.go` | sementes do gabarito; achado em `testdata/fuzz/<alvo>/` |
| Dados de teste | `internal/testeapoio` ([README](../../../../src/backend-go/internal/testeapoio/README.md)) | `NovoBanco`, `NovaFabrica`, `HojeEmBrasilia`, `Chamar` + `ConferirJSON`, `NovoServicoDeFora`. Tabela nova = método novo da `Fabrica` |

**Não copie de** pacote listado em `regraComHTTPPorEnquanto` no teste de arquitetura: é regra que ainda chega ao HTTP por outro pacote, uma exceção com prazo.
