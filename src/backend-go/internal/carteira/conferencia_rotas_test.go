package carteira

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
)

const (
	rotaConferencia = "/api/carteira/conferencia"
	rotaCorrigir    = rotaConferencia + "/corrigir"
	rotaTrocos      = rotaConferencia + "/trocos-automaticos"

	compraDoCenario = `{"transacaoId":131,"ticker":"FALT11","tipo":1,"quantidade":1,"precoUnitario":1,"taxas":0}`
	correcaoInteira = `{"ativosParaFii":[3],"compras":[` + compraDoCenario + `],"trocos":[102],"rendimentos":[151],"taxas":[1]}`
)

// cenaDaConferencia é a cena com o banco de testdata/conferencia-cenario.sql, o mesmo da sonda.
func cenaDaConferencia(t *testing.T, a cotacoesFalsas) cena {
	t.Helper()
	c := cenaComCotacoes(t, cambioFalso{}, reservasFalsas{}, selicFalsa{}, a)
	cenario, err := os.ReadFile("testdata/conferencia-cenario.sql")
	if err != nil {
		t.Fatal(err)
	}
	executar(t, c, string(cenario))
	return c
}

type usuarioDeTeste struct {
	Id    int64
	Login string
}

func usuarioDaConferencia(t *testing.T, c cena) usuarioDeTeste {
	t.Helper()
	executar(t, c, `INSERT INTO Usuario (Id, Login, SenhaHash, CarimboSeguranca, CriadoEm) VALUES (7, 'teste', 'x', 'y', '2026-01-01 00:00:00')`)
	return usuarioDeTeste{7, "teste"}
}

func chaveDaCorrecao(t *testing.T, c cena) string {
	t.Helper()
	r := testeapoio.Chamar(t, c.mux, "POST", rotaCorrigir, correcaoInteira)
	r.ConferirStatus(t, http.StatusOK)
	var d struct{ Desfazer string }
	if err := json.Unmarshal(r.Corpo, &d); err != nil || d.Desfazer == "" {
		t.Fatalf("resposta %s, %v", r.Corpo, err)
	}
	return d.Desfazer
}

// Regra: as rotas da conferência: corrigir devolve o que mudou e a chave, desfazer usa a chave uma vez só, e os trocos
// automáticos voltam ou são dispensados. Quem corrigiu fica na auditoria.
func TestRotasDaConferencia(t *testing.T) {
	t.Parallel()
	c := cenaDaConferencia(t, cotacoesFalsas{})
	usuario := usuarioDaConferencia(t, c)
	r := testeapoio.Chamar(t, c.mux, "GET", rotaConferencia, "")
	r.ConferirStatus(t, http.StatusOK)
	if !strings.Contains(string(r.Corpo), `"ticker":"CCCC3"`) {
		t.Errorf("conferência %s", r.Corpo)
	}

	r = testeapoio.ChamarComo(t, c.mux, usuario.Id, "POST", rotaCorrigir, `{"ativosParaFii":[3],"compras":[`+compraDoCenario+`],"trocos":[102],"rendimentos":[151],"taxas":[1]}`)
	r.ConferirStatus(t, http.StatusOK)
	var d ResultadoDaCorrecao
	if err := json.Unmarshal(r.Corpo, &d); err != nil || d.Desfazer == nil {
		t.Fatalf("resposta %s, %v", r.Corpo, err)
	}
	if d.Tipos != 1 || d.Compras != 1 || d.Trocos != 1 || d.Rendimentos != 1 || d.Taxas != 1 {
		t.Errorf("resultado %+v", d)
	}
	evento := "ConferenciaInvestimentos (1 tipo, 1 compra, 1 troco, 1 rendimento, 1 taxa) | " + usuario.Login
	if a := umaLinha(t, c, `SELECT Evento, Login FROM AuditoriaAcesso ORDER BY Id DESC LIMIT 1`); a != evento {
		t.Errorf("auditoria %q", a)
	}

	desfazer := rotaConferencia + "/desfazer/" + *d.Desfazer
	r = testeapoio.ChamarComo(t, c.mux, usuario.Id, "POST", desfazer, "")
	r.ConferirStatus(t, http.StatusOK)
	r.ConferirJSON(t, `{"transacoesAtualizadas":5}`)
	if a := umaLinha(t, c, `SELECT Evento, Login FROM AuditoriaAcesso ORDER BY Id DESC LIMIT 1`); a != "ConferenciaInvestimentosDesfeita (5) | "+usuario.Login {
		t.Errorf("auditoria %q", a)
	}
	r = testeapoio.Chamar(t, c.mux, "POST", desfazer, "")
	r.ConferirStatus(t, http.StatusNotFound)
	if !strings.Contains(string(r.Corpo), "o prazo passou ou já foi desfeito") {
		t.Errorf("resposta %s", r.Corpo)
	}

	if n, err := c.s.ClassificarTrocosSemDuvida(t.Context()); err != nil || n == 0 {
		t.Fatalf("classificar: %d, %v", n, err)
	}
	r = testeapoio.ChamarComo(t, c.mux, usuario.Id, "POST", rotaTrocos+"/desfazer", "")
	r.ConferirStatus(t, http.StatusOK)
	if !strings.HasPrefix(string(r.Corpo), `{"transacoesAtualizadas":`) || strings.Contains(string(r.Corpo), `:0}`) {
		t.Errorf("resposta %s", r.Corpo)
	}
	if a := umaLinha(t, c, `SELECT Login FROM AuditoriaAcesso ORDER BY Id DESC LIMIT 1`); a != usuario.Login {
		t.Errorf("auditoria %q", a)
	}
	if n, err := c.s.ClassificarTrocosSemDuvida(t.Context()); err != nil || n != 0 {
		t.Fatalf("o que foi desfeito fica como escolha manual: %d, %v", n, err)
	}
	testeapoio.Chamar(t, c.mux, "POST", rotaTrocos+"/dispensar", "").ConferirStatus(t, http.StatusNoContent)
}

