package reservas

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
)

// ambienteDaSaida é um banco com uma conta do Open Finance (com 500 guardados no cofrinho Emergência em 01/10), uma
// manual e uma categoria, hoje 30/10/2026, e as rotas das reservas.
func ambienteDaSaida(t *testing.T) (*http.ServeMux, *sql.DB, testeapoio.Conta, testeapoio.Conta, testeapoio.Categoria) {
	t.Helper()
	db := testeapoio.NovoBancoVazio(t)
	r := testeapoio.AgoraEmBrasilia(2026, 10, 30, 15, 0)
	fab := testeapoio.NovaFabrica(t, db, r)
	of := fab.Conta(func(c *testeapoio.Conta) { c.Origem = 2 })
	manual := fab.Conta()
	categoria := fab.Categoria()
	fab.Transacao(of, func(tr *testeapoio.Transacao) {
		tr.Descricao, tr.Valor, tr.Data = "Dinheiro reservado Emergência", testeapoio.Valor("-500.00"), testeapoio.Dia(2026, 10, 1)
	})
	mux := http.NewServeMux()
	Registrar(mux, NovoServico(db, cambioDaLista{}, r, slog.New(slog.DiscardHandler)))
	return mux, db, of, manual, categoria
}

// saldoDe é o saldo da reserva Emergência pela rota da lista, em texto ("" se ela não está).
func saldoDe(t *testing.T, mux *http.ServeMux) string {
	t.Helper()
	var lista []ReservaDto
	if err := jsonDe(testeapoio.Chamar(t, mux, "GET", "/api/reservas", "").Corpo, &lista); err != nil {
		t.Fatal(err)
	}
	for _, r := range lista {
		if r.Nome == "Emergência" {
			return r.Saldo.String()
		}
	}
	return ""
}

// Regra (#691): a saída do cofrinho só vale em conta do Open Finance, fora do Méli Dólar, com categoria que existe e
// data que não está no futuro; gravada, vira despesa do app marcada com o cofrinho e baixa a reserva.
func TestRegistrarSaida(t *testing.T) {
	t.Parallel()
	mux, db, of, manual, categoria := ambienteDaSaida(t)
	corpo := func(conta int64, nome, data string, categoriaId string) string {
		return fmt.Sprintf(`{"contaId":%d,"nome":%q,"data":%q,"valor":120.5,"descricao":"  Compra no Mercado Livre ",`+
			`"categoriaId":%s}`, conta, nome, data, categoriaId)
	}
	casos := []struct {
		nome, corpo string
		status      int
		mensagem    string
	}{
		{"conta que não existe", corpo(999, "Emergência", "2026-10-29", "null"), 400, "Conta inválida."},
		{"conta manual", corpo(manual.Id, "Emergência", "2026-10-29", "null"), 400, "O cofrinho tem de ser de uma conta do Open Finance."},
		{"Méli Dólar", corpo(of.Id, "Meli Dolar", "2026-10-29", "null"), 400, "O Méli Dólar não tem saída pelo site: ela é em dólar."},
		{"categoria que não existe", corpo(of.Id, "Emergência", "2026-10-29", "999"), 400, "Categoria inválida."},
		{"no futuro", corpo(of.Id, "Emergência", "2026-10-31", "null"), 400, "A data da saída não pode estar no futuro."},
		{"sem a data", `{"contaId":1,"nome":"Emergência","valor":10,"descricao":"x"}`, 400, ""},
		{"valor zero", `{"contaId":1,"nome":"Emergência","data":"2026-10-29","valor":0,"descricao":"x"}`, 400, ""},
		{"gravada", corpo(of.Id, "  emergência ", "2026-10-29", fmt.Sprint(categoria.Id)), 204, ""},
	}
	for _, c := range casos {
		resposta := testeapoio.Chamar(t, mux, "POST", "/api/reservas/saida", c.corpo)
		if resposta.Status != c.status {
			t.Errorf("%s: status %d: %s", c.nome, resposta.Status, resposta.Corpo)
			continue
		}
		if c.mensagem != "" {
			var m map[string]string
			if err := jsonDe(resposta.Corpo, &m); err != nil || m["mensagem"] != c.mensagem {
				t.Errorf("%s: %s", c.nome, resposta.Corpo)
			}
		}
	}
	var descricao, valor, reserva string
	var tipo int
	if err := db.QueryRow(`SELECT "Descricao", "Valor", "ReservaNome", "Tipo" FROM "Transacao" WHERE "ReservaNome" IS NOT NULL`).
		Scan(&descricao, &valor, &reserva, &tipo); err != nil {
		t.Fatal(err)
	}
	if descricao != "Compra no Mercado Livre" || valor != "-120.5" || reserva != "emergência" || tipo != 1 {
		t.Errorf("lançamento: %q %s %q tipo %d", descricao, valor, reserva, tipo)
	}
	if saldo := saldoDe(t, mux); !mesmoValor(saldo, "379.50") {
		t.Errorf("saldo da reserva: %s, esperado 379.50", saldo)
	}
}

