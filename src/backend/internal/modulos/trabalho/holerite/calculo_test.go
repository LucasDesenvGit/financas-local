package holerite

import (
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/encargos"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

var v = testeapoio.Valor

func ptr(s string) *tipos.Decimal { d := v(s); return &d }

// tabelas2026 são as tabelas oficiais de 2026, lidas dos dados iniciais.
func tabelas2026(t *testing.T) (encargos.TabelaInss, encargos.TabelaIrrf) {
	t.Helper()
	db := testeapoio.NovoBanco(t)
	inss, err := encargos.LerTabelasInss(t.Context(), db)
	if err != nil {
		t.Fatal(err)
	}
	irrf, err := encargos.LerTabelasIrrf(t.Context(), db)
	if err != nil {
		t.Fatal(err)
	}
	ti, _ := encargos.Vigente(inss, testeapoio.Dia(2026, 1, 1))
	tr, _ := encargos.Vigente(irrf, testeapoio.Dia(2026, 1, 1))
	return ti, tr
}

func TestDiasTrabalhados(t *testing.T) {
	t.Parallel()
	out := testeapoio.Dia(2026, 10, 1)
	fev := testeapoio.Dia(2026, 2, 1)
	d := testeapoio.Dia
	casos := []struct {
		nome     string
		mes      tipos.Data
		admissao tipos.Data
		saida    *tipos.Data
		dias     int
	}{
		{"mês inteiro", out, d(2024, 3, 11), nil, 30},
		{"admissão no dia 16", out, d(2026, 10, 16), nil, 15},
		{"admissão no dia 1", out, d(2026, 10, 1), nil, 30},
		{"admissão no dia 31 conta um dia", out, d(2026, 10, 31), nil, 1},
		{"saída no dia 15", out, d(2024, 3, 11), &[]tipos.Data{d(2026, 10, 15)}[0], 15},
		{"saída no dia 31", out, d(2024, 3, 11), &[]tipos.Data{d(2026, 10, 31)}[0], 30},
		{"fevereiro inteiro, saída no último dia", fev, d(2024, 3, 11), &[]tipos.Data{d(2026, 2, 28)}[0], 30},
		{"entrou e saiu no mesmo mês", out, d(2026, 10, 5), &[]tipos.Data{d(2026, 10, 14)}[0], 10},
		{"admissão depois do mês", out, d(2026, 11, 1), nil, 0},
		{"saída antes do mês", out, d(2024, 3, 11), &[]tipos.Data{d(2026, 9, 30)}[0], 0},
		{"saída depois do mês", out, d(2024, 3, 11), &[]tipos.Data{d(2026, 11, 3)}[0], 30},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			if got := DiasTrabalhados(c.mes, c.admissao, c.saida); got != c.dias {
				t.Fatalf("dias = %d; quer %d", got, c.dias)
			}
		})
	}
}

func TestDiasDoDsr(t *testing.T) {
	t.Parallel()
	// Outubro de 2026: 31 dias, 4 domingos (4, 11, 18, 25). Fevereiro de 2026: 28 dias, 4 domingos (1, 8, 15, 22).
	for _, c := range []struct {
		mes             tipos.Data
		uteis, descanso int64
	}{{testeapoio.Dia(2026, 10, 1), 27, 4}, {testeapoio.Dia(2026, 2, 1), 24, 4}, {testeapoio.Dia(2026, 11, 1), 25, 5}} {
		u, d := DiasDoDsr(c.mes)
		if u != c.uteis || d != c.descanso {
			t.Errorf("%s: %d úteis e %d de descanso; quer %d e %d", c.mes, u, d, c.uteis, c.descanso)
		}
	}
}

