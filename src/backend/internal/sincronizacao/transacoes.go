package sincronizacao

import (
	"cmp"
	"context"
	"database/sql"
	"slices"
	"strings"
	"unicode"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/categorizacao"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/pluggy"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/regrasdescricao"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/transacoes"
)

// Situação da transação (o enum StatusTransacao do C#).
const (
	efetivada int64 = 1
	pendente  int64 = 2
)

const (
	// margemDaJanela é o quanto antes da data inicial as transações locais são carregadas.
	margemDaJanela = 60
	// diasPendenteEfetivada (#172): até quantos dias depois da pendente a efetivada igual pode aparecer.
	diasPendenteEfetivada = 10
	diasAntesDaPendente   = 2
)

// sincronizarTransacoes aplica as transações da Pluggy. Tudo é decidido antes de gravar, numa transação só: como o
// SaveChanges do .NET, a limpeza das pendentes antigas lê o banco sem as mudanças desta rodada.
func (r *rodada) sincronizarTransacoes(ctx context.Context, conta contaLocal, dataInicial tipos.Data, mapaFaturas map[string]int64) (novas, atualizadas int, err error) {
	recebidas, err := r.pluggy.ListarTransacoes(ctx, r.sessao, conta.pluggyId, dataInicial)
	if err != nil {
		return 0, 0, err
	}
	err = banco.EmTransacao(ctx, r.db, func(tx *sql.Tx) error {
		locais := map[string]*transacoes.Transacao{}
		lista, err := transacoes.Listar(ctx, tx, `WHERE "ContaId" = ? AND "PluggyTransactionId" IS NOT NULL AND "Data" >= ?`, conta.id, dataInicial.SomarDias(-margemDaJanela))
		if err != nil {
			return err
		}
		porId := map[int64]*transacoes.Transacao{}
		for _, t := range lista {
			locais[*t.PluggyTransactionId] = t
			porId[t.Id] = t
		}
		recebidos := map[string]bool{}
		var inserir, gravar []*transacoes.Transacao
		for i := range recebidas {
			pt := &recebidas[i]
			if strings.TrimSpace(pt.Id) == "" || pt.Date == nil || recebidos[pt.Id] {
				continue
			}
			recebidos[pt.Id] = true
			t, ok := locais[pt.Id]
			if !ok {
				// Pode existir fora da janela carregada (data alterada pela instituição).
				fora, err := transacoes.Listar(ctx, tx, `WHERE "PluggyTransactionId" = ?`, pt.Id)
				if err != nil {
					return err
				}
				if len(fora) > 0 {
					t = fora[0]
					if t.ContaId != conta.id {
						continue
					}
					porId[t.Id] = t
				}
			}
			nova := t == nil
			if nova {
				t = &transacoes.Transacao{ContaId: conta.id, PluggyTransactionId: &pt.Id, Status: efetivada, AtualizadoEm: r.agora()}
			}
			if err := r.aplicar(t, pt, nova, conta.ehCartao, mapaFaturas); err != nil {
				return err
			}
			if nova {
				inserir = append(inserir, t)
				novas++
			} else if t.Mudou() {
				t.AtualizadoEm = r.agora()
				gravar = append(gravar, t)
				atualizadas++
			}
		}

		// Pendentes que sumiram na instituição: canceladas ou efetivadas com outro ID.
		var apagar []int64
		for _, t := range lista {
			if t.Status == pendente && !t.Data.Antes(dataInicial) && !recebidos[*t.PluggyTransactionId] {
				apagar = append(apagar, t.Id)
			}
		}
		antigas, err := r.pendentesJaEfetivadas(ctx, tx, conta.id, dataInicial, porId)
		if err != nil {
			return err
		}
		apagar = append(apagar, antigas...)

		for _, t := range inserir {
			if err := transacoes.Inserir(ctx, tx, t, r.agora()); err != nil {
				return err
			}
		}
		if err := transacoes.GravarTodas(ctx, tx, gravar); err != nil {
			return err
		}
		return transacoes.Excluir(ctx, tx, apagar...)
	})
	if err != nil {
		return 0, 0, err
	}
	return novas, atualizadas, nil
}

