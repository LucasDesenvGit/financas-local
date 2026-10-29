package relatorios

import (
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Regras de MesesCompletos e JanelaHistorico: os últimos N meses completos antes do mês atual, a partir do primeiro mês
// completo com transações. O primeiro mês completo é o da primeira transação, se ela é do dia 1, ou o mês seguinte. Com
// "meses" 0, ainda não há mês completo. A primeira transação é a do banco todo (inclusive ignorada, transferência, pendente).
func TestMesesCompletosEJanelaHistorico(t *testing.T) {
	t.Parallel()
	em := func(ano int, mes time.Month, d int) *tipos.Data { v := dia(ano, mes, d); return &v }
	casos := []struct {
		nome       string
		agora      relogio.Fixo
		primeira   *tipos.Data // a data da primeira transação; nil = banco sem transações
		quantidade int
		inicio     string
		fim        string
		meses      int
	}{
		{"sem transações: os seis meses cheios", testeapoio.HojeEmBrasilia(2026, 10, 22), nil, 6, "2026-04-01", "2026-09-30", 6},
		{"sem transações: os doze meses", testeapoio.HojeEmBrasilia(2026, 10, 22), nil, 12, "2025-10-01", "2026-09-30", 12},
		{"primeira transação antiga, no dia 1", testeapoio.HojeEmBrasilia(2026, 10, 22), em(2026, 1, 1), 6, "2026-04-01", "2026-09-30", 6},
		{"primeira no dia 1 dentro da janela: o próprio mês", testeapoio.HojeEmBrasilia(2026, 10, 22), em(2026, 6, 1), 6, "2026-06-01", "2026-09-30", 4},
		{"primeira no dia 2: o mês seguinte", testeapoio.HojeEmBrasilia(2026, 10, 22), em(2026, 6, 2), 6, "2026-07-01", "2026-09-30", 3},
		{"primeira no último dia do mês", testeapoio.HojeEmBrasilia(2026, 10, 22), em(2026, 6, 30), 6, "2026-07-01", "2026-09-30", 3},
		{"primeira no mês passado, dia 1: um mês", testeapoio.HojeEmBrasilia(2026, 10, 22), em(2026, 9, 1), 6, "2026-09-01", "2026-09-30", 1},
		{"primeira no mês passado, depois do dia 1: nenhum", testeapoio.HojeEmBrasilia(2026, 10, 22), em(2026, 9, 15), 6, "2026-10-01", "2026-09-30", 0},
		{"primeira no dia 1 do mês atual: nenhum", testeapoio.HojeEmBrasilia(2026, 10, 22), em(2026, 10, 1), 6, "2026-10-01", "2026-09-30", 0},
		{"primeira no mês atual: nenhum", testeapoio.HojeEmBrasilia(2026, 10, 22), em(2026, 10, 5), 6, "2026-11-01", "2026-09-30", 0},
		{"primeira no futuro: nenhum", testeapoio.HojeEmBrasilia(2026, 10, 22), em(2027, 3, 10), 6, "2027-04-01", "2026-09-30", 0},
		{"primeira na virada de ano, janela de doze", testeapoio.HojeEmBrasilia(2026, 10, 22), em(2025, 12, 31), 12, "2026-01-01", "2026-09-30", 9},
		{"hoje em janeiro: a janela atravessa o ano", testeapoio.HojeEmBrasilia(2026, 1, 5), nil, 6, "2025-07-01", "2025-12-31", 6},
		{"hoje em janeiro, primeira em dezembro", testeapoio.HojeEmBrasilia(2026, 1, 5), em(2025, 12, 1), 6, "2025-12-01", "2025-12-31", 1},
		{"hoje em março de ano bissexto: o fim é 29/02", testeapoio.HojeEmBrasilia(2024, 3, 1), nil, 1, "2024-02-01", "2024-02-29", 1},
		{"hoje em março de ano comum: o fim é 28/02", testeapoio.HojeEmBrasilia(2026, 3, 31), nil, 2, "2026-01-01", "2026-02-28", 2},
		{"31/10 às 22:30 em Brasília já é 01/11 em UTC: o mês atual é outubro", testeapoio.AgoraEmBrasilia(2026, 10, 31, 22, 30), nil, 6, "2026-04-01", "2026-09-30", 6},
		{"01/11 às 00:30 em Brasília: o mês atual é novembro", testeapoio.AgoraEmBrasilia(2026, 11, 1, 0, 30), nil, 6, "2026-05-01", "2026-10-31", 6},
		{"zero meses", testeapoio.HojeEmBrasilia(2026, 10, 22), nil, 0, "2026-10-01", "2026-09-30", 0},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			db := testeapoio.NovoBancoVazio(t)
			if c.primeira != nil {
				fab := testeapoio.NovaFabrica(t, db, c.agora)
				conta := fab.Conta()
				// Uma transação mais nova primeiro, para a primeira ser achada pelo menor Data, e não pela ordem do Id.
				fab.Transacao(conta, func(tr *testeapoio.Transacao) { tr.Data = c.primeira.SomarDias(40) })
				fab.Transacao(conta, func(tr *testeapoio.Transacao) { tr.Data = *c.primeira })
			}
			s := NovoServico(db, c.agora)
			inicio, fim, meses, err := s.MesesCompletos(t.Context(), c.quantidade)
			if err != nil {
				t.Fatal(err)
			}
			if inicio.String() != c.inicio || fim.String() != c.fim || meses != c.meses {
				t.Errorf("MesesCompletos(%d) = %s, %s, %d; esperado %s, %s, %d", c.quantidade, inicio, fim, meses, c.inicio, c.fim, c.meses)
			}
			if c.quantidade == MesesDoHistorico {
				inicioJanela, mesesJanela, err := s.JanelaHistorico(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				if inicioJanela.String() != c.inicio || mesesJanela != c.meses {
					t.Errorf("JanelaHistorico = %s, %d; esperado %s, %d", inicioJanela, mesesJanela, c.inicio, c.meses)
				}
			}
		})
	}
}

