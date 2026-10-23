// Pacote esquema guarda o esquema do banco do lado do Go: de onde vêm as cópias dos scripts embutidas no executável
// (subpacote embutido), a leitura das linhas adicionar_coluna_se_ausente e a assinatura que vai no
// PRAGMA user_version, as duas iguais às do EsquemaBanco.cs. Quem aplica o esquema é o banco.AplicarEsquema.
//
// Também monta o esquema para o sqlc: o banco/01-estrutura.sql mais as colunas que só existem nas linhas
// adicionar_coluna_se_ausente (oito delas, em 21/10/2026, não estão no CREATE TABLE). O sqlc não recebe o DDL
// original: o parser de SQLite dele não entende "TEXT NULL" (a coluna vira interface{}) e passa para minúsculas os
// nomes do CREATE TABLE (CriadoEm vira o campo Criadoem). Por isso o esquema é aplicado num SQLite em memória e
// reescrito como o próprio SQLite o vê: tabela, coluna, tipo e NOT NULL, nomes entre aspas.
package esquema

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	_ "modernc.org/sqlite"
)

// Arquivos de origem, a partir de src/backend-go.
const (
	ArquivoEstrutura     = "../../banco/01-estrutura.sql"
	ArquivoDadosIniciais = "../../banco/02-dados-iniciais.sql"
	ArquivoColunas       = "../../scripts/geral/2-criar-banco.sh"
	ArquivoGerado        = "internal/banco/esquema/esquema-sqlc.sql"
	PastaEmbutido        = "internal/banco/esquema/embutido"
)

// espacoDotnet é o \s do .NET: [\f\n\r\t\v\x85\p{Z}]. O \s do Go só conhece os espaços ASCII e não pega a
// tabulação vertical, o NBSP nem os outros espaços do Unicode.
const espacoDotnet = `[\f\n\r\t\v\x{85}\p{Z}]`

// linhaColuna é a mesma expressão do EsquemaBanco.cs (e do instalar-windows.ps1): três argumentos entre aspas duplas.
// O [^"] pega também a quebra de linha, como no .NET.
var linhaColuna = regexp.MustCompile(`(?m)^` + espacoDotnet + `*adicionar_coluna_se_ausente` + espacoDotnet +
	`+"([A-Za-z0-9_]+)"` + espacoDotnet + `+"([A-Za-z0-9_]+)"` + espacoDotnet + `+"([^"]+)"`)

// ColunaNova é uma linha adicionar_coluna_se_ausente.
type ColunaNova struct {
	Tabela, Coluna, Definicao string
}

// LerColunasNovas tira as colunas do texto do 2-criar-banco.sh, na ordem em que aparecem.
func LerColunasNovas(script string) []ColunaNova {
	var colunas []ColunaNova
	for _, m := range linhaColuna.FindAllStringSubmatch(script, -1) {
		colunas = append(colunas, ColunaNova{Tabela: m[1], Coluna: m[2], Definicao: m[3]})
	}
	return colunas
}

// GerarEsquemaSqlc aplica a estrutura e as colunas novas num banco em memória e devolve o esquema para o sqlc.
func GerarEsquemaSqlc(ctx context.Context, estrutura, script string) (string, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return "", err
	}
	defer db.Close()
	db.SetMaxOpenConns(1) // ":memory:" é um banco por conexão
	if _, err := db.ExecContext(ctx, estrutura); err != nil {
		return "", fmt.Errorf("aplicar a estrutura: %w", err)
	}
	// Coluna nova só onde a estrutura não a cria, como o adicionar_coluna_se_ausente faz.
	for _, c := range LerColunasNovas(script) {
		var existe int
		err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?", c.Tabela, c.Coluna).Scan(&existe)
		if err != nil {
			return "", fmt.Errorf("conferir %s.%s: %w", c.Tabela, c.Coluna, err)
		}
		if existe == 0 {
			if _, err := db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", c.Tabela, c.Coluna, c.Definicao)); err != nil {
				return "", fmt.Errorf("acrescentar %s.%s: %w", c.Tabela, c.Coluna, err)
			}
		}
	}

	tabelas, err := listar(ctx, db, "SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY rowid")
	if err != nil {
		return "", err
	}
	var saida strings.Builder
	saida.WriteString("-- GERADO por `go generate ./...` a partir do banco/01-estrutura.sql e do scripts/geral/2-criar-banco.sh.\n")
	saida.WriteString("-- Não edite: é só para o sqlc (veja internal/banco/esquema/esquema.go). O esquema de verdade continua no banco/.\n")
	for _, tabela := range tabelas {
		linhas, err := db.QueryContext(ctx, "SELECT name, type, \"notnull\" FROM pragma_table_info(?) ORDER BY cid", tabela)
		if err != nil {
			return "", fmt.Errorf("ler as colunas de %s: %w", tabela, err)
		}
		var colunas []string
		for linhas.Next() {
			var nome, tipo string
			var obrigatoria bool
			if err := linhas.Scan(&nome, &tipo, &obrigatoria); err != nil {
				linhas.Close()
				return "", err
			}
			coluna := fmt.Sprintf("    %q %s", nome, tipo)
			if obrigatoria {
				coluna += " NOT NULL"
			}
			colunas = append(colunas, coluna)
		}
		if err := linhas.Close(); err != nil {
			return "", err
		}
		fmt.Fprintf(&saida, "\nCREATE TABLE %q (\n%s\n);\n", tabela, strings.Join(colunas, ",\n"))
	}
	return saida.String(), nil
}

func listar(ctx context.Context, db *sql.DB, consulta string) ([]string, error) {
	linhas, err := db.QueryContext(ctx, consulta)
	if err != nil {
		return nil, err
	}
	defer linhas.Close()
	var nomes []string
	for linhas.Next() {
		var nome string
		if err := linhas.Scan(&nome); err != nil {
			return nil, err
		}
		nomes = append(nomes, nome)
	}
	return nomes, linhas.Err()
}
