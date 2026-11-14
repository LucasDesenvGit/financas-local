package holeritepdf

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/emprego"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/holerite"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/leitor/ficticios"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

type mux struct{ *http.ServeMux }

func (m mux) HandleFunc(p string, h http.HandlerFunc) { m.ServeMux.HandleFunc(p, h) }

type ambiente struct {
	t         *testing.T
	s         *Servico
	holerites *holerite.Servico
	h         http.Handler
}

func novo(t *testing.T) ambiente {
	t.Helper()
	db := testeapoio.NovoBancoVazio(t)
	r := testeapoio.HojeEmBrasilia(2026, 11, 13)
	log := slog.New(slog.DiscardHandler)
	empregos := emprego.NovoServico(db, r, log)
	holerites := holerite.NovoServico(db, empregos, r, log)
	s := NovoServico(db, empregos, holerites, r, log)
	m := mux{http.NewServeMux()}
	emprego.Registrar(m, empregos)
	holerite.Registrar(m, holerites)
	Registrar(m, s)
	return ambiente{t: t, s: s, holerites: holerites, h: m}
}

func (a ambiente) chamar(metodo, caminho, corpo string) testeapoio.Resposta {
	a.t.Helper()
	return testeapoio.Chamar(a.t, a.h, metodo, caminho, corpo)
}

func decodificar[T any](t *testing.T, r testeapoio.Resposta) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(r.Corpo, &v); err != nil {
		t.Fatalf("ler %s: %v", r.Corpo, err)
	}
	return v
}

// emprego cadastra um emprego CLT e devolve o id (extra: os campos a mais do JSON, como o CNPJ).
func (a ambiente) emprego(salario, extra string) int64 {
	a.t.Helper()
	r := a.chamar("POST", "/api/trabalho/empregos", `{"empregador":"Empresa Fictícia","admissao":"2024-03-11","salarioBruto":`+salario+extra+`}`)
	r.ConferirStatus(a.t, http.StatusCreated)
	return decodificar[emprego.Emprego](a.t, r).Id
}

func corpoDoPdf(pdf []byte, senha string) string {
	b, _ := json.Marshal(struct {
		Arquivo []byte `json:"arquivo"`
		Senha   string `json:"senha"`
	}{pdf, senha})
	return string(b)
}

func (a ambiente) ler(empregoId int64, pdf []byte, senha string) testeapoio.Resposta {
	a.t.Helper()
	return a.chamar("POST", fmt.Sprintf("/api/trabalho/empregos/%d/holerites/pdf", empregoId), corpoDoPdf(pdf, senha))
}

func (a ambiente) previa(empregoId int64, paginas ...[]ficticios.Texto) Previa {
	a.t.Helper()
	r := a.ler(empregoId, ficticios.GerarPDF(paginas, ""), "")
	r.ConferirStatus(a.t, http.StatusOK)
	return decodificar[Previa](a.t, r)
}

// confirmacao monta o corpo do "Confirmar" com as linhas como a prévia trouxe, e muda o tipo das que a pessoa trocou.
func confirmacao(p Previa, mes string, tipos map[string]int64) string {
	c := Confirmacao{SalarioBase: p.SalarioBase, BaseInss: p.BaseInss, BaseFgts: p.BaseFgts, FgtsDoMes: p.FgtsDoMes}
	for _, r := range p.Rubricas {
		tipo := r.Tipo
		if t, ok := tipos[r.Descricao]; ok {
			tipo = t
		}
		c.Rubricas = append(c.Rubricas, RubricaConfirmada{Codigo: r.Codigo, Descricao: r.Descricao, Referencia: r.Referencia,
			Valor: r.Valor, Provento: r.Provento, Tipo: tipo})
	}
	b, _ := json.Marshal(c)
	return strings.Replace(string(b), `"mes":"0001-01-01"`, `"mes":"`+mes+`"`, 1)
}

func (a ambiente) confirmar(empregoId int64, corpo string) testeapoio.Resposta {
	a.t.Helper()
	return a.chamar("POST", fmt.Sprintf("/api/trabalho/empregos/%d/holerites/pdf/confirmar", empregoId), corpo)
}

func (a ambiente) desfazer(empregoId int64, chave string) testeapoio.Resposta {
	a.t.Helper()
	return a.chamar("POST", fmt.Sprintf("/api/trabalho/empregos/%d/holerites/pdf/desfazer/%s", empregoId, chave), "")
}

