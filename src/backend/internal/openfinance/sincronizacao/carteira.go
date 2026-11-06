package sincronizacao

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/openfinance/pluggy"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/patrimonio/carteira"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/patrimonio/cotacoes"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// Enums da carteira (Enums.cs).
const (
	ativoFii  int64 = 1
	ativoAcao int64 = 2
	ativoEtf  int64 = 3
	ativoBdr  int64 = 4

	operacaoCompra int64 = 1
	operacaoVenda  int64 = 2

	origemOpenFinance int64 = 2
)

const (
	prefixoDaPosicao = "posicao:"
	// diasParaCasarLancamento: até quantos dias de diferença um lançamento seu é a mesma operação que a corretora mandou depois.
	diasParaCasarLancamento = 5
	observacaoImportada     = "Importada do Open Finance"
	observacaoDaPosicao     = "Posição do Open Finance sem histórico de compras: confira a data e o preço pago."
	fonteOpenFinance        = "Open Finance"
	casasDaOperacao         = 6
	casasDoPreco            = 4
)

// ResultadoCarteira é o ResultadoImportacaoCarteiraDto.
type ResultadoCarteira struct {
	AtivosCriados       int      `json:"ativosCriados"`
	OperacoesImportadas int      `json:"operacoesImportadas"`
	SemHistorico        []string `json:"semHistorico"`
	ComLancamentoManual []string `json:"comLancamentoManual"`
	Erros               []string `json:"erros"`
	Mensagem            *string  `json:"mensagem"`
}

func novoResultadoCarteira() ResultadoCarteira {
	return ResultadoCarteira{SemHistorico: []string{}, ComLancamentoManual: []string{}, Erros: []string{}}
}

func carteiraFalhou(mensagem string) ResultadoCarteira {
	r := novoResultadoCarteira()
	r.Mensagem = &mensagem
	return r
}

// ImportarCarteira registra na carteira, como compras, os FIIs, ações e ETFs que o Open Finance informa, em todas as
// conexões ativas. Erro da Pluggy na autenticação volta como *pluggy.Erro.
func (s *Servico) ImportarCarteira(ctx context.Context) (ResultadoCarteira, error) {
	if !s.emExecucao.CompareAndSwap(false, true) {
		return carteiraFalhou("Há uma sincronização em andamento. Tente em instantes."), nil
	}
	defer s.emExecucao.Store(false)

	id, secret, ok, err := s.credenciais(ctx)
	if err != nil {
		return ResultadoCarteira{}, err
	}
	if !ok {
		return carteiraFalhou(semCredenciais), nil
	}
	sessao, err := s.pluggy.CriarSessao(ctx, id, secret)
	if err != nil {
		return ResultadoCarteira{}, err
	}
	conexoes, err := s.conexoesAtivas(ctx, nil)
	if err != nil {
		return ResultadoCarteira{}, err
	}
	r := &rodada{Servico: s, sessao: sessao}
	total := novoResultadoCarteira()
	for _, conexao := range conexoes {
		parcial, err := r.importarRendaVariavel(ctx, conexao)
		if err != nil {
			return ResultadoCarteira{}, err
		}
		total.AtivosCriados += parcial.AtivosCriados
		total.OperacoesImportadas += parcial.OperacoesImportadas
		total.SemHistorico = append(total.SemHistorico, parcial.SemHistorico...)
		total.ComLancamentoManual = append(total.ComLancamentoManual, parcial.ComLancamentoManual...)
		total.Erros = append(total.Erros, parcial.Erros...)
	}
	return total, nil
}

// investimentoDaCarteira são as colunas do Investimento que a importação usa.
type investimentoDaCarteira struct {
	pluggyId, tipo, nome string
	subtipo, codigo      *string
	emissor              *string
	saldo                tipos.Decimal
	valorAplicado        *tipos.Decimal
	quantidade           *tipos.Decimal
	dataAplicacao        *tipos.Data
}

