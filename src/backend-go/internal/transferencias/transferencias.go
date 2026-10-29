// Pacote transferencias é o TransferenciasService do .NET (ADR 0031): transferência só fica fora dos relatórios quando
// é entre contas suas. Vale para a categoria genérica "Transferências", que recebe as transferências do Open Finance.
// Cada lançamento dela é entre contas suas (fora dos relatórios) quando foi lançado pelo site, o banco informa o mesmo
// titular, tem o par em outra conta sua (mesmo valor, sinal oposto, até DiasDoPar dias), é dinheiro de uma reserva ou,
// sem titular informado, cita o seu nome como aparece nos Pix (#172). O resto conta como despesa ou receita.
//
// O dinheiro é comparado em Go, com decimal: no SQLite o valor é texto (CLAUDE.md, "Código Go").
package transferencias

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/categorizacao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipolancamento"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
)

// DiasDoPar é até quantos dias de diferença os dois lados de uma transferência entre contas suas aparecem.
const DiasDoPar = 5

// ChaveMeuNome é o seu nome como aparece nos Pix (Configuracao, #172).
const ChaveMeuNome = "Transferencias.MeuNome"

// categoriasDeTransferencia são as categorias do tipo transferência, por Id (o C# lê sem ORDER BY: pelo Id).
func categoriasDeTransferencia(ctx context.Context, q consultas.DBTX) ([]struct {
	id   int64
	nome string
}, error) {
	linhas, err := q.QueryContext(ctx, `SELECT "Id", "Nome" FROM "Categoria" WHERE "Tipo" = ? ORDER BY "Id"`,
		categorizacao.CategoriaTransferencia)
	if err != nil {
		return nil, fmt.Errorf("ler as categorias de transferência: %w", err)
	}
	defer linhas.Close()
	var lista []struct {
		id   int64
		nome string
	}
	for linhas.Next() {
		var c struct {
			id   int64
			nome string
		}
		if err := linhas.Scan(&c.id, &c.nome); err != nil {
			return nil, fmt.Errorf("ler categoria de transferência: %w", err)
		}
		lista = append(lista, c)
	}
	return lista, linhas.Err()
}

// CategoriaGenerica é a categoria "Transferências" (ou "Transferência"), do tipo transferência.
func CategoriaGenerica(ctx context.Context, q consultas.DBTX) (*int64, error) {
	lista, err := categoriasDeTransferencia(ctx, q)
	if err != nil {
		return nil, err
	}
	for _, c := range lista {
		if n := texto.ParaRegras(c.nome); n == "TRANSFERENCIAS" || n == "TRANSFERENCIA" {
			id := c.id
			return &id, nil
		}
	}
	return nil, nil
}

// Reclassificar é o ReclassificarAsync: decide, para cada lançamento da categoria genérica, se é entre contas suas ou
// com outra pessoa, depois de acertar os pagamentos de fatura pela conta. Grava o que mudou (com AtualizadoEm = agora)
// e devolve quantos mudaram.
func Reclassificar(ctx context.Context, q consultas.DBTX, agora tipos.DataHora) (int, error) {
	cartoes, err := categorizacao.Cartoes(ctx, q)
	if err != nil {
		return 0, err
	}
	mudaram, err := pagamentosDeFatura(ctx, q, cartoes, agora)
	if err != nil {
		return 0, err
	}
	categoria, err := CategoriaGenerica(ctx, q)
	if err != nil || categoria == nil {
		return mudaram, err
	}
	lancamentos, err := transacoes.Listar(ctx, q, `WHERE "CategoriaId" = ?`, *categoria)
	if err != nil || len(lancamentos) == 0 {
		return mudaram, err
	}
	pareados, err := pareados(ctx, q, lancamentos)
	if err != nil {
		return 0, err
	}
	var nome *string
	err = q.QueryRowContext(ctx, `SELECT "Valor" FROM "Configuracao" WHERE "Chave" = ?`, ChaveMeuNome).Scan(&nome)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("ler o seu nome nos Pix: %w", err)
	}
	meuNome := PalavrasDoNome(nome)
	for _, t := range lancamentos {
		if EntreContasSuas(t, pareados[t.Id], meuNome) {
			if t.Tipo == transacoes.Transferencia && t.IgnorarRelatorios {
				continue
			}
			t.Tipo = transacoes.Transferencia
			t.IgnorarRelatorios = true
		} else {
			// Só muda o que ainda está como transferência: um lançamento que você mandou ignorar nos relatórios
			// depois de reclassificado continua ignorado.
			if t.Tipo != transacoes.Transferencia {
				continue
			}
			t.Tipo = categorizacao.TipoPeloSinal(t.Valor, cartoes[t.ContaId])
			t.IgnorarRelatorios = false
		}
		t.AtualizadoEm = agora
		mudaram++
	}
	return mudaram, transacoes.GravarTodas(ctx, q, lancamentos)
}