func fixo(v tipos.Decimal) string { return v.Decimal().StringFixed(2) }

func resumo(p Previa) string {
	var b strings.Builder
	for _, r := range p.Rubricas {
		fmt.Fprintf(&b, "%s=%d/%s ", r.Descricao, r.Tipo, r.Como)
	}
	return strings.TrimSpace(b.String())
}

func temAviso(p Previa, trecho string) bool {
	for _, a := range p.Avisos {
		if strings.Contains(a, trecho) {
			return true
		}
	}
	return false
}

func TestLerConfirmarEDesfazer(t *testing.T) {
	t.Parallel()
	a := novo(t)
	id := a.emprego("5000", `,"cnpj":"12.345.678/0001-95"`)
	p := a.previa(id, ficticios.ViaClassica())
	if p.Mes == nil || p.Mes.String() != "2026-10-01" || p.Existente != nil || len(p.Provas) != 3 {
		t.Fatalf("prévia: %+v", p)
	}
	want := "SALARIO=0/dicionario HORAS EXTRAS 50%=1/dicionario DSR S/ HORAS EXTRAS=4/dicionario INSS=20/dicionario " +
		"IRRF=21/dicionario VALE TRANSPORTE 6%=10/dicionario ASSIST. MEDICA=12/dicionario"
	if got := resumo(p); got != want {
		t.Errorf("rubricas:\n%s\nquero:\n%s", got, want)
	}
	// O CNPJ é o do emprego e o salário base é o cadastrado: sem aviso deles. O INSS e o IRRF lidos não são os da
	// estimativa (que não tem as horas extras): aviso.
	// O líquido também difere, mas não vira aviso: as horas extras e os descontos do mês explicam.
	if temAviso(p, "CNPJ") || temAviso(p, "salário base") || !temAviso(p, "O INSS do PDF (R$ 601,22)") || !temAviso(p, "O IRRF do PDF (R$ 437,45)") ||
		temAviso(p, "Líquido do PDF") || p.Comparacoes[2].Diferenca.Decimal().IsZero() {
		t.Errorf("avisos: %v", p.Avisos)
	}
	estimativa, err := a.holerites.Estimativa(context.Background(), id, *p.Mes)
	if err != nil || estimativa == nil {
		t.Fatal(err)
	}
	if len(p.Comparacoes) != 3 || p.Comparacoes[0].Descricao != "INSS" || p.Comparacoes[0].Lido.Decimal().StringFixed(2) != "601.22" ||
		!p.Comparacoes[2].Calculado.Decimal().Equal(estimativa.Liquido.Decimal()) || p.Comparacoes[2].Lido.Decimal().StringFixed(2) != "3920.42" {
		t.Errorf("comparações: %+v", p.Comparacoes)
	}

	r := a.confirmar(id, confirmacao(p, "2026-10-01", nil))
	r.ConferirStatus(t, http.StatusCreated)
	g := decodificar[Gravado](t, r)
	h := g.Holerite
	c := h.Calculo
	if h.Situacao != holerite.Fechado || h.Origem != holerite.LidoDoPdf || g.Desfazer == "" || c == nil {
		t.Fatalf("gravado: %+v", g)
	}
	if fmt.Sprintln(fixo(c.TotalProventos), fixo(c.TotalDescontos), fixo(c.Liquido), fixo(c.FgtsDoMes), fixo(c.BaseInss),
		fixo(c.SalarioContratual), fixo(c.ValorHora), c.DiasTrabalhados, len(c.Proventos), len(c.Descontos)) !=
		"5409.09 1488.67 3920.42 432.73 5409.09 5000.00 22.73 30 3 4\n" {
		t.Errorf("cálculo: %+v", c)
	}
	// As horas extras e o DSR lidos entram nas médias do 13º e das férias.
	soma, meses, err := a.holerites.VariaveisDoAno(context.Background(), id, 2026)
	if err != nil || soma.Decimal().StringFixed(2) != "409.09" || meses != 1 {
		t.Errorf("variáveis: %s em %d meses (%v)", soma.Decimal().StringFixed(2), meses, err)
	}
	lista := decodificar[[]holerite.Resumo](t, a.chamar("GET", fmt.Sprintf("/api/trabalho/empregos/%d/holerites", id), ""))
	if len(lista) != 1 || lista[0].Origem != holerite.LidoDoPdf {
		t.Errorf("lista: %+v", lista)
	}
	// O mês já tem o lido, fechado: a prévia avisa e o confirmar recusa.
	p2 := a.previa(id, ficticios.ViaClassica())
	if p2.Existente == nil || p2.Existente.Id != h.Id || !temAviso(p2, "está fechado: reabra") {
		t.Errorf("prévia com o mês fechado: %+v", p2)
	}
	r = a.confirmar(id, confirmacao(p2, "2026-10-01", nil))
	r.ConferirStatus(t, http.StatusConflict)
	conferirMensagem(t, r, MensagemFechado)

	// Desfazer: o holerite era novo, então some. A chave vale uma vez só.
	a.desfazer(id, g.Desfazer).ConferirStatus(t, http.StatusNoContent)
	a.chamar("GET", fmt.Sprintf("/api/trabalho/holerites/%d", h.Id), "").ConferirStatus(t, http.StatusNotFound)
	r = a.desfazer(id, g.Desfazer)
	r.ConferirStatus(t, http.StatusNotFound)
	conferirMensagem(t, r, "Não dá mais para desfazer: o prazo passou ou já foi desfeito.")
}

