package banco

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/esquema"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/esquema/embutido"
)

// #367: toda coluna das linhas adicionar_coluna_se_ausente do 2-criar-banco.sh também está no CREATE TABLE do
// 01-estrutura.sql, com a mesma definição (docs/BANCO-DE-DADOS.md). A linha do .sh atualiza o banco antigo; o
// CREATE TABLE cria o banco novo. Os dois têm de chegar às mesmas colunas: só a ordem delas pode mudar.

func TestColunasNovasTambemNoCreateTable(t *testing.T) {
	ctx := context.Background()
	db, err := Abrir(ctx, filepath.Join(t.TempDir(), "so-a-estrutura.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, string(embutido.Estrutura)); err != nil {
		t.Fatalf("aplicar a estrutura: %v", err)
	}
	colunas := esquema.LerColunasNovas(string(embutido.CriarBanco))
	if len(colunas) == 0 {
		t.Fatal("nenhuma linha adicionar_coluna_se_ausente no 2-criar-banco.sh embutido")
	}
	for _, c := range colunas {
		var n int
		err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?", c.Tabela, c.Coluna).Scan(&n)
		if err != nil {
			t.Fatalf("conferir %s.%s: %v", c.Tabela, c.Coluna, err)
		}
		if n == 0 {
			t.Errorf("%s.%s só existe na linha adicionar_coluna_se_ausente: ponha também no CREATE TABLE do 01-estrutura.sql",
				c.Tabela, c.Coluna)
		}
	}
}

func TestBancoAntigoAtualizadoFicaIgualAoNovo(t *testing.T) {
	ctx := context.Background()
	pasta := t.TempDir()
	novo := filepath.Join(pasta, "novo", "dados", "sql", "FinancasLocal.db")
	if err := AplicarEsquema(ctx, novo, instanteDosTestes, semLog()); err != nil {
		t.Fatal(err)
	}

	// Banco antigo: o de hoje sem nenhuma das colunas que as linhas do .sh acrescentam, e com outra assinatura.
	antigo := filepath.Join(pasta, "antigo", "dados", "sql", "FinancasLocal.db")
	copiarBanco(t, novo, antigo)
	var remover strings.Builder
	for _, c := range esquema.LerColunasNovas(string(embutido.CriarBanco)) {
		fmt.Fprintf(&remover, "ALTER TABLE %s DROP COLUMN %s;\n", c.Tabela, c.Coluna)
	}
	executar(t, antigo, remover.String()+"PRAGMA user_version = 7;")
	if err := AplicarEsquema(ctx, antigo, instanteDosTestes, semLog()); err != nil {
		t.Fatal(err)
	}

	esperado, obtido := colunasPorTabela(t, novo), colunasPorTabela(t, antigo)
	for tabela, want := range esperado {
		if got := obtido[tabela]; !slices.Equal(got, want) {
			t.Errorf("%s: só no banco antigo atualizado (linha do .sh): %q; só no banco novo (CREATE TABLE): %q",
				tabela, faltaEm(got, want), faltaEm(want, got))
		}
	}
	for tabela := range obtido {
		if _, ok := esperado[tabela]; !ok {
			t.Errorf("tabela %s só no banco antigo atualizado", tabela)
		}
	}
}

// faltaEm devolve o que está em a e não está em b.
func faltaEm(a, b []string) []string {
	var falta []string
	for _, x := range a {
		if !slices.Contains(b, x) {
			falta = append(falta, x)
		}
	}
	return falta
}

// colunasPorTabela devolve, para cada tabela, as colunas como o SQLite as vê (nome, tipo, NOT NULL, padrão e chave
// primária), em ordem de nome: a posição da coluna fica de fora de propósito.
func colunasPorTabela(t *testing.T, caminho string) map[string][]string {
	t.Helper()
	db, err := Abrir(context.Background(), caminho)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	linhas, err := db.Query(`SELECT m.name, c.name, c.type, c."notnull", c.dflt_value, c.pk
		FROM sqlite_master m, pragma_table_info(m.name) c
		WHERE m.type = 'table' AND m.name NOT LIKE 'sqlite_%'`)
	if err != nil {
		t.Fatal(err)
	}
	defer linhas.Close()
	tabelas := map[string][]string{}
	for linhas.Next() {
		var tabela, coluna, tipo string
		var obrigatoria, chave int
		var padrao sql.NullString
		if err := linhas.Scan(&tabela, &coluna, &tipo, &obrigatoria, &padrao, &chave); err != nil {
			t.Fatal(err)
		}
		descricao := fmt.Sprintf("%s %s notnull=%d pk=%d", coluna, tipo, obrigatoria, chave)
		if padrao.Valid {
			descricao += " default=" + padrao.String
		}
		tabelas[tabela] = append(tabelas[tabela], descricao)
	}
	if err := linhas.Err(); err != nil {
		t.Fatal(err)
	}
	for _, colunas := range tabelas {
		slices.Sort(colunas)
	}
	return tabelas
}
