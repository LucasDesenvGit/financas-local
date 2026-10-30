package carteira

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cambio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cotacoes"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/reservas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/selic"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Os falsos da carteira: o conversor, as reservas e a Selic que o teste escolher.
type cambioFalso struct {
	conversor   *cambio.Conversor
	erro        error
	cotacao     *cambio.Cotacao
	erroCotacao error
	moedas      []cambio.Moeda
	erroMoedas  error
}

func (c cambioFalso) ObterConversor(context.Context, *tipos.Data) (*cambio.Conversor, error) {
	return c.conversor, c.erro
}

func (c cambioFalso) ObterCotacao(context.Context, string, *tipos.Data) (*cambio.Cotacao, error) {
	return c.cotacao, c.erroCotacao
}

func (c cambioFalso) ListarMoedas(context.Context) ([]cambio.Moeda, error) {
	return c.moedas, c.erroMoedas
}

type reservasFalsas struct {
	lista []reservas.ReservaDto
	erro  error
}

func (r reservasFalsas) Listar(context.Context) ([]reservas.ReservaDto, error) {
	return r.lista, r.erro
}

type selicFalsa struct {
	serie    *selic.Serie
	erro     error
	meta     *tipos.Decimal
	erroMeta error
	inicios  *[]string
}

func (s selicFalsa) ObterSerie(_ context.Context, inicio tipos.Data) (*selic.Serie, error) {
	if s.inicios != nil {
		*s.inicios = append(*s.inicios, inicio.String())
	}
	return s.serie, s.erro
}

func (s selicFalsa) MetaAtual(context.Context) (*tipos.Decimal, error) { return s.meta, s.erroMeta }

// hoje dos testes: 30/10/2026, às 15h em Brasília (18h em UTC).
type cena struct {
	s     *Servico
	db    *sql.DB
	fab   *testeapoio.Fabrica
	saida *bytes.Buffer
	mux   *http.ServeMux
}

// cofreFalso guarda o segredo com uma marca, para o teste ver que passou pelo cofre.
type cofreFalso struct{}

func (cofreFalso) Cifrar(_, segredo string) (string, error)   { return "cifrado:" + segredo, nil }
func (cofreFalso) Decifrar(_, gravado string) (string, error) { return gravado, nil }

// cotacoesFalsas anotam o que foi pedido.
type cotacoesFalsas struct {
	resultado cotacoes.Resultado
	erro      error
	pedidos   *[][]int64
}

func (c cotacoesFalsas) Atualizar(_ context.Context, somente []int64) (cotacoes.Resultado, error) {
	if c.pedidos != nil {
		*c.pedidos = append(*c.pedidos, somente)
	}
	return c.resultado, c.erro
}

func novaCena(t *testing.T, c cambioFalso, r reservasFalsas, sf selicFalsa) cena {
	return cenaComCotacoes(t, c, r, sf, cotacoesFalsas{})
}

func cenaComCotacoes(t *testing.T, c cambioFalso, r reservasFalsas, sf selicFalsa, a cotacoesFalsas) cena {
	t.Helper()
	db := testeapoio.NovoBancoVazio(t)
	rel := testeapoio.AgoraEmBrasilia(2026, 10, 30, 15, 0)
	if c.conversor == nil && c.erro == nil {
		c.conversor = cambio.NovoConversor(nil, nil)
	}
	if sf.serie == nil {
		sf.serie = selic.NovaSerie(nil)
	}
	var saida bytes.Buffer
	s := NovoServico(db, c, r, sf, a, cofreFalso{}, rel, slog.New(slog.NewTextHandler(&saida, nil)))
	mux := http.NewServeMux()
	Registrar(mux, s)
	return cena{s: s, db: db, fab: testeapoio.NovaFabrica(t, db, rel), saida: &saida, mux: mux}
}

func cenaSimples(t *testing.T) cena {
	return novaCena(t, cambioFalso{}, reservasFalsas{}, selicFalsa{})
}

func (c cena) montar(t *testing.T) *CarteiraDto {
	t.Helper()
	dto, err := c.s.Montar(t.Context(), selic.NovaSerie(nil), nil)
	if err != nil {
		t.Fatal(err)
	}
	return dto
}

func textoDe(s string) *string { return &s }

func diaDoAno(mes time.Month, dia int) tipos.Data { return testeapoio.Dia(2026, mes, dia) }

