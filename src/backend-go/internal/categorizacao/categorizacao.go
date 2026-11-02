// Pacote categorizacao é o CategorizacaoService do .NET: define a categoria das transações, nesta ordem de
// prioridade: a escolhida à mão (nunca sobrescrita, quem confere é quem chama), as regras de categorização e o
// mapeamento da categoria que a Pluggy sugere. Também acerta o tipo (despesa, receita, transferência) e o "fora dos
// relatórios" pela categoria e pelo valor (ADR 0036). Fica num pacote próprio porque a sincronização (#83) usa o mesmo.
//
// Os acertos de dados da partida (regras padrão novas, entradas nos cartões, compras parceladas estornadas) estão em
// acertos.go (#444); quem os chama ao subir é o internal/partida, só quando o Go está na frente do .NET.
package categorizacao

import (
	"context"
	"fmt"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipolancamento"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
)

// Tipos de categoria (o enum TipoCategoria do C#).
const (
	CategoriaDespesa       = 1
	CategoriaReceita       = 2
	CategoriaTransferencia = 3
)

// Tipos de correspondência de uma regra (o enum TipoCorrespondencia do C#).
const (
	Contem    = 1
	ComecaCom = 2
	Igual     = 3
)

type regra struct {
	texto       string // já normalizado
	tipo        int64
	categoriaId int64
}

// Regras é o CategorizacaoService depois do CarregarAsync: as regras ativas, os tipos das categorias, o mapa da
// Pluggy e os cartões. Leia de novo (Carregar) depois de mudar regras ou categorias.
type Regras struct {
	regras                   []regra
	tiposCategoria           map[int64]int64
	mapaPluggy               map[string]*int64
	cartoes                  map[int64]bool
	categoriaPagamentoFatura *int64
}

// Carregar é o CarregarAsync.
func Carregar(ctx context.Context, q consultas.DBTX) (*Regras, error) {
	rs := &Regras{tiposCategoria: map[int64]int64{}, mapaPluggy: map[string]*int64{}, cartoes: map[int64]bool{}}

	linhas, err := q.QueryContext(ctx, `SELECT "Texto", "TipoCorrespondencia", "CategoriaId" FROM "RegraCategorizacao"
		WHERE "Ativa" ORDER BY "Prioridade", "Id"`)
	if err != nil {
		return nil, fmt.Errorf("ler as regras de categorização: %w", err)
	}
	for linhas.Next() {
		var r regra
		if err := linhas.Scan(&r.texto, &r.tipo, &r.categoriaId); err != nil {
			linhas.Close()
			return nil, fmt.Errorf("ler regra de categorização: %w", err)
		}
		r.texto = texto.ParaRegras(r.texto)
		rs.regras = append(rs.regras, r)
	}
	linhas.Close()

	// Sem ORDER BY no C#: o SQLite devolve pelo Id. Com dois nomes que ficam iguais normalizados, vale o primeiro.
	linhas, err = q.QueryContext(ctx, `SELECT "Id", "Tipo", "Nome" FROM "Categoria" ORDER BY "Id"`)
	if err != nil {
		return nil, fmt.Errorf("ler as categorias: %w", err)
	}
	for linhas.Next() {
		var id, tipo int64
		var nome string
		if err := linhas.Scan(&id, &tipo, &nome); err != nil {
			linhas.Close()
			return nil, fmt.Errorf("ler categoria: %w", err)
		}
		rs.tiposCategoria[id] = tipo
		if rs.categoriaPagamentoFatura == nil && tipo == CategoriaTransferencia && texto.ParaRegras(nome) == "PAGAMENTO DE FATURA" {
			rs.categoriaPagamentoFatura = &id
		}
	}
	linhas.Close()

	if rs.cartoes, err = Cartoes(ctx, q); err != nil {
		return nil, err
	}

	linhas, err = q.QueryContext(ctx, `SELECT "PluggyCategoriaId", "CategoriaId", "PaiPluggyId" FROM "CategoriaPluggy"`)
	if err != nil {
		return nil, fmt.Errorf("ler o mapa da Pluggy: %w", err)
	}
	type mapa struct {
		id, pai     *string
		categoriaId *int64
	}
	var mapas []mapa
	porId := map[string]*int64{}
	for linhas.Next() {
		var m mapa
		var id string
		if err := linhas.Scan(&id, &m.categoriaId, &m.pai); err != nil {
			linhas.Close()
			return nil, fmt.Errorf("ler mapa da Pluggy: %w", err)
		}
		m.id = &id
		mapas = append(mapas, m)
		porId[id] = m.categoriaId
	}
	linhas.Close()
	for _, m := range mapas {
		// Se a subcategoria não tiver mapeamento, herda o da categoria pai.
		categoria := m.categoriaId
		if categoria == nil && m.pai != nil {
			if doPai, ok := porId[*m.pai]; ok {
				categoria = doPai
			}
		}
		rs.mapaPluggy[*m.id] = categoria
	}
	return rs, nil
}

