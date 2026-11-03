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

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/erros"
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
			log.Error(mensagemDeFalha(nome), "origem", origem(nome), "erro", fmt.Sprint(r), "pilha", string(debug.Stack()))
		}
	}()
	if err := ciclo(ctx); err != nil && ctx.Err() == nil {
		log.Error(mensagemDeFalha(nome), "origem", origem(nome), "erro", err)
	}
}

// mensagemDeFalha é a do .NET: a tarefa da sincronização só verifica se é hora, e a falha fala disso.
func mensagemDeFalha(nome string) string {
	if strings.Contains(nome, "sincronização") {
		return "Falha na verificação da sincronização automática."
	}
	return "Falha na atualização automática d" + nome + "."
}

// origem é a origem do erro em Configurações › Erros, a mesma do .NET (RegistroErrosLoggerProvider.Origem): o câmbio
// em "Câmbio", a sincronização em "Open Finance" e, desde a #298, a cripto em "Cripto".
func origem(nome string) string {
	switch {
	case strings.Contains(nome, "cripto"):
		return erros.OrigemCripto
	case strings.Contains(nome, "câmbio"):
		return "Câmbio"
	case strings.Contains(nome, "sincronização"):
		return "Open Finance"
	}
	return erros.OrigemSistema
}

// Conhecidas são as tarefas em segundo plano do site.
var Conhecidas = []string{"Sincronizacao", "Cambio", "Cripto"}

// Todas são as tarefas que o site roda (#85: o Go roda todas; até a #86, o Go:Tarefas dizia quais o .NET deixava
// para o Go).
func Todas() map[string]bool {
	todas := map[string]bool{}
	for _, c := range Conhecidas {
		todas[c] = true
	}
	return todas
}
