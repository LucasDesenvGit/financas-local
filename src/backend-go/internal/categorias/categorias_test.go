package categorias

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
)

func rotasDeTeste(t *testing.T) (*http.ServeMux, *sql.DB, *testeapoio.Fabrica) {
	t.Helper()
	db := testeapoio.NovoBancoVazio(t)
	r := testeapoio.HojeEmBrasilia(2026, 10, 15)
	mux := http.NewServeMux()
	Registrar(mux, NovoServico(db, r, slog.New(slog.DiscardHandler)))
	return mux, db, testeapoio.NovaFabrica(t, db, r)
}

// Regra: o que passa pelo JSON e o banco recusa (enum fora da lista no CHECK, Tipo ausente = 0) é o 500 genérico,
// como o DbUpdateException do C#; nada fica gravado. (Fora do contrato: o 500 grava em ErroSistema.)
func TestValorQueOBancoRecusaEhO500(t *testing.T) {
	casos := []struct{ nome, corpo string }{
		{"tipo fora da lista", `{"nome":"X","tipo":9,"cor":"#64748b"}`},
		{"tipo ausente vale 0", `{"nome":"X","cor":"#64748b"}`},
		{"natureza fora da lista", `{"nome":"X","tipo":1,"cor":"#64748b","natureza":3}`},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			mux, db, _ := rotasDeTeste(t)
			resp := testeapoio.Chamar(t, mux, "POST", "/api/categorias", c.corpo)
			resp.ConferirStatus(t, http.StatusInternalServerError)
			var n int
			if err := db.QueryRow(`SELECT COUNT(*) FROM Categoria`).Scan(&n); err != nil || n != 0 {
				t.Errorf("ficaram %d categorias (%v)", n, err)
			}
		})
	}
}

// Regra: o enum lido de texto é recusado (sem JsonStringEnumConverter), o int lido de texto passa (AllowReadingFromString),
// byte fora de 0 a 255 é recusado, e null num int não anulável também.
func TestModelBindingDosNumeros(t *testing.T) {
	casos := []struct {
		nome, corpo string
		status      int
		trecho      string
	}{
		{"enum em texto", `{"nome":"X","tipo":"1","cor":"#64748b"}`, 400, `"$.tipo"`},
		{"enum acima de 255", `{"nome":"X","tipo":256,"cor":"#64748b"}`, 400, `"$.tipo"`},
		{"enum negativo", `{"nome":"X","tipo":-1,"cor":"#64748b"}`, 400, `"$.tipo"`},
		{"enum com casa decimal", `{"nome":"X","tipo":1.0,"cor":"#64748b"}`, 400, `"$.tipo"`},
		{"int anulável em texto passa", `{"nome":"X","tipo":1,"cor":"#64748b","categoriaPaiId":"999"}`, 400, `Categoria pai inválida.`},
		{"int anulável com casa decimal", `{"nome":"X","tipo":1,"cor":"#64748b","categoriaPaiId":7.5}`, 400, `"$.categoriaPaiId"`},
		{"bool em texto é recusado", `{"nome":"X","tipo":1,"cor":"#64748b","ativa":"true"}`, 400, `"$.ativa"`},
		{"cor só de espaços: obrigatória e fora do padrão", `{"nome":"X","tipo":1,"cor":"   "}`, 400, `"The Cor field is required.","The field Cor must match the regular expression '^#[0-9a-fA-F]{6}$'."`},
		{"cor com o padrão no meio não vale", `{"nome":"X","tipo":1,"cor":"x#123456"}`, 400, `must match the regular expression`},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			mux, _, _ := rotasDeTeste(t)
			resp := testeapoio.Chamar(t, mux, "POST", "/api/categorias", c.corpo)
			resp.ConferirStatus(t, c.status)
			if !strings.Contains(string(resp.Corpo), c.trecho) {
				t.Errorf("a resposta não tem %s:\n%s", c.trecho, resp.Corpo)
			}
		})
	}
}

// Regra: o {id:int} do ASP.NET: o que não é int de 32 bits é o 404 sem corpo do fallback.
func TestIdDaRota(t *testing.T) {
	mux, _, _ := rotasDeTeste(t)
	for _, caminho := range []string{"/api/categorias/abc", "/api/categorias/2147483648", "/api/regras/1.5", "/api/regras-descricao/-"} {
		resp := testeapoio.Chamar(t, mux, "DELETE", caminho, "")
		if resp.Status != http.StatusNotFound || len(resp.Corpo) != 0 {
			t.Errorf("DELETE %s: %d %q; esperado 404 sem corpo", caminho, resp.Status, resp.Corpo)
		}
	}
	resp := testeapoio.Chamar(t, mux, "DELETE", "/api/categorias/2147483647", "")
	resp.ConferirJSON(t, `{"mensagem":"Registro não encontrado."}`)
}

