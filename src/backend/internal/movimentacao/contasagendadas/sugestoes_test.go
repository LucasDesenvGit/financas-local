package contasagendadas

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// Os testes deste arquivo vieram da #640: cada um pega mutações que sobreviviam no teste de mutação (o comentário diz
// quais), sem mudar o comportamento.

// Regra (#62): mensal também na virada do ano, e no limite exato de 1,5 pagamento por mês (2n = 3m ainda vale).
// Mutações: o mês corrido com "/" ou "-" no lugar de "*" ou "+" (os meses de anos diferentes se misturam), e o ">" do
// 1,5 trocado por ">=".
func TestEhMensalNaViradaDoAnoENoLimite(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome  string
		hoje  tipos.Data
		datas []tipos.Data
		quer  bool
	}{
		{"novembro, dezembro e janeiro", testeapoio.Dia(2026, 1, 20),
			[]tipos.Data{testeapoio.Dia(2026, 1, 5), testeapoio.Dia(2025, 12, 5), testeapoio.Dia(2025, 11, 5)}, true},
		{"6 pagamentos em 4 meses: 1,5 por mês, no limite", dia(14),
			[]tipos.Data{dia(9), dia(2), testeapoio.Dia(2026, 9, 9), testeapoio.Dia(2026, 9, 2), testeapoio.Dia(2026, 8, 9),
				testeapoio.Dia(2026, 7, 9)}, true},
	}
	for _, c := range casos {
		if obtido := ehMensal(c.datas, c.hoje); obtido != c.quer {
			t.Errorf("%s: %v", c.nome, obtido)
		}
	}
}

// Regra: a categoria que mais aparece entre os lançamentos com categoria; no empate, a primeira; sem categoria no
// primeiro, as dos outros contam. Mutações: o "continue" do sem categoria trocado por "break", o "== 0" da primeira
// aparição trocado por "!= 0" e a contagem que desce em vez de subir.
func TestCategoriaMaisFrequente(t *testing.T) {
	t.Parallel()
	com := func(id int64) lancamento {
		return lancamento{categoriaId: ptr(id), categoriaNome: ptr(fmt.Sprint("C", id))}
	}
	casos := []struct {
		nome  string
		itens []lancamento
		quer  *int64
	}{
		{"o primeiro sem categoria", []lancamento{{}, com(7), com(7)}, ptr(int64(7))},
		{"três diferentes: a primeira", []lancamento{com(1), com(2), com(3)}, ptr(int64(1))},
		{"a que mais aparece, mesmo depois", []lancamento{com(1), com(2), com(2)}, ptr(int64(2))},
		{"nenhuma", []lancamento{{}, {}}, nil},
	}
	for _, c := range casos {
		id, nome := categoriaMaisFrequente(c.itens)
		if (id == nil) != (c.quer == nil) || (id != nil && (*id != *c.quer || *nome != fmt.Sprint("C", *c.quer))) {
			t.Errorf("%s: %v %v", c.nome, id, nome)
		}
	}
}

// Regra (#158): todo trecho que casa conta, não só o primeiro: em "LOJA1/2 PARC", o "1/2" não tem borda, e o "PARC"
// tem. Mutação: o -1 do FindAllStringIndex trocado (só o primeiro trecho).
func TestParcelaNoTextoOlhaTodosOsTrechos(t *testing.T) {
	t.Parallel()
	if !parcelaNoTexto("LOJA1/2 PARC") {
		t.Error("o PARC depois de um trecho sem borda não contou")
	}
}

