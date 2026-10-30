package selic

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// fonteFalsa é o Banco Central dos testes do serviço: responde pelo começo da faixa pedida e anota os pedidos.
type fonteFalsa struct {
	taxas    map[string][]Taxa
	errosEm  map[string]error
	meta     *string
	erroMeta error
	pedidos  []string
	metas    int
}

func (f *fonteFalsa) BuscarTaxas(_ context.Context, de, ate tipos.Data) ([]Taxa, error) {
	f.pedidos = append(f.pedidos, de.String()+" a "+ate.String())
	if err := f.errosEm[de.String()]; err != nil {
		return nil, err
	}
	return f.taxas[de.String()], nil
}

func (f *fonteFalsa) BuscarMeta(context.Context) (*string, error) {
	f.metas++
	return f.meta, f.erroMeta
}

// servicoDeTeste é o serviço num banco vazio, com hoje em 30/10/2026, às 15h em Brasília (18h em UTC).
func servicoDeTeste(t *testing.T, f *fonteFalsa) (*Servico, *sql.DB, *testeapoio.Fabrica, *bytes.Buffer) {
	t.Helper()
	db := testeapoio.NovoBancoVazio(t)
	r := testeapoio.AgoraEmBrasilia(2026, 10, 30, 15, 0)
	var saida bytes.Buffer
	return NovoServico(db, f, r, slog.New(slog.NewTextHandler(&saida, nil))), db, testeapoio.NovaFabrica(t, db, r), &saida
}

func outubro(d int) tipos.Data { return testeapoio.Dia(2026, time.October, d) }

func guardarTaxas(t *testing.T, db *sql.DB, datas ...string) {
	t.Helper()
	for _, d := range datas {
		if _, err := db.Exec(`INSERT INTO TaxaSelicDiaria (Data, Taxa) VALUES (?, '0.05')`, d); err != nil {
			t.Fatal(err)
		}
	}
}

func taxasGuardadas(t *testing.T, db *sql.DB) string {
	t.Helper()
	linhas, err := db.Query(`SELECT Data, Taxa FROM TaxaSelicDiaria ORDER BY Data`)
	if err != nil {
		t.Fatal(err)
	}
	defer linhas.Close()
	var lista []string
	for linhas.Next() {
		var d, v string
		if err := linhas.Scan(&d, &v); err != nil {
			t.Fatal(err)
		}
		lista = append(lista, d+" "+v)
	}
	return strings.Join(lista, ", ")
}

func configuracaoGuardada(t *testing.T, db *sql.DB, chave string) string {
	t.Helper()
	var v sql.NullString
	if err := db.QueryRow(`SELECT Valor FROM Configuracao WHERE Chave = ?`, chave).Scan(&v); err != nil && !errors.Is(err, sql.ErrNoRows) {
		t.Fatal(err)
	}
	return v.String
}

func umaTaxa(dia tipos.Data, valor string) Taxa {
	return Taxa{Data: dia, Taxa: testeapoio.Valor(valor)}
}

const (
	consultaAgora = "2026-10-30T18:00:00.0000000Z"
	errFora       = "Não foi possível atualizar a Selic no Banco Central."
)

