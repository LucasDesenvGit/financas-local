package autenticacao

import (
	"sync"
	"time"
)

// limitador é a política "login" do .NET (Program.cs, AddRateLimiter): janela fixa de 1 minuto, 5 pedidos por IP,
// sem fila, somando "entrar" e "configurar". O sexto pedido no mesmo minuto recebe 429 sem corpo, antes de olhar
// CSRF, corpo ou senha (o UseRateLimiter vem antes de tudo isso no .NET).
//
// A janela começa no primeiro pedido do IP e vale um minuto inteiro, como o FixedWindowRateLimiter. O tempo é o do
// relógio monotônico do processo (time.Now), e não o relógio de negócio: limite não depende da data.
type limitador struct {
	mu      sync.Mutex
	janelas map[string]janela
	maximo  int
	duracao time.Duration
	agora   func() time.Time
}

type janela struct {
	inicio time.Time
	usados int
}

func novoLimitador(maximo int, duracao time.Duration) *limitador {
	return &limitador{janelas: map[string]janela{}, maximo: maximo, duracao: duracao, agora: time.Now}
}

// permitir conta o pedido do IP e diz se ele cabe na janela atual.
func (l *limitador) permitir(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	agora := l.agora()
	j, existe := l.janelas[ip]
	if !existe || agora.Sub(j.inicio) >= l.duracao {
		// Janelas vencidas saem do mapa de vez em quando: o mapa não cresce sem fim com IPs que não voltam.
		if len(l.janelas) >= 1024 {
			for chave, outra := range l.janelas {
				if agora.Sub(outra.inicio) >= l.duracao {
					delete(l.janelas, chave)
				}
			}
		}
		j = janela{inicio: agora}
	}
	if j.usados >= l.maximo {
		l.janelas[ip] = j
		return false
	}
	j.usados++
	l.janelas[ip] = j
	return true
}
