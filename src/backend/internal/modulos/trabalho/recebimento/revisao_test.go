package recebimento

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/shopspring/decimal"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/emprego"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/holerite"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// Revisão do #703: a parte só a partir de 20% do líquido; a transferência entre contas suas nunca; e a sugestão precisa
// de um motivo forte (a parte e a conta do salário, sozinhas, não bastam).
func TestPontuarESugerir(t *testing.T) {
	t.Parallel()
	conta := int64(7)
	casos := []struct {
		nome    string
		t       Transacao
		pontos  int
		motivos string
		sugerir bool
	}{
		{"PIX de 50 na conta do salário", Transacao{Livre: v("50"), ContaId: 7}, 1, MotivoContaSalario, false},
		{"abaixo de 20% do líquido não é a parte", Transacao{Livre: v("947.60")}, 0, "", false},
		{"20% do líquido é a parte", Transacao{Livre: v("947.61")}, 1, MotivoParte, false},
		{"a parte na conta do salário, sem motivo forte", Transacao{Livre: v("2000"), ContaId: 7}, 2, MotivoParte + " | " + MotivoContaSalario, false},
		{"a parte que o banco marcou como salário", Transacao{Livre: v("2000"), Salario: true}, 3, MotivoParte + " | " + MotivoSalario, true},
		{"o valor do líquido", Transacao{Livre: v("4738.02")}, 3, MotivoValor, true},
		{"cita o empregador", Transacao{Livre: v("10"), Descricao: "TED FICTICIA"}, 2, MotivoEmpregador, true},
		{"transferência entre contas suas", Transacao{Livre: v("4738.02"), Salario: true, ContaId: 7, Transferencia: true}, 0, "", false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			p, m := Pontuar(c.t, liquido("4738.02"), decimal.Zero, "Fictícia Comércio Ltda", &conta)
			if p != c.pontos || strings.Join(m, " | ") != c.motivos || Sugerir(p, m) != c.sugerir {
				t.Fatalf("pontos %d %v, sugerir %v; quer %d %q %v", p, m, Sugerir(p, m), c.pontos, c.motivos, c.sugerir)
			}
		})
	}
	if Sugerir(1, []string{MotivoValor}) {
		t.Fatal("1 ponto não vira sugestão")
	}
}

