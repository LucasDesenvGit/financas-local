package leitor

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"
)

// A leitura do conteúdo das páginas, com limites (revisão do #706). O PDF vem de quem usa o site, e um PDF malformado
// trava ou estoura a memória da biblioteca: a árvore de páginas com ciclo (o r.Page dela não termina), o conteúdo
// comprimido que abre para dezenas de MB (o Content() dela guarda uma struct por letra, sem teto) e a fonte cujo
// ToUnicode traduz um código num texto enorme. Aqui a árvore é percorrida com limite, o tamanho aberto do conteúdo e
// das fontes é conferido antes de ler, e o texto sai letra a letra, com a mesma conta do Content() da biblioteca (a
// posição de cada letra), um teto de letras e um prazo. Todos os limites ficam muito acima de um holerite de verdade.
const (
	profundidadeMaxima = 8         // níveis da árvore de páginas e da cadeia de Parent (um holerite tem 1 ou 2)
	nosMaximo          = 64        // nós visitados na árvore de páginas
	conteudoMaximo     = 1 << 20   // bytes do conteúdo aberto de uma página (um holerite tem uns 20 KB)
	fonteMaxima        = 128 << 10 // bytes do ToUnicode aberto de uma fonte (o de uma fonte inteira tem uns 60 KB)
	letrasMaximo       = 20000     // letras de uma página (um holerite tem umas 3 mil)
	prazo              = 5 * time.Second
)

// Erros dos limites da leitura.
var (
	ErrPesado  = errors.New("o conteúdo do PDF passa do limite do leitor")
	ErrDemorou = errors.New("o PDF demorou demais para ler")
	ErrOcupado = errors.New("o leitor está lendo outro PDF")
)

// vaga é a leitura de um PDF por vez no site: a que trava (um laço da biblioteca que o prazo não alcança) ocupa uma vaga
// só, e não uma a cada tentativa.
var vaga = make(chan struct{}, 1)

// falha é o erro que interrompe a leitura de dentro da biblioteca (o Interpret não tem como parar): vai por panic e
// volta como erro no recover do ler.
type falha struct{ err error }

func falhar(err error) { panic(falha{err}) }

type resultado struct {
	paginas [][]Linha
	err     error
}

// comPrazo roda a leitura numa goroutine, uma por vez (a vaga), e desiste de esperar por ela depois do prazo (mais um
// segundo de folga). A leitura confere o prazo sozinha, pelo contexto; a folga só pega o que ela não alcança (um laço
// da biblioteca ao abrir o PDF): aí a goroutine continua presa, com a vaga, e as próximas leituras recebem ErrOcupado
// até o site reiniciar. Sem a vaga na espera do prazo, também ErrOcupado.
func comPrazo(vaga chan struct{}, prazo time.Duration, ler func(ctx context.Context) ([][]Linha, error)) ([][]Linha, error) {
	espera := time.NewTimer(prazo)
	defer espera.Stop()
	select {
	case vaga <- struct{}{}:
	case <-espera.C:
		return nil, ErrOcupado
	}
	feito := make(chan resultado, 1)
	ctx, cancelar := context.WithTimeout(context.Background(), prazo)
	go func() {
		defer func() { <-vaga }()
		defer cancelar()
		p, err := ler(ctx)
		feito <- resultado{p, err}
	}()
	desiste := time.NewTimer(prazo + time.Second)
	defer desiste.Stop()
	select {
	case r := <-feito:
		return r.paginas, r.err
	case <-desiste.C:
		return nil, ErrDemorou
	}
}

