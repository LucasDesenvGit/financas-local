package cambio

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tarefas"
)

// Agenda do CambioHostedService (.cs:26, 33, 71, 10): 1 minuto depois de subir, ciclo de 15 minutos, nova tentativa
// depois de uma falha só 30 minutos depois, e o boletim de fechamento da PTAX disponível às 13h30 de Brasília.
const (
	AtrasoDaTarefa          = time.Minute
	IntervaloDaTarefa       = 15 * time.Minute
	esperaDepoisDaTentativa = 30 * time.Minute
	horaDoFechamento        = 13
	minutoDoFechamento      = 30
)

// Tarefa é o CambioHostedService: mantém as cotações guardadas para uso sem internet. Atualiza uma vez por dia e de
// novo depois do boletim de fechamento (publicado por volta das 13h de Brasília).
type Tarefa struct {
	servico *Servico
	relogio relogio.Relogio
	log     *slog.Logger

	mu              sync.Mutex
	ultimaTentativa *time.Time // na memória, como o campo _ultimaTentativa do .NET
}

// NovaTarefa cria a tarefa.
func NovaTarefa(s *Servico, r relogio.Relogio, log *slog.Logger) *Tarefa {
	return &Tarefa{servico: s, relogio: r, log: log}
}

// Rodar roda a tarefa até o ctx acabar (chame numa goroutine).
func (t *Tarefa) Rodar(ctx context.Context) {
	tarefas.Periodica(ctx, t.log, "o câmbio", AtrasoDaTarefa, IntervaloDaTarefa, t.Ciclo)
}

// Ciclo é o VerificarEExecutarAsync: atualiza se a última atualização venceu e a última tentativa já tem 30 minutos.
func (t *Tarefa) Ciclo(ctx context.Context) error {
	ultima, err := t.servico.config.ObterData(ctx, ChaveUltimaAtualizacao)
	if err != nil {
		return err
	}
	var ultimaUtc *time.Time
	if ultima != nil {
		u := ultima.Tempo()
		ultimaUtc = &u
	}
	agora := t.relogio.Agora()
	if !Vencida(ultimaUtc, agora) {
		return nil
	}
	// Sem internet ou API fora do ar: não tenta de novo a cada ciclo.
	t.mu.Lock()
	if t.ultimaTentativa != nil && agora.Sub(*t.ultimaTentativa) < esperaDepoisDaTentativa {
		t.mu.Unlock()
		return nil
	}
	t.ultimaTentativa = &agora
	t.mu.Unlock()

	resultado, err := t.servico.AtualizarTodas(ctx)
	if err != nil {
		return err
	}
	if len(resultado.Falhas) > 0 {
		t.log.Warn("Câmbio atualizado parcialmente.", "origem", "Câmbio", "novas", resultado.Novas,
			"corrigidas", resultado.Corrigidas, "falharam", strings.Join(resultado.Falhas, ", "))
	} else {
		t.log.Info("Câmbio atualizado.", "origem", "Câmbio", "novas", resultado.Novas, "corrigidas", resultado.Corrigidas)
	}
	return nil
}

// Vencida é o Vencida do .NET: a atualização venceu se nunca rodou, se não rodou hoje (em Brasília), ou se já
// passou das 13h30 de hoje e a última foi antes disso.
func Vencida(ultimaUtc *time.Time, agoraUtc time.Time) bool {
	if ultimaUtc == nil {
		return true
	}
	ultima := relogio.ParaHoraLocal(*ultimaUtc)
	agora := relogio.ParaHoraLocal(agoraUtc)
	diaUltima := time.Date(ultima.Year(), ultima.Month(), ultima.Day(), 0, 0, 0, 0, time.UTC)
	diaAgora := time.Date(agora.Year(), agora.Month(), agora.Day(), 0, 0, 0, 0, time.UTC)
	if diaUltima.Before(diaAgora) {
		return true
	}
	fechamento := time.Date(agora.Year(), agora.Month(), agora.Day(), horaDoFechamento, minutoDoFechamento, 0, 0, agora.Location())
	return !agora.Before(fechamento) && ultima.Before(fechamento)
}
