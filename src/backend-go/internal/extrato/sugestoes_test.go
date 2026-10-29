package extrato

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
)

// Regra: as rotas das sugestões: somenteSemCategoria vale true e dias vale 365 sem os parâmetros (o nome sem diferença
// de maiúsculas); o período vai de 1 a 3660 dias; o que não converte é o 400 do model binding; a transação que não
// existe é 404 com mensagem, e o {id:long} que não casa, 404 sem corpo.
func TestRotasDasSugestoes(t *testing.T) {
	mux, _, fab := rotasDeTeste(t, minimoFixo{n: 4})
	conta := fab.Conta()
	transporte := fab.Categoria(func(x *testeapoio.Categoria) { x.Nome = "Transporte" })
	semCategoria := fab.Transacao(conta, func(x *testeapoio.Transacao) { x.Descricao = "UBER *TRIP" })
	fab.Transacao(conta, func(x *testeapoio.Transacao) { x.Descricao, x.CategoriaId = "POSTO SHELL", &transporte.Id })

	var grupos []struct{ Chave string }
	resp := testeapoio.Chamar(t, mux, "GET", "/api/transacoes/sugestoes-categoria", "")
	resp.ConferirStatus(t, http.StatusOK)
	if err := json.Unmarshal(resp.Corpo, &grupos); err != nil || len(grupos) != 1 || grupos[0].Chave != "UBER TRIP" {
		t.Errorf("padrão: %s (%v)", resp.Corpo, err)
	}
	testeapoio.Chamar(t, mux, "GET", "/api/transacoes/sugestoes-categoria?DIAS=1&SomenteSemCategoria=false", "").ConferirStatus(t, http.StatusOK)
	testeapoio.Chamar(t, mux, "GET", "/api/transacoes/sugestoes-categoria?dias=3660", "").ConferirStatus(t, http.StatusOK)
	for _, dias := range []string{"0", "3661"} {
		r := testeapoio.Chamar(t, mux, "GET", "/api/transacoes/sugestoes-categoria?dias="+dias, "")
		r.ConferirStatus(t, http.StatusBadRequest)
		r.ConferirJSON(t, `{"mensagem":"Período inválido."}`)
	}
	r := testeapoio.Chamar(t, mux, "GET", "/api/transacoes/sugestoes-categoria?dias=muitos&somenteSemCategoria=talvez", "")
	r.ConferirStatus(t, http.StatusBadRequest)
	var problema struct{ Errors json.RawMessage }
	if err := json.Unmarshal(r.Corpo, &problema); err != nil {
		t.Fatal(err)
	}
	testeapoio.ConferirJSON(t, problema.Errors, `{"dias":["The value 'muitos' is not valid."],"somenteSemCategoria":["The value 'talvez' is not valid."]}`)

	testeapoio.Chamar(t, mux, "GET", fmt.Sprintf("/api/transacoes/%d/sugestoes-categoria", semCategoria.Id), "").
		ConferirJSON(t, fmt.Sprintf(`[{"categoriaId":%d,"nome":"Transporte","cor":"#64748b","motivo":"Palavra-chave \"UBER\""}]`, transporte.Id))
	inexistente := testeapoio.Chamar(t, mux, "GET", "/api/transacoes/999/sugestoes-categoria", "")
	inexistente.ConferirStatus(t, http.StatusNotFound)
	inexistente.ConferirJSON(t, `{"mensagem":"Registro não encontrado."}`)
	semCorpo := testeapoio.Chamar(t, mux, "GET", "/api/transacoes/abc/sugestoes-categoria", "")
	semCorpo.ConferirStatus(t, http.StatusNotFound)
	if len(semCorpo.Corpo) != 0 {
		t.Errorf("abc: %s", semCorpo.Corpo)
	}
}

// Regra: erro do banco nas sugestões é o 500.
func TestSugestoesComErroDoBanco(t *testing.T) {
	mux, db, _ := rotasDeTeste(t, minimoFixo{n: 4})
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	testeapoio.Chamar(t, mux, "GET", "/api/transacoes/sugestoes-categoria", "").ConferirStatus(t, http.StatusInternalServerError)
	testeapoio.Chamar(t, mux, "GET", "/api/transacoes/1/sugestoes-categoria", "").ConferirStatus(t, http.StatusInternalServerError)
}
