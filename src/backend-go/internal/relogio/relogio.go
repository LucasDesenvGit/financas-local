// Pacote relogio é o Relogio.cs do Go: datas no fuso de Brasília, seja qual for o fuso do computador. Ao contrário
// do C# (classe estática), o relógio é passado como dependência, e os testes usam um relógio parado (Fixo) ou um
// que começa numa hora dada e anda dali (APartirDe, o do contrato).
package relogio

import (
	"fmt"
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

// APartirDe é um relógio que começa num instante dado e anda dali, no ritmo do relógio de verdade. Só para teste
// (#363): é o que o FAKETIME="@..." da libfaketime faz com o .NET no contrato. A libfaketime não age sobre o Go, que
// lê o relógio sem passar pela libc, por isso a opção -inicio-do-relogio do cmd/financas.
//
// Ao contrário do Fixo, o tempo passa: o bloqueio de login de 15 minutos e o "há quanto tempo" continuam
// funcionando. O tempo decorrido vem da leitura monotônica do time.Now, então acertar o relógio do computador no
// meio do teste não muda nada.
type APartirDe struct {
	inicio  time.Time
	partida time.Time        // quando o relógio foi criado, com a leitura monotônica
	agora   func() time.Time // time.Now; os testes trocam
}

// NovoAPartirDe cria um relógio que agora marca inicio.
func NovoAPartirDe(inicio time.Time) APartirDe { return novoAPartirDe(inicio, time.Now) }

func novoAPartirDe(inicio time.Time, agora func() time.Time) APartirDe {
	return APartirDe{inicio: inicio, partida: agora(), agora: agora}
}

func (r APartirDe) Agora() time.Time { return r.inicio.Add(r.agora().Sub(r.partida)).UTC() }

// LerInicio lê o instante de partida do relógio de teste: RFC 3339 com o fuso explícito ("2026-10-15T01:00:00Z" ou
// "2026-10-14T22:00:00-03:00"), para não depender do fuso do computador. Fração de segundo é aceita.
func LerInicio(texto string) (time.Time, error) {
	instante, err := time.Parse(time.RFC3339, texto)
	if err != nil {
		return time.Time{}, fmt.Errorf("início do relógio %q: use RFC 3339 com fuso, como 2026-10-15T01:00:00Z: %w", texto, err)
	}
	return instante.UTC(), nil
}

// AgoraLocal é a hora atual em Brasília (Relogio.AgoraLocal no C#).
func AgoraLocal(r Relogio) time.Time { return r.Agora().In(Brasilia) }

// Hoje é a data de hoje em Brasília (Relogio.Hoje no C#).
func Hoje(r Relogio) tipos.Data { return tipos.DataDe(AgoraLocal(r)) }

// ParaHoraLocal converte um instante para a hora de Brasília.
func ParaHoraLocal(instante time.Time) time.Time { return instante.In(Brasilia) }

// InicioDoDiaUtc é o instante, em UTC, em que o dia começa em Brasília (meia-noite local).
//
// No dia em que o horário de verão começava (até 2019), a meia-noite não existia: o relógio ia de 23:59:59 direto
// para 01:00. Para uma hora que não existe, o time.Date não garante qual dos dois fusos usa, e aqui devolve as 23:00
// do dia anterior. O dia então começa na troca de fuso, às 01:00 (#372, achado pelo fuzz). O C# lança exceção
// nesse dia (#393).
func InicioDoDiaUtc(dia tipos.Data) time.Time {
	inicio := time.Date(dia.Ano(), dia.Mes(), dia.Dia(), 0, 0, 0, 0, Brasilia)
	if tipos.DataDe(inicio).String() != dia.String() {
		_, inicio = inicio.ZoneBounds()
	}
	return inicio.UTC()
}

// PrimeiroDiaMes e UltimoDiaMes são os do C#.
func PrimeiroDiaMes(ano int, mes time.Month) tipos.Data { return tipos.NovaData(ano, mes, 1) }

func UltimoDiaMes(ano int, mes time.Month) tipos.Data {
	return tipos.NovaData(ano, mes+1, 1).SomarDias(-1) // dia 1 do mês seguinte, menos um (time.Date normaliza o mês 13)
}