// Regra: a Selic começa um ano atrás, ou antes, na primeira operação ou aplicação da conexão ativa; nunca antes de 2000.
func TestInicioDaSerieSelic(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome     string
		preparo  func(x testeapoio.ConexaoBancaria, c cena)
		esperado string
	}{
		{"sem nada: um ano atrás", func(testeapoio.ConexaoBancaria, cena) {}, "2025-10-30"},
		{"a operação mais velha", func(_ testeapoio.ConexaoBancaria, c cena) {
			c.fab.OperacaoAtivo(c.fab.Ativo(), func(o *testeapoio.OperacaoAtivo) { o.Data = testeapoio.Dia(2024, 3, 1) })
		}, "2024-03-01"},
		{"a operação recente não muda", func(_ testeapoio.ConexaoBancaria, c cena) {
			c.fab.OperacaoAtivo(c.fab.Ativo(), func(o *testeapoio.OperacaoAtivo) { o.Data = testeapoio.Dia(2026, 3, 1) })
		}, "2025-10-30"},
		{"a aplicação mais velha", func(x testeapoio.ConexaoBancaria, c cena) {
			d := testeapoio.Dia(2023, 3, 1)
			c.fab.Investimento(x, func(i *testeapoio.Investimento) { i.DataAplicacao = &d })
		}, "2023-03-01"},
		{"a aplicação recente não muda", func(x testeapoio.ConexaoBancaria, c cena) {
			d := testeapoio.Dia(2026, 3, 1)
			c.fab.Investimento(x, func(i *testeapoio.Investimento) { i.DataAplicacao = &d })
		}, "2025-10-30"},
		{"antes de 2000", func(_ testeapoio.ConexaoBancaria, c cena) {
			c.fab.OperacaoAtivo(c.fab.Ativo(), func(o *testeapoio.OperacaoAtivo) { o.Data = testeapoio.Dia(1999, 12, 31) })
		}, "2000-01-01"},
	}
	for _, x := range casos {
		t.Run(x.nome, func(t *testing.T) {
			t.Parallel()
			c := cenaSimples(t)
			x.preparo(c.fab.ConexaoBancaria(), c)
			inicio, err := c.s.InicioDaSerieSelic(t.Context())
			if err != nil || inicio.String() != x.esperado {
				t.Errorf("início %s, %v; esperado %s", inicio, err, x.esperado)
			}
		})
	}
}

// Regra: a aplicação da conexão desligada não puxa o começo da Selic.
func TestInicioDaSerieIgnoraConexaoDesligada(t *testing.T) {
	t.Parallel()
	c := cenaSimples(t)
	d := testeapoio.Dia(2023, 3, 1)
	c.fab.Investimento(c.fab.ConexaoBancaria(func(x *testeapoio.ConexaoBancaria) { x.Ativa = false }), func(i *testeapoio.Investimento) { i.DataAplicacao = &d })
	if inicio, err := c.s.InicioDaSerieSelic(t.Context()); err != nil || inicio.String() != "2025-10-30" {
		t.Errorf("início %s, %v", inicio, err)
	}
}

