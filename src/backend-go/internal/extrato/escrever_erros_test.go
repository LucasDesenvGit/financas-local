package extrato

import (
	"database/sql"
	"fmt"
	"net/http"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// sabotagem estraga o banco de um jeito só, para o erro vir de um passo conhecido da rota.
type sabotagem func(t *testing.T, db *sql.DB)

func executar(sqls ...string) sabotagem {
	return func(t *testing.T, db *sql.DB) {
		t.Helper()
		for _, s := range sqls {
			if _, err := db.Exec(s); err != nil {
				t.Fatal(err)
			}
		}
	}
}

var (
	fecharBanco sabotagem = func(t *testing.T, db *sql.DB) {
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}
	// Sem as tabelas, a consulta delas falha (as chaves estrangeiras ficam desligadas para o DROP passar).
	semRegras     = executar("PRAGMA foreign_keys = OFF", "DROP TABLE RegraCategorizacao")
	semCategorias = executar("PRAGMA foreign_keys = OFF", "DROP TABLE CategoriaPluggy", "DROP TABLE Categoria")
	semTransacoes = executar("PRAGMA foreign_keys = OFF", "DROP TABLE Transacao")
	// O acerto das transferências lê o seu nome nos Pix na Configuracao.
	semConfiguracao = executar("DROP TABLE Configuracao")
)

func semGravar(quando string) sabotagem {
	return executar(`CREATE TRIGGER sabotagem BEFORE ` + quando + ` ON Transacao BEGIN SELECT RAISE(ABORT, 'sabotagem'); END`)
}

// Regra: erro do banco em cada passo das rotas que gravam (ler, conferir a categoria ou as contas, as regras, gravar e
// o acerto das transferências) é o 500.
func TestErroDoBancoNasRotasQueGravam(t *testing.T) {
	type cenario struct{ manual, aberta, transferencia, conta, outra, categoria int64 }
	montar := func(t *testing.T) (*http.ServeMux, *sql.DB, cenario) {
		mux, db, fab := rotasDeTeste(t, minimoFixo{n: 4})
		conta, outra := fab.Conta(), fab.Conta()
		conexao := fab.ConexaoBancaria()
		banco := fab.Conta(func(x *testeapoio.Conta) { x.Origem, x.ConexaoBancariaId = 2, &conexao.Id })
		categoria := fab.Categoria()
		generica := fab.Categoria(func(x *testeapoio.Categoria) { x.Nome, x.Tipo = "Transferências", 3 })
		// Um lançamento na categoria genérica: o acerto das transferências chega até o seu nome nos Pix.
		fab.Transacao(conta, func(x *testeapoio.Transacao) { x.CategoriaId = &generica.Id })
		grupo := tipos.NovoGuid()
		return mux, db, cenario{
			manual:        fab.Transacao(conta).Id,
			aberta:        fab.Transacao(banco, doOpenFinance).Id,
			transferencia: fab.Transacao(conta, func(x *testeapoio.Transacao) { x.TransferenciaGrupo = &grupo }).Id,
			conta:         conta.Id, outra: outra.Id, categoria: categoria.Id,
		}
	}
	id := func(campo func(cenario) int64) func(cenario) string {
		return func(c cenario) string { return fmt.Sprintf("/api/transacoes/%d", campo(c)) }
	}
	fixo := func(caminho string) func(cenario) string { return func(cenario) string { return caminho } }
	manual := id(func(c cenario) int64 { return c.manual })
	aberta := id(func(c cenario) int64 { return c.aberta })
	salvar := func(c cenario) string {
		return fmt.Sprintf(`{"contaId":%d,"data":"2026-10-10","descricao":"Compra","valor":10,"tipo":1}`, c.conta)
	}
	ajuste := func(cenario) string { return `{"descricao":"x"}` }
	ajusteComCategoria := func(c cenario) string { return fmt.Sprintf(`{"descricao":"x","categoriaId":%d}`, c.categoria) }
	ids := func(c cenario) string { return fmt.Sprintf(`{"ids":[%d]}`, c.manual) }
	idsComCategoria := func(c cenario) string { return fmt.Sprintf(`{"ids":[%d],"categoriaId":%d}`, c.manual, c.categoria) }
	marcar := func(c cenario) string { return fmt.Sprintf(`{"ids":[%d],"emergencia":true}`, c.manual) }
	transferir := func(c cenario) string {
		return fmt.Sprintf(`{"contaOrigemId":%d,"contaDestinoId":%d,"valor":1,"data":"2026-10-10"}`, c.conta, c.outra)
	}
	casos := []struct {
		nome          string
		sabotar       sabotagem
		metodo        string
		caminho, body func(cenario) string
	}{
		{"alterar: ler", fecharBanco, "PUT", manual, salvar},
		{"alterar: as regras", semRegras, "PUT", manual, salvar},
		{"alterar: gravar", semGravar("UPDATE"), "PUT", manual, salvar},
		{"alterar: o acerto", semConfiguracao, "PUT", manual, salvar},
		{"ajustar: ler", fecharBanco, "PATCH", aberta, ajuste},
		{"ajustar: a categoria", semCategorias, "PATCH", aberta, ajusteComCategoria},
		{"ajustar: as regras", semRegras, "PATCH", aberta, ajuste},
		{"ajustar: gravar", semGravar("UPDATE"), "PATCH", aberta, ajuste},
		{"ajustar: o acerto", semConfiguracao, "PATCH", aberta, ajuste},
		{"excluir: ler", fecharBanco, "DELETE", manual, nil},
		{"excluir: apagar", semGravar("DELETE"), "DELETE", manual, nil},
		{"excluir a transferência: apagar", semGravar("DELETE"), "DELETE", id(func(c cenario) int64 { return c.transferencia }), nil},
		{"excluir: o acerto", semConfiguracao, "DELETE", manual, nil},
		{"emergência: ler", semTransacoes, "PUT", fixo("/api/transacoes/emergencia-em-lote"), marcar},
		{"emergência: gravar", semGravar("UPDATE"), "PUT", fixo("/api/transacoes/emergencia-em-lote"), marcar},
		{"revisada: gravar", semGravar("UPDATE"), "PUT", fixo("/api/transacoes/revisada-em-lote"), ids},
		{"categoria em lote: a categoria", fecharBanco, "PUT", fixo("/api/transacoes/categoria-em-lote"), idsComCategoria},
		{"categoria em lote: as regras", semRegras, "PUT", fixo("/api/transacoes/categoria-em-lote"), ids},
		{"categoria em lote: gravar", semGravar("UPDATE"), "PUT", fixo("/api/transacoes/categoria-em-lote"), idsComCategoria},
		{"categoria em lote: o acerto", semConfiguracao, "PUT", fixo("/api/transacoes/categoria-em-lote"), idsComCategoria},
		{"transferir: as contas", fecharBanco, "POST", fixo("/api/transacoes/transferencia"), transferir},
		{"transferir: a categoria", semCategorias, "POST", fixo("/api/transacoes/transferencia"), transferir},
		{"transferir: lançar", semGravar("INSERT"), "POST", fixo("/api/transacoes/transferencia"), transferir},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			mux, db, c := montar(t)
			caso.sabotar(t, db)
			corpo := ""
			if caso.body != nil {
				corpo = caso.body(c)
			}
			testeapoio.Chamar(t, mux, caso.metodo, caso.caminho(c), corpo).ConferirStatus(t, http.StatusInternalServerError)
		})
	}
}