// lerComLimites abre o PDF e tira as letras de cada página, até o limite. Pânico da biblioteca (PDF malformado) vira
// ErrIlegivel, sem o detalhe: ele pode ter texto do PDF (#228, nada do holerite no log).
func lerComLimites(ctx context.Context, dados []byte, senha string) (paginas [][]Linha, err error) {
	defer func() {
		if r := recover(); r != nil {
			paginas, err = nil, ErrIlegivel
			if f, ok := r.(falha); ok {
				err = f.err
			}
		}
	}()
	tentativas := 0
	r, err := pdf.NewReaderEncrypted(bytes.NewReader(dados), int64(len(dados)), func() string {
		tentativas++
		if tentativas > 1 {
			return "" // a senha errada não vira um laço
		}
		return senha
	})
	switch {
	case errors.Is(err, pdf.ErrInvalidPassword):
		return nil, ErrSenha
	case err != nil:
		return nil, ErrIlegivel
	}
	if r.NumPage() > PaginasMaximo {
		return nil, ErrGrande
	}
	folhas := paginasDa(r.Trailer().Key("Root").Key("Pages"))
	if len(folhas) > PaginasMaximo {
		return nil, ErrGrande
	}
	algumTexto := false
	for _, folha := range folhas {
		letras := letrasDa(ctx, folha)
		for _, l := range letras {
			algumTexto = algumTexto || strings.TrimSpace(l.s) != ""
		}
		paginas = append(paginas, Agrupar(letras))
	}
	if !algumTexto {
		return nil, ErrSemTexto
	}
	return paginas, nil
}

// paginasDa são as páginas da árvore, na ordem, como o r.Page da biblioteca acha (um nó "Pages" tem filhos; um "Page" é
// a folha), mas com a profundidade e os nós limitados: um ciclo (um Pages filho dele mesmo) vira ErrIlegivel.
func paginasDa(raiz pdf.Value) []pdf.Value {
	var folhas []pdf.Value
	nos := 0
	var descer func(no pdf.Value, nivel int)
	descer = func(no pdf.Value, nivel int) {
		nos++
		if nivel > profundidadeMaxima || nos > nosMaximo {
			falhar(ErrIlegivel)
		}
		switch no.Key("Type").Name() {
		case "Page":
			folhas = append(folhas, no)
		case "Pages":
			kids := no.Key("Kids")
			for i := range kids.Len() {
				descer(kids.Index(i), nivel+1)
			}
		}
	}
	descer(raiz, 1)
	return folhas
}

// herdado é a chave da página ou do primeiro Parent que a tem (como o findInherited da biblioteca), com a cadeia
// limitada: um Parent em ciclo vira ErrIlegivel.
func herdado(pagina pdf.Value, chave string) pdf.Value {
	v := pagina
	for range profundidadeMaxima {
		if v.IsNull() {
			return v
		}
		if r := v.Key(chave); !r.IsNull() {
			return r
		}
		v = v.Key("Parent")
	}
	falhar(ErrIlegivel)
	return pdf.Value{}
}

// aberto lê o fluxo já aberto (descomprimido e decifrado) até o limite, sem ler nem guardar mais que ele, e escreve em
// destino (io.Discard quando só o tamanho importa). Passou do limite: ErrPesado. O fluxo que falha no meio vale o que
// deu para ler, como na biblioteca.
func aberto(v pdf.Value, limite int64, destino io.Writer) int64 {
	rd := v.Reader()
	defer rd.Close()
	n, _ := io.Copy(destino, io.LimitReader(rd, limite+1))
	if n > limite {
		falhar(ErrPesado)
	}
	return n
}

// fonte é uma fonte da página, com o tradutor dos códigos e o maior texto que um código pode virar.
type fonte struct {
	f       *pdf.Font
	enc     pdf.TextEncoding
	destino int
}

// semTraducao é o tradutor sem tabela da biblioteca: o código é o próprio texto.
type semTraducao struct{}

func (semTraducao) Decode(raw string) string { return raw }

