package carteira

import (
	"context"
	"fmt"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cambio"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// ValorDaCarteira é a parte "Carteira" do ComposicaoAsync: as posições abertas pela última cotação, em reais. Sem cotação (do
// ativo ou da moeda), vale o custo. O valor sai a 2 casas.
func (s *Servico) ValorDaCarteira(ctx context.Context) (tipos.Decimal, error) {
	soma, _, _, err := s.posicoesEmReais(ctx)
	if err != nil {
		return tipos.Zero, err
	}
	return tipos.Arredondar(soma, 2), nil
}

// posicoesEmReais soma as posições abertas da carteira em reais (sem arredondar) e devolve os tickers delas (em
// maiúsculas, para comparar sem diferença) e o conversor usado.
func (s *Servico) posicoesEmReais(ctx context.Context) (tipos.Decimal, map[string]bool, *cambio.Conversor, error) {
	ativos, err := s.ativosPeloId(ctx)
	if err != nil {
		return tipos.Zero, nil, nil, err
	}
	conversor, err := s.cambio.ObterConversor(ctx, nil)
	if err != nil {
		return tipos.Zero, nil, nil, err
	}
	soma := tipos.Zero
	tickers := map[string]bool{}
	for i := range ativos {
		p, err := Calcular(ativos[i].Operacoes, nil)
		if err != nil {
			return tipos.Zero, nil, nil, err
		}
		if !positivo(p.Quantidade) {
			continue
		}
		tickers[texto.MaiusculasInvariantes(ativos[i].Ticker)] = true
		if preco := precoEmReais(&ativos[i], conversor); preco != nil {
			soma = tipos.Somar(soma, tipos.Multiplicar(p.Quantidade, *preco))
		} else {
			soma = tipos.Somar(soma, p.Custo)
		}
	}
	return soma, tickers, conversor, nil
}

// Composicao são os valores de investimento do patrimônio, em reais (o ComposicaoInvestimentos do C#).
type Composicao struct {
	Carteira, OpenFinanceRendaVariavel, OpenFinanceDemais, Reservas tipos.Decimal
	MoedasSemCotacao                                                []string
}

// RendaVariavel é a carteira mais as ações e os ETFs do Open Finance.
func (c Composicao) RendaVariavel() tipos.Decimal {
	return tipos.Somar(c.Carteira, c.OpenFinanceRendaVariavel)
}

// Total é a carteira, o Open Finance (renda variável e o resto) e as reservas, somados nessa ordem.
func (c Composicao) Total() tipos.Decimal {
	return tipos.Somar(tipos.Somar(tipos.Somar(c.Carteira, c.OpenFinanceRendaVariavel), c.OpenFinanceDemais), c.Reservas)
}

// Composicao é o ComposicaoAsync do C# (#80): a carteira pela última cotação (ou o custo), os investimentos do Open
// Finance das conexões ativas em reais, sem repetir os que já estão na carteira pelo código (a moeda sem cotação fica
// de fora e é anotada), e as reservas estimadas pelo extrato.
func (s *Servico) Composicao(ctx context.Context) (Composicao, error) {
	carteira, tickers, conversor, err := s.posicoesEmReais(ctx)
	if err != nil {
		return Composicao{}, err
	}
	linhas, err := s.db.QueryContext(ctx, `SELECT i."Saldo", i."Moeda", i."Codigo", i."Tipo" FROM "Investimento" i
		JOIN "ConexaoBancaria" c ON c."Id" = i."ConexaoBancariaId" WHERE c."Ativa" ORDER BY i."Id"`)
	if err != nil {
		return Composicao{}, fmt.Errorf("ler os investimentos da composição: %w", err)
	}
	defer linhas.Close()
	rendaVariavel, demais := tipos.Zero, tipos.Zero
	for linhas.Next() {
		var saldo tipos.Decimal
		var moeda, tipo string
		var codigo *string
		if err := linhas.Scan(&saldo, &moeda, &codigo, &tipo); err != nil {
			return Composicao{}, fmt.Errorf("ler investimento da composição: %w", err)
		}
		if noCodigo(codigo, tickers) {
			continue
		}
		emReais := conversor.ParaReais(saldo, moeda)
		if emReais == nil {
			continue
		}
		if ehRendaVariavel(tipo) {
			rendaVariavel = tipos.Somar(rendaVariavel, *emReais)
		} else {
			demais = tipos.Somar(demais, *emReais)
		}
	}
	if err := linhas.Err(); err != nil {
		return Composicao{}, fmt.Errorf("ler os investimentos da composição: %w", err)
	}
	reservas, err := s.reservas.Total(ctx)
	if err != nil {
		return Composicao{}, err
	}
	return Composicao{Carteira: tipos.Arredondar(carteira, 2), OpenFinanceRendaVariavel: rendaVariavel,
		OpenFinanceDemais: demais, Reservas: reservas, MoedasSemCotacao: conversor.MoedasSemCotacao()}, nil
}
