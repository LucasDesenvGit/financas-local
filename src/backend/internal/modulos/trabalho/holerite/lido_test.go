package holerite

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

func lido(mes string) Calculo {
	d, _ := tipos.LerData(mes)
	v := tipos.DeDecimal(decimal.RequireFromString("4000"))
	return Calculo{Mes: d, SalarioContratual: v, DiasTrabalhados: 30, Proventos: []Linha{{Tipo: TipoSalario, Descricao: "SALARIO", Valor: v},
		{Tipo: HoraExtra, Descricao: "HE 50%", Valor: tipos.DeDecimal(decimal.RequireFromString("100"))}},
		Descontos: []Linha{}, TotalProventos: tipos.DeDecimal(decimal.RequireFromString("4100")), Liquido: tipos.DeDecimal(decimal.RequireFromString("4100")),
		FgtsDoMes: tipos.DeDecimal(decimal.RequireFromString("328"))}
}

// O holerite lido do PDF (#228): novo, fechado com o cálculo do PDF; no mês com a estimativa aberta, toma o lugar
// dela e guarda os lançamentos; no mês fechado, recusa.
func TestGravarEDesfazerOLido(t *testing.T) {
	t.Parallel()
	a := novo(t)
	ctx := context.Background()
	id := a.empregoDe5500()
	h, existia, err := a.s.GravarLido(ctx, id, lido("2026-09-17"))
	if err != nil || existia || h.Situacao != Fechado || h.Origem != LidoDoPdf || h.Mes.String() != "2026-09-01" || h.FechadoEm == nil ||
		h.Calculo == nil || h.Calculo.Liquido.Decimal().String() != "4100" || h.Calculo.Mes.String() != "2026-09-01" ||
		h.DiasUteis != 26 || h.DiasDescanso != 4 {
		t.Fatalf("novo: %+v %t %v", h, existia, err)
	}
	// As horas extras lidas entram nas variáveis do ano, como as digitadas.
	if soma, meses, _ := a.s.VariaveisDoAno(ctx, id, 2026); soma.Decimal().String() != "100" || meses != 1 {
		t.Errorf("variáveis: %s %d", soma.Decimal().String(), meses)
	}
	if _, _, err := a.s.GravarLido(ctx, id, lido("2026-09-01")); !errors.Is(err, ErrFechado) {
		t.Errorf("mês fechado: %v", err)
	}
	if err := a.s.DesfazerLido(ctx, h.Id, false); err != nil {
		t.Fatal(err)
	}
	if _, err := a.s.Obter(ctx, h.Id); !errors.Is(err, ErrNaoEncontrado) {
		t.Errorf("desfeito, o novo some: %v", err)
	}
	if err := a.s.DesfazerLido(ctx, h.Id, false); !errors.Is(err, ErrNaoELido) {
		t.Errorf("desfazer de novo: %v", err)
	}

	aberto := a.criar(id, "2026-10")
	a.chamar("PUT", fmt.Sprintf("/api/trabalho/holerites/%d", aberto.Id), lancamentosDeOutubro).ConferirStatus(t, http.StatusOK)
	h, existia, err = a.s.GravarLido(ctx, id, lido("2026-10-01"))
	if err != nil || !existia || h.Id != aberto.Id || h.Origem != LidoDoPdf || h.Situacao != Fechado || len(h.Lancamentos) != 6 ||
		h.Calculo.TotalProventos.Decimal().String() != "4100" {
		t.Fatalf("no lugar do aberto: %+v %t %v", h, existia, err)
	}
	lista, _ := a.s.Listar(ctx, id)
	if len(lista) != 1 || lista[0].Origem != LidoDoPdf || lista[0].Liquido.Decimal().String() != "4100" {
		t.Errorf("lista: %+v", lista)
	}
	if err := a.s.DesfazerLido(ctx, h.Id, true); err != nil {
		t.Fatal(err)
	}
	h, _ = a.s.Obter(ctx, aberto.Id)
	if h.Situacao != Aberto || h.Origem != Calculado || h.FechadoEm != nil || len(h.Lancamentos) != 6 ||
		h.Calculo.TotalProventos.Decimal().String() == "4100" {
		t.Errorf("de volta à estimativa: %+v", h)
	}
	// O aberto (não é o lido) não se desfaz.
	if err := a.s.DesfazerLido(ctx, aberto.Id, true); !errors.Is(err, ErrNaoELido) {
		t.Errorf("desfazer o aberto: %v", err)
	}
	// O calculado e fechado também não.
	a.chamar("POST", fmt.Sprintf("/api/trabalho/holerites/%d/fechar", aberto.Id), "").ConferirStatus(t, http.StatusOK)
	if err := a.s.DesfazerLido(ctx, aberto.Id, true); !errors.Is(err, ErrNaoELido) {
		t.Errorf("desfazer o fechado calculado: %v", err)
	}
}