// Revisão do #703: a sincronização que troca o crédito (a pendente efetivada com outro código) não perde a ligação: ela
// fica sem a transação, conta como recebida, e o site religa sozinho quando a nova aparece. A ligação de um crédito que
// virou débito aparece, não conta e dá para desligar. A parte é arredondada antes de conferir.
func TestReligarDebitoEArredondar(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	rel := testeapoio.HojeEmBrasilia(2026, 11, 13)
	log := slog.New(slog.DiscardHandler)
	empregos := emprego.NovoServico(db, rel, log)
	holerites := holerite.NovoServico(db, empregos, rel, log)
	m := http.NewServeMux()
	emprego.Registrar(m, empregos)
	holerite.Registrar(m, holerites)
	Registrar(m, NovoServico(db, empregos, holerites, rel, log))
	chamar := func(metodo, caminho, corpo string) testeapoio.Resposta {
		return testeapoio.Chamar(t, m, metodo, caminho, corpo)
	}
	ler := func(r testeapoio.Resposta) Resposta {
		t.Helper()
		r.ConferirStatus(t, http.StatusOK)
		var c Resposta
		if err := json.Unmarshal(r.Corpo, &c); err != nil {
			t.Fatalf("%s: %v", r.Corpo, err)
		}
		return c
	}
	fab := testeapoio.NovaFabrica(t, db, rel)
	salario := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome, c.Tipo = "Salário", 2 })
	conta := fab.Conta(func(c *testeapoio.Conta) { c.Nome = "Conta salário" })
	outra := fab.Conta(func(c *testeapoio.Conta) { c.Nome = "Outra" })
	credito := func(c testeapoio.Conta, dia tipos.Data, valor string) testeapoio.Transacao {
		return fab.Transacao(c, func(x *testeapoio.Transacao) {
			x.Data, x.Valor, x.Tipo, x.Descricao = dia, v(valor), 2, "CREDITO FICTICIA"
		})
	}
	var e emprego.Emprego
	_ = json.Unmarshal(chamar("POST", "/api/trabalho/empregos", `{"empregador":"Fictícia Comércio Ltda","admissao":"2024-03-11","salarioBruto":5500}`).Corpo, &e)
	var h holerite.Holerite
	_ = json.Unmarshal(chamar("POST", fmt.Sprintf("/api/trabalho/empregos/%d/holerites", e.Id), `{"mes":"2026-10"}`).Corpo, &h)
	caminho := fmt.Sprintf("/api/trabalho/holerites/%d/recebimentos", h.Id)

	pendente := credito(conta, testeapoio.Dia(2026, 11, 5), "4738.02")
	ler(chamar("POST", caminho, fmt.Sprintf(`{"transacaoId":%d}`, pendente.Id)))
	// A sincronização apaga a pendente: a ligação fica, sem a transação, e conta.
	if _, err := db.ExecContext(t.Context(), `DELETE FROM "Transacao" WHERE "Id" = ?`, pendente.Id); err != nil {
		t.Fatal(err)
	}
	c := ler(chamar("GET", caminho, ""))
	if len(c.Recebimentos) != 1 || !c.Recebimentos[0].Sumiu || c.Recebido.String() != "4738.02" || c.Recebimentos[0].Valor.String() != "4738.02" ||
		c.Recebimentos[0].Data.String() != "2026-11-05" {
		t.Fatalf("sumiu = %+v", c)
	}
	// Outro crédito, em outra conta, com o mesmo valor, não serve; o de 8 dias depois também não; o efetivado 2 dias
	// depois, na mesma conta, sim (o mais perto na data).
	credito(outra, testeapoio.Dia(2026, 11, 6), "4738.02")
	credito(conta, testeapoio.Dia(2026, 11, 13), "4738.02")
	credito(conta, testeapoio.Dia(2026, 11, 2), "4738.03")
	if c = ler(chamar("GET", caminho, "")); !c.Recebimentos[0].Sumiu {
		t.Fatalf("religou com o crédito errado: %+v", c.Recebimentos[0])
	}
	longe := credito(conta, testeapoio.Dia(2026, 11, 9), "4738.02")
	efetivado := credito(conta, testeapoio.Dia(2026, 11, 7), "4738.02")
	c = ler(chamar("GET", caminho, ""))
	var religada int64
	if err := db.QueryRowContext(t.Context(), `SELECT "TransacaoId" FROM "HoleriteRecebimento" WHERE "Id" = ?`, c.Recebimentos[0].Id).Scan(&religada); err != nil ||
		c.Recebimentos[0].Sumiu || religada != efetivado.Id || c.Recebimentos[0].Data.String() != "2026-11-07" {
		t.Fatalf("religado = %+v na transação %d (longe %d, %v)", c.Recebimentos[0], religada, longe.Id, err)
	}
	var categoria *int64
	if err := db.QueryRowContext(t.Context(), `SELECT "CategoriaId" FROM "Transacao" WHERE "Id" = ?`, efetivado.Id).Scan(&categoria); err != nil ||
		categoria == nil || *categoria != salario.Id {
		t.Fatalf("categoria do religado = %v (%v)", categoria, err)
	}
	// O que já está todo ligado a outro holerite não serve para religar.
	var nov holerite.Holerite
	_ = json.Unmarshal(chamar("POST", fmt.Sprintf("/api/trabalho/empregos/%d/holerites", e.Id), `{"mes":"2026-11"}`).Corpo, &nov)
	caminhoNov := fmt.Sprintf("/api/trabalho/holerites/%d/recebimentos", nov.Id)
	ler(chamar("POST", caminhoNov, fmt.Sprintf(`{"transacaoId":%d}`, longe.Id)))
	if _, err := db.ExecContext(t.Context(), `DELETE FROM "Transacao" WHERE "Id" = ?`, efetivado.Id); err != nil {
		t.Fatal(err)
	}
	if c = ler(chamar("GET", caminho, "")); !c.Recebimentos[0].Sumiu {
		t.Fatalf("religou com o crédito de outro holerite: %+v", c.Recebimentos[0])
	}
	ler(chamar("DELETE", fmt.Sprintf("%s/%d", caminho, c.Recebimentos[0].Id), ""))

	// Várias ligações no mesmo holerite, numa leitura só: a viva fica; a que não acha a nova continua sumida; as outras
	// religam, e no empate de distância vale a de data mais antiga.
	var dez holerite.Holerite
	_ = json.Unmarshal(chamar("POST", fmt.Sprintf("/api/trabalho/empregos/%d/holerites", e.Id), `{"mes":"2026-12"}`).Corpo, &dez)
	caminhoDez := fmt.Sprintf("/api/trabalho/holerites/%d/recebimentos", dez.Id)
	viva := credito(conta, testeapoio.Dia(2026, 12, 20), "1000.00")
	o1 := credito(conta, testeapoio.Dia(2026, 12, 21), "1100.00")
	o2 := credito(conta, testeapoio.Dia(2026, 12, 22), "1200.00")
	o3 := credito(conta, testeapoio.Dia(2026, 12, 24), "1300.00")
	for _, x := range []testeapoio.Transacao{viva, o1, o2, o3} {
		ler(chamar("POST", caminhoDez, fmt.Sprintf(`{"transacaoId":%d}`, x.Id)))
	}
	if _, err := db.ExecContext(t.Context(), `DELETE FROM "Transacao" WHERE "Id" IN (?, ?, ?)`, o1.Id, o2.Id, o3.Id); err != nil {
		t.Fatal(err)
	}
	r2 := credito(conta, testeapoio.Dia(2026, 12, 23), "1200.00")
	r3a := credito(conta, testeapoio.Dia(2026, 12, 22), "1300.00")
	credito(conta, testeapoio.Dia(2026, 12, 26), "1300.00")
	c = ler(chamar("GET", caminhoDez, ""))
	var ligadas []string
	for _, rec := range c.Recebimentos {
		var id *int64
		_ = db.QueryRowContext(t.Context(), `SELECT "TransacaoId" FROM "HoleriteRecebimento" WHERE "Id" = ?`, rec.Id).Scan(&id)
		if id == nil {
			ligadas = append(ligadas, "sumiu")
		} else {
			ligadas = append(ligadas, fmt.Sprint(*id))
		}
	}
	if quer := []string{fmt.Sprint(viva.Id), "sumiu", fmt.Sprint(r2.Id), fmt.Sprint(r3a.Id)}; strings.Join(ligadas, " ") != strings.Join(quer, " ") {
		t.Fatalf("ligações de dezembro = %v; quer %v", ligadas, quer)
	}

	// Virou débito: aparece, não conta, e dá para desligar.
	adiantamento := credito(conta, testeapoio.Dia(2026, 10, 20), "2000.00")
	ler(chamar("POST", caminho, fmt.Sprintf(`{"transacaoId":%d}`, adiantamento.Id)))
	if _, err := db.ExecContext(t.Context(), `UPDATE "Transacao" SET "Valor" = '-2000.00' WHERE "Id" = ?`, adiantamento.Id); err != nil {
		t.Fatal(err)
	}
	c = ler(chamar("GET", caminho, ""))
	if len(c.Recebimentos) != 1 || !c.Recebimentos[0].Debito || c.Recebido.String() != "0.00" {
		t.Fatalf("débito = %+v", c)
	}
	if c = ler(chamar("DELETE", fmt.Sprintf("%s/%d", caminho, c.Recebimentos[0].Id), "")); len(c.Recebimentos) != 0 {
		t.Fatalf("depois de desligar = %+v", c.Recebimentos)
	}

	// Meio centavo vira zero: recusa, sem gravar "0.00".
	pix := credito(conta, testeapoio.Dia(2026, 10, 30), "10.00")
	r := chamar("POST", caminho, fmt.Sprintf(`{"transacaoId":%d,"valor":0.004}`, pix.Id))
	r.ConferirStatus(t, http.StatusBadRequest)
	r.ConferirJSON(t, `{"mensagem":"A parte ligada vai de R$ 0,01 até o que sobra do crédito (10,00)."}`)
	if c = ler(chamar("POST", caminho, fmt.Sprintf(`{"transacaoId":%d,"valor":0.005}`, pix.Id))); c.Recebimentos[0].ValorLigado.String() != "0.01" {
		t.Fatalf("meio centavo arredonda para 1: %+v", c.Recebimentos[0])
	}
}

