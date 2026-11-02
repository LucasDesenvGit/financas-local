package sincronizacao

import "strings"

// mapaPadrao é o MapaPadraoPluggy do CategorizacaoService: a categoria sua que combina com a categoria nova da Pluggy,
// pelo nome em inglês. A chave está em minúsculas, porque o C# compara sem diferença de maiúsculas.
var mapaPadrao = map[string]string{
	"salary":                           "Salário",
	"income":                           "Outras receitas",
	"retirement":                       "Outras receitas",
	"entrepreneurial activities":       "Outras receitas",
	"government aid":                   "Outras receitas",
	"non-recurring income":             "Outras receitas",
	"proceeds interests and dividends": "Rendimentos",
	"investment income":                "Rendimentos",
	"investments":                      "Investimentos",
	"automatic investment":             "Investimentos",
	"same person transfer":             "Transferências",
	"transfers":                        "Transferências",
	"transfer - pix":                   "Transferências",
	"transfer - ted":                   "Transferências",
	"credit card payment":              "Pagamento de fatura",
	"loans and financing":              "Juros e empréstimos",
	"interests charged":                "Juros e empréstimos",
	"late payment and overdraft costs": "Juros e empréstimos",
	"services":                         "Serviços e assinaturas",
	"telecommunications":               "Serviços e assinaturas",
	"digital services":                 "Serviços e assinaturas",
	"video streaming":                  "Serviços e assinaturas",
	"music streaming":                  "Serviços e assinaturas",
	"shopping":                         "Compras",
	"online shopping":                  "Compras",
	"electronics":                      "Compras",
	"clothing":                         "Compras",
	"groceries":                        "Mercado",
	"food and drinks":                  "Alimentação",
	"eating out":                       "Alimentação",
	"food delivery":                    "Alimentação",
	"travel":                           "Viagens",
	"accomodation":                     "Viagens",
	"airport and airlines":             "Viagens",
	"donations":                        "Doações",
	"taxes":                            "Impostos e taxas",
	"legal obligations":                "Impostos e taxas",
	"bank fees":                        "Tarifas bancárias",
	"housing":                          "Moradia",
	"rent":                             "Moradia",
	"utilities":                        "Moradia",
	"healthcare":                       "Saúde",
	"pharmacy":                         "Saúde",
	"transportation":                   "Transporte",
	"automotive":                       "Transporte",
	"gas stations":                     "Transporte",
	"taxi and ride-hailing":            "Transporte",
	"insurance":                        "Seguros",
	"leisure":                          "Lazer",
	"gambling":                         "Lazer",
	"education":                        "Educação",
	"pet supplies and vet":             "Pets",
	"wellness and fitness":             "Cuidados pessoais",
	"personal care":                    "Cuidados pessoais",
}

// sugerirCategoriaPadrao é o SugerirCategoriaPadrao: pelo nome da categoria ou do pai dela. Vazio: nenhuma.
func sugerirCategoriaPadrao(descricao, paiDescricao *string) string {
	for _, texto := range []*string{descricao, paiDescricao} {
		if texto == nil {
			continue
		}
		if nome, ok := mapaPadrao[strings.ToLower(strings.TrimSpace(*texto))]; ok {
			return nome
		}
	}
	return ""
}
