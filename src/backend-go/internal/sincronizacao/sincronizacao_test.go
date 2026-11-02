package sincronizacao

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/categorizacao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/pluggy"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/regrasdescricao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/segredos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
)

// O cenário usa os mesmos arquivos da Pluggy falsa do contrato (testes/contrato/dados/pluggy): fictícios.
const item = "a1b2c3d4-0000-4000-8000-000000000001"

type cofreFalso struct{ erro error }

func (cofreFalso) Cifrar(_, segredo string) (string, error) { return segredo, nil }
func (c cofreFalso) Decifrar(_, gravado string) (string, error) {
	return gravado, c.erro
}

type trocosFalsos struct {
	chamadas int
	erro     error
}

func (f *trocosFalsos) ClassificarTrocosSemDuvida(context.Context) (int, error) {
	f.chamadas++
	return 0, f.erro
}

type cenario struct {
	s      *Servico
	db     *sql.DB
	fab    *testeapoio.Fabrica
	falsa  *testeapoio.ServicoDeFora
	trocos *trocosFalsos
}

func arquivo(t *testing.T, nome string) testeapoio.RespostaDeFora {
	t.Helper()
	corpo, err := os.ReadFile(filepath.Join("..", "..", "testes", "contrato", "dados", "pluggy", nome+".json"))
	if err != nil {
		t.Fatal(err)
	}
	return testeapoio.RespostaDeFora{Corpo: string(corpo)}
}

