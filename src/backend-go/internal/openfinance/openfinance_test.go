package openfinance

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/configuracao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transferencias"
)

func p[T any](v T) *T { return &v }

func conferirStatus(t *testing.T, r testeapoio.Resposta, esperado int) {
	t.Helper()
	r.ConferirStatus(t, esperado)
}

// parametrosFixos é o internal/parametros num número só: os dias do aviso de consentimento.
type parametrosFixos int

func (d parametrosFixos) ObterInteiro(context.Context, string) (int, error) { return int(d), nil }

// cofreFalso cifra sem chave: o que importa aqui é que o texto puro não vá para o banco.
type cofreFalso struct{}

func (cofreFalso) Cifrar(_, segredo string) (string, error)   { return "v2:" + segredo + ":cifrado", nil }
func (cofreFalso) Decifrar(_, gravado string) (string, error) { return gravado, nil }

type cenario struct {
	mux *http.ServeMux
	s   *Servico
	db  *sql.DB
	fab *testeapoio.Fabrica
}

// novoCenario: 15/10/2026, meio-dia em Brasília (15h em UTC), com aviso de consentimento a 30 dias.
func novoCenario(t *testing.T) cenario {
	t.Helper()
	db := testeapoio.NovoBancoVazio(t)
	r := testeapoio.HojeEmBrasilia(2026, 10, 15)
	mux := http.NewServeMux()
	s := NovoServico(db, cofreFalso{}, parametrosFixos(30), r, slog.New(slog.DiscardHandler))
	Registrar(mux, s)
	return cenario{mux, s, db, testeapoio.NovaFabrica(t, db, r)}
}

func (c cenario) chamar(t *testing.T, metodo, caminho, corpo string) testeapoio.Resposta {
	t.Helper()
	return testeapoio.Chamar(t, c.mux, metodo, caminho, corpo)
}

func (c cenario) contar(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := c.db.QueryRow(sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (c cenario) valor(t *testing.T, chave string) *string {
	t.Helper()
	var v *string
	if err := c.db.QueryRow(`SELECT "Valor" FROM "Configuracao" WHERE "Chave" = ?`, chave).Scan(&v); err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		t.Fatal(err)
	}
	return v
}

// definir grava a chave por cima dos dados iniciais do banco de teste.
func (c cenario) definir(t *testing.T, chave, valor string) {
	t.Helper()
	if _, err := c.db.Exec(`INSERT INTO "Configuracao" ("Chave", "Valor", "Protegido", "AtualizadoEm") VALUES (?, ?, 0, '2026-01-01 00:00:00')
		ON CONFLICT ("Chave") DO UPDATE SET "Valor" = excluded."Valor"`, chave, valor); err != nil {
		t.Fatal(err)
	}
}

func (c cenario) auditoria(t *testing.T, evento string) int {
	t.Helper()
	return c.contar(t, `SELECT COUNT(*) FROM "AuditoriaAcesso" WHERE "Evento" = ?`, evento)
}

func TestObterConfiguracao(t *testing.T) {
	t.Run("sem nada gravado vale o padrão", func(t *testing.T) {
		c := novoCenario(t)
		if _, err := c.db.Exec(`DELETE FROM "Configuracao" WHERE "Chave" LIKE 'Sincronizacao.%'`); err != nil {
			t.Fatal(err)
		}
		r := c.chamar(t, "GET", "/api/open-finance/configuracao", "")
		conferirStatus(t, r, 200)
		testeapoio.ConferirJSON(t, r.Corpo, `{"clientId":null,"secretConfigurado":false,"intervaloHoras":6,"diasRetroativos":30,"habilitada":true,"meuNome":null}`)
	})
	t.Run("o gravado, sem o segredo", func(t *testing.T) {
		c := novoCenario(t)
		c.definir(t, chaveClientId, "cliente")
		c.definir(t, chaveClientSecret, "guardado")
		c.definir(t, chaveIntervaloHoras, "12")
		c.definir(t, chaveDiasRetroativos, "60")
		c.definir(t, chaveHabilitada, "False")
		c.definir(t, transferencias.ChaveMeuNome, "Ana Teste")
		r := c.chamar(t, "GET", "/api/open-finance/configuracao", "")
		testeapoio.ConferirJSON(t, r.Corpo, `{"clientId":"cliente","secretConfigurado":true,"intervaloHoras":12,"diasRetroativos":60,"habilitada":false,"meuNome":"Ana Teste"}`)
		if strings.Contains(string(r.Corpo), "guardado") {
			t.Errorf("o segredo saiu na resposta: %s", r.Corpo)
		}
	})
	t.Run("número que não se lê vale o padrão", func(t *testing.T) {
		c := novoCenario(t)
		c.definir(t, chaveIntervaloHoras, "seis")
		c.definir(t, chaveDiasRetroativos, "")
		r := c.chamar(t, "GET", "/api/open-finance/configuracao", "")
		testeapoio.ConferirJSON(t, r.Corpo, `{"clientId":null,"secretConfigurado":false,"intervaloHoras":6,"diasRetroativos":30,"habilitada":true,"meuNome":null}`)
	})
}

func TestSalvarConfiguracaoRecusa(t *testing.T) {
	casos := []struct{ nome, corpo, erro string }{
		{"sem o Client ID", `{}`, "The ClientId field is required."},
		{"Client ID em branco", `{"clientId":"  "}`, "The ClientId field is required."},
		{"Client ID longo", `{"clientId":"` + strings.Repeat("x", 101) + `"}`, "maximum length of 100"},
		{"Client Secret longo", `{"clientId":"c","clientSecret":"` + strings.Repeat("x", 201) + `"}`, "maximum length of 200"},
		{"intervalo abaixo", `{"clientId":"c","intervaloHoras":0}`, "The field IntervaloHoras must be between 1 and 48."},
		{"intervalo acima", `{"clientId":"c","intervaloHoras":49}`, "The field IntervaloHoras must be between 1 and 48."},
		{"dias abaixo", `{"clientId":"c","diasRetroativos":0}`, "The field DiasRetroativos must be between 1 and 365."},
		{"dias acima", `{"clientId":"c","diasRetroativos":366}`, "The field DiasRetroativos must be between 1 and 365."},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			c := novoCenario(t)
			r := c.chamar(t, "PUT", "/api/open-finance/configuracao", caso.corpo)
			conferirStatus(t, r, 400)
			if !strings.Contains(string(r.Corpo), caso.erro) {
				t.Errorf("corpo = %s, esperado %q", r.Corpo, caso.erro)
			}
			if v := c.valor(t, chaveClientId); v != nil {
				t.Errorf("gravou o Client ID %q", *v)
			}
		})
	}
}

