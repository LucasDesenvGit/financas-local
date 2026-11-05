package tipos

import (
	"math"
	"math/big"

	"github.com/shopspring/decimal"
)

// dois64 é 2^64 em float64: o que o (double)decimal do .NET usa para juntar as duas partes do número.
const dois64 = 1.8446744073709552e+19

// mascara64 são os 64 bits de baixo do número sem a vírgula.
var mascara64 = new(big.Int).SetUint64(math.MaxUint64)

// ParaFloat64 é o (double)decimal do .NET. Não é para dinheiro: existe porque o fator da Selic é calculado em double no
// C#, e o mesmo número só sai com as mesmas contas. O float64(...) no meio impede o compilador de fundir a multiplicação
// com a soma numa só operação (FMA), que arredondaria uma vez a menos.
func (d Decimal) ParaFloat64() float64 {
	c := new(big.Int).Abs(d.v.Coefficient())
	baixo := new(big.Int).And(c, mascara64).Uint64()
	alto := new(big.Int).Rsh(c, 64).Uint64()
	f := (float64(baixo) + float64(float64(alto)*dois64)) / math.Pow10(int(d.Escala()))
	if d.v.IsNegative() {
		return -f
	}
	return f
}

// DecimalDeFloat64 é o (decimal)double do .NET (o VarDecFromR8), que guarda só 15 dígitos significativos: arredonda o
// número para um inteiro de 15 dígitos e tira os zeros à direita. Fora do decimal (NaN e infinito inclusive), ErrEstouro.
func DecimalDeFloat64(x float64) (Decimal, error) {
	// A escala máxima é 10^28, pouco mais de 2^93: com o expoente abaixo de -94, o número arredonda para zero.
	expoente := int(math.Float64bits(x)>>52&0x7FF) - 1022
	if expoente < -94 {
		return Zero, nil
	}
	if expoente > 96 {
		return Decimal{}, ErrEstouro
	}
	dobro := math.Abs(x)
	// log10(2) * 2^16 = 19728,3: quantas casas faltam para o número ter 15 dígitos.
	casas := 14 - (expoente * 19728 >> 16)
	switch {
	case casas >= 0:
		casas = min(casas, casasMaximas)
		dobro *= math.Pow10(casas)
	case casas != -1 || dobro >= 1e15:
		dobro /= math.Pow10(-casas)
	default:
		casas = 0
	}
	if dobro < 1e14 && casas < casasMaximas {
		dobro *= 10
		casas++
	}
	inteiro := uint64(math.RoundToEven(dobro))
	if inteiro == 0 {
		return Zero, nil
	}
	if casas < 0 {
		m := new(big.Int).Mul(new(big.Int).SetUint64(inteiro), potencia(-casas))
		return decimalDoSinal(m, x, 0), nil
	}
	for limite := min(casas, 14); limite > 0 && inteiro%10 == 0; limite-- {
		inteiro /= 10
		casas--
	}
	return decimalDoSinal(new(big.Int).SetUint64(inteiro), x, casas), nil
}

func decimalDoSinal(m *big.Int, x float64, casas int) Decimal {
	if math.Signbit(x) {
		m.Neg(m)
	}
	return Decimal{v: decimal.NewFromBigInt(m, int32(-casas))}
}

// ehEspaco é o espaço do parser de números do .NET: só o espaço e os controles de \t a \r.
func ehEspaco(c byte) bool { return c == ' ' || (c >= '\t' && c <= '\r') }

// LerNumero é o decimal.TryParse com NumberStyles.Number e a cultura invariante: espaços nas pontas, um sinal antes ou
// depois do número, ponto decimal e vírgula de milhar depois do primeiro dígito. Casas demais arredondam pelo método
// bancário, e o número que não cabe no decimal do C# é recusado.
func LerNumero(texto string) (Decimal, bool) {
	i, n := 0, len(texto)
	for i < n && ehEspaco(texto[i]) {
		i++
	}
	negativo, comSinal := false, false
	if i < n && (texto[i] == '-' || texto[i] == '+') {
		negativo, comSinal = texto[i] == '-', true
		i++
	}
	var digitos []byte
	casas, ponto := 0, false
numero:
	for ; i < n; i++ {
		c := texto[i]
		switch {
		case c >= '0' && c <= '9':
			digitos = append(digitos, c)
			if ponto {
				casas++
			}
		case c == '.' && !ponto:
			ponto = true
		case c == ',' && len(digitos) > 0 && !ponto:
		default:
			break numero
		}
	}
	if len(digitos) == 0 {
		return Decimal{}, false
	}
	for i < n && ehEspaco(texto[i]) {
		i++
	}
	if i < n && !comSinal && (texto[i] == '-' || texto[i] == '+') {
		negativo = texto[i] == '-'
		i++
		for i < n && ehEspaco(texto[i]) {
			i++
		}
	}
	for i < n && texto[i] == 0 {
		i++
	}
	if i != n {
		return Decimal{}, false
	}
	m, _ := new(big.Int).SetString(string(digitos), 10)
	if negativo {
		m.Neg(m)
	}
	arredondado, escala, ok := caber(m, casas)
	if !ok {
		return Decimal{}, false
	}
	return Decimal{v: decimal.NewFromBigInt(arredondado, int32(-escala))}, true
}

// caber arredonda o número m / 10^casas pelo método bancário para a maior escala, até 28, em que ele cabe nos 96 bits do
// decimal do C#, como as contas do C# fazem. Tira as casas sempre a partir do número inteiro, para não arredondar duas
// vezes. false se nem sem casas ele couber.
func caber(m *big.Int, casas int) (*big.Int, int, bool) {
	for escala := min(casas, casasMaximas); escala >= 0; escala-- {
		arredondado := dividirArredondandoAoPar(m, potencia(casas-escala))
		if new(big.Int).Abs(arredondado).Cmp(limite96) < 0 {
			return arredondado, escala, true
		}
	}
	return nil, 0, false
}
