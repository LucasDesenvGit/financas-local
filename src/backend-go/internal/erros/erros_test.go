package erros

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

func texto(s string) *string { return &s }

// Regra: o Sanitizador.Limpar, padrão por padrão e na mesma ordem do .NET.
func TestLimpar(t *testing.T) {
	casos := []struct{ entrada, saida string }{
		{"", ""},
		{"nada a tirar", "nada a tirar"},
		{"GET https://brapi.dev/api/quote/PETR4?token=abc&x=1 falhou", "GET https://brapi.dev/api/quote/PETR4?[removido] falhou"},
		{"HTTPS://EXEMPLO.COM/a?b=c#frag", "HTTPS://EXEMPLO.COM/a?[removido]#frag"},
		{"client_secret=xyz;outra", "client_secret=[removido];outra"},
		{`{"clientSecret": "xyz"}`, `{"clientSecret": "[removido]"}`},
		{`"api_key":"k-1"`, `"api_key":"[removido]"`},
		{"(senha: segredo1)", "(senha: [removido])"},                         // #396: o ")" que fecha fica
		{"[token=abc] {secret=x}", "[token=[removido]] {secret=[removido]}"}, // #396: "]" e "}" também
		{"token=(abc)", "token=[removido])"},                                 // o "(" que abre não para o valor
		{"PASSWORD = abc def", "PASSWORD = [removido] def"},
		{"Bearer abc.def-123==", "Bearer [removido]"},
		{"Authorization: Bearer abc.def-123", "Authorization: Bearer [removido]"}, // #396: antes o token ficava
		{"authorization=bearer   abc", "authorization=bearer   [removido]"},
		{`"Authorization": "Bearer abc"`, `"Authorization": "Bearer [removido]"`},
		{"Authorization: Bearer", "Authorization: [removido]"}, // sem token depois: a palavra é o valor
		{"Cookie: Bearerzinho=1", "Cookie: [removido]"},        // "Bearer" só conta seguido de espaço
		{"eyJhbGciOi.eyJzdWIiOi.assinatura", "[removido]"},
		{"CPF 123.456.789-09 e 12345678909", "CPF [removido] e [removido]"},
		{"a123.456.789-09", "a123.456.789-09"}, // sem fronteira antes: não é CPF
		{"R$ 1.234,56 e R$-10 e R$5", "R$ [removido] e R$ [removido] e R$ [removido]"},
		{"valor R$ 100,5", "valor R$ [removido]"}, // espaço não separável, que o \s do .NET pega
		{"token = abc", "token = [removido]"},
	}
	for _, c := range casos {
		if obtido := Limpar(c.entrada); obtido != c.saida {
			t.Errorf("Limpar(%q) = %q, esperado %q", c.entrada, obtido, c.saida)
		}
	}
}

// Regra: cortar no tamanho da coluna em caracteres UTF-16, com "…" no fim; nil fica nil.
func TestLimparECortar(t *testing.T) {
	if LimparECortar(nil, 10) != nil {
		t.Error("nil deveria ficar nil")
	}
	casos := []struct {
		entrada string
		maximo  int
		saida   string
	}{
		{"abc", 3, "abc"},
		{"abcd", 3, "ab…"},
		{"😀😀", 4, "😀😀"},  // 4 unidades UTF-16
		{"😀😀😀", 4, "😀…"}, // corta sem partir o emoji
		{"CPF 123.456.789-09", 8, "CPF [re…"},
	}
	for _, c := range casos {
		if obtido := *LimparECortar(texto(c.entrada), c.maximo); obtido != c.saida {
			t.Errorf("LimparECortar(%q, %d) = %q, esperado %q", c.entrada, c.maximo, obtido, c.saida)
		}
	}
}