// Regra: a carteira vazia sai com as listas vazias, nunca null, e os percentuais sem base ficam null.
func TestCarteiraVazia(t *testing.T) {
	t.Parallel()
	dados, err := json.Marshal(cenaSimples(t).montar(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, trecho := range []string{`"posicoes":[]`, `"evolucao":[]`, `"openFinance":[]`, `"distribuicao":[]`, `"moedasSemCotacao":[]`, `"reservas":[]`,
		`"cotasDiferentes":[]`, `"valorizacaoPercentual":null`, `"acumulado12Meses":null`, `"atualizadaAte":null`, `"cotacoesAtualizadasEm":null`} {
		if !bytes.Contains(dados, []byte(trecho)) {
			t.Errorf("falta %s em %s", trecho, dados)
		}
	}
}

// Regra: o segmento informado vale; sem ele, só o FII ganha a sugestão.
func TestSegmentoDaPosicao(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome     string
		tipo     int64
		segmento *string
		esperado string
	}{
		{"FII sem segmento", tipoFii, nil, "Logística"},
		{"FII com outro segmento", tipoFii, textoDe("Híbrido"), "Híbrido"},
		{"ação com o ticker de um FII", tipoAcao, nil, ""},
	}
	for _, x := range casos {
		t.Run(x.nome, func(t *testing.T) {
			t.Parallel()
			c := cenaSimples(t)
			c.fab.Ativo(func(a *testeapoio.Ativo) { a.Ticker, a.Tipo, a.Segmento = "HGLG11", x.tipo, x.segmento })
			obtido := ""
			if s := c.montar(t).Posicoes[0].Segmento; s != nil {
				obtido = *s
			}
			if obtido != x.esperado {
				t.Errorf("segmento %q, esperado %q", obtido, x.esperado)
			}
		})
	}
}

// Regra: a evolução para em 240 meses, a partir do mês da primeira operação; o valor de mercado vai só no mês de hoje.
func TestEvolucaoTemNoMaximo240Meses(t *testing.T) {
	t.Parallel()
	c := cenaSimples(t)
	c.fab.OperacaoAtivo(c.fab.Ativo(), func(o *testeapoio.OperacaoAtivo) { o.Data = testeapoio.Dia(1996, 2, 20) })
	evolucao := c.montar(t).Evolucao
	if len(evolucao) != 240 || evolucao[0].Ano != 1996 || evolucao[0].Mes != 2 || evolucao[239].Ano != 2016 || evolucao[239].Mes != 1 {
		t.Fatalf("%d meses, de %+v a %+v", len(evolucao), evolucao[0], evolucao[len(evolucao)-1])
	}
	if evolucao[239].ValorMercado != nil {
		t.Errorf("mês antigo com valor de mercado: %s", evolucao[239].ValorMercado)
	}
}

func TestEvolucaoTerminaEmHoje(t *testing.T) {
	t.Parallel()
	c := cenaSimples(t)
	c.fab.OperacaoAtivo(c.fab.Ativo(), func(o *testeapoio.OperacaoAtivo) { o.Data = diaDoAno(9, 2) })
	evolucao := c.montar(t).Evolucao
	if len(evolucao) != 2 || evolucao[0].ValorMercado != nil || evolucao[1].ValorMercado == nil || evolucao[1].ValorMercado.String() != "100.00" {
		t.Errorf("evolução %+v", evolucao)
	}
}

// Regra: o "% da Selic" só existe com a rentabilidade e com a Selic do período acima de 0,05; sem data de aplicação, ou
// com ela em hoje, ou com a série vazia, não há comparação.
func TestComparadoComSelic(t *testing.T) {
	t.Parallel()
	ontem, hoje, antes := diaDoAno(10, 29), diaDoAno(10, 30), diaDoAno(10, 1)
	aplicado := testeapoio.Valor("100.0")
	cheia := selic.NovaSerie([]selic.Taxa{{Data: antes, Taxa: testeapoio.Valor("1.0")}, {Data: ontem, Taxa: testeapoio.Valor("0.05")}})
	casos := []struct {
		nome          string
		aplicacao     *tipos.Data
		valorAplicado *tipos.Decimal
		serie         *selic.Serie
		selicEsperada string
		percentual    string
		rentabilidade string
	}{
		{"acima de 0,05", &antes, &aplicado, cheia, "1.05", "95.2", "1.00"},
		{"0,05 exatos", &ontem, &aplicado, cheia, "0.05", "", "1.00"},
		{"sem a rentabilidade", &antes, nil, cheia, "1.05", "", ""},
		{"aplicado zero", &antes, opcional(testeapoio.Valor("0.0")), cheia, "1.05", "", ""},
		{"aplicado hoje", &hoje, &aplicado, cheia, "", "", "1.00"},
		{"sem a data", nil, &aplicado, cheia, "", "", "1.00"},
		{"série vazia", &antes, &aplicado, selic.NovaSerie(nil), "", "", "1.00"},
	}
	for _, x := range casos {
		t.Run(x.nome, func(t *testing.T) {
			t.Parallel()
			var c conta
			i := investimento{Nome: "CDB", Tipo: "FIXED_INCOME", Moeda: "BRL", Saldo: testeapoio.Valor("101.0"), ValorAplicado: x.valorAplicado, DataAplicacao: x.aplicacao}
			d := comparadoComSelic(i, x.serie, cambio.NovoConversor(nil, nil), hoje, map[int64]string{}, map[string]bool{}, &c)
			if c.err != nil {
				t.Fatal(c.err)
			}
			valor := func(p *tipos.Decimal) string {
				if p == nil {
					return ""
				}
				return p.String()
			}
			if valor(d.SelicNoPeriodo) != x.selicEsperada || valor(d.PercentualDaSelic) != x.percentual || valor(d.Rentabilidade) != x.rentabilidade {
				t.Errorf("selic %s, %% %s, rentabilidade %s", valor(d.SelicNoPeriodo), valor(d.PercentualDaSelic), valor(d.Rentabilidade))
			}
		})
	}
}

// Regra: as cotas na corretora: só ações e ETFs, com quantidade e saldo; o ticker pelo código ou pelo nome; o mesmo
// ticker soma.
func TestCotasNaCorretora(t *testing.T) {
	t.Parallel()
	cinco, zero := opcional(testeapoio.Valor("5.0")), opcional(testeapoio.Valor("0.0"))
	um, saldo := opcional(testeapoio.Valor("1.5")), testeapoio.Valor("100.0")
	lista := []investimento{
		{Nome: "Renda fixa", Tipo: "COE", Codigo: textoDe("ERRO3"), Quantidade: cinco, Saldo: saldo},
		{Nome: "Sem quantidade", Tipo: "EQUITY", Codigo: textoDe("SEMQ3"), Saldo: saldo},
		{Nome: "Quantidade zero", Tipo: "EQUITY", Codigo: textoDe("ZERO3"), Quantidade: zero, Saldo: saldo},
		{Nome: "Saldo zero", Tipo: "EQUITY", Codigo: textoDe("SALD3"), Quantidade: cinco, Saldo: testeapoio.Valor("0.0")},
		{Nome: "Não é ticker", Tipo: "EQUITY", Quantidade: cinco, Saldo: saldo},
		{Nome: "PETROBRAS", Tipo: "EQUITY", Codigo: textoDe("petr4"), Quantidade: cinco, Saldo: saldo},
		{Nome: "PETR4", Tipo: "ETF", Quantidade: um, Saldo: saldo},
	}
	cotas := cotasNaCorretora(lista)
	if len(cotas) != 1 || cotas["PETR4"].String() != "6.5" {
		t.Errorf("cotas %v", cotas)
	}
}

// Regra: as cotas diferentes: só o ticker que está na carteira e que difere em 0,000001 ou mais.
func TestCotasDiferentes(t *testing.T) {
	t.Parallel()
	c := cenaSimples(t)
	conexao := c.fab.ConexaoBancaria()
	for _, ticker := range []string{"IGUA3", "PERT3", "DIFE3"} {
		c.fab.OperacaoAtivo(c.fab.Ativo(func(a *testeapoio.Ativo) { a.Ticker = ticker }))
	}
	for _, x := range []struct{ ticker, quantidade string }{{"IGUA3", "10.0"}, {"PERT3", "10.0000009"}, {"DIFE3", "10.000001"}, {"FORA3", "3.0"}} {
		q := testeapoio.Valor(x.quantidade)
		c.fab.Investimento(conexao, func(i *testeapoio.Investimento) {
			i.Nome, i.Tipo, i.Codigo, i.Quantidade = x.ticker, "EQUITY", &x.ticker, &q
		})
	}
	if obtido := c.montar(t).CotasDiferentes; strings.Join(obtido, ",") != "DIFE3" {
		t.Errorf("cotas diferentes %v", obtido)
	}
}

// Regra: a cotação de um ativo em carteira, fora o preço manual, vence em 4 horas; a mais antiga vai para a tela.
func TestCotacaoDesatualizada(t *testing.T) {
	t.Parallel()
	const agora = "2026-10-30 18:00:00"
	casos := []struct {
		nome      string
		em        string
		manual    bool
		zerada    bool
		vencida   bool
		maisVelha string
	}{
		{nome: "agora", em: agora, maisVelha: "2026-10-30T18:00:00"},
		{nome: "4 horas exatas", em: "2026-10-30 14:00:00", maisVelha: "2026-10-30T14:00:00"},
		{nome: "mais de 4 horas", em: "2026-10-30 13:59:59", vencida: true, maisVelha: "2026-10-30T13:59:59"},
		{nome: "nunca atualizada", vencida: true},
		{nome: "preço manual", em: "2026-01-01 12:00:00", manual: true},
		{nome: "fora da carteira", em: "2026-01-01 12:00:00", zerada: true},
	}
	for _, x := range casos {
		t.Run(x.nome, func(t *testing.T) {
			t.Parallel()
			c := cenaSimples(t)
			ativo := c.fab.Ativo(func(a *testeapoio.Ativo) {
				a.PrecoManual = x.manual
				if x.em != "" {
					d, err := tipos.LerDataHora(x.em)
					if err != nil {
						t.Fatal(err)
					}
					a.PrecoAtualizadoEm = &d
				}
			})
			if !x.zerada {
				c.fab.OperacaoAtivo(ativo)
			}
			dto := c.montar(t)
			velha := ""
			if dto.CotacoesAtualizadasEm != nil {
				dados, err := dto.CotacoesAtualizadasEm.MarshalJSON()
				if err != nil {
					t.Fatal(err)
				}
				velha = strings.Trim(string(dados), `"`)
			}
			if dto.CotacoesDesatualizadas != x.vencida || velha != x.maisVelha {
				t.Errorf("desatualizadas %v, mais antiga %q", dto.CotacoesDesatualizadas, velha)
			}
		})
	}
}

// Regra: a primeira falha da conta é a que fica, e o valor que falhou não vira resultado.
func TestContaGuardaOPrimeiroErro(t *testing.T) {
	t.Parallel()
	var c conta
	if r := c.dividir(testeapoio.Valor("6"), testeapoio.Valor("3")); r.String() != "2" || c.err != nil {
		t.Fatalf("6 / 3 = %s, %v", r, c.err)
	}
	enorme := testeapoio.Valor("79228162514264337593543950335")
	serie := selic.NovaSerie([]selic.Taxa{{Data: diaDoAno(1, 1), Taxa: enorme}, {Data: diaDoAno(1, 2), Taxa: enorme}})
	if r := c.fator(serie, diaDoAno(2, 1), diaDoAno(2, 2)); r.String() != "1" || c.err != nil {
		t.Fatalf("fator %s, %v", r, c.err)
	}
	c.dividir(um, tipos.Zero)
	if !errors.Is(c.err, tipos.ErrDivisaoPorZero) {
		t.Fatalf("erro %v", c.err)
	}
	c.fator(serie, diaDoAno(1, 1), diaDoAno(1, 3))
	if !errors.Is(c.err, tipos.ErrDivisaoPorZero) {
		t.Errorf("o segundo erro apagou o primeiro: %v", c.err)
	}
	var f conta
	f.fator(serie, diaDoAno(1, 1), diaDoAno(1, 3))
	if !errors.Is(f.err, tipos.ErrEstouro) {
		t.Errorf("fator: %v", f.err)
	}
	f.dividir(um, tipos.Zero)
	if !errors.Is(f.err, tipos.ErrEstouro) {
		t.Errorf("a divisão apagou o primeiro erro: %v", f.err)
	}
}

// Regra: as rotas do CarteiraController: a visão completa com a Selic desde o início e a meta; as operações com a Selic
// desde a primeira operação (sem operação, hoje); ativo que não existe é a mensagem; id que não é int nem casa com a rota.
func TestRotasDaCarteira(t *testing.T) {
	t.Parallel()
	var inicios []string
	meta := testeapoio.Valor("15.00")
	c := novaCena(t, cambioFalso{}, reservasFalsas{}, selicFalsa{meta: &meta, inicios: &inicios})
	ativo := c.fab.Ativo()
	c.fab.OperacaoAtivo(ativo, func(o *testeapoio.OperacaoAtivo) { o.Data = diaDoAno(5, 2) })
	semOperacao := c.fab.Ativo()

	r := testeapoio.Chamar(t, c.mux, "GET", "/api/carteira", "")
	r.ConferirStatus(t, http.StatusOK)
	if !bytes.Contains(r.Corpo, []byte(`"metaAnual":15.00`)) || len(inicios) != 1 || inicios[0] != "2025-10-30" {
		t.Errorf("corpo %.80s, inícios %v", r.Corpo, inicios)
	}

	inicios = nil
	r = testeapoio.Chamar(t, c.mux, "GET", "/api/carteira/ativos/+1/operacoes", "")
	r.ConferirStatus(t, http.StatusOK)
	if !bytes.Contains(r.Corpo, []byte(`"ticker":"`+ativo.Ticker+`"`)) || len(inicios) != 1 || inicios[0] != "2026-05-02" {
		t.Errorf("corpo %.80s, inícios %v", r.Corpo, inicios)
	}

	inicios = nil
	testeapoio.Chamar(t, c.mux, "GET", "/api/carteira/ativos/"+strconv.FormatInt(semOperacao.Id, 10)+"/operacoes", "").ConferirJSON(t, "[]")
	if len(inicios) != 1 || inicios[0] != "2026-10-30" {
		t.Errorf("inícios %v", inicios)
	}

	naoExiste := testeapoio.Chamar(t, c.mux, "GET", "/api/carteira/ativos/999/operacoes", "")
	naoExiste.ConferirStatus(t, http.StatusNotFound)
	naoExiste.ConferirJSON(t, `{"mensagem":"Registro não encontrado."}`)
	for _, id := range []string{"abc", "2147483648", "1.5"} {
		r := testeapoio.Chamar(t, c.mux, "GET", "/api/carteira/ativos/"+id+"/operacoes", "")
		r.ConferirStatus(t, http.StatusNotFound)
		if len(r.Corpo) != 0 {
			t.Errorf("id %s com corpo %s", id, r.Corpo)
		}
	}
}

// Regra: erro de qualquer dependência é o 500 genérico, com o erro no log.
func TestRotasComErro(t *testing.T) {
	t.Parallel()
	falha := errors.New("falhou")
	casos := []struct {
		nome    string
		cambio  cambioFalso
		res     reservasFalsas
		sf      selicFalsa
		caminho string
	}{
		{nome: "a série, na carteira", sf: selicFalsa{erro: falha}, caminho: "/api/carteira"},
		{nome: "a meta", sf: selicFalsa{erroMeta: falha}, caminho: "/api/carteira"},
		{nome: "o câmbio", cambio: cambioFalso{erro: falha}, caminho: "/api/carteira"},
		{nome: "as reservas", res: reservasFalsas{erro: falha}, caminho: "/api/carteira"},
		{nome: "a série, nas operações", sf: selicFalsa{erro: falha}, caminho: "/api/carteira/ativos/1/operacoes"},
		{nome: "o câmbio, nas operações", cambio: cambioFalso{erro: falha}, caminho: "/api/carteira/ativos/1/operacoes"},
	}
	for _, x := range casos {
		t.Run(x.nome, func(t *testing.T) {
			t.Parallel()
			c := novaCena(t, x.cambio, x.res, x.sf)
			c.fab.OperacaoAtivo(c.fab.Ativo())
			testeapoio.Chamar(t, c.mux, "GET", x.caminho, "").ConferirStatus(t, http.StatusInternalServerError)
			if !strings.Contains(c.saida.String(), "erro na carteira") {
				t.Errorf("log %q", c.saida.String())
			}
		})
	}
}

// Regra: banco fora do ar é 500 nas duas rotas, em cada leitura.
func TestRotasComErroDoBanco(t *testing.T) {
	t.Parallel()
	for _, tabela := range []string{"Ativo", "OperacaoAtivo", "ProventoAtivo", "Investimento", "ConexaoBancaria", "Conta"} {
		t.Run(tabela, func(t *testing.T) {
			t.Parallel()
			c := cenaSimples(t)
			if _, err := c.db.Exec(`DROP TABLE "` + tabela + `"`); err != nil {
				t.Fatal(err)
			}
			testeapoio.Chamar(t, c.mux, "GET", "/api/carteira", "").ConferirStatus(t, http.StatusInternalServerError)
		})
	}
	c := cenaSimples(t)
	if err := c.db.Close(); err != nil {
		t.Fatal(err)
	}
	testeapoio.Chamar(t, c.mux, "GET", "/api/carteira/ativos/1/operacoes", "").ConferirStatus(t, http.StatusInternalServerError)
}

// Regra: pedido cancelado não é erro (nem resposta, nem log), venha o cancelamento no erro ou no contexto do pedido.
func TestPedidoCanceladoNaoEhErro(t *testing.T) {
	t.Parallel()
	cancelado, cancelar := context.WithCancel(t.Context())
	cancelar()
	casos := []struct {
		nome string
		ctx  context.Context
		err  error
	}{
		{"o erro é o cancelamento", t.Context(), context.Canceled},
		{"o pedido foi cancelado", cancelado, errors.New("outro erro")},
	}
	for _, x := range casos {
		t.Run(x.nome, func(t *testing.T) {
			t.Parallel()
			c := cenaSimples(t)
			gravador := httptest.NewRecorder()
			c.s.erroInterno(gravador, httptest.NewRequestWithContext(x.ctx, http.MethodGet, "/api/carteira", nil), "montar", x.err)
			if gravador.Body.Len() != 0 || c.saida.Len() != 0 {
				t.Errorf("corpo %q, log %q", gravador.Body.String(), c.saida.String())
			}
		})
	}
}

// Regra: os nomes das classes da distribuição, os do Open Finance sem diferença de maiúsculas, e o segmento válido.
func TestNomesDasClasses(t *testing.T) {
	t.Parallel()
	classes := map[int64]string{tipoFii: "Fundos imobiliários", tipoAcao: "Ações", tipoEtf: "ETFs", tipoBdr: "BDRs", tipoOutro: "Outros (renda variável)", 99: "Outros (renda variável)"}
	for tipo, esperado := range classes {
		if n := nomeDoTipo(tipo); n != esperado {
			t.Errorf("nomeDoTipo(%d) = %q", tipo, n)
		}
	}
	pluggy := map[string]string{"fixed_income": "Renda fixa", "SECURITY": "Previdência", "MUTUAL_FUND": "Fundos de investimento", "EQUITY": "Ações",
		"Etf": "ETFs", "coe": "COE", "OUTRO": "Outros investimentos", "": "Outros investimentos"}
	for tipo, esperado := range pluggy {
		if n := nomeDoTipoPluggy(tipo); n != esperado {
			t.Errorf("nomeDoTipoPluggy(%q) = %q", tipo, n)
		}
	}
	if !ehRendaVariavel("equity") || !ehRendaVariavel("ETF") || ehRendaVariavel("COE") {
		t.Error("renda variável")
	}
	if !SegmentoValido(nil) || !SegmentoValido(textoDe("Logística")) || SegmentoValido(textoDe("logística")) || SegmentoValido(textoDe("")) {
		t.Error("segmento válido")
	}
}

// Regra: atualizar as cotações: sem o ativoId, ou vazio, são todos; com ele, só aquele; o que não é número é o 400 do model
// binding; erro é o 500.
func TestRotaAtualizarCotacoes(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome     string
		consulta string
		status   int
		pedidos  string
	}{
		{"todos", "", 200, "[[]]"},
		{"vazio", "?ativoId=", 200, "[[]]"},
		{"só espaços", "?ativoId=%20", 200, "[[]]"},
		{"um", "?ativoId=7", 200, "[[7]]"},
		{"sem diferença de maiúsculas", "?ATIVOID=7", 200, "[[7]]"},
		{"não é número", "?ativoId=abc", 400, "[]"},
	}
	for _, x := range casos {
		t.Run(x.nome, func(t *testing.T) {
			t.Parallel()
			var pedidos [][]int64
			c := cenaComCotacoes(t, cambioFalso{}, reservasFalsas{}, selicFalsa{}, cotacoesFalsas{
				resultado: cotacoes.Resultado{Atualizados: 2, ProventosNovos: 1, Falhas: []string{"PETR4"}}, pedidos: &pedidos})
			r := testeapoio.Chamar(t, c.mux, "POST", "/api/carteira/atualizar-cotacoes"+x.consulta, "")
			r.ConferirStatus(t, x.status)
			if x.status == 200 {
				r.ConferirJSON(t, `{"atualizados":2,"proventosNovos":1,"falhas":["PETR4"]}`)
			}
			if obtido := strings.ReplaceAll(fmt.Sprint(pedidos), " ", ","); obtido != x.pedidos {
				t.Errorf("pedidos %s, esperado %s", obtido, x.pedidos)
			}
		})
	}
	c := cenaComCotacoes(t, cambioFalso{}, reservasFalsas{}, selicFalsa{}, cotacoesFalsas{erro: errors.New("falhou")})
	testeapoio.Chamar(t, c.mux, "POST", "/api/carteira/atualizar-cotacoes", "").ConferirStatus(t, http.StatusInternalServerError)
}
