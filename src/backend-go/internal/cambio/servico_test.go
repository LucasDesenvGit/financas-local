package cambio

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// fonteFalsa faz o papel da PTAX: devolve o que o teste mandar e anota cada consulta.
type fonteFalsa struct {
	mu          sync.Mutex
	moedas      []Moeda
	errMoedas   error
	fechamentos map[string][]Fechamento // por moeda
	falham      map[string]bool         // moedas cuja consulta falha (sem internet)
	consultas   []string                // "USD 2026-10-07 2026-10-14"
}

func (f *fonteFalsa) ListarMoedas(context.Context) ([]Moeda, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.consultas = append(f.consultas, "moedas")
	return f.moedas, f.errMoedas
}

func (f *fonteFalsa) BuscarFechamentos(_ context.Context, moeda string, de, ate tipos.Data) ([]Fechamento, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.consultas = append(f.consultas, moeda+" "+de.String()+" "+ate.String())
	if f.falham[moeda] {
		return nil, errors.New("sem internet")
	}
	lista := []Fechamento{}
	for _, fe := range f.fechamentos[moeda] {
		if !fe.Data.Antes(de) && !fe.Data.Depois(ate) {
			lista = append(lista, fe)
		}
	}
	return lista, nil
}

func (f *fonteFalsa) consultou() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string{}, f.consultas...)
}

// logDoTeste guarda o log em texto, para conferir os avisos.
func logDoTeste() (*slog.Logger, *bytes.Buffer) {
	var b bytes.Buffer
	return slog.New(slog.NewTextHandler(&sincronizado{b: &b}, nil)), &b
}

type sincronizado struct {
	mu sync.Mutex
	b  *bytes.Buffer
}

func (s *sincronizado) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func novoServicoDeTeste(t *testing.T, r relogio.Relogio, fonte *fonteFalsa) (*Servico, *sql.DB, *testeapoio.Fabrica) {
	t.Helper()
	db := testeapoio.NovoBancoVazio(t)
	log, _ := logDoTeste()
	s := NovoServico(db, fonte, r, log)
	s.esperar = func(context.Context, time.Duration) error { return nil }
	return s, db, testeapoio.NovaFabrica(t, db, r)
}

func dia(d int) tipos.Data { return tipos.NovaData(2026, 10, d) }

func valor(t *testing.T, texto string) tipos.Decimal {
	t.Helper()
	return testeapoio.Valor(texto)
}

