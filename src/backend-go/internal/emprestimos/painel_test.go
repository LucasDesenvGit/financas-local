package emprestimos

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

func ptr[T any](v T) *T { return &v }

// Regra (ADR 0014): a modalidade conhecida do Open Finance tem o nome em português (sem diferenciar maiúsculas); sem
// modalidade, "Financiamento" ou "Empréstimo" pela natureza; a desconhecida sai em título, como o ToTitleCase pt-BR.
func TestDescreverModalidade(t *testing.T) {
	t.Parallel()
	casos := []struct {
		modalidade *string
		natureza   int64
		quer       string
	}{
		{ptr("CREDITO_PESSOAL_SEM_CONSIGNACAO"), NaturezaEmprestimo, "Crédito pessoal"},
		{ptr("home_equity"), NaturezaEmprestimo, "Empréstimo com garantia de imóvel"},
		{ptr("MICROCREDITO"), NaturezaEmprestimo, "Microcrédito"},
		{ptr("AQUISICAO_BENS_VEICULOS_AUTOMOTORES"), NaturezaFinanciamento, "Financiamento de veículo"},
		{nil, NaturezaFinanciamento, "Financiamento"},
		{ptr(""), NaturezaEmprestimo, "Empréstimo"},
		{ptr("FINANCIAMENTO_AQUISICAO_BENS_VEICULOS_AUTOMOTORES"), NaturezaFinanciamento,
			"Financiamento Aquisicao Bens Veiculos Automotores"},
		{ptr("CREDITO_2A_FASE"), NaturezaEmprestimo, "Credito 2a Fase"},
	}
	for _, c := range casos {
		if obtido := DescreverModalidade(c.modalidade, c.natureza); obtido != c.quer {
			t.Errorf("DescreverModalidade(%v, %d) = %q, quer %q", c.modalidade, c.natureza, obtido, c.quer)
		}
	}
}

// Regra: o próximo vencimento só existe no contrato mensal (MONTHLY) em aberto, com a 1ª parcela e as pagas: é a 1ª
// mais as pagas e as em atraso; depois do vencimento final, ou mais de um ano atrás, não há previsão.
func TestProximoVencimento(t *testing.T) {
	t.Parallel()
	hoje := testeapoio.Dia(2026, 10, 14)
	primeira := testeapoio.Dia(2026, 1, 31)
	base := func(ajuste func(*EmprestimoDto)) EmprestimoDto {
		e := EmprestimoDto{SaldoDevedor: dec("100.0"), ParcelasPagas: ptr(int64(8)), Periodicidade: ptr("monthly")}
		if ajuste != nil {
			ajuste(&e)
		}
		return e
	}
	casos := []struct {
		nome     string
		e        EmprestimoDto
		primeira *tipos.Data
		quer     string
	}{
		{"8 pagas, fim de mês", base(nil), &primeira, "2026-09-30"},
		{"com atraso", base(func(e *EmprestimoDto) { e.ParcelasEmAtraso = 2 }), &primeira, "2026-11-30"},
		{"quitado", base(func(e *EmprestimoDto) { e.SaldoDevedor = dec("0.0") }), &primeira, ""},
		{"sem a 1ª parcela", base(nil), nil, ""},
		{"sem as pagas", base(func(e *EmprestimoDto) { e.ParcelasPagas = nil }), &primeira, ""},
		{"não é mensal", base(func(e *EmprestimoDto) { e.Periodicidade = ptr("MENSAL") }), &primeira, ""},
		{"sem periodicidade", base(func(e *EmprestimoDto) { e.Periodicidade = nil }), &primeira, ""},
		{"depois do vencimento final", base(func(e *EmprestimoDto) { e.DataVencimentoFinal = ptr(testeapoio.Dia(2026, 9, 1)) }),
			&primeira, ""},
		{"no vencimento final", base(func(e *EmprestimoDto) { e.DataVencimentoFinal = ptr(testeapoio.Dia(2026, 9, 30)) }),
			&primeira, "2026-09-30"},
		{"mais de um ano atrás", base(func(e *EmprestimoDto) { e.ParcelasPagas = ptr(int64(0)) }),
			ptr(testeapoio.Dia(2025, 10, 13)), ""},
		{"um ano atrás exato", base(func(e *EmprestimoDto) { e.ParcelasPagas = ptr(int64(0)) }),
			ptr(testeapoio.Dia(2025, 10, 14)), "2025-10-14"},
	}
	for _, c := range casos {
		obtido := ""
		if p := proximoVencimento(c.e, c.primeira, hoje); p != nil {
			obtido = p.String()
		}
		if obtido != c.quer {
			t.Errorf("%s: %q, quer %q", c.nome, obtido, c.quer)
		}
	}
}

