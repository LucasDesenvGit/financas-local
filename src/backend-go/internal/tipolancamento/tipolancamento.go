// Pacote tipolancamento é o TipoLancamentoService do .NET (#111, ADR 0025): como o dinheiro entrou ou saiu (compra
// no cartão, Pix, pagamento de fatura…), calculado na hora, nesta ordem: transferência lançada pelo sistema; o tipo de
// operação que o banco informa (Transacao.MeioPagamento), com "mesma titularidade" para separar Pix para você de Pix
// para outra pessoa; e, sem isso, a descrição, por palavras inteiras. Quando nada indica, "outro".
package tipolancamento

import (
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Tipo é o enum TipoLancamento do C#.
type Tipo int64

const (
	Outro                 Tipo = 0
	CompraCartao          Tipo = 1
	EstornoCartao         Tipo = 2
	PagamentoFatura       Tipo = 3
	PixEnviado            Tipo = 4
	PixRecebido           Tipo = 5
	TransferenciaEnviada  Tipo = 6
	TransferenciaRecebida Tipo = 7
	EntreContas           Tipo = 8
	Boleto                Tipo = 9
	CompraDebito          Tipo = 10
	Saque                 Tipo = 11
	Deposito              Tipo = 12
	TarifaJuros           Tipo = 13
	Rendimento            Tipo = 14
	AplicacaoResgate      Tipo = 15
	Salario               Tipo = 16
	Credito               Tipo = 17
	Dinheiro              Tipo = 18
)

// Tipos de conta (o enum TipoConta do C#) que mudam a classificação.
const (
	ContaCorrente = 1
	CartaoCredito = 3
	ContaDinheiro = 5
)

// Classificar é o TipoLancamentoService.Classificar.
func Classificar(valor tipos.Decimal, descricao string, descricaoOriginal, meio *string, mesmaTitularidade *bool,
	transferenciaDoSistema bool, tipoConta int64) Tipo {
	if transferenciaDoSistema {
		return EntreContas
	}
	saida := valor.Decimal().IsNegative()
	origem := descricao
	if descricaoOriginal != nil {
		origem = *descricaoOriginal
	}
	t := texto.ParaPalavras(origem)
	operacao := ""
	if meio != nil {
		operacao = texto.MaiusculasInvariantes(strings.TrimSpace(*meio))
	}

	if tipoConta == CartaoCredito {
		return noCartao(saida, t, operacao)
	}
	if tipoConta == ContaDinheiro {
		return Dinheiro
	}
	// Pagamento de fatura pela conta: o banco costuma mandar como boleto, convênio ou "outros"; a descrição diz.
	if saida && ehPagamentoFatura(t) {
		return PagamentoFatura
	}
	if informado, ok := peloInformado(saida, operacao); ok && informado != Outro {
		if mesmaTitularidade != nil && *mesmaTitularidade && ehTransferencia(informado) {
			return EntreContas
		}
		return informado
	}
	return pelaDescricao(saida, t, mesmaTitularidade)
}

// peloInformado é o switch do tipo de operação informado pelo banco; false quando a operação não é conhecida.
func peloInformado(saida bool, operacao string) (Tipo, bool) {
	se := func(entrada, s Tipo) Tipo {
		if saida {
			return s
		}
		return entrada
	}
	switch operacao {
	case "PIX":
		return se(PixRecebido, PixEnviado), true
	case "PAGAMENTO_FATURA": // #172: o banco pode mandar a operação "pagamento de fatura" também do lado da conta
		return se(Outro, PagamentoFatura), true
	case "TED", "DOC", "TEF", "TRANSFERENCIA_MESMA_INSTITUICAO":
		return se(TransferenciaRecebida, TransferenciaEnviada), true
	case "BOLETO", "CONVENIO_ARRECADACAO":
		return Boleto, true
	case "CARTAO":
		return se(Outro, CompraDebito), true
	case "SAQUE":
		return Saque, true
	case "DEPOSITO":
		return Deposito, true
	case "PACOTE_TARIFA_SERVICOS", "TARIFA_SERVICOS_AVULSOS", "ENCARGOS_JUROS_CHEQUE_ESPECIAL":
		return TarifaJuros, true
	case "RENDIMENTO_APLIC_FINANCEIRA":
		return Rendimento, true
	case "RESGATE_APLIC_FINANCEIRA":
		return AplicacaoResgate, true
	case "FOLHA_PAGAMENTO", "PORTABILIDADE_SALARIO":
		return Salario, true
	case "OPERACAO_CREDITO":
		return Credito, true
	}
	return Outro, false
}

func noCartao(saida bool, t, operacao string) Tipo {
	switch operacao {
	case "PAGAMENTO_FATURA":
		return PagamentoFatura
	case "ESTORNO", "CASHBACK":
		return EstornoCartao
	case "TARIFA":
		return TarifaJuros
	case "OPERACOES_CREDITO_CONTRATADAS_CARTAO":
		return Credito
	case "PAGAMENTO":
		if saida {
			return CompraCartao
		}
		return EstornoCartao
	}
	// No cartão, entrada é o pagamento da fatura ou um estorno.
	if !saida {
		if texto.TemPalavra(t, "PAGAMENTO", "PAGTO", "PGTO", "PAG") {
			return PagamentoFatura
		}
		return EstornoCartao
	}
	if texto.TemPalavra(t, "ANUIDADE", "IOF", "JUROS", "ENCARGOS", "ENCARGO", "TARIFA", "MULTA") {
		return TarifaJuros
	}
	return CompraCartao
}

func pelaDescricao(saida bool, t string, mesmaTitularidade *bool) Tipo {
	mesma := mesmaTitularidade != nil && *mesmaTitularidade
	switch {
	case texto.TemPalavra(t, "PIX"):
		if mesma {
			return EntreContas
		}
		if saida {
			return PixEnviado
		}
		return PixRecebido
	case texto.TemPalavra(t, "TED", "DOC", "TEF", "TRANSFERENCIA", "TRANSF"):
		if mesma {
			return EntreContas
		}
		if saida {
			return TransferenciaEnviada
		}
		return TransferenciaRecebida
	case texto.TemPalavra(t, "BOLETO", "TITULO", "CONVENIO", "DEBITO AUTOMATICO", "DEB AUTOMATICO", "DEB AUT"):
		return Boleto
	case texto.TemPalavra(t, "SAQUE"):
		return Saque
	case texto.TemPalavra(t, "DEPOSITO"):
		return Deposito
	case texto.TemPalavra(t, "TARIFA", "JUROS", "IOF", "ENCARGOS", "CESTA", "ANUIDADE"):
		return TarifaJuros
	case texto.TemPalavra(t, "RENDIMENTO", "RENDIMENTOS", "REMUNERACAO"):
		return Rendimento
	case texto.TemPalavra(t, "RESGATE", "APLICACAO", "APLIC"):
		return AplicacaoResgate
	case texto.TemPalavra(t, "SALARIO", "FOLHA"):
		return Salario
	case saida && texto.TemPalavra(t, "COMPRA", "DEBITO", "COMPRA DEBITO"):
		return CompraDebito
	}
	return Outro
}

func ehTransferencia(t Tipo) bool {
	return t == PixEnviado || t == PixRecebido || t == TransferenciaEnviada || t == TransferenciaRecebida
}

func ehPagamentoFatura(t string) bool {
	return (texto.TemPalavra(t, "FATURA") && texto.TemPalavra(t, "PAGAMENTO", "PAGTO", "PGTO", "PAG", "CARTAO")) ||
		texto.TemPalavra(t, "PAGAMENTO CARTAO", "PAGTO CARTAO", "PGTO CARTAO", "PAG CARTAO")
}
