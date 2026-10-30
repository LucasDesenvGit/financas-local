package extrato

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
)

func gatilhoEm(tabela, quando string) sabotagem {
	return executar(`CREATE TRIGGER sabotagem_` + tabela + ` BEFORE ` + quando + ` ON ` + tabela +
		` BEGIN SELECT RAISE(ABORT, 'sabotagem'); END`)
}

var (
	semAuditoria = executar("DROP TABLE AuditoriaAcesso")
	// O login de quem fez vem do Usuario (o da sessão): sem a tabela, a auditoria falha antes de gravar.
	semUsuario = executar("PRAGMA foreign_keys = OFF", "DROP TABLE Usuario")
)

// Regra: erro do banco em cada passo do renomear, da categoria em lote e dos Desfazer é o 500.
func TestErroDoBancoNoRenomearENoLote(t *testing.T) {
	type cenario struct {
		mux                                *http.ServeMux
		pix, outra, mercado, manual, regra int64
		categoria, transferencias          int64
	}
	montar := func(t *testing.T) (cenario, *sql.DB) {
		mux, db, fab := rotasDeTeste(t, minimoFixo{n: 4})
		conta := fab.Conta()
		c := cenario{mux: mux}
		c.regra = regraDescricao(t, db, "MERCADO", 3, "Mercado")
		c.pix = fab.Transacao(conta, doBanco("PIX JOAO")).Id
		c.outra = fab.Transacao(conta, doBanco("PIX JOAO")).Id
		c.mercado = fab.Transacao(conta, doBanco("MERCADO"), func(x *testeapoio.Transacao) { x.Descricao, x.RegraDescricaoId = "Mercado", &c.regra }).Id
		c.manual = fab.Transacao(conta).Id
		c.categoria = fab.Categoria().Id
		c.transferencias = fab.Categoria(func(x *testeapoio.Categoria) { x.Nome, x.Tipo = "Transferências", 3 }).Id
		fab.Transacao(conta, func(x *testeapoio.Transacao) { x.CategoriaId = &c.transferencias })
		return c, db
	}
	// chave faz a operação antes da sabotagem e devolve a chave do Desfazer.
	chave := func(t *testing.T, c cenario, metodo, caminho, corpo string) string {
		t.Helper()
		resp := testeapoio.Chamar(t, c.mux, metodo, caminho, corpo)
		var r struct{ Desfazer *string }
		if err := json.Unmarshal(resp.Corpo, &r); err != nil || r.Desfazer == nil {
			t.Fatalf("sem chave: %s", resp.Corpo)
		}
		return *r.Desfazer
	}
	renomear := func(c cenario) (string, string) {
		return fmt.Sprintf("/api/transacoes/%d/descricao", c.pix), `{"descricao":"Pix"}`
	}
	regra := func(c cenario) (string, string) {
		return fmt.Sprintf("/api/transacoes/%d/descricao", c.pix), `{"descricao":"Pix","aplicarNasOutras":true}`
	}
	lote := func(c cenario) (string, string) {
		return "/api/transacoes/descricao/em-lote", fmt.Sprintf(`{"ids":[%d,%d],"descricao":"Mercado novo","criarRegras":true}`,
			c.pix, c.manual)
	}
	loteExistente := func(c cenario) (string, string) {
		return "/api/transacoes/descricao/em-lote", fmt.Sprintf(`{"ids":[%d],"descricao":"Pix","criarRegras":true}`, c.pix)
	}
	categorias := func(c cenario) (string, string) {
		return "/api/transacoes/em-lote", fmt.Sprintf(`{"itens":[{"id":%d,"categoriaId":%d}]}`, c.pix, c.categoria)
	}
	automatica := func(c cenario) (string, string) {
		return "/api/transacoes/em-lote", fmt.Sprintf(`{"itens":[{"id":%d,"automatica":true}]}`, c.pix)
	}
	filtro := func(cenario) (string, string) {
		return "/api/transacoes/em-lote", `{"filtro":{"busca":"pix"},"automatica":true,"itens":null}`
	}
	casos := []struct {
		nome    string
		sabotar sabotagem
		metodo  string
		pedido  func(cenario) (string, string)
		// desfazer: o caminho do Desfazer da operação de pedido, que roda antes da sabotagem.
		desfazer string
	}{
		{"prévia: ler", fecharBanco, "GET", func(c cenario) (string, string) {
			return fmt.Sprintf("/api/transacoes/%d/renomear/previa", c.pix), ""
		}, ""},
		{"renomear: ler", fecharBanco, "PUT", renomear, ""},
		{"renomear: a auditoria", semAuditoria, "PUT", renomear, ""},
		{"renomear: gravar", semGravar("UPDATE"), "PUT", renomear, ""},
		{"renomear com regra: criar a regra", gatilhoEm("RegraDescricao", "INSERT"), "PUT", regra, ""},
		{"renomear com regra: reaplicar", semGravar("UPDATE"), "PUT", regra, ""},
		{"renomear em lote: ler", fecharBanco, "PATCH", lote, ""},
		{"renomear em lote: as regras", executar("PRAGMA foreign_keys = OFF", "DROP TABLE RegraDescricao"), "PATCH", lote, ""},
		{"renomear em lote: criar a regra", gatilhoEm("RegraDescricao", "INSERT"), "PATCH", lote, ""},
		{"renomear em lote: a regra que já existe", gatilhoEm("RegraDescricao", "UPDATE"), "PATCH", func(c cenario) (string, string) {
			return "/api/transacoes/descricao/em-lote", fmt.Sprintf(`{"ids":[%d],"descricao":"Outro","criarRegras":true}`, c.mercado)
		}, ""},
		{"renomear em lote: reaplicar", semGravar("UPDATE"), "PATCH", lote, ""},
		{"renomear em lote: a auditoria", semAuditoria, "PATCH", loteExistente, ""},
		{"desfazer o renomear: ler", semTransacoes, "PUT", renomear, "/api/transacoes/descricao/desfazer/"},
		{"desfazer o renomear: apagar a regra", gatilhoEm("RegraDescricao", "DELETE"), "PUT", regra, "/api/transacoes/descricao/desfazer/"},
		{"desfazer o renomear: a regra alterada", gatilhoEm("RegraDescricao", "UPDATE"), "PATCH", func(c cenario) (string, string) {
			return "/api/transacoes/descricao/em-lote", fmt.Sprintf(`{"ids":[%d],"descricao":"Outro","criarRegras":true}`, c.mercado)
		}, "/api/transacoes/descricao/desfazer/"},
		{"desfazer o renomear: a auditoria", semAuditoria, "PUT", renomear, "/api/transacoes/descricao/desfazer/"},
		{"desfazer o renomear: gravar", semGravar("UPDATE"), "PUT", renomear, "/api/transacoes/descricao/desfazer/"},
		{"categorias em lote: as categorias", fecharBanco, "PATCH", categorias, ""},
		{"categorias em lote: ler", fecharBanco, "PATCH", automatica, ""},
		{"categorias em lote: o filtro", fecharBanco, "PATCH", filtro, ""},
		{"categorias em lote: as regras", semRegras, "PATCH", categorias, ""},
		{"categorias em lote: a auditoria", semAuditoria, "PATCH", categorias, ""},
		{"categorias em lote: gravar", semGravar("UPDATE"), "PATCH", categorias, ""},
		{"categorias em lote: o acerto", semConfiguracao, "PATCH", categorias, ""},
		{"renomear: o login", semUsuario, "PUT", renomear, ""},
		{"renomear em lote: o login", semUsuario, "PATCH", loteExistente, ""},
		{"desfazer o renomear: o login", semUsuario, "PUT", renomear, "/api/transacoes/descricao/desfazer/"},
		{"categorias em lote: o login", semUsuario, "PATCH", categorias, ""},
		{"desfazer as categorias: o login", semUsuario, "PATCH", categorias, "/api/transacoes/em-lote/desfazer/"},
		{"desfazer as categorias: ler", semTransacoes, "PATCH", categorias, "/api/transacoes/em-lote/desfazer/"},
		{"desfazer as categorias: as categorias", semCategorias, "PATCH", categorias, "/api/transacoes/em-lote/desfazer/"},
		{"desfazer as categorias: a auditoria", semAuditoria, "PATCH", categorias, "/api/transacoes/em-lote/desfazer/"},
		{"desfazer as categorias: gravar", semGravar("UPDATE"), "PATCH", categorias, "/api/transacoes/em-lote/desfazer/"},
		{"desfazer as categorias: o acerto", semConfiguracao, "PATCH", categorias, "/api/transacoes/em-lote/desfazer/"},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			c, db := montar(t)
			caminho, corpo := caso.pedido(c)
			metodo := caso.metodo
			if caso.desfazer != "" {
				caminho, corpo, metodo = caso.desfazer+chave(t, c, metodo, caminho, corpo), "", "POST"
			}
			caso.sabotar(t, db)
			testeapoio.Chamar(t, c.mux, metodo, caminho, corpo).ConferirStatus(t, http.StatusInternalServerError)
		})
	}
}

