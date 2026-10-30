package cambio

import (
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Regra: ObterConversor monta o conversor com as últimas cotações guardadas (o dólar de hoje converte; a moeda sem
// cotação não) e, com a data, também o histórico desde ela; erro do banco volta como erro.
func TestObterConversor(t *testing.T) {
	t.Parallel()
	s, db, fab := novoServicoDeTeste(t, testeapoio.AgoraEmBrasilia(2026, 10, 14, 15, 0), &fonteFalsa{})
	fab.CotacaoMoeda()
	dez := testeapoio.Valor("10")
	c, err := s.ObterConversor(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if v := c.ParaReais(dez, "USD"); v == nil || v.String() != "54.12" {
		t.Errorf("10 dólares = %v, esperado 54.12 (a compra, 5.4123)", v)
	}
	if v := c.ParaReais(dez, "JPY"); v != nil {
		t.Errorf("iene sem cotação = %v, esperado nil", v)
	}
	desde := tipos.NovaData(2026, 10, 1)
	if _, err := s.ObterConversor(t.Context(), &desde); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ObterConversor(t.Context(), nil); err == nil {
		t.Error("banco fechado não deu erro")
	}
	if _, err := s.ObterConversor(t.Context(), &desde); err == nil {
		t.Error("banco fechado não deu erro com o histórico")
	}
}

// Regra: com a data, o histórico começa 7 dias antes dela: a compra do último dia útil antes da data ainda vale para
// ela, e não a de hoje.
func TestObterConversorComHistorico(t *testing.T) {
	t.Parallel()
	s, _, fab := novoServicoDeTeste(t, testeapoio.AgoraEmBrasilia(2026, 10, 14, 15, 0), &fonteFalsa{})
	fab.CotacaoMoeda()
	desde := tipos.NovaData(2026, 10, 8)
	fab.CotacaoMoeda(func(c *testeapoio.CotacaoMoeda) { c.Data, c.Compra = desde.SomarDias(-3), testeapoio.Valor("5") })
	c, err := s.ObterConversor(t.Context(), &desde)
	if err != nil {
		t.Fatal(err)
	}
	if v := c.ParaReaisNaData(testeapoio.Valor("10"), "USD", desde); v == nil || v.String() != "50.0" {
		t.Errorf("10 dólares em 08/10 = %v, esperado 50.0 (a compra de 05/10)", v)
	}
}
