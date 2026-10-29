package texto

import (
	"sync"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

// O Collator guarda estado entre uma comparação e outra e não pode ser usado por duas goroutines ao mesmo tempo.
var (
	culturaPtBR = collate.New(language.BrazilianPortuguese)
	travaPtBR   sync.Mutex
)

// CompararNaCultura é o Comparer<string>.Default do C# com a cultura pt-BR (a do processo da API, com o ICU): primeiro
// as letras sem acento e sem caixa ("árvore" antes de "Bolsa"), depois o acento e por último a caixa (minúscula antes).
// É a ordem de um OrderBy/ThenBy de texto feito em memória; o ORDER BY do SQLite compara os bytes, e é outra.
//
// As tabelas do x/text são do CLDR 23 (Unicode 6.2), mais velhas que as do ICU do .NET: letra criada depois disso pode
// sair noutra posição. O gabarito testdata/ordem.json, gravado pela sonda com o próprio .NET, confere os nomes comuns.
func CompararNaCultura(a, b string) int {
	travaPtBR.Lock()
	defer travaPtBR.Unlock()
	return culturaPtBR.CompareString(a, b)
}