// aplicar passa os dados da Pluggy para a transação, na ordem do C#: a regra de descrição usa o valor, e a categoria usa
// a descrição.
func (r *rodada) aplicar(t *transacoes.Transacao, pt *pluggy.Transacao, nova, ehCartao bool, mapaFaturas map[string]int64) error {
	recebida := "(sem descrição)"
	if pt.Description != nil {
		if d := limitado(strings.TrimSpace(*pt.Description), 300); d != "" {
			recebida = d
		}
	}
	// Nome automático (o do banco ou o de uma regra, #123) acompanha o banco e as regras; nome escolhido à mão fica.
	nomeAutomatico := nova || regrasdescricao.NomeAutomatico(t)
	t.DescricaoOriginal = &recebida
	t.Estabelecimento = estabelecimento(pt.Merchant)
	t.Data = pt.Date.Data()
	valor, err := calcularValor(pt, ehCartao)
	if err != nil {
		return err
	}
	t.Valor = valor
	if nomeAutomatico {
		t.Descricao = recebida
		t.RegraDescricaoId = nil
		r.descricoes.Aplicar(t) // depois do valor: a regra pode ter faixa de valor (#241)
	}
	t.Status = efetivada
	if pt.Status != nil && strings.EqualFold(*pt.Status, "PENDING") {
		t.Status = pendente
	}
	t.PluggyCategoriaId = limitar(pt.CategoryId, 20)
	t.ParcelaNumero, t.ParcelaTotal = nil, nil
	if m := pt.CreditCardMetadata; m != nil {
		t.ParcelaNumero, t.ParcelaTotal = parcela(m.InstallmentNumber), parcela(m.TotalInstallments)
		if m.BillId != nil {
			if id, ok := mapaFaturas[*m.BillId]; ok {
				t.FaturaCartaoId = &id
			}
		}
	}
	// #111: como foi (Pix, boleto, pagamento de fatura...) e se foi entre contas do mesmo titular.
	t.MeioPagamento = meioDePagamento(pt)
	t.MesmaTitularidade = mesmoTitular(pt.PaymentData)

	anterior := t.CategoriaId
	if !t.CategoriaManual {
		t.CategoriaId = r.regras.Resolver(t)
	}
	// #156: com a categoria igual, o tipo já foi decidido (inclusive pela reclassificação das transferências, no fim).
	// Reaplicar marcaria de novo como transferência o que ela devolveu para despesa ou receita, e a transação contaria
	// como atualizada a cada rodada. Só o sinal do valor ainda muda o tipo de uma despesa ou receita.
	switch {
	case nova || !mesmoId(t.CategoriaId, anterior):
		r.regras.AplicarTipo(t)
	case t.Tipo != transacoes.Transferencia && t.TransferenciaGrupo == nil:
		t.Tipo = categorizacao.TipoPeloSinal(t.Valor, ehCartao)
	}
	return nil
}

func mesmoId(a, b *int64) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

func estabelecimento(e *pluggy.Estabelecimento) *string {
	if e == nil {
		return nil
	}
	if e.BusinessName != nil {
		return limitar(e.BusinessName, 200)
	}
	return limitar(e.Name, 200)
}

// parcela é o (short?) do C#: o int vira 16 bits sem conferir.
func parcela(n *pluggy.Inteiro) *int64 {
	if n == nil {
		return nil
	}
	v := int64(int16(*n))
	return &v
}

func meioDePagamento(pt *pluggy.Transacao) *string {
	meio := pt.OperationType
	if meio == nil && pt.PaymentData != nil {
		meio = pt.PaymentData.PaymentMethod
	}
	if meio == nil {
		return nil
	}
	maiusculas := texto.MaiusculasInvariantes(strings.TrimSpace(*meio))
	return limitar(&maiusculas, 40)
}

// calcularValor normaliza o sinal: no banco local, negativo é saída. Na Pluggy, em cartão, valor positivo é compra; em
// conta, o tipo DEBIT ou CREDIT define o sentido.
func calcularValor(pt *pluggy.Transacao, ehCartao bool) (tipos.Decimal, error) {
	numero := pt.AmountInAccountCurrency
	if numero == nil {
		numero = pt.Amount
	}
	valor, err := arredondar(numero)
	if err != nil {
		return tipos.Decimal{}, err
	}
	negativo := tipos.DeDecimal(valor.Decimal().Abs().Neg())
	if ehCartao {
		return tipos.DeDecimal(valor.Decimal().Neg()), nil
	}
	if pt.Type != nil {
		switch texto.MaiusculasInvariantes(*pt.Type) {
		case "DEBIT":
			return negativo, nil
		case "CREDIT":
			return tipos.DeDecimal(valor.Decimal().Abs()), nil
		}
	}
	return valor, nil
}

