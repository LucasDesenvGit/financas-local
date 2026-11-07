package sincronizacao

import (
	"context"
	"log/slog"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/configuracao"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tarefas"
)

// Agenda do SincronizacaoHostedService: 2 minutos depois de subir, ciclo de 5 minutos, nova tentativa só 1 hora depois da
// última, e o intervalo entre as sincronizações da Configuracao, de 1 a 48 horas.
const (
	AtrasoDaTarefa          = 2 * time.Minute
	IntervaloDaTarefa       = 5 * time.Minute
	esperaDepoisDaTentativa = time.Hour
	horasPadrao             = 6
	menorIntervaloEmHoras   = 1
	maiorIntervaloEmHoras   = 48
)

// Tarefa é o SincronizacaoHostedService. Roda com o mesmo Servico das rotas, para as duas dividirem a trava de uma rodada
// por vez.
type Tarefa struct {
	servico         *Servico
	log             *slog.Logger
	ultimaTentativa *time.Time // na memória, como o campo _ultimaTentativa do .NET
}

// NovaTarefa cria a tarefa.
func NovaTarefa(s *Servico, log *slog.Logger) *Tarefa {
	return &Tarefa{servico: s, log: log}
}

// Rodar roda a tarefa até o ctx acabar (chame numa goroutine).
func (t *Tarefa) Rodar(ctx context.Context) {
	tarefas.Periodica(ctx, t.log, "a sincronização", AtrasoDaTarefa, IntervaloDaTarefa, t.Ciclo)
}

// Ciclo é o VerificarEExecutarAsync: sincroniza tudo se está ligada, tem conexão ativa, o intervalo já passou e a última
// tentativa já tem 1 hora.
func (t *Tarefa) Ciclo(ctx context.Context) error {
	s := t.servico
	if s.EmExecucao() {
		return nil
	}
	ligada, err := s.config.ObterBool(ctx, chaveHabilitada, true)
	if err != nil {
		return err
	}
	if !ligada {
		return nil
	}
	var temConexao bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM "ConexaoBancaria" WHERE "Ativa")`).Scan(&temConexao); err != nil {
		return err
	}
	if !temConexao {
		return nil
	}
	horas, err := s.config.ObterInteiro(ctx, chaveIntervaloHoras, horasPadrao)
	if err != nil {
		return err
	}
	intervalo := time.Duration(min(max(horas, menorIntervaloEmHoras), maiorIntervaloEmHoras)) * time.Hour
	ultima, err := s.config.ObterData(ctx, configuracao.UltimaSincronizacaoGeral)
	if err != nil {
		return err
	}
	agora := s.relogio.Agora()
	if ultima != nil && agora.Sub(ultima.ParaUtc().Tempo()) < intervalo {
		return nil
	}
	// Evita repetir a tentativa a cada ciclo quando a sincronização está falhando (ex.: credenciais inválidas).
	if t.ultimaTentativa != nil && agora.Sub(*t.ultimaTentativa) < esperaDepoisDaTentativa {
		return nil
	}
	t.ultimaTentativa = &agora

	resultado, err := s.Sincronizar(ctx, nil, OrigemAutomatica)
	if err != nil {
		return err
	}
	t.log.Info("Sincronização automática.", "origem", "Open Finance", "mensagem", resultado.Mensagem,
		"novas", resultado.TransacoesNovas, "atualizadas", resultado.TransacoesAtualizadas)
	return nil
}