func TestDistancia(t *testing.T) {
	t.Parallel()
	a, b := testeapoio.Dia(2026, 10, 30), testeapoio.Dia(2026, 11, 2)
	if distancia(a, b) != 3 || distancia(b, a) != 3 || distancia(a, a) != 0 {
		t.Fatalf("distância = %d, %d, %d", distancia(a, b), distancia(b, a), distancia(a, a))
	}
}

// Revisão do #703 (2ª): o holerite liga X (05/11) e Y (06/11), de 1000, na mesma conta; a sincronização troca Y por Y'
// (08/11). Religar não pode escolher X, que este holerite já liga (o índice único dava 500 em todo GET): escolhe Y'. Duas
// sumidas de mesmo valor e uma transação nova só: uma religa, a outra continua sumida.
func TestReligarNaoRepeteATransacaoDoHolerite(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	rel := testeapoio.HojeEmBrasilia(2026, 11, 13)
	log := slog.New(slog.DiscardHandler)
	empregos := emprego.NovoServico(db, rel, log)
	holerites := holerite.NovoServico(db, empregos, rel, log)
	m := http.NewServeMux()
	emprego.Registrar(m, empregos)
	holerite.Registrar(m, holerites)
	Registrar(m, NovoServico(db, empregos, holerites, rel, log))
	chamar := func(metodo, caminho, corpo string) testeapoio.Resposta {
		return testeapoio.Chamar(t, m, metodo, caminho, corpo)
	}
	ler := func(r testeapoio.Resposta) Resposta {
		t.Helper()
		r.ConferirStatus(t, http.StatusOK)
		var c Resposta
		if err := json.Unmarshal(r.Corpo, &c); err != nil {
			t.Fatalf("%s: %v", r.Corpo, err)
		}
		return c
	}
	fab := testeapoio.NovaFabrica(t, db, rel)
	fab.Categoria(func(c *testeapoio.Categoria) { c.Nome, c.Tipo = "Salário", 2 })
	conta := fab.Conta(func(c *testeapoio.Conta) { c.Nome = "Conta salário" })
	credito := func(dia tipos.Data) testeapoio.Transacao {
		return fab.Transacao(conta, func(x *testeapoio.Transacao) {
			x.Data, x.Valor, x.Tipo, x.Descricao = dia, v("1000.00"), 2, "CREDITO FICTICIA"
		})
	}
	apagar := func(ids ...int64) {
		for _, id := range ids {
			if _, err := db.ExecContext(t.Context(), `DELETE FROM "Transacao" WHERE "Id" = ?`, id); err != nil {
				t.Fatal(err)
			}
		}
	}
	var e emprego.Emprego
	_ = json.Unmarshal(chamar("POST", "/api/trabalho/empregos", `{"empregador":"Fictícia Comércio Ltda","admissao":"2024-03-11","salarioBruto":5500}`).Corpo, &e)
	var holeriteId int64
	novoHolerite := func(mes string) string {
		var h holerite.Holerite
		_ = json.Unmarshal(chamar("POST", fmt.Sprintf("/api/trabalho/empregos/%d/holerites", e.Id), `{"mes":"`+mes+`"}`).Corpo, &h)
		holeriteId = h.Id
		return fmt.Sprintf("/api/trabalho/holerites/%d/recebimentos", h.Id)
	}
	// transacoes são as ligadas no banco (no JSON, o id do recebimento esconde o da transação).
	transacoes := func() map[int64]bool {
		linhas, err := db.QueryContext(t.Context(), `SELECT "TransacaoId" FROM "HoleriteRecebimento" WHERE "HoleriteId" = ? AND "TransacaoId" IS NOT NULL`, holeriteId)
		if err != nil {
			t.Fatal(err)
		}
		defer linhas.Close()
		ids := map[int64]bool{}
		for linhas.Next() {
			var id int64
			_ = linhas.Scan(&id)
			ids[id] = true
		}
		return ids
	}

	caminho := novoHolerite("2026-10")
	x, y := credito(testeapoio.Dia(2026, 11, 5)), credito(testeapoio.Dia(2026, 11, 6))
	ler(chamar("POST", caminho, fmt.Sprintf(`{"transacaoId":%d}`, x.Id)))
	ler(chamar("POST", caminho, fmt.Sprintf(`{"transacaoId":%d}`, y.Id)))
	apagar(y.Id)
	y2 := credito(testeapoio.Dia(2026, 11, 8))
	c := ler(chamar("GET", caminho, ""))
	if ids := transacoes(); len(c.Recebimentos) != 2 || c.Recebimentos[0].Sumiu || c.Recebimentos[1].Sumiu || !ids[x.Id] || !ids[y2.Id] {
		t.Fatalf("religado = %+v", c.Recebimentos)
	}

	caminho = novoHolerite("2026-09")
	a, b := credito(testeapoio.Dia(2026, 10, 3)), credito(testeapoio.Dia(2026, 10, 4))
	ler(chamar("POST", caminho, fmt.Sprintf(`{"transacaoId":%d}`, a.Id)))
	ler(chamar("POST", caminho, fmt.Sprintf(`{"transacaoId":%d}`, b.Id)))
	apagar(a.Id, b.Id)
	nova := credito(testeapoio.Dia(2026, 10, 5))
	c = ler(chamar("GET", caminho, ""))
	sumidas := 0
	for _, r := range c.Recebimentos {
		if r.Sumiu {
			sumidas++
		}
	}
	if ids := transacoes(); len(c.Recebimentos) != 2 || sumidas != 1 || len(ids) != 1 || !ids[nova.Id] {
		t.Fatalf("duas sumidas, uma nova = %+v", c.Recebimentos)
	}
	ler(chamar("GET", caminho, "")) // e continua abrindo
}