// O lido troca a estimativa aberta do mês, que volta com os lançamentos ao desfazer (ou ao reabrir).
func TestLidoTrocaAEstimativaAberta(t *testing.T) {
	t.Parallel()
	a := novo(t)
	id := a.emprego("5000", `,"cnpj":"12345678000195"`)
	criar := a.chamar("POST", fmt.Sprintf("/api/trabalho/empregos/%d/holerites", id), `{"mes":"2026-10"}`)
	criar.ConferirStatus(t, http.StatusCreated)
	aberto := decodificar[holerite.Holerite](t, criar)
	a.chamar("PUT", fmt.Sprintf("/api/trabalho/holerites/%d", aberto.Id),
		`{"diasUteis":27,"diasDescanso":4,"lancamentos":[{"tipo":1,"quantidade":10,"percentual":50}]}`).ConferirStatus(t, http.StatusOK)
	p := a.previa(id, ficticios.ViaClassica())
	if p.Existente == nil || p.Existente.Situacao != holerite.Aberto || !temAviso(p, "vai ser trocado pelo lido") {
		t.Fatalf("prévia: %+v", p)
	}
	// A estimativa agora tem as horas extras: o INSS calculado é o do holerite aberto, com elas.
	estimativa, err := a.holerites.Estimativa(context.Background(), id, dataDe("2026-10-01"))
	if err != nil || len(p.Comparacoes) != 3 || !p.Comparacoes[0].Calculado.Decimal().Equal(estimativa.Descontos[0].Valor.Decimal()) ||
		estimativa.Descontos[0].Tipo != holerite.TipoInss || len(estimativa.Proventos) != 3 {
		t.Errorf("INSS calculado: %+v, estimativa %+v (%v)", p.Comparacoes, estimativa, err)
	}
	r := a.confirmar(id, confirmacao(p, "2026-10-01", nil))
	r.ConferirStatus(t, http.StatusCreated)
	g := decodificar[Gravado](t, r)
	if g.Holerite.Id != aberto.Id || g.Holerite.Origem != holerite.LidoDoPdf || len(g.Holerite.Lancamentos) != 1 {
		t.Fatalf("gravado: %+v", g.Holerite)
	}
	a.desfazer(id, g.Desfazer).ConferirStatus(t, http.StatusNoContent)
	h := decodificar[holerite.Holerite](t, a.chamar("GET", fmt.Sprintf("/api/trabalho/holerites/%d", aberto.Id), ""))
	if h.Situacao != holerite.Aberto || h.Origem != holerite.Calculado || len(h.Lancamentos) != 1 || h.FechadoEm != nil {
		t.Errorf("depois do desfazer: %+v", h)
	}
	// Reabrir o lido também volta para a estimativa; depois disso, o desfazer não tem mais o que desfazer.
	g = decodificar[Gravado](t, a.confirmar(id, confirmacao(p, "2026-10-01", nil)))
	a.chamar("POST", fmt.Sprintf("/api/trabalho/holerites/%d/reabrir", aberto.Id), "").ConferirStatus(t, http.StatusOK)
	h = decodificar[holerite.Holerite](t, a.chamar("GET", fmt.Sprintf("/api/trabalho/holerites/%d", aberto.Id), ""))
	if h.Origem != holerite.Calculado || h.Situacao != holerite.Aberto {
		t.Errorf("depois de reabrir: %+v", h)
	}
	a.desfazer(id, g.Desfazer).ConferirStatus(t, http.StatusNotFound)
	// A chave de um emprego não desfaz no outro.
	outro := a.emprego("3000", "")
	g = decodificar[Gravado](t, a.confirmar(id, confirmacao(p, "2026-10-01", nil)))
	a.desfazer(outro, g.Desfazer).ConferirStatus(t, http.StatusNotFound)
}

