package tarefas

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

type logEmMemoria struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *logEmMemoria) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *logEmMemoria) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// Dentro do synctest.Test o relógio é de mentira: o time.Sleep do teste só avança o relógio quando todas as
// goroutines da bolha estão paradas, e o synctest.Wait espera a tarefa chegar à próxima espera. Nada de tempo real.
func TestPeriodicaRodaNoAtrasoENoIntervaloEParaComOContexto(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		inicio := time.Now()
		var mu sync.Mutex
		var quando []time.Duration
		ctx, cancelar := context.WithCancel(t.Context())
		terminou := make(chan struct{})
		go func() {
			defer close(terminou)
			Periodica(ctx, slog.New(slog.DiscardHandler), "o teste", time.Minute, 15*time.Minute, func(context.Context) error {
				mu.Lock()
				defer mu.Unlock()
				quando = append(quando, time.Since(inicio))
				return nil
			})
		}()
		vezes := func() []time.Duration { mu.Lock(); defer mu.Unlock(); return append([]time.Duration{}, quando...) }

		time.Sleep(time.Minute - time.Second)
		synctest.Wait()
		if len(vezes()) != 0 {
			t.Fatalf("rodou antes do atraso de 1 minuto: %v", vezes())
		}
		time.Sleep(time.Second)
		synctest.Wait()
		time.Sleep(30 * time.Minute) // mais dois ciclos: 16 e 31 minutos
		synctest.Wait()
		esperado := []time.Duration{time.Minute, 16 * time.Minute, 31 * time.Minute}
		if obtido := vezes(); len(obtido) != len(esperado) || obtido[0] != esperado[0] || obtido[1] != esperado[1] || obtido[2] != esperado[2] {
			t.Fatalf("rodou em %v, esperado %v", obtido, esperado)
		}

		cancelar()
		synctest.Wait()
		select {
		case <-terminou:
		default:
			t.Fatal("a tarefa não parou com o contexto cancelado")
		}
	})
}

func TestPeriodicaCanceladaNoAtrasoNaoRoda(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancelar := context.WithCancel(t.Context())
		rodou := false
		terminou := make(chan struct{})
		go func() {
			defer close(terminou)
			Periodica(ctx, slog.New(slog.DiscardHandler), "o teste", time.Minute, time.Minute, func(context.Context) error {
				rodou = true
				return nil
			})
		}()
		time.Sleep(30 * time.Second)
		cancelar()
		<-terminou
		if rodou {
			t.Error("rodou mesmo cancelada antes do primeiro minuto (o Task.Delay do .NET volta sem rodar)")
		}
	})
}

func TestPanicoEErroNoCicloViramLogEATarefaSegue(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var log logEmMemoria
		ctx, cancelar := context.WithCancel(t.Context())
		defer cancelar()
		ciclos := 0
		go Periodica(ctx, slog.New(slog.NewTextHandler(&log, nil)), "o câmbio", time.Minute, time.Minute, func(context.Context) error {
			ciclos++
			switch ciclos {
			case 1:
				panic("bug no ciclo")
			case 2:
				return errors.New("banco ocupado")
			}
			return nil
		})
		time.Sleep(3 * time.Minute)
		synctest.Wait()
		if ciclos != 3 {
			t.Fatalf("%d ciclos: o pânico ou o erro pararam a tarefa", ciclos)
		}
		texto := log.String()
		for _, trecho := range []string{"Falha na atualização automática do câmbio.", "bug no ciclo", "pilha=", "banco ocupado", "origem=Câmbio"} {
			if !strings.Contains(texto, trecho) {
				t.Errorf("o log não tem %q:\n%s", trecho, texto)
			}
		}
	})
}

func TestCanceladoNoMeioDoCicloNaoEhErro(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var log logEmMemoria
		ctx, cancelar := context.WithCancel(t.Context())
		go Periodica(ctx, slog.New(slog.NewTextHandler(&log, nil)), "o câmbio", time.Minute, time.Minute, func(ctx context.Context) error {
			cancelar()
			return ctx.Err()
		})
		time.Sleep(time.Minute)
		synctest.Wait()
		if strings.Contains(log.String(), "Falha") {
			t.Errorf("parar o serviço no meio do ciclo virou erro no log:\n%s", log.String())
		}
	})
}

func TestLigadas(t *testing.T) {
	t.Parallel()
	casos := []struct {
		texto    string
		esperado []string
		erro     bool
	}{
		{"", nil, false},
		{"Cambio", []string{"Cambio"}, false},
		{" cambio ; CRIPTO ;", []string{"Cambio", "Cripto"}, false},
		{"Cambio; ;Cripto", []string{"Cambio", "Cripto"}, false}, // a parte vazia no meio não para a leitura (#456)
		{"Cambio;Cambio", []string{"Cambio"}, false},
		{"Sincronizacao", []string{"Sincronizacao"}, false},
		{"Cambio;Cotacoes", nil, true},
	}
	for _, c := range casos {
		t.Run(c.texto, func(t *testing.T) {
			ligadas, err := Ligadas(c.texto)
			if c.erro {
				if err == nil {
					t.Fatalf("nome desconhecido aceito: %v", ligadas)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(ligadas) != len(c.esperado) {
				t.Fatalf("ligadas %v, esperado %v", ligadas, c.esperado)
			}
			for _, nome := range c.esperado {
				if !ligadas[nome] {
					t.Errorf("%s não ligada", nome)
				}
			}
		})
	}
}

// Regra: a origem na tela de erros é a do .NET (RegistroErrosLoggerProvider.Origem): o câmbio em "Câmbio" e, desde a
// #298, a cripto em "Cripto"; o resto em "Sistema".
func TestOrigemComoNoDotnet(t *testing.T) {
	casos := []struct{ nome, origem string }{
		{"o câmbio", "Câmbio"},
		{"a cotação de criptomoedas", "Cripto"},
		{"a sincronização", "Sistema"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := origem(c.nome); got != c.origem {
				t.Errorf("origem(%q) = %q, esperado %q", c.nome, got, c.origem)
			}
		})
	}
}
