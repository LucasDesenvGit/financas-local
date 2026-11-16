// Pacote versao guarda a versão do Finanças Local (a calculada por cmd/versao, ADR 0070), gravada na compilação:
//
//	go build -ldflags "-X github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/versao.Atual=2.38.0" ./cmd/financas
//
// Os scripts de instalação e o gerar-msi.ps1 fazem isso (ADRs 0070 e 0049). Sem o -ldflags, fica "dev".
package versao

// Atual é a versão deste executável.
var Atual = "dev"
