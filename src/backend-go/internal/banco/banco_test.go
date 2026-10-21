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

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/esquema"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// raiz é src/backend-go, a partir desta pasta.
const raiz = "../.."

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

func TestEsquemaDoSqlcEmDia(t *testing.T) {
	estrutura, err := os.ReadFile(filepath.Join(raiz, esquema.ArquivoEstrutura))
	if err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile(filepath.Join(raiz, esquema.ArquivoColunas))
	if err != nil {
		t.Fatal(err)
	}
	esperado, err := esquema.GerarEsquemaSqlc(context.Background(), string(estrutura), string(script))
	if err != nil {
		t.Fatal(err)
	}
	atual, err := os.ReadFile(filepath.Join(raiz, esquema.ArquivoGerado))
	if err != nil {
		t.Fatal(err)
	}
	if string(atual) != esperado {
		t.Fatal("o esquema do sqlc está velho: o banco/ ou o 2-criar-banco.sh mudou. Rode `go generate ./...` em src/backend-go")
	}
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
			chave, tipos.NovaDataHora(time.Now()))
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

func TestConsultasDaConfiguracaoLeemOFormatoDoEfCore(t *testing.T) {
	ctx := context.Background()
	db := bancoDeTeste(t)
	// Como o .NET grava (conferido pela sonda em internal/tipos/testdata).
	_, err := db.ExecContext(ctx, `INSERT INTO Configuracao (Chave, Valor, Protegido, AtualizadoEm) VALUES
		('Pluggy.ClientSecret', 'CfDJ8...', 1, '2026-10-15 22:55:31.3481471'),
		('Sincronizacao.IntervaloHoras', NULL, 0, '2026-01-01 00:00:00')`)
	if err != nil {
		t.Fatal(err)
	}
	q := consultas.New(db)

	segredo, err := q.ObterConfiguracao(ctx, "Pluggy.ClientSecret")
	if err != nil {
		t.Fatal(err)
	}
	if !segredo.Protegido || segredo.AtualizadoEm.String() != "2026-10-15 22:55:31.3481471" {
		t.Errorf("leu %+v", segredo)
	}

	todas, err := q.ListarConfiguracoes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(todas) != 2 || todas[1].Valor != nil {
		t.Errorf("Valor NULL deveria virar nil: %+v", todas)
	}

	if _, err := q.ObterConfiguracao(ctx, "NaoExiste"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("chave que não existe deveria dar sql.ErrNoRows: %v", err)
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

// O sqlc (v1.31.1, motor sqlite) recorta o SQL no lugar errado quando há caractere fora do ASCII antes da consulta
// (um acento num comentário): a consulta gerada sai quebrada. Texto com acento vai por parâmetro (?), nunca no SQL.
func TestConsultasSemAcento(t *testing.T) {
	arquivos, err := filepath.Glob("consultas/sql/*.sql")
	if err != nil || len(arquivos) == 0 {
		t.Fatalf("nenhum arquivo de consulta: %v", err)
	}
	for _, arquivo := range arquivos {
		dados, err := os.ReadFile(arquivo)
		if err != nil {
			t.Fatal(err)
		}
		for i, b := range dados {
			if b > 127 {
				linha := 1 + strings.Count(string(dados[:i]), "\n")
				t.Errorf("%s:%d: caractere fora do ASCII (acento?); o sqlc gera a consulta errada", arquivo, linha)
				break
			}
		}
	}
}
