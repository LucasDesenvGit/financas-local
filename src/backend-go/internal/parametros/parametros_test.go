package parametros

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
)

// novo monta o serviço num banco novo (com as configurações iniciais) e o mux com as rotas.
func novo(t *testing.T, r relogio.Relogio) (*sql.DB, *http.ServeMux) {
	t.Helper()
	db := testeapoio.NovoBanco(t)
	if _, err := db.ExecContext(t.Context(), `INSERT INTO "Usuario" ("Id", "Login", "SenhaHash", "CarimboSeguranca", "TentativasFalhas", "CriadoEm")
		VALUES (?, 'ana.teste', 'x', '00000000-0000-0000-0000-000000000001', 0, '2026-10-01 00:00:00')`, testeapoio.UsuarioDosTestes); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	Registrar(mux, Novo(Opcoes{
		DB: db, Relogio: r, Log: slog.New(slog.DiscardHandler), Versao: "9.9.9",
		CaminhoBanco: filepath.Join(t.TempDir(), "FinancasLocal.db"), PastaChaves: t.TempDir(),
		Padroes: map[string]int{"Seguranca.MaximoTentativasLogin": 7, "Sincronizacao.IntervaloHoras": 40},
	}))
	return db, mux
}

func configuracoes(t *testing.T, db *sql.DB) map[string]string {
	t.Helper()
	linhas, err := db.QueryContext(t.Context(), `SELECT "Chave", COALESCE("Valor", '<null>') || '|' || "Protegido" || '|' || "AtualizadoEm" FROM "Configuracao"`)
	if err != nil {
		t.Fatal(err)
	}
	defer linhas.Close()
	resultado := map[string]string{}
	for linhas.Next() {
		var chave, valor string
		if err := linhas.Scan(&chave, &valor); err != nil {
			t.Fatal(err)
		}
		resultado[chave] = valor
	}
	return resultado
}

func auditorias(t *testing.T, db *sql.DB) []string {
	t.Helper()
	linhas, err := db.QueryContext(t.Context(), `SELECT "DataHora" || '|' || "Evento" || '|' || COALESCE("Login", '<null>') || '|' || "Ip" || '|' || "UserAgent" FROM "AuditoriaAcesso" ORDER BY "Id"`)
	if err != nil {
		t.Fatal(err)
	}
	defer linhas.Close()
	var resultado []string
	for linhas.Next() {
		var a string
		if err := linhas.Scan(&a); err != nil {
			t.Fatal(err)
		}
		resultado = append(resultado, a)
	}
	return resultado
}

func valorNaLista(t *testing.T, corpo []byte, chave string) ParametroDto {
	t.Helper()
	var lista []ParametroDto
	if err := json.Unmarshal(corpo, &lista); err != nil {
		t.Fatalf("lista: %v\n%s", err, corpo)
	}
	for _, p := range lista {
		if p.Chave == chave {
			return p
		}
	}
	t.Fatalf("%s não está na lista", chave)
	return ParametroDto{}
}

