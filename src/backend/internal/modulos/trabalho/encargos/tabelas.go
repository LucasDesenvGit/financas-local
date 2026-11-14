package encargos

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// LerTabelasInss lê todas as vigências do INSS, com as faixas em ordem.
func LerTabelasInss(ctx context.Context, db *sql.DB) ([]TabelaInss, error) {
	linhas, err := db.QueryContext(ctx, `SELECT v."Inicio", v."Fonte", f."Ate", f."Aliquota"
		FROM "InssVigencia" v JOIN "InssFaixa" f ON f."Inicio" = v."Inicio"
		ORDER BY v."Inicio", f."Ordem"`)
	if err != nil {
		return nil, fmt.Errorf("ler as tabelas do INSS: %w", err)
	}
	defer linhas.Close()
	var tabelas []TabelaInss
	for linhas.Next() {
		var inicio tipos.Data
		var fonte string
		var f FaixaInss
		if err := linhas.Scan(&inicio, &fonte, &f.Ate, &f.Aliquota); err != nil {
			return nil, fmt.Errorf("ler faixa do INSS: %w", err)
		}
		if n := len(tabelas); n == 0 || !tabelas[n-1].Inicio.Igual(inicio) {
			tabelas = append(tabelas, TabelaInss{Inicio: inicio, Fonte: fonte})
		}
		tabelas[len(tabelas)-1].Faixas = append(tabelas[len(tabelas)-1].Faixas, f)
	}
	return tabelas, linhas.Err()
}

// todosPreenchidos: a redução só vale com as cinco colunas preenchidas (uma faltando, a vigência fica sem redução).
func todosPreenchidos(valores ...*tipos.Decimal) bool {
	for _, v := range valores {
		if v == nil {
			return false
		}
	}
	return true
}

// LerTabelasIrrf lê todas as vigências do IRRF, com as faixas em ordem e a redução de 2026 em diante.
func LerTabelasIrrf(ctx context.Context, db *sql.DB) ([]TabelaIrrf, error) {
	linhas, err := db.QueryContext(ctx, `SELECT v."Inicio", v."Fonte", v."DeducaoDependente", v."DescontoSimplificado",
			v."ReducaoIsencaoAte", v."ReducaoMaxima", v."ReducaoAte", v."ReducaoConstante", v."ReducaoFator",
			f."Ate", f."Aliquota", f."Deducao"
		FROM "IrrfVigencia" v JOIN "IrrfFaixa" f ON f."Inicio" = v."Inicio"
		ORDER BY v."Inicio", f."Ordem"`)
	if err != nil {
		return nil, fmt.Errorf("ler as tabelas do IRRF: %w", err)
	}
	defer linhas.Close()
	var tabelas []TabelaIrrf
	for linhas.Next() {
		var t TabelaIrrf
		var isencaoAte, maxima, ate, constante, fator *tipos.Decimal
		var f FaixaIrrf
		if err := linhas.Scan(&t.Inicio, &t.Fonte, &t.DeducaoDependente, &t.DescontoSimplificado,
			&isencaoAte, &maxima, &ate, &constante, &fator, &f.Ate, &f.Aliquota, &f.Deducao); err != nil {
			return nil, fmt.Errorf("ler faixa do IRRF: %w", err)
		}
		if n := len(tabelas); n == 0 || !tabelas[n-1].Inicio.Igual(t.Inicio) {
			if todosPreenchidos(isencaoAte, maxima, ate, constante, fator) {
				t.Reducao = &Reducao{IsencaoAte: *isencaoAte, Maxima: *maxima, Ate: *ate, Constante: *constante, Fator: *fator}
			}
			tabelas = append(tabelas, t)
		}
		tabelas[len(tabelas)-1].Faixas = append(tabelas[len(tabelas)-1].Faixas, f)
	}
	return tabelas, linhas.Err()
}