// Regra (#691): conferir com "registrar a diferença" lança o que a estimativa passa do conferido como gasto do
// cofrinho, hoje, e grava a conferência: a reserva fica com o conferido. Só com a data de hoje, numa reserva estimada
// em reais e com o conferido abaixo da estimativa.
func TestConferirComDiferenca(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome, corpo string
		status      int
		mensagem    string
		saldo       string
		lancamentos int
	}{
		{"conferido abaixo: registra a diferença", `{"contaId":%d,"nome":"emergência","saldo":420.25,"data":"2026-10-30","registrarDiferenca":true}`,
			204, "", "420.25", 1},
		{"outra data", `{"contaId":%d,"nome":"Emergência","saldo":420,"data":"2026-10-29","registrarDiferenca":true}`,
			400, "Para registrar a diferença como gasto, confira com a data de hoje.", "500.00", 0},
		{"sem a data vale hoje", `{"contaId":%d,"nome":"Emergência","saldo":400,"registrarDiferenca":true}`, 204, "", "400", 1},
		{"conferido igual", `{"contaId":%d,"nome":"Emergência","saldo":500,"registrarDiferenca":true}`,
			400, "O saldo conferido não está abaixo da estimativa: não há diferença para registrar.", "500.00", 0},
		{"reserva que não existe", `{"contaId":%d,"nome":"Viagem","saldo":1,"registrarDiferenca":true}`,
			400, "Esta reserva não tem estimativa para comparar (o valor vem do banco ou é em dólar).", "500.00", 0},
		{"categoria que não existe: nada gravado", `{"contaId":%d,"nome":"Emergência","saldo":400,"registrarDiferenca":true,"categoriaId":999}`,
			400, "Categoria inválida.", "500.00", 0},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			mux, db, of, _, _ := ambienteDaSaida(t)
			resposta := testeapoio.Chamar(t, mux, "PUT", "/api/reservas/conferencia", fmt.Sprintf(c.corpo, of.Id))
			if resposta.Status != c.status {
				t.Fatalf("status %d: %s", resposta.Status, resposta.Corpo)
			}
			if c.mensagem != "" {
				var m map[string]string
				if err := jsonDe(resposta.Corpo, &m); err != nil || m["mensagem"] != c.mensagem {
					t.Errorf("mensagem: %s", resposta.Corpo)
				}
			}
			var lancamentos, conferencias int
			if err := db.QueryRow(`SELECT (SELECT COUNT(*) FROM "Transacao" WHERE "ReservaNome" IS NOT NULL),
				(SELECT COUNT(*) FROM "ReservaConferencia")`).Scan(&lancamentos, &conferencias); err != nil {
				t.Fatal(err)
			}
			if saldo := saldoDe(t, mux); !mesmoValor(saldo, c.saldo) || lancamentos != c.lancamentos || conferencias != c.lancamentos {
				t.Errorf("saldo %s, %d lançamentos, %d conferências; esperado %s, %d", saldo, lancamentos, conferencias,
					c.saldo, c.lancamentos)
			}
		})
	}
}

func jsonDe(corpo []byte, destino any) error { return json.Unmarshal(corpo, destino) }

// mesmoValor compara dois decimais escritos em texto ("379.5" e "379.50" são o mesmo); vazio só é igual a vazio.
func mesmoValor(a, b string) bool {
	if a == "" || b == "" {
		return a == b
	}
	return testeapoio.Valor(a).Decimal().Cmp(testeapoio.Valor(b).Decimal()) == 0
}

// Regra (#691): a diferença como gasto só vale na reserva estimada: a que o banco informa pelo nome e a linha do
// restante do total do banco não têm estimativa para comparar, e nada é gravado.
func TestConferirComDiferencaSemEstimativa(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome, reservado, informadas, reserva string
	}{
		{"informada pelo banco pelo nome", "500.0", `[{"nome":"Emergência","valor":500.0}]`, "Emergência"},
		{"restante do total do banco", "900.0", "", "Reservado"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			mux, db, of, _, _ := ambienteDaSaida(t)
			var informadas any
			if c.informadas != "" {
				informadas = c.informadas
			}
			if _, err := db.Exec(`UPDATE "Conta" SET "SaldoReservado" = ?, "ReservasInformadas" = ? WHERE "Id" = ?`,
				c.reservado, informadas, of.Id); err != nil {
				t.Fatal(err)
			}
			resposta := testeapoio.Chamar(t, mux, "PUT", "/api/reservas/conferencia",
				fmt.Sprintf(`{"contaId":%d,"nome":%q,"saldo":10,"registrarDiferenca":true}`, of.Id, c.reserva))
			var m map[string]string
			if resposta.Status != 400 || jsonDe(resposta.Corpo, &m) != nil ||
				m["mensagem"] != "Esta reserva não tem estimativa para comparar (o valor vem do banco ou é em dólar)." {
				t.Errorf("status %d: %s", resposta.Status, resposta.Corpo)
			}
			var lancamentos int
			if err := db.QueryRow(`SELECT COUNT(*) FROM "Transacao" WHERE "ReservaNome" IS NOT NULL`).Scan(&lancamentos); err != nil ||
				lancamentos != 0 {
				t.Errorf("%d lançamentos pelo cofrinho, %v", lancamentos, err)
			}
		})
	}
}