// mesmoTitular (#111): pagador e recebedor com o mesmo CPF ou CNPJ, só dígitos, completos. Documento mascarado ou ausente
// dá nil (não dá para saber). Só o resultado é gravado; os documentos não.
func mesmoTitular(dados *pluggy.DadosPagamento) *bool {
	if dados == nil {
		return nil
	}
	pagador, recebedor := digitos(dados.Payer), digitos(dados.Receiver)
	if pagador == "" || recebedor == "" {
		return nil
	}
	igual := pagador == recebedor
	return &igual
}

func digitos(p *pluggy.Participante) string {
	documento := p.Documento()
	if strings.TrimSpace(documento) == "" || strings.ContainsRune(documento, '*') {
		return ""
	}
	soDigitos := strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) {
			return r
		}
		return -1
	}, documento)
	if n := len([]rune(soDigitos)); n != 11 && n != 14 {
		return ""
	}
	return soDigitos
}

// pendentesJaEfetivadas (#172): pendente de antes da janela relida (a instituição não a manda mais) que tem, na mesma
// conta, uma efetivada do mesmo valor entre 2 dias antes e 10 dias depois: o banco a efetivou com outro código, e a
// pendente sai para não contar em dobro.
func (r *rodada) pendentesJaEfetivadas(ctx context.Context, tx *sql.Tx, contaId int64, dataInicial tipos.Data, carregadas map[int64]*transacoes.Transacao) ([]int64, error) {
	antigas, err := transacoes.Listar(ctx, tx, `WHERE "ContaId" = ? AND "Status" = ? AND "PluggyTransactionId" IS NOT NULL AND "Data" < ?`, contaId, pendente, dataInicial)
	if err != nil || len(antigas) == 0 {
		return nil, err
	}
	for i, t := range antigas {
		if carregada, ok := carregadas[t.Id]; ok {
			antigas[i] = carregada // como o EF, que devolve a entidade que já acompanha, com os valores da rodada
		}
	}
	de, ate := antigas[0].Data, antigas[0].Data
	for _, t := range antigas {
		if t.Data.Antes(de) {
			de = t.Data
		}
		if t.Data.Depois(ate) {
			ate = t.Data
		}
	}
	efetivadas, err := transacoes.Listar(ctx, tx, `WHERE "ContaId" = ? AND "Status" = ? AND "Data" >= ? AND "Data" <= ? ORDER BY "Data" DESC, "Id"`,
		contaId, efetivada, de.SomarDias(-diasAntesDaPendente), ate.SomarDias(diasPendenteEfetivada))
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(antigas, func(a, b *transacoes.Transacao) int {
		return cmp.Or(cmp.Compare(a.Data.NumeroDoDia(), b.Data.NumeroDoDia()), cmp.Compare(a.Id, b.Id))
	})
	usadas := map[int64]bool{}
	var apagar []int64
	for _, p := range antigas {
		var par *transacoes.Transacao
		for _, e := range efetivadas {
			if usadas[e.Id] || e.Valor.Decimal().Cmp(p.Valor.Decimal()) != 0 || e.Data.Antes(p.Data.SomarDias(-diasAntesDaPendente)) || e.Data.Depois(p.Data.SomarDias(diasPendenteEfetivada)) {
				continue
			}
			if par == nil || distancia(e, p) < distancia(par, p) {
				par = e
			}
		}
		if par == nil {
			continue
		}
		usadas[par.Id] = true
		apagar = append(apagar, p.Id)
		r.log.Info("pendente removida: o banco a efetivou com outro código", "origem", "Open Finance", "pendente", p.Id, "efetivada", par.Id)
	}
	return apagar, nil
}

func distancia(a, b *transacoes.Transacao) int {
	d := a.Data.NumeroDoDia() - b.Data.NumeroDoDia()
	if d < 0 {
		return -d
	}
	return d
}