func TestSalvarConfiguracao(t *testing.T) {
	t.Run("só o Client ID: o resto fica", func(t *testing.T) {
		c := novoCenario(t)
		c.definir(t, chaveIntervaloHoras, "12")
		c.definir(t, chaveDiasRetroativos, "60")
		c.definir(t, chaveHabilitada, "true")
		c.definir(t, chaveClientSecret, "antigo")
		r := c.chamar(t, "PUT", "/api/open-finance/configuracao", `{"clientId":"  cliente  ","clientSecret":"   "}`)
		conferirStatus(t, r, 204)
		if *c.valor(t, chaveClientId) != "cliente" {
			t.Errorf("Client ID = %q, esperado aparado", *c.valor(t, chaveClientId))
		}
		if *c.valor(t, chaveIntervaloHoras) != "12" || *c.valor(t, chaveDiasRetroativos) != "60" ||
			*c.valor(t, chaveHabilitada) != "true" || *c.valor(t, chaveClientSecret) != "antigo" {
			t.Error("campo omitido mudou")
		}
		if n := c.auditoria(t, "CredencialPluggyAlterada"); n != 0 {
			t.Errorf("%d auditorias, sem segredo novo", n)
		}
	})
	t.Run("tudo, com o segredo cifrado e a auditoria", func(t *testing.T) {
		c := novoCenario(t)
		r := c.chamar(t, "PUT", "/api/open-finance/configuracao",
			`{"clientId":"cliente","clientSecret":"  segredo  ","intervaloHoras":1,"diasRetroativos":365,"habilitada":false}`)
		conferirStatus(t, r, 204)
		if *c.valor(t, chaveIntervaloHoras) != "1" || *c.valor(t, chaveDiasRetroativos) != "365" || *c.valor(t, chaveHabilitada) != "false" {
			t.Error("valores gravados errados")
		}
		if v := *c.valor(t, chaveClientSecret); v != "v2:segredo:cifrado" {
			t.Errorf("segredo gravado = %q", v)
		}
		if n := c.contar(t, `SELECT COUNT(*) FROM "Configuracao" WHERE "Chave" = ? AND "Protegido"`, chaveClientSecret); n != 1 {
			t.Error("o segredo não ficou marcado como protegido")
		}
		if n := c.auditoria(t, "CredencialPluggyAlterada"); n != 1 {
			t.Errorf("%d auditorias, esperado 1", n)
		}
	})
	t.Run("habilitada true e os limites da faixa", func(t *testing.T) {
		c := novoCenario(t)
		r := c.chamar(t, "PUT", "/api/open-finance/configuracao", `{"clientId":"c","intervaloHoras":48,"diasRetroativos":1,"habilitada":true}`)
		conferirStatus(t, r, 204)
		if *c.valor(t, chaveIntervaloHoras) != "48" || *c.valor(t, chaveDiasRetroativos) != "1" || *c.valor(t, chaveHabilitada) != "true" {
			t.Error("valores gravados errados")
		}
	})
}

func TestRemoverSecret(t *testing.T) {
	c := novoCenario(t)
	c.definir(t, chaveClientSecret, "guardado")
	conferirStatus(t, c.chamar(t, "DELETE", "/api/open-finance/configuracao/secret", ""), 204)
	if n := c.contar(t, `SELECT COUNT(*) FROM "Configuracao" WHERE "Chave" = ?`, chaveClientSecret); n != 0 {
		t.Error("a linha do segredo ficou")
	}
	if n := c.auditoria(t, "CredencialPluggyRemovida"); n != 1 {
		t.Errorf("%d auditorias, esperado 1", n)
	}
	conferirStatus(t, c.chamar(t, "DELETE", "/api/open-finance/configuracao/secret", ""), 204)
}