// Regra: o que falta buscar: sem nada guardado, de início a hoje; o começo, só se faltar mais de 5 dias; o fim, se a
// última taxa for de antes de ontem e a última consulta tiver mais de 6 horas. O início no futuro vale hoje. Sem nada a
// buscar, a hora da consulta não muda.
func TestFaixasQueFaltam(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome     string
		guardar  []string
		ultima   string
		inicio   tipos.Data
		esperado []string
	}{
		{nome: "nada guardado", inicio: outubro(1), esperado: []string{"2026-10-01 a 2026-10-30"}},
		{nome: "início no futuro", inicio: testeapoio.Dia(2026, time.November, 5), esperado: []string{"2026-10-30 a 2026-10-30"}},
		{nome: "o começo a 6 dias", guardar: []string{"2026-10-10", "2026-10-29"}, inicio: outubro(4), esperado: []string{"2026-10-04 a 2026-10-09"}},
		{nome: "o começo a 5 dias não busca", guardar: []string{"2026-10-10", "2026-10-29"}, inicio: outubro(5)},
		{nome: "o fim: a última de anteontem", guardar: []string{"2026-10-28"}, inicio: outubro(28), esperado: []string{"2026-10-29 a 2026-10-30"}},
		{nome: "a última de ontem não busca", guardar: []string{"2026-10-29"}, inicio: outubro(29)},
		{nome: "consulta com mais de 6 horas", guardar: []string{"2026-10-28"}, ultima: "2026-10-30T11:59:59.0000000Z", inicio: outubro(28),
			esperado: []string{"2026-10-29 a 2026-10-30"}},
		{nome: "consulta com 6 horas exatas não busca", guardar: []string{"2026-10-28"}, ultima: "2026-10-30T12:00:00.0000000Z", inicio: outubro(28)},
		{nome: "o começo e o fim", guardar: []string{"2026-10-20"}, inicio: outubro(1), esperado: []string{"2026-10-01 a 2026-10-19", "2026-10-21 a 2026-10-30"}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			f := &fonteFalsa{}
			s, db, fab, _ := servicoDeTeste(t, f)
			guardarTaxas(t, db, c.guardar...)
			if c.ultima != "" {
				fab.Configuracao(chaveUltimaConsulta, c.ultima)
			}
			if _, err := s.ObterSerie(t.Context(), c.inicio); err != nil {
				t.Fatal(err)
			}
			if strings.Join(f.pedidos, ",") != strings.Join(c.esperado, ",") {
				t.Errorf("pedidos %v, esperado %v", f.pedidos, c.esperado)
			}
			esperada := c.ultima
			if len(c.esperado) > 0 {
				esperada = consultaAgora
			}
			if consulta := configuracaoGuardada(t, db, chaveUltimaConsulta); consulta != esperada {
				t.Errorf("última consulta %q, esperada %q", consulta, esperada)
			}
		})
	}
}

// Regra: a API aceita até 10 anos por consulta: a faixa longa vai em partes de 9 anos.
func TestFaixaLongaEmPartes(t *testing.T) {
	t.Parallel()
	f := &fonteFalsa{}
	s, _, _, _ := servicoDeTeste(t, f)
	if _, err := s.ObterSerie(t.Context(), testeapoio.Dia(2000, time.January, 1)); err != nil {
		t.Fatal(err)
	}
	esperado := "2000-01-01 a 2008-12-31,2009-01-01 a 2017-12-31,2018-01-01 a 2026-10-30"
	if obtido := strings.Join(f.pedidos, ","); obtido != esperado {
		t.Errorf("pedidos %s", obtido)
	}
}

// Regra: grava o que veio, com a taxa já guardada intacta e a repetida na resposta uma vez só; a série vem do banco e
// começa no início pedido.
func TestObterSerieGrava(t *testing.T) {
	t.Parallel()
	f := &fonteFalsa{taxas: map[string][]Taxa{"2026-10-01": {
		umaTaxa(outubro(1), "0.05"), umaTaxa(outubro(2), "0.04"), umaTaxa(outubro(2), "0.09"), umaTaxa(outubro(5), "0.1"), umaTaxa(outubro(6), "0.3"),
	}}}
	s, db, _, _ := servicoDeTeste(t, f)
	guardarTaxas(t, db, "2026-10-06")
	if err := s.baixar(t.Context(), outubro(1), outubro(6)); err != nil {
		t.Fatal(err)
	}
	esperado := "2026-10-01 0.05, 2026-10-02 0.04, 2026-10-05 0.1, 2026-10-06 0.05"
	if obtido := taxasGuardadas(t, db); obtido != esperado {
		t.Errorf("guardadas %s", obtido)
	}
	serie, err := s.ObterSerie(t.Context(), outubro(2))
	if err != nil {
		t.Fatal(err)
	}
	if u := serie.UltimaData(); u == nil || !u.Igual(outubro(6)) {
		t.Errorf("última data %v", u)
	}
	// Só as taxas desde o início entram: a de 01/10 não pesa no fator.
	if fator, err := serie.Fator(outubro(1), outubro(3)); err != nil || fator.String() != "1.0004" {
		t.Errorf("fator %s, %v", fator, err)
	}
}