// O holerite completo, conferido à mão (contas nos comentários).
func TestCalcularHoleriteCompleto(t *testing.T) {
	t.Parallel()
	ti, tr := tabelas2026(t)
	c := Calcular(Entrada{
		Mes: testeapoio.Dia(2026, 10, 1), Admissao: testeapoio.Dia(2024, 3, 11), SalarioContratual: v("5500.00"),
		JornadaHoras: 220, DiasUteis: 27, DiasDescanso: 4, Inss: ti, Irrf: tr,
		Lancamentos: []Lancamento{
			// valor da hora 5500 / 220 = 25; 10 h x 25 x 1,5 = 375,00
			{Tipo: HoraExtra, Quantidade: ptr("10"), Percentual: ptr("50")},
			// 20 h x 25 x 20% = 100,00
			{Tipo: AdicionalNoturno, Quantidade: ptr("20"), Percentual: ptr("20")},
			// PLR: fora do INSS e do IRRF do mês (tem tributação própria)
			{Tipo: OutroProvento, Descricao: "PLR", Valor: ptr("1000.00")},
			// custo de 400, acima dos 6% (330,00): desconta 330,00
			{Tipo: ValeTransporte, Valor: ptr("400.00")},
			{Tipo: PlanoDeSaude, Valor: ptr("250.00")},
			{Tipo: PensaoAlimenticia, Valor: ptr("500.00")},
		},
	})
	quer := map[string]string{
		// DSR: 475 / 27 x 4 = 70,37
		"proventos": "Salário 5500.00 | Horas extras 375.00 | Adicional noturno 100.00 | PLR 1000.00 | DSR sobre horas extras e adicional noturno 70.37",
		// INSS de 6045,37: 121,575 + 115,3656 + 174,1716 + 1691,10 x 14% (236,754) = 647,8662.
		// IRRF: 6045,37 - (647,87 + 500 de pensão) = 4897,50 x 27,5% - 908,73 = 438,08; redução 978,62 - 0,133145 x 6045,37
		// = 173,71; retido 264,37.
		"descontos":      "INSS 647.87 | IRRF 264.37 | Vale-transporte 330.00 | Plano de saúde 250.00 | Pensão alimentícia 500.00",
		"totalProventos": "7045.37", "baseInss": "6045.37", "totalDescontos": "1992.24", "liquido": "5053.13",
		"fgts": "483.63", "valorHora": "25.00",
	}
	got := map[string]string{
		"proventos": linhas(c.Proventos), "descontos": linhas(c.Descontos), "totalProventos": c.TotalProventos.String(),
		"baseInss": c.BaseInss.String(), "totalDescontos": c.TotalDescontos.String(), "liquido": c.Liquido.String(),
		"fgts": c.FgtsDoMes.String(), "valorHora": c.ValorHora.String(),
	}
	for k := range quer {
		if got[k] != quer[k] {
			t.Errorf("%s = %q\n                quer %q", k, got[k], quer[k])
		}
	}
	if c.Proventos[0].Referencia != "30 dias" || c.Proventos[1].Referencia != "10 h a 50%" || c.Descontos[2].Referencia != "6% do salário (o limite)" {
		t.Errorf("referências: %q, %q", c.Proventos[1].Referencia, c.Descontos[2].Referencia)
	}
	if c.Irrf.DeducoesLegais.String() != "1147.87" {
		t.Errorf("a pensão entra nas deduções do IRRF: %s", c.Irrf.DeducoesLegais)
	}
}

func linhas(ls []Linha) string {
	var partes []string
	for _, l := range ls {
		partes = append(partes, l.Descricao+" "+l.Valor.String())
	}
	return strings.Join(partes, " | ")
}