func TestSalvarMeuNome(t *testing.T) {
	casos := []struct {
		nome, corpo string
		status      int
		gravado     *string
		mensagem    string
	}{
		{"nome e sobrenome, aparado", `{"nome":"  Ana Teste  "}`, 204, p("Ana Teste"), ""},
		{"um nome só", `{"nome":"Ana"}`, 400, nil, "Informe pelo menos o primeiro e o último nome, como aparecem nos Pix."},
		{"em branco apaga", `{"nome":"   "}`, 204, nil, ""},
		{"vazio apaga", `{"nome":""}`, 204, nil, ""},
		{"null apaga", `{"nome":null}`, 204, nil, ""},
		{"sem o campo apaga", `{}`, 204, nil, ""},
		{"121 caracteres", `{"nome":"` + strings.Repeat("a", 121) + `"}`, 400, nil, "O nome pode ter até 120 caracteres."},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			c := novoCenario(t)
			r := c.chamar(t, "PUT", "/api/open-finance/meu-nome", caso.corpo)
			conferirStatus(t, r, caso.status)
			if !strings.Contains(string(r.Corpo), caso.mensagem) {
				t.Errorf("corpo = %s, esperado %q", r.Corpo, caso.mensagem)
			}
			v := c.valor(t, transferencias.ChaveMeuNome)
			if (v == nil) != (caso.gravado == nil) || (v != nil && *v != *caso.gravado) {
				t.Errorf("gravado = %v, esperado %v", v, caso.gravado)
			}
		})
	}

	t.Run("reclassifica na hora", func(t *testing.T) {
		c := novoCenario(t)
		categoria := c.fab.Categoria(func(k *testeapoio.Categoria) { k.Nome, k.Tipo = "Transferências", transacoes.Transferencia })
		conta := c.fab.Conta()
		id := c.fab.Transacao(conta, func(tr *testeapoio.Transacao) {
			tr.Descricao, tr.DescricaoOriginal, tr.CategoriaId = "PIX ENVIADO ANA TESTE", p("PIX ENVIADO ANA TESTE"), &categoria.Id
		}).Id
		conferirStatus(t, c.chamar(t, "PUT", "/api/open-finance/meu-nome", `{"nome":"Ana Teste"}`), 204)
		if n := c.contar(t, `SELECT COUNT(*) FROM "Transacao" WHERE "Id" = ? AND "Tipo" = ? AND "IgnorarRelatorios"`, id, transacoes.Transferencia); n != 1 {
			t.Error("o Pix com o seu nome não virou transferência")
		}
	})
}

func TestListarConexoes(t *testing.T) {
	c := novoCenario(t)
	r := c.chamar(t, "GET", "/api/open-finance/conexoes", "")
	conferirStatus(t, r, 200)
	testeapoio.ConferirJSON(t, r.Corpo, `[]`)

	datas := tipos.NovaDataHora(time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC))
	zebra := c.fab.ConexaoBancaria(func(k *testeapoio.ConexaoBancaria) {
		k.NomeInstituicao, k.PluggyItemId, k.Ativa = p("Zebra"), "item-z", false
	})
	acento := c.fab.ConexaoBancaria(func(k *testeapoio.ConexaoBancaria) {
		k.NomeInstituicao, k.PluggyItemId, k.CorPrimaria, k.Status, k.StatusExecucao = p("Água"), "item-a", p("#112233"), p("UPDATED"), p("SUCCESS")
		k.UltimaAtualizacaoInstituicao, k.ConsentimentoExpiraEm, k.UltimaSincronizacaoEm, k.UltimoErro = &datas, &datas, &datas, p("falhou")
	})
	banco := c.fab.ConexaoBancaria(func(k *testeapoio.ConexaoBancaria) { k.NomeInstituicao, k.PluggyItemId = p("Banco"), "item-b" })
	for range 2 {
		c.fab.Conta(func(k *testeapoio.Conta) { k.ConexaoBancariaId = &acento.Id })
	}

	r = c.chamar(t, "GET", "/api/open-finance/conexoes", "")
	conferirStatus(t, r, 200)
	var lista []conexaoDto
	if err := json.Unmarshal(r.Corpo, &lista); err != nil {
		t.Fatal(err)
	}
	ordem := []int64{}
	for _, k := range lista {
		ordem = append(ordem, k.Id)
	}
	if !slices.Equal(ordem, []int64{acento.Id, banco.Id, zebra.Id}) {
		t.Errorf("ordem = %v: Água vem antes de Banco na ordem pt-BR", ordem)
	}
	a := lista[0]
	if a.PluggyItemId != "item-a" || *a.NomeInstituicao != "Água" || *a.CorPrimaria != "#112233" || *a.Status != "UPDATED" ||
		*a.StatusExecucao != "SUCCESS" || *a.UltimoErro != "falhou" || !a.Ativa || a.QuantidadeContas != 2 || a.NomeExibicao == nil {
		t.Errorf("conexão = %+v", a)
	}
	if lista[2].Ativa {
		t.Error("a conexão desligada veio ligada")
	}
	if !strings.Contains(string(r.Corpo), `"ultimaAtualizacaoInstituicao":"2026-10-14T09:00:00"`) {
		t.Errorf("data fora do formato: %s", r.Corpo)
	}
}

func TestCompararOpcional(t *testing.T) {
	casos := []struct {
		nome     string
		a, b     *string
		esperado int
	}{
		{"dois nulos", nil, nil, 0},
		{"nulo primeiro", nil, p("a"), -1},
		{"nulo depois", p("a"), nil, 1},
		{"ordem pt-BR", p("Água"), p("Banco"), -1},
		{"iguais", p("a"), p("a"), 0},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			if obtido := compararOpcional(caso.a, caso.b); obtido != caso.esperado {
				t.Errorf("compararOpcional = %d, esperado %d", obtido, caso.esperado)
			}
		})
	}
}

func TestAlterarAtiva(t *testing.T) {
	casos := []struct {
		nome, caminho string
		status        int
		ativa         bool
		erro          string
	}{
		{"desliga", "%d/ativa?ativa=false", 204, false, ""},
		{"liga", "%d/ativa?ativa=true", 204, true, ""},
		{"sem o parâmetro desliga", "%d/ativa", 204, false, ""},
		{"valor que não é booleano", "%d/ativa?ativa=talvez", 400, true, "is not valid"},
		{"conexão que não existe", "999999/ativa?ativa=true", 404, true, mensagemNaoEncontrado},
		{"id que não é número", "abc/ativa?ativa=true", 404, true, ""},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			c := novoCenario(t)
			k := c.fab.ConexaoBancaria()
			caminho := caso.caminho
			if strings.Contains(caminho, "%d") {
				caminho = fmt.Sprintf(caminho, k.Id)
			}
			r := c.chamar(t, "PUT", "/api/open-finance/conexoes/"+caminho, "")
			conferirStatus(t, r, caso.status)
			if !strings.Contains(string(r.Corpo), caso.erro) {
				t.Errorf("corpo = %s, esperado %q", r.Corpo, caso.erro)
			}
			var ativa bool
			if err := c.db.QueryRow(`SELECT "Ativa" FROM "ConexaoBancaria" WHERE "Id" = ?`, k.Id).Scan(&ativa); err != nil {
				t.Fatal(err)
			}
			if ativa != caso.ativa {
				t.Errorf("ativa = %v, esperado %v", ativa, caso.ativa)
			}
		})
	}
}

