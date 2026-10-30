// Pacote desfazer guarda, na memória do processo e por pouco tempo, o estado de antes de uma alteração em lote, para o
// botão "Desfazer" (o IMemoryCache do C#, com o PrazoDesfazer de 10 minutos). O navegador recebe só a chave: um GUID
// sem hífens, em minúsculas (o Guid.NewGuid().ToString("N")). Cada chave vale uma vez só, e some no fim do prazo.
//
// Como no C#, nada disso vai para o banco: reiniciar o serviço perde o Desfazer, e quem está com a tela aberta vê
// "Não dá mais para desfazer".
package desfazer

import (
	"encoding/hex"
	"sync"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Prazo é o PrazoDesfazer do C#.
const Prazo = 10 * time.Minute

// MensagemVencida é o 404 de quando a chave não existe, venceu ou já foi usada.
const MensagemVencida = "Não dá mais para desfazer: o prazo passou ou já foi desfeito."

type guardado[T any] struct {
	estado T
	vence  time.Time
}

// Memoria guarda estados do tipo T por chave. É segura para várias requisições ao mesmo tempo.
type Memoria[T any] struct {
	relogio relogio.Relogio
	mu      sync.Mutex
	itens   map[string]guardado[T]
}

// Nova cria a memória com o relógio que diz quando cada estado vence.
func Nova[T any](r relogio.Relogio) *Memoria[T] {
	return &Memoria[T]{relogio: r, itens: map[string]guardado[T]{}}
}

// Guardar guarda o estado e devolve a chave nova. Aproveita para jogar fora os vencidos.
func (m *Memoria[T]) Guardar(estado T) string {
	g := tipos.NovoGuid()
	chave := hex.EncodeToString(g[:])
	agora := m.relogio.Agora()
	m.mu.Lock()
	defer m.mu.Unlock()
	for c, item := range m.itens {
		if !agora.Before(item.vence) {
			delete(m.itens, c)
		}
	}
	m.itens[chave] = guardado[T]{estado, agora.Add(Prazo)}
	return chave
}

// Tirar devolve o estado da chave e o apaga (vale uma vez só). False se a chave não existe ou venceu.
func (m *Memoria[T]) Tirar(chave string) (T, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	item, ok := m.itens[chave]
	delete(m.itens, chave)
	if !ok || !m.relogio.Agora().Before(item.vence) {
		var zero T
		return zero, false
	}
	return item.estado, true
}
