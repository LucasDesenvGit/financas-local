package encargos

import (
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

var v = testeapoio.Valor

// As tabelas oficiais, lidas do banco com os dados iniciais (o mesmo que o site usa).
func tabelas(t *testing.T) ([]TabelaInss, []TabelaIrrf) {
	t.Helper()
	db := testeapoio.NovoBanco(t)
	inss, err := LerTabelasInss(t.Context(), db)
	if err != nil {
		t.Fatal(err)
	}
	irrf, err := LerTabelasIrrf(t.Context(), db)
	if err != nil {
		t.Fatal(err)
	}
	return inss, irrf
}

func TestTabelasOficiaisDosDadosIniciais(t *testing.T) {
	t.Parallel()
	inss, irrf := tabelas(t)
	if len(inss) != 2 || inss[0].Inicio.String() != "2025-01-01" || inss[1].Inicio.String() != "2026-01-01" {
		t.Fatalf("vigências do INSS = %+v", inss)
	}
	if teto := inss[1].Faixas[3].Ate.String(); teto != "8475.55" {
		t.Fatalf("teto do INSS de 2026 = %s; a Portaria MPS/MF 13/2026 diz 8475.55", teto)
	}
	if len(irrf) != 2 || irrf[0].Reducao != nil || irrf[1].Reducao == nil {
		t.Fatalf("o IRRF de 2025 não tem a redução e o de 2026 tem: %+v", irrf)
	}
	if len(irrf[1].Faixas) != 5 || irrf[1].Faixas[4].Ate != nil {
		t.Fatalf("faixas do IRRF de 2026 = %+v", irrf[1].Faixas)
	}
}

// Os casos foram conferidos à mão, faixa por faixa (contas no comentário de cada um).
func TestInss(t *testing.T) {
	t.Parallel()
	inss, _ := tabelas(t)
	t2025, _ := Vigente(inss, testeapoio.Dia(2025, 6, 1))
	t2026, _ := Vigente(inss, testeapoio.Dia(2026, 11, 1))
	casos := []struct {
		nome   string
		tabela TabelaInss
		base   string
		valor  string
		noTeto bool
		faixas int
	}{
		// 1621 x 7,5% = 121,575
		{"salário mínimo de 2026", t2026, "1621.00", "121.58", false, 1},
		// 121,575 + 1281,84 x 9% (115,3656) + 97,16 x 12% (11,6592) = 248,5998
		{"três faixas", t2026, "3000.00", "248.60", false, 3},
		// 121,575 + 115,3656 + 174,1716 + 645,73 x 14% (90,4022) = 501,5144
		{"quatro faixas", t2026, "5000.00", "501.51", false, 4},
		// o desconto máximo de 2026: 121,575 + 115,3656 + 174,1716 + 4121,28 x 14% (576,9792) = 988,0914
		{"no teto", t2026, "8475.55", "988.09", false, 4},
		{"acima do teto", t2026, "15000.00", "988.09", true, 4},
		// 2025: 113,85 + 114,8292 + 167,634 + 809,17 x 14% (113,2838) = 509,597
		{"tabela de 2025", t2025, "5000.00", "509.60", false, 4},
		{"sem salário", t2026, "0", "0.00", false, 0},
		// um centavo de base: 0,01 x 7,5% = 0,00075
		{"um centavo", t2026, "0.01", "0.00", false, 1},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			r := CalcularInss(c.tabela, v(c.base))
			if r.Valor.String() != c.valor || r.NoTeto != c.noTeto || len(r.Parcelas) != c.faixas {
				t.Fatalf("INSS de %s = %s (teto %v, %d faixas); quer %s (teto %v, %d faixas)", c.base, r.Valor, r.NoTeto,
					len(r.Parcelas), c.valor, c.noTeto, c.faixas)
			}
		})
	}
}

func TestInssComTabelaVazia(t *testing.T) {
	t.Parallel()
	r := CalcularInss(TabelaInss{}, v("3000.00"))
	if r.Valor.String() != "0.00" || r.NoTeto || len(r.Parcelas) != 0 {
		t.Fatalf("INSS sem faixas = %+v", r)
	}
}