// Regra (#575): tudo o que veio da conexão sai, numa transação; a outra conexão não muda.
func TestRemoverConexao(t *testing.T) {
	c := novoCenario(t)
	sai := c.fab.ConexaoBancaria()
	fica := c.fab.ConexaoBancaria()
	popular := func(k testeapoio.ConexaoBancaria) (conta testeapoio.Conta) {
		conta = c.fab.Conta(func(ct *testeapoio.Conta) { ct.ConexaoBancariaId = &k.Id })
		fatura := c.fab.Fatura(conta, tipos.NovaData(2026, 10, 20), testeapoio.Valor("10.0"))
		c.fab.Transacao(conta, func(tr *testeapoio.Transacao) { tr.FaturaCartaoId = &fatura.Id })
		c.fab.Investimento(k)
		c.fab.Emprestimo(k)
		if _, err := c.db.Exec(`INSERT INTO "HistoricoInvestimento" ("PluggyInvestmentId", "ConexaoBancariaId", "Nome", "Tipo", "Data",
			"Saldo", "AtualizadoEm") VALUES (?, ?, 'CDB', 'FIXED_INCOME', '2026-10-01', '10.0', '2026-10-01 09:00:00')`, fmt.Sprint("inv-", k.Id), k.Id); err != nil {
			t.Fatal(err)
		}
		if _, err := c.db.Exec(`INSERT INTO "SincronizacaoLog" ("ConexaoBancariaId", "Origem", "IniciadoEm") VALUES (?, 'Manual', '2026-10-01 09:00:00')`, k.Id); err != nil {
			t.Fatal(err)
		}
		return conta
	}
	popular(sai)
	ficou := popular(fica)

	conferirStatus(t, c.chamar(t, "DELETE", fmt.Sprintf("/api/open-finance/conexoes/%d", sai.Id), ""), 204)

	for _, tabela := range []string{"Conta", "Investimento", "Emprestimo", "HistoricoInvestimento", "SincronizacaoLog"} {
		if n := c.contar(t, `SELECT COUNT(*) FROM "`+tabela+`" WHERE "ConexaoBancariaId" = ?`, sai.Id); n != 0 {
			t.Errorf("%s: %d linhas da conexão removida", tabela, n)
		}
		if n := c.contar(t, `SELECT COUNT(*) FROM "`+tabela+`" WHERE "ConexaoBancariaId" = ?`, fica.Id); n != 1 {
			t.Errorf("%s: %d linhas da outra conexão, esperado 1", tabela, n)
		}
	}
	if n := c.contar(t, `SELECT COUNT(*) FROM "ConexaoBancaria"`); n != 1 {
		t.Errorf("%d conexões, esperado 1", n)
	}
	// Sem o DELETE, o ON DELETE SET NULL deixa a linha com a coluna nula, e a contagem por conexão não a vê.
	if n := c.contar(t, `SELECT COUNT(*) FROM "SincronizacaoLog"`); n != 1 {
		t.Errorf("%d logs, esperado 1: o da conexão removida ficou", n)
	}
	if n := c.contar(t, `SELECT COUNT(*) FROM "Transacao"`); n != 1 {
		t.Errorf("%d transações, esperado 1", n)
	}
	if n := c.contar(t, `SELECT COUNT(*) FROM "Transacao" WHERE "ContaId" = ? AND "FaturaCartaoId" IS NOT NULL`, ficou.Id); n != 1 {
		t.Error("a transação da outra conexão perdeu a fatura")
	}
	if n := c.auditoria(t, "ConexaoRemovida"); n != 1 {
		t.Errorf("%d auditorias, esperado 1", n)
	}
}

func TestRemoverConexaoRecusa(t *testing.T) {
	t.Run("não existe", func(t *testing.T) {
		c := novoCenario(t)
		r := c.chamar(t, "DELETE", "/api/open-finance/conexoes/999", "")
		conferirStatus(t, r, 404)
		if !strings.Contains(string(r.Corpo), mensagemNaoEncontrado) {
			t.Errorf("corpo = %s", r.Corpo)
		}
	})
	t.Run("id que não é número", func(t *testing.T) {
		conferirStatus(t, novoCenario(t).chamar(t, "DELETE", "/api/open-finance/conexoes/abc", ""), 404)
	})
	t.Run("sincronização em andamento", func(t *testing.T) {
		c := novoCenario(t)
		k := c.fab.ConexaoBancaria()
		c.s.emExecucao.Store(true)
		r := c.chamar(t, "DELETE", fmt.Sprintf("/api/open-finance/conexoes/%d", k.Id), "")
		conferirStatus(t, r, 409)
		if !strings.Contains(string(r.Corpo), "Aguarde a sincronização em andamento terminar.") {
			t.Errorf("corpo = %s", r.Corpo)
		}
		if n := c.contar(t, `SELECT COUNT(*) FROM "ConexaoBancaria"`); n != 1 {
			t.Error("removeu durante a sincronização")
		}
	})
}

func TestEstado(t *testing.T) {
	c := novoCenario(t)
	testeapoio.ConferirJSON(t, c.chamar(t, "GET", "/api/open-finance/sincronizacao/estado", "").Corpo, `{"emExecucao":false}`)
	c.s.emExecucao.Store(true)
	testeapoio.ConferirJSON(t, c.chamar(t, "GET", "/api/open-finance/sincronizacao/estado", "").Corpo, `{"emExecucao":true}`)
}