func (r *rodada) investimentosDaCarteira(ctx context.Context, conexao int64) ([]investimentoDaCarteira, error) {
	linhas, err := r.db.QueryContext(ctx, `SELECT "PluggyInvestmentId", "Tipo", "Nome", "Subtipo", "Codigo", "Emissor", "Saldo", "ValorAplicado", "Quantidade", "DataAplicacao"
		FROM "Investimento" WHERE "ConexaoBancariaId" = ? AND ("Tipo" = 'EQUITY' OR "Tipo" = 'ETF') ORDER BY "Id"`, conexao)
	if err != nil {
		return nil, fmt.Errorf("ler os investimentos: %w", err)
	}
	defer linhas.Close()
	var lista []investimentoDaCarteira
	for linhas.Next() {
		var i investimentoDaCarteira
		if err := linhas.Scan(&i.pluggyId, &i.tipo, &i.nome, &i.subtipo, &i.codigo, &i.emissor, &i.saldo, &i.valorAplicado, &i.quantidade, &i.dataAplicacao); err != nil {
			return nil, fmt.Errorf("ler investimento: %w", err)
		}
		lista = append(lista, i)
	}
	return lista, linhas.Err()
}

// importarRendaVariavel: FIIs, ações e ETFs da conexão viram ativos da carteira, com as compras e vendas da corretora. Sem
// histórico, a posição atual vira uma compra para ajustar. Ativo com lançamentos seus e sem nada da corretora não recebe
// importação, para não duplicar. Cada investimento é gravado à parte: a falha de um não impede os outros.
func (r *rodada) importarRendaVariavel(ctx context.Context, conexao int64) (ResultadoCarteira, error) {
	resultado := novoResultadoCarteira()
	lista, err := r.investimentosDaCarteira(ctx, conexao)
	if err != nil {
		return resultado, err
	}
	for _, inv := range lista {
		ticker := carteira.NormalizarTicker(inv.codigo)
		if ticker == nil {
			ticker = carteira.NormalizarTicker(&inv.nome)
		}
		if ticker == nil {
			continue
		}
		if err := r.importarInvestimento(ctx, inv, *ticker, &resultado); err != nil {
			if ctx.Err() != nil {
				return resultado, ctx.Err()
			}
			r.avisar("Falha ao importar do Open Finance.", "ticker", *ticker, "erro", err)
			resultado.Erros = append(resultado.Erros, *ticker)
		}
	}
	return resultado, nil
}

// operacao é uma linha de OperacaoAtivo.
type operacao struct {
	id                int64
	tipo, origem      int64
	data              tipos.Data
	quantidade, preco tipos.Decimal
	pluggyTransacaoId *string
}

// ativoLocal é o Ativo com as operações, antes de mudar.
type ativoLocal struct {
	id             int64
	existe, manual bool
	precoAtual     *tipos.Decimal
	operacoes      []*operacao
}

func movimentoDaCorretora(o *operacao) bool {
	return o.origem == origemOpenFinance && o.pluggyTransacaoId != nil && !strings.HasPrefix(*o.pluggyTransacaoId, prefixoDaPosicao)
}

func (r *rodada) lerAtivo(ctx context.Context, ticker string) (*ativoLocal, error) {
	a := &ativoLocal{}
	err := r.db.QueryRowContext(ctx, `SELECT "Id", "PrecoManual", "PrecoAtual" FROM "Ativo" WHERE "Ticker" = ?`, ticker).Scan(&a.id, &a.manual, &a.precoAtual)
	if errors.Is(err, sql.ErrNoRows) {
		return a, nil
	}
	if err != nil {
		return nil, err
	}
	a.existe = true
	linhas, err := r.db.QueryContext(ctx, `SELECT "Id", "Tipo", "Origem", "Data", "Quantidade", "PrecoUnitario", "PluggyTransacaoId" FROM "OperacaoAtivo" WHERE "AtivoId" = ? ORDER BY "Id"`, a.id)
	if err != nil {
		return nil, err
	}
	defer linhas.Close()
	for linhas.Next() {
		var o operacao
		if err := linhas.Scan(&o.id, &o.tipo, &o.origem, &o.data, &o.quantidade, &o.preco, &o.pluggyTransacaoId); err != nil {
			return nil, err
		}
		a.operacoes = append(a.operacoes, &o)
	}
	return a, linhas.Err()
}

// novaOperacao é a linha a inserir.
type novaOperacao struct {
	tipo                  int64
	data                  tipos.Data
	quantidade, preco     tipos.Decimal
	taxas                 tipos.Decimal
	idExterno, observacao string
}

// mudancasDoAtivo é o que a importação grava de um investimento, tudo junto.
type mudancasDoAtivo struct {
	precoDaCorretora *tipos.Decimal
	ligadas          map[int64]string
	novas            []novaOperacao
	apagar           []int64
	posicaoId        int64
	posicaoData      *tipos.Data
	posicaoQtd       *tipos.Decimal
	posicaoPreco     *tipos.Decimal
}

