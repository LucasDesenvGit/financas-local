// Pacote carteira é o CarteiraService do C# (#82): a carteira de renda variável lançada à mão (FIIs, ações, ETFs, BDRs),
// com preço médio, valorização, proventos e comparação com a Selic, e os investimentos do Open Finance ao lado dela.
package carteira

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cambio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/texto"
)

var (
	// formatoTicker é o código da B3 ("MXRF11", "PETR4"), com o "F" do mercado fracionário.
	formatoTicker = regexp.MustCompile(`^[A-Z][A-Z0-9]{3}[0-9]{1,2}F?$`)
	// formatoTickerExterior é o código como no Yahoo Finance ("AAPL", "SAP.DE").
	formatoTickerExterior = regexp.MustCompile(`^[A-Z0-9][A-Z0-9.\-]{0,11}$`)
)

func tickerLimpo(ticker *string) string {
	if ticker == nil {
		return ""
	}
	return texto.MaiusculasInvariantes(strings.TrimFunc(*ticker, unicode.IsSpace))
}

// NormalizarTicker normaliza e valida um código da B3 ("mxrf11 " → "MXRF11"), ou nil se for inválido. O formato é estrito
// de propósito: ativos fora da B3 entram pela moeda, com NormalizarTickerExterior (ADR 0009). O mercado fracionário
// (final F) usa a cotação do lote padrão.
func NormalizarTicker(ticker *string) *string {
	t := strings.TrimSuffix(tickerLimpo(ticker), ".SA")
	if !formatoTicker.MatchString(t) {
		return nil
	}
	if len(t) >= 6 {
		t = strings.TrimSuffix(t, "F")
	}
	return &t
}

// NormalizarTickerExterior normaliza e valida o código de um ativo no exterior ("aapl" → "AAPL"), ou nil se for inválido
// ou se for um código da B3, que colidiria com o da B3.
func NormalizarTickerExterior(ticker *string) *string {
	t := tickerLimpo(ticker)
	if !formatoTickerExterior.MatchString(t) || !strings.ContainsFunc(t, unicode.IsLetter) {
		return nil
	}
	if strings.HasSuffix(t, ".SA") || formatoTicker.MatchString(t) {
		return nil
	}
	return &t
}

// NormalizarTickerDaMoeda normaliza o código conforme a moeda do ativo: BRL é a B3; outra moeda, o exterior.
func NormalizarTickerDaMoeda(ticker *string, moeda string) *string {
	if moeda == cambio.MoedaBase {
		return NormalizarTicker(ticker)
	}
	return NormalizarTickerExterior(ticker)
}
