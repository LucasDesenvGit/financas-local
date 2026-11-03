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
		// #86: as bordas que o teste de mutação mostrou. O texto continua depois do par de substitutos; o último
		// caractere ASCII (DEL, 0x7F) fica como está, e o primeiro fora dele (0x80) vira \u.
		{"😀 ok", barra + "uD83D" + barra + "uDE00 ok"},
		{"a\x7fb", "a\x7fb"},
		{"\u0080", barra + "u0080"},
	}
	for _, c := range casos {
		if obtido := EscaparComoDotnet(c.entrada); obtido != c.esperado {
			t.Errorf("EscaparComoDotnet(%q) = %q, esperado %q", c.entrada, obtido, c.esperado)
		}
	}
}