func TestStatus(t *testing.T) {
	c := novoCenario(t)
	testeapoio.ConferirJSON(t, c.chamar(t, "GET", "/api/open-finance/status", "").Corpo,
		`{"ultimaSincronizacao":null,"emExecucao":false,"conexoesAtivas":0,"conexoesComErro":0,"consentimentosVencendo":0}`)

	noLimite := tipos.NovaDataHora(time.Date(2026, 11, 14, 15, 0, 0, 0, time.UTC)) // 30 dias depois do relógio, exatos
	depois := tipos.NovaDataHora(time.Date(2026, 11, 14, 15, 0, 1, 0, time.UTC))
	vencido := tipos.NovaDataHora(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	conexoes := []func(*testeapoio.ConexaoBancaria){
		func(k *testeapoio.ConexaoBancaria) { k.ConsentimentoExpiraEm = &noLimite },
		func(k *testeapoio.ConexaoBancaria) { k.ConsentimentoExpiraEm = &depois },
		func(k *testeapoio.ConexaoBancaria) { k.ConsentimentoExpiraEm = &vencido },
		func(k *testeapoio.ConexaoBancaria) { k.UltimoErro = p("falhou") },
		func(k *testeapoio.ConexaoBancaria) { k.UltimoErro = p("") },
		func(k *testeapoio.ConexaoBancaria) { k.Status = p("LOGIN_ERROR") },
		func(k *testeapoio.ConexaoBancaria) { k.Status = p("WAITING_USER_ACTION") },
		func(k *testeapoio.ConexaoBancaria) { k.Status = p("WAITING_USER_INPUT") }, // #532: não conta, como no .NET hoje
		func(k *testeapoio.ConexaoBancaria) {
			k.Ativa, k.UltimoErro, k.ConsentimentoExpiraEm = false, p("falhou"), &vencido
		}, // desligada
	}
	for _, ajustar := range conexoes {
		c.fab.ConexaoBancaria(ajustar)
	}
	c.definir(t, configuracao.UltimaSincronizacaoGeral, "2026-10-14T09:00:00.0000000Z")
	c.s.emExecucao.Store(true)
	testeapoio.ConferirJSON(t, c.chamar(t, "GET", "/api/open-finance/status", "").Corpo,
		`{"ultimaSincronizacao":"2026-10-14T09:00:00Z","emExecucao":true,"conexoesAtivas":8,"conexoesComErro":3,"consentimentosVencendo":2}`)
}

// Regra: a mesma do Painel. Uma conexão por caso: na soma, uma condição invertida compensa a outra.
func TestStatusConexaoComErro(t *testing.T) {
	casos := []struct {
		nome   string
		erro   *string
		status *string
		conta  int
	}{
		{"sem nada", nil, nil, 0},
		{"tudo certo", nil, p("UPDATED"), 0},
		{"erro gravado", p("falhou"), nil, 1},
		{"erro vazio", p(""), nil, 0},
		{"LOGIN_ERROR", nil, p("LOGIN_ERROR"), 1},
		{"WAITING_USER_ACTION", nil, p("WAITING_USER_ACTION"), 1},
		{"WAITING_USER_INPUT não conta, como no .NET (#532)", nil, p("WAITING_USER_INPUT"), 0},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			c := novoCenario(t)
			c.fab.ConexaoBancaria(func(k *testeapoio.ConexaoBancaria) { k.UltimoErro, k.Status = caso.erro, caso.status })
			var d statusDto
			if err := json.Unmarshal(c.chamar(t, "GET", "/api/open-finance/status", "").Corpo, &d); err != nil {
				t.Fatal(err)
			}
			if d.ConexoesComErro != caso.conta || d.ConexoesAtivas != 1 {
				t.Errorf("com erro %d de %d, esperado %d de 1", d.ConexoesComErro, d.ConexoesAtivas, caso.conta)
			}
		})
	}
}

func TestStatusDataSemFusoViraUTC(t *testing.T) {
	c := novoCenario(t)
	c.definir(t, configuracao.UltimaSincronizacaoGeral, "2026-10-14 06:00:00")
	testeapoio.ConferirJSON(t, c.chamar(t, "GET", "/api/open-finance/status", "").Corpo,
		`{"ultimaSincronizacao":"2026-10-14T09:00:00Z","emExecucao":false,"conexoesAtivas":0,"conexoesComErro":0,"consentimentosVencendo":0}`)
}

