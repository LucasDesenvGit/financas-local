// Pacote testeapoio é o apoio comum aos testes do back-end em Go (#369, ADR 0051): banco temporário com o esquema
// real, fábrica de dados inventados, relógio parado no fuso de Brasília e chamada de handler com o usuário logado.
//
// Só arquivos _test.go importam este pacote: ele importa testing e não pode entrar no executável. O teste
// TestSoImportadoPorTestes confere isso. Exemplo de uso no README.md desta pasta.
package testeapoio

import (
	"database/sql"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco"
)

// NovoBanco cria um banco SQLite num arquivo do t.TempDir(), com o esquema real: os scripts de banco/ embutidos,
// aplicados pelo banco.AplicarEsquema, como num banco novo do usuário. Por isso ele já vem com os dados iniciais
// (categorias padrão, regras das corretoras, configurações); para começar sem eles, use NovoBancoVazio.
//
// O banco fecha sozinho no fim do teste (t.Cleanup), antes de a pasta temporária ser apagada. Cada chamada cria um
// arquivo próprio, então testes com t.Parallel() não dividem banco.
func NovoBanco(t testing.TB) *sql.DB {
	t.Helper()
	caminho := filepath.Join(t.TempDir(), "FinancasLocal.db")
	if err := banco.AplicarEsquema(t.Context(), caminho, RelogioDosTestes(), slog.New(slog.DiscardHandler)); err != nil {
		t.Fatalf("aplicar o esquema no banco temporário: %v", err)
	}
	db, err := banco.Abrir(t.Context(), caminho)
	if err != nil {
		t.Fatalf("abrir o banco temporário: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("fechar o banco temporário: %v", err)
		}
	})
	return db
}

// tabelasDosDadosIniciais são as que o 02-dados-iniciais.sql preenche e que os testes de assunto querem controlar.
// A ordem respeita as chaves estrangeiras (a regra aponta para a categoria). Configuracao fica: é a configuração
// padrão do sistema, não dado do usuário.
var tabelasDosDadosIniciais = []string{"RegraCategorizacao", "Categoria"}

// NovoBancoVazio é o NovoBanco sem as categorias e regras padrão, e com o AUTOINCREMENT delas zerado: o primeiro
// Id gravado pela fábrica é 1. Use quando o teste confere uma lista inteira ou os Ids no JSON.
func NovoBancoVazio(t testing.TB) *sql.DB {
	t.Helper()
	db := NovoBanco(t)
	for _, tabela := range tabelasDosDadosIniciais {
		// tabela vem da lista fixa acima, nunca de fora.
		if _, err := db.ExecContext(t.Context(), "DELETE FROM "+tabela); err != nil {
			t.Fatalf("esvaziar %s: %v", tabela, err)
		}
		if _, err := db.ExecContext(t.Context(), "DELETE FROM sqlite_sequence WHERE name = ?", tabela); err != nil {
			t.Fatalf("zerar o AUTOINCREMENT de %s: %v", tabela, err)
		}
	}
	return db
}