// Regra: a lista tem o catálogo inteiro, na ordem, com mínimo e máximo só nos inteiros.
func TestListarTemOCatalogoNaOrdem(t *testing.T) {
	_, mux := novo(t, testeapoio.RelogioDosTestes())
	r := testeapoio.Chamar(t, mux, "GET", "/api/parametros", "")
	r.ConferirStatus(t, http.StatusOK)
	var lista []map[string]any
	if err := json.Unmarshal(r.Corpo, &lista); err != nil {
		t.Fatal(err)
	}
	if len(lista) != len(Catalogo) {
		t.Fatalf("%d itens, esperado %d", len(lista), len(Catalogo))
	}
	for i, d := range Catalogo {
		if lista[i]["chave"] != d.Chave {
			t.Errorf("item %d: %v, esperado %s", i, lista[i]["chave"], d.Chave)
		}
	}
	testeapoio.ConferirJSON(t, mustJSON(t, lista[0]), `{"chave":"Sincronizacao.Habilitada","grupo":"Sincronização",
		"nome":"Sincronização automática","descricao":"Busca os dados dos bancos sozinha, no intervalo abaixo. Desligada, só sincroniza pelo botão.",
		"tipo":2,"valor":1,"padrao":1,"minimo":null,"maximo":null,"unidade":null}`)
	testeapoio.ConferirJSON(t, mustJSON(t, lista[8]), `{"chave":"Orcamento.MetaPoupanca","grupo":"Orçamentos","nome":"Meta de poupança",
		"descricao":"Quanto da renda a sugestão de orçamentos reserva para guardar ou investir antes de distribuir o resto (regra 50/30/20: pelo menos 20%).",
		"tipo":1,"valor":20,"padrao":20,"minimo":0,"maximo":60,"unidade":"% da renda"}`)
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Regra: o valor gravado volta limitado à faixa; o que não se lê, e o segredo, voltam como o padrão; o padrão vem do
// appsettings só nos parâmetros que existiam lá (DoAppsettings).
func TestListarNormalizaOsValoresGravados(t *testing.T) {
	db, mux := novo(t, testeapoio.RelogioDosTestes())
	casos := []struct {
		chave, gravado string
		protegido      int
		valor, padrao  int
	}{
		{"Painel.DiasAvisoConsentimento", "500", 0, 120, 30},  // acima do máximo
		{"Orcamento.CartaoAlerta", "-3", 0, 10, 70},           // abaixo do mínimo
		{"Transacoes.MinimoLetrasDescricao", " +4 ", 0, 4, 3}, // espaços e sinal, como o int.TryParse
		{"Orcamento.CorteCartao", "1.5", 0, 10, 10},           // não é inteiro: o padrão
		{"Orcamento.FolgaEssenciais", "99999999999", 0, 5, 5}, // passa do int: o padrão
		{"Metas.MargemSeguranca", "12", 1, 10, 10},            // segredo não entra
		{"Sincronizacao.Habilitada", " FALSE ", 0, 0, 1},      // bool.TryParse ignora maiúsculas e espaços
		{"Seguranca.MinutosBloqueio", "", 0, 15, 15},          // vazio: o padrão
		{"Seguranca.MaximoTentativasLogin", "abc", 0, 7, 7},   // padrão do appsettings
		{"Sincronizacao.IntervaloHoras", "x", 0, 6, 6},        // padrão no appsettings não vale aqui
		{"Diagnostico.DiasRetencaoErros", "1000", 0, 365, 90}, // limitado ao máximo
		{"Painel.DiasProximosVencimentos", " 15", 0, 15, 15},  // espaço que o int.TryParse não aceita: padrão
	}
	for _, c := range casos {
		if _, err := db.ExecContext(t.Context(), `INSERT OR REPLACE INTO "Configuracao" VALUES (?, ?, ?, '2026-10-01 00:00:00')`, c.chave, c.gravado, c.protegido); err != nil {
			t.Fatal(err)
		}
	}
	r := testeapoio.Chamar(t, mux, "GET", "/api/parametros", "")
	r.ConferirStatus(t, http.StatusOK)
	for _, c := range casos {
		p := valorNaLista(t, r.Corpo, c.chave)
		if p.Valor != c.valor || p.Padrao != c.padrao {
			t.Errorf("%s gravado %q: valor %d e padrão %d, esperado %d e %d", c.chave, c.gravado, p.Valor, p.Padrao, c.valor, c.padrao)
		}
	}
}

// Regra: salvar grava cada valor (booleano como true/false), regrava AtualizadoEm mesmo sem mudança, registra a
// auditoria com o login da sessão e devolve a lista nova.
func TestSalvarGravaValoresEAuditoria(t *testing.T) {
	agora := time.Date(2026, 10, 15, 12, 0, 0, 123456700, time.UTC)
	db, mux := novo(t, relogio.Fixo{Instante: agora})
	r := testeapoio.Chamar(t, mux, "PUT", "/api/parametros",
		`{"valores":{"Sincronizacao.IntervaloHoras":12,"Sincronizacao.Habilitada":0,"Orcamento.MetaPoupanca":"60","Sincronizacao.DiasRetroativos":30}}`)
	r.ConferirStatus(t, http.StatusOK)
	if p := valorNaLista(t, r.Corpo, "Sincronizacao.IntervaloHoras"); p.Valor != 12 {
		t.Errorf("a resposta não traz a lista nova: %+v", p)
	}
	c := configuracoes(t, db)
	for chave, esperado := range map[string]string{
		"Sincronizacao.IntervaloHoras":  "12|0|2026-10-15 12:00:00.1234567",
		"Sincronizacao.Habilitada":      "false|0|2026-10-15 12:00:00.1234567",
		"Orcamento.MetaPoupanca":        "60|0|2026-10-15 12:00:00.1234567", // não existia: criado
		"Sincronizacao.DiasRetroativos": "30|0|2026-10-15 12:00:00.1234567", // mesmo valor: a hora muda
	} {
		if c[chave] != esperado {
			t.Errorf("%s = %q, esperado %q", chave, c[chave], esperado)
		}
	}
	if c["Sincronizacao.DiasPrimeiraCarga"] == "" || strings.HasPrefix(c["Sincronizacao.DiasPrimeiraCarga"], "365|0|2026-10-15 12") {
		t.Errorf("o que não foi enviado não muda: %q", c["Sincronizacao.DiasPrimeiraCarga"])
	}
	if a := auditorias(t, db); len(a) != 1 || a[0] != "2026-10-15 12:00:00.1234567|ParametrosAlterados|ana.teste|192.0.2.1|" {
		t.Errorf("auditoria: %q", a)
	}

	testeapoio.Chamar(t, mux, "PUT", "/api/parametros", `{"valores":{"Sincronizacao.Habilitada":1}}`).ConferirStatus(t, http.StatusOK)
	if v := configuracoes(t, db)["Sincronizacao.Habilitada"]; !strings.HasPrefix(v, "true|") {
		t.Errorf("ligado vira true: %q", v)
	}
}

// Regra: com um valor errado, nada é gravado (nem os válidos, nem a auditoria) e as mensagens saem juntas, na ordem
// enviada.
func TestSalvarComErroNaoGravaNada(t *testing.T) {
	db, mux := novo(t, testeapoio.RelogioDosTestes())
	antes := configuracoes(t, db)
	r := testeapoio.Chamar(t, mux, "PUT", "/api/parametros", `{"valores":{"Sincronizacao.IntervaloHoras":12,"Nao.Existe":1,
		"Sincronizacao.Habilitada":2,"Painel.DiasProximosVencimentos":91,"Orcamento.MetaPoupanca":-1,"Orcamento.FolgaEssenciais":0,
		"Orcamento.CartaoAlerta":101}}`)
	r.ConferirStatus(t, http.StatusBadRequest)
	r.ConferirJSON(t, `{"mensagem":"Parâmetro desconhecido: Nao.Existe. Sincronização automática: use ligado ou desligado. Próximos vencimentos: informe um valor entre 1 e 90 dias. Meta de poupança: informe um valor entre 0 e 60 % da renda. Uso do limite do cartão: alerta: informe um valor entre 10 e 100 % do limite."}`)
	if depois := configuracoes(t, db); len(depois) != len(antes) || depois["Sincronizacao.IntervaloHoras"] != antes["Sincronizacao.IntervaloHoras"] {
		t.Errorf("gravou com erro: antes %v, depois %v", antes, depois)
	}
	if a := auditorias(t, db); len(a) != 0 {
		t.Errorf("auditoria gravada com erro: %q", a)
	}
}

// Regra: os limites da faixa valem (mínimo e máximo inclusive); booleano só 0 ou 1.
func TestValidarLimites(t *testing.T) {
	casos := []struct {
		valor Valor
		erro  string
	}{
		{Valor{"Seguranca.MinutosBloqueio", 1}, ""},
		{Valor{"Seguranca.MinutosBloqueio", 1440}, ""},
		{Valor{"Seguranca.MinutosBloqueio", 0}, "Tempo de bloqueio: informe um valor entre 1 e 1440 minutos."},
		{Valor{"Seguranca.MinutosBloqueio", 1441}, "Tempo de bloqueio: informe um valor entre 1 e 1440 minutos."},
		{Valor{"Orcamento.FolgaEssenciais", 31}, "Folga nos gastos essenciais: informe um valor entre 0 e 30 %."},
		{Valor{"Sincronizacao.Habilitada", 0}, ""},
		{Valor{"Sincronizacao.Habilitada", -1}, "Sincronização automática: use ligado ou desligado."},
		{Valor{"sincronizacao.habilitada", 1}, "Parâmetro desconhecido: sincronizacao.habilitada."}, // chave exata
	}
	for _, c := range casos {
		erros := validar([]Valor{c.valor})
		obtido := strings.Join(erros, " ")
		if obtido != c.erro {
			t.Errorf("%+v: %q, esperado %q", c.valor, obtido, c.erro)
		}
	}
}

// Regra: o corpo é lido como o System.Text.Json do ASP.NET, com as mesmas mensagens e posições.
func TestSalvarCorpo(t *testing.T) {
	casos := []struct {
		nome, corpo string
		status      int
		esperado    string
	}{
		{"sem valores", `{"valores":{}}`, 400, `{"mensagem":"Nenhum parâmetro informado."}`},
		{"sem o campo", `{}`, 400, `{"type":"https://tools.ietf.org/html/rfc9110#section-15.5.1","title":"One or more validation errors occurred.",
			"status":400,"errors":{"Valores":["The Valores field is required."]},"traceId":"<traceId>"}`},
		{"valores nulo", `{"valores":null}`, 400, `{"type":"https://tools.ietf.org/html/rfc9110#section-15.5.1","title":"One or more validation errors occurred.",
			"status":400,"errors":{"Valores":["The Valores field is required."]},"traceId":"<traceId>"}`},
		{"corpo null", `null`, 400, `{"type":"https://tools.ietf.org/html/rfc9110#section-15.5.1","title":"One or more validation errors occurred.",
			"status":400,"errors":{"":["A non-empty request body is required."],"dto":["The dto field is required."]},"traceId":"<traceId>"}`},
		{"decimal, com a posição do fim do número", `{ "valores": { "Sincronizacao.IntervaloHoras": 1.5 } }`, 400, erroJSON(
			`$.valores['Sincronizacao.IntervaloHoras']`, "System.Int32", 50)},
		{"chave sem caractere especial vai com ponto", `{"valores":{"Abc":true}}`, 400, erroJSON(`$.valores.Abc`, "System.Int32", 22)},
		{"texto que não é número", `{"valores":{"A.b":"1,5"}}`, 400, erroJSON(`$.valores['A.b']`, "System.Int32", 23)},
		{"texto com espaço", `{"valores":{"A.b":" 1"}}`, 400, erroJSON(`$.valores['A.b']`, "System.Int32", 22)},
		{"passa do int", `{"valores":{"A.b":2147483648}}`, 400, erroJSON(`$.valores['A.b']`, "System.Int32", 28)},
		{"nulo no valor", `{"valores":{"A.b":null}}`, 400, erroJSON(`$.valores['A.b']`, "System.Int32", 22)},
		{"valores não é objeto", `{"valores":[1]}`, 400, erroJSON(`$.valores`, "System.Collections.Generic.Dictionary`2[System.String,System.Int32]", 12)},
		{"corpo não é objeto", `[1]`, 400, erroJSON(`$`, "Financas.Api.Dtos.SalvarParametrosDto", 1)},
		{"o erro de tipo vem antes do erro de sintaxe depois dele", `{"valores":{"A.b":1.5}, x}`, 400, erroJSON(`$.valores['A.b']`, "System.Int32", 21)},
		{"texto com sinal vale, e o nome não liga para maiúsculas", `{"VALORES":{"Seguranca.MinutosBloqueio":"+0020"}}`, 200, ""},
		{"propriedade a mais é ignorada, a repetida fica com o último", `{"outra":{"x":[1,{"y":2}]},"valores":{"Seguranca.MinutosBloqueio":9999},"valores":{"Seguranca.MinutosBloqueio":21}}`, 200, ""},
		{"chave repetida fica com o último valor", `{"valores":{"Seguranca.MinutosBloqueio":0,"Seguranca.MinutosBloqueio":22}}`, 200, ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			_, mux := novo(t, testeapoio.RelogioDosTestes())
			r := testeapoio.Chamar(t, mux, "PUT", "/api/parametros", c.corpo)
			r.ConferirStatus(t, c.status)
			if c.esperado != "" {
				// O traceId é novo a cada resposta (o formato é conferido no pacote respostas).
				testeapoio.ConferirJSON(t, traceID.ReplaceAll(r.Corpo, []byte(`"traceId":"<traceId>"`)), c.esperado)
			}
		})
	}
}

