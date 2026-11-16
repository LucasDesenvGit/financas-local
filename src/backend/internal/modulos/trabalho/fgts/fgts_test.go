package fgts

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/decimoterceiro"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/emprego"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/ferias"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/holerite"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

var v = testeapoio.Valor

func motivo(m int64) *int64 { return &m }

func TestEstimar(t *testing.T) {
	t.Parallel()
	jan, abr := testeapoio.Dia(2026, 1, 1), testeapoio.Dia(2026, 4, 1)
	dep := map[string]decimal.Decimal{}
	for m := jan; !m.Depois(abr); m = m.SomarMeses(1) {
		dep[m.String()] = decimal.NewFromInt(480)
	}
	casos := []struct {
		nome        string
		lancamentos []Lancamento
		saldos      string // o saldo de cada mês
		conferencia string // informado, estimado, diferença
	}{
		// jan 480; fev 480 x 0,0024663 = 1,18 + 480 = 961,18; mar 2,37 → 1443,55; abr 3,56 → 1927,11.
		{"só os depósitos", nil, "480.00 961.18 1443.55 1927.11", ""},
		// O app disse 1500 em 20/03: ele já tem o depósito de fevereiro, mas não o de março (pago até 20/04). O site
		// estimava 1443,55 - 480 = 963,55 sem ele; março fica 1500 + 480 = 1980, e abril 1980 + 4,88 + 480 = 2464,88
		// (revisão do #702: antes, o depósito de março sumia).
		{"conferido em março", []Lancamento{{Data: testeapoio.Dia(2026, 3, 20), Tipo: TipoConferencia, Valor: v("1500.00")}},
			"480.00 961.18 1980.00 2464.88", "1500.00 963.55 536.45"},
		// No dia 19, nem o de fevereiro o app mostra: o site estimava 1443,55 - 960 = 483,55; março fica 1500 + 960.
		{"conferido antes do dia 20", []Lancamento{{Data: testeapoio.Dia(2026, 3, 19), Tipo: TipoConferencia, Valor: v("1500.00")}},
			"480.00 961.18 2460.00 2946.07", "1500.00 483.55 1016.45"},
		// Em janeiro, antes do dia 20, só o depósito do próprio mês falta (não há mês anterior).
		{"conferido no primeiro mês", []Lancamento{{Data: testeapoio.Dia(2026, 1, 10), Tipo: TipoConferencia, Valor: v("100.00")}},
			"580.00 1061.43 1544.05 2027.86", "100.00 0.00 100.00"},
		// Saque de 1000 e distribuição de 50 em abril.
		{"saque e distribuição", []Lancamento{
			{Data: testeapoio.Dia(2026, 4, 2), Tipo: TipoSaque, Valor: v("1000.00"), Motivo: motivo(MotivoSaqueAniversario)},
			{Data: testeapoio.Dia(2026, 4, 30), Tipo: TipoDistribuicao, Valor: v("50.00")}},
			"480.00 961.18 1443.55 977.11", ""},
		// Regressão: o saque depois da conferência, no mesmo mês, entra depois dela (antes, sumia). Abril: 1443,55 + 3,56 +
		// 480 - 100 (o saque do dia 5) = 1827,11; no dia 10 o app ainda não tem os depósitos de março e abril (960): o site
		// estimava 867,11; o app disse 2000; com os 960 e menos o saque do dia 20: 2460.
		{"saques antes e depois da conferência", []Lancamento{
			{Data: testeapoio.Dia(2026, 4, 5), Tipo: TipoSaque, Valor: v("100.00")},
			{Data: testeapoio.Dia(2026, 4, 10), Tipo: TipoConferencia, Valor: v("2000.00")},
			{Data: testeapoio.Dia(2026, 4, 20), Tipo: TipoSaque, Valor: v("500.00")}},
			"480.00 961.18 1443.55 2460.00", "2000.00 867.11 1132.89"},
		// Lançamento fora do intervalo não conta.
		{"fora do intervalo", []Lancamento{{Data: testeapoio.Dia(2026, 5, 1), Tipo: TipoSaque, Valor: v("100.00")},
			{Data: testeapoio.Dia(2025, 12, 31), Tipo: TipoSaque, Valor: v("100.00")}}, "480.00 961.18 1443.55 1927.11", ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			r := Estimar(jan, abr, dep, c.lancamentos)
			got := ""
			for i, m := range r.Meses {
				if i > 0 {
					got += " "
				}
				got += m.Saldo.String()
			}
			if got != c.saldos || r.Saldo.String() != r.Meses[3].Saldo.String() {
				t.Fatalf("saldos = %s; quer %s", got, c.saldos)
			}
			conf := ""
			if len(r.Conferencias) > 0 {
				k := r.Conferencias[0]
				conf = k.Informado.String() + " " + k.Estimado.String() + " " + k.Diferenca.String()
			}
			if conf != c.conferencia {
				t.Fatalf("conferência = %q; quer %q", conf, c.conferencia)
			}
		})
	}
	r := Estimar(jan, abr, dep, []Lancamento{{Data: testeapoio.Dia(2026, 4, 2), Tipo: TipoSaque, Valor: v("1000.00")},
		{Data: testeapoio.Dia(2026, 4, 3), Tipo: TipoDistribuicao, Valor: v("50.00")}})
	if m := r.Meses[3]; m.Saques.String() != "1000.00" || m.Distribuicao.String() != "50.00" || m.Correcao.String() != "3.56" ||
		m.Deposito.String() != "480.00" || m.Conferido {
		t.Fatalf("abril = %+v", m)
	}
}

