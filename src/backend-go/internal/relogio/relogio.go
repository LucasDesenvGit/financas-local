// Pacote relogio é o Relogio.cs do Go: datas no fuso de Brasília, seja qual for o fuso do computador. Ao contrário
// do C# (classe estática), o relógio é passado como dependência, e os testes usam um relógio parado (Fixo).
package relogio

import (
	"time"
	_ "time/tzdata" // fusos embutidos no executável: o Windows não tem a base de fusos que o time.LoadLocation lê

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Brasilia é o fuso America/Sao_Paulo, com o horário de verão antigo (até 2019).
var Brasilia = carregarBrasilia()

func carregarBrasilia() *time.Location {
	fuso, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		panic("fuso America/Sao_Paulo não encontrado, nem no time/tzdata: " + err.Error())
	}
	return fuso
}

// Relogio dá o instante atual. Regra de negócio nunca chama time.Now(): recebe um Relogio.
type Relogio interface {
	Agora() time.Time // instante atual, em UTC
}

// Sistema é o relógio de verdade.
type Sistema struct{}

func (Sistema) Agora() time.Time { return time.Now().UTC() }

// Fixo é um relógio parado, para testes.
type Fixo struct {
	Instante time.Time
}

func (f Fixo) Agora() time.Time { return f.Instante.UTC() }

// AgoraLocal é a hora atual em Brasília (Relogio.AgoraLocal no C#).
func AgoraLocal(r Relogio) time.Time { return r.Agora().In(Brasilia) }

// Hoje é a data de hoje em Brasília (Relogio.Hoje no C#).
func Hoje(r Relogio) tipos.Data { return tipos.DataDe(AgoraLocal(r)) }

// ParaHoraLocal converte um instante para a hora de Brasília.
func ParaHoraLocal(instante time.Time) time.Time { return instante.In(Brasilia) }

// InicioDoDiaUtc é o instante, em UTC, em que o dia começa em Brasília (meia-noite local).
func InicioDoDiaUtc(dia tipos.Data) time.Time {
	return time.Date(dia.Ano(), dia.Mes(), dia.Dia(), 0, 0, 0, 0, Brasilia).UTC()
}

// PrimeiroDiaMes e UltimoDiaMes são os do C#.
func PrimeiroDiaMes(ano int, mes time.Month) tipos.Data { return tipos.NovaData(ano, mes, 1) }

func UltimoDiaMes(ano int, mes time.Month) tipos.Data {
	return tipos.NovaData(ano, mes+1, 1).SomarDias(-1) // dia 1 do mês seguinte, menos um (time.Date normaliza o mês 13)
}