func TestReducaoIncompletaNaoVale(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBanco(t)
	// Cada uma das cinco colunas da redução faltando: a vigência fica sem redução.
	for _, coluna := range []string{"ReducaoIsencaoAte", "ReducaoMaxima", "ReducaoAte", "ReducaoConstante", "ReducaoFator"} {
		t.Run(coluna, func(t *testing.T) {
			tx, err := db.BeginTx(t.Context(), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			if _, err := tx.ExecContext(t.Context(), `UPDATE "IrrfVigencia" SET "`+coluna+`" = NULL WHERE "Inicio" = '2026-01-01'`); err != nil {
				t.Fatal(err)
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			irrf, err := LerTabelasIrrf(t.Context(), db)
			if err != nil {
				t.Fatal(err)
			}
			if irrf[1].Reducao != nil {
				t.Fatalf("sem %s, a redução não vale", coluna)
			}
			// Volta, para o próximo caso.
			if _, err := db.ExecContext(t.Context(), `UPDATE "IrrfVigencia" SET "ReducaoIsencaoAte" = '5000.00', "ReducaoMaxima" = '312.89',
				"ReducaoAte" = '7350.00', "ReducaoConstante" = '978.62', "ReducaoFator" = '0.133145' WHERE "Inicio" = '2026-01-01'`); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestParcelasDoInss(t *testing.T) {
	t.Parallel()
	inss, _ := tabelas(t)
	t2026, _ := Vigente(inss, testeapoio.Dia(2026, 1, 1))
	r := CalcularInss(t2026, v("3000.00"))
	quer := []string{"0.00-1621.00 0.075 121.58", "1621.00-2902.84 0.09 115.37", "2902.84-3000.00 0.12 11.66"}
	for i, p := range r.Parcelas {
		if got := p.De.String() + "-" + p.Ate.String() + " " + p.Aliquota.String() + " " + p.Valor.String(); got != quer[i] {
			t.Errorf("parcela %d = %s; quer %s", i, got, quer[i])
		}
	}
	if r.Fonte == "" {
		t.Error("o cálculo leva a fonte da tabela")
	}
}

func TestIrrf(t *testing.T) {
	t.Parallel()
	_, irrf := tabelas(t)
	maio2025, _ := Vigente(irrf, testeapoio.Dia(2025, 6, 1))
	de2026, _ := Vigente(irrf, testeapoio.Dia(2026, 11, 1))
	casos := []struct {
		nome                   string
		tabela                 TabelaIrrf
		rendimento, inss       string
		dependentes            int
		deducao, base, imposto string
		reducao, valor         string
	}{
		// 3000 - 607,20 (simplificado > 248,60) = 2392,80: isento pela tabela.
		{"isento pela tabela", de2026, "3000.00", "248.60", 0, DeducaoSimplificada, "2392.80", "0.00", "0.00", "0.00"},
		// 5000 - 607,20 = 4392,80 x 22,5% - 675,49 = 312,89; até 5000 a redução zera o imposto (por isso 312,89).
		{"5 mil, zerado pela redução", de2026, "5000.00", "501.51", 0, DeducaoSimplificada, "4392.80", "312.89", "312.89", "0.00"},
		// 6000 - 641,51 (legais > 607,20) = 5358,49 x 27,5% - 908,73 = 564,85475; redução 978,62 - 0,133145 x 6000 = 179,75.
		{"6 mil, redução parcial", de2026, "6000.00", "641.51", 0, DeducaoLegal, "5358.49", "564.85", "179.75", "385.10"},
		// com 2 dependentes: 641,51 + 2 x 189,59 = 1020,69; 4979,31 x 27,5% - 908,73 = 460,58025.
		{"6 mil, dois dependentes", de2026, "6000.00", "641.51", 2, DeducaoLegal, "4979.31", "460.58", "179.75", "280.83"},
		// redução no limite: 978,62 - 0,133145 x 7350 = 0,00425 → 0,00; 6518,49 x 27,5% - 908,73 = 883,85475.
		{"no limite da redução", de2026, "7350.00", "831.51", 0, DeducaoLegal, "6518.49", "883.85", "0.00", "883.85"},
		// 8000 - 921,51 = 7078,49 x 27,5% - 908,73 = 1037,85475: acima de 7350, sem redução.
		{"8 mil, sem redução", de2026, "8000.00", "921.51", 0, DeducaoLegal, "7078.49", "1037.85", "0.00", "1037.85"},
		// 2025 não tem a redução: o mesmo 5000 paga 312,89.
		{"2025, sem a redução", maio2025, "5000.00", "509.60", 0, DeducaoSimplificada, "4392.80", "312.89", "0.00", "312.89"},
		// a dedução maior que o rendimento não dá base negativa.
		{"dedução maior que o rendimento", de2026, "500.00", "37.50", 0, DeducaoSimplificada, "0.00", "0.00", "0.00", "0.00"},
		// primeira faixa com imposto: 3500 - 607,20 = 2892,80, na faixa de 15% (até 3751,05): 433,92 - 394,16 = 39,76.
		{"faixa de 15%", de2026, "3500.00", "311.60", 0, DeducaoSimplificada, "2892.80", "39.76", "39.76", "0.00"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			r := CalcularIrrf(c.tabela, v(c.rendimento), v(c.inss), c.dependentes)
			got := []string{r.DeducaoUsada, r.Base.String(), r.ImpostoDaTabela.String(), r.ReducaoLei15270.String(), r.Valor.String()}
			quer := []string{c.deducao, c.base, c.imposto, c.reducao, c.valor}
			for i := range got {
				if got[i] != quer[i] {
					t.Fatalf("IRRF de %s = %v; quer %v", c.rendimento, got, quer)
				}
			}
		})
	}
}

func TestReducaoNaoPassaDoImposto(t *testing.T) {
	t.Parallel()
	_, irrf := tabelas(t)
	de2026, _ := Vigente(irrf, testeapoio.Dia(2026, 1, 1))
	// legais 563,49 < 607,20 (simplificado) → base 4892,80 x 27,5% - 908,73 = 436,79; redução 978,62 - 732,2975 = 246,32.
	r := CalcularIrrf(de2026, v("5500.00"), v("563.49"), 0)
	if r.ReducaoLei15270.String() != "246.32" || r.Valor.String() != "190.47" {
		t.Fatalf("redução %s, valor %s", r.ReducaoLei15270, r.Valor)
	}
	// Uma tabela com redução maior que o imposto: o imposto vai a zero, não fica negativo.
	t2 := de2026
	red := *t2.Reducao
	red.Maxima = v("9999.00")
	t2.Reducao = &red
	if r := CalcularIrrf(t2, v("5000.00"), v("501.51"), 0); r.Valor.String() != "0.00" || r.ReducaoLei15270.String() != "312.89" {
		t.Fatalf("redução %s, valor %s", r.ReducaoLei15270, r.Valor)
	}
}

func TestVigente(t *testing.T) {
	t.Parallel()
	inss, irrf := tabelas(t)
	casos := []struct {
		mes        tipos.Data
		inss, irrf string // "" = sem tabela
	}{
		{testeapoio.Dia(2024, 12, 1), "", ""},
		{testeapoio.Dia(2025, 1, 1), "2025-01-01", ""},
		{testeapoio.Dia(2025, 4, 30), "2025-01-01", ""},
		{testeapoio.Dia(2025, 5, 1), "2025-01-01", "2025-05-01"},
		{testeapoio.Dia(2025, 12, 31), "2025-01-01", "2025-05-01"},
		{testeapoio.Dia(2026, 1, 1), "2026-01-01", "2026-01-01"},
		{testeapoio.Dia(2030, 1, 1), "2026-01-01", "2026-01-01"},
	}
	// As tabelas em qualquer ordem: o mais recente que já começou.
	invertidas := []TabelaInss{inss[1], inss[0]}
	for _, c := range casos {
		t.Run(c.mes.String(), func(t *testing.T) {
			ti, ok := Vigente(invertidas, c.mes)
			if got := map[bool]string{true: ti.Inicio.String(), false: ""}[ok]; got != c.inss {
				t.Errorf("INSS = %q; quer %q", got, c.inss)
			}
			tr, ok := Vigente(irrf, c.mes)
			if got := map[bool]string{true: tr.Inicio.String(), false: ""}[ok]; got != c.irrf {
				t.Errorf("IRRF = %q; quer %q", got, c.irrf)
			}
		})
	}
}
