package servidor

import "testing"

// #81: o EscaparComoDotnet grava no banco o mesmo texto que o JsonSerializer.Serialize do .NET (JavaScriptEncoder.Default).
func TestEscaparComoDotnet(t *testing.T) {
	t.Parallel()
	barra := `\`
	casos := []struct{ entrada, esperado string }{
		{"ASCII fica", "ASCII fica"},
		{"D" + "ólar", "D" + barra + "u00F3lar"},
		{"Configurações › Erros", "Configura" + barra + "u00E7" + barra + "u00F5es " + barra + "u203A Erros"},
		// Fora do plano básico: um par de substitutos, como o System.Text.Json (e não um código de 5 dígitos).
		{"oi 😀", "oi " + barra + "uD83D" + barra + "uDE00"},
	}
	for _, c := range casos {
		if obtido := EscaparComoDotnet(c.entrada); obtido != c.esperado {
			t.Errorf("EscaparComoDotnet(%q) = %q, esperado %q", c.entrada, obtido, c.esperado)
		}
	}
}
