package servico

import (
	"errors"
	"fmt"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// espera pelo serviço ligar ou desligar, como o Espera do ServicoWindows.cs.
const espera = 60 * time.Second

// ComoInstalar é o que dizer quando o site não está instalado.
const ComoInstalar = "Instale pelo arquivo FinancasLocal.msi."

// Consultar lê o estado do serviço com permissão de usuário comum (só consulta).
func Consultar() (Estado, error) { return consultar(Nome) }

func consultar(nomeDoServico string) (Estado, error) {
	scm, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return NaoInstalado, fmt.Errorf("abrir o gerenciador de serviços: %w", err)
	}
	defer windows.CloseServiceHandle(scm)
	nome, _ := windows.UTF16PtrFromString(nomeDoServico)
	h, err := windows.OpenService(scm, nome, windows.SERVICE_QUERY_STATUS)
	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return NaoInstalado, nil
	}
	if err != nil {
		return NaoInstalado, fmt.Errorf("abrir o serviço: %w", err)
	}
	defer windows.CloseServiceHandle(h)
	var status windows.SERVICE_STATUS
	if err := windows.QueryServiceStatus(h, &status); err != nil {
		return NaoInstalado, fmt.Errorf("ler o serviço: %w", err)
	}
	return estadoDe(svc.State(status.CurrentState)), nil
}

func estadoDe(s svc.State) Estado {
	switch s {
	case svc.Stopped:
		return Parado
	case svc.Running:
		return Rodando
	case svc.StartPending, svc.ContinuePending:
		return Ligando
	default:
		return Desligando
	}
}

// Executar roda no processo elevado: desliga (desligar, reiniciar) e liga (ligar, reiniciar), esperando cada passo.
func Executar(acao string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(Nome)
	if err != nil {
		return err
	}
	defer s.Close()
	if acao == "desligar" || acao == "reiniciar" {
		st, err := s.Query()
		if err != nil {
			return err
		}
		if st.State != svc.Stopped {
			if _, err := s.Control(svc.Stop); err != nil {
				return err
			}
			if err := esperar(s, svc.Stopped); err != nil {
				return err
			}
		}
	}
	if acao == "ligar" || acao == "reiniciar" {
		st, err := s.Query()
		if err != nil {
			return err
		}
		if st.State != svc.Running {
			if err := s.Start(); err != nil {
				return err
			}
			return esperar(s, svc.Running)
		}
	}
	return nil
}

func esperar(s *mgr.Service, estado svc.State) error {
	limite := time.Now().Add(espera)
	for {
		st, err := s.Query()
		if err != nil {
			return err
		}
		if st.State == estado {
			return nil
		}
		if time.Now().After(limite) {
			return fmt.Errorf("o serviço não chegou ao estado %d em %s", estado, espera)
		}
		time.Sleep(250 * time.Millisecond)
	}
}