func (r *rodada) importarInvestimento(ctx context.Context, inv investimentoDaCarteira, ticker string, resultado *ResultadoCarteira) error {
	ativo, err := r.lerAtivo(ctx, ticker)
	if err != nil {
		return err
	}
	if !ativo.existe {
		resultado.AtivosCriados++
	}
	// Ativo só com lançamentos seus: não importa. Se a corretora já mandou movimentações dele, um lançamento seu só completa o
	// que ela não mandou, e as próximas continuam chegando.
	temDaCorretora := slices.ContainsFunc(ativo.operacoes, movimentoDaCorretora)
	var seus []*operacao
	for _, o := range ativo.operacoes {
		if o.origem != origemOpenFinance {
			seus = append(seus, o)
		}
	}
	if len(seus) > 0 && !temDaCorretora {
		resultado.ComLancamentoManual = append(resultado.ComLancamentoManual, ticker)
		return nil
	}
	m := mudancasDoAtivo{ligadas: map[int64]string{}}
	// Preço unitário informado pela corretora (saldo ÷ quantidade): usado até a primeira cotação.
	if ativo.precoAtual == nil && !ativo.manual && inv.quantidade != nil && inv.quantidade.Decimal().IsPositive() && inv.saldo.Decimal().IsPositive() {
		preco, err := tipos.Dividir(inv.saldo, *inv.quantidade)
		if err != nil {
			return err
		}
		arredondado := tipos.Arredondar(preco, casasDoPreco)
		m.precoDaCorretora = &arredondado
		ativo.precoAtual = &arredondado
	}
	movimentos, err := r.pluggy.ListarMovimentosInvestimento(ctx, r.sessao, inv.pluggyId)
	var erroPluggy *pluggy.Erro
	if errors.As(err, &erroPluggy) {
		r.log.Info("movimentações indisponíveis na Pluggy", "origem", "Open Finance", "ticker", ticker, "erro", erroPluggy.Mensagem)
		movimentos = nil
	} else if err != nil {
		return err
	}
	jaImportados := map[string]bool{}
	for _, o := range ativo.operacoes {
		if o.pluggyTransacaoId != nil {
			jaImportados[*o.pluggyTransacaoId] = true
		}
	}
	hoje := relogio.Hoje(r.relogio)
	temMovimento := false
	for i := range movimentos {
		mov := &movimentos[i]
		var tipo int64
		if mov.Type != nil {
			switch texto.MaiusculasInvariantes(*mov.Type) {
			case "BUY":
				tipo = operacaoCompra
			case "SELL":
				tipo = operacaoVenda
			}
		}
		if tipo == 0 || strings.TrimSpace(mov.Id) == "" {
			continue
		}
		quantidade := tipos.Zero
		if mov.Quantity != nil {
			q, err := mov.Quantity.Decimal()
			if err != nil {
				return err
			}
			quantidade = tipos.DeDecimal(q.Decimal().Abs())
		}
		if !quantidade.Decimal().IsPositive() {
			continue
		}
		temMovimento = true
		idExterno := limitado(mov.Id, 100)
		if jaImportados[idExterno] {
			continue
		}
		jaImportados[idExterno] = true
		data := hoje
		if d := primeiraData(mov.TradeDate, mov.Date); d != nil {
			data = d.Data()
		}
		arredondada := tipos.Arredondar(quantidade, casasDaOperacao)
		// A corretora mandou depois uma operação que você já tinha lançado: liga as duas, sem duplicar.
		if lancada := lancamentoIgual(seus, tipo, arredondada, data); lancada != nil {
			lancada.pluggyTransacaoId = &idExterno
			m.ligadas[lancada.id] = idExterno
			continue
		}
		preco, err := precoDoMovimento(mov, quantidade)
		if err != nil {
			return err
		}
		taxas, err := taxasDoMovimento(mov)
		if err != nil {
			return err
		}
		m.novas = append(m.novas, novaOperacao{tipo, data, arredondada, tipos.Arredondar(preco, casasDaOperacao), tipos.Arredondar(taxas, 2), idExterno, observacaoImportada})
		resultado.OperacoesImportadas++
	}

	if err := r.posicaoSemHistorico(inv, ativo, ticker, temMovimento, temDaCorretora, len(seus), &m, resultado); err != nil {
		return err
	}
	return r.gravarAtivo(ctx, ativo, inv, ticker, &m)
}