// Regra: corrigir recusa a entrada errada: a mensagem da regra é 400, os limites valem por lista, e a compra nula é o 500
// do C#.
func TestCorrigirComEntradaErrada(t *testing.T) {
	t.Parallel()
	numeros := func(n int) string { return strings.TrimSuffix(strings.Repeat("1,", n), ",") }
	casos := []struct {
		nome, corpo string
		status      int
		trecho      string
	}{
		{"regra", `{"ativosParaFii":[999]}`, http.StatusBadRequest, "Algum ativo não existe mais"},
		{"200 ativos", `{"ativosParaFii":[` + numeros(200) + `]}`, http.StatusOK, `"tipos":1`},
		{"201 ativos", `{"ativosParaFii":[` + numeros(201) + `]}`, http.StatusBadRequest, "AtivosParaFii"},
		{"201 compras", `{"compras":[` + strings.TrimSuffix(strings.Repeat(compraDoCenario+",", 201), ",") + `]}`, http.StatusBadRequest, "Compras"},
		{"2001 trocos", `{"trocos":[` + numeros(2001) + `]}`, http.StatusBadRequest, "Trocos"},
		{"2001 rendimentos", `{"rendimentos":[` + numeros(2001) + `]}`, http.StatusBadRequest, "Rendimentos"},
		{"2001 taxas", `{"taxas":[` + numeros(2001) + `]}`, http.StatusBadRequest, "Taxas"},
		{"compra inválida", `{"compras":[{"transacaoId":131,"ticker":"AB","tipo":1,"quantidade":1,"precoUnitario":1,"taxas":0}]}`, http.StatusBadRequest, "Compras[0].Ticker"},
		{"compra nula", `{"compras":[null]}`, http.StatusInternalServerError, ""},
		{"um FII antes de uma ação", `{"ativosParaFii":[6,10]}`, http.StatusOK, `"tipos":1`},
		{"sem as listas", `{}`, http.StatusOK, `"desfazer":null`},
		{"tudo nulo", `{"ativosParaFii":null,"compras":null,"trocos":null,"rendimentos":null,"taxas":null}`, http.StatusOK, `"desfazer":null`},
	}
	for _, x := range casos {
		t.Run(x.nome, func(t *testing.T) {
			t.Parallel()
			c := cenaDaConferencia(t, cotacoesFalsas{})
			r := testeapoio.Chamar(t, c.mux, "POST", rotaCorrigir, x.corpo)
			r.ConferirStatus(t, x.status)
			if !strings.Contains(string(r.Corpo), x.trecho) {
				t.Errorf("resposta %s", r.Corpo)
			}
		})
	}
}