func TestLogs(t *testing.T) {
	c := novoCenario(t)
	testeapoio.ConferirJSON(t, c.chamar(t, "GET", "/api/open-finance/logs", "").Corpo, `[]`)

	k := c.fab.ConexaoBancaria(func(k *testeapoio.ConexaoBancaria) { k.NomeInstituicao = p("MeuPluggy") })
	c.fab.Conta(func(ct *testeapoio.Conta) { ct.ConexaoBancariaId, ct.Banco = &k.Id, p("nubank") })
	inserir := func(conexao *int64, mensagem *string, sucesso bool) {
		if _, err := c.db.Exec(`INSERT INTO "SincronizacaoLog" ("ConexaoBancariaId", "Origem", "IniciadoEm", "FinalizadoEm", "Sucesso",
			"TransacoesNovas", "TransacoesAtualizadas", "Mensagem") VALUES (?, 'Manual', '2026-10-14 09:00:00', '2026-10-14 09:00:37.5', ?, 4, 2, ?)`,
			conexao, sucesso, mensagem); err != nil {
			t.Fatal(err)
		}
	}
	for range 51 {
		inserir(&k.Id, nil, true)
	}
	inserir(nil, p("sem conexão"), false)

	r := c.chamar(t, "GET", "/api/open-finance/logs", "")
	conferirStatus(t, r, 200)
	var lista []logDto
	if err := json.Unmarshal(r.Corpo, &lista); err != nil {
		t.Fatal(err)
	}
	if len(lista) != 50 {
		t.Fatalf("%d logs, esperado 50", len(lista))
	}
	if lista[0].Id != 52 || lista[49].Id != 3 {
		t.Errorf("logs de %d a %d, esperado do 52 ao 3", lista[0].Id, lista[49].Id)
	}
	if primeiro := lista[0]; primeiro.ConexaoBancariaId != nil || primeiro.NomeInstituicao != nil || primeiro.Sucesso || *primeiro.Mensagem != "sem conexão" {
		t.Errorf("log sem conexão = %+v", primeiro)
	}
	segundo := lista[1]
	if *segundo.ConexaoBancariaId != k.Id || *segundo.NomeInstituicao != "Nubank" || !segundo.Sucesso || segundo.Origem != "Manual" ||
		segundo.TransacoesNovas != 4 || segundo.TransacoesAtualizadas != 2 || segundo.Mensagem != nil {
		t.Errorf("log = %+v", segundo)
	}
	if !strings.Contains(string(r.Corpo), `"finalizadoEm":"2026-10-14T09:00:37.5"`) {
		t.Errorf("data fora do formato: %s", r.Corpo)
	}
}

func (c cenario) categoriaPluggy(t *testing.T, id, descricao string, pai, paiDescricao *string, categoria *int64) {
	t.Helper()
	if _, err := c.db.Exec(`INSERT INTO "CategoriaPluggy" ("PluggyCategoriaId", "Descricao", "PaiPluggyId", "PaiDescricao", "CategoriaId")
		VALUES (?, ?, ?, ?, ?)`, id, descricao, pai, paiDescricao, categoria); err != nil {
		t.Fatal(err)
	}
}

func TestCategorias(t *testing.T) {
	c := novoCenario(t)
	testeapoio.ConferirJSON(t, c.chamar(t, "GET", "/api/open-finance/categorias", "").Corpo, `[]`)

	minha := c.fab.Categoria(func(k *testeapoio.Categoria) { k.Nome = "Mercado" })
	c.categoriaPluggy(t, "pai-b", "Zebra", nil, p("Banco"), nil)
	c.categoriaPluggy(t, "pai-a", "Zebra", nil, p("Água"), nil)
	c.categoriaPluggy(t, "sem-pai", "Outro", nil, nil, nil)
	c.categoriaPluggy(t, "desc-b", "Banco", nil, p("Mesmo"), nil)
	c.categoriaPluggy(t, "desc-a", "Água", nil, p("Mesmo"), nil)
	c.categoriaPluggy(t, "usada", "Usada", nil, nil, &minha.Id)
	conta := c.fab.Conta()
	for range 2 {
		c.fab.Transacao(conta, func(tr *testeapoio.Transacao) { tr.PluggyCategoriaId = p("usada") })
	}

	r := c.chamar(t, "GET", "/api/open-finance/categorias", "")
	conferirStatus(t, r, 200)
	var lista []categoriaDto
	if err := json.Unmarshal(r.Corpo, &lista); err != nil {
		t.Fatal(err)
	}
	ordem := []string{}
	for _, k := range lista {
		ordem = append(ordem, k.PluggyCategoriaId)
	}
	if esperado := []string{"usada", "sem-pai", "pai-a", "pai-b", "desc-a", "desc-b"}; !slices.Equal(ordem, esperado) {
		t.Errorf("ordem = %v, esperado %v", ordem, esperado)
	}
	u := lista[0]
	if u.QuantidadeTransacoes != 2 || *u.CategoriaId != minha.Id || *u.CategoriaNome != "Mercado" || u.Descricao != "Usada" || u.PaiDescricao != nil {
		t.Errorf("categoria = %+v", u)
	}
	if lista[1].CategoriaId != nil || lista[1].CategoriaNome != nil || lista[1].QuantidadeTransacoes != 0 {
		t.Errorf("categoria sem mapa = %+v", lista[1])
	}
}

func TestMapearCategoriaRecusa(t *testing.T) {
	casos := []struct {
		nome, id, corpo string
		status          int
		mensagem        string
	}{
		{"categoria da Pluggy que não existe", "nao-existe", `{"categoriaId":null}`, 404, `{"mensagem":"Registro não encontrado."}`},
		{"categoria sua que não existe", "pluggy", `{"categoriaId":999}`, 400, `{"mensagem":"Categoria inválida."}`},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			c := novoCenario(t)
			c.categoriaPluggy(t, "pluggy", "Pluggy", nil, nil, nil)
			r := c.chamar(t, "PUT", "/api/open-finance/categorias/"+caso.id, caso.corpo)
			conferirStatus(t, r, caso.status)
			testeapoio.ConferirJSON(t, r.Corpo, caso.mensagem)
		})
	}
	t.Run("campo fora do tipo", func(t *testing.T) {
		c := novoCenario(t)
		c.categoriaPluggy(t, "pluggy", "Pluggy", nil, nil, nil)
		r := c.chamar(t, "PUT", "/api/open-finance/categorias/pluggy", `{"categoriaId":"x"}`)
		conferirStatus(t, r, 400)
		if !strings.Contains(string(r.Corpo), "could not be converted") {
			t.Errorf("corpo = %s", r.Corpo)
		}
	})
}

