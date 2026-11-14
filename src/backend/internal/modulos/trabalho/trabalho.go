// Pacote trabalho é o módulo Trabalho (#209, #210; ADRs 0064 e 0065): o dinheiro que vem do emprego (salário pelo
// bruto, INSS e IRRF estimados e, nas próximas histórias, o holerite, as férias, o 13º, a rescisão e o FGTS).
//
// Implementa o contrato modulos.Modulo e junta as rotas dos assuntos dele (internal/modulos/trabalho/<assunto>). Não
// acessa a internet e nasce desligado, como todo módulo (ADR 0064).
package trabalho

import (
	"database/sql"
	"log/slog"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/decimoterceiro"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/emprego"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/ferias"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/holerite"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/modulos"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
)

// ID é o id do módulo: o prefixo /api/trabalho/ e a chave Modulos.trabalho.Ligado.
const ID = "trabalho"

// Modulo é o Trabalho, com os serviços dos assuntos dele.
type Modulo struct {
	emprego  *emprego.Servico
	holerite *holerite.Servico
	decimo   *decimoterceiro.Servico
	ferias   *ferias.Servico
}

// Novo monta o módulo com o banco, o relógio e o log do site.
func Novo(db *sql.DB, r relogio.Relogio, log *slog.Logger) *Modulo {
	empregos := emprego.NovoServico(db, r, log)
	holerites := holerite.NovoServico(db, empregos, r, log)
	return &Modulo{emprego: empregos, holerite: holerites, decimo: decimoterceiro.NovoServico(db, empregos, holerites, r, log),
		ferias: ferias.NovoServico(db, empregos, holerites, r, log)}
}

// Info é o que Configurações › Módulos mostra.
func (m *Modulo) Info() modulos.Info {
	return modulos.Info{
		ID:        ID,
		Nome:      "Trabalho",
		Descricao: "O salário pelo bruto, o holerite de cada mês (horas extras, descontos, INSS e IRRF estimados pelas tabelas oficiais), o 13º e as férias, com o histórico. Não entra nos relatórios: a receita continua sendo a transação do salário.",
	}
}

// Registrar põe as rotas dos assuntos do Trabalho, guardadas pelo núcleo.
func (m *Modulo) Registrar(r *modulos.Rotas) {
	emprego.Registrar(r, m.emprego)
	holerite.Registrar(r, m.holerite)
	decimoterceiro.Registrar(r, m.decimo)
	ferias.Registrar(r, m.ferias)
}

// Tarefas: o Trabalho não tem tarefa em segundo plano.
func (m *Modulo) Tarefas() []modulos.Tarefa { return nil }