func TestCalcularCasosSimples(t *testing.T) {
	t.Parallel()
	ti, tr := tabelas2026(t)
	out := testeapoio.Dia(2026, 10, 1)
	casos := []struct {
		nome       string
		entrada    Entrada
		quer       string // proventos, descontos e líquido
		semDsr     bool
		referencia string // a do salário, quando o caso confere
	}{
		{"admissão no meio do mês: 15 dias; VT sem custo desconta os 6%",
			// 3000 x 15/30 = 1500; INSS 7,5% = 112,50; IRRF isento; VT 6% de 1500 = 90
			Entrada{Mes: out, Admissao: testeapoio.Dia(2026, 10, 16), SalarioContratual: v("3000.00"), JornadaHoras: 220,
				DiasUteis: 27, DiasDescanso: 4, Inss: ti, Irrf: tr, Lancamentos: []Lancamento{{Tipo: ValeTransporte}}},
			"1500.00 | 202.50 | 1297.50", true, "15 de 30 dias"},
		{"VT abaixo dos 6%: desconta o custo",
			Entrada{Mes: out, Admissao: testeapoio.Dia(2020, 1, 1), SalarioContratual: v("3000.00"), JornadaHoras: 220,
				DiasUteis: 27, DiasDescanso: 4, Inss: ti, Irrf: tr, Lancamentos: []Lancamento{{Tipo: ValeTransporte, Valor: ptr("100.00")}}},
			// INSS de 3000: 248,60; IRRF 0; VT 100
			"3000.00 | 348.60 | 2651.40", true, "30 dias"},
		{"bônus com INSS e IRRF",
			// base 6000: INSS 641,51, IRRF 385,10 (os casos do encargos)
			Entrada{Mes: out, Admissao: testeapoio.Dia(2020, 1, 1), SalarioContratual: v("5000.00"), JornadaHoras: 220,
				DiasUteis: 27, DiasDescanso: 4, Inss: ti, Irrf: tr,
				Lancamentos: []Lancamento{{Tipo: OutroProvento, Descricao: "Bônus", Valor: ptr("1000.00"), IncideInss: true, IncideIrrf: true}}},
			"6000.00 | 1026.61 | 4973.39", true, ""},
		{"outro desconto dedutível do IR",
			// 6000 - (641,51 + 1000) = 4358,49 x 22,5% - 675,49 = 305,17; - 179,75 = 125,42
			Entrada{Mes: out, Admissao: testeapoio.Dia(2020, 1, 1), SalarioContratual: v("6000.00"), JornadaHoras: 220,
				DiasUteis: 27, DiasDescanso: 4, Inss: ti, Irrf: tr,
				Lancamentos: []Lancamento{{Tipo: OutroDesconto, Descricao: "Previdência oficial", Valor: ptr("1000.00"), DeduzIrrf: true}}},
			"6000.00 | 1766.93 | 4233.07", true, ""},
		{"PJ: sem INSS, IRRF nem FGTS",
			Entrada{Mes: out, Admissao: testeapoio.Dia(2020, 1, 1), SalarioContratual: v("9000.00"), JornadaHoras: 220, SemEncargos: true,
				DiasUteis: 27, DiasDescanso: 4, Lancamentos: []Lancamento{{Tipo: OutroDesconto, Valor: ptr("100.00")}}},
			"9000.00 | 100.00 | 8900.00", true, ""},
		{"hora extra sem dias de descanso: sem DSR",
			// 2200 / 220 = 10 por hora; 10 h a 100% = 200; base 2400: INSS 1621 x 7,5% + 779 x 9% = 121,575 + 70,11 = 191,685
			Entrada{Mes: out, Admissao: testeapoio.Dia(2020, 1, 1), SalarioContratual: v("2200.00"), JornadaHoras: 220,
				DiasUteis: 27, DiasDescanso: 0, Inss: ti, Irrf: tr,
				Lancamentos: []Lancamento{{Tipo: HoraExtra, Quantidade: ptr("10"), Percentual: ptr("100")}}},
			"2400.00 | 191.69 | 2208.31", true, ""},
		{"adicional noturno sem percentual vale 20%",
			// 20 h x 10 x 20% = 40; DSR 40 / 27 x 4 = 5,93; base 2245,93: 121,575 + 624,93 x 9% (56,2437) = 177,8187
			Entrada{Mes: out, Admissao: testeapoio.Dia(2020, 1, 1), SalarioContratual: v("2200.00"), JornadaHoras: 220,
				DiasUteis: 27, DiasDescanso: 4, Inss: ti, Irrf: tr,
				Lancamentos: []Lancamento{{Tipo: AdicionalNoturno, Quantidade: ptr("20")}}},
			"2245.93 | 177.82 | 2068.11", false, ""},
		{"hora extra sem dias úteis: sem DSR (e sem dividir por zero)",
			Entrada{Mes: out, Admissao: testeapoio.Dia(2020, 1, 1), SalarioContratual: v("2200.00"), JornadaHoras: 220,
				DiasUteis: 0, DiasDescanso: 4, Inss: ti, Irrf: tr,
				Lancamentos: []Lancamento{{Tipo: HoraExtra, Quantidade: ptr("10"), Percentual: ptr("100")}}},
			"2400.00 | 191.69 | 2208.31", true, ""},
		{"sem jornada: a hora vale zero",
			Entrada{Mes: out, Admissao: testeapoio.Dia(2020, 1, 1), SalarioContratual: v("2000.00"), JornadaHoras: 0,
				DiasUteis: 0, DiasDescanso: 4, Inss: ti, Irrf: tr,
				Lancamentos: []Lancamento{{Tipo: HoraExtra, Quantidade: ptr("10"), Percentual: ptr("50")}}},
			// 2000: INSS 121,575 + 379 x 9% (34,11) = 155,685
			"2000.00 | 155.69 | 1844.31", true, ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			r := Calcular(c.entrada)
			got := r.TotalProventos.String() + " | " + r.TotalDescontos.String() + " | " + r.Liquido.String()
			if got != c.quer {
				t.Fatalf("holerite = %s; quer %s\n%s\n%s", got, c.quer, linhas(r.Proventos), linhas(r.Descontos))
			}
			if c.entrada.SemEncargos && (r.Inss != nil || r.FgtsDoMes.String() != "0.00") {
				t.Fatalf("PJ sem encargos: %+v", r)
			}
			// Sem horas extras, ou sem dias úteis ou de descanso, não há linha de DSR.
			if c.semDsr && strings.Contains(linhas(r.Proventos), "DSR") {
				t.Fatalf("DSR onde não devia: %s", linhas(r.Proventos))
			}
			if c.referencia != "" && r.Proventos[0].Referencia != c.referencia {
				t.Fatalf("referência do salário = %q; quer %q", r.Proventos[0].Referencia, c.referencia)
			}
		})
	}
}
