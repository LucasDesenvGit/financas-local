package parcelas

import (
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// DiasRecentes é até quantos dias para trás uma parcela conta como "recente" (base da projeção).
const DiasRecentes = 45

// Parcela é uma parcela de compra no cartão, como o ParcelasFuturas.Parcela do C#.
type Parcela struct {
	Id            int64
	ContaId       int64
	Data          tipos.Data
	Valor         tipos.Decimal
	Numero, Total int64
	Descricao     string
}

// Futura é uma parcela por vir. JaNoBanco: o banco já mandou (data futura); senão, foi projetada Passos meses depois da
// parcela OrigemId (a fatura dela + Passos, #167).
type Futura struct {
	ContaId   int64
	Data      tipos.Data
	Valor     tipos.Decimal // positivo (o valor absoluto da parcela)
	JaNoBanco bool
	OrigemId  int64
	Passos    int
}

// chave reconhece a mesma compra: a conta, o total de parcelas, o valor em reais inteiros (o banco arredonda a 1ª
// parcela: 291,19 e depois 291,17; Math.Round(x, 0), pelo método bancário) e as 8 primeiras letras da descrição.
type chave struct {
	conta, total int64
	reais        string
	compra       string
}

func chaveDe(p Parcela) chave {
	reais := tipos.Arredondar(tipos.DeDecimal(p.Valor.Decimal().Abs()), 0).Decimal().String()
	return chave{p.ContaId, p.Total, reais, Compra(p.Descricao)}
}

// Projetar é o ParcelasFuturas.Projetar do C# (#165): as parcelas com data depois de hoje entram como são (o banco já
// mandou); de cada compra, só a parcela de maior número projeta as que faltam, mês a mês, as que caem depois de hoje.
// parcelas são as de qualquer conta com data a partir de hoje − DiasRecentes, inclusive as de data futura. As
// compras saem na ordem em que aparecem (o GroupBy do LINQ), e a de maior número é a primeira delas (o MaxBy).
func Projetar(parcelas []Parcela, hoje tipos.Data) []Futura {
	var validas []Parcela
	for _, p := range parcelas {
		if p.Total > 1 && p.Numero >= 1 && p.Numero <= p.Total && !p.Valor.Decimal().IsZero() {
			validas = append(validas, p)
		}
	}
	futuras := []Futura{}
	for _, p := range validas {
		if p.Data.NumeroDoDia() > hoje.NumeroDoDia() {
			futuras = append(futuras, Futura{p.ContaId, p.Data, tipos.DeDecimal(p.Valor.Decimal().Abs()), true, p.Id, 0})
		}
	}
	var ordem []chave
	ultimas := map[chave]Parcela{}
	for _, p := range validas {
		k := chaveDe(p)
		u, existe := ultimas[k]
		if !existe {
			ordem = append(ordem, k)
		}
		if !existe || p.Numero > u.Numero {
			ultimas[k] = p
		}
	}
	for _, k := range ordem {
		u := ultimas[k]
		for passo := 1; passo <= int(u.Total-u.Numero); passo++ {
			if data := u.Data.SomarMeses(passo); data.NumeroDoDia() > hoje.NumeroDoDia() {
				futuras = append(futuras, Futura{u.ContaId, data, tipos.DeDecimal(u.Valor.Decimal().Abs()), false, u.Id, passo})
			}
		}
	}
	return futuras
}
