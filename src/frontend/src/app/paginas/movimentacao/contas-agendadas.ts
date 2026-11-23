import { dataIso, dataIsoValida, paramsTransacoes, paraData, primeiroDiaMes, ultimoDiaMes } from '../../compartilhado/utilidades';
import {
  AvisoContaAgendada, Categoria, Conta, ContaAgendada, FormaCobranca, NOMES_FORMA_COBRANCA, NOMES_RECORRENCIA, Recorrencia,
  SalvarContaAgendada, SugestaoContaAgendada, TipoCategoria, TipoConta
} from '../../core/modelos';
import { formAceitaCofrinho } from './pago-com-cofrinho';

/**
 * Regras da tela Contas a pagar (#777, ADR 0059): texto, totais, o formulário e o que vai para a API. Sem Angular nem
 * API; o "pago com o cofrinho" mora em `pago-com-cofrinho.ts`.
 */

/** As formas de cobrança que o formulário oferece, na ordem da lista. */
export const FORMAS_DE_COBRANCA = [FormaCobranca.AMao, FormaCobranca.DebitoAutomatico, FormaCobranca.CartaoCredito];

export function nomeRecorrencia(r: Recorrencia): string {
  return NOMES_RECORRENCIA[r];
}

export function nomeForma(f: FormaCobranca): string {
  return NOMES_FORMA_COBRANCA[f] ?? NOMES_FORMA_COBRANCA[FormaCobranca.AMao];
}

/** As categorias que a conta a pagar aceita: despesa e ativa. */
export function categoriasDeDespesa(categorias: Categoria[]): Categoria[] {
  return categorias.filter((c) => c.tipo === TipoCategoria.Despesa && c.ativa);
}

/* ------------------------------------------------------------ lista e totais */

/** Soma do que já venceu (só as ativas). */
export function totalVencido(itens: ContaAgendada[]): number {
  return itens.filter((c) => c.ativa && c.diasParaVencer < 0).reduce((s, c) => s + c.valor, 0);
}

/** Soma do que vence de hoje a 30 dias (só as ativas). */
export function totalProximos30(itens: ContaAgendada[]): number {
  return itens.filter((c) => c.ativa && c.diasParaVencer >= 0 && c.diasParaVencer <= 30).reduce((s, c) => s + c.valor, 0);
}

/** Quando vence, em texto, embaixo da data. */
export function textoDiasParaVencer(dias: number): string {
  return dias < 0 ? `vencida há ${-dias}d` : dias === 0 ? 'vence hoje' : `em ${dias} dias`;
}

/** #242 (ADR 0024): o banco que vale para a conta (null = selo neutro). */
export function bancoDe(contas: Conta[], contaId: number | null): string | null {
  return contas.find((c) => c.id === contaId)?.banco ?? null;
}

export function ehCartao(contas: Conta[], contaId: number | null): boolean {
  return contas.find((c) => c.id === contaId)?.tipo === TipoConta.CartaoCredito;
}

/* ------------------------------------------------------------ formulário */

export function formularioVazio(hoje = dataIso()): SalvarContaAgendada {
  return { descricao: '', valor: 0, proximoVencimento: hoje, recorrencia: Recorrencia.Mensal, categoriaId: null, contaId: null, ativa: true,
    dataLimite: null, formaCobranca: FormaCobranca.AMao, textoCobranca: null, observacao: null, reservaNome: null };
}

export function formularioDaConta(c: ContaAgendada): SalvarContaAgendada {
  return {
    descricao: c.descricao, valor: c.valor, proximoVencimento: c.proximoVencimento, recorrencia: c.recorrencia,
    categoriaId: c.categoriaId, contaId: c.contaId, ativa: c.ativa, dataLimite: c.dataLimite,
    formaCobranca: c.formaCobranca ?? FormaCobranca.AMao, textoCobranca: c.textoCobranca, observacao: c.observacao,
    reservaNome: c.reservaNome
  };
}

export function formularioDaSugestao(s: SugestaoContaAgendada): SalvarContaAgendada {
  return {
    descricao: s.descricao, valor: s.valor, proximoVencimento: s.proximoVencimento, recorrencia: s.recorrencia ?? Recorrencia.Mensal,
    categoriaId: s.categoriaId, contaId: s.contaId, ativa: true, dataLimite: null,
    formaCobranca: s.formaCobranca ?? FormaCobranca.AMao, textoCobranca: s.textoCobranca ?? null, observacao: null,
    // #205: a conta a pagar guarda de que sugestão veio; renomear não a traz de volta.
    chaveExtrato: s.chaveExtrato ?? null, reservaNome: null
  };
}

/** #69: data limite antes do próximo vencimento (só importa com a conta ativa; a API recusa do mesmo jeito). */
export function limiteInvalido(fm: SalvarContaAgendada): boolean {
  return fm.ativa && fm.recorrencia !== Recorrencia.Unica && !!fm.dataLimite && !!fm.proximoVencimento
    && fm.dataLimite < fm.proximoVencimento;
}

