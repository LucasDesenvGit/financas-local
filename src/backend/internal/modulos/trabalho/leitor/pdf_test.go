package leitor

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/leitor/ficticios"
)

func TestLerPDFRecusaComMensagemClara(t *testing.T) {
	umaPagina := [][]ficticios.Texto{ficticios.Pagina(ficticios.Linha(0, 30, "Holerite fictício"))}
	seis := make([][]ficticios.Texto, PaginasMaximo+1)
	for i := range seis {
		seis[i] = umaPagina[0]
	}
	cinco := seis[:PaginasMaximo]
	grande := append([]byte("%PDF-1.4\n"), bytes.Repeat([]byte(" "), TamanhoMaximo)...)
	semTexto := ficticios.GerarPDF([][]ficticios.Texto{{}}, "")
	soEspacos := ficticios.GerarPDF([][]ficticios.Texto{ficticios.Pagina(ficticios.Linha(0, 30, "   "))}, "")
	comSenha := ficticios.GerarPDF(umaPagina, "12345678900")
	casos := []struct {
		nome  string
		dados []byte
		senha string
		err   error
	}{
		{"texto qualquer com a extensão .pdf", []byte("isto não é um PDF"), "", ErrNaoEPDF},
		{"vazio", nil, "", ErrNaoEPDF},
		{"grande demais", grande, "", ErrGrande},
		{"páginas demais", ficticios.GerarPDF(seis, ""), "", ErrGrande},
		{"corrompido", []byte("%PDF-1.4\nlixo sem estrutura"), "", ErrIlegivel},
		{"corrompido no conteúdo", corromperConteudo(ficticios.GerarPDF(umaPagina, "")), "", ErrIlegivel},
		{"escaneado (sem texto)", semTexto, "", ErrSemTexto},
		{"só espaços", soEspacos, "", ErrSemTexto},
		{"com senha, sem a senha", comSenha, "", ErrSenha},
		{"com senha, a senha errada", comSenha, "98765432100", ErrSenha},
		{"com senha, a certa", comSenha, "12345678900", nil},
		{"sem senha, com uma senha à toa", ficticios.GerarPDF(umaPagina, ""), "qualquer", nil},
		{"o máximo de páginas", ficticios.GerarPDF(cinco, ""), "", nil},
		{"o cabeçalho depois do começo", append([]byte("x"), ficticios.GerarPDF(umaPagina, "")...), "", ErrNaoEPDF},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			paginas, err := LerPDF(c.dados, c.senha)
			if !errors.Is(err, c.err) {
				t.Fatalf("err = %v, quero %v", err, c.err)
			}
			if c.err != nil {
				if paginas != nil {
					t.Errorf("com erro, não devolve páginas: %v", paginas)
				}
				return
			}
			if len(paginas) == 0 || paginas[0][0].Texto() != "Holerite fictício" {
				t.Errorf("páginas: %v", paginas)
			}
		})
	}
}

// corromperConteudo troca o operador do texto por um que a biblioteca não espera, e ela entra em pânico.
func corromperConteudo(pdf []byte) []byte {
	return bytes.Replace(pdf, []byte("BT /F1 9 Tf"), []byte("BT /F1 Tf"), 1)
}

// O erro não leva texto do PDF (#228: nada do holerite nos logs).
func TestErroSemTextoDoPDF(t *testing.T) {
	_, err := LerPDF([]byte("%PDF-1.4\nSALARIO 5.000,00 CPF 123"), "")
	if err == nil || strings.Contains(err.Error(), "SALARIO") || strings.Contains(err.Error(), "5.000") || err.Error() != ErrIlegivel.Error() {
		t.Errorf("err = %v", err)
	}
}

func TestAgruparLinhasECelulas(t *testing.T) {
	l := func(s string, x, y, w Pontos) letra { return letra{s: s, x: x, y: y, w: w, fonte: 10} }
	casos := []struct {
		nome   string
		letras []letra
		want   string // linhas separadas por "/", células por "|"
	}{
		{"de cima para baixo, da esquerda para a direita",
			[]letra{l("b", 5, 100, 5), l("c", 0, 50, 5), l("a", 0, 100, 5)}, "ab/c"},
		{"a mesma linha com 2 pontos de folga", []letra{l("a", 0, 100, 5), l("b", 5, 98, 5)}, "ab"},
		{"mais de 2 pontos é outra linha", []letra{l("a", 0, 100, 5), l("b", 5, 97.9, 5)}, "a/b"},
		{"vão pequeno vira espaço", []letra{l("a", 0, 100, 5), l("b", 6.6, 100, 5)}, "a b"},
		{"vão mínimo não", []letra{l("a", 0, 100, 5), l("b", 6.5, 100, 5)}, "ab"},
		{"vão de letra e meia separa a célula", []letra{l("a", 0, 100, 5), l("b", 20.1, 100, 5)}, "a|b"},
		{"letra e meia ainda é a mesma", []letra{l("a", 0, 100, 5), l("b", 20, 100, 5)}, "a b"},
		{"o espaço do PDF não dobra", []letra{l("a", 0, 100, 5), l(" ", 5, 100, 5), l("b", 12, 100, 5)}, "a b"},
		{"espaço no começo da célula some", []letra{l(" ", 0, 100, 5), l("a", 5, 100, 5)}, "a"},
		{"o espaço do PDF conta, mesmo colado na letra", []letra{l("a", 0, 100, 5), l(" ", 5, 100, 1), l("b", 6, 100, 5)}, "a b"},
		{"sem largura: meia fonte", []letra{l("a", 0, 100, 0), l("b", 5, 100, 0), l("c", 25.1, 100, 0)}, "ab|c"},
		{"linha só de espaços some", []letra{l(" ", 0, 100, 5), l("a", 0, 50, 5)}, "a"},
	}
	for _, c := range casos {
		var linhas []string
		for _, ln := range Agrupar(c.letras) {
			var cels []string
			for _, cel := range ln.Celulas {
				cels = append(cels, cel.Texto)
			}
			linhas = append(linhas, strings.Join(cels, "|"))
		}
		if got := strings.Join(linhas, "/"); got != c.want {
			t.Errorf("%s: %q, quero %q", c.nome, got, c.want)
		}
	}
}

// As posições das células: o começo, o fim (com a largura da última letra) e o meio.
func TestPosicaoDasCelulas(t *testing.T) {
	ls := Agrupar([]letra{{s: "a", x: 10, y: 100, w: 5, fonte: 10}, {s: "b", x: 15, y: 100, w: 5, fonte: 10},
		{s: "c", x: 60, y: 100, w: 4, fonte: 10}})
	if len(ls) != 1 || len(ls[0].Celulas) != 2 || ls[0].Y != 100 {
		t.Fatalf("linhas: %+v", ls)
	}
	if c := Agrupar([]letra{{s: " ", x: 0, y: 1, w: 5, fonte: 10}, {s: "a", x: 5, y: 1, w: 5, fonte: 10}})[0].Celulas[0]; c.X0 != 5 {
		t.Errorf("o espaço do começo não conta na posição: %+v", c)
	}
	a, b := ls[0].Celulas[0], ls[0].Celulas[1]
	if a.X0 != 10 || a.X1 != 20 || a.Centro() != 15 || b.X0 != 60 || b.X1 != 64 || ls[0].Texto() != "ab c" {
		t.Errorf("células: %+v %+v", a, b)
	}
}