// Regra: a lista de trocos guardados fica nos últimos 500, e o troco novo entra no fim.
func TestTrocosGuardadosNoMaximo(t *testing.T) {
	t.Parallel()
	c := cenaDaConferencia(t, cotacoesFalsas{})
	lista := make([]trocoAutomatico, 500)
	for i := range lista {
		lista[i] = trocoAutomatico{Id: int64(10000 + i), Tipo: 2, Aplicada: 3}
	}
	dados, err := json.Marshal(lista)
	if err != nil {
		t.Fatal(err)
	}
	executar(t, c, fmt.Sprintf(`INSERT INTO Configuracao (Chave, Valor, AtualizadoEm) VALUES ('%s', '%s', '2026-01-01 12:00:00')`, ChaveTrocosAutomaticos, dados))
	n, err := c.s.ClassificarTrocosSemDuvida(t.Context())
	if err != nil || n == 0 {
		t.Fatalf("%d, %v", n, err)
	}
	guardados, err := c.s.trocosGuardados(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(guardados) != 500 || guardados[0].Id != int64(10000+n) || guardados[499].Id < 100 || guardados[499].Id >= 10000 {
		t.Errorf("%d guardados, de %d a %d", len(guardados), guardados[0].Id, guardados[499].Id)
	}
}

// Regra: sem a categoria do troco, os trocos continuam na conferência.
func TestClassificarSemCategoriaDoTroco(t *testing.T) {
	t.Parallel()
	c := cenaDaConferencia(t, cotacoesFalsas{})
	executar(t, c, `UPDATE Categoria SET Nome = 'Caixinha' WHERE Id = 3`)
	if n, err := c.s.ClassificarTrocosSemDuvida(t.Context()); err != nil || n != 0 {
		t.Errorf("%d, %v", n, err)
	}
}

// Regra: a conta do dia espera os 5 dias da sobra do troco: o dia 25 entra, com hoje em 30, e o dia 26 ainda não.
func TestContaDoDiaEsperaOsDiasDaSobra(t *testing.T) {
	t.Parallel()
	c := cenaDaConferencia(t, cotacoesFalsas{})
	executar(t, c, `INSERT INTO OperacaoAtivo (Id, AtivoId, Tipo, Data, Quantidade, PrecoUnitario, Taxas, Cambio, Origem, CriadoEm) VALUES
		(901, 1, 1, '2026-10-25', '10.0', '10.0', '0.0', '1.0', 1, '2026-10-25 12:00:00'), (902, 2, 1, '2026-10-26', '10.0', '10.0', '0.0', '1.0', 1, '2026-10-26 12:00:00');
		INSERT INTO Transacao (Id, ContaId, Data, Descricao, Valor, Tipo, Status, CategoriaManual, IgnorarRelatorios, Emergencia, DescricaoRevisada, DescricaoManual, CriadoEm, AtualizadoEm) VALUES
		(901, 1, '2026-10-25', 'Compra de Renda Variável', '-100.5', 1, 1, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
		(902, 1, '2026-10-26', 'Compra de Renda Variável', '-100.5', 1, 1, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00')`)
	d, err := c.s.Conferir(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	dados, err := json.Marshal(d.Taxas)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(dados), `"data":"2026-10-25"`) || strings.Contains(string(dados), `"data":"2026-10-26"`) {
		t.Errorf("taxas %s", dados)
	}
}

// quebrar aplica a quebra do caso: "INSERT em Ativo" faz o banco recusar; o resto é SQL.
func quebrar(t *testing.T, c cena, quebra string) {
	t.Helper()
	if quando, tabela, ok := strings.Cut(quebra, " em "); ok {
		recusar(t, c, quando, tabela)
		return
	}
	executar(t, c, quebra)
}

// Regra: o banco falhar em qualquer passo da conferência é o 500, e não um resultado pela metade.
func TestConferenciaComErroDoBanco(t *testing.T) {
	t.Parallel()
	const (
		semTransacao = `ALTER TABLE Transacao RENAME TO Outra`
		semCategoria = `ALTER TABLE Categoria RENAME TO Outra`
		semRegra     = `ALTER TABLE RegraCategorizacao RENAME TO Outra`
		semAtivo     = `ALTER TABLE Ativo RENAME TO Outro`
		semOperacao  = `ALTER TABLE OperacaoAtivo RENAME TO Outra`
		semAuditoria = `DROP TABLE AuditoriaAcesso`
	)
	type caso struct{ nome, quebra string }
	leituras := []caso{
		{"o extrato", semTransacao}, {"o valor no extrato", `UPDATE Transacao SET Valor = 'abc' WHERE Id = 101`}, {"as categorias", semCategoria},
		{"as regras", semRegra}, {"os ativos", semAtivo}, {"as operações", semOperacao},
		{"o valor da operação", `UPDATE OperacaoAtivo SET Taxas = 'abc' WHERE Id = 1`}, {"os investimentos", `ALTER TABLE Investimento RENAME TO Outro`},
		{"o saldo do investimento", `UPDATE Investimento SET Saldo = 'abc'`}, {"o fechamento mensal", `ALTER TABLE PrecoMensalAtivo RENAME TO Outra`},
		{"o valor do fechamento", `UPDATE PrecoMensalAtivo SET Fechamento = 'abc'`},
	}
	for _, x := range leituras {
		t.Run("conferir: "+x.nome, func(t *testing.T) {
			t.Parallel()
			c := cenaDaConferencia(t, cotacoesFalsas{})
			quebrar(t, c, x.quebra)
			testeapoio.Chamar(t, c.mux, "GET", rotaConferencia, "").ConferirStatus(t, http.StatusInternalServerError)
		})
	}
	t.Run("conferir: o histórico de cotações", func(t *testing.T) {
		t.Parallel()
		c := cenaDaConferencia(t, cotacoesFalsas{erroFechamentos: errors.New("falhou")})
		testeapoio.Chamar(t, c.mux, "GET", rotaConferencia, "").ConferirStatus(t, http.StatusInternalServerError)
	})

	correcoes := []struct{ nome, quebra, corpo string }{
		{"os ativos", semAtivo, `{"ativosParaFii":[3]}`},
		{"gravar o tipo", "UPDATE em Ativo", `{"ativosParaFii":[3]}`},
		{"a análise das taxas", semTransacao, `{"taxas":[1]}`},
		{"os ativos das taxas", semAtivo, `{"taxas":[1]}`},
		{"gravar a taxa", "UPDATE em OperacaoAtivo", `{"taxas":[1]}`},
		{"os débitos", semTransacao, `{"compras":[` + compraDoCenario + `]}`},
		{"o valor do débito", `UPDATE Transacao SET Valor = 'abc' WHERE Id = 131`, `{"compras":[` + compraDoCenario + `]}`},
		{"as operações ligadas", semOperacao, `{"compras":[` + compraDoCenario + `]}`},
		{"o ativo da compra", semAtivo, `{"compras":[` + compraDoCenario + `]}`},
		{"gravar a compra", "INSERT em OperacaoAtivo", `{"compras":[` + compraDoCenario + `]}`},
		{"os créditos", semTransacao, `{"trocos":[102]}`},
		{"as regras", semRegra, `{"trocos":[102]}`},
		{"as categorias", semCategoria, `{"trocos":[102]}`},
		{"gravar o crédito", "UPDATE em Transacao", `{"trocos":[102]}`},
		{"a auditoria", semAuditoria, `{"trocos":[102]}`},
	}
	for _, x := range correcoes {
		t.Run("corrigir: "+x.nome, func(t *testing.T) {
			t.Parallel()
			c := cenaDaConferencia(t, cotacoesFalsas{})
			quebrar(t, c, x.quebra)
			testeapoio.Chamar(t, c.mux, "POST", rotaCorrigir, x.corpo).ConferirStatus(t, http.StatusInternalServerError)
			if x.quebra == semOperacao {
				return
			}
			if a := umaLinha(t, c, `SELECT COUNT(*) FROM OperacaoAtivo WHERE TransacaoId = 131`); a != "0" {
				t.Errorf("gravou pela metade: %s", a)
			}
		})
	}

	desfazeres := []caso{
		{"o tipo", "UPDATE em Ativo"}, {"as compras", "DELETE em OperacaoAtivo"}, {"a taxa", "UPDATE em OperacaoAtivo"},
		{"o valor da taxa", `UPDATE OperacaoAtivo SET Taxas = 'abc' WHERE Id = 1`}, {"as transações", semTransacao}, {"as categorias", semCategoria}, {"a auditoria", semAuditoria},
	}
	for _, x := range desfazeres {
		t.Run("desfazer: "+x.nome, func(t *testing.T) {
			t.Parallel()
			c := cenaDaConferencia(t, cotacoesFalsas{})
			chave := chaveDaCorrecao(t, c)
			quebrar(t, c, x.quebra)
			testeapoio.Chamar(t, c.mux, "POST", rotaConferencia+"/desfazer/"+chave, "").ConferirStatus(t, http.StatusInternalServerError)
		})
	}

	classificacoes := []caso{
		{"os ativos", semAtivo}, {"o extrato", semTransacao}, {"as categorias", semCategoria}, {"as regras", semRegra},
		{"os guardados", `ALTER TABLE Configuracao RENAME TO Outra`}, {"gravar as transações", "UPDATE em Transacao"}, {"gravar os guardados", "INSERT em Configuracao"},
		{"a auditoria", semAuditoria},
	}
	for _, x := range classificacoes {
		t.Run("classificar: "+x.nome, func(t *testing.T) {
			t.Parallel()
			c := cenaDaConferencia(t, cotacoesFalsas{})
			quebrar(t, c, x.quebra)
			if _, err := c.s.ClassificarTrocosSemDuvida(t.Context()); err == nil {
				t.Error("sem erro")
			}
		})
	}

	guardados := []struct {
		nome, quebra string
		chamar       func(t *testing.T, c cena)
	}{
		{"resumo", semTransacao, func(t *testing.T, c cena) {
			if _, err := c.s.ResumoTrocosAutomaticos(t.Context()); err == nil {
				t.Error("sem erro")
			}
		}},
		{"resumo sem a configuração", `ALTER TABLE Configuracao RENAME TO Outra`, func(t *testing.T, c cena) {
			if _, err := c.s.ResumoTrocosAutomaticos(t.Context()); err == nil {
				t.Error("sem erro")
			}
		}},
		{"desfazer sem a configuração", `ALTER TABLE Configuracao RENAME TO Outra`, func(t *testing.T, c cena) {
			testeapoio.Chamar(t, c.mux, "POST", rotaTrocos+"/desfazer", "").ConferirStatus(t, http.StatusInternalServerError)
		}},
		{"desfazer sem as transações", semTransacao, func(t *testing.T, c cena) {
			testeapoio.Chamar(t, c.mux, "POST", rotaTrocos+"/desfazer", "").ConferirStatus(t, http.StatusInternalServerError)
		}},
		{"desfazer sem gravar", "UPDATE em Configuracao", func(t *testing.T, c cena) {
			testeapoio.Chamar(t, c.mux, "POST", rotaTrocos+"/desfazer", "").ConferirStatus(t, http.StatusInternalServerError)
		}},
		{"desfazer sem a auditoria", semAuditoria, func(t *testing.T, c cena) {
			testeapoio.Chamar(t, c.mux, "POST", rotaTrocos+"/desfazer", "").ConferirStatus(t, http.StatusInternalServerError)
		}},
		{"dispensar sem gravar", "UPDATE em Configuracao", func(t *testing.T, c cena) {
			testeapoio.Chamar(t, c.mux, "POST", rotaTrocos+"/dispensar", "").ConferirStatus(t, http.StatusInternalServerError)
		}},
	}
	for _, x := range guardados {
		t.Run(x.nome, func(t *testing.T) {
			t.Parallel()
			c := cenaDaConferencia(t, cotacoesFalsas{})
			if n, err := c.s.ClassificarTrocosSemDuvida(t.Context()); err != nil || n == 0 {
				t.Fatalf("classificar: %d, %v", n, err)
			}
			quebrar(t, c, x.quebra)
			x.chamar(t, c)
		})
	}

	t.Run("o login de quem corrigiu", func(t *testing.T) {
		t.Parallel()
		c := cenaDaConferencia(t, cotacoesFalsas{})
		usuario := usuarioDaConferencia(t, c)
		executar(t, c, `ALTER TABLE Usuario RENAME TO Outro`)
		for _, rota := range []string{rotaCorrigir, rotaConferencia + "/desfazer/x", rotaTrocos + "/desfazer"} {
			testeapoio.ChamarComo(t, c.mux, usuario.Id, "POST", rota, `{}`).ConferirStatus(t, http.StatusInternalServerError)
		}
	})
}