// Regra: a assinatura é a do .NET (os dois gabaritos foram calculados pelo .NET no contrato 17-erros).
func TestAssinaturaIgualAoDotnet(t *testing.T) {
	casos := []struct {
		categoria, mensagem string
		detalhe             *string
		esperada            string
	}{
		{"Financas.Go.Sistema", "relógio de teste: a hora começa no instante dado, e não na do computador (-inicio-do-relogio) (inicio=2026-10-15T01:00:01.7883336Z)",
			nil, "51A504261BDC48635660D44088910FD0"},
		{"Front-end", "TypeError: x is undefined (token=[removido], senha: [removido] CPF [removido], valor R$ [removido]",
			texto("Error: falhou\n    at https://127.0.0.1:5443/main.js?[removido]\n    Authorization: [removido] abc.def-123"),
			"05FD2121643646EAF82F6DFD716A33BC"},
	}
	for _, c := range casos {
		if obtida := Assinatura(c.categoria, c.mensagem, c.detalhe); obtida != c.esperada {
			t.Errorf("Assinatura(%q) = %s, esperada %s", c.categoria, obtida, c.esperada)
		}
	}
	// O tipo é a primeira linha até o ":": a mesma exceção com outra mensagem no detalhe agrupa junto.
	a := Assinatura("c", "m", texto("*fs.PathError: open a: no such file\n   cadeia: x"))
	b := Assinatura("c", "m", texto("*fs.PathError: open b: permission denied"))
	if a != b || a == Assinatura("c", "m", nil) {
		t.Errorf("o tipo do erro não entrou certo: %s %s", a, b)
	}
}

// Regra: a fila descarta quando cheia; o front-end tem 30 por hora, em janela que anda.
func TestFila(t *testing.T) {
	f := NovaFila()
	for i := range tamanhoFila {
		if !f.Registrar(Registro{Mensagem: fmt.Sprint(i)}) {
			t.Fatalf("recusou o %dº com a fila ainda com espaço", i+1)
		}
	}
	if f.Registrar(Registro{}) {
		t.Error("a fila cheia aceitou mais um")
	}

	f = NovaFila()
	inicio := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	for i := range maximoFrontEndPorHora {
		if !f.RegistrarDoFrontEnd(Registro{Em: inicio.Add(time.Duration(i) * time.Second)}) {
			t.Fatalf("recusou o %dº da hora", i+1)
		}
	}
	if f.RegistrarDoFrontEnd(Registro{Em: inicio.Add(59 * time.Minute)}) {
		t.Error("aceitou o 31º na mesma hora")
	}
	if !f.RegistrarDoFrontEnd(Registro{Em: inicio.Add(time.Hour + time.Second)}) {
		t.Error("depois de uma hora do primeiro, deveria aceitar de novo")
	}
}

// capturar devolve um logger com o Handler e uma função que tira o que ele pôs na fila.
func capturar(t *testing.T) (*slog.Logger, *[]string, func() []Registro) {
	t.Helper()
	fila := NovaFila()
	var proximo []string
	h := NovoHandler(gravadorDeTexto{saida: &proximo, nivel: slog.LevelInfo}, fila, testeapoio.RelogioDosTestes())
	return slog.New(h), &proximo, func() []Registro {
		var lidos []Registro
		for {
			select {
			case r := <-fila.canal:
				lidos = append(lidos, r)
			default:
				return lidos
			}
		}
	}
}

// gravadorDeTexto é um slog.Handler mínimo que guarda a mensagem: o "próximo" do Handler.
type gravadorDeTexto struct {
	saida *[]string
	nivel slog.Level
}

func (g gravadorDeTexto) Enabled(_ context.Context, n slog.Level) bool { return n >= g.nivel }
func (g gravadorDeTexto) Handle(_ context.Context, r slog.Record) error {
	*g.saida = append(*g.saida, r.Message)
	return nil
}
func (g gravadorDeTexto) WithAttrs([]slog.Attr) slog.Handler { return g }
func (g gravadorDeTexto) WithGroup(string) slog.Handler      { return g }

