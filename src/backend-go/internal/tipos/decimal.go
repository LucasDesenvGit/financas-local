// Pacote tipos tem os tipos de dado que atravessam o banco e o JSON no mesmo formato do back-end .NET: decimal,
// data, data e hora e GUID. O formato de cada um foi confirmado pela sonda em testdata/sonda-efcore, que grava
// valores pelo mesmo EF Core da API e guarda o resultado em testdata/efcore.json (o gabarito dos testes).
package tipos

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/shopspring/decimal"
)

// casasMaximas é a escala máxima do decimal do C# (28 casas).
const casasMaximas = 28

// Decimal é o decimal do C#: dinheiro, quantidade de cotas, taxa. Nunca use float64 para esses valores.
//
// Ele guarda a escala (número de casas) como o decimal do C# faz: lido do banco como "-5000.0", sai no JSON como
// -5000.0, igual ao que o System.Text.Json escreve. No banco, grava no formato do EF Core ("0.0" + até 28 casas,
// sem zeros à direita).
type Decimal struct {
	v decimal.Decimal
}

// Zero é o decimal 0 sem casas.
var Zero = Decimal{}

// NovoDecimal lê um decimal em texto ("-5000.0", "12.50"), mantendo a escala.
func NovoDecimal(texto string) (Decimal, error) {
	v, err := decimal.NewFromString(strings.TrimSpace(texto))
	if err != nil {
		return Decimal{}, fmt.Errorf("decimal inválido %q: %w", texto, err)
	}
	return DeDecimal(v), nil
}

// DecimalDeInteiro cria um decimal sem casas.
func DecimalDeInteiro(n int64) Decimal { return Decimal{v: decimal.NewFromInt(n)} }

// DeDecimal embrulha um shopspring/decimal, por exemplo o resultado de uma conta.
func DeDecimal(v decimal.Decimal) Decimal {
	// O C# não tem expoente positivo: 1e3 vira 1000, sem casas.
	if v.Exponent() > 0 {
		v = decimal.NewFromBigInt(v.BigInt(), 0)
	}
	if v.Exponent() < -casasMaximas {
		v = v.RoundBank(casasMaximas)
	}
	return Decimal{v: v}
}

// Decimal devolve o valor para fazer contas com o shopspring/decimal. Lembre que Math.Round do C# arredonda
// pelo método bancário: use RoundBank, não Round, onde o C# não diz o MidpointRounding.
func (d Decimal) Decimal() decimal.Decimal { return d.v }

// Escala é o número de casas que o valor guarda (como decimal.Scale no C#).
func (d Decimal) Escala() int32 {
	if d.v.Exponent() >= 0 {
		return 0
	}
	return -d.v.Exponent()
}

// String escreve o valor com a escala que ele guarda, como decimal.ToString() no C#: 1.50 → "1.50".
func (d Decimal) String() string { return d.v.StringFixed(d.Escala()) }

// TextoNoBanco é como o EF Core grava o decimal no SQLite: pelo menos uma casa e nenhum zero à direita
// (1.50 → "1.5", 100 → "100.0").
func (d Decimal) TextoNoBanco() string {
	texto := d.v.String()
	if !strings.Contains(texto, ".") {
		texto += ".0"
	}
	return texto
}

// NaEscalaDoBanco é o decimal como o EF Core o lê de volta depois de gravá-lo: pelo menos uma casa e nenhum zero à
// direita (21.00 → 21.0, 1.50 → 1.5, 100 → 100.0, o mesmo que TextoNoBanco guarda). É a escala que o ef_sum do SQLite
// devolve para uma soma, e por isso a de um total que o C# calculava no SQL (#447).
func (d Decimal) NaEscalaDoBanco() Decimal {
	numero := new(big.Int).Set(d.v.Coefficient())
	expoente := d.v.Exponent() // DeDecimal nunca deixa expoente positivo
	resto := new(big.Int)
	for expoente < -1 {
		quociente, r := new(big.Int).QuoRem(numero, dez, resto)
		if r.Sign() != 0 {
			break
		}
		numero, expoente = quociente, expoente+1
	}
	if expoente == 0 {
		numero, expoente = numero.Mul(numero, dez), -1
	}
	return Decimal{v: decimal.NewFromBigInt(numero, expoente)}
}

// Value grava no banco no formato do EF Core (driver.Valuer).
func (d Decimal) Value() (driver.Value, error) { return d.TextoNoBanco(), nil }

// Scan lê do banco (sql.Scanner). Dinheiro gravado como REAL é recusado: float64 erra centavos, e o EF Core
// nunca grava assim.
func (d *Decimal) Scan(origem any) error {
	switch v := origem.(type) {
	case string:
		return d.lerTexto(v)
	case []byte:
		return d.lerTexto(string(v))
	case int64:
		*d = DecimalDeInteiro(v)
		return nil
	case nil:
		return fmt.Errorf("decimal NULL num campo obrigatório: use *tipos.Decimal")
	default:
		return fmt.Errorf("decimal gravado como %T, esperado TEXT", origem)
	}
}

func (d *Decimal) lerTexto(texto string) error {
	lido, err := NovoDecimal(texto)
	if err != nil {
		return err
	}
	*d = lido
	return nil
}

// MarshalJSON escreve um número sem aspas, com a escala guardada, como o System.Text.Json.
// (O shopspring/decimal sozinho escreveria entre aspas.)
func (d Decimal) MarshalJSON() ([]byte, error) { return []byte(d.String()), nil }

// UnmarshalJSON aceita número ou texto com número, como o ASP.NET (JsonSerializerDefaults.Web lê número em texto).
func (d *Decimal) UnmarshalJSON(dados []byte) error {
	texto := string(dados)
	if texto == "null" {
		return fmt.Errorf("decimal null num campo obrigatório: use *tipos.Decimal")
	}
	if strings.HasPrefix(texto, `"`) {
		if err := json.Unmarshal(dados, &texto); err != nil {
			return err
		}
	}
	return d.lerTexto(texto)
}