func TestMapearCategoria(t *testing.T) {
	antigo := tipos.NovaDataHora(time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC))
	type montagem struct {
		c          cenario
		minha      testeapoio.Categoria
		automatica int64
		filha      int64
		manual     int64
		outra      int64
	}
	montar := func(t *testing.T) montagem {
		t.Helper()
		c := novoCenario(t)
		minha := c.fab.Categoria(func(k *testeapoio.Categoria) { k.Nome = "Mercado" })
		antiga := c.fab.Categoria(func(k *testeapoio.Categoria) { k.Nome = "Antiga" })
		c.categoriaPluggy(t, "pai", "Pai", nil, nil, &antiga.Id)
		c.categoriaPluggy(t, "filha", "Filha", p("pai"), nil, nil)
		c.categoriaPluggy(t, "fora", "Fora", nil, nil, &antiga.Id)
		conta := c.fab.Conta()
		lancar := func(pluggy string, manual bool) int64 {
			return c.fab.Transacao(conta, func(tr *testeapoio.Transacao) {
				tr.PluggyCategoriaId, tr.CategoriaId, tr.CategoriaManual, tr.AtualizadoEm = &pluggy, &antiga.Id, manual, antigo
			}).Id
		}
		return montagem{c, minha, lancar("pai", false), lancar("filha", false), lancar("pai", true), lancar("fora", false)}
	}
	categoriaDe := func(t *testing.T, c cenario, id int64) *int64 {
		t.Helper()
		var k *int64
		if err := c.db.QueryRow(`SELECT "CategoriaId" FROM "Transacao" WHERE "Id" = ?`, id).Scan(&k); err != nil {
			t.Fatal(err)
		}
		return k
	}

	t.Run("sem aplicar: só o mapa muda", func(t *testing.T) {
		m := montar(t)
		r := m.c.chamar(t, "PUT", "/api/open-finance/categorias/pai", fmt.Sprintf(`{"categoriaId":%d}`, m.minha.Id))
		conferirStatus(t, r, 200)
		testeapoio.ConferirJSON(t, r.Corpo, `{"transacoesAtualizadas":0}`)
		if n := m.c.contar(t, `SELECT COUNT(*) FROM "CategoriaPluggy" WHERE "PluggyCategoriaId" = 'pai' AND "CategoriaId" = ?`, m.minha.Id); n != 1 {
			t.Error("o mapa não mudou")
		}
		if k := categoriaDe(t, m.c, m.automatica); *k == m.minha.Id {
			t.Error("mudou a transação sem pedir")
		}
	})
	t.Run("null tira o mapa", func(t *testing.T) {
		m := montar(t)
		conferirStatus(t, m.c.chamar(t, "PUT", "/api/open-finance/categorias/pai", `{"categoriaId":null}`), 200)
		if n := m.c.contar(t, `SELECT COUNT(*) FROM "CategoriaPluggy" WHERE "PluggyCategoriaId" = 'pai' AND "CategoriaId" IS NULL`); n != 1 {
			t.Error("o mapa ficou")
		}
	})
	t.Run("aplicando: a categoria e as filhas, sem as manuais", func(t *testing.T) {
		m := montar(t)
		if _, err := m.c.db.Exec(`UPDATE "CategoriaPluggy" SET "CategoriaId" = ? WHERE "PluggyCategoriaId" = 'filha'`, m.minha.Id); err != nil {
			t.Fatal(err)
		}
		r := m.c.chamar(t, "PUT", "/api/open-finance/categorias/pai", fmt.Sprintf(`{"categoriaId":%d,"aplicarExistentes":true}`, m.minha.Id))
		conferirStatus(t, r, 200)
		testeapoio.ConferirJSON(t, r.Corpo, `{"transacoesAtualizadas":2}`)
		for nome, id := range map[string]int64{"da categoria": m.automatica, "da filha": m.filha} {
			if k := categoriaDe(t, m.c, id); k == nil || *k != m.minha.Id {
				t.Errorf("transação %s: categoria %v", nome, k)
			}
		}
		for nome, id := range map[string]int64{"manual": m.manual, "de outra categoria da Pluggy": m.outra} {
			if k := categoriaDe(t, m.c, id); k == nil || *k == m.minha.Id {
				t.Errorf("transação %s mudou: categoria %v", nome, k)
			}
		}
		if n := m.c.contar(t, `SELECT COUNT(*) FROM "Transacao" WHERE "Id" = ? AND "AtualizadoEm" = ?`, m.automatica, m.c.s.agora()); n != 1 {
			t.Error("AtualizadoEm não mudou")
		}
		if n := m.c.contar(t, `SELECT COUNT(*) FROM "Transacao" WHERE "Id" = ? AND "AtualizadoEm" = ?`, m.manual, antigo); n != 1 {
			t.Error("a transação manual teve o AtualizadoEm mexido")
		}
	})
	t.Run("aplicando acerta o tipo pela categoria nova", func(t *testing.T) {
		c := novoCenario(t)
		aplicacoes := c.fab.Categoria(func(k *testeapoio.Categoria) { k.Nome, k.Tipo = "Aplicações", transacoes.Transferencia })
		c.categoriaPluggy(t, "pluggy", "Pluggy", nil, nil, nil)
		id := c.fab.Transacao(c.fab.Conta(), func(tr *testeapoio.Transacao) { tr.PluggyCategoriaId = p("pluggy") }).Id
		r := c.chamar(t, "PUT", "/api/open-finance/categorias/pluggy", fmt.Sprintf(`{"categoriaId":%d,"aplicarExistentes":true}`, aplicacoes.Id))
		testeapoio.ConferirJSON(t, r.Corpo, `{"transacoesAtualizadas":1}`)
		if n := c.contar(t, `SELECT COUNT(*) FROM "Transacao" WHERE "Id" = ? AND "Tipo" = ? AND "IgnorarRelatorios"`, id, transacoes.Transferencia); n != 1 {
			t.Error("a despesa não virou transferência com a categoria nova")
		}
	})
	t.Run("aplicando duas vezes: a segunda não muda nada", func(t *testing.T) {
		m := montar(t)
		corpo := fmt.Sprintf(`{"categoriaId":%d,"aplicarExistentes":true}`, m.minha.Id)
		m.c.chamar(t, "PUT", "/api/open-finance/categorias/pai", corpo)
		testeapoio.ConferirJSON(t, m.c.chamar(t, "PUT", "/api/open-finance/categorias/pai", corpo).Corpo, `{"transacoesAtualizadas":0}`)
	})
	t.Run("a que já está certa não interrompe as seguintes", func(t *testing.T) {
		c := novoCenario(t)
		minha := c.fab.Categoria(func(k *testeapoio.Categoria) { k.Nome = "Mercado" })
		c.categoriaPluggy(t, "pai", "Pai", nil, nil, nil)
		conta := c.fab.Conta()
		c.fab.Transacao(conta, func(tr *testeapoio.Transacao) { tr.PluggyCategoriaId, tr.CategoriaId = p("pai"), &minha.Id })
		depois := c.fab.Transacao(conta, func(tr *testeapoio.Transacao) { tr.PluggyCategoriaId = p("pai") }).Id
		r := c.chamar(t, "PUT", "/api/open-finance/categorias/pai", fmt.Sprintf(`{"categoriaId":%d,"aplicarExistentes":true}`, minha.Id))
		testeapoio.ConferirJSON(t, r.Corpo, `{"transacoesAtualizadas":1}`)
		if k := categoriaDe(t, c, depois); k == nil || *k != minha.Id {
			t.Errorf("a transação seguinte ficou com a categoria %v", k)
		}
	})
	t.Run("aplicando acerta as transferências", func(t *testing.T) {
		c := novoCenario(t)
		generica := c.fab.Categoria(func(k *testeapoio.Categoria) { k.Nome, k.Tipo = "Transferências", transacoes.Transferencia })
		c.categoriaPluggy(t, "pix", "Pix", nil, nil, nil)
		id := c.fab.Transacao(c.fab.Conta(), func(tr *testeapoio.Transacao) { tr.PluggyCategoriaId = p("pix") }).Id
		r := c.chamar(t, "PUT", "/api/open-finance/categorias/pix", fmt.Sprintf(`{"categoriaId":%d,"aplicarExistentes":true}`, generica.Id))
		testeapoio.ConferirJSON(t, r.Corpo, `{"transacoesAtualizadas":1}`)
		if n := c.contar(t, `SELECT COUNT(*) FROM "Transacao" WHERE "Id" = ? AND "Tipo" = ? AND NOT "IgnorarRelatorios"`, id, transacoes.Despesa); n != 1 {
			t.Error("sem par, a transferência tem de voltar a despesa")
		}
	})
}