// Regra (#205): fora da variável, a faixa não vale; na variável, os dois lados da faixa valem; o dia a exatamente 3
// ainda é parecido. Mutações: os dois "&&" da faixa trocados por "||", e o "<= 3" por "< 3".
func TestParecidaNaFaixaENoLimiteDoDia(t *testing.T) {
	t.Parallel()
	v := testeapoio.Valor
	s := SugestaoDto{Valor: v("100.0"), ValorMinimo: v("80.0"), ValorMaximo: v("120.0"), ContaId: 1, DiaHabitual: 10,
		Recorrencia: 1}
	conta := func(valor string, d int) cadastrada {
		return cadastrada{valor: v(valor), contaId: ptr(int64(1)), proximoVencimento: dia(d), recorrencia: 1}
	}
	variavel := s
	variavel.ValorVariavel = true
	casos := []struct {
		nome string
		s    SugestaoDto
		c    cadastrada
		quer bool
	}{
		{"fixa: dentro da faixa, mas longe do valor", s, conta("82.0", 10), false},
		{"variável: acima da faixa", variavel, conta("200.0", 10), false},
		{"variável: abaixo da faixa", variavel, conta("50.0", 10), false},
		{"variável: dentro da faixa", variavel, conta("82.0", 10), true},
		{"dia a 3", s, conta("100.0", 13), true},
		{"dia a 4", s, conta("100.0", 14), false},
	}
	for _, c := range casos {
		if obtido := parecida(c.s, c.c); obtido != c.quer {
			t.Errorf("%s: %v", c.nome, obtido)
		}
	}
}

// Regra (#205): a sugestão que some (uma só conta a pagar, certa) não leva as outras junto. Mutação: o "continue"
// trocado por "break".
func TestSemAsParecidasSegueDepoisDaQueSome(t *testing.T) {
	t.Parallel()
	v := testeapoio.Valor
	certa := cadastrada{id: 1, descricao: "Internet", ativa: true, valor: v("100.0"), contaId: ptr(int64(1)),
		proximoVencimento: dia(10), recorrencia: 1}
	internet := SugestaoDto{Chave: "NET", Valor: v("100.0"), ContaId: 1, DiaHabitual: 10, Recorrencia: 1}
	academia := SugestaoDto{Chave: "ACADEMIA", Valor: v("300.0"), ContaId: 1, DiaHabitual: 20, Recorrencia: 1}
	r := semAsParecidas([]SugestaoDto{internet, academia}, []cadastrada{certa})
	if len(r) != 1 || r[0].Chave != "ACADEMIA" {
		t.Errorf("%+v", r)
	}
}

// Regra (#62, #158, #205): o que tira um pagamento das sugestões e o que não tira.
//   - a conta a pagar com o mesmo nome do grupo, mesmo inativa e sem a chave do extrato, tira o grupo; e os grupos
//     depois dele continuam;
//   - a conta a pagar com o nome da descrição (renomeada por regra) tira o grupo do estabelecimento;
//   - no cartão, a compra com parcela no texto não é sugestão; a assinatura é, com o valor fixo;
//   - fora do cartão, a parcela no texto não tira (mensalidade da escola);
//   - o estorno (despesa com valor positivo) não conta como pagamento.
//
// Mutações: a chave da descrição com "== \"\"", a chave do extrato com "||" (nil), o "continue" do grupo já cadastrado
// trocado por "break", o "||" da descrição renomeada trocado por "&&", as duas condições da parcela, o "&&" do valor
// variável trocado por "||" (e o do maiorQue), e o "&&" do valor negativo na leitura.
func TestSugerirOQueTiraEOQueNaoTira(t *testing.T) {
	t.Parallel()
	_, fab, mux := ambiente(t)
	corrente := fab.Conta(func(c *testeapoio.Conta) { c.Nome = "Corrente" })
	cartao := fab.Conta(func(c *testeapoio.Conta) { c.Nome, c.Tipo = "Cartão", 3 })
	gastar := func(conta testeapoio.Conta, d tipos.Data, valor, descricao string, estabelecimento *string) {
		fab.Transacao(conta, func(tr *testeapoio.Transacao) {
			tr.Data, tr.Valor, tr.Descricao, tr.Estabelecimento, tr.Tipo = d, testeapoio.Valor(valor), descricao, estabelecimento, 1
		})
	}
	mensal := func(conta testeapoio.Conta, diaDoMes int, valor, descricao string, estabelecimento *string) {
		for _, m := range []time.Month{10, 9, 8} {
			gastar(conta, testeapoio.Dia(2026, m, diaDoMes), valor, descricao, estabelecimento)
		}
	}
	// Os grupos saem na ordem do lançamento mais recente: a academia (já cadastrada) vem primeiro.
	mensal(corrente, 12, "-120.0", "ACADEMIA FORCA", nil)
	mensal(corrente, 11, "-55.9", "Netflix", ptr("NFLX STREAMING"))
	mensal(corrente, 10, "-900.0", "MENSALIDADE ESCOLA PARCELA", nil)
	mensal(cartao, 9, "-21.9", "SPOTIFY", nil)
	mensal(cartao, 8, "-300.0", "LOJA DE MOVEIS PARCELA", nil)
	gastar(cartao, testeapoio.Dia(2026, 7, 9), "21.9", "SPOTIFY", nil) // estorno: não é pagamento
	for _, nome := range []string{"Academia Força", "Netflix"} {
		fab.ContaAgendada(func(a *testeapoio.ContaAgendada) { a.Descricao, a.Ativa = nome, false })
	}

	sugestoes := lerJSON[[]SugestaoDto](t, testeapoio.Chamar(t, mux, "GET", "/api/contas-agendadas/sugestoes", "").Corpo)
	var chaves []string
	porChave := map[string]SugestaoDto{}
	for _, s := range sugestoes {
		chaves = append(chaves, s.Chave)
		porChave[s.Chave] = s
	}
	if strings.Join(chaves, ",") != "SPOTIFY,MENSALIDADE ESCOLA" {
		t.Fatalf("sugestões: %v", chaves)
	}
	if s := porChave["SPOTIFY"]; s.ValorVariavel || s.Pagamentos != 3 || s.PrimeiroPagamento.String() != "2026-08-09" ||
		s.FormaCobranca != formaCartao {
		t.Errorf("Spotify: %+v", s)
	}
}

