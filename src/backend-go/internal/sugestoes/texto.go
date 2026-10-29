// Pacote sugestoes é o SugestaoCategoriaService do C# (#78): sugere categorias pelo histórico (como você categorizou
// transações com descrição parecida; o que você escolheu à mão pesa mais) e por palavras-chave conhecidas, e o texto
// de uma regra "contém" para o grupo, com o aviso quando ela pegaria transações de outras categorias (#254).
//
// O que depende do texto (as palavras, a chave dos grupos, o texto da regra, o percentual) foi conferido contra o
// próprio C#: o gabarito testdata/sugestoes.json é gravado pela sonda das contas (internal/faturas/testdata/sonda-csharp).
package sugestoes

import (
	"math"
	"math/big"
	"slices"
	"strings"
	"unicode"
	"unicode/utf16"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/texto"
)

// conjunto monta um conjunto de palavras a partir de um texto separado por espaços.
func conjunto(palavras string) map[string]bool {
	c := map[string]bool{}
	for _, p := range strings.Fields(palavras) {
		c[p] = true
	}
	return c
}

// ruido são as palavras que não identificam o estabelecimento.
var ruido = conjunto(`PIX ENVIADO ENVIADA RECEBIDO RECEBIDA TRANSF TRANSFERENCIA COMPRA COMPRAS CARTAO CRED CREDITO DEB DEBITO
	PAG PAGTO PAGAMENTO PGTO TED DOC BOLETO PARC PARCELA PARCELADO NACIONAL INTERNACIONAL ELO VISA MASTERCARD MASTER LTDA
	EIRELI COM BRA BRASIL WWW HTTPS HTTP APP DES REM SAQUE QRCODE QR CODE AUT PARA DAS DOS COMERCIO SERVICOS SERV EMPRESA
	EPP ONLINE INTERNET AGENDADO AUTOMATICO CONTA VALOR REF NO NA EM DE DA DO E O A SAO PAULO SP RJ MG PR SC RS BR`)

// genericasParaRegra (#254): palavras de descrições de todo tipo. Servem para agrupar, mas sozinhas viram regra
// "contém" que pega o que não devia.
var genericasParaRegra = conjunto(`BANCO BCO BANK FATURA PAGAMENTOS RECEBIMENTO DEPOSITO APLICACAO RESGATE SALDO LOJA LOJAS
	MENSALIDADE SERVICO CENTRO GRUPO INSTITUICAO PAGADORA CLIENTE TITULAR MESMA TITULARIDADE OUTRA OUTROS DIVERSOS ESTORNO TAXA`)

// tamanho é o string.Length do C#: os chars UTF-16.
func tamanho(s string) int { return len(utf16.Encode([]rune(s))) }

// Palavras é o Palavras do C#: as palavras significativas da descrição (normalizada como nas regras, só letras, com 3
// chars UTF-16 ou mais e fora do ruído). O C# olha um char por vez: a letra fora do plano básico, como o "𝐀"
// matemático, chega em duas metades que não são letra, e vira espaço.
func Palavras(descricao string) []string {
	normalizado := texto.ParaRegras(descricao)
	var sb strings.Builder
	for _, r := range normalizado {
		if r <= 0xFFFF && unicode.IsLetter(r) {
			sb.WriteRune(r)
		} else {
			sb.WriteByte(' ')
		}
	}
	palavras := []string{}
	for _, p := range strings.Fields(sb.String()) {
		if tamanho(p) >= 3 && !ruido[p] {
			palavras = append(palavras, p)
		}
	}
	return palavras
}

// Chave é a de agrupamento: até as 3 primeiras palavras significativas.
func Chave(descricao string) string {
	palavras := Palavras(descricao)
	return strings.Join(palavras[:min(3, len(palavras))], " ")
}

// TextoRegra é o texto sugerido para a regra "contém", tirado das descrições do banco do grupo (o que a regra testa,
// #254): a primeira palavra que identifica (não genérica, com 4 letras ou mais) presente em quase todas (80%); curta,
// vai junto com a seguinte ("UBER EATS"). Sem palavra assim, o começo comum das descrições, se for longo e tiver alguma
// palavra que identifique; senão, vazio (sem regra).
func TextoRegra(descricoes []string) string {
	var textos []string
	for _, d := range descricoes {
		if t := texto.ParaRegras(d); t != "" && !slices.Contains(textos, t) {
			textos = append(textos, t)
		}
	}
	if len(textos) == 0 {
		return ""
	}
	listas := make([][]string, len(textos))
	for i, t := range textos {
		listas[i] = Palavras(t)
	}
	minimo := max(1, int(math.Ceil(float64(len(textos))*0.8)))
	contar := func(f func(int) bool) int {
		n := 0
		for i := range textos {
			if f(i) {
				n++
			}
		}
		return n
	}
	primeiras := listas[0]
	for i, p := range primeiras {
		if tamanho(p) < 4 || genericasParaRegra[p] || contar(func(j int) bool { return slices.Contains(listas[j], p) }) < minimo {
			continue
		}
		if tamanho(p) <= 4 && i+1 < len(primeiras) {
			dupla := p + " " + primeiras[i+1]
			if contar(func(j int) bool { return strings.Contains(textos[j], dupla) }) >= minimo {
				return dupla
			}
		}
		return p
	}
	// O começo comum só serve se tiver alguma palavra que identifique: "PIX ENVIADO" sozinho pegaria todo Pix.
	prefixo := prefixoComum(textos)
	if tamanho(prefixo) >= 10 && slices.ContainsFunc(Palavras(prefixo), func(p string) bool { return !genericasParaRegra[p] }) {
		return prefixo
	}
	return ""
}

// prefixoComum é o começo comum dos textos, cortado no fim da última palavra inteira. Os índices são de char UTF-16,
// como no C#.
func prefixoComum(textos []string) string {
	cs := make([][]uint16, len(textos))
	menor := math.MaxInt
	for i, t := range textos {
		cs[i] = utf16.Encode([]rune(t))
		menor = min(menor, len(cs[i]))
	}
	n := 0
	for n < menor && !slices.ContainsFunc(cs, func(c []uint16) bool { return c[n] != cs[0][n] }) {
		n++
	}
	prefixo := cs[0][:n]
	if n < len(cs[0]) && letraOuDigito(cs[0][n]) && !slices.ContainsFunc(cs, func(c []uint16) bool { return len(c) <= n }) {
		ultimo := -1
		for i, c := range prefixo {
			if c == ' ' {
				ultimo = i
			}
		}
		prefixo = prefixo[:max(0, ultimo)]
	}
	return strings.TrimSpace(string(utf16.Decode(prefixo)))
}

func letraOuDigito(c uint16) bool {
	r := rune(c)
	return !utf16.IsSurrogate(r) && (unicode.IsLetter(r) || unicode.Is(unicode.Nd, r))
}

// percentual é o "P0" do C# na cultura pt-BR: o valor exato do double vezes 100, arredondado para o par no empate
// (0,125 é "12%", e 0,005, que no binário fica um pouco acima, é "1%"), sem separador e colado no "%".
func percentual(razao float64) string {
	f := new(big.Float).SetPrec(256).SetFloat64(razao)
	f.Mul(f, big.NewFloat(100))
	return f.Text('f', 0) + "%"
}
