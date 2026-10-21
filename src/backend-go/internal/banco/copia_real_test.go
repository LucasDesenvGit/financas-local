package banco

import (
	"context"
	"os"
	"regexp"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// TestLerCopiaDoBancoReal lê todas as tabelas de uma CÓPIA do banco real e confere que cada valor que parece
// decimal, data, data e hora ou GUID é lido pelos tipos do Go. Teste manual (#74): só roda com a variável
// FINANCAS_COPIA_BANCO apontando para a cópia, e não mostra nenhum valor, só os nomes de tabela e coluna.
//
//	cp dados/sql/FinancasLocal.db /tmp/copia.db
//	FINANCAS_COPIA_BANCO=/tmp/copia.db go test ./internal/banco -run CopiaDoBancoReal -v
func TestLerCopiaDoBancoReal(t *testing.T) {
	caminho := os.Getenv("FINANCAS_COPIA_BANCO")
	if caminho == "" {
		t.Skip("defina FINANCAS_COPIA_BANCO com o caminho de uma cópia do banco")
	}
	ctx := context.Background()
	db, err := Abrir(ctx, caminho)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var journalAntes string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalAntes); err != nil {
		t.Fatal(err)
	}

	pareceDecimal := regexp.MustCompile(`^-?\d+\.\d+$`)
	pareceData := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	pareceDataHora := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}[ T]\d{2}:`)
	pareceGuid := regexp.MustCompile(`^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-`)

	tabelas, err := db.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		t.Fatal(err)
	}
	var nomes []string
	for tabelas.Next() {
		var nome string
		_ = tabelas.Scan(&nome)
		nomes = append(nomes, nome)
	}
	tabelas.Close()

	lidos := map[string]int{}
	for _, tabela := range nomes {
		linhas, err := db.QueryContext(ctx, `SELECT * FROM "`+tabela+`"`)
		if err != nil {
			t.Fatal(err)
		}
		colunas, _ := linhas.Columns()
		valores := make([]any, len(colunas))
		ponteiros := make([]any, len(colunas))
		for i := range valores {
			ponteiros[i] = &valores[i]
		}
		for linhas.Next() {
			if err := linhas.Scan(ponteiros...); err != nil {
				t.Fatal(err)
			}
			for i, v := range valores {
				if _, real := v.(float64); real {
					t.Errorf("%s.%s tem valor REAL (float): dinheiro nunca é gravado assim", tabela, colunas[i])
				}
				texto, ok := v.(string)
				if !ok {
					continue
				}
				var erro error
				switch {
				case pareceDecimal.MatchString(texto):
					_, erro = tipos.NovoDecimal(texto)
					lidos["decimal"]++
				case pareceData.MatchString(texto):
					_, erro = tipos.LerData(texto)
					lidos["data"]++
				case pareceDataHora.MatchString(texto):
					_, erro = tipos.LerDataHora(texto)
					lidos["data e hora"]++
				case pareceGuid.MatchString(texto):
					_, erro = tipos.LerGuid(texto)
					lidos["guid"]++
				}
				if erro != nil {
					t.Errorf("%s.%s: o Go não leu um valor do banco real (formato inesperado)", tabela, colunas[i])
				}
			}
		}
		linhas.Close()
	}
	t.Logf("%d tabelas; valores lidos por tipo: %v", len(nomes), lidos)

	var journalDepois string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalDepois); err != nil {
		t.Fatal(err)
	}
	if journalDepois != journalAntes {
		t.Errorf("abrir o banco mudou o journal_mode de %s para %s", journalAntes, journalDepois)
	}
}
