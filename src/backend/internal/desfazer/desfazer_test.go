package desfazer

import (
	"regexp"
	"sync"
	"testing"
	"time"
)

// relogioMovel é um relógio de teste que anda quando o teste manda.
type relogioMovel struct {
	mu    sync.Mutex
	agora time.Time
}

func (r *relogioMovel) Agora() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.agora
}

func (r *relogioMovel) andar(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agora = r.agora.Add(d)
}

func novoRelogio() *relogioMovel {
	return &relogioMovel{agora: time.Date(2026, 10, 15, 15, 0, 0, 0, time.UTC)}
}

// Regra: a chave é um GUID sem hífens, em minúsculas (o ToString("N") do C#), e vale uma vez só.
func TestGuardarETirar(t *testing.T) {
	m := Nova[[]int64](novoRelogio())
	chave := m.Guardar([]int64{1, 2})
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(chave) {
		t.Errorf("chave %q fora do formato N", chave)
	}
	if outra := m.Guardar(nil); outra == chave {
		t.Error("duas chaves iguais")
	}
	estado, ok := m.Tirar(chave)
	if !ok || len(estado) != 2 || estado[1] != 2 {
		t.Fatalf("Tirar = %v, %v", estado, ok)
	}
	if _, ok := m.Tirar(chave); ok {
		t.Error("a chave valeu duas vezes")
	}
	if _, ok := m.Tirar("naoexiste"); ok {
		t.Error("chave que não existe")
	}
}

// Regra: o estado vence com 10 minutos exatos (o AbsoluteExpirationRelativeToNow do IMemoryCache: vencido quando a hora
// de vencer não está mais no futuro); um segundo antes ainda vale.
func TestPrazo(t *testing.T) {
	r := novoRelogio()
	m := Nova[string](r)
	quase := m.Guardar("quase")
	vencida := m.Guardar("vencida")
	r.andar(Prazo - time.Second)
	if v, ok := m.Tirar(quase); !ok || v != "quase" {
		t.Errorf("um segundo antes do prazo: %q, %v", v, ok)
	}
	r.andar(time.Second)
	if _, ok := m.Tirar(vencida); ok {
		t.Error("valeu com o prazo exato")
	}
}

// Regra: guardar um estado novo joga fora os vencidos (a memória não cresce sem fim).
func TestGuardarLimpaOsVencidos(t *testing.T) {
	r := novoRelogio()
	m := Nova[int](r)
	m.Guardar(1)
	m.Guardar(2)
	r.andar(Prazo)
	nova := m.Guardar(3)
	if len(m.itens) != 1 {
		t.Errorf("%d estados guardados, esperado só o novo", len(m.itens))
	}
	if v, ok := m.Tirar(nova); !ok || v != 3 {
		t.Errorf("o novo: %d, %v", v, ok)
	}
}

// Regra: várias requisições ao mesmo tempo guardam e tiram sem perder nem repetir (go test -race).
func TestAoMesmoTempo(t *testing.T) {
	m := Nova[int](novoRelogio())
	var wg sync.WaitGroup
	tiradas := make(chan int, 100)
	for i := range 100 {
		wg.Go(func() {
			if v, ok := m.Tirar(m.Guardar(i)); ok {
				tiradas <- v
			}
		})
	}
	wg.Wait()
	close(tiradas)
	vistas := map[int]bool{}
	for v := range tiradas {
		vistas[v] = true
	}
	if len(vistas) != 100 {
		t.Errorf("%d de 100 estados voltaram", len(vistas))
	}
}
