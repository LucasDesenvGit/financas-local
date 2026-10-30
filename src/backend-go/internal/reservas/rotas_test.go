package reservas

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
)

// rotasDeTeste monta as rotas sobre um banco com uma conta (a 1) e a conferência "Viagem" dela, hoje 30/10/2026 às
// 15h em Brasília, com o dólar a 5 na compra.
func rotasDeTeste(t *testing.T) (*http.ServeMux, *Servico, *bytes.Buffer) {
	t.Helper()
	db := testeapoio.NovoBancoVazio(t)
	r := testeapoio.AgoraEmBrasilia(2026, 10, 30, 15, 0)
	fab := testeapoio.NovaFabrica(t, db, r)
	fab.ReservaConferencia(fab.Conta())
	var saida bytes.Buffer
	s := NovoServico(db, &cambioFalso{t: t, cotacao: dolar("5")}, r, slog.New(slog.NewTextHandler(&saida, nil)))
	mux := http.NewServeMux()
	Registrar(mux, s)
	return mux, s, &saida
}

// Regra: as rotas são as do ReservasController: a conferência sem data vale hoje em Brasília, no futuro não; conta
// inválida e Méli Dólar sem cotação são 400 com a mensagem do serviço; tirar a conferência exige o nome (vazio ou só
// espaços é o [Required]), a conta é número (sem ela, 0) e o que não havia é 404.
func TestRotasDasReservas(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome, metodo, caminho, corpo string
		status                       int
		resposta                     string // vazio = sem corpo; com "errors", só eles
		conferencias                 []string
	}{
		{"lista", "GET", "/api/reservas", "", 200, `[{"contaId":1,"nomeConta":"Conta de teste 1","nome":"Viagem","saldo":100.0,"guardado":0,
			"retirado":0,"movimentos":0,"primeiroMovimento":null,"ultimoMovimento":null,"saldoConferido":100.0,
			"dataConferencia":"2026-10-30","podeTerSaldoAnterior":false,"conferenciaAntiga":false,"informadoPeloBanco":false,
			"moeda":"BRL","quantidade":null,"cotacao":null,"foraDaListaDoBanco":false,"restanteDoBanco":false,"totalDoBanco":null}]`,
			[]string{"1 | Viagem | 100.0 | 2026-10-30"}},
		{"conferir sem a data vale hoje", "PUT", "/api/reservas/conferencia", `{"contaId":1,"nome":"Casa","saldo":12.5}`, 204, "",
			[]string{"1 | Viagem | 100.0 | 2026-10-30", "1 | Casa | 12.5 | 2026-10-30"}},
		{"conferir com a data de hoje", "PUT", "/api/reservas/conferencia", `{"contaId":1,"nome":"viagem","saldo":1,"data":"2026-10-30"}`,
			204, "", []string{"1 | Viagem | 1.0 | 2026-10-30"}},
		{"o Méli Dólar em reais, pela cotação", "PUT", "/api/reservas/conferencia",
			`{"contaId":1,"nome":"Meli Dolar","saldo":10,"data":"2026-10-01","moeda":"BRL"}`, 204, "",
			[]string{"1 | Viagem | 100.0 | 2026-10-30", "1 | Méli Dólar | 2.0 | 2026-10-01"}},
		{"conferir no futuro", "PUT", "/api/reservas/conferencia", `{"contaId":1,"nome":"Casa","saldo":1,"data":"2026-10-31"}`, 400,
			`{"mensagem":"A data da conferência não pode estar no futuro."}`, []string{"1 | Viagem | 100.0 | 2026-10-30"}},
		{"conferir na conta que não existe", "PUT", "/api/reservas/conferencia", `{"contaId":7,"nome":"Casa","saldo":1}`, 400,
			`{"mensagem":"Conta inválida."}`, []string{"1 | Viagem | 100.0 | 2026-10-30"}},
		{"moeda inválida", "PUT", "/api/reservas/conferencia", `{"contaId":1,"nome":"Casa","saldo":1,"moeda":"EUR"}`, 400,
			`{"errors":{"Moeda":["Moeda inválida: use BRL ou USD."]}}`, []string{"1 | Viagem | 100.0 | 2026-10-30"}},
		{"tirar a conferência", "DELETE", "/api/reservas/conferencia?contaId=1&nome=%20VIAGEM", "", 204, "", nil},
		{"tirar a que não há", "DELETE", "/api/reservas/conferencia?contaId=1&nome=Casa", "", 404,
			`{"mensagem":"Registro não encontrado."}`, []string{"1 | Viagem | 100.0 | 2026-10-30"}},
		{"tirar sem a conta é da conta 0", "DELETE", "/api/reservas/conferencia?nome=Viagem", "", 404,
			`{"mensagem":"Registro não encontrado."}`, []string{"1 | Viagem | 100.0 | 2026-10-30"}},
		{"tirar sem o nome", "DELETE", "/api/reservas/conferencia?contaId=1", "", 400,
			`{"errors":{"nome":["The nome field is required."]}}`, []string{"1 | Viagem | 100.0 | 2026-10-30"}},
		{"tirar com o nome só de espaços", "DELETE", "/api/reservas/conferencia?contaId=1&nome=%20%20", "", 400,
			`{"errors":{"nome":["The nome field is required."]}}`, []string{"1 | Viagem | 100.0 | 2026-10-30"}},
		{"tirar com a conta que não é número, e sem o nome", "DELETE", "/api/reservas/conferencia?contaId=abc", "", 400,
			`{"errors":{"contaId":["The value 'abc' is not valid."],"nome":["The nome field is required."]}}`,
			[]string{"1 | Viagem | 100.0 | 2026-10-30"}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			mux, s, _ := rotasDeTeste(t)
			resposta := testeapoio.Chamar(t, mux, c.metodo, c.caminho, c.corpo)
			resposta.ConferirStatus(t, c.status)
			switch {
			case c.resposta == "":
				if len(resposta.Corpo) != 0 {
					t.Errorf("corpo %s, esperado nenhum", resposta.Corpo)
				}
			case strings.Contains(c.resposta, `"errors"`):
				var corpo map[string]json.RawMessage
				if err := json.Unmarshal(resposta.Corpo, &corpo); err != nil {
					t.Fatal(err)
				}
				testeapoio.ConferirJSON(t, []byte(`{"errors":`+string(corpo["errors"])+`}`), c.resposta)
			default:
				resposta.ConferirJSON(t, c.resposta)
			}
			var ficaram []string
			for _, linha := range conferenciasDoBanco(t, s.db) {
				partes := strings.Split(linha, " | ")
				ficaram = append(ficaram, strings.Join(partes[:4], " | "))
			}
			if strings.Join(ficaram, "\n") != strings.Join(c.conferencias, "\n") {
				t.Errorf("no banco:\n%s\nesperado:\n%s", strings.Join(ficaram, "\n"), strings.Join(c.conferencias, "\n"))
			}
		})
	}
}