// Regra: o painel mostra os contratos das conexões ativas (em aberto antes dos quitados, pelo maior saldo), com o
// total pago e o último pagamento; os totais de cada natureza (abertos e quitados, saldo e contratado só dos abertos,
// a parcela mensal arredondada, o total pago de todos) e os pagamentos dos últimos 12 meses por natureza. O total pago
// sai no formato do EF Core ("0.0" sem pagamento) e o mês sem pagamento, 0.
func TestPainel(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, testeapoio.HojeEmBrasilia(2026, 10, 14))
	ativa := fab.ConexaoBancaria(func(c *testeapoio.ConexaoBancaria) { c.NomeInstituicao = ptr("Banco Fictício") })
	inativa := fab.ConexaoBancaria(func(c *testeapoio.ConexaoBancaria) { c.Ativa = false })
	carro := fab.Emprestimo(ativa, func(e *testeapoio.Emprestimo) {
		e.Natureza, e.SaldoDevedor, e.ValorContratado = NaturezaFinanciamento, dec("1000.0"), dec("3000.0")
		e.ValorParcela, e.Periodicidade = ptr(dec("1200.0")), ptr("YEARLY")
		e.ParcelasEmAtraso = ptr(int64(1))
	})
	casa := fab.Emprestimo(ativa, func(e *testeapoio.Emprestimo) {
		e.Natureza, e.SaldoDevedor, e.ValorContratado = NaturezaFinanciamento, dec("5000.0"), dec("9000.0")
		e.ValorParcela = ptr(dec("100.005"))
	})
	quitado := fab.Emprestimo(ativa, func(e *testeapoio.Emprestimo) { e.SaldoDevedor = dec("0.0") })
	fab.Emprestimo(inativa)
	pagar := func(e testeapoio.Emprestimo, data, valor string) {
		t.Helper()
		if _, err := db.Exec(`INSERT INTO "PagamentoEmprestimo" ("EmprestimoId", "Data", "Valor", "Extraordinario")
			VALUES (?, ?, ?, 0)`, e.Id, data, valor); err != nil {
			t.Fatal(err)
		}
	}
	pagar(carro, "2025-10-31", "50.25") // antes dos 12 meses: só no total pago
	pagar(carro, "2026-03-10", "100.25")
	pagar(carro, "2026-03-20", "100.75")
	pagar(quitado, "2026-10-14", "10.0")
	pagar(quitado, "2026-10-15", "99.0") // depois de hoje: fora dos meses

	p, err := NovoServico(db).Painel(t.Context(), testeapoio.Dia(2026, 10, 14))
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for _, c := range p.Contratos {
		ids = append(ids, c.Id)
	}
	if len(ids) != 3 || ids[0] != casa.Id || ids[1] != carro.Id || ids[2] != quitado.Id {
		t.Fatalf("ordem: %v (casa %d, carro %d, quitado %d)", ids, casa.Id, carro.Id, quitado.Id)
	}
	c := p.Contratos[1]
	if c.TotalPago.String() != "251.25" || c.UltimoPagamento == nil || c.UltimoPagamento.String() != "2026-03-20" ||
		c.ParcelaMensal.String() != "100.0" || c.ParcelasEmAtraso != 1 || c.Quitado {
		t.Errorf("carro: total %s, último %v, parcela %s", c.TotalPago, c.UltimoPagamento, c.ParcelaMensal)
	}
	if casaDto := p.Contratos[0]; casaDto.TotalPago.String() != "0.0" || casaDto.UltimoPagamento != nil {
		t.Errorf("casa sem pagamento: %s %v", casaDto.TotalPago, casaDto.UltimoPagamento)
	}
	if !p.Contratos[2].Quitado || p.Contratos[2].ModalidadeDescricao != "Empréstimo" {
		t.Errorf("quitado: %+v", p.Contratos[2])
	}
	f := p.Financiamentos
	if f.ContratosAbertos != 2 || f.ContratosQuitados != 0 || f.SaldoDevedor.String() != "6000.0" ||
		f.ValorContratado.String() != "12000.0" || f.ParcelaMensal.String() != "200.00" || f.TotalPago.String() != "251.25" ||
		f.ParcelasEmAtraso != 1 {
		t.Errorf("financiamentos: %+v", f)
	}
	e := p.Emprestimos
	if e.ContratosAbertos != 0 || e.ContratosQuitados != 1 || e.SaldoDevedor.String() != "0" || e.TotalPago.String() != "109.0" {
		t.Errorf("empréstimos: %+v", e)
	}
	var meses []string
	for _, m := range p.PagamentosPorMes {
		meses = append(meses, m.Financiamentos.String()+"/"+m.Emprestimos.String())
	}
	if len(meses) != 12 || p.PagamentosPorMes[0].Mes != 11 || p.PagamentosPorMes[0].Ano != 2025 ||
		meses[4] != "201.0/0" || meses[11] != "0/10.0" || meses[0] != "0/0" {
		t.Errorf("meses: %s", strings.Join(meses, " "))
	}
}