func novoCenario(t *testing.T) cenario {
	t.Helper()
	db := testeapoio.NovoBancoVazio(t)
	r := testeapoio.HojeEmBrasilia(2026, 10, 15)
	falsa := testeapoio.NovoServicoDeFora(t, "X-API-KEY")
	cliente, err := pluggy.NovoCliente(falsa.URL()+"/", falsa.Cliente(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	trocos := &trocosFalsos{}
	s := NovoServico(db, cofreFalso{}, cliente, trocos, r, slog.New(slog.DiscardHandler))
	c := cenario{s, db, testeapoio.NovaFabrica(t, db, r), falsa, trocos}
	c.falsa.Em("POST", "/auth", arquivo(t, "auth"))
	return c
}

func (c cenario) credenciais(t *testing.T) {
	t.Helper()
	for chave, protegido := range map[string]int{chaveClientId: 0, chaveClientSecret: 1} {
		if _, err := c.db.Exec(`INSERT INTO "Configuracao" ("Chave", "Valor", "Protegido", "AtualizadoEm") VALUES (?, 'valor', ?, '2026-01-01 00:00:00')
			ON CONFLICT ("Chave") DO UPDATE SET "Valor" = 'valor', "Protegido" = excluded."Protegido"`, chave, protegido); err != nil {
			t.Fatal(err)
		}
	}
}

// pluggyCompleta cadastra o cenário inteiro, com uma conexão no item dos arquivos.
func (c cenario) pluggyCompleta(t *testing.T) testeapoio.ConexaoBancaria {
	t.Helper()
	c.credenciais(t)
	conexao := c.fab.ConexaoBancaria(func(k *testeapoio.ConexaoBancaria) { k.PluggyItemId = item })
	for rota, nome := range map[string]string{
		"/items/" + item: "item-" + item, "/accounts": "contas-" + item, "/categories": "categorias", "/bills": "faturas-falsa-cartao",
		"/investments": "investimentos-" + item, "/loans": "emprestimos-" + item,
	} {
		c.falsa.Em("GET", rota, arquivo(t, nome))
	}
	c.falsa.Em("GET", "/v2/transactions", testeapoio.RespostaDeFora{Corpo: `{"results":[]}`})
	for _, investimento := range []string{"eq-fii", "eq-acao", "et-sem-historico"} {
		c.falsa.Em("GET", "/investments/"+investimento+"/transactions", arquivo(t, "movimentos-"+investimento))
	}
	return conexao
}

func (c cenario) valor(t *testing.T, consulta string, args ...any) string {
	t.Helper()
	var v sql.NullString
	if err := c.db.QueryRow(consulta, args...).Scan(&v); err != nil {
		t.Fatalf("%s: %v", consulta, err)
	}
	if !v.Valid {
		return "<nil>"
	}
	return v.String
}

func (c cenario) sincronizar(t *testing.T) Resultado {
	t.Helper()
	r, err := c.s.Sincronizar(t.Context(), nil, OrigemManual)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestSincronizarNaoComeca(t *testing.T) {
	casos := []struct {
		nome, mensagem string
		preparar       func(t *testing.T, c cenario)
	}{
		{"sem credenciais", semCredenciais, func(*testing.T, cenario) {}},
		{"sem o Client Secret", semCredenciais, func(t *testing.T, c cenario) {
			c.fab.Configuracao(chaveClientId, "id")
		}},
		{"Client ID em branco", semCredenciais, func(t *testing.T, c cenario) {
			c.credenciais(t)
			if _, err := c.db.Exec(`UPDATE "Configuracao" SET "Valor" = '   ' WHERE "Chave" = ?`, chaveClientId); err != nil {
				t.Fatal(err)
			}
		}},
		{"sem conexão", "Nenhuma conexão ativa para sincronizar.", func(t *testing.T, c cenario) { c.credenciais(t) }},
		{"conexão desligada", "Nenhuma conexão ativa para sincronizar.", func(t *testing.T, c cenario) {
			c.credenciais(t)
			c.fab.ConexaoBancaria(func(k *testeapoio.ConexaoBancaria) { k.Ativa = false })
		}},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			c := novoCenario(t)
			caso.preparar(t, c)
			r := c.sincronizar(t)
			if r.Sucesso || *r.Mensagem != caso.mensagem {
				t.Errorf("resultado = %+v, esperado %q", r, caso.mensagem)
			}
			if n := len(c.falsa.Pedidos()); n != 0 {
				t.Errorf("%d pedidos à Pluggy", n)
			}
		})
	}
}

// Regra (#85, ADR 0049): o segredo gravado pelo .NET não se lê no Go: vale como não configurado, sem derrubar nada.
func TestSegredoDoNetVaiSemCredencial(t *testing.T) {
	c := novoCenario(t)
	c.s.cofre = cofreFalso{erro: segredos.ErrInformarDeNovo}
	c.credenciais(t)
	if r := c.sincronizar(t); *r.Mensagem != semCredenciais {
		t.Errorf("mensagem = %q", *r.Mensagem)
	}
}

func TestPluggyRecusaAsCredenciais(t *testing.T) {
	c := novoCenario(t)
	c.credenciais(t)
	c.fab.ConexaoBancaria()
	c.falsa.Em("POST", "/auth", testeapoio.NaoAutorizado)
	r := c.sincronizar(t)
	if r.Sucesso || !strings.Contains(*r.Mensagem, "recusou o Client ID") {
		t.Fatalf("resultado = %+v", r)
	}
	if m := c.valor(t, `SELECT "Mensagem" FROM "SincronizacaoLog" WHERE "ConexaoBancariaId" IS NULL AND NOT "Sucesso" AND "FinalizadoEm" IS NOT NULL`); !strings.Contains(m, "recusou") {
		t.Errorf("log = %q", m)
	}
}

func TestPrimeiraSincronizacao(t *testing.T) {
	c := novoCenario(t)
	conexao := c.pluggyCompleta(t)
	c.falsa.Em("GET", "/v2/transactions", arquivo(t, "transacoes-falsa-cartao"))
	r := c.sincronizar(t)
	if !r.Sucesso || *r.Mensagem != "Sincronização concluída." {
		t.Fatalf("resultado = %+v", r)
	}
	if r.TransacoesNovas != 3 {
		t.Errorf("novas = %d, esperado 3: a resposta é a mesma para as 3 contas, e a transação não muda de conta", r.TransacoesNovas)
	}
	for consulta, esperado := range map[string]string{
		`SELECT "NomeInstituicao" FROM "ConexaoBancaria" WHERE "Id" = ` + fmt.Sprint(conexao.Id):     "Banco Falso",
		`SELECT "CorPrimaria" FROM "ConexaoBancaria" WHERE "Id" = ` + fmt.Sprint(conexao.Id):         "#ef3e42",
		`SELECT "Status" FROM "ConexaoBancaria" WHERE "Id" = ` + fmt.Sprint(conexao.Id):              "UPDATED",
		`SELECT "UltimoErro" FROM "ConexaoBancaria" WHERE "Id" = ` + fmt.Sprint(conexao.Id):          "<nil>",
		`SELECT "Nome" FROM "Conta" WHERE "PluggyAccountId" = 'falsa-corrente'`:                      "Conta Falsa",
		`SELECT "Nome" FROM "Conta" WHERE "PluggyAccountId" = 'falsa-cartao'`:                        "Cartão Falso",
		`SELECT "Tipo" FROM "Conta" WHERE "PluggyAccountId" = 'falsa-corrente'`:                      "1",
		`SELECT "Tipo" FROM "Conta" WHERE "PluggyAccountId" = 'falsa-poupanca'`:                      "2",
		`SELECT "Tipo" FROM "Conta" WHERE "PluggyAccountId" = 'falsa-cartao'`:                        "3",
		`SELECT "Cor" FROM "Conta" WHERE "PluggyAccountId" = 'falsa-corrente'`:                       "#ef3e42",
		`SELECT "Saldo" FROM "Conta" WHERE "PluggyAccountId" = 'falsa-corrente'`:                     "1520.36",
		`SELECT "Saldo" FROM "Conta" WHERE "PluggyAccountId" = 'falsa-cartao'`:                       "-840.1",
		`SELECT "Moeda" FROM "Conta" WHERE "PluggyAccountId" = 'falsa-poupanca'`:                     "BRL",
		`SELECT "LimiteCredito" FROM "Conta" WHERE "PluggyAccountId" = 'falsa-cartao'`:               "5000.0",
		`SELECT "DiaFechamento" FROM "Conta" WHERE "PluggyAccountId" = 'falsa-cartao'`:               "19",
		`SELECT "DiaVencimento" FROM "Conta" WHERE "PluggyAccountId" = 'falsa-cartao'`:               "28",
		`SELECT "SaldoReservado" FROM "Conta" WHERE "PluggyAccountId" = 'falsa-poupanca'`:            "75.12",
		`SELECT "SaldoReservado" FROM "Conta" WHERE "PluggyAccountId" = 'falsa-cartao'`:              "<nil>",
		`SELECT "ValorTotal" FROM "FaturaCartao" WHERE "PluggyBillId" = 'fatura-1'`:                  "840.1",
		`SELECT "ValorMinimo" FROM "FaturaCartao" WHERE "PluggyBillId" = 'fatura-1'`:                 "84.01",
		`SELECT "ValorPago" FROM "FaturaCartao" WHERE "PluggyBillId" = 'fatura-1'`:                   "0.3",
		`SELECT "ValorPago" FROM "FaturaCartao" WHERE "PluggyBillId" = 'fatura-2'`:                   "<nil>",
		`SELECT "ValorTotal" FROM "FaturaCartao" WHERE "PluggyBillId" = 'fatura-2'`:                  "123.46",
		`SELECT COUNT(*) FROM "FaturaCartao" WHERE "PluggyBillId" = 'fatura-sem-data'`:               "0",
		`SELECT "Saldo" FROM "Investimento" WHERE "PluggyInvestmentId" = 'fi-cdb'`:                   "1050.76",
		`SELECT "TaxaAnual" FROM "Investimento" WHERE "PluggyInvestmentId" = 'fi-cdb'`:               "12.3456",
		`SELECT "TaxaAnual" FROM "Investimento" WHERE "PluggyInvestmentId" = 'fi-taxa-absurda'`:      "<nil>",
		`SELECT "DataAplicacao" FROM "Investimento" WHERE "PluggyInvestmentId" = 'fi-taxa-absurda'`:  "2025-05-01",
		`SELECT COUNT(*) FROM "HistoricoInvestimento"`:                                               "2",
		`SELECT "Natureza" FROM "Emprestimo" WHERE "PluggyLoanId" = 'emp-1'`:                         "2",
		`SELECT "Produto" FROM "Emprestimo" WHERE "PluggyLoanId" = 'emp-1'`:                          "Financiamento Falso",
		`SELECT "SaldoDevedor" FROM "Emprestimo" WHERE "PluggyLoanId" = 'emp-1'`:                     "185000.51",
		`SELECT "ValorParcela" FROM "Emprestimo" WHERE "PluggyLoanId" = 'emp-1'`:                     "1890.13",
		`SELECT "Indexador" FROM "Emprestimo" WHERE "PluggyLoanId" = 'emp-1'`:                        "TR_TBF",
		`SELECT "TaxaJuros" FROM "Emprestimo" WHERE "PluggyLoanId" = 'emp-1'`:                        "0.08755556",
		`SELECT "Cet" FROM "Emprestimo" WHERE "PluggyLoanId" = 'emp-2'`:                              "<nil>",
		`SELECT "ValorParcela" FROM "Emprestimo" WHERE "PluggyLoanId" = 'emp-2'`:                     "33.33",
		`SELECT "SaldoDevedor" FROM "Emprestimo" WHERE "PluggyLoanId" = 'emp-3'`:                     "0.0",
		`SELECT "Natureza" FROM "Emprestimo" WHERE "PluggyLoanId" = 'emp-3'`:                         "2",
		`SELECT COUNT(*) FROM "PagamentoEmprestimo"`:                                                 "3",
		`SELECT COUNT(*) FROM "Ativo" WHERE "Ticker" IN ('FALS11', 'FALS3', 'FALS1')`:                "3",
		`SELECT COUNT(*) FROM "CategoriaPluggy" WHERE "PluggyCategoriaId" = '123456789012345678901'`: "0",
		`SELECT "PaiDescricao" FROM "CategoriaPluggy" WHERE "PluggyCategoriaId" = '01010000'`:        "Renda",
		`SELECT "PaiDescricao" FROM "CategoriaPluggy" WHERE "PluggyCategoriaId" = '07000000'`:        "Digital services",
		`SELECT "Descricao" FROM "CategoriaPluggy" WHERE "PluggyCategoriaId" = '09000000'`:           "Sem tradução e sem mapa",
		`SELECT "Mensagem" FROM "SincronizacaoLog" WHERE "Sucesso"`:                                  "3 conta(s) processada(s).",
		`SELECT "Origem" FROM "SincronizacaoLog" WHERE "Sucesso"`:                                    "Manual",
	} {
		if obtido := c.valor(t, consulta); obtido != esperado {
			t.Errorf("%s = %q, esperado %q", consulta, obtido, esperado)
		}
	}
	if c.trocos.chamadas != 1 {
		t.Errorf("trocos chamados %d vezes", c.trocos.chamadas)
	}
	if m := c.valor(t, `SELECT "Valor" FROM "Configuracao" WHERE "Chave" = 'Sincronizacao.UltimaExecucao'`); !strings.HasPrefix(m, "2026-10-15T") {
		t.Errorf("última execução = %q", m)
	}
}

// Regra (critério da #83): sincronizar duas vezes com as mesmas respostas não muda nada nas transações.
func TestSincronizarDuasVezesNaoMudaNada(t *testing.T) {
	c := novoCenario(t)
	c.pluggyCompleta(t)
	c.falsa.Em("GET", "/v2/transactions", arquivo(t, "transacoes-falsa-cartao"))
	if r := c.sincronizar(t); !r.Sucesso {
		t.Fatalf("primeira: %+v", r)
	}
	fotografia := func() string {
		var b strings.Builder
		linhas, err := c.db.Query(`SELECT "Id", "Descricao", "Valor", "Tipo", "Status", COALESCE("CategoriaId", 0), "IgnorarRelatorios", "AtualizadoEm" FROM "Transacao" ORDER BY "Id"`)
		if err != nil {
			t.Fatal(err)
		}
		defer linhas.Close()
		for linhas.Next() {
			colunas := make([]any, 8)
			for i := range colunas {
				colunas[i] = new(string)
			}
			if err := linhas.Scan(colunas...); err != nil {
				t.Fatal(err)
			}
			for _, v := range colunas {
				b.WriteString(*v.(*string) + "|")
			}
			b.WriteByte('\n')
		}
		return b.String()
	}
	antes := fotografia()
	r := c.sincronizar(t)
	if !r.Sucesso || r.TransacoesNovas != 0 || r.TransacoesAtualizadas != 0 {
		t.Errorf("segunda = %+v, esperado sem novas e sem atualizadas", r)
	}
	if depois := fotografia(); depois != antes {
		t.Errorf("as transações mudaram:\n%s\n%s", antes, depois)
	}
}

// Regra (critério da #83): uma sincronização por vez, sem fila. A segunda volta na hora, e a importação da carteira também.
func TestUmaSincronizacaoPorVez(t *testing.T) {
	c := novoCenario(t)
	c.credenciais(t)
	c.fab.ConexaoBancaria()
	c.s.emExecucao.Store(true)
	if !c.s.EmExecucao() {
		t.Fatal("EmExecucao = false")
	}
	if r := c.sincronizar(t); *r.Mensagem != "Já existe uma sincronização em andamento." {
		t.Errorf("mensagem = %q", *r.Mensagem)
	}
	r, err := c.s.ImportarCarteira(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if *r.Mensagem != "Há uma sincronização em andamento. Tente em instantes." {
		t.Errorf("mensagem = %q", *r.Mensagem)
	}
	if !c.s.EmExecucao() {
		t.Error("a chamada recusada soltou a trava da outra")
	}
}

func TestChamadasAoMesmoTempo(t *testing.T) {
	c := novoCenario(t)
	c.pluggyCompleta(t)
	var grupo sync.WaitGroup
	recusadas := make(chan string, 8)
	for range 8 {
		grupo.Go(func() {
			if r, err := c.s.Sincronizar(context.Background(), nil, OrigemAutomatica); err == nil && r.Mensagem != nil && strings.Contains(*r.Mensagem, "em andamento") {
				recusadas <- *r.Mensagem
			}
		})
	}
	grupo.Wait()
	close(recusadas)
	if c.s.EmExecucao() {
		t.Error("a trava ficou presa")
	}
}

func TestSoltaATravaDepoisDeTerminar(t *testing.T) {
	c := novoCenario(t)
	c.sincronizar(t)
	if c.s.EmExecucao() {
		t.Error("a trava ficou presa")
	}
}

// Regra: a falha fica na conexão e no log, com o que já tinha entrado; a outra conexão roda assim mesmo.
func TestFalhaDeUmaConexao(t *testing.T) {
	c := novoCenario(t)
	boa := c.pluggyCompleta(t)
	ruim := c.fab.ConexaoBancaria(func(k *testeapoio.ConexaoBancaria) { k.PluggyItemId = "item-que-falha" })
	c.falsa.Em("GET", "/items/item-que-falha", testeapoio.RespostaDeFora{Status: 404})
	r := c.sincronizar(t)
	if r.Sucesso {
		t.Fatalf("resultado = %+v", r)
	}
	esperado := "Recurso não encontrado na Pluggy (items/item-que-falha). Confira o Item ID."
	if *r.Mensagem != esperado {
		t.Errorf("mensagem = %q", *r.Mensagem)
	}
	if m := c.valor(t, `SELECT "UltimoErro" FROM "ConexaoBancaria" WHERE "Id" = ?`, ruim.Id); m != esperado {
		t.Errorf("conexão = %q", m)
	}
	if m := c.valor(t, `SELECT "Mensagem" FROM "SincronizacaoLog" WHERE "ConexaoBancariaId" = ? AND NOT "Sucesso" AND "FinalizadoEm" IS NOT NULL`, ruim.Id); m != esperado {
		t.Errorf("log = %q", m)
	}
	if m := c.valor(t, `SELECT "UltimoErro" FROM "ConexaoBancaria" WHERE "Id" = ?`, boa.Id); m != "<nil>" {
		t.Errorf("a conexão boa ficou com erro: %q", m)
	}
}

func TestErroQueNaoEDaPluggyViraErroInterno(t *testing.T) {
	c := novoCenario(t)
	c.pluggyCompleta(t)
	c.falsa.Em("GET", "/accounts", testeapoio.JSONQuebrado)
	r := c.sincronizar(t)
	if r.Sucesso || !strings.HasPrefix(*r.Mensagem, "Erro interno: ") {
		t.Errorf("resultado = %+v", r)
	}
}

func TestErroDaInstituicaoFicaNaConexao(t *testing.T) {
	c := novoCenario(t)
	conexao := c.pluggyCompleta(t)
	c.falsa.Em("GET", "/items/"+item, testeapoio.RespostaDeFora{Corpo: `{"id":"x","connector":{"name":"Outro","primaryColor":"nao-e-cor"},"error":{"message":"pediu confirmação"}}`})
	if r := c.sincronizar(t); !r.Sucesso {
		t.Fatalf("resultado = %+v", r)
	}
	if m := c.valor(t, `SELECT "UltimoErro" FROM "ConexaoBancaria" WHERE "Id" = ?`, conexao.Id); m != "Instituição: pediu confirmação" {
		t.Errorf("erro = %q", m)
	}
	if m := c.valor(t, `SELECT "CorPrimaria" FROM "ConexaoBancaria" WHERE "Id" = ?`, conexao.Id); m != "<nil>" {
		t.Errorf("cor = %q: a cor inválida não entra", m)
	}
}

func TestListaDeCategoriasNaoImpede(t *testing.T) {
	c := novoCenario(t)
	c.pluggyCompleta(t)
	c.falsa.Em("GET", "/categories", testeapoio.RespostaDeFora{Status: 400})
	if r := c.sincronizar(t); !r.Sucesso {
		t.Errorf("resultado = %+v", r)
	}
}

func TestTrocosNaoDesfazem(t *testing.T) {
	c := novoCenario(t)
	c.pluggyCompleta(t)
	c.trocos.erro = errors.New("sem os trocos")
	if r := c.sincronizar(t); !r.Sucesso {
		t.Errorf("resultado = %+v", r)
	}
}

func TestProdutosOpcionais(t *testing.T) {
	casos := []struct {
		nome     string
		resposta testeapoio.RespostaDeFora
		sucesso  bool
	}{
		{"sem o produto", testeapoio.RespostaDeFora{Status: 400}, true},
		{"sem autorização", testeapoio.RespostaDeFora{Status: 403}, true},
		{"não existe", testeapoio.RespostaDeFora{Status: 404}, true},
	}
	for _, caso := range casos {
		t.Run("investimentos "+caso.nome, func(t *testing.T) {
			c := novoCenario(t)
			c.pluggyCompleta(t)
			c.falsa.Em("GET", "/investments", caso.resposta)
			if r := c.sincronizar(t); r.Sucesso != caso.sucesso {
				t.Errorf("resultado = %+v", r)
			}
		})
		t.Run("empréstimos "+caso.nome, func(t *testing.T) {
			c := novoCenario(t)
			c.pluggyCompleta(t)
			c.falsa.Em("GET", "/loans", caso.resposta)
			if r := c.sincronizar(t); r.Sucesso != caso.sucesso {
				t.Errorf("resultado = %+v", r)
			}
		})
	}
	t.Run("empréstimos com a Pluggy fora do ar: segue", func(t *testing.T) {
		c := novoCenario(t)
		c.pluggyCompleta(t)
		c.s.pluggy = c.clienteSemEspera(t)
		c.falsa.Em("GET", "/loans", testeapoio.JSONQuebrado)
		if r := c.sincronizar(t); !r.Sucesso {
			t.Errorf("resultado = %+v", r)
		}
	})
	t.Run("investimentos com erro: falha", func(t *testing.T) {
		c := novoCenario(t)
		c.pluggyCompleta(t)
		c.falsa.Em("GET", "/investments", testeapoio.JSONQuebrado)
		if r := c.sincronizar(t); r.Sucesso {
			t.Errorf("resultado = %+v", r)
		}
	})
}

func (c cenario) clienteSemEspera(t *testing.T) *pluggy.Cliente {
	t.Helper()
	cliente, err := pluggy.NovoCliente(c.falsa.URL()+"/", c.falsa.Cliente(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	return cliente
}

func TestRemoveOQueSumiuDaPluggy(t *testing.T) {
	c := novoCenario(t)
	c.pluggyCompleta(t)
	c.sincronizar(t)
	c.falsa.Em("GET", "/investments", testeapoio.RespostaDeFora{Corpo: `{"totalPages":1,"results":[]}`})
	c.falsa.Em("GET", "/loans", testeapoio.RespostaDeFora{Corpo: `{"totalPages":1,"results":[]}`})
	if r := c.sincronizar(t); !r.Sucesso {
		t.Fatalf("resultado = %+v", r)
	}
	for _, tabela := range []string{"Investimento", "Emprestimo", "PagamentoEmprestimo"} {
		if n := c.valor(t, `SELECT COUNT(*) FROM "`+tabela+`"`); n != "0" {
			t.Errorf("%s: %s linhas", tabela, n)
		}
	}
}

// As regras das transações: uma rodada só, numa conta de teste, sem a sincronização inteira.

type rodadaDeTeste struct {
	cenario
	r     *rodada
	conta contaLocal
}

func novaRodada(t *testing.T, cartao bool) rodadaDeTeste {
	t.Helper()
	c := novoCenario(t)
	conta := c.fab.Conta(func(k *testeapoio.Conta) { k.PluggyAccountId = p("conta-pluggy") })
	regras, err := categorizacao.Carregar(t.Context(), c.db)
	if err != nil {
		t.Fatal(err)
	}
	descricoes, err := regrasdescricao.Carregar(t.Context(), c.db)
	if err != nil {
		t.Fatal(err)
	}
	r := &rodada{Servico: c.s, regras: regras, descricoes: descricoes, origem: OrigemManual}
	return rodadaDeTeste{c, r, contaLocal{id: conta.Id, pluggyId: "conta-pluggy", ehCartao: cartao}}
}

func (x rodadaDeTeste) lancar(t *testing.T, ajuste func(*testeapoio.Transacao)) int64 {
	t.Helper()
	return x.fab.Transacao(testeapoio.Conta{Id: x.conta.id}, ajuste).Id
}

func (x rodadaDeTeste) rodar(t *testing.T, corpo string, dataInicial tipos.Data, mapa map[string]int64) (int, int) {
	t.Helper()
	x.falsa.Em("GET", "/v2/transactions", testeapoio.RespostaDeFora{Corpo: corpo})
	novas, atualizadas, err := x.r.sincronizarTransacoes(t.Context(), x.conta, dataInicial, mapa)
	if err != nil {
		t.Fatal(err)
	}
	return novas, atualizadas
}

func (x rodadaDeTeste) transacao(t *testing.T, id int64) *transacoes.Transacao {
	t.Helper()
	lista, err := transacoes.Listar(t.Context(), x.db, `WHERE "Id" = ?`, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(lista) == 0 {
		return nil
	}
	return lista[0]
}

var inicioDoPeriodo = tipos.NovaData(2026, 10, 1)

func TestNovaTransacao(t *testing.T) {
	x := novaRodada(t, true)
	novas, atualizadas := x.rodar(t, `{"results":[{"id":"n1","date":"2026-10-05T12:00:00Z","description":"  LOJA  ","amount":25.5,"status":"PENDING","categoryId":"12345678901234567890123",
		"creditCardMetadata":{"installmentNumber":2,"totalInstallments":6,"billId":"f1"},"merchant":{"name":"Loja"},"operationType":"cartao"}]}`, inicioDoPeriodo, map[string]int64{})
	if novas != 1 || atualizadas != 0 {
		t.Fatalf("novas %d, atualizadas %d", novas, atualizadas)
	}
	lista, err := transacoes.Listar(t.Context(), x.db, `WHERE "PluggyTransactionId" = 'n1'`)
	if err != nil {
		t.Fatal(err)
	}
	tr := lista[0]
	if tr.Descricao != "LOJA" || *tr.DescricaoOriginal != "LOJA" || tr.Valor.Decimal().String() != "-25.5" || tr.Status != pendente || *tr.Estabelecimento != "Loja" ||
		*tr.ParcelaNumero != 2 || *tr.ParcelaTotal != 6 || *tr.MeioPagamento != "CARTAO" || len(*tr.PluggyCategoriaId) != 20 || tr.ContaId != x.conta.id || tr.FaturaCartaoId != nil {
		t.Errorf("transação = %+v", tr)
	}
}

func TestDescricaoVazia(t *testing.T) {
	x := novaRodada(t, false)
	x.rodar(t, `{"results":[{"id":"n1","date":"2026-10-05T12:00:00Z","description":"   ","amount":1}]}`, inicioDoPeriodo, nil)
	if d := x.valor(t, `SELECT "Descricao" FROM "Transacao" WHERE "PluggyTransactionId" = 'n1'`); d != "(sem descrição)" {
		t.Errorf("descrição = %q", d)
	}
}

func TestIgnoraSemIdSemDataERepetida(t *testing.T) {
	x := novaRodada(t, false)
	novas, _ := x.rodar(t, `{"results":[{"id":"","date":"2026-10-05T12:00:00Z","amount":1},{"id":"sem-data","amount":1},
		{"id":"n1","date":"2026-10-05T12:00:00Z","amount":1},{"id":"n1","date":"2026-10-06T12:00:00Z","amount":2}]}`, inicioDoPeriodo, nil)
	if novas != 1 {
		t.Errorf("novas = %d", novas)
	}
}

func TestAtualizaEMantemOQueEDoUsuario(t *testing.T) {
	x := novaRodada(t, false)
	minha := x.fab.Categoria(func(k *testeapoio.Categoria) { k.Nome = "Minha" })
	antigo := tipos.NovaDataHora(testeapoio.InstanteDosTestes.AddDate(0, -1, 0))
	id := x.lancar(t, func(tr *testeapoio.Transacao) {
		tr.PluggyTransactionId, tr.Descricao, tr.DescricaoOriginal, tr.DescricaoManual = p("t1"), "Meu nome", p("ANTIGA"), true
		tr.CategoriaId, tr.CategoriaManual, tr.Data, tr.AtualizadoEm = &minha.Id, true, tipos.NovaData(2026, 10, 5), antigo
	})
	novas, atualizadas := x.rodar(t, `{"results":[{"id":"t1","date":"2026-10-05T12:00:00Z","description":"NOVA","amount":-10,"type":"DEBIT"}]}`, inicioDoPeriodo, nil)
	if novas != 0 || atualizadas != 1 {
		t.Fatalf("novas %d, atualizadas %d", novas, atualizadas)
	}
	tr := x.transacao(t, id)
	if tr.Descricao != "Meu nome" || *tr.DescricaoOriginal != "NOVA" || *tr.CategoriaId != minha.Id || tr.AtualizadoEm.String() == antigo.String() {
		t.Errorf("transação = %+v", tr)
	}
}

func TestSemMudancaNaoContaNemCarimba(t *testing.T) {
	x := novaRodada(t, false)
	antigo := tipos.NovaDataHora(testeapoio.InstanteDosTestes.AddDate(0, -1, 0))
	id := x.lancar(t, func(tr *testeapoio.Transacao) {
		tr.PluggyTransactionId, tr.Descricao, tr.DescricaoOriginal, tr.Data, tr.AtualizadoEm = p("t1"), "IGUAL", p("IGUAL"), tipos.NovaData(2026, 10, 5), antigo
		tr.Valor = testeapoio.Valor("-10.00") // o número é o mesmo, com outra escala
	})
	_, atualizadas := x.rodar(t, `{"results":[{"id":"t1","date":"2026-10-05T12:00:00Z","description":"IGUAL","amount":-10,"type":"DEBIT"}]}`, inicioDoPeriodo, nil)
	if atualizadas != 0 {
		t.Errorf("atualizadas = %d", atualizadas)
	}
	if x.transacao(t, id).AtualizadoEm.String() != antigo.String() {
		t.Error("AtualizadoEm mudou sem mudança")
	}
}

// Regra (#156): com a categoria igual, o tipo já foi decidido: não volta a ser transferência a cada rodada.
func TestCategoriaIgualNaoRefazOTipo(t *testing.T) {
	x := novaRodada(t, false)
	transferencia := x.fab.Categoria(func(k *testeapoio.Categoria) { k.Nome, k.Tipo = "Aplicações", transacoes.Transferencia })
	if _, err := x.db.Exec(`INSERT INTO "RegraCategorizacao" ("Texto", "TipoCorrespondencia", "CategoriaId", "Prioridade", "Ativa", "CriadoEm") VALUES ('REGRA TESTE', 1, ?, 100, 1, '2026-01-01 00:00:00')`, transferencia.Id); err != nil {
		t.Fatal(err)
	}
	regras, err := categorizacao.Carregar(t.Context(), x.db)
	if err != nil {
		t.Fatal(err)
	}
	x.r.regras = regras
	id := x.lancar(t, func(tr *testeapoio.Transacao) {
		tr.PluggyTransactionId, tr.Descricao, tr.DescricaoOriginal, tr.Data = p("t1"), "REGRA TESTE", p("REGRA TESTE"), tipos.NovaData(2026, 10, 5)
		tr.CategoriaId, tr.Tipo, tr.IgnorarRelatorios = &transferencia.Id, transacoes.Despesa, true
	})
	x.rodar(t, `{"results":[{"id":"t1","date":"2026-10-05T12:00:00Z","description":"REGRA TESTE","amount":-10,"type":"DEBIT"}]}`, inicioDoPeriodo, nil)
	if tr := x.transacao(t, id); tr.Tipo != transacoes.Despesa || !tr.IgnorarRelatorios {
		t.Errorf("tipo %d, ignorar %v", tr.Tipo, tr.IgnorarRelatorios)
	}
}

func TestSinalMudaOTipoSemMudarACategoria(t *testing.T) {
	x := novaRodada(t, false)
	id := x.lancar(t, func(tr *testeapoio.Transacao) {
		tr.PluggyTransactionId, tr.Descricao, tr.DescricaoOriginal, tr.Data, tr.Tipo = p("t1"), "X", p("X"), tipos.NovaData(2026, 10, 5), transacoes.Despesa
	})
	x.rodar(t, `{"results":[{"id":"t1","date":"2026-10-05T12:00:00Z","description":"X","amount":10,"type":"CREDIT"}]}`, inicioDoPeriodo, nil)
	if tr := x.transacao(t, id); tr.Tipo != transacoes.Receita {
		t.Errorf("tipo = %d", tr.Tipo)
	}
}

func TestFaturaPeloMapa(t *testing.T) {
	x := novaRodada(t, true)
	fatura := x.fab.Fatura(testeapoio.Conta{Id: x.conta.id}, tipos.NovaData(2026, 10, 20), testeapoio.Valor("10.0"))
	outra := x.fab.Fatura(testeapoio.Conta{Id: x.conta.id}, tipos.NovaData(2026, 9, 20), testeapoio.Valor("10.0"))
	comFatura := x.lancar(t, func(tr *testeapoio.Transacao) {
		tr.PluggyTransactionId, tr.Data, tr.FaturaCartaoId = p("t1"), tipos.NovaData(2026, 10, 5), &outra.Id
	})
	mantem := x.lancar(t, func(tr *testeapoio.Transacao) {
		tr.PluggyTransactionId, tr.Data, tr.FaturaCartaoId = p("t2"), tipos.NovaData(2026, 10, 5), &outra.Id
	})
	x.rodar(t, `{"results":[{"id":"t1","date":"2026-10-05T12:00:00Z","amount":1,"creditCardMetadata":{"billId":"b1"}},
		{"id":"t2","date":"2026-10-05T12:00:00Z","amount":1,"creditCardMetadata":{"billId":"desconhecida"}}]}`, inicioDoPeriodo, map[string]int64{"b1": fatura.Id})
	if id := x.transacao(t, comFatura).FaturaCartaoId; id == nil || *id != fatura.Id {
		t.Errorf("fatura = %v", id)
	}
	if id := x.transacao(t, mantem).FaturaCartaoId; id == nil || *id != outra.Id {
		t.Errorf("a fatura desconhecida não muda o vínculo: %v", id)
	}
}

func TestPendenteQueSumiuSai(t *testing.T) {
	x := novaRodada(t, false)
	pend := func(id string, dia int) int64 {
		return x.lancar(t, func(tr *testeapoio.Transacao) {
			tr.PluggyTransactionId, tr.Status, tr.Data = &id, pendente, tipos.NovaData(2026, 10, dia)
		})
	}
	sumiu := pend("p1", 5)
	antiga := pend("p2", 1)
	if _, err := x.db.Exec(`UPDATE "Transacao" SET "Data" = '2026-09-20' WHERE "Id" = ?`, antiga); err != nil {
		t.Fatal(err)
	}
	efetivadaQueSumiu := x.lancar(t, func(tr *testeapoio.Transacao) {
		tr.PluggyTransactionId, tr.Data = p("e1"), tipos.NovaData(2026, 10, 5)
	})
	manual := x.lancar(t, func(tr *testeapoio.Transacao) { tr.Status, tr.Data = pendente, tipos.NovaData(2026, 10, 5) })
	x.rodar(t, `{"results":[]}`, inicioDoPeriodo, nil)
	if x.transacao(t, sumiu) != nil {
		t.Error("a pendente que sumiu ficou")
	}
	if x.transacao(t, antiga) == nil {
		t.Error("a pendente de fora da janela sem par saiu")
	}
	if x.transacao(t, efetivadaQueSumiu) == nil {
		t.Error("a efetivada saiu")
	}
	if x.transacao(t, manual) == nil {
		t.Error("a lançada à mão saiu")
	}
}

// Regra (#172): pendente de antes da janela sai quando há efetivada igual entre 2 dias antes e 10 depois.
func TestPendenteAntigaJaEfetivada(t *testing.T) {
	casos := []struct {
		nome   string
		diaPar int
		valor  string
		sai    bool
	}{
		{"10 dias depois", 10, "-10.0", true},
		{"11 dias depois", 11, "-10.0", false},
		{"2 dias antes", -2, "-10.0", true},
		{"3 dias antes", -3, "-10.0", false},
		{"no mesmo dia", 0, "-10.00", true},
		{"outro valor", 0, "-11.0", false},
	}
	base := tipos.NovaData(2026, 9, 15)
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			x := novaRodada(t, false)
			id := x.lancar(t, func(tr *testeapoio.Transacao) {
				tr.PluggyTransactionId, tr.Status, tr.Data, tr.Valor = p("p1"), pendente, base, testeapoio.Valor("-10.0")
			})
			x.lancar(t, func(tr *testeapoio.Transacao) {
				tr.PluggyTransactionId, tr.Data, tr.Valor = p("e1"), base.SomarDias(caso.diaPar), testeapoio.Valor(caso.valor)
			})
			x.rodar(t, `{"results":[]}`, inicioDoPeriodo, nil)
			if saiu := x.transacao(t, id) == nil; saiu != caso.sai {
				t.Errorf("saiu = %v, esperado %v", saiu, caso.sai)
			}
		})
	}
	t.Run("uma efetivada só paga uma pendente, a mais próxima", func(t *testing.T) {
		x := novaRodada(t, false)
		pendenteEm := func(id string, d tipos.Data) int64 {
			return x.lancar(t, func(tr *testeapoio.Transacao) {
				tr.PluggyTransactionId, tr.Status, tr.Data, tr.Valor = &id, pendente, d, testeapoio.Valor("-10.0")
			})
		}
		primeira := pendenteEm("p1", base)
		segunda := pendenteEm("p2", base.SomarDias(1))
		x.lancar(t, func(tr *testeapoio.Transacao) {
			tr.PluggyTransactionId, tr.Data, tr.Valor = p("e1"), base.SomarDias(2), testeapoio.Valor("-10.0")
		})
		x.lancar(t, func(tr *testeapoio.Transacao) {
			tr.PluggyTransactionId, tr.Data, tr.Valor = p("e2"), base.SomarDias(1), testeapoio.Valor("-10.0")
		})
		x.rodar(t, `{"results":[]}`, inicioDoPeriodo, nil)
		if x.transacao(t, primeira) != nil || x.transacao(t, segunda) != nil {
			t.Error("cada pendente tem o próprio par: as duas saem")
		}
	})
}

func TestTransacaoDeOutraContaFica(t *testing.T) {
	x := novaRodada(t, false)
	outra := x.fab.Conta()
	id := x.fab.Transacao(outra, func(tr *testeapoio.Transacao) {
		tr.PluggyTransactionId, tr.Descricao, tr.Data = p("t1"), "OUTRA", tipos.NovaData(2026, 10, 5)
	}).Id
	novas, atualizadas := x.rodar(t, `{"results":[{"id":"t1","date":"2026-10-05T12:00:00Z","description":"MUDOU","amount":1}]}`, inicioDoPeriodo, nil)
	if novas != 0 || atualizadas != 0 || x.transacao(t, id).Descricao != "OUTRA" {
		t.Errorf("novas %d, atualizadas %d", novas, atualizadas)
	}
}

func TestDataMudouParaForaDaJanela(t *testing.T) {
	x := novaRodada(t, false)
	id := x.lancar(t, func(tr *testeapoio.Transacao) {
		tr.PluggyTransactionId, tr.Descricao, tr.DescricaoOriginal, tr.Data = p("t1"), "X", p("X"), tipos.NovaData(2026, 1, 1)
	})
	novas, atualizadas := x.rodar(t, `{"results":[{"id":"t1","date":"2026-10-05T12:00:00Z","description":"X","amount":-10,"type":"DEBIT"}]}`, inicioDoPeriodo, nil)
	if novas != 0 || atualizadas != 1 {
		t.Errorf("novas %d, atualizadas %d", novas, atualizadas)
	}
	if d := x.transacao(t, id).Data; d.String() != "2026-10-05" {
		t.Errorf("data = %s", d)
	}
}

func TestImportarCarteira(t *testing.T) {
	investimento := func(c cenario, conexao testeapoio.ConexaoBancaria, ajuste func(*testeapoio.Investimento)) {
		c.fab.Investimento(conexao, func(i *testeapoio.Investimento) {
			i.Tipo, i.Saldo = "EQUITY", testeapoio.Valor("1000.0")
			ajuste(i)
		})
	}
	semHistorico := func(t *testing.T, c cenario) testeapoio.ConexaoBancaria {
		t.Helper()
		c.credenciais(t)
		c.falsa.Em("GET", "*", testeapoio.RespostaDeFora{Corpo: `{"totalPages":1,"results":[]}`})
		return c.fab.ConexaoBancaria()
	}
	importar := func(t *testing.T, c cenario) ResultadoCarteira {
		t.Helper()
		r, err := c.s.ImportarCarteira(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		return r
	}

	t.Run("sem credenciais", func(t *testing.T) {
		c := novoCenario(t)
		if r := importar(t, c); *r.Mensagem != semCredenciais {
			t.Errorf("mensagem = %q", *r.Mensagem)
		}
	})
	t.Run("a Pluggy recusa: o erro volta", func(t *testing.T) {
		c := novoCenario(t)
		c.credenciais(t)
		c.falsa.Em("POST", "/auth", testeapoio.NaoAutorizado)
		var e *pluggy.Erro
		if _, err := c.s.ImportarCarteira(t.Context()); !errors.As(err, &e) {
			t.Fatalf("erro = %v", err)
		}
	})
	t.Run("posição sem histórico vira uma compra", func(t *testing.T) {
		c := novoCenario(t)
		conexao := semHistorico(t, c)
		investimento(c, conexao, func(i *testeapoio.Investimento) {
			i.Codigo, i.Quantidade, i.ValorAplicado = p("FALS3"), p(testeapoio.Valor("4.0")), p(testeapoio.Valor("800.0"))
		})
		r := importar(t, c)
		if r.AtivosCriados != 1 || r.OperacoesImportadas != 1 || len(r.SemHistorico) != 1 {
			t.Fatalf("resultado = %+v", r)
		}
		if v := c.valor(t, `SELECT "PrecoUnitario" FROM "OperacaoAtivo"`); v != "200.0" {
			t.Errorf("preço = %q: o custo informado dividido pelas cotas", v)
		}
		if v := c.valor(t, `SELECT "PrecoAtual" FROM "Ativo"`); v != "250.0" {
			t.Errorf("preço atual = %q: saldo dividido pelas cotas", v)
		}
		if v := c.valor(t, `SELECT "FonteCotacao" FROM "Ativo"`); v != fonteOpenFinance {
			t.Errorf("fonte = %q", v)
		}
		// De novo: nada duplica, e o resultado continua dizendo que não há histórico.
		r = importar(t, c)
		if r.AtivosCriados != 0 || r.OperacoesImportadas != 0 || len(r.SemHistorico) != 1 {
			t.Errorf("segunda = %+v", r)
		}
		if n := c.valor(t, `SELECT COUNT(*) FROM "OperacaoAtivo"`); n != "1" {
			t.Errorf("%s operações", n)
		}
	})
	t.Run("ticker que não é da B3 fica de fora", func(t *testing.T) {
		c := novoCenario(t)
		conexao := semHistorico(t, c)
		investimento(c, conexao, func(i *testeapoio.Investimento) { i.Codigo, i.Nome = p("???"), "Sem ticker" })
		if r := importar(t, c); r.AtivosCriados != 0 {
			t.Errorf("resultado = %+v", r)
		}
	})
	t.Run("o ticker pode vir do nome", func(t *testing.T) {
		c := novoCenario(t)
		conexao := semHistorico(t, c)
		investimento(c, conexao, func(i *testeapoio.Investimento) {
			i.Codigo, i.Nome, i.Quantidade = nil, "FALS4", p(testeapoio.Valor("1.0"))
		})
		if r := importar(t, c); r.AtivosCriados != 1 {
			t.Errorf("resultado = %+v", r)
		}
	})
	t.Run("ativo só com lançamentos seus não importa", func(t *testing.T) {
		c := novoCenario(t)
		conexao := semHistorico(t, c)
		ativo := c.fab.Ativo(func(a *testeapoio.Ativo) { a.Ticker = "FALS3" })
		c.fab.OperacaoAtivo(ativo)
		investimento(c, conexao, func(i *testeapoio.Investimento) { i.Codigo = p("FALS3") })
		r := importar(t, c)
		if len(r.ComLancamentoManual) != 1 || r.OperacoesImportadas != 0 {
			t.Errorf("resultado = %+v", r)
		}
	})
	t.Run("sem quantidade: cotas pelo preço atual", func(t *testing.T) {
		c := novoCenario(t)
		conexao := semHistorico(t, c)
		c.fab.Ativo(func(a *testeapoio.Ativo) { a.Ticker, a.PrecoAtual = "FALS3", p(testeapoio.Valor("300.0")) })
		investimento(c, conexao, func(i *testeapoio.Investimento) { i.Codigo = p("FALS3") })
		if r := importar(t, c); r.OperacoesImportadas != 1 {
			t.Fatalf("resultado = %+v", r)
		}
		if v := c.valor(t, `SELECT "Quantidade" FROM "OperacaoAtivo"`); v != "3.0" {
			t.Errorf("quantidade = %q: 1000 ÷ 300, sem casas", v)
		}
	})
	t.Run("o histórico chega e a compra provisória sai", func(t *testing.T) {
		c := novoCenario(t)
		conexao := semHistorico(t, c)
		investimento(c, conexao, func(i *testeapoio.Investimento) { i.Codigo, i.Quantidade = p("FALS3"), p(testeapoio.Valor("4.0")) })
		importar(t, c)
		c.falsa.Em("GET", "*", testeapoio.RespostaDeFora{Corpo: `{"totalPages":1,"results":[{"id":"m1","type":"BUY","quantity":4,"value":200,"date":"2026-01-10T12:00:00Z"}]}`})
		r := importar(t, c)
		if r.OperacoesImportadas != 1 {
			t.Fatalf("resultado = %+v", r)
		}
		if id := c.valor(t, `SELECT "PluggyTransacaoId" FROM "OperacaoAtivo"`); id != "m1" {
			t.Errorf("operação = %q: a provisória devia ter saído", id)
		}
	})
	t.Run("a corretora confirma um lançamento seu", func(t *testing.T) {
		c := novoCenario(t)
		conexao := semHistorico(t, c)
		ativo := c.fab.Ativo(func(a *testeapoio.Ativo) { a.Ticker = "FALS3" })
		c.fab.OperacaoAtivo(ativo, func(o *testeapoio.OperacaoAtivo) {
			o.Origem, o.PluggyTransacaoId = origemOpenFinance, p("m0")
		})
		sua := c.fab.OperacaoAtivo(ativo, func(o *testeapoio.OperacaoAtivo) {
			o.Quantidade, o.Data = testeapoio.Valor("2.0"), tipos.NovaData(2026, 1, 10)
		})
		investimento(c, conexao, func(i *testeapoio.Investimento) { i.Codigo = p("FALS3") })
		c.falsa.Em("GET", "*", testeapoio.RespostaDeFora{Corpo: `{"totalPages":1,"results":[{"id":"m1","type":"BUY","quantity":2,"value":10,"date":"2026-01-12T12:00:00Z"}]}`})
		r := importar(t, c)
		if r.OperacoesImportadas != 0 {
			t.Fatalf("resultado = %+v", r)
		}
		if id := c.valor(t, `SELECT "PluggyTransacaoId" FROM "OperacaoAtivo" WHERE "Id" = ?`, sua.Id); id != "m1" {
			t.Errorf("vínculo = %q", id)
		}
	})
	t.Run("a Pluggy sem as movimentações não vira compra a mais", func(t *testing.T) {
		c := novoCenario(t)
		conexao := semHistorico(t, c)
		ativo := c.fab.Ativo(func(a *testeapoio.Ativo) { a.Ticker = "FALS3" })
		c.fab.OperacaoAtivo(ativo, func(o *testeapoio.OperacaoAtivo) { o.Origem, o.PluggyTransacaoId = origemOpenFinance, p("m0") })
		investimento(c, conexao, func(i *testeapoio.Investimento) { i.Codigo, i.Quantidade = p("FALS3"), p(testeapoio.Valor("4.0")) })
		c.falsa.Em("GET", "*", testeapoio.RespostaDeFora{Status: 400})
		if r := importar(t, c); r.OperacoesImportadas != 0 || len(r.SemHistorico) != 0 {
			t.Errorf("resultado = %+v", r)
		}
	})
	t.Run("a falha de um investimento não impede o outro", func(t *testing.T) {
		c := novoCenario(t)
		conexao := semHistorico(t, c)
		investimento(c, conexao, func(i *testeapoio.Investimento) {
			i.Codigo, i.PluggyInvestmentId, i.Quantidade = p("FALS3"), "falha", p(testeapoio.Valor("1.0"))
		})
		investimento(c, conexao, func(i *testeapoio.Investimento) {
			i.Codigo, i.PluggyInvestmentId, i.Quantidade = p("FALS4"), "bom", p(testeapoio.Valor("1.0"))
		})
		c.falsa.Em("GET", "/investments/falha/transactions", testeapoio.JSONQuebrado)
		r := importar(t, c)
		if len(r.Erros) != 1 || r.Erros[0] != "FALS3" || r.OperacoesImportadas != 1 {
			t.Errorf("resultado = %+v", r)
		}
	})
}