// EntreContasSuas: lançada pelo site, mesmo titular, com par, reserva ou (#172) Pix que cita o seu nome. O titular
// informado pelo banco vale sempre: outro titular nunca é.
func EntreContasSuas(t *transacoes.Transacao, temPar bool, meuNome []string) bool {
	if t.TransferenciaGrupo != nil {
		return true
	}
	if t.MesmaTitularidade != nil {
		return *t.MesmaTitularidade
	}
	descricao := t.Descricao
	if t.DescricaoOriginal != nil {
		descricao = *t.DescricaoOriginal
	}
	return temPar || EhMovimentoDeReserva(&descricao) || (meuNome != nil && CitaONome(&descricao, meuNome))
}

var prefixosDeReserva = []string{"Dinheiro reservado ", "Dinheiro retirado "}

// EhMovimentoDeReserva é o ReservaService.EhMovimentoDeReserva: dinheiro indo para uma reserva ou voltando dela
// (o começo da descrição, sem diferença de maiúsculas).
func EhMovimentoDeReserva(descricao *string) bool {
	if descricao == nil {
		return false
	}
	for _, p := range prefixosDeReserva {
		if len(*descricao) >= len(p) && strings.EqualFold((*descricao)[:len(p)], p) {
			return true
		}
	}
	return false
}

var conectivos = map[string]bool{"DE": true, "DA": true, "DO": true, "DAS": true, "DOS": true, "E": true}

// PalavrasDoNome (#172): o primeiro e o último nome, normalizados, que os bancos mantêm mesmo abreviando o meio.
// nil com menos de dois nomes: um nome só pegaria Pix de qualquer xará.
func PalavrasDoNome(nome *string) []string {
	n := ""
	if nome != nil {
		n = *nome
	}
	pedacos := strings.FieldsFunc(texto.ParaRegras(n), func(r rune) bool { return r == ' ' || r == '.' || r == ',' || r == '-' })
	var palavras []string
	for _, p := range pedacos {
		p = strings.TrimSpace(p)
		if respostas.TamanhoUtf16(p) >= 2 && !conectivos[p] {
			palavras = append(palavras, p)
		}
	}
	if len(palavras) < 2 {
		return nil
	}
	return []string{palavras[0], palavras[len(palavras)-1]}
}

// CitaONome: a descrição tem o primeiro e o último nome como palavras inteiras. Como o C#, que olha um char por vez,
// um caractere fora do plano básico vira dois espaços (as metades do par UTF-16 não são letra).
func CitaONome(descricao *string, palavras []string) bool {
	d := ""
	if descricao != nil {
		d = *descricao
	}
	var b strings.Builder
	b.WriteByte(' ')
	for _, r := range texto.ParaRegras(d) {
		switch {
		case r > 0xFFFF:
			b.WriteString("  ")
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			b.WriteByte(' ')
		}
	}
	b.WriteByte(' ')
	comEspacos := b.String()
	for _, p := range palavras {
		if !strings.Contains(comEspacos, " "+p+" ") {
			return false
		}
	}
	return true
}

