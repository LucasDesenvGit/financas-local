package servicosistema

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/sys/windows/svc"
)

// rodada é uma chamada ao Execute, sem o gerenciador de serviços: o teste faz o papel dele pelos canais.
type rodada struct {
	pedidos chan svc.ChangeRequest
	estado  chan svc.Status
	saida   chan [2]any
}

func iniciar(m *manipulador) rodada {
	r := rodada{make(chan svc.ChangeRequest), make(chan svc.Status, 8), make(chan [2]any, 1)}
	go func() {
		especifico, codigo := m.Execute(nil, r.pedidos, r.estado)
		r.saida <- [2]any{especifico, codigo}
	}()
	return r
}

func (r rodada) proximoEstado(t *testing.T) svc.Status {
	t.Helper()
	select {
	case e := <-r.estado:
		return e
	case <-time.After(5 * time.Second):
		t.Fatal("o serviço não informou o estado")
		return svc.Status{}
	}
}

func (r rodada) esperarIniciado(t *testing.T) {
	t.Helper()
	if e := r.proximoEstado(t); e.State != svc.StartPending {
		t.Fatalf("primeiro estado = %v", e.State)
	}
	e := r.proximoEstado(t)
	if e.State != svc.Running || e.Accepts != svc.AcceptStop|svc.AcceptShutdown {
		t.Fatalf("segundo estado = %+v", e)
	}
}

func (r rodada) conferirSaida(t *testing.T, especifico bool, codigo uint32) {
	t.Helper()
	select {
	case s := <-r.saida:
		if s[0] != especifico || s[1] != codigo {
			t.Errorf("saída = %v, esperada %v e %d", s, especifico, codigo)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("o serviço não terminou")
	}
}

// Regra: Stop e Shutdown param do mesmo jeito: o estado passa a "parando", o ctx acaba e o Execute só volta quando o
// trabalho terminou, para o banco fechar direito.
func TestPararOServico(t *testing.T) {
	casos := []struct {
		nome   string
		pedido svc.Cmd
	}{{"Stop", svc.Stop}, {"Shutdown (o computador desligando)", svc.Shutdown}}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			soltar := make(chan struct{})
			m := &manipulador{trabalho: func(ctx context.Context) error {
				<-ctx.Done()
				<-soltar
				return nil
			}}
			r := iniciar(m)
			r.esperarIniciado(t)
			r.pedidos <- svc.ChangeRequest{Cmd: caso.pedido}
			e := r.proximoEstado(t)
			if e.State != svc.StopPending || e.WaitHint != 40000 {
				t.Fatalf("estado = %+v", e)
			}
			select {
			case <-r.saida:
				t.Fatal("voltou antes de o trabalho terminar")
			case <-time.After(20 * time.Millisecond):
			}
			close(soltar)
			r.conferirSaida(t, false, 0)
			if m.erro != nil {
				t.Errorf("erro = %v", m.erro)
			}
		})
	}
}

// Regra: o erro ao parar vira falha do serviço, e o Rodar o devolve para ir ao log.
func TestErroAoParar(t *testing.T) {
	falha := errors.New("não fechou")
	m := &manipulador{trabalho: func(ctx context.Context) error {
		<-ctx.Done()
		return falha
	}}
	r := iniciar(m)
	r.esperarIniciado(t)
	r.pedidos <- svc.ChangeRequest{Cmd: svc.Stop}
	r.conferirSaida(t, true, codigoDeFalha)
	if !errors.Is(m.erro, falha) {
		t.Errorf("erro = %v", m.erro)
	}
}

// Regra: o trabalho que para sozinho encerra o serviço: com erro é falha, sem erro não é.
func TestTrabalhoTerminaSozinho(t *testing.T) {
	falha := errors.New("porta ocupada")
	casos := []struct {
		nome       string
		erro       error
		especifico bool
		codigo     uint32
	}{{"sem erro", nil, false, 0}, {"com erro", falha, true, codigoDeFalha}}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			soltar := make(chan struct{})
			m := &manipulador{trabalho: func(context.Context) error {
				<-soltar
				return caso.erro
			}}
			r := iniciar(m)
			r.esperarIniciado(t)
			close(soltar)
			r.conferirSaida(t, caso.especifico, caso.codigo)
			if !errors.Is(m.erro, caso.erro) {
				t.Errorf("erro = %v", m.erro)
			}
		})
	}
}

// Regra: o gerenciador de serviços pergunta o estado de tempos em tempos, e a resposta é o estado atual. Pedido que o
// serviço não aceitou não muda nada.
func TestPerguntaDoEstado(t *testing.T) {
	m := &manipulador{trabalho: func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	}}
	r := iniciar(m)
	r.esperarIniciado(t)
	r.pedidos <- svc.ChangeRequest{Cmd: svc.Pause}
	atual := svc.Status{State: svc.Running, CheckPoint: 7}
	r.pedidos <- svc.ChangeRequest{Cmd: svc.Interrogate, CurrentStatus: atual}
	if e := r.proximoEstado(t); e != atual {
		t.Errorf("estado = %+v", e)
	}
	r.pedidos <- svc.ChangeRequest{Cmd: svc.Stop}
	r.conferirSaida(t, false, 0)
}

// Regra: no teste, como no terminal, o processo não é um serviço: o handler fica como está.
func TestForaDeServicoFicaSemLogDeEventos(t *testing.T) {
	proximo := &registrador{}
	if h := ComLogDoSistema(proximo); h != proximo {
		t.Errorf("handler = %T", h)
	}
}
