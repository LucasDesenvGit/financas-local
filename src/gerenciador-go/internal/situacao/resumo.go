package situacao

import (
	"fmt"
	"io"

	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/instalacao"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/tela"
)

// Resumo escreve a situação no terminal, como o --situacao do Program.cs do Linux, e devolve o código de saída: 0 com
// o site no ar, 1 em qualquer outra situação. Serve também para a CI conferir o gerenciador sem tela (#277).
func Resumo(w io.Writer, s tela.Situacao, i instalacao.Instalacao, versaoGerenciador string) int {
	fmt.Fprintln(w, s.Titulo)
	fmt.Fprintln(w, s.Detalhe)
	fmt.Fprintf(w, "Pasta da API: %s\n", i.PastaPrograma)
	fmt.Fprintf(w, "Banco: %s\n", i.CaminhoBanco)
	fmt.Fprintf(w, "Gerenciador: %s\n", versaoGerenciador)
	if s.Estado == tela.NoAr {
		return 0
	}
	return 1
}