// Regra: erro do banco na rota é o 500 genérico, com o erro no log.
func TestRotasComErroDoBanco(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ metodo, caminho, corpo string }{
		{"GET", "/api/reservas", ""},
		{"PUT", "/api/reservas/conferencia", `{"contaId":1,"nome":"Casa","saldo":1}`},
		{"DELETE", "/api/reservas/conferencia?contaId=1&nome=Casa", ""},
	} {
		t.Run(c.metodo, func(t *testing.T) {
			t.Parallel()
			mux, s, saida := rotasDeTeste(t)
			if err := s.db.Close(); err != nil {
				t.Fatal(err)
			}
			testeapoio.Chamar(t, mux, c.metodo, c.caminho, c.corpo).ConferirStatus(t, http.StatusInternalServerError)
			if !strings.Contains(saida.String(), "erro nas reservas") {
				t.Errorf("o erro não foi para o log: %q", saida.String())
			}
		})
	}
}

// Regra: a requisição cancelada (quem pediu desistiu) não é erro: nem resposta, nem log.
func TestRotaCanceladaNaoViraErro(t *testing.T) {
	t.Parallel()
	mux, _, saida := rotasDeTeste(t)
	ctx, cancelar := context.WithCancel(t.Context())
	cancelar()
	gravador := httptest.NewRecorder()
	mux.ServeHTTP(gravador, httptest.NewRequestWithContext(ctx, "GET", "/api/reservas", nil))
	if gravador.Body.Len() != 0 || saida.Len() != 0 {
		t.Errorf("corpo %q, log %q", gravador.Body.String(), saida.String())
	}
}