// Regra: o erro dos parâmetros no filtro do lote (a pouco informativa precisa do mínimo de letras) é o 500.
func TestCategoriasEmLoteComErroNoFiltro(t *testing.T) {
	mux, _, _ := rotasDeTeste(t, minimoFixo{err: errors.New("parâmetros fora")})
	testeapoio.Chamar(t, mux, "PATCH", "/api/transacoes/em-lote", `{"filtro":{"poucoInformativa":true},"automatica":true}`).
		ConferirStatus(t, http.StatusInternalServerError)
}

// Regra: pelo filtro, no máximo 5000 transações de uma vez.
func TestCategoriasEmLoteComFiltroGrandeDemais(t *testing.T) {
	mux, db, fab := rotasDeTeste(t, minimoFixo{n: 4})
	conta := fab.Conta()
	if _, err := db.Exec(`WITH RECURSIVE n(i) AS (SELECT 0 UNION ALL SELECT i + 1 FROM n WHERE i + 1 < 5001)
		INSERT INTO "Transacao" ("ContaId", "Data", "Descricao", "Valor", "Tipo", "CriadoEm", "AtualizadoEm")
		SELECT ?, '2026-10-01', 'LOJA', '-1.0', 1, '2026-10-01 00:00:00', '2026-10-01 00:00:00' FROM n`, conta.Id); err != nil {
		t.Fatal(err)
	}
	resp := testeapoio.Chamar(t, mux, "PATCH", "/api/transacoes/em-lote", `{"filtro":{"busca":"loja"},"automatica":true}`)
	resp.ConferirStatus(t, http.StatusBadRequest)
	resp.ConferirJSON(t, `{"mensagem":"O filtro tem 5001 transações; o limite de uma vez é 5000. Restrinja o período."}`)
}