// Regra: dos avisos para cima vai para a fila, com a origem, a categoria e a mensagem como o .NET monta as do Go;
// tudo continua indo para o log de sempre.
func TestHandler(t *testing.T) {
	log, proximo, lidos := capturar(t)
	log.Info("só informação")
	log.Warn("relógio de teste", "inicio", "2026-10-15T01:00:00Z", "origem", "Sistema")
	log.Error("falhou", "origem", "Open Finance", "conta", 7, "ativa", true, "tempo", 2*time.Second)
	log.Log(context.Background(), NivelCritico, "caiu", "origem", "Desconhecida")
	log.With("assunto", "câmbio").WithGroup("http").Warn("lento", "status", 503)

	if len(*proximo) != 5 {
		t.Errorf("o log de sempre recebeu %d, esperado 5: %v", len(*proximo), *proximo)
	}
	r := lidos()
	if len(r) != 4 {
		t.Fatalf("%d na fila, esperado 4 (sem a informação)", len(r))
	}
	esperados := []struct {
		nivel             Nivel
		origem, categoria string
		mensagem          string
	}{
		{Aviso, "Sistema", "Financas.Go.Sistema", "relógio de teste (inicio=2026-10-15T01:00:00Z)"},
		{Erro, "Open Finance", "Financas.Go.Open Finance", "falhou (conta=7 ativa=true tempo=2000000000)"},
		{Critico, "Sistema", "Financas.Go", "caiu"}, // origem que não é das conhecidas: Sistema, e não vira atributo
		{Aviso, "Sistema", "Financas.Go", `lento (assunto=câmbio http={"status":"503"})`},
	}
	for i, e := range esperados {
		if r[i].Nivel != e.nivel || r[i].Origem != e.origem || r[i].Categoria != e.categoria || r[i].Mensagem != e.mensagem {
			t.Errorf("registro %d: %+v, esperado %+v", i, r[i], e)
		}
		if !r[i].Em.Equal(testeapoio.InstanteDosTestes) {
			t.Errorf("registro %d na hora %v, e não na do relógio", i, r[i].Em)
		}
	}
}

// Regra: o atributo error vai para o detalhe, com o tipo do erro mais de dentro na primeira linha (é ele que
// agrupa), e não para a mensagem; a rota e o login vêm do contexto da requisição.
func TestHandlerComErroEContexto(t *testing.T) {
	log, _, lidos := capturar(t)
	err := fmt.Errorf("abrir o banco: %w", &fs.PathError{Op: "open", Path: "x.db", Err: fs.ErrNotExist})
	ctx := ComRequisicao(context.Background(), "GET", "/api/contas", texto("ana.teste"))
	log.ErrorContext(ctx, "erro nos parâmetros", "origem", "API", "etapa", "listar", "erro", err)
	log.Error("sem mensagem de erro", "erro", errors.Join(errors.New("a"), errors.New("b")))

	r := lidos()
	if r[0].Mensagem != "erro nos parâmetros (etapa=listar)" || r[0].Origem != "API" {
		t.Errorf("mensagem %q, origem %q", r[0].Mensagem, r[0].Origem)
	}
	if r[0].Detalhe == nil || *r[0].Detalhe != "*fs.PathError: abrir o banco: open x.db: file does not exist\n   cadeia: *fmt.wrapError → *fs.PathError → *errors.errorString" {
		t.Errorf("detalhe %q", *r[0].Detalhe)
	}
	if r[0].Rota == nil || *r[0].Rota != "GET /api/contas" || r[0].Usuario == nil || *r[0].Usuario != "ana.teste" {
		t.Errorf("rota %v, usuário %v", r[0].Rota, r[0].Usuario)
	}
	if r[1].Rota != nil || r[1].Detalhe == nil || !strings.HasPrefix(*r[1].Detalhe, "*errors.joinError: a\nb") {
		t.Errorf("sem contexto: rota %v, detalhe %q", r[1].Rota, *r[1].Detalhe)
	}
}

// Regra: o próximo handler continua decidindo o que ele mesmo mostra; o aviso é capturado mesmo se ele não mostra.
func TestHandlerRespeitaONivelDoProximo(t *testing.T) {
	fila := NovaFila()
	var proximo []string
	log := slog.New(NovoHandler(gravadorDeTexto{saida: &proximo, nivel: slog.LevelError}, fila, testeapoio.RelogioDosTestes()))
	log.Warn("aviso")
	if len(proximo) != 0 || len(fila.canal) != 1 {
		t.Errorf("próximo %v, fila %d", proximo, len(fila.canal))
	}
}