// destinoMaximo é o maior texto, em letras, que um código da fonte pode virar pelo ToUnicode: o maior texto da tabela,
// com 4 dígitos por letra em hexadecimal (UTF-16) ou 2 bytes por letra entre parênteses. É um teto, não a conta exata:
// nenhum tradutor da biblioteca dá mais letras por byte que isso (os sem tabela dão uma letra por byte, no máximo).
func destinoMaximo(cmap []byte) int {
	maior := 1
	for i := 0; i < len(cmap); i++ {
		fim, por := byte('>'), 4
		switch {
		case cmap[i] == '(':
			fim, por = ')', 2
		case cmap[i] != '<':
			continue
		case bytes.HasPrefix(cmap[i:], []byte("<<")):
			i++ // os dois < do dicionário
			continue
		}
		n := bytes.IndexByte(cmap[i+1:], fim) // o tamanho do texto, sem os delimitadores
		if n < 0 {
			n = len(cmap) - i - 1 // sem o fim: vai até o fim do arquivo
		}
		maior = max(maior, (n+por-1)/por)
		i += n + 1
	}
	return maior
}

// estado é o estado gráfico do texto (PDF 32000-1, §9.3), só o que a posição das letras usa, como no Content().
type estado struct {
	tc, th, tl, tfs, trise Pontos
	tf                     *pdf.Font
	tm, tlm, ctm           matriz
}

type matriz [3][3]Pontos

// pontos é o operando como número: posição, tamanho ou escala na página (geometria, não dinheiro).
func pontos(v pdf.Value) Pontos { return v.Float64() }

var identidade = matriz{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}

func (x matriz) vezes(y matriz) matriz {
	var z matriz
	for i := range 3 {
		for j := range 3 {
			for k := range 3 {
				z[i][j] += x[i][k] * y[k][j]
			}
		}
	}
	return z
}

// argumentos tira os operandos da pilha, o primeiro no começo (como o popArgs da biblioteca).
func argumentos(stk *pdf.Stack) []pdf.Value {
	args := make([]pdf.Value, stk.Len())
	for i := len(args) - 1; i >= 0; i-- {
		args[i] = stk.Pop()
	}
	return args
}

// exigir confere o número de operandos: o operador com o número errado é PDF malformado (a biblioteca entra em pânico).
func exigir(args []pdf.Value, n int) {
	if len(args) != n {
		falhar(ErrIlegivel)
	}
}

