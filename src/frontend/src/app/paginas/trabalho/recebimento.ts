import { RecebimentosHolerite } from '../../core/modelos';

/** #212: o texto do recebimento do holerite (Nielsen 1: o estado em palavras). A conta é da API. */
export function situacaoDoRecebimento(r: RecebimentosHolerite): { texto: string; tom: 'ok' | 'atencao' | 'neutro' } {
  if (r.liquido === null) return { texto: 'O holerite está sem cálculo: ligue os créditos, e a diferença aparece quando houver a tabela.', tom: 'neutro' };
  if (!r.recebimentos.length) return { texto: 'Nenhum crédito ligado ainda.', tom: 'neutro' };
  const d = r.diferenca ?? 0;
  if (Math.abs(d) < 0.005) return { texto: 'Recebido igual ao líquido estimado.', tom: 'ok' };
  return d < 0
    ? { texto: `Faltam ${reais(-d)} para o líquido estimado: confira se veio outro crédito, ou se há um desconto que o holerite não tem.`, tom: 'atencao' }
    : { texto: `Veio ${reais(d)} a mais que o líquido estimado: confira se há um provento que o holerite não tem.`, tom: 'atencao' };
}

function reais(v: number): string {
  return v.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' });
}