// parametrosFixos é o prazo de retenção, sem o banco.
type parametrosFixos int

func (p parametrosFixos) ObterInteiro(context.Context, string) (int, error) { return int(p), nil }

func linhasDaTabela(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rs, err := db.QueryContext(t.Context(), `SELECT "Id", "PrimeiraEm", "UltimaEm", "Ocorrencias", "Nivel", "Origem", "Categoria",
		"Mensagem", COALESCE("Detalhe", '-'), COALESCE("Rota", '-'), COALESCE("Usuario", '-'), "Resolvido" FROM "ErroSistema" ORDER BY "Id"`)
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Close()
	var linhas []string
	for rs.Next() {
		var id, ocorrencias, nivel, resolvido int
		var primeira, ultima, origem, categoria, mensagem, detalhe, rota, usuario string
		if err := rs.Scan(&id, &primeira, &ultima, &ocorrencias, &nivel, &origem, &categoria, &mensagem, &detalhe, &rota, &usuario, &resolvido); err != nil {
			t.Fatal(err)
		}
		linhas = append(linhas, fmt.Sprintf("%d|%s|%s|%d|%d|%s|%s|%s|%s|%s|%s|%d", id, primeira, ultima, ocorrencias, nivel, origem,
			categoria, mensagem, detalhe, rota, usuario, resolvido))
	}
	return linhas
}

func gravar(t *testing.T, db *sql.DB, lote ...Registro) {
	t.Helper()
	g := NovoGravador(db, NovaFila(), testeapoio.RelogioDosTestes(), parametrosFixos(90), slog.New(slog.DiscardHandler))
	g.gravarLote(t.Context(), lote)
}

// Regra: a mesma ocorrência em aberto vista nas últimas 24 h soma na mesma linha (no mesmo lote ou entre lotes), com
// o nível mais alto, a rota nova e o usuário novo quando vêm; resolvida ou mais antiga, vira linha nova. Tudo já
// limpo e cortado no tamanho das colunas.
func TestGravarAgrupa(t *testing.T) {
	db := testeapoio.NovoBanco(t)
	t0 := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	base := Registro{Em: t0, Nivel: Aviso, Origem: "Open Finance", Categoria: "Financas.Go.Open Finance", Mensagem: "a Pluggy respondeu 503 (token=abc)"}
	mais := func(ajuste func(*Registro)) Registro { r := base; ajuste(&r); return r }

	gravar(t, db, base,
		mais(func(r *Registro) { r.Em = t0.Add(time.Minute); r.Nivel = Erro; r.Rota = texto("GET /api/x?y=1") }),
		mais(func(r *Registro) { r.Em = t0.Add(2 * time.Minute); r.Nivel = Aviso; r.Usuario = texto("ana.teste") }))
	gravar(t, db, mais(func(r *Registro) { r.Em = t0.Add(3 * time.Hour) }))
	if l := linhasDaTabela(t, db); len(l) != 1 ||
		l[0] != "1|2026-10-15 12:00:00|2026-10-15 15:00:00|4|4|Open Finance|Financas.Go.Open Finance|a Pluggy respondeu 503 (token=[removido])|-|GET /api/x?y=1|ana.teste|0" {
		t.Fatalf("agrupado:\n%s", strings.Join(l, "\n"))
	}

	// 24 h e 1 s depois da última: linha nova.
	gravar(t, db, mais(func(r *Registro) { r.Em = t0.Add(27*time.Hour + time.Second) }))
	// Resolvida: o que vem depois vira outra linha.
	if _, err := db.ExecContext(t.Context(), `UPDATE "ErroSistema" SET "Resolvido" = 1 WHERE "Id" = 2`); err != nil {
		t.Fatal(err)
	}
	gravar(t, db, mais(func(r *Registro) { r.Em = t0.Add(28 * time.Hour) }))
	// Outra mensagem: outra assinatura.
	gravar(t, db, mais(func(r *Registro) { r.Em = t0.Add(28 * time.Hour); r.Mensagem = "outra" }))
	if l := linhasDaTabela(t, db); len(l) != 4 {
		t.Fatalf("esperadas 4 linhas:\n%s", strings.Join(l, "\n"))
	}

	longa := strings.Repeat("ç", 1001)
	gravar(t, db, Registro{Em: t0, Nivel: Critico, Origem: strings.Repeat("o", 31), Categoria: strings.Repeat("c", 201), Mensagem: longa,
		Detalhe: texto(strings.Repeat("d", 8001)), Rota: texto(strings.Repeat("r", 301)), Usuario: texto(strings.Repeat("u", 101))})
	l := linhasDaTabela(t, db)
	partes := strings.Split(l[4], "|")
	tamanhos := []int{len([]rune(partes[5])), len([]rune(partes[6])), len([]rune(partes[7])), len([]rune(partes[8])), len([]rune(partes[9])), len([]rune(partes[10]))}
	if fmt.Sprint(tamanhos) != "[30 200 1000 8000 300 100]" || !strings.HasSuffix(partes[7], "…") {
		t.Errorf("tamanhos %v", tamanhos)
	}
}

