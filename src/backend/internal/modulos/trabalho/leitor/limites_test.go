package leitor

import (
	"bytes"
	"compress/zlib"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ledongthuc/pdf"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/leitor/ficticios"
)

// Os limites da leitura (revisão do #706): PDFs malformados de propósito, montados à mão. Nenhum pode travar a leitura
// nem estourar a memória: cada um vira um erro, e um holerite de verdade fica muito abaixo de todos os limites.

const courier = "<< /Type /Font /Subtype /Type1 /BaseFont /Courier /Encoding /WinAnsiEncoding /FirstChar 32 /LastChar 255 /Widths [" +
	"600 500 400 300 " + "600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 " +
	"600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 " +
	"600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 600 700 650 550] >>"

// fluxo é um objeto de fluxo; comprimido, com FlateDecode.
func fluxo(conteudo string, comprimido bool) string {
	if !comprimido {
		return fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(conteudo), conteudo)
	}
	var b bytes.Buffer
	z := zlib.NewWriter(&b)
	_, _ = z.Write([]byte(conteudo))
	_ = z.Close()
	return fmt.Sprintf("<< /Length %d /Filter /FlateDecode >>\nstream\n%s\nendstream", b.Len(), b.String())
}

// pagina é um PDF de uma página com a fonte F1 (o objeto 4) e o conteúdo (o 5); extras viram os objetos 6 em diante.
func pagina(fonte, conteudo string, extras ...string) []byte {
	return ficticios.Montar(append([]string{"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		fonte, conteudo}, extras...))
}

func lerJa(t *testing.T, dados []byte) ([][]Linha, error) {
	t.Helper()
	return lerComLimites(t.Context(), dados, "")
}

// A árvore de páginas: o ciclo, a fila de filhos e a profundidade não travam; os limites valem no número exato.
func TestArvoreDePaginasLimitada(t *testing.T) {
	t.Parallel()
	folha := "<< /Type /Page /Parent 2 0 R /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>"
	conteudo := fluxo("BT /F1 9 Tf 1 0 0 1 30 800 Tm (Holerite) Tj ET", false)
	// encadeado: a raiz (2) e n-1 nós "Pages" um dentro do outro (6, 7, ...), com a folha (3) no último.
	encadeado := func(n int) []byte {
		objs := []string{"<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [6 0 R] /Count 1 >>", folha, courier, conteudo}
		for i := range n - 1 {
			filho := fmt.Sprintf("%d 0 R", 7+i)
			if i == n-2 {
				filho = "3 0 R"
			}
			objs = append(objs, fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count 1 >>", filho))
		}
		if n == 1 {
			objs[1] = "<< /Type /Pages /Kids [3 0 R] /Count 1 >>"
		}
		return ficticios.Montar(objs)
	}
	// largo: a raiz com n-2 filhos que não são página (contam como nós) e a folha.
	largo := func(n int) []byte {
		kids := strings.TrimSpace(strings.Repeat("6 0 R ", n-2)) + " 3 0 R"
		return ficticios.Montar([]string{"<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [" + kids + "] /Count 1 >>",
			folha, courier, conteudo, "<< /Type /Outro >>"})
	}
	seisFolhas := ficticios.Montar([]string{"<< /Type /Catalog /Pages 2 0 R >>", // o Count mente: 1, com 6 folhas
		"<< /Type /Pages /Kids [3 0 R 3 0 R 3 0 R 3 0 R 3 0 R 3 0 R] /Count 1 >>", folha, courier, conteudo})
	cincoFolhas := bytes.Replace(seisFolhas, []byte("[3 0 R 3 0 R 3 0 R 3 0 R 3 0 R 3 0 R]"), []byte("[3 0 R 3 0 R 3 0 R 3 0 R 3 0 R]      "), 1)
	casos := []struct {
		nome    string
		dados   []byte
		err     error
		paginas int
	}{
		{"Pages filho dele mesmo (#706: o r.Page não terminava)", ficticios.Montar([]string{"<< /Type /Catalog /Pages 2 0 R >>",
			"<< /Type /Pages /Count 1 /Kids [2 0 R] >>"}), ErrIlegivel, 0},
		{"profundidade no limite", encadeado(profundidadeMaxima - 1), nil, 1},
		{"profundidade passa do limite", encadeado(profundidadeMaxima), ErrIlegivel, 0},
		{"nós no limite", largo(nosMaximo), nil, 1},
		{"nós passam do limite", largo(nosMaximo + 1), ErrIlegivel, 0},
		{"o Count mente: 5 folhas", cincoFolhas, nil, 5},
		{"o Count mente: 6 folhas", seisFolhas, ErrGrande, 0},
	}
	for _, c := range casos {
		paginas, err := lerJa(t, c.dados)
		if !errors.Is(err, c.err) || len(paginas) != c.paginas {
			t.Errorf("%s: %d páginas, err = %v; quero %d, %v", c.nome, len(paginas), err, c.paginas, c.err)
		}
	}
}

// O Parent em ciclo, sem Resources: o findInherited da biblioteca não terminava.
func TestParentEmCicloNaoTrava(t *testing.T) {
	t.Parallel()
	dados := ficticios.Montar([]string{"<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 6 0 R /Contents 5 0 R >>", courier, fluxo("BT ET", false), "<< /Parent 6 0 R >>"})
	if _, err := lerJa(t, dados); !errors.Is(err, ErrIlegivel) {
		t.Errorf("err = %v", err)
	}
	// Os Resources herdados do pai valem (o pai do pai, até o limite).
	herda := ficticios.Montar([]string{"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 /Resources << /Font << /F1 4 0 R >> >> >>",
		"<< /Type /Page /Parent 2 0 R /Contents 5 0 R >>", courier, fluxo("BT /F1 9 Tf 1 0 0 1 30 800 Tm (Herdado) Tj ET", false)})
	if p, err := lerJa(t, herda); err != nil || p[0][0].Texto() != "Herdado" {
		t.Errorf("herdado: %v %v", p, err)
	}
	// A página sem conteúdo não tem letras (e não lê os Resources).
	semConteudo := ficticios.Montar([]string{"<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [3 0 R 6 0 R] /Count 2 >>",
		"<< /Type /Page /Parent 2 0 R /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>", courier,
		fluxo("BT /F1 9 Tf 1 0 0 1 30 800 Tm (Com) Tj ET", false), "<< /Type /Page /Parent 7 0 R >>", "<< /Parent 7 0 R >>"})
	if p, err := lerJa(t, semConteudo); err != nil || len(p) != 2 || len(p[1]) != 0 {
		t.Errorf("sem conteúdo: %v %v", p, err)
	}
}

// A bomba de descompressão (#706: 143 KB abriam para 39 MB, e a leitura chegava a 12 GB de memória): o conteúdo aberto,
// o ToUnicode aberto e as letras têm teto, no número exato.
func TestBombaDeDescompressao(t *testing.T) {
	t.Parallel()
	// texto é um conteúdo de exatamente n bytes que escreve letras letras.
	texto := func(n, letras int) string {
		c := "BT /F1 9 Tf 1 0 0 1 30 800 Tm (" + strings.Repeat("A", letras) + ") Tj ET"
		return c + strings.Repeat(" ", n-len(c))
	}
	cmap := func(n int) string {
		c := "/CIDInit /ProcSet findresource begin 12 dict begin begincmap 1 begincodespacerange <00> <FF> endcodespacerange " +
			"1 beginbfchar <41> <0041> endbfchar endcmap"
		return c + strings.Repeat(" ", n-len(c))
	}
	comCmap := func(n int, conteudo string) []byte {
		return pagina("<< /Type /Font /Subtype /Type1 /BaseFont /Courier /ToUnicode 6 0 R >>", fluxo(conteudo, true), fluxo(cmap(n), true))
	}
	// Dois fluxos de conteúdo: o limite vale para a soma.
	dois := func(a, b int) []byte {
		return ficticios.Montar([]string{"<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
			"<< /Type /Page /Parent 2 0 R /Resources << /Font << /F1 4 0 R >> >> /Contents [5 0 R 6 0 R] >>", courier,
			fluxo(texto(a, 3), true), fluxo(strings.Repeat(" ", b), true)})
	}
	casos := []struct {
		nome  string
		dados []byte
		err   error
	}{
		{"conteúdo no limite", pagina(courier, fluxo(texto(conteudoMaximo, 3), true)), nil},
		{"conteúdo passa do limite", pagina(courier, fluxo(texto(conteudoMaximo+1, 3), true)), ErrPesado},
		{"conteúdo de 39 MB", pagina(courier, fluxo(texto(39<<20, 3), true)), ErrPesado},
		{"dois fluxos, a soma no limite", dois(conteudoMaximo/2, conteudoMaximo/2), nil},
		{"dois fluxos, a soma passa do limite", dois(conteudoMaximo/2, conteudoMaximo/2+1), ErrPesado},
		{"letras no limite", pagina(courier, fluxo(texto(letrasMaximo+100, letrasMaximo), true)), nil},
		{"letras passam do limite", pagina(courier, fluxo(texto(letrasMaximo+100, letrasMaximo+1), true)), ErrPesado},
		{"ToUnicode no limite", comCmap(fonteMaxima, "BT /F1 9 Tf (AAA) Tj ET"), nil},
		{"ToUnicode passa do limite", comCmap(fonteMaxima+1, "BT /F1 9 Tf (AAA) Tj ET"), ErrPesado},
	}
	for _, c := range casos {
		if _, err := lerJa(t, c.dados); !errors.Is(err, c.err) {
			t.Errorf("%s: err = %v, quero %v", c.nome, err, c.err)
		}
	}
	// Três fluxos de 400 KB: o limite vale para a soma de todos, não só a de dois.
	tres := ficticios.Montar([]string{"<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /Resources << /Font << /F1 4 0 R >> >> /Contents [5 0 R 6 0 R 7 0 R] >>", courier,
		fluxo(texto(400<<10, 3), true), fluxo(strings.Repeat(" ", 400<<10), true), fluxo(strings.Repeat(" ", 400<<10), true)})
	if _, err := lerJa(t, tres); !errors.Is(err, ErrPesado) {
		t.Errorf("três fluxos: %v", err)
	}
	// Muitos textos pequenos também param no teto: ele vale para a página, não para cada texto.
	muitos := "BT /F1 9 Tf " + strings.Repeat("("+strings.Repeat("A", letrasMaximo/4)+") Tj ", 5) + "ET"
	if _, err := lerJa(t, pagina(courier, fluxo(muitos, true))); !errors.Is(err, ErrPesado) {
		t.Errorf("muitos textos: %v", err)
	}
}

// A fonte cujo ToUnicode traduz um código num texto enorme: o teto vale antes de traduzir (a tradução de 5 letras daria
// 5 × 5000).
func TestToUnicodeQueAmplifica(t *testing.T) {
	t.Parallel()
	cmap := "/CIDInit /ProcSet findresource begin 12 dict begin begincmap 1 begincodespacerange <00> <FF> endcodespacerange " +
		"2 beginbfchar <41> <0041> <42> <" + strings.Repeat("0042", 5000) + "> endbfchar endcmap"
	fonte := "<< /Type /Font /Subtype /Type1 /BaseFont /Courier /ToUnicode 6 0 R >>"
	// Com 4 letras, o teto (4 × 5000) cabe; com 5, não, mesmo que o texto use só o A (o teto é a conta do pior caso).
	if p, err := lerJa(t, pagina(fonte, fluxo("BT /F1 9 Tf (AAAA) Tj ET", true), fluxo(cmap, true))); err != nil || p[0][0].Texto() != "AAAA" {
		t.Errorf("4 letras: %v %v", p, err)
	}
	if _, err := lerJa(t, pagina(fonte, fluxo("BT /F1 9 Tf (AAAAA) Tj ET", true), fluxo(cmap, true))); !errors.Is(err, ErrPesado) {
		t.Errorf("5 letras: %v", err)
	}
	if _, err := lerJa(t, pagina(fonte, fluxo("BT /F1 9 Tf (BBBBB) Tj ET", true), fluxo(cmap, true))); !errors.Is(err, ErrPesado) {
		t.Errorf("5 letras que amplificam: %v", err)
	}
}

func TestDestinoMaximo(t *testing.T) {
	t.Parallel()
	casos := map[string]int{
		"":                             1,
		"sem texto":                    1,
		"<0041>":                       1,
		"<00410042>":                   2,
		"<004100420043>":               3,
		"<0041004200>":                 3, // dígitos a mais arredondam para cima
		"(ab)":                         1,
		"(abc)":                        2,
		"(abcd)":                       2,
		"<< /A <00410042> >>":          2, // o << do dicionário não é texto
		"<><00410042>":                 2, // o texto vazio
		"<0041> <004100420043>":        3,
		"<00410042004300440045":        5, // sem o fim: vai até o fim do arquivo
		"(abcdefgh":                    4,
		"<0041> <00410042004300440045": 5, // sem o fim, depois do começo
		"<0041> (abcdefghij":           5,
		"<0041> (abcdefghij) <>":       5,
	}
	for cmap, quero := range casos {
		if got := destinoMaximo([]byte(cmap)); got != quero {
			t.Errorf("destinoMaximo(%q) = %d, quero %d", cmap, got, quero)
		}
	}
}

// A leitura confere o prazo a cada operador; e o comPrazo desiste da leitura presa, que fica com a vaga.
func TestPrazo(t *testing.T) {
	t.Parallel()
	vencido, cancelar := context.WithCancel(t.Context())
	cancelar()
	if _, err := lerComLimites(vencido, pagina(courier, fluxo("BT ET", false)), ""); !errors.Is(err, ErrDemorou) {
		t.Errorf("depois do prazo: %v", err)
	}
	minha := make(chan struct{}, 1)
	solta := make(chan struct{})
	presa := func(context.Context) ([][]Linha, error) { <-solta; return nil, nil }
	inicio := time.Now()
	if _, err := comPrazo(minha, 50*time.Millisecond, presa); !errors.Is(err, ErrDemorou) {
		t.Errorf("presa: %v", err)
	}
	if d := time.Since(inicio); d < time.Second || d > 3*time.Second {
		t.Errorf("desistiu em %s: quero o prazo e um segundo", d)
	}
	if _, err := comPrazo(minha, 50*time.Millisecond, presa); !errors.Is(err, ErrOcupado) {
		t.Errorf("com a vaga ocupada: %v", err)
	}
	close(solta)
	pronta := func(context.Context) ([][]Linha, error) { return [][]Linha{{}}, nil }
	if p, err := comPrazo(minha, time.Second, pronta); err != nil || len(p) != 1 {
		t.Errorf("solta, a vaga volta: %v %v", p, err)
	}
}

// Operador com os operandos errados é PDF malformado, como na biblioteca (que entra em pânico).
func TestOperandosErrados(t *testing.T) {
	t.Parallel()
	for _, c := range []string{"1 2 3 4 5 cm", "Q", "1 2 Tc", "1 TD", "1 Td", "/F1 Tf", "1 2 (A) \" (B) \"", "(A) (B) '", "(A) (B) Tj",
		"TJ", "1 2 TL", "1 2 3 4 5 Tm", "1 2 Ts", "1 2 Tz", "1 2 Tr", "1 2 Tw"} {
		if _, err := lerJa(t, pagina(courier, fluxo("BT /F1 9 Tf "+c+" ET", false))); !errors.Is(err, ErrIlegivel) {
			t.Errorf("%q: err = %v", c, err)
		}
	}
}

// A conta das posições é a do Content() da biblioteca, em todos os operadores de texto (o leitor depende dela para
// achar as linhas e as colunas).
func TestMesmaContaDaBiblioteca(t *testing.T) {
	t.Parallel()
	cmap := "/CIDInit /ProcSet findresource begin 12 dict begin begincmap 1 begincodespacerange <00> <FF> endcodespacerange " +
		"2 beginbfchar <41> <0041> <66> <00660069> endbfchar endcmap" // o f vira "fi": duas letras de um código
	conteudos := []string{
		"(Sem fonte) Tj BT (Antes) Tj ET",
		"q 1 0 0 1 10 20 cm BT /F1 10 Tf 12 TL 100 700 Td (Ab) Tj 2 Tc (Cd) Tj 0 Tc T* (Ef) Tj 5 -14 TD (Gh) Tj (Ij) ' " +
			"1 0.5 (Kl) \" 150 Tz [(M) -500 (n) 250 (o)] TJ 100 Tz 3 Ts (Op) Tj 0 Ts 1 0 0 1 300 400 Tm (Qr) Tj 0 Tr 0 Tw ET Q " +
			"BT /F1 8 Tf 50 50 Td (St) Tj ET 2 0 0 2 0 0 cm BT /F1 8 Tf 50 50 Td (Uv \040 ) Tj ET",
		"BT /F2 11 Tf 1 0 0 1 30 700 Tm (Afa) Tj /F1 9 Tf (Aa) Tj q /F2 9 Tf Q (Ab) Tj ET",
	}
	for _, c := range conteudos {
		dados := ficticios.Montar([]string{"<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
			"<< /Type /Page /Parent 2 0 R /Resources << /Font << /F1 4 0 R /F2 6 0 R >> >> /Contents 5 0 R >>", courier,
			fluxo(c, true), "<< /Type /Font /Subtype /Type1 /BaseFont /Courier /FirstChar 65 /LastChar 66 /Widths [500 700] /ToUnicode 7 0 R >>",
			fluxo(cmap, false)})
		r, err := pdf.NewReader(bytes.NewReader(dados), int64(len(dados)))
		if err != nil {
			t.Fatal(err)
		}
		esperado := r.Page(1).Content().Text
		obtido := letrasDa(t.Context(), r.Page(1).V)
		if len(obtido) != len(esperado) || len(esperado) == 0 {
			t.Fatalf("%q: %d letras, a biblioteca %d", c, len(obtido), len(esperado))
		}
		for i, e := range esperado {
			o := obtido[i]
			if o.s != e.S || o.x != e.X || o.y != e.Y || o.w != e.W || o.fonte != e.FontSize {
				t.Errorf("%q, letra %d: %+v, a biblioteca %+v", c, i, o, e)
			}
		}
	}
}
