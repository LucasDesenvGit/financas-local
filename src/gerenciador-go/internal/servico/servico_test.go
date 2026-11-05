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

// Regra: a saída do "systemctl show" vira Chave → valor; linha sem "=" ou sem chave fica de fora, e o valor pode ter
// "=" dentro.
func TestPropriedades(t *testing.T) {
	t.Parallel()
	p := Propriedades("LoadState=loaded\nActiveState=active \n\nsem igual\n=sem chave\nExecStart={ path=/x ; argv[]=/x a=b }\n")
	esperado := map[string]string{"LoadState": "loaded", "ActiveState": "active", "ExecStart": "{ path=/x ; argv[]=/x a=b }"}
	if len(p) != len(esperado) {
		t.Errorf("%v", p)
	}
	for chave, valor := range esperado {
		if p[chave] != valor {
			t.Errorf("%s: %q", chave, p[chave])
		}
	}
}

// Regra: os estados do systemd viram os do ServicoLinux.cs; unidade que não carregou (not-found, masked) não está
// instalada, qualquer que seja o ActiveState.
func TestEstadoDaUnidade(t *testing.T) {
	t.Parallel()
	casos := []struct {
		carregada, ativa string
		esperado         Estado
	}{
		{"loaded", "active", Rodando},
		{"loaded", "reloading", Rodando},
		{"loaded", "activating", Ligando},
		{"loaded", "deactivating", Desligando},
		{"loaded", "inactive", Parado},
		{"loaded", "failed", Parado},
		{"loaded", "", Parado},
		{"not-found", "inactive", NaoInstalado},
		{"masked", "active", NaoInstalado},
		{"", "", NaoInstalado},
	}
	for _, c := range casos {
		if obtido := EstadoDaUnidade(map[string]string{"LoadState": c.carregada, "ActiveState": c.ativa}); obtido != c.esperado {
			t.Errorf("%s/%s: %d, esperado %d", c.carregada, c.ativa, obtido, c.esperado)
		}
	}
}

// Regra: cada ação é um verbo do systemctl, e só as três.
func TestVerbos(t *testing.T) {
	t.Parallel()
	esperado := map[string]string{"ligar": "start", "desligar": "stop", "reiniciar": "restart"}
	if len(verbos) != len(esperado) {
		t.Errorf("%v", verbos)
	}
	for acao, verbo := range esperado {
		if verbos[acao] != verbo || !AcaoValida(acao) {
			t.Errorf("%s: %q", acao, verbos[acao])
		}
	}
}