// pagamentosDeFatura (#172): débito numa conta que é pagamento de fatura (pelo texto ou pela operação) e tem o crédito
// do mesmo valor num cartão seu, em até DiasDoPar dias, vira transferência na categoria "Pagamento de fatura". Sem
// esse crédito fica como está ("fatura" também é conta de telefone). Não mexe em categoria escolhida à mão.
func pagamentosDeFatura(ctx context.Context, q consultas.DBTX, cartoes map[int64]bool, agora tipos.DataHora) (int, error) {
	if len(cartoes) == 0 {
		return 0, nil
	}
	transferencias, err := categoriasDeTransferencia(ctx, q)
	if err != nil {
		return 0, err
	}
	var categoria *int64
	for _, c := range transferencias {
		if texto.ParaRegras(c.nome) == "PAGAMENTO DE FATURA" {
			id := c.id
			categoria = &id
			break
		}
	}
	if categoria == nil {
		if categoria, err = CategoriaGenerica(ctx, q); err != nil || categoria == nil {
			return 0, err
		}
	}
	idsCartoes := chaves(cartoes)
	idsTransferencia := make([]int64, len(transferencias))
	for i, c := range transferencias {
		idsTransferencia[i] = c.id
	}

	// Filtro grosso no banco (o LIKE do SQLite, como o EF.Functions.Like); a regra certa é a do tipolancamento.
	argumentos := []any{}
	onde := `WHERE NOT "CategoriaManual" AND "TransferenciaGrupo" IS NULL AND "ContaId" NOT IN (` + lista(idsCartoes, &argumentos) + `)
		AND ("CategoriaId" IS NULL OR "CategoriaId" NOT IN (` + lista(idsTransferencia, &argumentos) + `))
		AND ("MeioPagamento" = 'PAGAMENTO_FATURA' OR COALESCE("DescricaoOriginal", "Descricao") LIKE '%fatura%'
			OR COALESCE("DescricaoOriginal", "Descricao") LIKE '%cart%')`
	todos, err := transacoes.Listar(ctx, q, onde, argumentos...)
	if err != nil {
		return 0, err
	}
	var candidatos []*transacoes.Transacao
	for _, t := range todos {
		if t.Valor.Decimal().IsNegative() && tipolancamento.Classificar(t.Valor, t.Descricao, t.DescricaoOriginal,
			t.MeioPagamento, t.MesmaTitularidade, false, tipolancamento.ContaCorrente) == tipolancamento.PagamentoFatura {
			candidatos = append(candidatos, t)
		}
	}
	if len(candidatos) == 0 {
		return 0, nil
	}
	slices.SortStableFunc(candidatos, porDataEId)
	de, ate := candidatos[0].Data.SomarDias(-DiasDoPar), maiorData(candidatos).SomarDias(DiasDoPar)

	// O C# não tem ORDER BY: vale a ordem do índice IX_Transacao_Conta_Data (ContaId, Data DESC), que a consulta dele
	// e esta usam. Aqui ela fica escrita, sem custo (o plano é o mesmo), para o empate não depender do plano.
	argumentos = []any{}
	linhas, err := q.QueryContext(ctx, `SELECT "Id", "Data", "Valor" FROM "Transacao" WHERE "ContaId" IN (`+
		lista(idsCartoes, &argumentos)+`) AND "Data" >= ? AND "Data" <= ? ORDER BY "ContaId", "Data" DESC, "Id"`,
		append(argumentos, de, ate)...)
	if err != nil {
		return 0, fmt.Errorf("ler os créditos nos cartões: %w", err)
	}
	type credito struct {
		id    int64
		data  tipos.Data
		valor tipos.Decimal
	}
	var creditos []credito
	for linhas.Next() {
		var c credito
		if err := linhas.Scan(&c.id, &c.data, &c.valor); err != nil {
			linhas.Close()
			return 0, fmt.Errorf("ler crédito no cartão: %w", err)
		}
		if c.valor.Decimal().IsPositive() {
			creditos = append(creditos, c)
		}
	}
	linhas.Close()

	usados := map[int64]bool{}
	mudaram := 0
	for _, t := range candidatos {
		oposto := t.Valor.Decimal().Neg()
		achou, melhor := false, credito{}
		for _, c := range creditos {
			distancia := abs(dias(c.data, t.Data))
			if !c.valor.Decimal().Equal(oposto) || usados[c.id] || distancia > DiasDoPar {
				continue
			}
			// OrderBy(distância).FirstOrDefault(): o primeiro de menor distância, na ordem do índice (data maior antes).
			if !achou || distancia < abs(dias(melhor.data, t.Data)) {
				achou, melhor = true, c
			}
		}
		if !achou {
			continue
		}
		usados[melhor.id] = true
		id := *categoria
		t.CategoriaId = &id
		t.Tipo = transacoes.Transferencia
		t.IgnorarRelatorios = true
		t.AtualizadoEm = agora
		mudaram++
	}
	return mudaram, transacoes.GravarTodas(ctx, q, candidatos)
}

