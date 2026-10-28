package servico

import "testing"

// Regra: só ligar, desligar e reiniciar rodam elevados.
func TestAcaoValida(t *testing.T) {
	t.Parallel()
	casos := map[string]bool{"ligar": true, "desligar": true, "reiniciar": true, "copia": false, "": false, "Ligar": false}
	for acao, esperado := range casos {
		if AcaoValida(acao) != esperado {
			t.Errorf("%q", acao)
		}
	}
}
