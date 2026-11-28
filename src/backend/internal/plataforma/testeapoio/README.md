# testeapoio: apoio comum aos testes do back-end em Go

Banco temporário, fábrica de dados inventados, relógio parado em Brasília, chamada de handler com o usuário logado e serviço de fora falso, para cada assunto não escrever o seu ([ADR 0051](../../../../../docs/adr/0051-testes-do-back-end-em-go-cada-regra-com-teste.md)). Só a biblioteca padrão; nada de mock nem de asserção de terceiros.

**Só arquivo `_test.go` importa este pacote** (o `TestSoImportadoPorTestes` confere).

## O que tem

O detalhe de cada função, com os valores padrão da fábrica, está no comentário dela (`go doc -all ./internal/plataforma/testeapoio`).

| Função | Para quê |
|---|---|
| `NovoBanco(t) *sql.DB` | SQLite num arquivo do `t.TempDir()`, com o **esquema real** e os dados iniciais, fechado no fim do teste |
| `NovoBancoVazio(t) *sql.DB` | o mesmo, sem as categorias e regras padrão: o primeiro Id é 1. Para conferir uma lista inteira ou os Ids no JSON |
| `NovaFabrica(t, db, relogio) *Fabrica` | grava dados inventados no formato do EF Core; `db` pode ser `*sql.DB` ou `*sql.Tx` |
| `fab.Conta`, `Categoria`, `Subcategoria`, `Transacao`, `RegraCategorizacao`, `ConexaoBancaria`, `Emprestimo`, `Orcamento`, `Fatura`, `ContaAgendada`, `CotacaoMoeda`, `CotacaoCripto`, `ReservaConferencia`, `Ativo`, `OperacaoAtivo`, `Investimento`, `Configuracao` | uma linha da tabela com valores padrão; os `ajustes ...func(*T)` mudam o que o teste precisa |
| `RelogioDosTestes()` | parado em 23/10/2026 02:30 UTC, que em Brasília ainda é dia **22**: pega quem confunde UTC com Brasília |
| `HojeEmBrasilia(2026, 10, 31)` | "hoje em Brasília é dia 31", ao meio-dia (longe da virada do dia) |
| `AgoraEmBrasilia(2026, 12, 31, 23, 30)` | a hora exata em Brasília, para a virada do dia, do mês e do ano |
| `Dia(2024, 2, 29)`, `Valor("-12.50")` | atalhos para `tipos.Data` e `tipos.Decimal` (a escala fica: `"10.50"` sai `10.50` no JSON) |
| `Chamar(t, h, "GET", "/api/x", "")` | chama o handler direto, sem porta, com `UsuarioDosTestes` (Id 1) logado |
| `ChamarComo(t, h, id, ...)` | o mesmo com outro usuário; `0` = ninguém logado |
| `resposta.ConferirStatus(t, 200)` | status, mostrando o corpo quando erra |
| ``resposta.ConferirJSON(t, `{"valor":10.50}`)`` | o JSON pelo comparador do contrato: número pelo texto (`10.50` ≠ `10.5`), `[]` ≠ `null`, `null` ≠ ausente; a ordem das chaves não importa, a dos itens importa |
| `NovoServicoDeFora(t, segredos...)` | serviço de fora (PTAX, Binance, Yahoo, BCB, Pluggy) num servidor HTTPS de teste. `URL()` e `Cliente()` vão para o construtor do cliente; `Em(...)` cadastra as respostas; `Exigir(...)` confere um cabeçalho; `Pedidos()` e `URIs()` devolvem o que chegou, sem o valor dos segredos. Pedido sem rota é erro do teste |
| `RespostaDeFora{Arquivo: "x.json"}` | a resposta, de `testdata/` do pacote ou do `Corpo`. Prontas: `TempoEsgotado`, `NaoAutorizado`, `MuitosPedidos`, `ErroDoServidor`, `JSONQuebrado`, `PaginaVazia("results")`. **Fictícias**, no formato real do serviço: nunca de uma conta de verdade |

Campo da fábrica que aceita `NULL` é ponteiro; preencha com o `new` do Go 1.26: `c.Banco = new("nubank")`, `c.LimiteCredito = new(testeapoio.Valor("5000"))`. Os enums são `int64`, com os valores no comentário de cada campo.

## Exemplo

```go
func TestExcluirCategoriaComSubcategoria(t *testing.T) {
	t.Parallel() // cada teste tem o próprio banco
	db := testeapoio.NovoBancoVazio(t)
	r := testeapoio.HojeEmBrasilia(2026, 10, 22)
	fab := testeapoio.NovaFabrica(t, db, r)

	casa := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome = "Casa" })
	fab.Subcategoria(casa, func(c *testeapoio.Categoria) { c.Nome = "Aluguel" })

	mux := http.NewServeMux()
	Registrar(mux, NovoServico(db, r, slog.New(slog.DiscardHandler))) // o Registrar do assunto (aqui, categorias)
	resposta := testeapoio.Chamar(t, mux, "DELETE", fmt.Sprintf("/api/categorias/%d", casa.Id), "")
	resposta.ConferirStatus(t, http.StatusBadRequest)
	resposta.ConferirJSON(t, `{"mensagem":"Exclua ou mova as subcategorias antes."}`)
}
```

## Cuidados

- **Ciclo de import:** `banco`, `relogio`, `tipos` e `sessao` não importam este pacote nos próprios testes ([CONTRIBUINDO.md](../../../../../CONTRIBUINDO.md#testes-do-back-end-go)). Pelo mesmo motivo, ele não importa pacote de assunto: os enums ficam em `int64`.
- **Tabela nova:** acrescente o método na `Fabrica`, com todas as colunas no `INSERT` e os valores padrão da entidade, e um teste que confere o formato gravado. Dois assuntos que acrescentam métodos em paralelo resolvem o conflito ficando com os dois.
- **Dinheiro:** as [armadilhas do banco](../banco/README.md#armadilhas) valem aqui.
- **Sem dado real:** nomes, valores e textos inventados.
- **`t.Parallel()`:** pode em todo teste que usa `NovoBanco` (cada um tem o próprio arquivo); não pode com `t.Setenv`, `t.Chdir` ou variável de pacote.