type mux struct{ *http.ServeMux }

func (m mux) HandleFunc(p string, h http.HandlerFunc) { m.ServeMux.HandleFunc(p, h) }

func TestFgtsPelaApi(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	rel := testeapoio.HojeEmBrasilia(2026, 11, 13)
	log := slog.New(slog.DiscardHandler)
	empregos := emprego.NovoServico(db, rel, log)
	holerites := holerite.NovoServico(db, empregos, rel, log)
	decimo := decimoterceiro.NovoServico(db, empregos, holerites, rel, log)
	asFerias := ferias.NovoServico(db, empregos, holerites, rel, log)
	s := NovoServico(db, empregos, holerites, decimo, asFerias, rel, log)
	m := mux{http.NewServeMux()}
	emprego.Registrar(m, empregos)
	holerite.Registrar(m, holerites)
	ferias.Registrar(m, asFerias)
	Registrar(m, s)
	chamar := func(metodo, caminho, corpo string) testeapoio.Resposta {
		return testeapoio.Chamar(t, m, metodo, caminho, corpo)
	}
	criar := func(corpo string) int64 {
		var e emprego.Emprego
		_ = json.Unmarshal(chamar("POST", "/api/trabalho/empregos", corpo).Corpo, &e)
		return e.Id
	}
	ler := func(r testeapoio.Resposta) Resposta {
		t.Helper()
		r.ConferirStatus(t, http.StatusOK)
		var c Resposta
		if err := json.Unmarshal(r.Corpo, &c); err != nil {
			t.Fatalf("%s: %v", r.Corpo, err)
		}
		return c
	}

	// Admissão em 10/01/2026 com 3000: janeiro tem 21 dias (2100 x 8% = 168), os outros meses 240.
	id := criar(`{"empregador":"E","admissao":"2026-01-10","salarioBruto":3000}`)
	base := fmt.Sprintf("/api/trabalho/empregos/%d/fgts", id)
	c := ler(chamar("GET", base, ""))
	// jan 168; fev 408,41; mar 649,42; abr 891,02; mai 1133,22; jun 1376,01; jul 1619,40; ago 1863,39; set 2107,99;
	// out 2353,19 (cada mês: o saldo de antes x 0,0024663, arredondado, mais 240); nov 2598,99 + 120 = 2718,99: 8% da 1ª
	// parcela do 13º (1500), paga até 30/11 (revisão do #702: antes, o 13º inteiro entrava em dezembro).
	if len(c.Meses) != 11 || c.Meses[0].Deposito.String() != "168.00" || c.Meses[1].Deposito.String() != "240.00" ||
		c.Meses[10].Deposito.String() != "360.00" || c.Saldo.String() != "2718.99" || c.Ate.String() != "2026-11-01" {
		t.Fatalf("FGTS = saldo %s em %d meses (%+v)", c.Saldo, len(c.Meses), c.Meses[0])
	}
	// Em dezembro entra o resto do 13º (3000 - 1500 = 1500 x 8% = 120).
	dez, err := s.Obter(t.Context(), id, testeapoio.Dia(2026, 12, 15))
	if err != nil || dez.Meses[11].Deposito.String() != "360.00" {
		t.Fatalf("dezembro = %+v (%v)", dez.Meses[11], err)
	}
	// O holerite do mês vale no lugar do salário: outubro com 10 h extras a 50% (3000/220 x 1,5 x 10 = 204,55) e o DSR
	// (204,55 / 27 x 4 = 30,30): base 3234,85 x 8% = 258,79.
	var h holerite.Holerite
	_ = json.Unmarshal(chamar("POST", fmt.Sprintf("/api/trabalho/empregos/%d/holerites", id), `{"mes":"2026-10"}`).Corpo, &h)
	chamar("PUT", fmt.Sprintf("/api/trabalho/holerites/%d", h.Id), `{"diasUteis":27,"diasDescanso":4,"lancamentos":[{"tipo":1,"quantidade":10,"percentual":50}]}`).
		ConferirStatus(t, http.StatusOK)
	c = ler(chamar("GET", base, ""))
	if c.Meses[9].Deposito.String() != "258.79" {
		t.Fatalf("outubro com holerite = %+v", c.Meses[9])
	}

	// Lançar: a conferência recomeça a estimativa, com os depósitos que o app ainda não tinha no dia 10 (os de outubro,
	// 258,79, e novembro, 360,78: com as horas extras de outubro, a 1ª parcela do 13º sobe para 1509,75): 3619,57; o
	// saque precisa do motivo.
	c = ler(chamar("POST", base+"/lancamentos", `{"data":"2026-11-10","tipo":1,"valor":3000}`))
	if c.Saldo.String() != "3619.57" || len(c.Conferencias) != 1 || len(c.Lancamentos) != 1 || c.Lancamentos[0].Motivo != nil {
		t.Fatalf("conferido = %+v", c)
	}
	c = ler(chamar("POST", base+"/lancamentos", `{"data":"2026-11-11","tipo":2,"valor":500,"motivo":2}`))
	if c.Saldo.String() != "3119.57" {
		t.Fatalf("depois do saque = %s", c.Saldo)
	}
	// Distribuição sem motivo (o motivo que vier é ignorado).
	c = ler(chamar("POST", base+"/lancamentos", `{"data":"2026-11-12","tipo":3,"valor":10,"motivo":4}`))
	if c.Lancamentos[2].Motivo != nil {
		t.Fatalf("distribuição com motivo: %+v", c.Lancamentos[2])
	}
	c = ler(chamar("DELETE", fmt.Sprintf("%s/lancamentos/%d", base, c.Lancamentos[1].Id), ""))
	if len(c.Lancamentos) != 2 {
		t.Fatalf("depois de excluir = %+v", c.Lancamentos)
	}
	// O saldo conferido pode ser zero.
	ler(chamar("POST", base+"/lancamentos", `{"data":"2026-01-10","tipo":1,"valor":0}`))

	casos := []struct {
		nome, corpo, mensagem string
	}{
		{"tipo desconhecido", `{"data":"2026-11-10","tipo":4,"valor":1}`, "Escolha o tipo: saldo conferido, saque ou distribuição de resultados."},
		{"tipo zero", `{"data":"2026-11-10","tipo":0,"valor":1}`, "Escolha o tipo: saldo conferido, saque ou distribuição de resultados."},
		{"antes da admissão", `{"data":"2026-01-09","tipo":1,"valor":1}`, "A data vai da admissão até hoje."},
		{"no futuro", `{"data":"2026-11-14","tipo":1,"valor":1}`, "A data vai da admissão até hoje."},
		{"saque zerado", `{"data":"2026-11-10","tipo":2,"valor":0,"motivo":1}`, "O valor vai de R$ 0,01 a R$ 100.000.000,00 (o saldo conferido pode ser zero)."},
		{"negativo", `{"data":"2026-11-10","tipo":1,"valor":-1}`, "O valor vai de R$ 0,01 a R$ 100.000.000,00 (o saldo conferido pode ser zero)."},
		{"absurdo", `{"data":"2026-11-10","tipo":3,"valor":100000000.01}`, "O valor vai de R$ 0,01 a R$ 100.000.000,00 (o saldo conferido pode ser zero)."},
		{"saque sem motivo", `{"data":"2026-11-10","tipo":2,"valor":1}`, "Escolha o motivo do saque."},
		{"motivo desconhecido", `{"data":"2026-11-10","tipo":2,"valor":1,"motivo":5}`, "Escolha o motivo do saque."},
		{"motivo zero", `{"data":"2026-11-10","tipo":2,"valor":1,"motivo":0}`, "Escolha o motivo do saque."},
		{"sem data", `{"tipo":1,"valor":1}`, "Informe a data e o valor."},
		{"JSON quebrado", `{"tipo":`, MensagemJSONInvalido},
	}
	for _, k := range casos {
		t.Run(k.nome, func(t *testing.T) {
			r := chamar("POST", base+"/lancamentos", k.corpo)
			r.ConferirStatus(t, http.StatusBadRequest)
			r.ConferirJSON(t, fmt.Sprintf(`{"mensagem":%q}`, k.mensagem))
		})
	}
	// Os limites valem: hoje, a admissão, o motivo 4 e o valor máximo.
	ler(chamar("POST", base+"/lancamentos", `{"data":"2026-11-13","tipo":2,"valor":100000000,"motivo":4}`))
	ler(chamar("POST", base+"/lancamentos", `{"data":"2026-11-13","tipo":2,"valor":1,"motivo":1}`))

	// Férias: 8% das férias e do 1/3 entram no mês do início. Emprego de 2024: o 1º período termina em 10/03/2025.
	antigo := criar(`{"empregador":"A","admissao":"2024-03-11","saida":"2025-06-30","salarioBruto":3000}`)
	chamar("POST", fmt.Sprintf("/api/trabalho/empregos/%d/ferias", antigo), `{"periodoInicio":"2024-03-11","inicio":"2025-05-05","dias":30}`).
		ConferirStatus(t, http.StatusOK)
	c = ler(chamar("GET", fmt.Sprintf("/api/trabalho/empregos/%d/fgts", antigo), ""))
	mes := func(c Resposta, m string) Mes {
		t.Helper()
		for _, x := range c.Meses {
			if x.Mes.String() == m {
				return x
			}
		}
		t.Fatalf("sem o mês %s", m)
		return Mes{}
	}
	// Maio: 240 do salário + 8% de 4000 (as férias, 3000 + 1000), menos o salário dos 27 dias de férias de maio (2700 x
	// 8% = 216): 344. Junho: 240 menos os 3 dias de férias (24): 216 (revisão do #702: antes, os dias de férias
	// contavam duas vezes).
	if mes(c, "2025-05-01").Deposito.String() != "344.00" || mes(c, "2025-06-01").Deposito.String() != "216.00" {
		t.Fatalf("com férias = maio %+v, junho %+v", mes(c, "2025-05-01"), mes(c, "2025-06-01"))
	}
	// Depois da saída (junho de 2025) não há depósito, mas o saldo segue com a correção até hoje, e o saque da rescisão,
	// depois da saída, conta (revisão do #702). O 13º de 2025 vem na rescisão, e não em dezembro.
	julho := mes(c, "2025-07-01")
	if c.Ate.String() != "2026-11-01" || !julho.Deposito.Decimal().IsZero() || !julho.Correcao.Decimal().IsPositive() ||
		!mes(c, "2025-12-01").Deposito.Decimal().IsZero() {
		t.Fatalf("depois da saída = até %s, julho %+v", c.Ate, julho)
	}
	c = ler(chamar("POST", fmt.Sprintf("/api/trabalho/empregos/%d/fgts/lancamentos", antigo), `{"data":"2025-07-15","tipo":2,"valor":1000,"motivo":1}`))
	if mes(c, "2025-07-01").Saques.String() != "1000.00" || !mes(c, "2025-07-01").Saldo.Decimal().LessThan(julho.Saldo.Decimal()) {
		t.Fatalf("saque depois da saída = %+v", mes(c, "2025-07-01"))
	}

	// Saiu em 20/12/2026: dezembro tem 20 dias de salário (2000 x 8% = 160) e o 13º fica para a rescisão.
	dezembro := criar(`{"empregador":"D","admissao":"2026-01-10","saida":"2026-12-20","salarioBruto":3000}`)
	if c, err := s.Obter(t.Context(), dezembro, testeapoio.Dia(2026, 12, 1)); err != nil || c.Meses[11].Deposito.String() != "160.00" {
		t.Fatalf("saída em dezembro = %+v (%v)", c.Meses[11], err)
	}

	// O ano sem o 13º (dezembro de 2024 não tem tabela do IRRF) não tira o 13º do ano seguinte: em dezembro de 2025, 240
	// do salário + 8% da 2ª metade do 13º (1500 x 8% = 120).
	desde2024 := criar(`{"empregador":"Q","admissao":"2024-06-03","salarioBruto":3000}`)
	if c, err := s.Obter(t.Context(), desde2024, testeapoio.Dia(2025, 12, 1)); err != nil || mes(c, "2024-12-01").Deposito.String() != "240.00" ||
		mes(c, "2025-12-01").Deposito.String() != "360.00" {
		t.Fatalf("13º de 2025 depois de 2024 sem tabela = %+v / %+v (%v)", mes(c, "2024-12-01"), mes(c, "2025-12-01"), err)
	}

	pj := criar(`{"empregador":"P","admissao":"2020-01-01","salarioBruto":9000,"regime":3}`)
	r := chamar("GET", fmt.Sprintf("/api/trabalho/empregos/%d/fgts", pj), "")
	r.ConferirStatus(t, http.StatusBadRequest)
	r.ConferirJSON(t, fmt.Sprintf(`{"mensagem":%q}`, MensagemSemFgts))
	chamar("DELETE", fmt.Sprintf("/api/trabalho/empregos/%d/fgts/lancamentos/1", pj), "").ConferirStatus(t, http.StatusBadRequest)
	chamar("POST", fmt.Sprintf("/api/trabalho/empregos/%d/fgts/lancamentos", pj), `{"data":"2026-11-10","tipo":1,"valor":1}`).ConferirStatus(t, http.StatusBadRequest)
	chamar("GET", "/api/trabalho/empregos/99/fgts", "").ConferirStatus(t, http.StatusNotFound)
	chamar("DELETE", base+"/lancamentos/999", "").ConferirStatus(t, http.StatusNotFound)
	_ = tipos.Data{}
}