// A rubrica que o leitor não conhece: a pessoa escolhe o tipo uma vez, e no mês seguinte ela vem reconhecida. A
// corrigida também é lembrada; a que volta ao tipo do dicionário é esquecida.
func TestEscolhaDaRubricaLembradaPorEmprego(t *testing.T) {
	t.Parallel()
	a := novo(t)
	id := a.emprego("3200", "")
	outro := a.emprego("3200", "")
	p := a.previa(id, ficticios.CreditosEDebitos()...)
	want := "Salário=0/dicionario H.E. 100%=1/dicionario Reflexo DSR H.E.=4/dicionario I.N.S.S.=20/dicionario " +
		"Seguro de Vida=14/desconhecida Plano Odontológico=12/dicionario"
	if got := resumo(p); got != want {
		t.Fatalf("rubricas:\n%s\nquero:\n%s", got, want)
	}
	escolher := func(tipos map[string]int64) {
		t.Helper()
		r := a.confirmar(id, confirmacao(a.previa(id, ficticios.CreditosEDebitos()...), "2026-10-01", tipos))
		r.ConferirStatus(t, http.StatusCreated)
		a.desfazer(id, decodificar[Gravado](t, r).Desfazer).ConferirStatus(t, http.StatusNoContent)
	}
	escolher(map[string]int64{"Seguro de Vida": holerite.PlanoDeSaude, "Plano Odontológico": holerite.OutroDesconto})
	want = "Salário=0/dicionario H.E. 100%=1/dicionario Reflexo DSR H.E.=4/dicionario I.N.S.S.=20/dicionario " +
		"Seguro de Vida=12/escolha Plano Odontológico=14/escolha"
	if got := resumo(a.previa(id, ficticios.CreditosEDebitos()...)); got != want {
		t.Errorf("depois de escolher:\n%s\nquero:\n%s", got, want)
	}
	// Outro emprego não herda as escolhas.
	if got := resumo(a.previa(outro, ficticios.CreditosEDebitos()...)); !strings.Contains(got, "Seguro de Vida=14/desconhecida") {
		t.Errorf("outro emprego: %s", got)
	}
	escolher(map[string]int64{"Plano Odontológico": holerite.PlanoDeSaude})
	if got := resumo(a.previa(id, ficticios.CreditosEDebitos()...)); !strings.Contains(got, "Seguro de Vida=12/escolha Plano Odontológico=12/dicionario") {
		t.Errorf("de volta ao dicionário: %s", got)
	}
	// Sem código (o estilo "proventos e qtde"), a rubrica é lembrada só pela descrição.
	escolher2 := a.confirmar(id, confirmacao(a.previa(id, ficticios.ProventosEQtde()), "2026-10-01",
		map[string]int64{"Vale Refeição": holerite.OutroDesconto}))
	escolher2.ConferirStatus(t, http.StatusCreated)
	if got := resumo(a.previa(id, ficticios.ProventosEQtde())); !strings.Contains(got, "Vale Refeição=14/escolha") {
		t.Errorf("sem código: %s", got)
	}
}

