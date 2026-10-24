// Pacote tarefas roda as tarefas em segundo plano (#81): o BackgroundService do .NET com PeriodicTimer.
//
// Quem chama inicia a goroutine, quem cancela é o context (o serviço parou, ou o .NET fechou a entrada padrão) e
// quem espera é o sync.WaitGroup de quem iniciou. Uma tarefa só roda no Go quando o nome dela está em Go:Tarefas
// (ADR 0049): aí o .NET não roda a dele, e nunca as duas ao mesmo tempo.
package tarefas

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"
	"time"
)

// Periodica roda ciclo depois de atraso e, dali em diante, a cada intervalo, até o ctx acabar. É o
// "Task.Delay(atraso) + do { ... } while (await timer.WaitForNextTickAsync())" das tarefas do .NET:
//   - o ticker começa depois do primeiro ciclo esperar o atraso, e um ciclo demorado não acumula voltas (o
//     time.Ticker descarta as que perdeu, como o PeriodicTimer);
//   - erro do ciclo vira log de erro, e pânico também (com a pilha): a tarefa segue no próximo ciclo e o processo
//     não cai. O recover fica aqui, em volta de cada ciclo, e não no processo inteiro: um pânico em outro lugar
//     continua derrubando o Go, que é o certo para um bug fora das tarefas;
//   - cancelamento não é erro: a função só volta quando o ctx acaba.
func Periodica(ctx context.Context, log *slog.Logger, nome string, atraso, intervalo time.Duration, ciclo func(context.Context) error) {
	espera := time.NewTimer(atraso)
	select {
	case <-ctx.Done():
		espera.Stop()
		return
	case <-espera.C:
	}

	ticker := time.NewTicker(intervalo)
	defer ticker.Stop()
	for {
		rodarCiclo(ctx, log, nome, ciclo)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// rodarCiclo roda um ciclo protegido: erro e pânico viram log.
func rodarCiclo(ctx context.Context, log *slog.Logger, nome string, ciclo func(context.Context) error) {
	defer func() {
		if r := recover(); r != nil {
			log.Error("Falha na atualização automática d"+nome+".", "origem", origem(nome),
				"erro", fmt.Sprint(r), "pilha", string(debug.Stack()))
		}
	}()
	if err := ciclo(ctx); err != nil && ctx.Err() == nil {
		log.Error("Falha na atualização automática d"+nome+".", "origem", origem(nome), "erro", err)
	}
}

// origem é a origem do erro em Configurações › Erros ("Câmbio", como no .NET; cripto também cai em Câmbio).
func origem(nome string) string {
	if strings.Contains(nome, "câmbio") || strings.Contains(nome, "cripto") {
		return "Câmbio"
	}
	return "Sistema"
}

// Conhecidas são as tarefas que um assunto em Go pode assumir, os nomes de Go:Tarefas (OpcoesGo.TarefasConhecidas).
var Conhecidas = []string{"Sincronizacao", "Cambio", "Cripto"}

// Ligadas lê Go:Tarefas ("Cambio;Cripto", sem diferenciar maiúsculas) e devolve as que o Go deve rodar. Nome
// desconhecido é erro, como no .NET (OpcoesGo.Ler): um nome errado deixaria a tarefa sem ninguém para rodar.
func Ligadas(texto string) (map[string]bool, error) {
	ligadas := map[string]bool{}
	for _, parte := range strings.Split(texto, ";") {
		parte = strings.TrimSpace(parte)
		if parte == "" {
			continue
		}
		nome := ""
		for _, c := range Conhecidas {
			if strings.EqualFold(c, parte) {
				nome = c
			}
		}
		if nome == "" {
			return nil, fmt.Errorf("Go:Tarefas tem %q: as conhecidas são %s", parte, strings.Join(Conhecidas, ", "))
		}
		ligadas[nome] = true
	}
	return ligadas, nil
}
