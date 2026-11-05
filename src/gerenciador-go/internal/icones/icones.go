// Package icones desenha o ícone da bandeja com um ponto da cor da situação, como o Icones.Bandeja do C#. A cor
// nunca é a única pista (WCAG 1.4.1): a dica do ícone e a janela dizem a situação em texto.
package icones

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"

	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/tela"
)

// CorDoEstado é a cor do ponto, a mesma do C# (verde no ar, vermelho desligado, amarelo mudando ou sem resposta).
func CorDoEstado(e tela.Estado) color.RGBA {
	switch e {
	case tela.NoAr:
		return color.RGBA{0x2e, 0x9e, 0x6b, 0xff}
	case tela.Desligado:
		return color.RGBA{0xd2, 0x54, 0x6b, 0xff}
	case tela.SemResposta, tela.Mudando:
		return color.RGBA{0xe0, 0xa1, 0x00, 0xff}
	default:
		return color.RGBA{0x80, 0x80, 0x80, 0xff}
	}
}

// ComPonto devolve o PNG base com o ponto no canto de baixo à direita: no C#, num ícone de 32, um círculo branco de 15
// em (17, 17) e o colorido de 11 em (19, 19); aqui, nas mesmas proporções do tamanho do PNG.
func ComPonto(base []byte, cor color.RGBA) ([]byte, error) {
	img, err := png.Decode(bytes.NewReader(base))
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	saida := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(saida, saida.Bounds(), img, b.Min, draw.Src)
	escala := float64(b.Dx()) / 32
	circulo(saida, (17+7.5)*escala, (17+7.5)*escala, 7.5*escala, color.RGBA{0xff, 0xff, 0xff, 0xff})
	circulo(saida, (19+5.5)*escala, (19+5.5)*escala, 5.5*escala, cor)
	var buf bytes.Buffer
	if err := png.Encode(&buf, saida); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// circulo pinta um disco com a borda suavizada (4×4 amostras por pixel). Passa pela imagem inteira: ela é pequena,
// e o ícone de cada situação é desenhado uma vez só.
func circulo(img *image.RGBA, cx, cy, raio float64, cor color.RGBA) {
	const n = 4
	for y := range img.Bounds().Dy() {
		for x := range img.Bounds().Dx() {
			dentro := 0
			for sy := 0; sy < n; sy++ {
				for sx := 0; sx < n; sx++ {
					dx := float64(x) + (float64(sx)+0.5)/n - cx
					dy := float64(y) + (float64(sy)+0.5)/n - cy
					if math.Hypot(dx, dy) <= raio {
						dentro++
					}
				}
			}
			if dentro == 0 {
				continue
			}
			a := float64(dentro) / (n * n)
			atual := img.RGBAAt(x, y)
			misturar := func(c, f uint8) uint8 { return uint8(math.Round(float64(f)*(1-a) + float64(c)*a)) }
			img.SetRGBA(x, y, color.RGBA{misturar(cor.R, atual.R), misturar(cor.G, atual.G), misturar(cor.B, atual.B),
				misturar(0xff, atual.A)})
		}
	}
}