// linha lê o texto cru de uma coluna, como o SQLite guarda (para conferir o formato do EF Core).
func linha(t *testing.T, db *sql.DB, consulta string, args ...any) []string {
	t.Helper()
	linhas, err := db.Query(consulta, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer linhas.Close()
	colunas, _ := linhas.Columns()
	if !linhas.Next() {
		return nil
	}
	valores := make([]sql.NullString, len(colunas))
	ponteiros := make([]any, len(colunas))
	for i := range valores {
		ponteiros[i] = &valores[i]
	}
	if err := linhas.Scan(ponteiros...); err != nil {
		t.Fatal(err)
	}
	textos := make([]string, len(valores))
	for i, v := range valores {
		textos[i] = v.String
		if !v.Valid {
			textos[i] = "NULL"
		}
	}
	return textos
}

func jsonDe(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestNormalizarMoeda(t *testing.T) {
	t.Parallel()
	casos := []struct {
		entrada, codigo string
		valido          bool
	}{
		{"USD", "USD", true},
		{" usd ", "USD", true},
		{"\teur\n", "EUR", true},
		{"BRL", "BRL", true},
		{"", "", false},
		{"US", "US", false},
		{"USDT", "USDT", false},
		{"U$D", "U$D", false},
		{"ÉUR", "ÉUR", false}, // letra com acento não é de A a Z
		{"12A", "12A", false},
	}
	for _, c := range casos {
		t.Run(c.entrada, func(t *testing.T) {
			codigo, ok := NormalizarMoeda(c.entrada)
			if codigo != c.codigo || ok != c.valido {
				t.Errorf("NormalizarMoeda(%q) = %q, %v; esperado %q, %v", c.entrada, codigo, ok, c.codigo, c.valido)
			}
		})
	}
}

func TestObterCotacao(t *testing.T) {
	t.Parallel()
	// 14/10/2026, 15h em Brasília (já passou do fechamento).
	agora := testeapoio.AgoraEmBrasilia(2026, 10, 14, 15, 0)
	casos := []struct {
		nome           string
		moeda          string
		data           *tipos.Data
		guardadas      []testeapoio.CotacaoMoeda
		config         map[string]string
		fonte          *fonteFalsa
		esperado       string // JSON, ou "null"
		consultas      []string
		gravouConsulta bool // Cambio.UltimaConsulta.<moeda> gravada
	}{
		{
			nome: "real é fixo, sem banco nem API", moeda: "BRL", data: new(dia(10)), fonte: &fonteFalsa{},
			esperado:  `{"moeda":"BRL","dataSolicitada":"2026-10-10","data":"2026-10-10","compra":1,"venda":1,"fonte":"Fixa"}`,
			consultas: []string{},
		},
		{
			nome: "cotação do dia já guardada: não consulta a API", moeda: "USD",
			guardadas: []testeapoio.CotacaoMoeda{{Moeda: "USD", Data: dia(14), Compra: valor(t, "5.4123"), Venda: valor(t, "5.4129")}},
			fonte:     &fonteFalsa{},
			esperado:  `{"moeda":"USD","dataSolicitada":"2026-10-14","data":"2026-10-14","compra":5.4123,"venda":5.4129,"fonte":"PTAX/Banco Central"}`,
			consultas: []string{},
		},
		{
			nome: "feriado sem internet: o último dia útil guardado", moeda: "USD", data: new(dia(12)),
			guardadas: []testeapoio.CotacaoMoeda{{Moeda: "USD", Data: dia(9), Compra: valor(t, "5.3871"), Venda: valor(t, "5.3877")}},
			fonte:     &fonteFalsa{falham: map[string]bool{"USD": true}},
			esperado:  `{"moeda":"USD","dataSolicitada":"2026-10-12","data":"2026-10-09","compra":5.3871,"venda":5.3877,"fonte":"PTAX/Banco Central"}`,
			consultas: []string{"USD 2026-10-05 2026-10-12"},
		},
		{
			nome: "data no futuro vale hoje", moeda: "USD", data: new(tipos.NovaData(2026, 12, 1)),
			guardadas: []testeapoio.CotacaoMoeda{{Moeda: "USD", Data: dia(14), Compra: valor(t, "5.4123"), Venda: valor(t, "5.4129")}},
			fonte:     &fonteFalsa{},
			esperado:  `{"moeda":"USD","dataSolicitada":"2026-10-14","data":"2026-10-14","compra":5.4123,"venda":5.4129,"fonte":"PTAX/Banco Central"}`,
			consultas: []string{},
		},
		{
			nome: "mais de 7 dias sem cotação: nenhuma", moeda: "EUR", data: new(dia(13)),
			guardadas: []testeapoio.CotacaoMoeda{{Moeda: "EUR", Data: dia(5), Compra: valor(t, "6.1"), Venda: valor(t, "6.2")}},
			fonte:     &fonteFalsa{falham: map[string]bool{"EUR": true}},
			esperado:  "null", consultas: []string{"EUR 2026-10-06 2026-10-13"},
		},
		{
			nome: "hoje sem boletim e consultado há 10 minutos: não consulta de novo", moeda: "USD",
			guardadas: []testeapoio.CotacaoMoeda{{Moeda: "USD", Data: dia(13), Compra: valor(t, "5.4012"), Venda: valor(t, "5.4018")}},
			config:    map[string]string{"Cambio.UltimaConsulta.USD": "2026-10-14T17:50:00.0000000Z"},
			fonte:     &fonteFalsa{},
			esperado:  `{"moeda":"USD","dataSolicitada":"2026-10-14","data":"2026-10-13","compra":5.4012,"venda":5.4018,"fonte":"PTAX/Banco Central"}`,
			consultas: []string{},
		},
		{
			nome: "hoje sem boletim e consultado há 40 minutos: consulta e grava quando", moeda: "USD",
			guardadas: []testeapoio.CotacaoMoeda{{Moeda: "USD", Data: dia(13), Compra: valor(t, "5.4012"), Venda: valor(t, "5.4018")}},
			config:    map[string]string{"Cambio.UltimaConsulta.USD": "2026-10-14T17:20:00.0000000Z"},
			fonte: &fonteFalsa{fechamentos: map[string][]Fechamento{"USD": {
				{Data: dia(14), Compra: valor(t, "5.41230"), Venda: valor(t, "5.41290")},
			}}},
			esperado:       `{"moeda":"USD","dataSolicitada":"2026-10-14","data":"2026-10-14","compra":5.4123,"venda":5.4129,"fonte":"PTAX/Banco Central"}`,
			consultas:      []string{"USD 2026-10-07 2026-10-14"},
			gravouConsulta: true,
		},
		{
			nome: "hoje, a consulta que falha não grava o quando", moeda: "USD",
			fonte:    &fonteFalsa{falham: map[string]bool{"USD": true}},
			esperado: "null", consultas: []string{"USD 2026-10-07 2026-10-14"},
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			s, db, fab := novoServicoDeTeste(t, agora, c.fonte)
			for _, g := range c.guardadas {
				fab.CotacaoMoeda(func(x *testeapoio.CotacaoMoeda) {
					x.Moeda, x.Data, x.Compra, x.Venda = g.Moeda, g.Data, g.Compra, g.Venda
				})
			}
			for chave, v := range c.config {
				fab.Configuracao(chave, v)
			}
			cotacao, err := s.ObterCotacao(t.Context(), c.moeda, c.data)
			if err != nil {
				t.Fatal(err)
			}
			if obtido := jsonDe(t, cotacao); obtido != c.esperado {
				t.Errorf("JSON\n%s\nesperado\n%s", obtido, c.esperado)
			}
			if consultas := c.fonte.consultou(); strings.Join(consultas, "|") != strings.Join(c.consultas, "|") {
				t.Errorf("consultas à API %v, esperado %v", consultas, c.consultas)
			}
			gravada := linha(t, db, `SELECT Valor FROM Configuracao WHERE Chave = ?`, "Cambio.UltimaConsulta."+c.moeda)
			if c.gravouConsulta {
				// DateTime.UtcNow.ToString("O"), sem a fração (o relógio de teste está no segundo cheio).
				if gravada == nil || gravada[0] != "2026-10-14T18:00:00.0000000Z" {
					t.Errorf("Cambio.UltimaConsulta.%s = %v", c.moeda, gravada)
				}
			} else if gravada != nil && (c.config == nil || gravada[0] != c.config["Cambio.UltimaConsulta."+c.moeda]) {
				t.Errorf("gravou Cambio.UltimaConsulta.%s sem dever: %v", c.moeda, gravada)
			}
		})
	}
}

func TestHojeEhODiaDeBrasiliaEnaoODeUTC(t *testing.T) {
	t.Parallel()
	// 15/10 01:00 UTC ainda é 14/10, 22h, em Brasília: a cotação do dia é a de 14/10, que já está guardada.
	agora := relogio.Fixo{Instante: time.Date(2026, 10, 15, 1, 0, 0, 0, time.UTC)}
	fonte := &fonteFalsa{}
	s, _, fab := novoServicoDeTeste(t, agora, fonte)
	fab.CotacaoMoeda(func(c *testeapoio.CotacaoMoeda) { c.Data = dia(14) })
	cotacao, err := s.ObterCotacao(t.Context(), "USD", nil)
	if err != nil {
		t.Fatal(err)
	}
	if cotacao == nil || cotacao.DataSolicitada.String() != "2026-10-14" || len(fonte.consultou()) != 0 {
		t.Errorf("cotação %+v, consultas %v: hoje tem de ser 14/10 (Brasília), e não 15/10 (UTC)", cotacao, fonte.consultou())
	}
}

func TestBaixarInsereECorrigeNoFormatoDoEfCore(t *testing.T) {
	t.Parallel()
	agora := testeapoio.AgoraEmBrasilia(2026, 10, 14, 15, 0) // 18:00 UTC
	fonte := &fonteFalsa{fechamentos: map[string][]Fechamento{"USD": {
		{Data: dia(9), Compra: valor(t, "5.38710"), Venda: valor(t, "5.38770")}, // igual à guardada, com outra escala
		{Data: dia(13), Compra: valor(t, "5.40150"), Venda: valor(t, "5.4018")}, // republicada: a compra mudou
		{Data: dia(14), Compra: valor(t, "5.41230"), Venda: valor(t, "5.4129")}, // nova
	}}}
	s, db, fab := novoServicoDeTeste(t, agora, fonte)
	antes := relogio.Fixo{Instante: time.Date(2026, 10, 13, 16, 0, 0, 0, time.UTC)}
	fabAntes := testeapoio.NovaFabrica(t, db, antes)
	fabAntes.CotacaoMoeda(func(c *testeapoio.CotacaoMoeda) {
		c.Data, c.Compra, c.Venda = dia(9), valor(t, "5.3871"), valor(t, "5.3877")
	})
	fabAntes.CotacaoMoeda(func(c *testeapoio.CotacaoMoeda) {
		c.Data, c.Compra, c.Venda = dia(13), valor(t, "5.4012"), valor(t, "5.4018")
	})
	_ = fab

	novas, corrigidas, err := s.baixar(t.Context(), "USD", dia(7), dia(14), nil)
	if err != nil {
		t.Fatal(err)
	}
	if novas != 1 || corrigidas != 1 {
		t.Errorf("novas %d, corrigidas %d; esperado 1 e 1", novas, corrigidas)
	}
	casos := []struct {
		data     tipos.Data
		esperado string // Compra, Venda, AtualizadoEm, como o SQLite guarda
	}{
		{dia(9), "5.3871 5.3877 2026-10-13 16:00:00"},  // igual: não mexe (nem no AtualizadoEm)
		{dia(13), "5.4015 5.4018 2026-10-14 18:00:00"}, // corrigida
		{dia(14), "5.4123 5.4129 2026-10-14 18:00:00"}, // nova, sem os zeros à direita
	}
	for _, c := range casos {
		if obtido := strings.Join(linha(t, db, `SELECT Compra, Venda, AtualizadoEm FROM CotacaoMoeda WHERE Moeda = 'USD' AND Data = ?`, c.data), " "); obtido != c.esperado {
			t.Errorf("%s: %s, esperado %s", c.data, obtido, c.esperado)
		}
	}
	if tipo := linha(t, db, `SELECT typeof(Compra) FROM CotacaoMoeda WHERE Data = '2026-10-14'`); tipo[0] != "text" {
		t.Errorf("Compra gravada como %s, o EF Core grava text", tipo[0])
	}
}

func TestListarMoedas(t *testing.T) {
	t.Parallel()
	agora := testeapoio.AgoraEmBrasilia(2026, 10, 14, 15, 0)
	daAPI := []Moeda{{Codigo: "EUR", Nome: "Euro"}, {Codigo: "USD", Nome: "Dólar dos Estados Unidos"}}
	// O .NET grava o "ó" escapado (JavaScriptEncoder.Default): uma barra, a letra u e o código 00F3.
	escapado := strings.ReplaceAll("Dólar dos Estados Unidos", "ó", `\`+"u00F3")
	guardada := `[{"Codigo":"USD","Nome":"` + escapado + `"}]`
	casos := []struct {
		nome         string
		guardada     string
		consultadaEm string
		fonte        *fonteFalsa
		esperado     string
		consultou    bool
		gravou       string // o novo Cambio.Moedas, ou "" se não mudou
	}{
		{"guardada há menos de 7 dias: não consulta", guardada, "2026-10-10T18:00:00.0000000Z", &fonteFalsa{moedas: daAPI},
			`[{"codigo":"USD","nome":"Dólar dos Estados Unidos"}]`, false, ""},
		{"guardada há mais de 7 dias: consulta e grava como o .NET", guardada, "2026-10-07T17:00:00.0000000Z", &fonteFalsa{moedas: daAPI},
			`[{"codigo":"EUR","nome":"Euro"},{"codigo":"USD","nome":"Dólar dos Estados Unidos"}]`, true,
			`[{"Codigo":"EUR","Nome":"Euro"},{"Codigo":"USD","Nome":"` + escapado + `"}]`},
		{"sem internet: a guardada, mesmo velha", guardada, "2026-09-01T00:00:00.0000000Z", &fonteFalsa{errMoedas: errors.New("sem rede")},
			`[{"codigo":"USD","nome":"Dólar dos Estados Unidos"}]`, true, ""},
		{"nada guardado e sem internet: lista vazia", "", "", &fonteFalsa{errMoedas: errors.New("sem rede")}, `[]`, true, ""},
		{"API sem moedas: segue a guardada", guardada, "2026-09-01T00:00:00.0000000Z", &fonteFalsa{moedas: []Moeda{}},
			`[{"codigo":"USD","nome":"Dólar dos Estados Unidos"}]`, true, ""},
		{"guardada que não é a lista vale vazia", "isto não é JSON", "", &fonteFalsa{errMoedas: errors.New("sem rede")}, `[]`, true, ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			s, db, fab := novoServicoDeTeste(t, agora, c.fonte)
			if c.guardada != "" {
				fab.Configuracao("Cambio.Moedas", c.guardada)
			}
			if c.consultadaEm != "" {
				fab.Configuracao("Cambio.MoedasConsultadasEm", c.consultadaEm)
			}
			moedas, err := s.ListarMoedas(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if obtido := jsonDe(t, moedas); obtido != c.esperado {
				t.Errorf("moedas %s, esperado %s", obtido, c.esperado)
			}
			if consultou := len(c.fonte.consultou()) > 0; consultou != c.consultou {
				t.Errorf("consultou a API: %v, esperado %v", consultou, c.consultou)
			}
			if c.gravou != "" {
				if v := linha(t, db, `SELECT Valor FROM Configuracao WHERE Chave = 'Cambio.Moedas'`); v[0] != c.gravou {
					t.Errorf("Cambio.Moedas gravado\n%s\nesperado\n%s", v[0], c.gravou)
				}
				if v := linha(t, db, `SELECT Valor, Protegido, AtualizadoEm FROM Configuracao WHERE Chave = 'Cambio.MoedasConsultadasEm'`); strings.Join(v, " ") != "2026-10-14T18:00:00.0000000Z 0 2026-10-14 18:00:00" {
					t.Errorf("Cambio.MoedasConsultadasEm gravado %v", v)
				}
			}
		})
	}
}

func TestAtualizarTodas(t *testing.T) {
	t.Parallel()
	agora := testeapoio.AgoraEmBrasilia(2026, 10, 14, 15, 0)
	moedas := []Moeda{{Codigo: "EUR", Nome: "Euro"}, {Codigo: "USD", Nome: "Dólar"}}
	fechamentos := map[string][]Fechamento{
		"EUR": {{Data: dia(14), Compra: valor(t, "6.2981"), Venda: valor(t, "6.2995")}},
		"USD": {{Data: dia(13), Compra: valor(t, "5.4012"), Venda: valor(t, "5.4018")}, {Data: dia(14), Compra: valor(t, "5.4123"), Venda: valor(t, "5.4129")}},
	}
	casos := []struct {
		nome         string
		fonte        *fonteFalsa
		esperado     ResultadoAtualizacao
		marcouQuando bool
	}{
		{"todas certas: marca a atualização", &fonteFalsa{moedas: moedas, fechamentos: fechamentos},
			ResultadoAtualizacao{Novas: 3, Falhas: []string{}}, true},
		{"uma falha: grava a outra e não marca", &fonteFalsa{moedas: moedas, fechamentos: fechamentos, falham: map[string]bool{"EUR": true}},
			ResultadoAtualizacao{Novas: 2, Falhas: []string{"EUR"}}, false},
		{"sem lista de moedas", &fonteFalsa{errMoedas: errors.New("sem rede")},
			ResultadoAtualizacao{Falhas: []string{"lista de moedas"}}, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			s, db, _ := novoServicoDeTeste(t, agora, c.fonte)
			resultado, err := s.AtualizarTodas(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if jsonDe(t, resultado) != jsonDe(t, c.esperado) {
				t.Errorf("resultado %+v, esperado %+v", resultado, c.esperado)
			}
			marcou := linha(t, db, `SELECT Valor FROM Configuracao WHERE Chave = ?`, ChaveUltimaAtualizacao) != nil
			if marcou != c.marcouQuando {
				t.Errorf("marcou a atualização: %v, esperado %v", marcou, c.marcouQuando)
			}
		})
	}
}

func TestUltimas(t *testing.T) {
	t.Parallel()
	agora := testeapoio.AgoraEmBrasilia(2026, 10, 14, 15, 0)
	s, _, fab := novoServicoDeTeste(t, agora, &fonteFalsa{})
	for _, c := range []testeapoio.CotacaoMoeda{
		{Moeda: "USD", Data: dia(13), Compra: valor(t, "5.4012"), Venda: valor(t, "5.4018")},
		{Moeda: "USD", Data: dia(14), Compra: valor(t, "5.4123"), Venda: valor(t, "5.4129")},
		{Moeda: "EUR", Data: dia(9), Compra: valor(t, "6.2981"), Venda: valor(t, "6.2995")},
	} {
		fab.CotacaoMoeda(func(x *testeapoio.CotacaoMoeda) {
			x.Moeda, x.Data, x.Compra, x.Venda = c.Moeda, c.Data, c.Compra, c.Venda
		})
	}
	fab.Configuracao(ChaveUltimaAtualizacao, "2026-10-14T16:31:02.5000000Z")
	ultimas, err := s.Ultimas(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	esperado := `{"atualizadoEm":"2026-10-14T16:31:02.5Z","cotacoes":[` +
		`{"moeda":"EUR","dataSolicitada":"2026-10-14","data":"2026-10-09","compra":6.2981,"venda":6.2995,"fonte":"PTAX/Banco Central"},` +
		`{"moeda":"USD","dataSolicitada":"2026-10-14","data":"2026-10-14","compra":5.4123,"venda":5.4129,"fonte":"PTAX/Banco Central"}]}`
	if obtido := jsonDe(t, ultimas); obtido != esperado {
		t.Errorf("JSON\n%s\nesperado\n%s", obtido, esperado)
	}

	vazio, _, _ := novoServicoDeTeste(t, agora, &fonteFalsa{})
	nada, err := vazio.Ultimas(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if obtido := jsonDe(t, nada); obtido != `{"atualizadoEm":null,"cotacoes":[]}` {
		t.Errorf("sem nada guardado: %s (lista vazia é [], não null)", obtido)
	}
}

func TestGarantirHistorico(t *testing.T) {
	t.Parallel()
	agora := testeapoio.AgoraEmBrasilia(2026, 10, 14, 15, 0)
	casos := []struct {
		nome      string
		desde     tipos.Data
		minimo    *tipos.Data // a cotação mais antiga guardada
		consultas []string
	}{
		{"real: nada a fazer", dia(1), nil, []string{}},
		{"desde no futuro: nada a fazer", dia(20), nil, []string{}},
		{"já cobre o começo: nada a baixar", dia(1), new(dia(6)), []string{}},
		{"nada guardado: de 7 dias antes até hoje", dia(1), nil, []string{"USD 2026-09-24 2026-10-14"}},
		{"guardado só do meio: até a véspera do primeiro", dia(1), new(dia(12)), []string{"USD 2026-09-24 2026-10-11"}},
		{"mais de um ano: um ano por consulta", tipos.NovaData(2025, 3, 10), nil, []string{"USD 2025-03-03 2026-03-02", "USD 2026-03-03 2026-10-14"}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			fonte := &fonteFalsa{}
			s, _, fab := novoServicoDeTeste(t, agora, fonte)
			if c.minimo != nil {
				fab.CotacaoMoeda(func(x *testeapoio.CotacaoMoeda) { x.Data = *c.minimo })
			}
			moeda := "USD"
			if c.nome == "real: nada a fazer" {
				moeda = "BRL"
			}
			if err := s.GarantirHistorico(t.Context(), moeda, c.desde); err != nil {
				t.Fatal(err)
			}
			if consultas := fonte.consultou(); strings.Join(consultas, "|") != strings.Join(c.consultas, "|") {
				t.Errorf("consultas %v, esperado %v", consultas, c.consultas)
			}
		})
	}
}

func TestSomarAnosComoOCSharp(t *testing.T) {
	t.Parallel()
	// O GarantirHistorico anda um ano por vez: 29/02 + 1 ano é 28/02 no C# (DateOnly.AddYears).
	casos := []struct{ de, esperado tipos.Data }{
		{tipos.NovaData(2024, 2, 29), tipos.NovaData(2025, 2, 28)},
		{tipos.NovaData(2025, 3, 3), tipos.NovaData(2026, 3, 3)},
		{tipos.NovaData(2025, 12, 31), tipos.NovaData(2026, 12, 31)},
	}
	for _, c := range casos {
		if obtido := c.de.SomarAnos(1); !obtido.Igual(c.esperado) {
			t.Errorf("%s + 1 ano = %s, esperado %s", c.de, obtido, c.esperado)
		}
	}
}
