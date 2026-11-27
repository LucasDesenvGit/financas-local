package banco

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/banco/esquema"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// raiz é src/backend, a partir desta pasta.
const raiz = "../../.."

// bancoDeTeste cria um SQLite descartável com a estrutura do .NET (01-estrutura.sql e colunas novas).
func bancoDeTeste(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()
	db, err := Abrir(ctx, filepath.Join(t.TempDir(), "teste.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	estrutura, err := os.ReadFile(filepath.Join(raiz, esquema.ArquivoEstrutura))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, string(estrutura)); err != nil {
		t.Fatalf("aplicar a estrutura: %v", err)
	}
	return db
}

func TestAbrirComOsPragmasDoEfCore(t *testing.T) {
	db := bancoDeTeste(t)
	for pragma, esperado := range map[string]int{"foreign_keys": 1, "busy_timeout": EsperaSeOcupado} {
		var valor int
		if err := db.QueryRow("PRAGMA " + pragma).Scan(&valor); err != nil {
			t.Fatal(err)
		}
		if valor != esperado {
			t.Errorf("PRAGMA %s = %d, esperado %d", pragma, valor, esperado)
		}
	}
}

func TestChaveEstrangeiraLigada(t *testing.T) {
	db := bancoDeTeste(t)
	_, err := db.Exec(`INSERT INTO Transacao (ContaId, Data, Descricao, Valor, Tipo, CriadoEm, AtualizadoEm)
		VALUES (999, '2026-10-15', 'conta que não existe', '-10.0', 2, '2026-10-15 12:00:00', '2026-10-15 12:00:00')`)
	if err == nil || !strings.Contains(err.Error(), "FOREIGN KEY") {
		t.Fatalf("transação de uma conta que não existe deveria falhar pela chave estrangeira; erro: %v", err)
	}
}

func TestEmTransacao(t *testing.T) {
	ctx := context.Background()
	db := bancoDeTeste(t)
	gravar := func(tx *sql.Tx, chave string) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO Configuracao (Chave, Valor, Protegido, AtualizadoEm) VALUES (?, 'x', 0, ?)`,
			chave, tipos.NovaDataHora(instanteDosTestes.Agora()))
		return err
	}
	existe := func(chave string) bool {
		var n int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM Configuracao WHERE Chave = ?`, chave).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n == 1
	}

	if err := EmTransacao(ctx, db, func(tx *sql.Tx) error { return gravar(tx, "Confirmada") }); err != nil {
		t.Fatal(err)
	}
	if !existe("Confirmada") {
		t.Error("a transação sem erro deveria ter sido confirmada")
	}

	errEsperado := errors.New("deu errado")
	err := EmTransacao(ctx, db, func(tx *sql.Tx) error {
		if err := gravar(tx, "Desfeita"); err != nil {
			return err
		}
		return errEsperado
	})
	if !errors.Is(err, errEsperado) {
		t.Errorf("o erro da função deveria voltar: %v", err)
	}
	if existe("Desfeita") {
		t.Error("a transação com erro deveria ter sido desfeita")
	}

	func() {
		defer func() {
			if recover() == nil {
				t.Error("o pânico deveria continuar depois do desfazer")
			}
		}()
		_ = EmTransacao(ctx, db, func(tx *sql.Tx) error {
			_ = gravar(tx, "ComPanico")
			panic("pânico de teste")
		})
	}()
	if existe("ComPanico") {
		t.Error("a transação com pânico deveria ter sido desfeita")
	}
}

func TestGoGravaNoFormatoDoEfCore(t *testing.T) {
	ctx := context.Background()
	db := bancoDeTeste(t)
	momento := tipos.NovaDataHora(time.Date(2026, 10, 15, 22, 55, 31, 348147100, time.UTC))
	if _, err := db.ExecContext(ctx, `INSERT INTO Configuracao (Chave, Valor, Protegido, AtualizadoEm) VALUES ('A', NULL, ?, ?)`,
		true, momento); err != nil {
		t.Fatal(err)
	}
	var protegido, atualizadoEm, tipoProtegido string
	if err := db.QueryRowContext(ctx, `SELECT Protegido, typeof(Protegido), AtualizadoEm FROM Configuracao WHERE Chave = 'A'`).
		Scan(&protegido, &tipoProtegido, &atualizadoEm); err != nil {
		t.Fatal(err)
	}
	if protegido != "1" || tipoProtegido != "integer" || atualizadoEm != "2026-10-15 22:55:31.3481471" {
		t.Errorf("gravou Protegido=%s (%s), AtualizadoEm=%s", protegido, tipoProtegido, atualizadoEm)
	}

	// Decimal e decimal opcional (nil vira NULL).
	if _, err := db.ExecContext(ctx, `CREATE TABLE Valores (Valor TEXT NOT NULL, Opcional TEXT NULL)`); err != nil {
		t.Fatal(err)
	}
	valor, _ := tipos.NovoDecimal("1.50")
	var opcional *tipos.Decimal
	if _, err := db.ExecContext(ctx, `INSERT INTO Valores VALUES (?, ?)`, valor, opcional); err != nil {
		t.Fatal(err)
	}
	var texto string
	var lidoOpcional *tipos.Decimal
	if err := db.QueryRowContext(ctx, `SELECT Valor, Opcional FROM Valores`).Scan(&texto, &lidoOpcional); err != nil {
		t.Fatal(err)
	}
	if texto != "1.5" || lidoOpcional != nil {
		t.Errorf("gravou Valor=%q, Opcional=%v; o EF Core grava \"1.5\" e NULL", texto, lidoOpcional)
	}
}