/**
 * Próximo vencimento como a API avança ao pagar (DateOnly.AddMonths/AddYears a partir do anterior): dia que não
 * existe no mês cai no último dia (31/01 → 28/02 → 28/03).
 */
export function avancarVencimento(iso: string, recorrencia: Recorrencia): string {
  const [a, m, d] = iso.split('-').map(Number);
  const meses = recorrencia === Recorrencia.Anual ? 12 : 1;
  const alvo = new Date(a, m - 1 + meses, 1);
  const ultimo = new Date(alvo.getFullYear(), alvo.getMonth() + 1, 0).getDate();
  return dataIso(new Date(alvo.getFullYear(), alvo.getMonth(), Math.min(d, ultimo)));
}

/** Pagamentos que faltam pela data limite, contando o próximo (o atalho do formulário mostra os dois jeitos). */
export function restantesDoForm(fm: SalvarContaAgendada): number | null {
  if (!fm.dataLimite || !dataIsoValida(fm.proximoVencimento) || fm.dataLimite < fm.proximoVencimento) return null;
  let n = 0;
  for (let v = fm.proximoVencimento; v <= fm.dataLimite && n < 1200; v = avancarVencimento(v, fm.recorrencia)) n++;
  return n;
}

/** "Faltam N pagamentos" vira a data do N-ésimo vencimento a partir do próximo (null apaga a data limite). */
export function dataLimitePelosRestantes(fm: SalvarContaAgendada, valor: number | string | null): string | null {
  const n = Math.floor(Number(valor));
  if (!valor || !Number.isFinite(n) || n < 1 || !dataIsoValida(fm.proximoVencimento)) return null;
  let v = fm.proximoVencimento;
  for (let i = 1; i < Math.min(n, 1200); i++) v = avancarVencimento(v, fm.recorrencia);
  return v;
}

/** O que vai para a API ao salvar o formulário. */
export function corpoParaSalvar(fm: SalvarContaAgendada, contas: Conta[]): SalvarContaAgendada {
  return {
    ...fm, valor: Number(fm.valor),
    dataLimite: fm.recorrencia === Recorrencia.Unica || !fm.dataLimite ? null : fm.dataLimite,
    // #158: o texto só vale para a cobrada sozinha.
    textoCobranca: fm.formaCobranca === FormaCobranca.AMao ? null : fm.textoCobranca?.trim() || null,
    observacao: fm.observacao?.trim() || null,
    // #691: o cofrinho só vale em conta do Open Finance, fora do cartão.
    reservaNome: formAceitaCofrinho(fm, contas) ? fm.reservaNome?.trim() || null : null
  };
}

/** #158: "Atualizar o valor" ou "Atualizar o catálogo": a conta a pagar fica como a última cobrança. */
export function corpoDoAviso(c: ContaAgendada, a: AvisoContaAgendada): SalvarContaAgendada {
  return {
    ...formularioDaConta(c),
    valor: a.tipo === 'aumentou' && a.valorNovo !== null ? a.valorNovo : c.valor,
    contaId: a.tipo === 'mudou-de-cartao' && a.contaNovaId !== null ? a.contaNovaId : c.contaId
  };
}

/* ------------------------------------------------------------ sugestões do extrato */

/** "Ver pagamentos": Transações com o período dos meses inteiros, só despesa e a busca, sem prender numa conta. */
export function paramsPagamentos(s: SugestaoContaAgendada): Record<string, string> {
  const inicio = paraData(s.primeiroPagamento);
  const fim = paraData(s.ultimoPagamento);
  return {
    ...paramsTransacoes({
      tipo: 'despesa',
      inicio: primeiroDiaMes(inicio.getFullYear(), inicio.getMonth() + 1),
      fim: ultimoDiaMes(fim.getFullYear(), fim.getMonth() + 1)
    }),
    busca: s.termoBusca
  };
}

/** As sugestões que a pessoa ainda não ignorou e as que ignorou (para "Mostrar de novo"). */
export function separarSugestoes(sugestoes: SugestaoContaAgendada[], ignoradas: string[]) {
  return {
    visiveis: sugestoes.filter((s) => !ignoradas.includes(s.chave)),
    ignoradas: sugestoes.filter((s) => ignoradas.includes(s.chave))
  };
}

/** As chaves ignoradas, lidas do que o navegador guardou (texto JSON); o que não é lista de texto vira lista vazia. */
export function chavesIgnoradasDe(texto: string | null): string[] {
  try {
    const valor = JSON.parse(texto ?? '[]');
    return Array.isArray(valor) ? valor.filter((c): c is string => typeof c === 'string') : [];
  } catch {
    return [];
  }
}

/** As chaves a guardar: sem repetir, as 200 últimas. */
export function chavesParaGuardar(chaves: string[]): string[] {
  return [...new Set(chaves)].slice(-200);
}
