package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
)

// Regra (#723): o subcomando cria o banco que não existe e acrescenta as colunas novas a um banco antigo, e rodar de novo
// não muda nada. Sem -banco, recusa.
func TestAplicarEsquemaSubcomando(t *testing.T) {
	ctx := context.Background()
	r := relogio.Fixo{Instante: inicioDoContrato}
	caminho := filepath.Join(t.TempDir(), "dados", "sql", "FinancasLocal.db")

	if err := aplicarEsquema(ctx, nil, r); err == nil || !strings.Contains(err.Error(), "-banco") {
		t.Errorf("sem -banco: erro = %v, esperado pedir o -banco", err)
	}
	if err := aplicarEsquema(ctx, []string{"-banco", caminho}, r); err != nil {
		t.Fatalf("banco novo: %v", err)
	}

	db, err := banco.Abrir(ctx, caminho)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	contar := func(consulta string) int {
		t.Helper()
		var n int
		if err := db.QueryRowContext(ctx, consulta).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if contar("SELECT COUNT(*) FROM Categoria") == 0 {
		t.Error("banco novo sem as categorias iniciais")
	}
	// Banco antigo: falta uma coluna nova. O subcomando a devolve.
	if _, err := db.ExecContext(ctx, "ALTER TABLE Meta DROP COLUMN Icone; PRAGMA user_version = 7;"); err != nil {
		t.Fatal(err)
	}
	if err := aplicarEsquema(ctx, []string{"-banco", caminho}, r); err != nil {
		t.Fatalf("banco antigo: %v", err)
	}
	if contar("SELECT COUNT(*) FROM pragma_table_info('Meta') WHERE name = 'Icone'") != 1 {
		t.Error("a coluna Meta.Icone não voltou")
	}
}
