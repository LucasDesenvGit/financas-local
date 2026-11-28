import { RecebimentosHolerite } from '../../core/modelos';

/** #212: o texto do recebimento do holerite (Nielsen 1: o estado em palavras). A conta é da API. */
export function situacaoDoRecebimento(r: RecebimentosHolerite): { texto: string; tom: 'ok' | 'atencao' | 'neutro' } {
  if (r.liquido === null) return { texto: 'O holerite está sem cálculo: ligue os créditos, e a diferença aparece quando houver a tabela.', tom: 'neutro' };
  if (!r.recebimentos.length) return { texto: 'Nenhum crédito ligado ainda.', tom: 'neutro' };
  const d = r.diferenca ?? 0;
  const alvo = r.adiantamento ? 'esperado' : 'líquido estimado';
  if (Math.abs(d) < 0.005) return { texto: `Recebido igual ao ${alvo}.`, tom: 'ok' };
  return d < 0
    ? { texto: `Faltam ${reais(-d)} para o ${alvo}: confira se veio outro crédito, ou se há um desconto que o holerite não tem.`, tom: 'atencao' }
    : { texto: `Veio ${reais(d)} a mais que o ${alvo}: confira se há um provento que o holerite não tem.`, tom: 'atencao' };
}

/** #768: o esperado em partes, só quando o holerite desconta um adiantamento (Nielsen 1): "adiantamento R$ X + saldo R$ Y". */
export function esperadoEmPartes(r: RecebimentosHolerite): string | null {
  if (!r.adiantamento || r.liquido === null) return null;
  return `adiantamento ${reais(r.adiantamento)} + saldo ${reais(r.liquido)} = ${reais(r.esperado ?? r.liquido + r.adiantamento)}`;
}

function reais(v: number): string {
  return v.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' });
}
