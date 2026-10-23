package esquema

import (
	"crypto/sha256"
	"testing"
)

// A assinatura igual à do .NET, com os scripts e com textos escolhidos, é conferida contra o gabarito da sonda em
// internal/banco (TestAssinaturaIgualAoDotnet). Aqui ficam os casos que um SHA-256 de verdade não produz.

func TestAssinaturaZeroViraUm(t *testing.T) {
	// Os 4 primeiros bytes zerados, ou só com o bit de sinal (que a máscara tira), dariam 0: o .NET usa 1.
	for _, inicio := range [][4]byte{{0, 0, 0, 0}, {0, 0, 0, 0x80}} {
		var hash [sha256.Size]byte
		copy(hash[:], inicio[:])
		if got := assinaturaDoHash(hash); got != 1 {
			t.Errorf("hash começando com % x: assinatura %d, esperado 1", inicio, got)
		}
	}
}

func TestAssinaturaLittleEndianSemSinal(t *testing.T) {
	// Como o BitConverter.ToInt32 num processador x86 ou ARM: 01 02 03 04 é 0x04030201; o bit de sinal sai.
	for _, c := range []struct {
		inicio [4]byte
		want   int32
	}{
		{[4]byte{0x01, 0x02, 0x03, 0x04}, 0x04030201},
		{[4]byte{0xFF, 0xFF, 0xFF, 0xFF}, 0x7FFFFFFF},
	} {
		var hash [sha256.Size]byte
		copy(hash[:], c.inicio[:])
		if got := assinaturaDoHash(hash); got != c.want {
			t.Errorf("hash começando com % x: assinatura %#x, esperado %#x", c.inicio, got, c.want)
		}
	}
}