// Regra: sem internet, segue com o guardado e avisa; o que veio antes do erro fica, e a hora da consulta não muda, para
// tentar de novo na próxima vez.
func TestBancoCentralFalhaNoMeio(t *testing.T) {
	t.Parallel()
	f := &fonteFalsa{
		taxas:   map[string][]Taxa{"2009-01-01": {umaTaxa(testeapoio.Dia(2009, time.January, 2), "0.03")}},
		errosEm: map[string]error{"2018-01-01": errors.New("sem internet")},
	}
	s, db, _, saida := servicoDeTeste(t, f)
	serie, err := s.ObterSerie(t.Context(), testeapoio.Dia(2009, time.January, 1))
	if err != nil {
		t.Fatal(err)
	}
	if serie.Vazia() || taxasGuardadas(t, db) != "2009-01-02 0.03" {
		t.Errorf("guardadas %s", taxasGuardadas(t, db))
	}
	if configuracaoGuardada(t, db, chaveUltimaConsulta) != "" {
		t.Error("anotou a consulta que falhou")
	}
	if !strings.Contains(saida.String(), errFora) || !strings.Contains(saida.String(), "origem=Investimentos") {
		t.Errorf("log %q", saida.String())
	}
}

// Regra: a resposta que não dá para gravar não deixa a parte pela metade, e o erro vira aviso como o da internet.
func TestGravarFalhaDesfazAParte(t *testing.T) {
	t.Parallel()
	f := &fonteFalsa{taxas: map[string][]Taxa{"2026-10-01": {umaTaxa(outubro(1), "0.05"), umaTaxa(outubro(2), "0.04")}}}
	s, db, _, saida := servicoDeTeste(t, f)
	if _, err := db.Exec(`CREATE TRIGGER nao_grava BEFORE INSERT ON TaxaSelicDiaria WHEN NEW.Data = '2026-10-02'
		BEGIN SELECT RAISE(ABORT, 'não grava'); END`); err != nil {
		t.Fatal(err)
	}
	serie, err := s.ObterSerie(t.Context(), outubro(1))
	if err != nil {
		t.Fatal(err)
	}
	if !serie.Vazia() || taxasGuardadas(t, db) != "" || !strings.Contains(saida.String(), errFora) {
		t.Errorf("guardadas %q, log %q", taxasGuardadas(t, db), saida.String())
	}
}

// Regra: pedido cancelado não vira aviso: sobe como erro.
func TestObterSerieCancelado(t *testing.T) {
	t.Parallel()
	ctx, cancelar := context.WithCancel(t.Context())
	f := &fonteFalsa{errosEm: map[string]error{"2026-10-01": context.Canceled}}
	s, _, _, saida := servicoDeTeste(t, f)
	cancelar()
	if _, err := s.ObterSerie(ctx, outubro(1)); !errors.Is(err, context.Canceled) {
		t.Errorf("erro %v", err)
	}
	if strings.Contains(saida.String(), errFora) {
		t.Errorf("log %q", saida.String())
	}
}

