package erros

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
)

// rotasComErros monta o mux com dois erros gravados: o 1 em aberto e o 2 já resolvido.
func rotasComErros(t *testing.T) (*sql.DB, *http.ServeMux) {
	t.Helper()
	db := testeapoio.NovoBanco(t)
	if _, err := db.ExecContext(t.Context(), `INSERT INTO "ErroSistema" ("Id", "PrimeiraEm", "UltimaEm", "Nivel", "Origem", "Categoria",
		"Mensagem", "Detalhe", "Assinatura", "Resolvido", "ResolvidoEm") VALUES
		(1, '2026-10-01 12:00:00', '2026-10-02 12:00:00', 4, 'API', 'Erro', 'um', 'System.Exception: um', 'a', 0, NULL),
		(2, '2026-10-01 12:00:00', '2026-10-03 12:00:00', 3, 'Câmbio', 'c', 'dois', NULL, 'b', 1, '2026-10-04 00:00:00')`); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	Registrar(mux, NovoServico(db, NovaFila(), relogio.Fixo{Instante: time.Date(2026, 10, 15, 1, 2, 3, 0, time.UTC)}, slog.New(slog.DiscardHandler)))
	return db, mux
}

// Regra: o detalhe traz a categoria e a pilha; o que não existe é 404 com a mensagem; o id que não é número não casa
// com a rota ({id:long}) e é o 404 sem corpo do fallback.
func TestDetalhe(t *testing.T) {
	_, mux := rotasComErros(t)
	r := testeapoio.Chamar(t, mux, "GET", "/api/erros/1", "")
	r.ConferirStatus(t, http.StatusOK)
	r.ConferirJSON(t, `{"id":1,"categoria":"Erro","detalhe":"System.Exception: um"}`)
	nao := testeapoio.Chamar(t, mux, "GET", "/api/erros/999", "")
	nao.ConferirStatus(t, http.StatusNotFound)
	nao.ConferirJSON(t, `{"mensagem":"Registro não encontrado."}`)
	for _, id := range []string{"abc", "99999999999999999999", "1.5"} {
		r := testeapoio.Chamar(t, mux, "GET", "/api/erros/"+id, "")
		if r.Status != http.StatusNotFound || len(r.Corpo) != 0 {
			t.Errorf("/api/erros/%s: %d %q, esperado 404 sem corpo", id, r.Status, r.Corpo)
		}
	}
}