// A primeira transação é a de qualquer tipo e status: ignorada nos relatórios, transferência e pendente também contam
// para saber desde quando há dados.
func TestMesesCompletosContaAPrimeiraTransacaoDeQualquerTipo(t *testing.T) {
	t.Parallel()
	agora := testeapoio.HojeEmBrasilia(2026, 10, 22)
	casos := []struct {
		nome     string
		ajustar  func(*testeapoio.Transacao)
		esperado string
	}{
		{"ignorada nos relatórios", func(tr *testeapoio.Transacao) { tr.IgnorarRelatorios = true }, "2026-06-01"},
		{"transferência", func(tr *testeapoio.Transacao) { tr.Tipo = 3 }, "2026-06-01"},
		{"pendente", func(tr *testeapoio.Transacao) { tr.Status = 2 }, "2026-06-01"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			db := testeapoio.NovoBancoVazio(t)
			fab := testeapoio.NovaFabrica(t, db, agora)
			conta := fab.Conta()
			fab.Transacao(conta, func(tr *testeapoio.Transacao) { tr.Data = dia(2026, 8, 20) })
			fab.Transacao(conta, func(tr *testeapoio.Transacao) { tr.Data = dia(2026, 6, 1); c.ajustar(tr) })
			inicio, _, meses, err := NovoServico(db, agora).MesesCompletos(t.Context(), 6)
			if err != nil {
				t.Fatal(err)
			}
			if inicio.String() != c.esperado || meses != 4 {
				t.Errorf("início %s, %d meses; esperado %s, 4", inicio, meses, c.esperado)
			}
		})
	}
}

// montarRenda grava os lançamentos da renda: a janela de 12 meses vai de 01/10/2025 a 30/09/2026 (hoje é 22/10/2026).
func montarRenda(t *testing.T, agora relogio.Fixo) *Servico {
	t.Helper()
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, agora)
	conta := fab.Conta()
	grava := func(d tipos.Data, valor string, ajustes ...func(*testeapoio.Transacao)) {
		fab.Transacao(conta, append([]func(*testeapoio.Transacao){func(tr *testeapoio.Transacao) {
			tr.Data = d
			tr.Valor = dec(valor)
		}}, ajustes...)...)
	}
	tipo := func(n int64) func(*testeapoio.Transacao) { return func(tr *testeapoio.Transacao) { tr.Tipo = n } }
	parcelas := func(n int64) func(*testeapoio.Transacao) {
		return func(tr *testeapoio.Transacao) { tr.ParcelaTotal = &n }
	}
	ignorar := func(tr *testeapoio.Transacao) { tr.IgnorarRelatorios = true }
	pendente := func(tr *testeapoio.Transacao) { tr.Status = 2 }

	// A primeira transação (30/09/2025, fora da janela) fixa o começo dela em 01/10/2025.
	grava(dia(2025, 9, 30), "500.00", tipo(2))
	grava(dia(2025, 10, 1), "1000.00", tipo(2)) // primeiro dia da janela: entra
	grava(dia(2026, 9, 30), "2000.00", tipo(2)) // último dia da janela: entra
	grava(dia(2026, 10, 1), "5000.00", tipo(2)) // mês atual: fora
	grava(dia(2026, 3, 10), "700.00", tipo(2), ignorar)
	grava(dia(2026, 3, 11), "800.00", tipo(3))
	grava(dia(2026, 3, 12), "300.00", tipo(2), pendente) // pendente entra
	grava(dia(2026, 3, 13), "-50.00")                    // despesa: não é renda

	// Parcelas do último mês da janela (setembro de 2026).
	grava(dia(2026, 9, 1), "-120.50", parcelas(3))
	grava(dia(2026, 9, 30), "-79.50", parcelas(2))
	grava(dia(2026, 9, 15), "-10.00", parcelas(1))          // à vista (1 parcela): fora
	grava(dia(2026, 9, 16), "-20.00")                       // sem parcelas: fora
	grava(dia(2026, 8, 31), "-99.00", parcelas(3))          // agosto: fora
	grava(dia(2026, 10, 1), "-77.00", parcelas(3))          // mês atual: fora
	grava(dia(2026, 9, 17), "-33.00", parcelas(3), ignorar) // ignorada: fora
	grava(dia(2026, 9, 18), "-44.00", parcelas(3), tipo(3)) // transferência: fora
	grava(dia(2026, 9, 19), "60.00", parcelas(3), tipo(2))  // receita parcelada: não é despesa (mas é receita)
	return NovoServico(db, agora)
}