// Regra: a limpeza apaga o que passou do prazo pela última ocorrência, uma vez por dia, já na primeira volta.
func TestLimpeza(t *testing.T) {
	db := testeapoio.NovoBanco(t)
	agora := testeapoio.InstanteDosTestes
	for i, dias := range []int{91, 90, 89} {
		ultima := tipos.NovaDataHora(agora.Add(-time.Duration(dias)*24*time.Hour + time.Duration(i)*time.Second))
		if _, err := db.ExecContext(t.Context(), `INSERT INTO "ErroSistema" ("PrimeiraEm", "UltimaEm", "Nivel", "Origem", "Categoria",
			"Mensagem", "Assinatura") VALUES (?, ?, 3, 'Sistema', 'c', ?, 'x')`, ultima, ultima, fmt.Sprint(dias)); err != nil {
			t.Fatal(err)
		}
	}
	r := &relogioMovel{agora: agora}
	g := NovoGravador(db, NovaFila(), r, parametrosFixos(90), slog.New(slog.DiscardHandler))
	g.limparSeVenceu(t.Context())
	if l := linhasDaTabela(t, db); len(l) != 2 {
		t.Fatalf("depois da limpeza:\n%s", strings.Join(l, "\n"))
	}
	// Menos de um dia depois, não roda de novo, nem com o prazo menor.
	g.parametros = parametrosFixos(1)
	r.agora = agora.Add(23 * time.Hour)
	g.limparSeVenceu(t.Context())
	if l := linhasDaTabela(t, db); len(l) != 2 {
		t.Fatalf("limpou antes de um dia: %d linhas", len(l))
	}
	r.agora = agora.Add(24 * time.Hour)
	g.limparSeVenceu(t.Context())
	if l := linhasDaTabela(t, db); len(l) != 0 {
		t.Fatalf("depois de um dia, com prazo de 1 dia: %d linhas", len(l))
	}
}

type relogioMovel struct{ agora time.Time }

func (r *relogioMovel) Agora() time.Time { return r.agora }