// letrasDa tira as letras da página com a posição de cada uma, pela mesma conta do Content() da biblioteca (que não
// aplica o espaço entre palavras, Tw, e não lê o texto dos XObjects), com os limites de tamanho, de letras e de tempo.
func letrasDa(ctx context.Context, pagina pdf.Value) []letra {
	conteudo := pagina.Key("Contents")
	if pagina.IsNull() || conteudo.Kind() == pdf.Null {
		return nil
	}
	total := int64(0)
	if conteudo.Kind() == pdf.Array {
		for i := range conteudo.Len() {
			total += aberto(conteudo.Index(i), conteudoMaximo-total, io.Discard)
		}
	} else {
		aberto(conteudo, conteudoMaximo, io.Discard)
	}
	recursos := herdado(pagina, "Resources")
	fontes := map[string]*fonte{}
	acharFonte := func(nome string) *fonte {
		if f, ok := fontes[nome]; ok {
			return f
		}
		v := recursos.Key("Font").Key(nome)
		f := &fonte{f: &pdf.Font{V: v}, destino: 1}
		if tu := v.Key("ToUnicode"); tu.Kind() == pdf.Stream {
			var cmap bytes.Buffer
			aberto(tu, fonteMaxima, &cmap)
			f.destino = destinoMaximo(cmap.Bytes())
		}
		if f.enc = f.f.Encoder(); f.enc == nil {
			f.enc = semTraducao{}
		}
		fontes[nome] = f
		return f
	}

	var letras []letra
	var enc pdf.TextEncoding = semTraducao{}
	destino := 1
	g := estado{th: 1, ctm: identidade}
	var pilha []estado
	mostrar := func(s string) {
		// O teto antes de traduzir: a tradução de um texto curto pode ser enorme. O destino é um teto, então depois de
		// traduzir as letras nunca passam do limite.
		if len(s)*destino > letrasMaximo-len(letras) {
			falhar(ErrPesado)
		}
		n := 0
		for _, ch := range enc.Decode(s) {
			var w0 Pontos
			if n < len(s) && g.tf != nil {
				w0 = g.tf.Width(int(s[n]))
			}
			n++
			trm := matriz{{g.tfs * g.th, 0, 0}, {0, g.tfs, 0}, {0, g.trise, 1}}.vezes(g.tm).vezes(g.ctm)
			letras = append(letras, letra{s: string(ch), x: trm[2][0], y: trm[2][1], w: w0 / 1000 * trm[0][0], fonte: trm[0][0]})
			tx := (w0/1000*g.tfs + g.tc) * g.th
			g.tm = matriz{{1, 0, 0}, {0, 1, 0}, {tx, 0, 1}}.vezes(g.tm)
		}
	}
	matrizDe := func(args []pdf.Value) matriz {
		exigir(args, 6)
		var m matriz
		for i := range 6 {
			m[i/2][i%2] = pontos(args[i])
		}
		m[2][2] = 1
		return m
	}
	pdf.Interpret(conteudo, func(stk *pdf.Stack, op string) {
		if ctx.Err() != nil {
			falhar(ErrDemorou)
		}
		args := argumentos(stk)
		switch op {
		case "cm":
			g.ctm = matrizDe(args).vezes(g.ctm)
		case "q":
			pilha = append(pilha, g)
		case "Q":
			if len(pilha) == 0 {
				falhar(ErrIlegivel)
			}
			g, pilha = pilha[len(pilha)-1], pilha[:len(pilha)-1]
		case "BT":
			g.tm, g.tlm = identidade, identidade
		case "T*":
			g.tlm = matriz{{1, 0, 0}, {0, 1, 0}, {0, -g.tl, 1}}.vezes(g.tlm)
			g.tm = g.tlm
		case "Tc":
			exigir(args, 1)
			g.tc = pontos(args[0])
		case "TD", "Td":
			exigir(args, 2)
			if op == "TD" {
				g.tl = -pontos(args[1])
			}
			g.tlm = matriz{{1, 0, 0}, {0, 1, 0}, {pontos(args[0]), pontos(args[1]), 1}}.vezes(g.tlm)
			g.tm = g.tlm
		case "Tf":
			exigir(args, 2)
			f := acharFonte(args[0].Name())
			g.tf, enc, destino = f.f, f.enc, f.destino
			g.tfs = pontos(args[1])
		case "\"", "'", "Tj":
			if op == "\"" { // o espaço entre palavras não entra na conta (como na biblioteca), o entre letras sim
				exigir(args, 3)
				g.tc = pontos(args[1])
				args = args[2:]
			}
			exigir(args, 1)
			if op != "Tj" {
				g.tlm = matriz{{1, 0, 0}, {0, 1, 0}, {0, -g.tl, 1}}.vezes(g.tlm)
				g.tm = g.tlm
			}
			mostrar(args[0].RawString())
		case "TJ":
			if len(args) == 0 {
				falhar(ErrIlegivel)
			}
			v := args[0]
			for i := range v.Len() {
				x := v.Index(i)
				if x.Kind() == pdf.String {
					mostrar(x.RawString())
				} else {
					tx := -pontos(x) / 1000 * g.tfs * g.th
					g.tm = matriz{{1, 0, 0}, {0, 1, 0}, {tx, 0, 1}}.vezes(g.tm)
				}
			}
			mostrar("\n")
		case "Tr", "Tw": // não mudam a posição; só a conferência dos operandos
			exigir(args, 1)
		case "TL":
			exigir(args, 1)
			g.tl = pontos(args[0])
		case "Tm":
			g.tm = matrizDe(args)
			g.tlm = g.tm
		case "Ts":
			exigir(args, 1)
			g.trise = pontos(args[0])
		case "Tz":
			exigir(args, 1)
			g.th = pontos(args[0]) / 100
		}
	})
	return letras
}