// Regra: só os 20.000 lançamentos mais recentes entram (o Take do C#): o 20.001º, o mais antigo, fica de fora, mesmo
// que forme uma anual com outro. Mutação: o "<" do limite trocado por "<=".
func TestSugerirLeNoMaximo20000Lancamentos(t *testing.T) {
	t.Parallel()
	db, fab, mux := ambiente(t)
	corrente := fab.Conta()
	modelo := fab.Transacao(corrente, func(tr *testeapoio.Transacao) {
		tr.Data, tr.Valor, tr.Descricao, tr.Tipo = dia(14), testeapoio.Valor("-1.0"), "AVULSA", 1
	})
	// Mais 19.998 avulsas iguais a ela, no mesmo dia: o mesmo grupo, que não é recorrente.
	var colunas []string
	linhas, err := db.Query(`SELECT name FROM pragma_table_info('Transacao') WHERE name <> 'Id'`)
	if err != nil {
		t.Fatal(err)
	}
	for linhas.Next() {
		var c string
		if err := linhas.Scan(&c); err != nil {
			t.Fatal(err)
		}
		colunas = append(colunas, `"`+c+`"`)
	}
	if err := linhas.Close(); err != nil {
		t.Fatal(err)
	}
	lista := strings.Join(colunas, ", ")
	if _, err := db.Exec(`WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i + 1 FROM n WHERE i < 19998)
		INSERT INTO "Transacao" (`+lista+`) SELECT `+lista+` FROM n, "Transacao" WHERE "Id" = ?`, modelo.Id); err != nil {
		t.Fatal(err)
	}
	// O seguro anual: a cobrança deste ano é o 20.000º lançamento; a do ano passado, o 20.001º.
	for _, d := range []tipos.Data{dia(1), testeapoio.Dia(2025, 10, 20)} {
		fab.Transacao(corrente, func(tr *testeapoio.Transacao) {
			tr.Data, tr.Valor, tr.Descricao, tr.Tipo = d, testeapoio.Valor("-900.0"), "SEGURO ANUAL CASA", 1
		})
	}
	sugestoes := lerJSON[[]SugestaoDto](t, testeapoio.Chamar(t, mux, "GET", "/api/contas-agendadas/sugestoes", "").Corpo)
	if len(sugestoes) != 0 {
		t.Errorf("o 20.001º lançamento entrou: %+v", sugestoes)
	}
}
