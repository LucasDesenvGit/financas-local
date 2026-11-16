// Pacote leitor lê o holerite em PDF (#228, ADR 0068): as palavras com a posição, as linhas e as colunas, as rubricas
// da tabela, os totais e o rodapé, e a prova real. Tudo no computador: nenhum serviço de fora. O PDF não é guardado.
//
// Não há um modelo por empresa: o leitor se apoia na anatomia comum dos holerites brasileiros (cabeçalho com o CNPJ e a
// competência; a tabela Código · Descrição · Referência · Vencimentos · Descontos; os totais e o líquido; o rodapé com
// as bases).
package leitor

import (
	"bytes"
	"errors"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"
)

// Limites do arquivo (ADR 0068).
const (
	TamanhoMaximo = 700 << 10 // 700 KB: em base64, cabe no limite de 1 MB do corpo dos pedidos ao site
	PaginasMaximo = 5
)

// Erros que a pessoa resolve.
var (
	ErrNaoEPDF   = errors.New("o arquivo não é um PDF")
	ErrGrande    = errors.New("o PDF é grande demais")
	ErrSenha     = errors.New("o PDF tem senha")
	ErrSemTexto  = errors.New("o PDF não tem texto")
	ErrIlegivel  = errors.New("o PDF não abriu")
	ErrSemTabela = errors.New("não achei a tabela do holerite")
)

// Pontos é uma posição ou uma medida na página do PDF (1/72 de polegada), como a biblioteca dá. Não é dinheiro.
type Pontos = float64

// Celula é um pedaço de texto contínuo numa linha (as letras sem um vão grande entre elas).
type Celula struct {
	Texto  string
	X0, X1 Pontos
}

// Centro é o meio da célula, para achar a coluna dela.
func (c Celula) Centro() Pontos { return (c.X0 + c.X1) / 2 }

// Linha é uma linha da página, de cima para baixo, com as células da esquerda para a direita.
type Linha struct {
	Y       Pontos
	Celulas []Celula
}

// Texto é a linha inteira, com as células separadas por espaço.
func (l Linha) Texto() string {
	partes := make([]string, len(l.Celulas))
	for i, c := range l.Celulas {
		partes[i] = c.Texto
	}
	return strings.Join(partes, " ")
}

// letra é uma letra do PDF com a posição.
type letra struct {
	s       string
	x, y, w Pontos
	fonte   Pontos
}

// LerPDF abre o PDF (com a senha, se tiver) e devolve as linhas de cada página. Só extrai texto: script, formulário e
// anexo dentro do PDF não rodam (a biblioteca não executa nada). Um PDF por vez, com prazo e com o conteúdo limitado
// (conteudo.go, revisão do #706).
func LerPDF(dados []byte, senha string) ([][]Linha, error) {
	if len(dados) > TamanhoMaximo {
		return nil, ErrGrande
	}
	if !bytes.HasPrefix(dados, []byte("%PDF-")) { // pelo conteúdo, não pela extensão
		return nil, ErrNaoEPDF
	}
	return comPrazo(vaga, prazo, func(limite time.Time) ([][]Linha, error) { return lerComLimites(dados, senha, limite) })
}

// Agrupar junta as letras em linhas (a mesma altura, com folga de 2 pontos) e as linhas em células: um vão de mais
// de uma letra e meia separa duas células; um vão pequeno vira espaço.
func Agrupar(letras []letra) []Linha {
	sort.SliceStable(letras, func(i, j int) bool {
		if math.Abs(letras[i].y-letras[j].y) > 2 {
			return letras[i].y > letras[j].y
		}
		return letras[i].x < letras[j].x
	})
	var linhas []Linha
	var atual []letra
	fechar := func() {
		if len(atual) > 0 {
			if l := celulas(atual); len(l.Celulas) > 0 {
				linhas = append(linhas, l)
			}
		}
		atual = nil
	}
	for _, l := range letras {
		if len(atual) > 0 && math.Abs(atual[0].y-l.y) > 2 {
			fechar()
		}
		atual = append(atual, l)
	}
	fechar()
	return linhas
}

// celulas monta as células de uma linha (as letras já da esquerda para a direita).
func celulas(letras []letra) Linha {
	linha := Linha{Y: letras[0].y}
	var b strings.Builder
	var c Celula
	var fim Pontos // onde acabou a última letra
	fechar := func() {
		if t := strings.TrimSpace(b.String()); t != "" {
			c.Texto = strings.Join(strings.Fields(t), " ")
			linha.Celulas = append(linha.Celulas, c)
		}
		b.Reset()
	}
	for _, l := range letras {
		if b.Len() == 0 && strings.TrimFunc(l.s, unicode.IsSpace) == "" {
			continue // espaço no começo da célula
		}
		largura := l.w
		if largura <= 0 { // fonte sem as larguras: estima meia letra
			largura = l.fonte * 0.5
		}
		vao := l.x - fim
		switch {
		case b.Len() == 0:
			c = Celula{X0: l.x}
		case vao > l.fonte*1.5:
			fechar()
			c = Celula{X0: l.x}
		case vao > l.fonte*0.15 && !strings.HasSuffix(b.String(), " "):
			b.WriteByte(' ')
		}
		b.WriteString(l.s)
		fim = l.x + largura
		c.X1 = fim
	}
	fechar()
	return linha
}