// Revisão do #703 (2ª): religar é conserto; se falhar (aqui, um gatilho recusa o UPDATE), a tela abre do mesmo jeito, a
// ligação continua sumida e a falha vai para o log.
func TestReligarQueFalhaNaoDerrubaATela(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	rel := testeapoio.HojeEmBrasilia(2026, 11, 13)
	var registro bytes.Buffer
	log := slog.New(slog.NewTextHandler(&registro, nil))
	empregos := emprego.NovoServico(db, rel, log)
	holerites := holerite.NovoServico(db, empregos, rel, log)
	m := http.NewServeMux()
	emprego.Registrar(m, empregos)
	holerite.Registrar(m, holerites)
	Registrar(m, NovoServico(db, empregos, holerites, rel, log))
	chamar := func(metodo, caminho, corpo string) testeapoio.Resposta {
		return testeapoio.Chamar(t, m, metodo, caminho, corpo)
	}
	fab := testeapoio.NovaFabrica(t, db, rel)
	fab.Categoria(func(c *testeapoio.Categoria) { c.Nome, c.Tipo = "Salário", 2 })
	conta := fab.Conta(func(c *testeapoio.Conta) { c.Nome = "Conta salário" })
	credito := func(dia tipos.Data) testeapoio.Transacao {
		return fab.Transacao(conta, func(x *testeapoio.Transacao) {
			x.Data, x.Valor, x.Tipo, x.Descricao = dia, v("1000.00"), 2, "CREDITO FICTICIA"
		})
	}
	var e emprego.Emprego
	_ = json.Unmarshal(chamar("POST", "/api/trabalho/empregos", `{"empregador":"Fictícia Comércio Ltda","admissao":"2024-03-11","salarioBruto":5500}`).Corpo, &e)
	var h holerite.Holerite
	_ = json.Unmarshal(chamar("POST", fmt.Sprintf("/api/trabalho/empregos/%d/holerites", e.Id), `{"mes":"2026-10"}`).Corpo, &h)
	caminho := fmt.Sprintf("/api/trabalho/holerites/%d/recebimentos", h.Id)
	x := credito(testeapoio.Dia(2026, 11, 5))
	chamar("POST", caminho, fmt.Sprintf(`{"transacaoId":%d}`, x.Id)).ConferirStatus(t, http.StatusOK)
	if _, err := db.ExecContext(t.Context(), `DELETE FROM "Transacao" WHERE "Id" = ?`, x.Id); err != nil {
		t.Fatal(err)
	}
	credito(testeapoio.Dia(2026, 11, 6))
	if _, err := db.ExecContext(t.Context(), `CREATE TRIGGER "FalhaAoReligar" BEFORE UPDATE OF "TransacaoId" ON "HoleriteRecebimento"
		BEGIN SELECT RAISE(ABORT, 'falha de teste'); END`); err != nil {
		t.Fatal(err)
	}
	registro.Reset()
	r := chamar("GET", caminho, "")
	r.ConferirStatus(t, http.StatusOK)
	var c Resposta
	_ = json.Unmarshal(r.Corpo, &c)
	if len(c.Recebimentos) != 1 || !c.Recebimentos[0].Sumiu || !strings.Contains(registro.String(), "falha de teste") {
		t.Fatalf("recebimentos = %+v; log = %s", c.Recebimentos, registro.String())
	}
}