// Regra: resolver grava a hora; reabrir (ou o corpo sem o campo) limpa; 404 para o que não existe; tipo errado é 400.
func TestSituacao(t *testing.T) {
	db, mux := rotasComErros(t)
	estado := func(id int) string {
		var s string
		if err := db.QueryRowContext(t.Context(), `SELECT "Resolvido" || '|' || COALESCE("ResolvidoEm", '-') FROM "ErroSistema" WHERE "Id" = ?`, id).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	testeapoio.Chamar(t, mux, "PUT", "/api/erros/1/situacao", `{"resolvido":true}`).ConferirStatus(t, http.StatusNoContent)
	if s := estado(1); s != "1|2026-10-15 01:02:03" {
		t.Errorf("resolvido: %s", s)
	}
	testeapoio.Chamar(t, mux, "PUT", "/api/erros/1/situacao", `{}`).ConferirStatus(t, http.StatusNoContent)
	if s := estado(1); s != "0|-" {
		t.Errorf("reaberto: %s", s)
	}
	testeapoio.Chamar(t, mux, "PUT", "/api/erros/999/situacao", `{"resolvido":true}`).ConferirStatus(t, http.StatusNotFound)
	testeapoio.Chamar(t, mux, "PUT", "/api/erros/1/situacao", `{"resolvido":"sim"}`).ConferirStatus(t, http.StatusBadRequest)
	testeapoio.Chamar(t, mux, "PUT", "/api/erros/x/situacao", `{"resolvido":true}`).ConferirStatus(t, http.StatusNotFound)
}

// Regra: "resolver todos" marca só os em aberto que passam no filtro, e devolve quantos; filtro inválido é 400.
func TestResolverTodos(t *testing.T) {
	db, mux := rotasComErros(t)
	r := testeapoio.Chamar(t, mux, "POST", "/api/erros/resolver?situacao=todos", "")
	r.ConferirStatus(t, http.StatusOK)
	r.ConferirJSON(t, `{"quantidade":1}`) // o 2 já estava resolvido e fica com a hora dele
	var resolvidoEm string
	if err := db.QueryRowContext(t.Context(), `SELECT "ResolvidoEm" FROM "ErroSistema" WHERE "Id" = 2`).Scan(&resolvidoEm); err != nil || resolvidoEm != "2026-10-04 00:00:00" {
		t.Errorf("o já resolvido mudou: %q %v", resolvidoEm, err)
	}
	testeapoio.Chamar(t, mux, "POST", "/api/erros/resolver", "").ConferirJSON(t, `{"quantidade":0}`)
	testeapoio.Chamar(t, mux, "POST", "/api/erros/resolver?pagina=0", "").ConferirStatus(t, http.StatusBadRequest)
	testeapoio.Chamar(t, mux, "GET", "/api/erros?tamanhoPagina=0", "").ConferirStatus(t, http.StatusBadRequest)
}

// Regra: a lista traz os itens do filtro, o total, os em aberto da tabela toda e as origens em ordem.
func TestListar(t *testing.T) {
	_, mux := rotasComErros(t)
	r := testeapoio.Chamar(t, mux, "GET", "/api/erros?situacao=todos&nivel=Aviso", "")
	r.ConferirStatus(t, http.StatusOK)
	var p struct {
		Itens []struct {
			Id          int64   `json:"id"`
			ResolvidoEm *string `json:"resolvidoEm"`
		} `json:"itens"`
		Total, Abertos int
		Origens        []string
	}
	if err := json.Unmarshal(r.Corpo, &p); err != nil {
		t.Fatal(err)
	}
	if p.Total != 1 || len(p.Itens) != 1 || p.Itens[0].Id != 2 || p.Abertos != 1 || strings.Join(p.Origens, ",") != "API,Câmbio" ||
		p.Itens[0].ResolvidoEm == nil || *p.Itens[0].ResolvidoEm != "2026-10-04T00:00:00Z" {
		t.Errorf("lista: %s", r.Corpo)
	}
}

// Regra: erro no banco é o 500 de sempre (e vai para o log), sem expor o erro.
func TestErroInterno(t *testing.T) {
	db, mux := rotasComErros(t)
	db.Close()
	for _, c := range []struct{ metodo, caminho, corpo string }{
		{"GET", "/api/erros", ""}, {"GET", "/api/erros/1", ""}, {"PUT", "/api/erros/1/situacao", `{"resolvido":true}`},
		{"POST", "/api/erros/resolver", ""},
	} {
		r := testeapoio.Chamar(t, mux, c.metodo, c.caminho, c.corpo)
		if r.Status != http.StatusInternalServerError || strings.Contains(string(r.Corpo), "closed") {
			t.Errorf("%s %s: %d %s", c.metodo, c.caminho, r.Status, r.Corpo)
		}
	}
}

// Regra: o valor do atributo sai como o slog.JSONHandler o escreve e o .NET o lê.
func TestTextoDoValor(t *testing.T) {
	type ponto struct{ X, Y int }
	casos := []struct {
		valor    slog.Value
		esperado string
	}{
		{slog.Uint64Value(7), "7"},
		{slog.Float64Value(1.5), "1.5"},
		{slog.TimeValue(time.Date(2026, 10, 15, 1, 2, 3, 400, time.UTC)), "2026-10-15T01:02:03.0000004Z"},
		{slog.AnyValue(ponto{1, 2}), `{"X":1,"Y":2}`},
		{slog.AnyValue(json.RawMessage(`"texto"`)), "texto"},
		{slog.AnyValue(func() {}), "<função>"},
	}
	for _, c := range casos {
		obtido := textoDoValor(c.valor)
		if c.esperado == "<função>" {
			if obtido == "" {
				t.Error("valor que não vira JSON deveria sair pelo fmt")
			}
			continue
		}
		if obtido != c.esperado {
			t.Errorf("%v: %q, esperado %q", c.valor, obtido, c.esperado)
		}
	}
}
