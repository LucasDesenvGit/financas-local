package servico

import (
	"testing"

	"golang.org/x/sys/windows/svc"
)

// Regra: os estados do Windows viram os do ServicoWindows.cs.
func TestEstadoDe(t *testing.T) {
	t.Parallel()
	casos := map[svc.State]Estado{
		svc.Stopped: Parado, svc.Running: Rodando, svc.StartPending: Ligando, svc.ContinuePending: Ligando,
		svc.StopPending: Desligando, svc.PausePending: Desligando, svc.Paused: Desligando,
	}
	for s, esperado := range casos {
		if obtido := estadoDe(s); obtido != esperado {
			t.Errorf("%d: %d, esperado %d", s, obtido, esperado)
		}
	}
}

// Regra: serviço que não existe é "não instalado", sem erro; um que existe e roda sempre no Windows (o registro de
// eventos) é "rodando". Só consulta, sem administrador.
func TestConsultar(t *testing.T) {
	t.Parallel()
	if e, err := consultar("FinancasLocalNaoExiste"); err != nil || e != NaoInstalado {
		t.Errorf("não existe: %d, %v", e, err)
	}
	if e, err := consultar("EventLog"); err != nil || e != Rodando {
		t.Errorf("EventLog: %d, %v", e, err)
	}
}
