// Pacote reservas é o ReservaService do C# (#82): as reservas ("cofrinhos", "caixinhas") das contas do Open Finance.
// Quando a instituição informa o valor reservado (#168, ADR 0037), ele vale. Senão, o saldo é estimado pelos lançamentos
// de entrada e saída de cada reserva no extrato da conta (o Mercado Pago, ADR 0010), a partir da conferência que você
// fez no app da instituição. O Méli Dólar do Mercado Pago (#169, ADR 0038) é uma reserva em dólar, convertida pela PTAX.
//
// Fica num pacote próprio porque as metas (#79), o painel (#80), a conferência do banco (#445) e a carteira (#82) usam.
package reservas

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

const (
	// TamanhoMaximoNome é o tamanho do nome de uma reserva (em chars UTF-16, o string.Length do C#).
	TamanhoMaximoNome = 100
	// DiasConferenciaAntiga: conferência mais velha que isso é sinalizada para ser refeita.
	DiasConferenciaAntiga = 30
	// toleranciaInicioJanela: a reserva cujo primeiro lançamento está até estes dias depois do primeiro lançamento
	// importado da conta provavelmente já tinha dinheiro antes da janela importada.
	toleranciaInicioJanela = 7
	// NomeMeliDolar e MoedaMeliDolar: o Méli Dólar (#169, ADR 0038), que o Open Finance não informa.
	NomeMeliDolar  = "Méli Dólar"
	MoedaMeliDolar = "USD"
	// NomeRestanteDoBanco é o nome da linha com a parte do total informado pelo banco que não está em nenhuma reserva
	// (#237).
	NomeRestanteDoBanco = "Reservado"
)

// prefixos (do extrato) que identificam o dinheiro indo para uma reserva ou voltando dela.
var prefixos = []string{"Dinheiro reservado ", "Dinheiro retirado "}

// semPrefixo devolve o resto da descrição depois do prefixo de reserva (o StartsWith OrdinalIgnoreCase do C#).
func semPrefixo(descricao string) (string, bool) {
	for _, p := range prefixos {
		if len(descricao) >= len(p) && strings.EqualFold(descricao[:len(p)], p) {
			return descricao[len(p):], true
		}
	}
	return "", false
}

// EhMovimentoDeReserva: dinheiro indo para uma reserva ou voltando dela ("Dinheiro reservado …", "Dinheiro retirado
// …"), pelo começo da descrição, sem diferença de maiúsculas.
func EhMovimentoDeReserva(descricao *string) bool {
	if descricao == nil {
		return false
	}
	_, ok := semPrefixo(*descricao)
	return ok
}

// ExtrairNome é o nome da reserva a partir da descrição do lançamento (aparado, até 100 chars UTF-16), ou nil se não
// for movimento de reserva.
func ExtrairNome(descricao string) *string {
	if strings.TrimFunc(descricao, unicode.IsSpace) == "" {
		return nil
	}
	resto, ok := semPrefixo(descricao)
	if !ok {
		return nil
	}
	nome := respostas.CortarUtf16(strings.TrimFunc(resto, unicode.IsSpace), TamanhoMaximoNome)
	if nome == "" {
		return nil
	}
	return &nome
}

// EhMeliDolar: compra ou venda de Méli Dólar ("Compra de Meli Dólar", com ou sem acento).
func EhMeliDolar(descricao string) bool {
	return strings.Contains(texto.ParaRegras(descricao), "MELI DOLAR")
}

func ehNomeMeliDolar(nome string) bool { return texto.ParaRegras(nome) == "MELI DOLAR" }

// quantidadeMusd é o Regex "MUSD\s*(\d(?:[\d.,]*\d)?)" do C#, com IgnoreCase e CultureInvariant. O \d e o \s do .NET
// são de Unicode, e o (?i) do RE2 iria além (o "ſ" casaria com o "s"): as classes e as letras ficam escritas por
// extenso.
var quantidadeMusd = regexp.MustCompile(`[Mm][Uu][Ss][Dd][\t\n\v\f\r\x{85}\p{Z}]*(\p{Nd}(?:[\p{Nd}.,]*\p{Nd})?)`)

// QuantidadeNaDescricao são os dólares que a descrição informa ("Venda de Meli Dólar MUSD 32,09161263"), ou nil.
// Vírgula é a casa decimal; sem vírgula, um ponto seguido de 3 dígitos é de milhar. Só vale número maior que zero.
func QuantidadeNaDescricao(descricao string) *tipos.Decimal {
	m := quantidadeMusd.FindStringSubmatch(descricao)
	if m == nil {
		return nil
	}
	numero := m[1]
	if strings.Contains(numero, ",") {
		numero = strings.ReplaceAll(strings.ReplaceAll(numero, ".", ""), ",", ".")
	} else if strings.Count(numero, ".") != 1 || len(numero)-strings.Index(numero, ".")-1 == 3 {
		numero = strings.ReplaceAll(numero, ".", "")
	}
	// O decimal.TryParse invariante lê só os dígitos de 0 a 9 e recusa o que não cabe no decimal do C#.
	q, ok := tipos.LerNumero(numero)
	if !ok || !q.Decimal().IsPositive() {
		return nil
	}
	return &q
}