// Cartoes são os ids das contas de cartão de crédito (#166: entrada nelas é pagamento de fatura ou estorno).
func Cartoes(ctx context.Context, q consultas.DBTX) (map[int64]bool, error) {
	linhas, err := q.QueryContext(ctx, `SELECT "Id" FROM "Conta" WHERE "Tipo" = ?`, tipolancamento.CartaoCredito)
	if err != nil {
		return nil, fmt.Errorf("ler os cartões: %w", err)
	}
	defer linhas.Close()
	cartoes := map[int64]bool{}
	for linhas.Next() {
		var id int64
		if err := linhas.Scan(&id); err != nil {
			return nil, fmt.Errorf("ler cartão: %w", err)
		}
		cartoes[id] = true
	}
	return cartoes, linhas.Err()
}

// ResolverTexto é o Resolver(descricao, pluggyCategoriaId) do C#: a categoria pelas regras e pelo mapa da Pluggy.
func (rs *Regras) ResolverTexto(descricao string, pluggyCategoriaId *string) *int64 {
	return rs.resolver(descricao, nil, pluggyCategoriaId)
}

// Resolver é o Resolver(Transacao) (#250): olha a descrição do banco e o nome que a transação tem agora (dado por uma
// regra de descrição ou escolhido à mão).
func (rs *Regras) Resolver(t *transacoes.Transacao) *int64 {
	descricao := t.Descricao
	if t.DescricaoOriginal != nil {
		descricao = *t.DescricaoOriginal
	}
	return rs.resolver(descricao, &t.Descricao, t.PluggyCategoriaId)
}

func (rs *Regras) resolver(descricao string, outraDescricao, pluggyCategoriaId *string) *int64 {
	t := texto.ParaRegras(descricao)
	var outro *string
	if outraDescricao != nil {
		if o := texto.ParaRegras(*outraDescricao); o != t {
			outro = &o
		}
	}
	for _, r := range rs.regras {
		corresponde := Corresponde(t, r.texto, r.tipo) || (outro != nil && Corresponde(*outro, r.texto, r.tipo))
		if _, existe := rs.tiposCategoria[r.categoriaId]; corresponde && existe {
			id := r.categoriaId
			return &id
		}
	}
	if pluggyCategoriaId != nil {
		if mapeada, ok := rs.mapaPluggy[*pluggyCategoriaId]; ok {
			return mapeada
		}
	}
	return nil
}

// Corresponde compara o texto e a regra, os dois já normalizados, pelo tipo da regra (o que não é "começa com" nem
// "igual" é "contém", como o default do switch do C#).
func Corresponde(t, regraNormalizada string, tipo int64) bool {
	switch tipo {
	case ComecaCom:
		return strings.HasPrefix(t, regraNormalizada)
	case Igual:
		return t == regraNormalizada
	}
	return strings.Contains(t, regraNormalizada)
}

// TipoPeloSinal é o tipo de um lançamento que não é transferência (ADR 0036): saída é despesa; entrada é receita,
// menos no cartão de crédito, onde a entrada que não é pagamento de fatura é estorno (despesa com valor positivo).
func TipoPeloSinal(valor tipos.Decimal, noCartao bool) int64 {
	if valor.Decimal().IsNegative() || noCartao {
		return transacoes.Despesa
	}
	return transacoes.Receita
}

// NoCartao diz se a conta é um cartão de crédito.
func (rs *Regras) NoCartao(contaId int64) bool { return rs.cartoes[contaId] }

// TipoDaCategoria é o tipo da categoria, se ela existe.
func (rs *Regras) TipoDaCategoria(categoriaId *int64) (int64, bool) {
	if categoriaId == nil {
		return 0, false
	}
	tipo, ok := rs.tiposCategoria[*categoriaId]
	return tipo, ok
}