// Regra: erro do banco sobe com o contexto.
func TestPainelComErroDoBanco(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	if _, err := NovoServico(bancoQueFalha{db}).Painel(t.Context(), testeapoio.Dia(2026, 10, 14)); err == nil {
		t.Error("o erro do banco sumiu")
	}
}

// bancoQueFalhaEm é o banco de verdade, mas a consulta que tem o trecho dado devolve erro.
type bancoQueFalhaEm struct {
	consultas.DBTX
	trecho string
}

func (b bancoQueFalhaEm) QueryContext(ctx context.Context, consulta string, argumentos ...any) (*sql.Rows, error) {
	if strings.Contains(consulta, b.trecho) {
		return nil, errDeTeste
	}
	return b.DBTX.QueryContext(ctx, consulta, argumentos...)
}

// Regra: o erro de cada leitura do painel sobe (os contratos, os pagamentos de cada um e os pagamentos por mês), e o
// quitado gravado antes do aberto vai para o fim.
func TestPainelErrosEOrdem(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, testeapoio.HojeEmBrasilia(2026, 10, 14))
	conexao := fab.ConexaoBancaria()
	quitado := fab.Emprestimo(conexao, func(e *testeapoio.Emprestimo) { e.SaldoDevedor = dec("0.0") })
	aberto := fab.Emprestimo(conexao)
	hoje := testeapoio.Dia(2026, 10, 14)
	p, err := NovoServico(db).Painel(t.Context(), hoje)
	if err != nil || p.Contratos[0].Id != aberto.Id || p.Contratos[1].Id != quitado.Id {
		t.Errorf("ordem: %+v %v", p.Contratos, err)
	}
	for _, trecho := range []string{`ORDER BY e."Id"`, `FROM "PagamentoEmprestimo" ORDER BY`, `JOIN "Emprestimo" e ON`} {
		if _, err := NovoServico(bancoQueFalhaEm{db, trecho}).Painel(t.Context(), hoje); !errors.Is(err, errDeTeste) {
			t.Errorf("%s: %v", trecho, err)
		}
	}
}
