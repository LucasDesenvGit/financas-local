package servicosistema

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
)

// prazoParaParar é o que o gerenciador de serviços espera depois do Stop: um pouco mais que os 30 s de servidor.Rodar para
// as requisições em andamento.
const prazoParaParar = 40 * time.Second

// codigoDeFalha é o código de saída do serviço que parou com erro: o gerenciador de serviços o registra como falha.
const codigoDeFalha = 1

// Rodar conversa com o gerenciador de serviços quando foi ele quem iniciou o processo. No terminal, para com Ctrl+C.
func Rodar(trabalho Trabalho) error {
	emServico, err := svc.IsWindowsService()
	if err != nil {
		return fmt.Errorf("ver se é um serviço: %w", err)
	}
	if !emServico {
		return comSinais(trabalho)
	}
	m := &manipulador{trabalho: trabalho}
	if err := svc.Run(Nome, m); err != nil {
		return fmt.Errorf("rodar como serviço: %w", err)
	}
	return m.erro
}

// ComLogDoSistema acrescenta o Log de Eventos quando o processo é um serviço. Sem ele, os avisos ficariam só na tabela
// de erros, e um erro antes de o banco abrir não ficaria em lugar nenhum.
func ComLogDoSistema(proximo slog.Handler) slog.Handler {
	if emServico, err := svc.IsWindowsService(); err != nil || !emServico {
		return proximo
	}
	// A origem precisa ser registrada uma vez, e o serviço roda com permissão para isso. Se já existe, é o caso normal.
	_ = eventlog.InstallAsEventCreate(Nome, eventlog.Error|eventlog.Warning|eventlog.Info)
	eventos, err := eventlog.Open(Nome)
	if err != nil {
		return proximo
	}
	return novoParaEventos(proximo, eventos)
}

// manipulador é o svc.Handler: o trabalho roda numa goroutine, e Stop ou Shutdown cancelam o ctx dele, o mesmo encerramento
// que o SIGTERM faz no Linux.
type manipulador struct {
	trabalho Trabalho
	erro     error
}

// Execute responde ao gerenciador de serviços até o trabalho terminar. O serviço fica como iniciado logo no começo: aplicar
// o esquema pode demorar, e quem espera o site responder são os scripts.
func (m *manipulador) Execute(_ []string, pedidos <-chan svc.ChangeRequest, estado chan<- svc.Status) (bool, uint32) {
	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()
	terminou := make(chan error, 1)
	estado <- svc.Status{State: svc.StartPending}
	go func() { terminou <- m.trabalho(ctx) }()
	estado <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for {
		select {
		case err := <-terminou:
			return m.fim(err)
		case pedido := <-pedidos:
			switch pedido.Cmd {
			case svc.Interrogate:
				estado <- pedido.CurrentStatus
			case svc.Stop, svc.Shutdown:
				estado <- svc.Status{State: svc.StopPending, WaitHint: uint32(prazoParaParar / time.Millisecond)}
				cancelar()
				return m.fim(<-terminou)
			}
		}
	}
}

func (m *manipulador) fim(err error) (bool, uint32) {
	if err != nil {
		m.erro = err
		return true, codigoDeFalha
	}
	return false, 0
}