// AplicarTipo ajusta Tipo e IgnorarRelatorios pelo tipo da categoria e pelo valor. No cartão (#166, ADR 0036), uma
// entrada é pagamento de fatura (transferência, pelo texto ou pela categoria) ou estorno (despesa positiva); o estorno
// que chegou na categoria genérica de transferências ganha a categoria do texto sem o "Estorno de", ou fica sem.
func (rs *Regras) AplicarTipo(t *transacoes.Transacao) {
	ehTransferencia := rs.ehTransferencia(t.CategoriaId)
	positivo := t.Valor.Decimal().IsPositive()
	if rs.NoCartao(t.ContaId) && positivo && t.TransferenciaGrupo == nil && !(t.CategoriaManual && ehTransferencia) {
		if rs.ehPagamentoNoCartao(t) {
			if !t.CategoriaManual && rs.categoriaPagamentoFatura != nil {
				id := *rs.categoriaPagamentoFatura
				t.CategoriaId = &id
			}
			t.Tipo = transacoes.Transferencia
			t.IgnorarRelatorios = true
			return
		}
		// Na categoria de transferências, só o texto de estorno vira estorno aqui; o resto segue a regra das
		// transferências (ADR 0031), que no cartão também dá despesa.
		if !ehTransferencia || ehTextoDeEstorno(t) {
			if ehTransferencia {
				t.CategoriaId = rs.categoriaDoEstorno(t)
			}
			if t.Tipo == transacoes.Transferencia {
				t.IgnorarRelatorios = false
			}
			t.Tipo = transacoes.Despesa
			return
		}
	}
	if ehTransferencia {
		t.Tipo = transacoes.Transferencia
		t.IgnorarRelatorios = true
		return
	}
	if t.Tipo == transacoes.Transferencia && t.TransferenciaGrupo == nil {
		t.IgnorarRelatorios = false
	}
	if t.TransferenciaGrupo == nil {
		t.Tipo = TipoPeloSinal(t.Valor, rs.NoCartao(t.ContaId))
	}
}

func (rs *Regras) ehTransferencia(categoriaId *int64) bool {
	tipo, ok := rs.TipoDaCategoria(categoriaId)
	return ok && tipo == CategoriaTransferencia
}

// ehPagamentoNoCartao: pela categoria "Pagamento de fatura", ou pela operação e o texto (o do banco ou o nome que
// aparece), como o TipoLancamentoService classifica no cartão.
func (rs *Regras) ehPagamentoNoCartao(t *transacoes.Transacao) bool {
	if t.CategoriaId != nil && rs.categoriaPagamentoFatura != nil && *t.CategoriaId == *rs.categoriaPagamentoFatura {
		return true
	}
	pagamento := func(descricao *string) bool {
		d := t.Descricao
		if descricao != nil {
			d = *descricao
		}
		return tipolancamento.Classificar(t.Valor, d, nil, t.MeioPagamento, t.MesmaTitularidade, false,
			tipolancamento.CartaoCredito) == tipolancamento.PagamentoFatura
	}
	return pagamento(t.DescricaoOriginal) || pagamento(&t.Descricao)
}

var palavrasDeEstorno = []string{" ESTORNO ", " ESTORNADO ", " CASHBACK ", " DEVOLUCAO ", " REEMBOLSO "}

func ehTextoDeEstorno(t *transacoes.Transacao) bool {
	original := ""
	if t.DescricaoOriginal != nil {
		original = *t.DescricaoOriginal
	}
	comEspacos := " " + texto.ParaRegras(original+" "+t.Descricao) + " "
	for _, p := range palavrasDeEstorno {
		if strings.Contains(comEspacos, p) {
			return true
		}
	}
	return false
}

// categoriaDoEstorno é a categoria pelo texto sem "Estorno de" e sem a carteira digital ("- NuPay"), se der uma
// categoria que não é de transferência.
func (rs *Regras) categoriaDoEstorno(t *transacoes.Transacao) *int64 {
	origem := t.Descricao
	if t.DescricaoOriginal != nil {
		origem = *t.DescricaoOriginal
	}
	s := texto.ParaRegras(origem)
	for _, prefixo := range []string{"ESTORNO DE ", "ESTORNO DA ", "ESTORNO DO ", "ESTORNO "} {
		if strings.HasPrefix(s, prefixo) {
			s = s[len(prefixo):]
			break
		}
	}
	for _, sufixo := range []string{" NUPAY", " NU PAY"} {
		s = strings.TrimSuffix(s, sufixo)
	}
	c := rs.ResolverTexto(strings.TrimSpace(s), nil)
	if c != nil && !rs.ehTransferencia(c) {
		return c
	}
	return nil
}
