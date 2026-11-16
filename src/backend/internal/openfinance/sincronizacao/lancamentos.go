package sincronizacao

import (
	"slices"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// maximoDeIds é quantos ids o Resultado leva (#688): o mesmo limite da lista de ids do extrato (extrato/filtro.go) e do
// ?ids= das Transações (MAXIMO_IDS_URL no front-end).
const maximoDeIds = 200

// Lancamentos são as transações que a rodada trouxe ou mudou (#688), para o aviso abrir as Transações só nelas.
type Lancamentos struct {
	// Ids das novas e das atualizadas, em ordem crescente. Vazio quando passam de maximoDeIds: a tela abre o período.
	Ids []int64 `json:"ids"`
	// Inicio e Fim são a data mais antiga e a mais recente entre elas, todas, inclusive as que passaram do limite.
	Inicio tipos.Data `json:"inicio"`
	Fim    tipos.Data `json:"fim"`
}

// lancamentosDaRodada junta as novas e as atualizadas de todas as contas e conexões de uma rodada.
type lancamentosDaRodada struct {
	ids         []int64
	inicio, fim tipos.Data
}

func (l *lancamentosDaRodada) somar(id int64, data tipos.Data) {
	if len(l.ids) == 0 || data.Antes(l.inicio) {
		l.inicio = data
	}
	if len(l.ids) == 0 || data.Depois(l.fim) {
		l.fim = data
	}
	l.ids = append(l.ids, id)
}

// resultado é o que vai no Resultado: nil quando nada entrou nem mudou.
func (l *lancamentosDaRodada) resultado() *Lancamentos {
	if len(l.ids) == 0 {
		return nil
	}
	ids := []int64{}
	if len(l.ids) <= maximoDeIds {
		ids = slices.Sorted(slices.Values(l.ids))
	}
	return &Lancamentos{Ids: ids, Inicio: l.inicio, Fim: l.fim}
}