// Regra: RendaEParcelas (a Saúde financeira e a sugestão de orçamentos): a renda é a soma das receitas da janela dividida
// pelos meses, e as parcelas são as despesas parceladas (mais de uma parcela) do último mês, em valor positivo.
func TestRendaEParcelas(t *testing.T) {
	t.Parallel()
	agora := testeapoio.HojeEmBrasilia(2026, 10, 22)
	renda, parcelas, meses, err := montarRenda(t, agora).RendaEParcelas(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	// Receitas: 1000 + 2000 + 300 (pendente) + 60 (a receita parcelada também é receita) = 3360, em 12 meses: 280.
	if renda.String() != "280.0" || parcelas.String() != "200.0" || meses != 12 {
		t.Errorf("renda %s, parcelas %s, meses %d; esperado 280.0, 200.0, 12", renda, parcelas, meses)
	}
}

// Hoje em Brasília é 31/10 (22:30); em UTC já é novembro. A receita de 15/10 é do mês atual e fica fora da janela. Quem usa
// o fuso errado leva outubro para dentro da janela.
func TestRendaEParcelasNaVirada(t *testing.T) {
	t.Parallel()
	agora := testeapoio.AgoraEmBrasilia(2026, 10, 31, 22, 30)
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, agora)
	conta := fab.Conta()
	fab.Transacao(conta, func(tr *testeapoio.Transacao) { tr.Data = dia(2025, 10, 1); tr.Tipo = 2; tr.Valor = dec("1200.00") })
	fab.Transacao(conta, func(tr *testeapoio.Transacao) { tr.Data = dia(2026, 10, 15); tr.Tipo = 2; tr.Valor = dec("9999.00") })
	renda, _, meses, err := NovoServico(db, agora).RendaEParcelas(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if renda.String() != "100.0" || meses != 12 {
		t.Errorf("renda %s em %d meses; esperado 100.0 em 12 (1200 ÷ 12; outubro de 2026 não entra)", renda, meses)
	}
}

// Sem mês completo (a primeira transação é deste mês), tudo é zero, sem casas.
func TestRendaEParcelasSemMesCompleto(t *testing.T) {
	t.Parallel()
	agora := testeapoio.HojeEmBrasilia(2026, 10, 22)
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, agora)
	fab.Transacao(fab.Conta(), func(tr *testeapoio.Transacao) { tr.Data = dia(2026, 10, 5); tr.Tipo = 2; tr.Valor = dec("3000.00") })
	renda, parcelas, meses, err := NovoServico(db, agora).RendaEParcelas(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if renda.String() != "0" || parcelas.String() != "0" || meses != 0 {
		t.Errorf("renda %s, parcelas %s, meses %d; esperado 0, 0, 0", renda, parcelas, meses)
	}
}

// Menos de 12 meses de dados: divide pelos meses que existem, com a divisão do decimal do C# (até 28 casas, sem
// arredondar). Parcela que só tem estorno no último mês dá zero (Math.Max(0m, ...)), sem casas.
func TestRendaEParcelasComPoucosMeses(t *testing.T) {
	t.Parallel()
	agora := testeapoio.HojeEmBrasilia(2026, 10, 22)
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, agora)
	conta := fab.Conta()
	fab.Transacao(conta, func(tr *testeapoio.Transacao) { tr.Data = dia(2026, 7, 1); tr.Tipo = 2; tr.Valor = dec("1000.00") })
	// Estorno de uma compra parcelada: despesa positiva, e o total de parcelas do mês fica negativo.
	fab.Transacao(conta, func(tr *testeapoio.Transacao) {
		tr.Data = dia(2026, 9, 10)
		tr.Valor = dec("50.00")
		tr.ParcelaTotal = new(int64(4))
	})
	renda, parcelas, meses, err := NovoServico(db, agora).RendaEParcelas(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if renda.String() != "333.33333333333333333333333333" || parcelas.String() != "0" || meses != 3 {
		t.Errorf("renda %s, parcelas %s, meses %d; esperado 333.33333333333333333333333333, 0, 3", renda, parcelas, meses)
	}
}
