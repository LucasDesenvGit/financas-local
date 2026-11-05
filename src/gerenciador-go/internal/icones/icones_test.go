package icones

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/tela"
)

func base(t *testing.T, lado int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, lado, lado))
	for i := range img.Pix {
		img.Pix[i] = 0x10
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// Regra: o ponto da cor do estado no canto de baixo à direita, com a borda branca, e o resto do ícone como estava.
func TestComPonto(t *testing.T) {
	t.Parallel()
	for _, lado := range []int{32, 256} {
		saida, err := ComPonto(base(t, lado), CorDoEstado(tela.NoAr))
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(bytes.NewReader(saida))
		if err != nil {
			t.Fatal(err)
		}
		e := float64(lado) / 32
		ponto := func(x, y float64) color.RGBA { return color.RGBAModel.Convert(img.At(int(x*e), int(y*e))).(color.RGBA) }
		if c := ponto(24.5, 24.5); c != (color.RGBA{0x2e, 0x9e, 0x6b, 0xff}) {
			t.Errorf("%d: centro %v", lado, c)
		}
		if c := ponto(24.5, 17.6); c != (color.RGBA{0xff, 0xff, 0xff, 0xff}) {
			t.Errorf("%d: borda %v", lado, c)
		}
		if c := ponto(2, 2); c != (color.RGBA{0x10, 0x10, 0x10, 0x10}) {
			t.Errorf("%d: canto %v", lado, c)
		}
		if img.Bounds().Dx() != lado {
			t.Errorf("tamanho %d", img.Bounds().Dx())
		}
	}
}

// Regra: as cores do C# por estado.
func TestCorDoEstado(t *testing.T) {
	t.Parallel()
	casos := map[tela.Estado]color.RGBA{
		tela.NoAr: {0x2e, 0x9e, 0x6b, 0xff}, tela.Desligado: {0xd2, 0x54, 0x6b, 0xff},
		tela.SemResposta: {0xe0, 0xa1, 0x00, 0xff}, tela.Mudando: {0xe0, 0xa1, 0x00, 0xff},
		tela.NaoInstalado: {0x80, 0x80, 0x80, 0xff},
	}
	for e, cor := range casos {
		if CorDoEstado(e) != cor {
			t.Errorf("%d", e)
		}
	}
}

// Regra: o que não é PNG é erro.
func TestBaseInvalida(t *testing.T) {
	t.Parallel()
	if _, err := ComPonto([]byte("não é png"), color.RGBA{}); err == nil {
		t.Error("sem erro")
	}
}

// Regra: o desenho não muda sem querer. O resumo é o do ícone de 32 com o ponto verde conferido a olho (a cópia
// ampliada vai no PR #419); se o desenho mudar de propósito, confira de novo e troque o resumo.
func TestDesenhoGravado(t *testing.T) {
	t.Parallel()
	saida, err := ComPonto(base(t, 32), CorDoEstado(tela.NoAr))
	if err != nil {
		t.Fatal(err)
	}
	if resumo := fmt.Sprintf("%x", sha256.Sum256(saida)); resumo != gravado {
		t.Errorf("desenho mudou: %s", resumo)
	}
}

const gravado = "550e29bc19235b6f53e2735726f95d035dd668a737a54f83668dbaea912686ca"