func primeiraData(datas ...*pluggy.Instante) *pluggy.Instante {
	for _, d := range datas {
		if d != nil {
			return d
		}
	}
	return nil
}

func lancamentoIgual(seus []*operacao, tipo int64, quantidade tipos.Decimal, data tipos.Data) *operacao {
	for _, o := range seus {
		dias := o.data.NumeroDoDia() - data.NumeroDoDia()
		if dias < 0 {
			dias = -dias
		}
		if o.pluggyTransacaoId == nil && o.tipo == tipo && o.quantidade.Decimal().Cmp(quantidade.Decimal()) == 0 && dias <= diasParaCasarLancamento {
			return o
		}
	}
	return nil
}

func precoDoMovimento(mov *pluggy.MovimentoInvestimento, quantidade tipos.Decimal) (tipos.Decimal, error) {
	if mov.Value != nil {
		preco, err := mov.Value.Decimal()
		if err != nil {
			return tipos.Decimal{}, err
		}
		if preco.Decimal().IsPositive() {
			return preco, nil
		}
	}
	if mov.Amount != nil {
		total, err := mov.Amount.Decimal()
		if err != nil {
			return tipos.Decimal{}, err
		}
		if !total.Decimal().IsZero() {
			return tipos.Dividir(tipos.DeDecimal(total.Decimal().Abs()), quantidade)
		}
	}
	return tipos.Zero, nil
}

// taxasDoMovimento soma as despesas, menos o imposto de renda.
func taxasDoMovimento(mov *pluggy.MovimentoInvestimento) (tipos.Decimal, error) {
	soma := tipos.Zero
	for chave, valor := range mov.Expenses {
		if chave == "incomeTax" || valor == nil {
			continue
		}
		d, err := valor.Decimal()
		if err != nil {
			return tipos.Decimal{}, err
		}
		if d.Decimal().IsPositive() {
			soma = tipos.Somar(soma, d)
		}
	}
	return soma, nil
}

// posicaoSemHistorico: sem histórico, a posição atual vira uma compra para ajustar; quando o histórico chega, ela sai. Só para
// ativo sem histórico: se a Pluggy falhou ao listar as movimentações de um que já tem, a posição não pode virar compra a mais.
func (r *rodada) posicaoSemHistorico(inv investimentoDaCarteira, ativo *ativoLocal, ticker string, temMovimento, temDaCorretora bool, seus int, m *mudancasDoAtivo, resultado *ResultadoCarteira) error {
	idPosicao := prefixoDaPosicao + limitado(inv.pluggyId, 90)
	var posicao *operacao
	for _, o := range ativo.operacoes {
		if o.pluggyTransacaoId != nil && *o.pluggyTransacaoId == idPosicao {
			posicao = o
			break
		}
	}
	if temMovimento && posicao != nil {
		m.apagar = append(m.apagar, posicao.id)
		return nil
	}
	if temMovimento || temDaCorretora || seus > 0 || !inv.saldo.Decimal().IsPositive() {
		return nil
	}
	quantidade := inv.quantidade
	if quantidade == nil || !quantidade.Decimal().IsPositive() {
		quantidade = nil
		if ativo.precoAtual != nil && ativo.precoAtual.Decimal().IsPositive() {
			cotas, err := tipos.Dividir(inv.saldo, *ativo.precoAtual)
			if err != nil {
				return err
			}
			arredondada := tipos.Arredondar(cotas, 0) // cotas = saldo ÷ cotação
			quantidade = &arredondada
		}
	}
	if quantidade == nil || !quantidade.Decimal().IsPositive() {
		return nil
	}
	arredondada := tipos.Arredondar(*quantidade, casasDaOperacao)
	custo := inv.saldo
	if inv.valorAplicado != nil && inv.valorAplicado.Decimal().IsPositive() {
		custo = *inv.valorAplicado
	}
	dividido, err := tipos.Dividir(custo, *quantidade)
	if err != nil {
		return err
	}
	preco := tipos.Arredondar(dividido, casasDaOperacao)
	resultado.SemHistorico = append(resultado.SemHistorico, ticker)
	if posicao == nil {
		data := relogio.Hoje(r.relogio)
		if inv.dataAplicacao != nil {
			data = *inv.dataAplicacao
		}
		m.novas = append(m.novas, novaOperacao{operacaoCompra, data, arredondada, preco, tipos.Zero, idPosicao, observacaoDaPosicao})
		resultado.OperacoesImportadas++
		return nil
	}
	m.posicaoId, m.posicaoData, m.posicaoQtd = posicao.id, inv.dataAplicacao, &arredondada
	// Só atualiza o preço se ainda for o provisório: o usuário pode ter corrigido.
	if posicao.preco.Decimal().IsZero() {
		m.posicaoPreco = &preco
	}
	return nil
}