func TestAvisosDoEmpregoEDoMes(t *testing.T) {
	t.Parallel()
	a := novo(t)
	semCnpj := a.emprego("5000", "")
	comOutro := a.emprego("5000", `,"cnpj":"99.999.999/0001-99"`)
	doPdf := a.emprego("5000", `,"cnpj":"98.765.432/0001-10"`)
	estagio := a.emprego("1500", `,"regime":2`)
	recente := a.chamar("POST", "/api/trabalho/empregos", `{"empregador":"Nova","admissao":"2026-11-03","salarioBruto":5000}`)
	recente.ConferirStatus(t, http.StatusCreated)
	casos := []struct {
		nome      string
		empregoId int64
		paginas   [][]ficticios.Texto
		avisos    []string
		semAviso  []string
	}{
		{"emprego sem CNPJ", semCnpj, [][]ficticios.Texto{ficticios.ViaClassica()},
			[]string{"Este emprego não tem CNPJ cadastrado; o do PDF é 12.345.678/0001-95 (EMPRESA FICTÍCIA DE TESTES LTDA)"}, nil},
		{"CNPJ de outro emprego", comOutro, [][]ficticios.Texto{ficticios.ProventosEQtde()},
			[]string{`O CNPJ do PDF (98.765.432/0001-10) é o do emprego "Empresa Fictícia", não deste.`}, nil},
		{"CNPJ de nenhum emprego", comOutro, [][]ficticios.Texto{ficticios.ViaClassica()},
			[]string{"O CNPJ do PDF (12.345.678/0001-95, EMPRESA FICTÍCIA DE TESTES LTDA) não é o deste emprego (99.999.999/0001-99)"}, nil},
		{"salário diferente", doPdf, [][]ficticios.Texto{ficticios.ProventosEQtde()},
			[]string{"O salário base do PDF (R$ 4.000,00) é diferente do cadastrado para outubro de 2026 (R$ 5.000,00). Se foi aumento, registre em Trabalho › Salário, a partir de outubro de 2026."},
			[]string{"CNPJ"}},
		{"estágio: sem lido × calculado", estagio, [][]ficticios.Texto{ficticios.ViaClassica()}, nil, []string{"INSS do PDF", "IRRF do PDF"}},
		{"mês antes da admissão", decodificar[emprego.Emprego](t, recente).Id, [][]ficticios.Texto{ficticios.ViaClassica()},
			[]string{"Outubro de 2026 é de antes da admissão ou depois da saída deste emprego"}, nil},
		{"sem o mês nem os totais", semCnpj, [][]ficticios.Texto{ficticios.Pagina(
			ficticios.Linha(0, 30, "Descrição", 290, "Proventos", 400, "Descontos"),
			ficticios.Linha(0, 30, "Salário", 295, "2.000,00"))},
			[]string{"Não achei o mês do holerite no PDF", "Não achei os totais no PDF"}, []string{"CNPJ"}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			p := a.previa(c.empregoId, c.paginas...)
			for _, aviso := range c.avisos {
				if !temAviso(p, aviso) {
					t.Errorf("falta o aviso %q em %v", aviso, p.Avisos)
				}
			}
			for _, aviso := range c.semAviso {
				if temAviso(p, aviso) {
					t.Errorf("não devia avisar %q: %v", aviso, p.Avisos)
				}
			}
		})
	}
	if p := a.previa(estagio, ficticios.ViaClassica()); len(p.Comparacoes) != 0 {
		t.Errorf("estágio com comparações: %+v", p.Comparacoes)
	}
}

// O PDF que não dá para ler: mensagem clara, e nada gravado.
func TestPDFRecusado(t *testing.T) {
	t.Parallel()
	a := novo(t)
	id := a.emprego("5000", "")
	comSenha := ficticios.GerarPDF([][]ficticios.Texto{ficticios.ViaClassica()}, "12345678900")
	casos := []struct {
		nome     string
		pdf      []byte
		senha    string
		mensagem string
		motivo   string
	}{
		{"com senha", comSenha, "", MensagemSenha, MotivoSenha},
		{"senha errada", comSenha, "000", MensagemSenhaErrada, MotivoSenha},
		{"escaneado", ficticios.GerarPDF([][]ficticios.Texto{{}}, ""), "", MensagemSemTexto, MotivoSemTexto},
		{"sem a tabela", ficticios.GerarPDF([][]ficticios.Texto{ficticios.Pagina(ficticios.Linha(0, 30, "Informe de rendimentos"))}, ""),
			"", MensagemSemTabela, MotivoSemTexto},
		{"não é PDF", []byte("planilha"), "", MensagemNaoEPDF, ""},
		{"grande", append([]byte("%PDF-1.4\n"), make([]byte, 700<<10)...), "", MensagemGrande, ""},
		{"corrompido", []byte("%PDF-1.4\nquebrado"), "", MensagemIlegivel, ""},
	}
	for _, c := range casos {
		r := a.ler(id, c.pdf, c.senha)
		r.ConferirStatus(t, http.StatusBadRequest)
		corpo := decodificar[map[string]string](t, r)
		if corpo["mensagem"] != c.mensagem || corpo["motivo"] != c.motivo {
			t.Errorf("%s: %v", c.nome, corpo)
		}
	}
	if r := a.ler(id, comSenha, "12345678900"); r.Status != http.StatusOK {
		t.Errorf("com a senha certa: %d %s", r.Status, r.Corpo)
	}
	lista := decodificar[[]holerite.Resumo](t, a.chamar("GET", fmt.Sprintf("/api/trabalho/empregos/%d/holerites", id), ""))
	if len(lista) != 0 {
		t.Errorf("gravou: %+v", lista)
	}
	a.ler(999, comSenha, "").ConferirStatus(t, http.StatusNotFound)
	a.chamar("POST", "/api/trabalho/empregos/x/holerites/pdf", "{}").ConferirStatus(t, http.StatusNotFound)
	a.chamar("POST", fmt.Sprintf("/api/trabalho/empregos/%d/holerites/pdf", id), `{"arquivo":1}`).ConferirStatus(t, http.StatusBadRequest)
	a.chamar("POST", fmt.Sprintf("/api/trabalho/empregos/%d/holerites/pdf", id), `nada`).ConferirStatus(t, http.StatusBadRequest)
}

