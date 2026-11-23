// Pacote esquema guarda o esquema do banco do lado do Go: de onde vêm as cópias dos scripts embutidas no executável
// (subpacote embutido), a leitura das linhas adicionar_coluna_se_ausente e a assinatura que vai no
// PRAGMA user_version, as duas iguais às do EsquemaBanco.cs. Quem aplica o esquema é o banco.AplicarEsquema.
package esquema

import "regexp"

// Arquivos de origem, a partir de src/backend.
const (
	ArquivoEstrutura     = "../../banco/01-estrutura.sql"
	ArquivoDadosIniciais = "../../banco/02-dados-iniciais.sql"
	ArquivoColunas       = "../../scripts/geral/2-criar-banco.sh"
	PastaEmbutido        = "internal/plataforma/banco/esquema/embutido"
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
