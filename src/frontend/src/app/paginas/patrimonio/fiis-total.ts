import { Fii } from '../../core/modelos';

/** #384: a linha de total de "Meus FIIs" e o destaque ao lado do título. */
export interface TotalFiis {
  /** Quantos fundos há na tabela. */
  fundos: number;
  /** A soma do custo de cada fundo (cotas × preço médio). */
  investido: number;
  /** A soma de cotas × cotação, só dos fundos com cotação. */
  valorAtual: number;
  /** Quantos fundos ficaram fora do valor atual e da valorização, por não ter cotação. */
  semCotacao: number;
  /** O valor atual menos o custo dos fundos com cotação. */
  valorizacao: number;
  /** A valorização sobre o custo dos fundos com cotação, em %; null sem custo. */
  valorizacaoPercentual: number | null;
  /** A soma da renda mensal estimada de cada fundo. */
  rendaMensal: number;
  /** Yield on cost da carteira: a renda do mês sobre o investido, em %; null sem investido. */
  yieldOnCostMensal: number | null;
}

/** Arredonda em centavos: a soma de números com casas decimais em ponto flutuante deixa resto (0,1 + 0,2). */
const centavos = (v: number) => Math.round(v * 100) / 100;

/**
 * #384: soma a tabela "Meus FIIs". O fundo sem cotação entra no investido e na renda, mas fica fora do valor atual e
 * da valorização (a API o conta pelo preço médio, o que mostraria uma valorização de zero que não existe).
 */
export function totalDosFiis(fiis: readonly Fii[]): TotalFiis {
  let investido = 0, valorAtual = 0, custoComCotacao = 0, valorizacao = 0, rendaMensal = 0, semCotacao = 0;
  for (const f of fiis) {
    investido += f.custo;
    rendaMensal += f.rendaMensalEstimada;
    if (f.precoAtual === null) {
      semCotacao++;
      continue;
    }
    valorAtual += f.valorMercado;
    custoComCotacao += f.custo;
    valorizacao += f.valorizacao;
  }
  investido = centavos(investido);
  rendaMensal = centavos(rendaMensal);
  return {
    fundos: fiis.length, investido, valorAtual: centavos(valorAtual), semCotacao, valorizacao: centavos(valorizacao),
    valorizacaoPercentual: custoComCotacao > 0 ? (valorizacao / custoComCotacao) * 100 : null,
    rendaMensal, yieldOnCostMensal: investido > 0 ? (rendaMensal / investido) * 100 : null
  };
}