func TestConfirmarRecusa(t *testing.T) {
	t.Parallel()
	a := novo(t)
	id := a.emprego("5000", "")
	linha := func(campos string) string {
		return `{"mes":"2026-10-01","rubricas":[{"descricao":"Salário","valor":100,"tipo":0` + campos + `}]}`
	}
	casos := []struct {
		nome, corpo, mensagem string
	}{
		{"sem linhas", `{"mes":"2026-10-01","rubricas":[]}`, "O holerite precisa de pelo menos uma linha (e vai até 100)."},
		{"tipo que não existe", linha(`,"tipo":5`), "Escolha o tipo de cada linha."},
		{"sem descrição", linha(`,"descricao":"  "`), "A descrição de cada linha vai de 1 a 100 caracteres."},
		{"descrição longa", linha(`,"descricao":"` + strings.Repeat("a", 101) + `"`), "A descrição de cada linha vai de 1 a 100 caracteres."},
		{"referência longa", linha(`,"referencia":"` + strings.Repeat("1", 31) + `"`), "O código vai até 20 caracteres, e a referência, até 30."},
		{"código longo", linha(`,"codigo":"` + strings.Repeat("1", 21) + `"`), "O código vai até 20 caracteres, e a referência, até 30."},
		{"valor zero", linha(`,"valor":0`), "O valor de cada linha vai de R$ 0,01 a R$ 9.999.999,99, em centavos."},
		{"valor negativo", linha(`,"valor":-1`), "O valor de cada linha vai de R$ 0,01 a R$ 9.999.999,99, em centavos."},
		{"valor grande", linha(`,"valor":10000000`), "O valor de cada linha vai de R$ 0,01 a R$ 9.999.999,99, em centavos."},
		{"meio centavo", linha(`,"valor":1.005`), "O valor de cada linha vai de R$ 0,01 a R$ 9.999.999,99, em centavos."},
		{"mês antes da admissão", `{"mes":"2020-01-01","rubricas":[{"descricao":"Salário","valor":100,"tipo":0}]}`,
			"O mês do holerite é de antes da admissão ou depois da saída deste emprego."},
		{"JSON quebrado", `{"mes":`, MensagemJSONInvalido},
	}
	for _, c := range casos {
		r := a.confirmar(id, c.corpo)
		r.ConferirStatus(t, http.StatusBadRequest)
		conferirMensagem(t, r, c.mensagem)
	}
	muitas := make([]string, 101)
	for i := range muitas {
		muitas[i] = `{"descricao":"Salário","valor":1,"tipo":0}`
	}
	r := a.confirmar(id, `{"mes":"2026-10-01","rubricas":[`+strings.Join(muitas, ",")+`]}`)
	r.ConferirStatus(t, http.StatusBadRequest)
	a.confirmar(id, `{"mes":"2026-09-01","rubricas":[`+strings.Join(muitas[1:], ",")+`]}`).ConferirStatus(t, http.StatusCreated)
	a.confirmar(999, linha("")).ConferirStatus(t, http.StatusNotFound)
	a.chamar("POST", "/api/trabalho/empregos/x/holerites/pdf/confirmar", linha("")).ConferirStatus(t, http.StatusNotFound)
	a.chamar("POST", "/api/trabalho/empregos/x/holerites/pdf/desfazer/abc", "").ConferirStatus(t, http.StatusNotFound)
	// Com o limite certinho, grava.
	a.confirmar(id, `{"mes":"2026-10-01","rubricas":[{"descricao":"`+strings.Repeat("a", 100)+`","referencia":"`+
		strings.Repeat("1", 30)+`","codigo":"`+strings.Repeat("1", 20)+`","valor":9999999.99,"tipo":0}]}`).ConferirStatus(t, http.StatusCreated)
}