// Regra: erro de banco em qualquer rota é o 500 do .NET, sem detalhe na resposta.
func TestBancoForaDoAr(t *testing.T) {
	casos := []struct{ metodo, caminho, corpo string }{
		{"GET", "/api/open-finance/configuracao", ""},
		{"PUT", "/api/open-finance/configuracao", `{"clientId":"c"}`},
		{"DELETE", "/api/open-finance/configuracao/secret", ""},
		{"PUT", "/api/open-finance/meu-nome", `{"nome":"Ana Teste"}`},
		{"GET", "/api/open-finance/conexoes", ""},
		{"PUT", "/api/open-finance/conexoes/1/ativa?ativa=true", ""},
		{"DELETE", "/api/open-finance/conexoes/1", ""},
		{"GET", "/api/open-finance/status", ""},
		{"GET", "/api/open-finance/logs", ""},
		{"GET", "/api/open-finance/categorias", ""},
		{"PUT", "/api/open-finance/categorias/x", `{"categoriaId":null}`},
	}
	for _, caso := range casos {
		t.Run(caso.metodo+" "+caso.caminho, func(t *testing.T) {
			c := novoCenario(t)
			if err := c.db.Close(); err != nil {
				t.Fatal(err)
			}
			r := c.chamar(t, caso.metodo, caso.caminho, caso.corpo)
			conferirStatus(t, r, 500)
			if strings.Contains(string(r.Corpo), "closed") {
				t.Errorf("detalhe do erro na resposta: %s", r.Corpo)
			}
		})
	}
}

// Regra: requisição cancelada não responde nem registra, venha o cancelamento pelo erro ou pelo contexto.
func TestErroInterno(t *testing.T) {
	cancelado, cancelar := context.WithCancel(t.Context())
	cancelar()
	casos := []struct {
		nome   string
		ctx    context.Context
		err    error
		status int
	}{
		{"erro de cancelamento", t.Context(), context.Canceled, 200},
		{"contexto cancelado", cancelado, errors.New("outro"), 200},
		{"erro de verdade", t.Context(), errors.New("outro"), 500},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			c := novoCenario(t)
			w := httptest.NewRecorder()
			c.s.erroInterno(w, httptest.NewRequestWithContext(caso.ctx, "GET", "/", nil), "teste", caso.err)
			if w.Code != caso.status {
				t.Errorf("status = %d, esperado %d", w.Code, caso.status)
			}
		})
	}
}

func TestMesmoId(t *testing.T) {
	casos := []struct {
		nome     string
		a, b     *int64
		esperado bool
	}{
		{"dois nulos", nil, nil, true},
		{"nulo e valor", nil, p(int64(1)), false},
		{"valor e nulo", p(int64(1)), nil, false},
		{"iguais", p(int64(1)), p(int64(1)), true},
		{"diferentes", p(int64(1)), p(int64(2)), false},
	}
	for _, caso := range casos {
		if obtido := mesmoId(caso.a, caso.b); obtido != caso.esperado {
			t.Errorf("%s: mesmoId = %v", caso.nome, obtido)
		}
	}
}