// Regra: a meta: dentro de 12 horas, a guardada; depois, a do Banco Central, gravada com a hora; sem internet ou sem
// número na resposta, a guardada, mesmo velha, e nada muda no banco; a guardada que não é número é nil.
func TestMetaAtual(t *testing.T) {
	t.Parallel()
	meta := func(s string) *string { return &s }
	casos := []struct {
		nome        string
		guardada    string
		consultada  string
		resposta    *string
		erro        error
		esperada    string
		consultas   int
		gravada     string
		gravadaEm   string
		comAviso    bool
		semGravacao bool
	}{
		{nome: "sem nada guardado", resposta: meta("15.00"), esperada: "15.00", consultas: 1, gravada: "15.00", gravadaEm: consultaAgora},
		{nome: "guardada há menos de 12 horas", guardada: "14.75", consultada: "2026-10-30T06:00:01.0000000Z", esperada: "14.75", gravada: "14.75",
			gravadaEm: "2026-10-30T06:00:01.0000000Z"},
		{nome: "guardada há 12 horas exatas", guardada: "14.75", consultada: "2026-10-30T06:00:00.0000000Z", resposta: meta("15.00"), esperada: "15.00",
			consultas: 1, gravada: "15.00", gravadaEm: consultaAgora},
		{nome: "sem a hora da consulta", guardada: "14.75", resposta: meta(" 1,015.50 "), esperada: "1015.50", consultas: 1, gravada: "1015.50", gravadaEm: consultaAgora},
		{nome: "a hora sem a meta", consultada: "2026-10-30T17:00:00.0000000Z", resposta: meta("15.00"), esperada: "15.00", consultas: 1, gravada: "15.00", gravadaEm: consultaAgora},
		{nome: "guardada que não é número, ainda válida", guardada: "abc", consultada: "2026-10-30T17:00:00.0000000Z", gravada: "abc", gravadaEm: "2026-10-30T17:00:00.0000000Z"},
		{nome: "sem internet: a velha", guardada: "14.75", consultada: "2026-10-01T06:00:00.0000000Z", erro: errors.New("sem internet"), esperada: "14.75", consultas: 1,
			gravada: "14.75", gravadaEm: "2026-10-01T06:00:00.0000000Z", comAviso: true},
		{nome: "sem internet e sem nada guardado", erro: errors.New("sem internet"), consultas: 1, comAviso: true},
		{nome: "resposta sem número: a velha", guardada: "14.75", consultada: "2026-10-01T06:00:00.0000000Z", resposta: meta("abc"), esperada: "14.75", consultas: 1,
			gravada: "14.75", gravadaEm: "2026-10-01T06:00:00.0000000Z"},
		{nome: "resposta sem nada e sem nada guardado", consultas: 1},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			f := &fonteFalsa{meta: c.resposta, erroMeta: c.erro}
			s, db, fab, saida := servicoDeTeste(t, f)
			if c.guardada != "" {
				fab.Configuracao(chaveMeta, c.guardada)
			}
			if c.consultada != "" {
				fab.Configuracao(chaveMetaEm, c.consultada)
			}
			obtida, err := s.MetaAtual(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			texto := ""
			if obtida != nil {
				texto = obtida.String()
			}
			if texto != c.esperada || f.metas != c.consultas {
				t.Errorf("meta %q com %d consultas, esperada %q com %d", texto, f.metas, c.esperada, c.consultas)
			}
			if gravada := configuracaoGuardada(t, db, chaveMeta); gravada != c.gravada {
				t.Errorf("gravada %q, esperada %q", gravada, c.gravada)
			}
			gravadaEm := c.gravadaEm
			if c.gravadaEm == "" {
				gravadaEm = c.consultada
			}
			if em := configuracaoGuardada(t, db, chaveMetaEm); em != gravadaEm {
				t.Errorf("gravada em %q, esperado %q", em, gravadaEm)
			}
			if strings.Contains(saida.String(), "meta Selic") != c.comAviso {
				t.Errorf("log %q", saida.String())
			}
		})
	}
}

// Regra: a meta nova que não dá para gravar vira aviso, como no C#, e vale a guardada.
func TestMetaQueNaoGrava(t *testing.T) {
	t.Parallel()
	resposta := "15.00"
	s, db, fab, saida := servicoDeTeste(t, &fonteFalsa{meta: &resposta})
	fab.Configuracao(chaveMeta, "14.75")
	if _, err := db.Exec(`CREATE TRIGGER nao_grava BEFORE UPDATE ON Configuracao BEGIN SELECT RAISE(ABORT, 'não grava'); END`); err != nil {
		t.Fatal(err)
	}
	obtida, err := s.MetaAtual(t.Context())
	if err != nil || obtida == nil || obtida.String() != "14.75" {
		t.Errorf("meta %v, %v", obtida, err)
	}
	if !strings.Contains(saida.String(), "meta Selic") {
		t.Errorf("log %q", saida.String())
	}
}

func TestMetaCancelada(t *testing.T) {
	t.Parallel()
	ctx, cancelar := context.WithCancel(t.Context())
	s, _, _, saida := servicoDeTeste(t, &fonteFalsa{erroMeta: context.Canceled})
	cancelar()
	if _, err := s.MetaAtual(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("erro %v", err)
	}
	if strings.Contains(saida.String(), "meta Selic") {
		t.Errorf("log %q", saida.String())
	}
}

// Regra: erro do banco fora da atualização sobe: quem pediu recebe o 500.
func TestErroDoBanco(t *testing.T) {
	t.Parallel()
	s, db, _, _ := servicoDeTeste(t, &fonteFalsa{})
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ObterSerie(t.Context(), outubro(1)); err == nil {
		t.Error("ObterSerie sem erro")
	}
	if _, err := s.MetaAtual(t.Context()); err == nil {
		t.Error("MetaAtual sem erro")
	}
}
