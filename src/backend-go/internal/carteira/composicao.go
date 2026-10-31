package carteira

import (
	"context"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// ValorDaCarteira é a parte "Carteira" do ComposicaoAsync: as posições abertas pela última cotação, em reais. Sem cotação (do
// ativo ou da moeda), vale o custo. O valor sai a 2 casas. O resto da composição é do painel (#80).
func (s *Servico) ValorDaCarteira(ctx context.Context) (tipos.Decimal, error) {
	ativos, err := s.ativosPeloId(ctx)
	if err != nil {
		return tipos.Zero, err
	}
	conversor, err := s.cambio.ObterConversor(ctx, nil)
	if err != nil {
		return tipos.Zero, err
	}
	soma := tipos.Zero
	for i := range ativos {
		p, err := Calcular(ativos[i].Operacoes, nil)
		if err != nil {
			return tipos.Zero, err
		}
		if !positivo(p.Quantidade) {
			continue
		}
		if preco := precoEmReais(&ativos[i], conversor); preco != nil {
			soma = tipos.Somar(soma, tipos.Multiplicar(p.Quantidade, *preco))
		} else {
			soma = tipos.Somar(soma, p.Custo)
		}
	}
	return tipos.Arredondar(soma, 2), nil
}
