package tipos

import (
	"errors"
	"math/big"

	"github.com/shopspring/decimal"
)

// Aritmética com o mesmo resultado do decimal do C#, escala inclusive (#81). O gabarito é o
// testdata/csharp.json, gravado pela sonda testdata/sonda-csharp com o .NET da API.
//
// A escala aparece no JSON (o System.Text.Json escreve 250.0, e não 250), então não basta o valor estar certo: o
// shopspring/decimal divide com 16 casas e arredonda "para cima" no .5, e o C# não faz nem um nem outro.

// ErrDivisaoPorZero é a DivideByZeroException do C#.
var ErrDivisaoPorZero = errors.New("divisão por zero")

// ErrEstouro é a OverflowException do C#: o resultado não cabe nos 96 bits do decimal.
var ErrEstouro = errors.New("o resultado não cabe num decimal do C#")

// limite96 é 2^96: o decimal do C# guarda o número sem a vírgula em 96 bits.
var limite96 = new(big.Int).Lsh(big.NewInt(1), 96)

var dez = big.NewInt(10)

// Dividir é a divisão do decimal do C# (a / b), com a mesma escala no resultado:
//   - a escala "natural" é a de a menos a de b (1.00 / 1 = 1.00; 1 / 1.00 = 1);
//   - quando ela não basta, a divisão continua até a maior escala, até 28, em que o número cabe em 96 bits
//     (1 / 3 = 0.3333333333333333333333333333; 22 / 7 tem 29 dígitos), com a última casa arredondada pelo método
//     bancário (5 / 2e28 = 2e-28);
//   - os zeros à direita saem, mas só até a escala natural (10 / 4 = 2.5; 123.4500 / 1.5 = 82.300).
func Dividir(a, b Decimal) (Decimal, error) {
	if b.v.IsZero() {
		return Decimal{}, ErrDivisaoPorZero
	}
	natural := max(int(a.Escala())-int(b.Escala()), 0)

	// a / b = (ma / 10^sa) / (mb / 10^sb) = (ma * 10^sb) / (mb * 10^sa).
	num := new(big.Int).Mul(a.v.Coefficient(), potencia(int(-b.v.Exponent())))
	den := new(big.Int).Mul(b.v.Coefficient(), potencia(int(-a.v.Exponent())))
	if a.v.Exponent() > 0 || b.v.Exponent() > 0 {
		// DeDecimal nunca deixa expoente positivo; um Decimal{} montado à mão poderia ter.
		return Decimal{}, errors.New("decimal com expoente positivo")
	}

	for escala := casasMaximas; escala >= 0; escala-- {
		m := dividirArredondandoAoPar(new(big.Int).Mul(num, potencia(escala)), den)
		if new(big.Int).Abs(m).Cmp(limite96) >= 0 {
			continue // não cabe: uma casa a menos
		}
		// Tira os zeros à direita até a escala natural.
		resto := new(big.Int)
		for escala > natural {
			q, r := new(big.Int).QuoRem(m, dez, resto)
			if r.Sign() != 0 {
				break
			}
			m = q
			escala--
		}
		return Decimal{v: decimal.NewFromBigInt(m, int32(-escala))}, nil
	}
	return Decimal{}, ErrEstouro
}

// Multiplicar é a multiplicação do decimal do C#: a escala é a soma das escalas (10.0 * 5.4123 = 54.12300), até 28.
func Multiplicar(a, b Decimal) Decimal { return DeDecimal(a.v.Mul(b.v)) }

// Somar e Subtrair mantêm a maior escala, como no C# (1.5 + 2.25 = 3.75; 2.50 - 1 = 1.50).
func Somar(a, b Decimal) Decimal    { return DeDecimal(a.v.Add(b.v)) }
func Subtrair(a, b Decimal) Decimal { return DeDecimal(a.v.Sub(b.v)) }

// Maior é o Math.Max do decimal do C#: no empate devolve o primeiro, com a escala dele (Max(100, 100.0) = 100).
func Maior(a, b Decimal) Decimal {
	if a.v.Cmp(b.v) >= 0 {
		return a
	}
	return b
}

// Arredondar é o Math.Round(x, casas) do C#, sem o MidpointRounding: arredonda pelo método bancário (2.565 →
// 2.56; 0.125 → 0.12) e só mexe quando o valor tem mais casas que o pedido (250.0 continua 250.0, não 250.00).
func Arredondar(d Decimal, casas int32) Decimal {
	if d.Escala() <= casas {
		return d
	}
	return Decimal{v: d.v.RoundBank(casas)}
}

// dividirArredondandoAoPar divide n por d e arredonda o quociente pelo método bancário (o empate vai para o par).
func dividirArredondandoAoPar(n, d *big.Int) *big.Int {
	q, r := new(big.Int).QuoRem(n, d, new(big.Int))
	if r.Sign() == 0 {
		return q
	}
	// Compara 2*|r| com |d|: maior arredonda para longe do zero; igual é empate.
	dobro := new(big.Int).Abs(r)
	dobro.Lsh(dobro, 1)
	cmp := dobro.Cmp(new(big.Int).Abs(d))
	if cmp > 0 || (cmp == 0 && q.Bit(0) == 1) {
		if (n.Sign() < 0) != (d.Sign() < 0) {
			q.Sub(q, big.NewInt(1))
		} else {
			q.Add(q, big.NewInt(1))
		}
	}
	return q
}

func potencia(n int) *big.Int {
	if n <= 0 {
		return big.NewInt(1)
	}
	return new(big.Int).Exp(dez, big.NewInt(int64(n)), nil)
}