// Regra: o gravador grava o que chega na fila, sem esperar o desligamento, e volta quando o contexto acaba.
//
// Antes (#458) o teste também registrava um erro logo depois de cancelar e esperava 2 linhas no fim: isso dependia de
// qual goroutine corria primeiro (se o Rodar pegasse o erro pela fila já com o contexto cancelado, o gravarLote falhava
// com context.Canceled e o erro se perdia, e o teste falhava 1 vez em 20). O mesmo acontece no .NET
// (GravadorErrosHostedService, SaveChangesAsync com o token cancelado): fica como está, e a lacuna vai para uma issue.
// Dentro do synctest, o synctest.Wait volta quando o gravador terminou de gravar e parou de novo na fila (#458); o banco
// nasce fora da bolha, porque as goroutines do database/sql são dele e não do teste.
func TestRodarGravaOQueChegaNaFila(t *testing.T) {
	db := testeapoio.NovoBanco(t)
	synctest.Test(t, func(t *testing.T) {
		fila := NovaFila()
		g := NovoGravador(db, fila, testeapoio.RelogioDosTestes(), parametrosFixos(90), slog.New(slog.DiscardHandler))
		ctx, parar := context.WithCancel(t.Context())
		parou := make(chan struct{})
		go func() { g.Rodar(ctx); close(parou) }()
		fila.Registrar(Registro{Em: testeapoio.InstanteDosTestes, Nivel: Erro, Origem: "Sistema", Categoria: "c", Mensagem: "primeiro"})
		synctest.Wait()
		if l := linhasDaTabela(t, db); len(l) != 1 {
			t.Fatalf("antes de parar, gravou %d, esperado 1:\n%s", len(l), strings.Join(l, "\n"))
		}
		fila.Registrar(Registro{Em: testeapoio.InstanteDosTestes, Nivel: Erro, Origem: "Sistema", Categoria: "c", Mensagem: "segundo"})
		synctest.Wait()
		if l := linhasDaTabela(t, db); len(l) != 2 {
			t.Fatalf("antes de parar, gravou %d, esperado 2:\n%s", len(l), strings.Join(l, "\n"))
		}
		parar()
		<-parou
	})
}

// Regra: falha ao gravar vai para o log sem a captura, e não volta para a fila nem derruba nada.
func TestFalhaAoGravar(t *testing.T) {
	db := testeapoio.NovoBanco(t)
	fila := NovaFila()
	var log []string
	g := NovoGravador(db, fila, testeapoio.RelogioDosTestes(), parametrosFixos(90), slog.New(gravadorDeTexto{saida: &log}))
	db.Close()
	g.gravarLote(t.Context(), []Registro{{Mensagem: "x"}})
	if len(log) != 1 || log[0] != "Não foi possível gravar 1 erro(s) na tabela de erros." || len(fila.canal) != 0 {
		t.Errorf("log %v, fila %d", log, len(fila.canal))
	}
}

// Regra: o filtro da URL lido como o model binding do ASP.NET (o contrato confere as mensagens; aqui, os limites).
func TestLerFiltro(t *testing.T) {
	f, erros := lerFiltro("")
	if erros != nil || f.pagina != 1 || f.tamanhoPagina != 25 || f.situacao != nil {
		t.Errorf("padrão: %+v %v", f, erros)
	}
	casos := []struct {
		consulta string
		erro     string // "" é aceito
	}{
		{"tamanhoPagina=100", ""},
		{"tamanhoPagina=101", "TamanhoPagina"},
		{"pagina=2147483647", ""},
		{"pagina=0x7FFFFFFF", ""},
		{"pagina=0xFFFFFFFF", "Pagina"}, // -1: passa no int, cai no Range
		{"pagina=0x1FFFFFFFF", "Pagina"},
		{"pagina=-1", "Pagina"},
		{"nivel=5", ""}, {"nivel=Critico", ""}, {"nivel=%20erro%20", ""}, {"nivel=4,4", ""}, {"nivel=2", "Nivel"}, {"nivel=Aviso,Erro", "Nivel"},
		{"situacao=todos", ""}, {"situacao=%20todos", "Situacao"},
		{"origem=" + strings.Repeat("ç", 30), ""}, {"origem=" + strings.Repeat("😀", 16), "Origem"},
		{"inicio=2026-2-28", ""}, {"inicio=2/28/2026", ""}, {"inicio=28/02/2026", "Inicio"}, {"inicio=2026-10-03T25:00", "Inicio"},
		{"fim=2026-10-03%2010:00:00", ""},
	}
	for _, c := range casos {
		_, erros := lerFiltro(c.consulta)
		obtido := ""
		for chave := range erros {
			obtido = chave
		}
		if obtido != c.erro {
			t.Errorf("%s: erro em %q (%v), esperado %q", c.consulta, obtido, erros, c.erro)
		}
	}
}

