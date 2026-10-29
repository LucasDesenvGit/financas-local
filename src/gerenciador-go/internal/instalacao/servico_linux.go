package instalacao

import "github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/servico"

// pastaDoServico lê a unidade do systemd --user (do próprio usuário, sem senha).
func pastaDoServico() string {
	p, err := servico.Mostrar("LoadState", "WorkingDirectory", "ExecStart")
	if err != nil {
		return ""
	}
	return PastaDaUnidade(p)
}