// Regra: a troca de tipo refaz as transações em lotes (mais de 500 ids), pelo sinal e pelo cartão (ADR 0036), sem
// mexer em AtualizadoEm, e só nas que não são transferência lançada pelo site.
func TestTrocarOTipoRefazAsTransacoesEmLotes(t *testing.T) {
	mux, db, fab := rotasDeTeste(t)
	cat := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome, c.Tipo = "Entre contas", 3 })
	corrente, cartao := fab.Conta(), fab.Conta(func(c *testeapoio.Conta) { c.Tipo = 3 })
	grupo := tipos.NovoGuid()
	for i := 0; i < 600; i++ {
		conta, valor := corrente, "-1.00"
		switch {
		case i%3 == 1:
			valor = "1.00" // entrada na conta: receita
		case i%3 == 2:
			conta, valor = cartao, "1.00" // entrada no cartão: estorno, despesa
		}
		fab.Transacao(conta, func(tr *testeapoio.Transacao) {
			tr.CategoriaId, tr.Tipo, tr.IgnorarRelatorios, tr.Valor = &cat.Id, transacoes.Transferencia, true, tipos.Decimal{}
			tr.Valor, _ = tipos.NovoDecimal(valor)
		})
	}
	site := fab.Transacao(corrente, func(tr *testeapoio.Transacao) {
		tr.CategoriaId, tr.Tipo, tr.IgnorarRelatorios, tr.TransferenciaGrupo = &cat.Id, transacoes.Transferencia, true, &grupo
	})
	resp := testeapoio.Chamar(t, mux, "PUT", fmt.Sprintf("/api/categorias/%d", cat.Id), `{"nome":"Entre contas","tipo":2,"cor":"#64748b"}`)
	resp.ConferirStatus(t, http.StatusOK)
	contar := func(onde string) int {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM Transacao WHERE `+onde, cat.Id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := contar(`CategoriaId = ? AND Tipo = 1 AND IgnorarRelatorios = 0 AND TransferenciaGrupo IS NULL`); n != 400 {
		t.Errorf("%d despesas (saídas e entradas no cartão); esperado 400", n)
	}
	if n := contar(`CategoriaId = ? AND Tipo = 2 AND IgnorarRelatorios = 0`); n != 200 {
		t.Errorf("%d receitas; esperado 200", n)
	}
	var tipo int64
	var ignorar bool
	if err := db.QueryRow(`SELECT Tipo, IgnorarRelatorios FROM Transacao WHERE Id = ?`, site.Id).Scan(&tipo, &ignorar); err != nil || tipo != transacoes.Transferencia || !ignorar {
		t.Errorf("a lançada pelo site mudou: %d %v %v", tipo, ignorar, err)
	}
	if n := contar(`CategoriaId = ? AND AtualizadoEm <> CriadoEm`); n != 0 {
		t.Errorf("%d transações com AtualizadoEm mudado: o ExecuteUpdate do C# não mexe nele", n)
	}
}

// Regra: a query da prévia como o model binding do ASP.NET: tipo pelo nome (sem diferença de maiúsculas), pelo
// número ou pela combinação com vírgula; decimal na cultura invariante, com vírgula de milhar.
func TestLerDaQuery(t *testing.T) {
	enums := []struct {
		valor    string
		esperado int64
		ok       bool
	}{
		{"Igual", 3, true}, {"comecacom", 2, true}, {" CONTEM ", 1, true}, {"2", 2, true}, {"9", 9, true},
		{"Contem, Igual", 3, true}, {"Igual,Contem", 3, true}, {"2,1", 3, true}, {"abc", 0, false}, {"256", 0, false}, {"-1", 0, false}, {"", 0, false},
	}
	for _, e := range enums {
		if got, ok := lerEnumDaQuery(e.valor, nomesDoTipoCorrespondencia); got != e.esperado || ok != e.ok {
			t.Errorf("lerEnumDaQuery(%q) = %d, %v; esperado %d, %v", e.valor, got, ok, e.esperado, e.ok)
		}
	}
	decimais := []struct {
		valor, esperado string
		ok              bool
	}{
		{"100", "100", true}, {" 200.50 ", "200.50", true}, {"1,000.5", "1000.5", true}, {"-3", "-3", true},
		{"abc", "", false}, {"1e3", "", false}, {"1.2.3", "", false},
	}
	for _, d := range decimais {
		got, ok := lerDecimalDaQuery(d.valor)
		if ok != d.ok || (ok && got.String() != d.esperado) {
			t.Errorf("lerDecimalDaQuery(%q) = %s, %v; esperado %s, %v", d.valor, got, ok, d.esperado, d.ok)
		}
	}
}

// Regra: a prévia responde o 400 automático com o nome do parâmetro e, sem texto, zero e lista vazia.
func TestPreviaPelaRota(t *testing.T) {
	mux, _, _ := rotasDeTeste(t)
	resp := testeapoio.Chamar(t, mux, "GET", "/api/regras-descricao/previa?texto=x&TIPO=xyz&valorMaximo=1,0,0.x", "")
	resp.ConferirStatus(t, http.StatusBadRequest)
	if !strings.Contains(string(resp.Corpo), `"tipo":["The value 'xyz' is not valid."]`) ||
		!strings.Contains(string(resp.Corpo), `"valorMaximo":["The value '1,0,0.x' is not valid."]`) {
		t.Errorf("400 da prévia: %s", resp.Corpo)
	}
	testeapoio.Chamar(t, mux, "GET", "/api/regras-descricao/previa", "").ConferirJSON(t, `{"quantidade":0,"exemplos":[]}`)
}