// Reabrir o lido volta para o calculado, com os lançamentos de antes.
func TestReabrirOLidoVoltaParaOCalculado(t *testing.T) {
	t.Parallel()
	a := novo(t)
	ctx := context.Background()
	id := a.empregoDe5500()
	aberto := a.criar(id, "2026-10")
	h, _, err := a.s.GravarLido(ctx, id, lido("2026-10-01"))
	if err != nil {
		t.Fatal(err)
	}
	a.chamar("POST", fmt.Sprintf("/api/trabalho/holerites/%d/reabrir", h.Id), "").ConferirStatus(t, http.StatusOK)
	h, _ = a.s.Obter(ctx, aberto.Id)
	if h.Origem != Calculado || h.Situacao != Aberto || h.Calculo == nil || h.Calculo.TotalProventos.Decimal().String() != "5500" {
		t.Errorf("reaberto: %+v", h)
	}
}

func TestGravarLidoRecusa(t *testing.T) {
	t.Parallel()
	a := novo(t)
	ctx := context.Background()
	id := a.empregoDe5500()
	if _, _, err := a.s.GravarLido(ctx, 999, lido("2026-10-01")); !errors.Is(err, ErrNaoEncontrado) {
		t.Errorf("emprego que não existe: %v", err)
	}
	_, _, err := a.s.GravarLido(ctx, id, lido("2024-02-01"))
	if !errors.Is(err, ErrInvalido) || err.Error() != "O mês do holerite é de antes da admissão ou depois da saída deste emprego." {
		t.Errorf("antes da admissão: %v", err)
	}
	// O mês da admissão (11/03) vale, mesmo proporcional.
	if _, _, err := a.s.GravarLido(ctx, id, lido("2024-03-01")); err != nil {
		t.Errorf("mês da admissão: %v", err)
	}
}

// A estimativa do mês, para o lido × calculado: com os lançamentos do holerite do mês ou sem lançamentos; nil sem
// salário ou sem tabela.
func TestEstimativa(t *testing.T) {
	t.Parallel()
	a := novo(t)
	ctx := context.Background()
	id := a.empregoDe5500()
	sem, err := a.s.Estimativa(ctx, id, tipos.NovaData(2026, 10, 20))
	if err != nil || sem == nil || sem.TotalProventos.Decimal().String() != "5500" || sem.Mes.String() != "2026-10-01" {
		t.Fatalf("sem holerite: %+v %v", sem, err)
	}
	aberto := a.criar(id, "2026-10")
	a.chamar("PUT", fmt.Sprintf("/api/trabalho/holerites/%d", aberto.Id), lancamentosDeOutubro).ConferirStatus(t, http.StatusOK)
	h, _ := a.s.Obter(ctx, aberto.Id)
	com, err := a.s.Estimativa(ctx, id, tipos.NovaData(2026, 10, 1))
	if err != nil || com == nil || !com.Liquido.Decimal().Equal(h.Calculo.Liquido.Decimal()) || len(com.Proventos) != len(h.Calculo.Proventos) {
		t.Errorf("com o aberto: %+v %v", com, err)
	}
	// O lido no mês: a estimativa é a dos lançamentos guardados, não o PDF.
	if _, _, err := a.s.GravarLido(ctx, id, lido("2026-10-01")); err != nil {
		t.Fatal(err)
	}
	if lida, _ := a.s.Estimativa(ctx, id, tipos.NovaData(2026, 10, 1)); lida == nil || !lida.Liquido.Decimal().Equal(h.Calculo.Liquido.Decimal()) {
		t.Errorf("com o lido: %+v", lida)
	}
	if v, err := a.s.Estimativa(ctx, id, tipos.NovaData(2023, 1, 1)); v != nil || err != nil {
		t.Errorf("sem salário no mês: %+v %v", v, err)
	}
	if v, err := a.s.Estimativa(ctx, id, tipos.NovaData(2025, 3, 1)); v != nil || err != nil {
		t.Errorf("sem tabela (o IRRF começa em maio de 2025): %+v %v", v, err)
	}
	if _, err := a.s.Estimativa(ctx, 999, tipos.NovaData(2026, 10, 1)); !errors.Is(err, ErrNaoEncontrado) {
		t.Errorf("emprego que não existe: %v", err)
	}
}

func TestNomeDoMes(t *testing.T) {
	if got := NomeDoMes(tipos.NovaData(2026, 3, 1)); got != "março de 2026" {
		t.Errorf("NomeDoMes: %s", got)
	}
}
