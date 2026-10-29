package bancos

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
)

func p[T any](v T) *T { return &v }

func texto(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

// Regra: a detecção e a normalização são as do BancoService do C# (palavra inteira, sem acento nem caixa, a ordem do
// catálogo, a instituição antes do nome, o char UTF-16). O gabarito é o testdata/bancos.json, gravado pela sonda das
// contas (internal/faturas/testdata/sonda-csharp) com o código da própria API.
func TestDetectarIgualAoCSharp(t *testing.T) {
	dados, err := os.ReadFile("testdata/bancos.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Bancos []struct {
			Instituicao, Nome, Banco, Normalizado *string
		} `json:"bancos"`
	}
	if err := json.Unmarshal(dados, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Bancos) < 70 {
		t.Fatalf("o gabarito tem só %d casos: a sonda mudou?", len(g.Bancos))
	}
	for _, c := range g.Bancos {
		if obtido := Detectar(c.Instituicao, c.Nome); texto(obtido) != texto(c.Banco) {
			t.Errorf("Detectar(%q, %q) = %s, o C# dá %s", texto(c.Instituicao), texto(c.Nome), texto(obtido), texto(c.Banco))
		}
		if c.Instituicao != nil && normalizar(*c.Instituicao) != *c.Normalizado {
			t.Errorf("normalizar(%q) = %q, o C# dá %q", *c.Instituicao, normalizar(*c.Instituicao), *c.Normalizado)
		}
	}
}

// Regra: Existe e Nome só para os códigos do catálogo, com o "outro" inclusive.
func TestExisteENome(t *testing.T) {
	casos := []struct {
		codigo *string
		nome   string
	}{
		{p("nubank"), "Nubank"},
		{p("itau"), "Itaú"},
		{p("outro"), "Outro (sem banco)"},
		{p("paypal"), "PayPal"},
		{p("Nubank"), "<nil>"},
		{p(""), "<nil>"},
		{nil, "<nil>"},
	}
	for _, c := range casos {
		if obtido := texto(Nome(c.codigo)); obtido != c.nome {
			t.Errorf("Nome(%s) = %s, esperado %s", texto(c.codigo), obtido, c.nome)
		}
		if Existe(c.codigo) != (c.nome != "<nil>") {
			t.Errorf("Existe(%s) = %v", texto(c.codigo), Existe(c.codigo))
		}
	}
}

// Regra: o banco que vale (ADR 0024): o escolhido; o do cartão herda o da conta vinculada (escolhido ou detectado);
// sem ela, o detectado pelos nomes (a instituição antes do nome); "outro" vira nil e impede a detecção, inclusive no
// cartão que herdaria; conta vinculada que não existe não quebra.
func TestEfetivos(t *testing.T) {
	contas := []ContaParaBanco{
		{Id: 1, Nome: "Conta", NomeInstituicao: p("Nu Pagamentos")},
		{Id: 2, Nome: "Cartão Itaú", ContaVinculadaId: p(int64(1))},
		{Id: 3, Nome: "Corrente", Banco: p("caixa")},
		{Id: 4, Nome: "Cartão", ContaVinculadaId: p(int64(3))},
		{Id: 5, Nome: "Cartão Inter", Banco: p("bb"), ContaVinculadaId: p(int64(3))},
		{Id: 6, Nome: "Conta Itaú", Banco: p(Outro)},
		{Id: 7, Nome: "Cartão Itaú", ContaVinculadaId: p(int64(6))},
		{Id: 8, Nome: "Cartão Itaú", ContaVinculadaId: p(int64(99))},
		{Id: 9, Nome: "Sem nada"},
		{Id: 10, Nome: "Banco Inter", NomeInstituicao: p("Nubank")},
		{Id: 11, Nome: "Cartão Nubank", ContaVinculadaId: p(int64(9))},
	}
	esperado := map[int64]string{1: "nubank", 2: "nubank", 3: "caixa", 4: "caixa", 5: "bb", 6: "<nil>", 7: "<nil>",
		8: "itau", 9: "<nil>", 10: "nubank", 11: "nubank"}
	obtido := Efetivos(contas)
	if len(obtido) != len(esperado) {
		t.Fatalf("%d bancos, esperado %d", len(obtido), len(esperado))
	}
	for id, banco := range esperado {
		if texto(obtido[id]) != banco {
			t.Errorf("conta %d: %s, esperado %s", id, texto(obtido[id]), banco)
		}
	}
}

// Regra: Carregar lê a instituição da conexão (LEFT JOIN: conta sem conexão também entra) e devolve o banco de cada uma.
func TestCarregar(t *testing.T) {
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, testeapoio.HojeEmBrasilia(2026, 10, 15))
	conexao := fab.ConexaoBancaria(func(c *testeapoio.ConexaoBancaria) { c.NomeInstituicao = p("Banco Inter S.A.") })
	daConexao := fab.Conta(func(c *testeapoio.Conta) { c.Nome, c.ConexaoBancariaId = "Conta", &conexao.Id })
	manual := fab.Conta(func(c *testeapoio.Conta) { c.Nome = "Poupança Caixa" })
	bancos, err := Carregar(t.Context(), db)
	if err != nil {
		t.Fatal(err)
	}
	if texto(bancos[daConexao.Id]) != "inter" || texto(bancos[manual.Id]) != "caixa" {
		t.Errorf("bancos = %s e %s", texto(bancos[daConexao.Id]), texto(bancos[manual.Id]))
	}
}

// Regra: erro do banco volta como erro.
func TestCarregarComErroDoBanco(t *testing.T) {
	db := testeapoio.NovoBancoVazio(t)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Carregar(t.Context(), db); err == nil {
		t.Error("banco fechado não deu erro")
	}
}
