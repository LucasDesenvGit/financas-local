package banco

import (
	"regexp"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/banco/esquema"
)

// O gabarito do .NET (testdata/entradas-esquema) guarda o 2-criar-banco.sh de quando a sonda rodou, com as colunas novas
// em linhas adicionar_coluna_se_ausente. Só o teste o lê (a lista de agora é esquema.ColunasNovas, em Go): este é o
// único lugar que ainda entende essas linhas.
var linhaColuna = regexp.MustCompile(`(?m)^[ \t]*adicionar_coluna_se_ausente[ \t]+"([A-Za-z0-9_]+)"[ \t]+"([A-Za-z0-9_]+)"[ \t]+"([^"]+)"`)

func colunasDoGabarito(script string) []esquema.ColunaNova {
	var colunas []esquema.ColunaNova
	for _, m := range linhaColuna.FindAllStringSubmatch(script, -1) {
		colunas = append(colunas, esquema.ColunaNova{Tabela: m[1], Coluna: m[2], Definicao: m[3]})
	}
	return colunas
}
