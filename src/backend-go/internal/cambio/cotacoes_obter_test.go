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