func tipoPorPluggy(tipo string, subtipo *string, ticker string, emissor *string, nome string) int64 {
	if tipo == "ETF" {
		return ativoEtf
	}
	// O Nubank manda FII com o subtipo "STOCK": o nome do emissor ("... FDO INV IMOB - FII") desempata.
	if cotacoes.PareceFii(emissor) || cotacoes.PareceFii(&nome) {
		return ativoFii
	}
	if subtipo != nil {
		switch texto.MaiusculasInvariantes(*subtipo) {
		case "REAL_ESTATE_FUND":
			return ativoFii
		case "BDR":
			return ativoBdr
		case "STOCK":
			return ativoAcao
		}
	}
	// Final 11 (cota de FII) e 12 e 13 (direitos e recibos de subscrição de FII).
	if strings.HasSuffix(ticker, "11") || strings.HasSuffix(ticker, "12") || strings.HasSuffix(ticker, "13") {
		return ativoFii
	}
	return ativoAcao
}

func (r *rodada) gravarAtivo(ctx context.Context, ativo *ativoLocal, inv investimentoDaCarteira, ticker string, m *mudancasDoAtivo) error {
	return banco.EmTransacao(ctx, r.db, func(tx *sql.Tx) error {
		agora := r.agora()
		if !ativo.existe {
			res, err := tx.ExecContext(ctx, `INSERT INTO "Ativo" ("Ticker", "Tipo", "Moeda", "PrecoManual", "CriadoEm") VALUES (?, ?, 'BRL', 0, ?)`,
				ticker, tipoPorPluggy(texto.MaiusculasInvariantes(inv.tipo), inv.subtipo, ticker, inv.emissor, inv.nome), agora)
			if err != nil {
				return err
			}
			if ativo.id, err = res.LastInsertId(); err != nil {
				return err
			}
		}
		if m.precoDaCorretora != nil {
			if _, err := tx.ExecContext(ctx, `UPDATE "Ativo" SET "PrecoAtual" = ?, "PrecoAtualizadoEm" = ?, "FonteCotacao" = ? WHERE "Id" = ?`, m.precoDaCorretora, agora, fonteOpenFinance, ativo.id); err != nil {
				return err
			}
		}
		for id, externo := range m.ligadas {
			if _, err := tx.ExecContext(ctx, `UPDATE "OperacaoAtivo" SET "PluggyTransacaoId" = ? WHERE "Id" = ?`, externo, id); err != nil {
				return err
			}
		}
		for _, n := range m.novas {
			if _, err := tx.ExecContext(ctx, `INSERT INTO "OperacaoAtivo" ("AtivoId", "Tipo", "Data", "Quantidade", "PrecoUnitario", "Taxas", "Cambio", "Observacao", "Origem",
				"PluggyTransacaoId", "CriadoEm") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, ativo.id, n.tipo, n.data, n.quantidade, n.preco, n.taxas, tipos.DecimalDeInteiro(1), n.observacao,
				origemOpenFinance, n.idExterno, agora); err != nil {
				return err
			}
		}
		for _, id := range m.apagar {
			if _, err := tx.ExecContext(ctx, `DELETE FROM "OperacaoAtivo" WHERE "Id" = ?`, id); err != nil {
				return err
			}
		}
		if m.posicaoQtd == nil {
			return nil
		}
		_, err := tx.ExecContext(ctx, `UPDATE "OperacaoAtivo" SET "Data" = COALESCE(?, "Data"), "Quantidade" = ?, "PrecoUnitario" = COALESCE(?, "PrecoUnitario") WHERE "Id" = ?`,
			m.posicaoData, m.posicaoQtd, m.posicaoPreco, m.posicaoId)
		return err
	})
}
