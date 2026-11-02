package extrato

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
)

// Regra (#80, contrato 14-relatorios): as maiores despesas vêm na ordem que o EF Core dá ao valor gravado (o texto lido
// em pt-BR, com o ponto de milhar: "-27.45" vale −2745 e vem antes de "-223.5", −2235); o empate fica com a data mais
// nova primeiro (o índice IX_Transacao_Data); só as despesas do período, sem as ignoradas, no máximo quantas.
func TestMaioresDespesas(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, testeapoio.HojeEmBrasilia(2026, 10, 14))
	conta := fab.Conta()
	gravar := func(dia int, valor string, ajustes ...func(*testeapoio.Transacao)) int64 {
		return fab.Transacao(conta, append([]func(*testeapoio.Transacao){func(x *testeapoio.Transacao) {
			x.Data, x.Valor = testeapoio.Dia(2026, 9, dia), testeapoio.Valor(valor)
		}}, ajustes...)...).Id
	}
	velho := gravar(8, "-1850.0")
	novo := gravar(20, "-1850.0")
	gravar(5, "-27.45")
	gravar(6, "-223.5")
	gravar(7, "-5000.0", func(x *testeapoio.Transacao) { x.IgnorarRelatorios = true })
	gravar(9, "300.0", func(x *testeapoio.Transacao) { x.Tipo = 2 })
	fab.Transacao(conta, func(x *testeapoio.Transacao) {
		x.Data, x.Valor = testeapoio.Dia(2026, 10, 1), testeapoio.Valor("-9999.0")
	})

	lista, err := MaioresDespesas(t.Context(), db, testeapoio.Dia(2026, 9, 1), testeapoio.Dia(2026, 9, 30), 3)
	if err != nil {
		t.Fatal(err)
	}
	var partes []string
	for _, d := range lista {
		partes = append(partes, fmt.Sprintf("%d:%s", d.Id, d.Valor))
	}
	quer := fmt.Sprintf("%d:-1850.0|%d:-1850.0|%d:-27.45", novo, velho, lista[2].Id)
	if strings.Join(partes, "|") != quer || len(lista) != 3 {
		t.Errorf("maiores: %s, quer %s", strings.Join(partes, "|"), quer)
	}
	vazia, err := MaioresDespesas(t.Context(), db, testeapoio.Dia(2026, 1, 1), testeapoio.Dia(2026, 1, 31), 15)
	if err != nil || vazia == nil || len(vazia) != 0 {
		t.Errorf("período vazio: %v %v", vazia, err)
	}
	if _, err := MaioresDespesas(t.Context(), falhaNa{db, `SELECT "Id", "Valor"`}, testeapoio.Dia(2026, 9, 1),
		testeapoio.Dia(2026, 9, 30), 3); err == nil {
		t.Error("o erro da leitura sumiu")
	}
	if _, err := MaioresDespesas(t.Context(), falhaNa{db, `WHERE t."Id" IN`}, testeapoio.Dia(2026, 9, 1),
		testeapoio.Dia(2026, 9, 30), 3); err == nil {
		t.Error("o erro da leitura dos DTOs sumiu")
	}
	// Um valor gravado que não é número (banco estragado à mão) é erro, e não uma lista pela metade.
	if _, err := db.Exec(`UPDATE "Transacao" SET "Valor" = 'xyz' WHERE "Id" = ?`, velho); err != nil {
		t.Fatal(err)
	}
	if _, err := MaioresDespesas(t.Context(), db, testeapoio.Dia(2026, 9, 1), testeapoio.Dia(2026, 9, 30), 3); err == nil {
		t.Error("o valor estragado passou")
	}
}

// falhaNa é o banco de verdade, mas a consulta que tem o trecho dado devolve erro.
type falhaNa struct {
	consultas.DBTX
	trecho string
}

func (f falhaNa) QueryContext(ctx context.Context, consulta string, argumentos ...any) (*sql.Rows, error) {
	if strings.Contains(consulta, f.trecho) {
		return nil, errors.New("falha de teste")
	}
	return f.DBTX.QueryContext(ctx, consulta, argumentos...)
}

// QueryRowContext da consulta com o trecho também falha (a linha volta com o erro de uma consulta inválida).
func (f falhaNa) QueryRowContext(ctx context.Context, consulta string, argumentos ...any) *sql.Row {
	if strings.Contains(consulta, f.trecho) {
		return f.DBTX.QueryRowContext(ctx, `SELECT "falha de teste" FROM "TabelaQueNaoExiste"`)
	}
	return f.DBTX.QueryRowContext(ctx, consulta, argumentos...)
}

// Regra (#80, contrato 13-painel): o painel mostra as últimas transações até hoje (da mais nova; no mesmo dia, do Id
// maior), as próximas depois de hoje (da mais perto; no mesmo dia, do Id menor), no máximo quantas de cada, e quantas
// futuras existem ao todo.
func TestTransacoesDoPainel(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, testeapoio.HojeEmBrasilia(2026, 10, 14))
	conta := fab.Conta()
	gravar := func(mes time.Month, dia int) int64 {
		return fab.Transacao(conta, func(x *testeapoio.Transacao) { x.Data = testeapoio.Dia(2026, mes, dia) }).Id
	}
	gravar(10, 1)
	hojeA := gravar(10, 14)
	hojeB := gravar(10, 14)
	amanhaA := gravar(10, 15)
	amanhaB := gravar(10, 15)
	gravar(11, 1)
	gravar(12, 1)

	ids := func(l []TransacaoDto) string {
		var p []string
		for _, d := range l {
			p = append(p, fmt.Sprint(d.Id))
		}
		return strings.Join(p, ",")
	}
	hoje := testeapoio.Dia(2026, 10, 14)
	ultimas, futuras, quantas, err := TransacoesDoPainel(t.Context(), db, hoje, 2)
	if err != nil {
		t.Fatal(err)
	}
	if ids(ultimas) != fmt.Sprintf("%d,%d", hojeB, hojeA) || ids(futuras) != fmt.Sprintf("%d,%d", amanhaA, amanhaB) ||
		quantas != 4 {
		t.Errorf("últimas %s, futuras %s, quantas %d", ids(ultimas), ids(futuras), quantas)
	}
	for _, trecho := range []string{`t."Data" <= ?`, `t."Data" > ?`, `COUNT(*)`} {
		if _, _, _, err := TransacoesDoPainel(t.Context(), falhaNa{db, trecho}, hoje, 2); err == nil {
			t.Errorf("%s: o erro sumiu", trecho)
		}
	}
}
