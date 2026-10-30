# testeapoio: apoio comum aos testes do back-end em Go

O que os testes dos assuntos do #70 dividem, para cada um não escrever o seu (#369, ADR 0051): banco temporário, fábrica de dados inventados, relógio parado em Brasília e chamada de handler com o usuário logado. Só a biblioteca padrão e o que o projeto já tem; nada de mock nem de asserção de terceiros.

**Só arquivo `_test.go` importa este pacote.** Ele importa `testing`, e o `go build` ignora os `_test.go`, então ele não entra no executável. O `TestSoImportadoPorTestes` falha se um arquivo comum o importar; para conferir à mão: `go list -deps ./cmd/financas | grep testeapoio` não mostra nada.

## O que tem

| Função | Para quê |
|---|---|
| `NovoBanco(t) *sql.DB` | SQLite num arquivo do `t.TempDir()`, com o **esquema real** (`banco.AplicarEsquema`: os scripts de `banco/` embutidos, inclusive as colunas novas do `2-criar-banco.sh`), aberto pelo `banco.Abrir` (chave estrangeira ligada) e fechado no `t.Cleanup`. Vem com os dados iniciais, como um banco novo do usuário. |
| `NovoBancoVazio(t) *sql.DB` | o mesmo, sem as categorias e regras padrão e com o `AUTOINCREMENT` delas zerado: o primeiro Id é 1. Para conferir uma lista inteira ou os Ids no JSON. |
| `NovaFabrica(t, db, relogio) *Fabrica` | grava dados inventados no formato do EF Core. `db` pode ser `*sql.DB` ou `*sql.Tx`. |
| `fab.Conta(ajustes...)` | conta corrente manual, em reais, saldo zero, entra no saldo |
| `fab.Categoria(ajustes...)` | categoria de despesa ativa, "Categoria de teste N" (nome único) |
| `fab.Subcategoria(pai, ajustes...)` | categoria filha, com o tipo do pai |
| `fab.Transacao(conta, ajustes...)` | despesa efetivada de `-10.00`, sem categoria, com a data de hoje em Brasília (pelo relógio da fábrica) |
| `fab.RegraCategorizacao(categoria, ajustes...)` | regra ativa "contém", prioridade 100 |
| `fab.ConexaoBancaria(ajustes...)` | conexão do Open Finance ativa, "Banco de teste N" (#447) |
| `fab.Emprestimo(conexao, ajustes...)` | empréstimo (Natureza 1) da conexão: contratado 10000.0, saldo devedor 5000.0, sem parcela (#447) |
| `fab.Orcamento(categoria, ajustes...)` | orçamento de 500.0 da categoria no mês de hoje em Brasília (#447) |
| `fab.Fatura(cartao, vencimento, total, ajustes...)` | fatura importada do cartão, sem fechamento, mínimo nem pago (#77) |
| `fab.ContaAgendada(ajustes...)` | conta a pagar ativa de 100.0, mensal, paga à mão, que vence hoje em Brasília (#447) |
| `fab.CotacaoMoeda(ajustes...)` | PTAX do dólar de hoje (em Brasília): compra 5.4123, venda 5.4129 (#81) |
| `fab.CotacaoCripto(ajustes...)` | bitcoin agora (UTC): 612345.67 (#81) |
| `fab.ReservaConferencia(conta, ajustes...)` | conferência da reserva "Viagem" da conta, hoje (em Brasília), com 100.0 (#82) |
| `fab.Configuracao(chave, valor)` | configuração que não é segredo (`Protegido = 0`) |
| `RelogioDosTestes()` | parado em 23/10/2026 02:30 UTC, que em Brasília ainda é dia **22**: pega quem confunde UTC com Brasília |
| `HojeEmBrasilia(2026, 10, 31)` | "hoje em Brasília é dia 31", ao meio-dia (longe da virada do dia) |
| `AgoraEmBrasilia(2026, 12, 31, 23, 30)` | a hora exata em Brasília, para a virada do dia, do mês e do ano |
| `Dia(2024, 2, 29)`, `Valor("-12.50")` | atalhos para `tipos.Data` e `tipos.Decimal` (a escala fica: `"10.50"` sai `10.50` no JSON) |
| `Chamar(t, h, "GET", "/api/x", "")` | chama o handler direto, sem porta, com `UsuarioDosTestes` (Id 1) no contexto, como o servidor faz depois do .NET (`sessao.ComUsuario`). Corpo não vazio vai como `application/json`. |
| `ChamarComo(t, h, id, ...)` | o mesmo com outro usuário; `0` = ninguém logado |
| `NovoServicoDeFora(t, segredos...)` | um serviço de fora (PTAX, Mercado Bitcoin, Yahoo, brapi, BCB, Pluggy) num servidor HTTPS de teste (#460). `URL()` e `Cliente()` vão para o construtor do cliente; `Em("GET", "/caminho", respostas...)` cadastra as respostas em sequência (a última se repete; `"*"` vale para todo caminho sem rota própria); `Exigir("User-Agent", "FinancasLocal/1.0")` confere um cabeçalho em todo pedido; `Pedidos()` e `URIs()` devolvem o que chegou, com o valor dos cabeçalhos de segredo trocado por `<segredo>`. Pedido sem rota é erro do teste (404) |
| `RespostaDeFora{Arquivo: "x.json"}` | a resposta: o arquivo de `testdata/` do pacote ou o `Corpo`, com `Status` (0 = 200), `TipoConteudo` (vazio = JSON) e `Cabecalhos`. Prontas: `TempoEsgotado` (só responde quando o cliente desiste), `NaoAutorizado` (401), `MuitosPedidos` (429, com `Retry-After`), `ErroDoServidor` (500), `JSONQuebrado` e `PaginaVazia("results")`. As respostas são **fictícias**, no formato real de cada serviço: nunca de uma conta de verdade |
| `resposta.ConferirStatus(t, 200)` | status, mostrando o corpo quando erra |
| ``resposta.ConferirJSON(t, `{"valor":10.50}`)`` | o JSON pelo comparador do contrato (`contrato.CompararJSON`): número pelo texto, nunca `float64` (`10.50` ≠ `10.5`), `[]` ≠ `null`, `null` ≠ ausente; a ordem das chaves não importa, a dos itens importa |

Campos da fábrica que o banco aceita `NULL` são ponteiros; preencha com o `new` do Go 1.26: `c.Banco = new("nubank")`, `c.LimiteCredito = new(testeapoio.Valor("5000"))`. Os enums são `int64`, com os valores do `Entidades/Enums.cs` no comentário de cada campo.

## Exemplo

```go
func TestTotalPorCategoria(t *testing.T) {
	t.Parallel() // cada teste tem o próprio banco
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, testeapoio.HojeEmBrasilia(2026, 10, 22))

	mercado := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome = "Mercado" })
	conta := fab.Conta()
	fab.Transacao(conta, func(tr *testeapoio.Transacao) {
		tr.Valor = testeapoio.Valor("-12.50")
		tr.CategoriaId = &mercado.Id
	})

	mux := http.NewServeMux()
	relatorios.Registrar(mux, db, testeapoio.RelogioDosTestes()) // o Registrar do assunto
	resposta := testeapoio.Chamar(t, mux, "GET", "/api/relatorios/categorias?mes=2026-10", "")
	resposta.ConferirStatus(t, http.StatusOK)
	resposta.ConferirJSON(t, `[{"categoriaId":1,"nome":"Mercado","total":-12.50}]`)
}
```

O `TestExemploDeIntegracao` (`testeapoio_test.go`) é um exemplo que roda: banco, fábrica e uma soma em decimal.

## Tempo

Medido aqui (16 núcleos, Linux) no `TestExemploDeIntegracao`: banco novo com o esquema inteiro, quatro linhas da fábrica e uma consulta.

| Como | Tempo |
|---|---|
| `go test` | ~5 ms |
| `go test -race`, sozinho | ~0,16 s |
| `go test -race`, com os 10 testes paralelos do pacote | ~0,6 s |

O `-race` deixa o SQLite em Go puro dezenas de vezes mais lento. Por isso o exemplo só falha acima de 1 s sem o `-race`; a trava de tempo dos testes é da #371.

## Cuidados

- **Quem este pacote importa não pode importá-lo no teste do próprio pacote** (`package banco` dentro de `internal/banco`): o Go recusa o ciclo. `banco`, `relogio`, `tipos` e `sessao` usam um ajudante local ou um pacote de teste externo (`package banco_test`). Pelo mesmo motivo, **este pacote não importa pacote de assunto**: os enums ficam em `int64`.
- **Assunto que precisa de outra tabela:** acrescente o método na `Fabrica` (todas as colunas no `INSERT`, como o EF Core; valores padrão do C# da entidade), com um teste que confere o formato gravado. Assuntos em paralelo acrescentam métodos diferentes, então o conflito no merge é "fique com os dois".
- **Dinheiro:** nunca `float64`; some e compare em Go com `tipos.Decimal`. No SQL, nada de `SUM`, `ORDER BY` ou `<`/`>` em coluna de dinheiro (é `TEXT`).
- **Sem dado real:** nomes, valores e textos inventados. A fábrica já dá nomes como "Conta de teste 1".
- **`t.Parallel()`:** pode, em todo teste que usa `NovoBanco`, porque cada um tem o próprio arquivo. Não pode com `t.Setenv`, `t.Chdir` ou variável de pacote (ADR 0051).