var traceID = regexp.MustCompile(`"traceId":"00-[0-9a-f]{32}-[0-9a-f]{16}-00"`)

func erroJSON(caminho, tipo string, posicao int) string {
	mensagem := "The JSON value could not be converted to " + tipo + ". Path: " + caminho + " | LineNumber: 0 | BytePositionInLine: " + itoa(posicao) + "."
	erros, _ := json.Marshal(map[string][]string{caminho: {mensagem}, "dto": {"The dto field is required."}})
	return `{"type":"https://tools.ietf.org/html/rfc9110#section-15.5.1","title":"One or more validation errors occurred.","status":400,"errors":` +
		string(erros) + `,"traceId":"<traceId>"}`
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

// Regra: o valor repetido e o com sinal chegam ao banco como o .NET gravaria.
func TestSalvarRepetidoESinal(t *testing.T) {
	db, mux := novo(t, testeapoio.RelogioDosTestes())
	testeapoio.Chamar(t, mux, "PUT", "/api/parametros", `{"valores":{"Seguranca.MinutosBloqueio":0,"Seguranca.MinutosBloqueio":"+022"}}`).ConferirStatus(t, http.StatusOK)
	if v := configuracoes(t, db)["Seguranca.MinutosBloqueio"]; !strings.HasPrefix(v, "22|0|") {
		t.Errorf("gravado %q, esperado 22", v)
	}
}

// Regra: o diagnóstico conta contas, transações e conexões ativas, e lê a última sincronização.
func TestSistema(t *testing.T) {
	db, mux := novo(t, testeapoio.RelogioDosTestes())
	if _, err := db.ExecContext(t.Context(), `INSERT OR REPLACE INTO "Configuracao" VALUES ('Sincronizacao.UltimaExecucao', '2026-10-14T09:00:00.0000000Z', 0, '2026-10-01 00:00:00')`); err != nil {
		t.Fatal(err)
	}
	r := testeapoio.Chamar(t, mux, "GET", "/api/parametros/sistema", "")
	r.ConferirStatus(t, http.StatusOK)
	var info map[string]any
	if err := json.Unmarshal(r.Corpo, &info); err != nil {
		t.Fatal(err)
	}
	if info["versao"] != "9.9.9" || info["contas"] != 0.0 || info["transacoes"] != 0.0 || info["conexoesAtivas"] != 0.0 ||
		info["ultimaSincronizacao"] != "2026-10-14T09:00:00Z" || info["tamanhoBancoBytes"] != nil {
		t.Errorf("sistema: %s", r.Corpo)
	}
	if s, _ := info["iniciadoEm"].(string); !strings.HasSuffix(s, "Z") {
		t.Errorf("iniciadoEm em UTC com Z: %q", s)
	}
	if s, _ := info["sistemaOperacional"].(string); s == "" {
		t.Error("sem sistema operacional")
	}
}

// Regra: o valor do dicionário é o Int32Converter com AllowReadingFromString.
func TestInteiroDoValor(t *testing.T) {
	casos := []struct {
		token json.Token
		n     int
		ok    bool
	}{
		{json.Number("12"), 12, true}, {json.Number("-0"), 0, true}, {json.Number("-2147483648"), -2147483648, true},
		{json.Number("1.0"), 0, false}, {json.Number("1e2"), 0, false}, {json.Number("2147483648"), 0, false},
		{"12", 12, true}, {"+12", 12, true}, {"-12", -12, true}, {"0012", 12, true},
		{"+-5", 0, false}, {"-+5", 0, false}, {"--5", 0, false}, {"+", 0, false}, {"", 0, false}, {" 1", 0, false}, {"1 ", 0, false},
		{"1.5", 0, false}, {"١", 0, false}, {nil, 0, false}, {true, 0, false}, {json.Delim('{'), 0, false},
	}
	for _, c := range casos {
		if n, ok := inteiro(c.token); n != c.n || ok != c.ok {
			t.Errorf("inteiro(%#v) = %d, %v; esperado %d, %v", c.token, n, ok, c.n, c.ok)
		}
	}
}
