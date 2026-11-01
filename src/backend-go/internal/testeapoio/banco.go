// Pacote testeapoio é o apoio comum aos testes do back-end em Go (#369, ADR 0051): banco temporário com o esquema
// real, fábrica de dados inventados, relógio parado no fuso de Brasília e chamada de handler com o usuário logado.
//
// Só arquivos _test.go importam este pacote: ele importa testing e não pode entrar no executável. O teste
// TestSoImportadoPorTestes confere isso. Exemplo de uso no README.md desta pasta.
package testeapoio

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco"
)

// modelo é o banco novo com o esquema aplicado, montado uma vez por execução de testes: aplicar os scripts custa
// ~370 ms com o -race, e um pacote com 60 testes passava a maior parte do tempo nisso (#542).
var modelo struct {
	sync.Once
	dados []byte
	err   error
}

// bancoModelo aplica o esquema num banco numa pasta temporária, junta o WAL no arquivo principal e devolve os bytes
// do arquivo. A pasta é apagada logo em seguida.
func bancoModelo() ([]byte, error) {
	modelo.Do(func() {
		modelo.dados, modelo.err = montarModelo()
	})
	return modelo.dados, modelo.err
}

func montarModelo() ([]byte, error) {
	pasta, err := os.MkdirTemp("", "financas-modelo-*")
	if err != nil {
		return nil, fmt.Errorf("criar a pasta do banco-modelo: %w", err)
	}
	defer os.RemoveAll(pasta)
	ctx := context.Background()
	caminho := filepath.Join(pasta, "FinancasLocal.db")
	if err := banco.AplicarEsquema(ctx, caminho, RelogioDosTestes(), slog.New(slog.DiscardHandler)); err != nil {
		return nil, fmt.Errorf("aplicar o esquema no banco-modelo: %w", err)
	}
	db, err := banco.Abrir(ctx, caminho)
	if err != nil {
		return nil, fmt.Errorf("abrir o banco-modelo: %w", err)
	}
	// Tudo o que está no WAL vai para o arquivo principal, que é o único copiado.
	if _, err := db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		db.Close()
		return nil, fmt.Errorf("juntar o WAL do banco-modelo: %w", err)
	}
	if err := db.Close(); err != nil {
		return nil, fmt.Errorf("fechar o banco-modelo: %w", err)
	}
	return os.ReadFile(caminho)
}

// NovoBanco cria um banco SQLite num arquivo do t.TempDir(), com o esquema real: os scripts de banco/ embutidos,
// aplicados pelo banco.AplicarEsquema, como num banco novo do usuário. Por isso ele já vem com os dados iniciais
// (categorias padrão, regras das corretoras, configurações); para começar sem eles, use NovoBancoVazio.
//
// O esquema é aplicado uma vez por execução, num banco-modelo, e cada chamada recebe uma cópia do arquivo (#542): o
// resultado é o mesmo de aplicar de novo, porque o relógio dos testes é fixo.
//
// O banco fecha sozinho no fim do teste (t.Cleanup), antes de a pasta temporária ser apagada. Cada chamada cria um
// arquivo próprio, então testes com t.Parallel() não dividem banco.
func NovoBanco(t testing.TB) *sql.DB {
	t.Helper()
	dados, err := bancoModelo()
	if err != nil {
		t.Fatalf("montar o banco-modelo: %v", err)
	}
	caminho := filepath.Join(t.TempDir(), "FinancasLocal.db")
	if err := os.WriteFile(caminho, dados, 0o600); err != nil {
		t.Fatalf("copiar o banco-modelo: %v", err)
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