// pareados: os lançamentos que têm o outro lado em outra conta sua (mesmo valor, sinal oposto, até DiasDoPar dias), em
// qualquer categoria. Cada lado forma um par só, o de data mais próxima (e, empatando, o de menor Id).
func pareados(ctx context.Context, q consultas.DBTX, lancamentos []*transacoes.Transacao) (map[int64]bool, error) {
	ordenados := slices.Clone(lancamentos)
	slices.SortStableFunc(ordenados, porDataEId)
	de, ate := ordenados[0].Data.SomarDias(-DiasDoPar), maiorData(ordenados).SomarDias(DiasDoPar)
	linhas, err := q.QueryContext(ctx, `SELECT "Id", "ContaId", "Data", "Valor" FROM "Transacao" WHERE "Data" >= ? AND "Data" <= ?`, de, ate)
	if err != nil {
		return nil, fmt.Errorf("ler as transações do período: %w", err)
	}
	type outro struct {
		id, contaId int64
		data        tipos.Data
	}
	porValor := map[string][]outro{}
	for linhas.Next() {
		var o outro
		var valor tipos.Decimal
		if err := linhas.Scan(&o.id, &o.contaId, &o.data, &valor); err != nil {
			linhas.Close()
			return nil, fmt.Errorf("ler transação do período: %w", err)
		}
		chave := chaveDoValor(valor)
		porValor[chave] = append(porValor[chave], o)
	}
	linhas.Close()

	usados := map[int64]bool{}
	for _, t := range ordenados {
		if usados[t.Id] || t.Valor.Decimal().IsZero() {
			continue
		}
		opostos, ok := porValor[chaveDoValor(tipos.DeDecimal(t.Valor.Decimal().Neg()))]
		if !ok {
			continue
		}
		achou, melhor := false, outro{}
		for _, o := range opostos {
			distancia := abs(dias(o.data, t.Data))
			if o.contaId == t.ContaId || usados[o.id] || distancia > DiasDoPar {
				continue
			}
			if d := abs(dias(melhor.data, t.Data)); !achou || distancia < d || (distancia == d && o.id < melhor.id) {
				achou, melhor = true, o
			}
		}
		if !achou {
			continue
		}
		usados[t.Id] = true
		usados[melhor.id] = true
	}
	return usados, nil
}

// chaveDoValor agrupa pelo valor sem a escala, como a igualdade do decimal do C# (10.0 == 10.00).
func chaveDoValor(v tipos.Decimal) string { return v.Decimal().String() }

func porDataEId(a, b *transacoes.Transacao) int {
	if c := strings.Compare(a.Data.String(), b.Data.String()); c != 0 {
		return c
	}
	switch {
	case a.Id < b.Id:
		return -1
	case a.Id > b.Id:
		return 1
	}
	return 0
}

func maiorData(lista []*transacoes.Transacao) tipos.Data {
	maior := lista[0].Data
	for _, t := range lista[1:] {
		if t.Data.Depois(maior) {
			maior = t.Data
		}
	}
	return maior
}

// dias é o DayNumber de a menos o de b.
func dias(a, b tipos.Data) int {
	ta := time.Date(a.Ano(), a.Mes(), a.Dia(), 0, 0, 0, 0, time.UTC)
	tb := time.Date(b.Ano(), b.Mes(), b.Dia(), 0, 0, 0, 0, time.UTC)
	return int(ta.Sub(tb).Hours() / 24)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func chaves(m map[int64]bool) []int64 {
	lista := make([]int64, 0, len(m))
	for id := range m {
		lista = append(lista, id)
	}
	slices.Sort(lista)
	return lista
}

// lista escreve "?, ?, ?" para os ids e põe os ids nos argumentos (o IN do SQL com parâmetros).
func lista(ids []int64, argumentos *[]any) string {
	marcas := make([]string, len(ids))
	for i, id := range ids {
		marcas[i] = "?"
		*argumentos = append(*argumentos, id)
	}
	return strings.Join(marcas, ", ")
}
