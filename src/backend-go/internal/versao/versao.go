// Pacote versao guarda a versão do Finanças Local (a do Directory.Build.props), gravada na compilação:
//
//	go build -ldflags "-X github.com/LucasDesenvGit/financas-local/src/backend-go/internal/versao.Atual=2.38.0" ./cmd/financas
//
// Os scripts de instalação e o gerar-msi.ps1 fazem isso (ADRs 0039 e 0049). Sem o -ldflags, fica "dev".
package versao

// Atual é a versão deste executável.
var Atual = "dev"
