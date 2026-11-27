// Pacote esquema guarda o esquema do banco do lado do Go: de onde vêm as cópias dos scripts embutidas no executável
// (subpacote embutido), a lista de colunas novas (ColunasNovas) e a assinatura que vai no PRAGMA user_version, igual à
// do EsquemaBanco.cs. Quem aplica o esquema é o banco.AplicarEsquema.
package esquema

// Arquivos de origem, a partir de src/backend.
const (
	ArquivoEstrutura       = "../../banco/01-estrutura.sql"
	ArquivoDadosIniciais   = "../../banco/02-dados-iniciais.sql"
	ArquivoTabelasOficiais = "../../banco/03-tabelas-oficiais.sql"
	PastaEmbutido          = "internal/plataforma/banco/esquema/embutido"
)

// ColunaNova é uma coluna acrescentada a uma tabela que já existia.
type ColunaNova struct {
	Tabela, Coluna, Definicao string
}
