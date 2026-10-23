// Pacote embutido põe dentro do executável os scripts do esquema do banco, como os EmbeddedResource do
// Financas.Api.csproj. Os arquivos desta pasta são CÓPIAS geradas pelo go generate ./... a partir do banco/ e do
// scripts/geral/2-criar-banco.sh: não edite aqui (veja esquema.Copias).
//
// Fica num pacote à parte para o gerador (esquema/gerar) não depender dele: o gerador é quem cria estes arquivos.
package embutido

import _ "embed" // o //go:embed precisa do pacote embed importado

var (
	//go:embed 01-estrutura.sql
	Estrutura []byte
	//go:embed 02-dados-iniciais.sql
	DadosIniciais []byte
	//go:embed 2-criar-banco.sh
	CriarBanco []byte
)
