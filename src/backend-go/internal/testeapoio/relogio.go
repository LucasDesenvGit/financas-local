package testeapoio

import (
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// InstanteDosTestes é 23/10/2026 02:30 em UTC, que em Brasília ainda é 22/10/2026 23:30: o dia muda com o fuso, e
// um teste que confunda UTC com Brasília erra o dia. É o mesmo instante do instanteDosTestes de internal/banco.
var InstanteDosTestes = time.Date(2026, 10, 23, 2, 30, 0, 0, time.UTC)

// RelogioDosTestes é o relógio parado em InstanteDosTestes: relogio.Hoje dá 22/10/2026.
func RelogioDosTestes() relogio.Fixo { return relogio.Fixo{Instante: InstanteDosTestes} }

// HojeEmBrasilia é um relógio parado ao meio-dia (de Brasília) do dia dado: "hoje em Brasília é dia X", longe da
// meia-noite. Para testar a virada do dia, use AgoraEmBrasilia com a hora exata.
func HojeEmBrasilia(ano int, mes time.Month, dia int) relogio.Fixo {
	return AgoraEmBrasilia(ano, mes, dia, 12, 0)
}

// AgoraEmBrasilia é um relógio parado na hora de parede dada, no fuso de Brasília (com o horário de verão antigo,
// até 2019). Ex.: AgoraEmBrasilia(2026, 10, 22, 23, 30) é 23/10/2026 02:30 em UTC.
func AgoraEmBrasilia(ano int, mes time.Month, dia, hora, minuto int) relogio.Fixo {
	return relogio.Fixo{Instante: time.Date(ano, mes, dia, hora, minuto, 0, 0, relogio.Brasilia).UTC()}
}

// Dia é um atalho para tipos.NovaData (o DateOnly do C#).
func Dia(ano int, mes time.Month, dia int) tipos.Data { return tipos.NovaData(ano, mes, dia) }

// Valor lê um decimal escrito no teste ("-12.50"), guardando a escala. Entra em pânico se o texto não for um
// decimal: é para literais do próprio teste, como o regexp.MustCompile.
func Valor(texto string) tipos.Decimal {
	v, err := tipos.NovoDecimal(texto)
	if err != nil {
		panic("testeapoio.Valor: " + err.Error())
	}
	return v
}