// O FGTS do mês: o do PDF, ou 8% da base do FGTS, ou 8% dos proventos; nada no estágio. O salário contratual: o do PDF,
// ou o cadastrado, ou a soma das linhas de salário.
func TestMontar(t *testing.T) {
	t.Parallel()
	dec := func(s string) *tiposDecimal { v := decimalDe(s); return &v }
	clt := emprego.Emprego{Admissao: dataDe("2024-03-11"), Regime: emprego.RegimeClt, JornadaMensalHoras: 200,
		Salarios: []emprego.Salario{{Inicio: dataDe("2024-03-01"), SalarioBruto: decimalDe("4000")}}}
	semSalario := clt
	semSalario.Salarios = nil
	estagio := clt
	estagio.Regime = emprego.RegimeEstagio
	linhas := []RubricaConfirmada{{Descricao: "Salário", Valor: decimalDe("4100"), Tipo: holerite.TipoSalario},
		{Descricao: "Salário (diferença)", Valor: decimalDe("100"), Tipo: holerite.TipoSalario},
		{Descricao: "Bônus", Valor: decimalDe("800"), Tipo: holerite.OutroProvento},
		{Descricao: "INSS", Valor: decimalDe("500"), Tipo: holerite.TipoInss}}
	casos := []struct {
		nome      string
		e         emprego.Emprego
		c         Confirmacao
		fgts, sal string
		valorHora string
	}{
		{"FGTS do PDF e salário do PDF", clt, Confirmacao{FgtsDoMes: dec("401.11"), BaseFgts: dec("1000"), SalarioBase: dec("4100")}, "401.11", "4100.00", "20.50"},
		{"8% da base do FGTS", clt, Confirmacao{BaseFgts: dec("5000.01")}, "400.00", "4000.00", "20.00"},
		{"8% dos proventos", clt, Confirmacao{}, "400.00", "4000.00", "20.00"},
		{"estágio", estagio, Confirmacao{FgtsDoMes: dec("10")}, "0.00", "4000.00", "20.00"},
		{"sem salário cadastrado: a soma das linhas", semSalario, Confirmacao{}, "400.00", "4200.00", "21.00"},
	}
	for _, c := range casos {
		c.c.Mes, c.c.Rubricas = dataDe("2026-10-15"), linhas
		calc, err := montar(c.e, c.c)
		if err != nil {
			t.Fatal(err)
		}
		got := fmt.Sprintln(fixo(calc.FgtsDoMes), fixo(calc.SalarioContratual), fixo(calc.ValorHora), calc.Mes, fixo(calc.TotalProventos),
			fixo(calc.TotalDescontos), fixo(calc.Liquido))
		want := fmt.Sprintln(c.fgts, c.sal, c.valorHora, "2026-10-01", "5000.00", "500.00", "4500.00")
		if got != want {
			t.Errorf("%s: %s, quero %s", c.nome, got, want)
		}
	}
}

func TestEmReaisECnpj(t *testing.T) {
	casos := map[string]string{"0": "R$ 0,00", "1234.5": "R$ 1.234,50", "-12": "-R$ 12,00", "1234567.891": "R$ 1.234.567,89", "999": "R$ 999,00",
		"1000": "R$ 1.000,00"}
	for v, want := range casos {
		if got := emReais(decimalDe(v).Decimal()); got != want {
			t.Errorf("emReais(%s) = %q, quero %q", v, got, want)
		}
	}
	if got := cnpjFormatado("12345678000195"); got != "12.345.678/0001-95" {
		t.Errorf("cnpj: %s", got)
	}
}
