package situacao

import (
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/instalacao"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/tela"
)

// Regra: o --situacao escreve as linhas do Program.cs do Linux e só sai com 0 com o site no ar (#277).
func TestResumo(t *testing.T) {
	inst := instalacao.Instalacao{PastaPrograma: "/opt/financas/publicado/api", CaminhoBanco: "/opt/financas/dados/sql/FinancasLocal.db"}
	casos := []struct {
		nome   string
		estado tela.Estado
		codigo int
	}{
		{"no ar", tela.NoAr, 0},
		{"desligado", tela.Desligado, 1},
		{"ligando", tela.Mudando, 1},
		{"sem resposta", tela.SemResposta, 1},
		{"não instalado", tela.NaoInstalado, 1},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			var saida strings.Builder
			s := tela.Situacao{Estado: c.estado, Titulo: "Título de " + c.nome, Detalhe: "Detalhe de " + c.nome}
			if codigo := Resumo(&saida, s, inst, "2.65.0"); codigo != c.codigo {
				t.Errorf("código = %d, quero %d", codigo, c.codigo)
			}
			quero := "Título de " + c.nome + "\nDetalhe de " + c.nome + "\n" +
				"Pasta da API: /opt/financas/publicado/api\nBanco: /opt/financas/dados/sql/FinancasLocal.db\nGerenciador: 2.65.0\n"
			if saida.String() != quero {
				t.Errorf("saída:\n%s\nquero:\n%s", saida.String(), quero)
			}
		})
	}
}
