package carteira

import (
	"slices"
	"strings"
	"unicode"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/bancos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/texto"
)

// Os tipos de ativo (o enum TipoAtivo do C#).
const (
	tipoFii   int64 = 1
	tipoAcao  int64 = 2
	tipoEtf   int64 = 3
	tipoBdr   int64 = 4
	tipoOutro int64 = 5
)

// Os tipos de operação (o enum TipoOperacao do C#).
const (
	operacaoCompra int64 = 1
	operacaoVenda  int64 = 2
)

// nomeDoTipo é o NomeTipo: o nome da classe na distribuição.
func nomeDoTipo(tipo int64) string {
	switch tipo {
	case tipoFii:
		return "Fundos imobiliários"
	case tipoAcao:
		return "Ações"
	case tipoEtf:
		return "ETFs"
	case tipoBdr:
		return "BDRs"
	}
	return "Outros (renda variável)"
}

// nomeDoTipoPluggy é o NomeTipoPluggy: o nome da classe do investimento do Open Finance.
func nomeDoTipoPluggy(tipo string) string {
	switch texto.MaiusculasInvariantes(tipo) {
	case "FIXED_INCOME":
		return "Renda fixa"
	case "SECURITY":
		return "Previdência"
	case "MUTUAL_FUND":
		return "Fundos de investimento"
	case "EQUITY":
		return "Ações"
	case "ETF":
		return "ETFs"
	case "COE":
		return "COE"
	}
	return "Outros investimentos"
}

// ehRendaVariavel diz se o tipo da Pluggy é ação ou ETF.
func ehRendaVariavel(tipo string) bool {
	t := texto.MaiusculasInvariantes(tipo)
	return t == "EQUITY" || t == "ETF"
}

// segmentosFii são os segmentos para agrupar os FIIs, sem diferença de ordem do C#.
var segmentosFii = []string{"Logística", "Shoppings", "Lajes corporativas", "Papel (recebíveis)", "Híbrido", "Fundo de fundos",
	"Renda urbana", "Agronegócio", "Residencial", "Hotéis", "Hospitais/Educação", "Desenvolvimento", "Outros"}

// sugestoesDeSegmento cobrem alguns fundos conhecidos; o segmento pode (e deve) ser conferido em "editar" no ativo.
var sugestoesDeSegmento = map[string]string{
	"HGLG11": "Logística", "BTLG11": "Logística", "XPLG11": "Logística", "VILG11": "Logística",
	"BRCO11": "Logística", "LVBI11": "Logística", "GGRC11": "Logística", "RBRL11": "Logística",
	"XPML11": "Shoppings", "VISC11": "Shoppings", "HGBS11": "Shoppings", "HSML11": "Shoppings", "MALL11": "Shoppings",
	"HGRE11": "Lajes corporativas", "PVBI11": "Lajes corporativas", "JSRE11": "Lajes corporativas",
	"RCRB11": "Lajes corporativas", "BRCR11": "Lajes corporativas",
	"KNCR11": "Papel (recebíveis)", "KNIP11": "Papel (recebíveis)", "KNSC11": "Papel (recebíveis)",
	"CPTS11": "Papel (recebíveis)", "IRDM11": "Papel (recebíveis)", "RECR11": "Papel (recebíveis)",
	"VGIR11": "Papel (recebíveis)", "MCCI11": "Papel (recebíveis)", "RBRR11": "Papel (recebíveis)",
	"BCRI11": "Papel (recebíveis)", "VRTA11": "Papel (recebíveis)", "HGCR11": "Papel (recebíveis)",
	"KNHY11": "Papel (recebíveis)", "MXRF11": "Papel (recebíveis)", "DEVA11": "Papel (recebíveis)",
	"KNRI11": "Híbrido", "ALZR11": "Híbrido", "HGRU11": "Renda urbana", "TRXF11": "Renda urbana",
	"BCFF11": "Fundo de fundos", "RBRF11": "Fundo de fundos", "HFOF11": "Fundo de fundos",
	"KFOF11": "Fundo de fundos", "BPFF11": "Fundo de fundos",
	"TGAR11": "Desenvolvimento", "RZTR11": "Agronegócio", "SNAG11": "Agronegócio",
	"KNCA11": "Agronegócio", "VGIA11": "Agronegócio", "HCTR11": "Papel (recebíveis)",
}

// sugerirSegmento é o SegmentosFii.Sugerir: sem diferença de maiúsculas, nil para o fundo que não conhece.
func sugerirSegmento(ticker string) *string {
	if s, ok := sugestoesDeSegmento[texto.MaiusculasInvariantes(ticker)]; ok {
		return &s
	}
	return nil
}

// SegmentoValido é o SegmentosFii.Valido: nil ou um dos segmentos.
func SegmentoValido(segmento *string) bool {
	return segmento == nil || slices.Contains(segmentosFii, *segmento)
}

// tamanhoMaximoProduto: até este tamanho, o texto antes do " - " é o produto (CDB, LCI, RDB…), e não parte da razão
// social.
const tamanhoMaximoProduto = 24

// NomeDeExibicao é o NomeInvestimento.Exibicao (#170): o nome que você reconhece. O banco manda a razão social do
// emissor ("CDB - NU FINANCEIRA S.A. - SOCIEDADE DE CREDITO, F…"), e aqui vira "CDB · Nubank". O nome escolhido vale
// sempre; ações e ETFs ficam como vieram (o ticker já é o nome).
func NomeDeExibicao(nome, tipo string, emissor, personalizado *string) string {
	aparar := func(s string) string { return strings.TrimFunc(s, unicode.IsSpace) }
	if personalizado != nil && aparar(*personalizado) != "" {
		return aparar(*personalizado)
	}
	if tipo == "EQUITY" || tipo == "ETF" {
		return nome
	}
	banco := bancos.Nome(bancos.Detectar(emissor, &nome))
	if banco == nil {
		return nome
	}
	cabeNoProduto := func(s string) bool {
		n := respostas.TamanhoUtf16(s)
		return n > 0 && n <= tamanhoMaximoProduto
	}
	// "CDB - NU FINANCEIRA S.A. - …": o produto é o que vem antes do primeiro " - ".
	if antes, depois, ok := strings.Cut(nome, " - "); ok {
		produto, resto := aparar(antes), aparar(depois)
		if cabeNoProduto(produto) && bancos.Detectar(&produto) == nil && bancos.Detectar(&resto) != nil {
			return produto + " · " + *banco
		}
		return nome
	}
	// "CDB" com o emissor à parte: o nome curto ganha o banco.
	if curto := aparar(nome); cabeNoProduto(curto) && bancos.Detectar(&nome) == nil {
		return curto + " · " + *banco
	}
	return nome
}