// Regra: o período é em dias de Brasília, pela última ocorrência: o fim inclui até 23h59 de Brasília (03:00 UTC do
// dia seguinte), e o início começa às 03:00 UTC.
// O hexadecimal são os 32 bits do Convert.ToInt32(texto, 16): acima de 0x7FFFFFFF dá negativo, e não "não válido".
func TestLerInt32(t *testing.T) {
	casos := []struct {
		bruto string
		valor int
		erro  string
	}{
		{" 42 ", 42, ""},
		{"0x7FFFFFFF", 2147483647, ""},
		{"0x80000000", -2147483648, ""},
		{"&hFFFFFFFF", -1, ""},
		{"0x1FFFFFFFF", 0, "The value '0x1FFFFFFFF' is not valid for Pagina."},
		{"0x", 0, "The input was not valid."},
		{"abc", 0, "The value 'abc' is not valid for Pagina."},
	}
	for _, c := range casos {
		valor, erro := lerInt32(c.bruto, "Pagina")
		if valor != c.valor || erro != c.erro {
			t.Errorf("lerInt32(%q) = %d, %q; esperado %d, %q", c.bruto, valor, erro, c.valor, c.erro)
		}
	}
}

func TestPeriodoEmDiasDeBrasilia(t *testing.T) {
	db := testeapoio.NovoBanco(t)
	for i, ultima := range []string{"2026-10-06 02:59:59.9999999", "2026-10-06 03:00:00", "2026-10-07 02:59:59", "2026-10-07 03:00:00"} {
		if _, err := db.ExecContext(t.Context(), `INSERT INTO "ErroSistema" ("PrimeiraEm", "UltimaEm", "Nivel", "Origem", "Categoria",
			"Mensagem", "Assinatura") VALUES (?, ?, 3, 'Sistema', 'c', ?, 'x')`, ultima, ultima, fmt.Sprint(i)); err != nil {
			t.Fatal(err)
		}
	}
	mux := http.NewServeMux()
	Registrar(mux, NovoServico(db, NovaFila(), testeapoio.RelogioDosTestes(), slog.New(slog.DiscardHandler)))
	r := testeapoio.Chamar(t, mux, "GET", "/api/erros?inicio=2026-10-06&fim=2026-10-06", "")
	r.ConferirStatus(t, http.StatusOK)
	if !strings.Contains(string(r.Corpo), `"total":2`) || !strings.Contains(string(r.Corpo), `"mensagem":"1"`) || !strings.Contains(string(r.Corpo), `"mensagem":"2"`) {
		t.Errorf("06/10 em Brasília: %s", r.Corpo)
	}
}

// Regra: o erro do front-end vai para a fila como o .NET monta (origem e categoria Front-end, "tela <rota>", o login),
// e responde 204 mesmo descartado (31º na hora).
func TestErroDoFrontEnd(t *testing.T) {
	db := testeapoio.NovoBanco(t)
	if _, err := db.ExecContext(t.Context(), `INSERT INTO "Usuario" ("Id", "Login", "SenhaHash", "CarimboSeguranca", "TentativasFalhas", "CriadoEm")
		VALUES (?, 'ana.teste', 'x', '00000000-0000-0000-0000-000000000001', 0, '2026-10-01 00:00:00')`, testeapoio.UsuarioDosTestes); err != nil {
		t.Fatal(err)
	}
	fila := NovaFila()
	mux := http.NewServeMux()
	Registrar(mux, NovoServico(db, fila, relogio.Fixo{Instante: testeapoio.InstanteDosTestes}, slog.New(slog.DiscardHandler)))
	for range maximoFrontEndPorHora + 1 {
		testeapoio.Chamar(t, mux, "POST", "/api/erros/front-end", `{"mensagem":"m","rota":"/painel"}`).ConferirStatus(t, http.StatusNoContent)
	}
	if len(fila.canal) != maximoFrontEndPorHora {
		t.Errorf("%d na fila, esperado %d", len(fila.canal), maximoFrontEndPorHora)
	}
	r := <-fila.canal
	if r.Origem != "Front-end" || r.Categoria != "Front-end" || r.Nivel != Erro || *r.Rota != "tela /painel" || *r.Usuario != "ana.teste" || r.Detalhe != nil {
		t.Errorf("registro %+v", r)
	}
}
