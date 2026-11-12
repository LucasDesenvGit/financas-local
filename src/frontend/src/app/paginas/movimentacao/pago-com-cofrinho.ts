import { Conta, ContaAgendada, FormaCobranca, OrigemConta, PagamentoCofrinho, Reserva } from '../../core/modelos';

/**
 * #691 (ADR 0063): "Pago com" um cofrinho nas contas a pagar. O dinheiro sai direto de uma reserva da conta do Open
 * Finance, e o app lança a despesa: no vencimento (débito automático) ou pelo "Pagar" (à mão).
 */

/** Opção do campo "Pago com": um cofrinho da conta, com o saldo (null = não está mais na lista de reservas). */
export interface OpcaoCofrinho { nome: string; saldo: number | null }

/** O cofrinho só existe em conta do Open Finance, e a conta cobrada no cartão não sai dele. */
export function aceitaCofrinho(conta: Conta | undefined, forma: FormaCobranca): boolean {
  return conta?.origem === OrigemConta.OpenFinance && forma !== FormaCobranca.CartaoCredito;
}

/**
 * Cofrinhos da conta, pelo nome (Nielsen 6: escolher, não digitar). O escolhido que sumiu da lista continua nela,
 * marcado, para não ser trocado sem querer (como na meta, #89).
 */
export function opcoesCofrinho(reservas: Reserva[], contaId: number | null, escolhido: string | null): OpcaoCofrinho[] {
  if (contaId === null) return [];
  const opcoes: OpcaoCofrinho[] = reservas.filter((r) => r.contaId === contaId && !r.restanteDoBanco && r.moeda !== 'USD')
    .map((r) => ({ nome: r.nome, saldo: r.saldo }))
    .sort((a, b) => a.nome.localeCompare(b.nome, 'pt-BR'));
  if (escolhido && !opcoes.some((o) => mesmoNome(o.nome, escolhido))) opcoes.push({ nome: escolhido, saldo: null });
  return opcoes;
}

/** O cofrinho da conta a pagar ainda está na lista de reservas da conta (conta + nome, sem diferenciar maiúsculas). */
export function cofrinhoEncontrado(c: ContaAgendada, reservas: Reserva[]): boolean {
  return !c.reservaNome || reservas.some((r) => r.contaId === c.contaId && mesmoNome(r.nome, c.reservaNome!));
}

export function mesmoNome(a: string, b: string): boolean {
  return a.toLocaleUpperCase('pt-BR') === b.toLocaleUpperCase('pt-BR');
}

/**
 * Dá baixa sozinha, sem "Pagar": cobrada sozinha e com o texto (#158, quando a cobrança chega no extrato) ou em débito
 * automático pelo cofrinho (#691, no vencimento).
 */
export function baixaSozinha(c: ContaAgendada): boolean {
  return (c.formaCobranca !== FormaCobranca.AMao && !!c.textoCobranca)
    || (c.formaCobranca === FormaCobranca.DebitoAutomatico && !!c.reservaNome);
}

/**
 * "Pagar" aparece na ativa que não dá baixa sozinha e, no débito pelo cofrinho, também na vencida: a baixa automática
 * foi desfeita (ela não volta sozinha naquele vencimento).
 */
export function mostrarPagar(c: ContaAgendada): boolean {
  if (!c.ativa) return false;
  if (c.formaCobranca === FormaCobranca.DebitoAutomatico && c.reservaNome) return c.diasParaVencer < 0;
  return !baixaSozinha(c);
}

/** O "Pagar" lança a despesa: na conta manual, do saldo; na do Open Finance, só saindo do cofrinho. */
export function podeLancar(c: ContaAgendada, contas: Conta[]): boolean {
  const conta = contas.find((x) => x.id === c.contaId);
  if (!conta) return false;
  return conta.origem === OrigemConta.Manual || (conta.origem === OrigemConta.OpenFinance && !!c.reservaNome);
}

/** "dd/MM" de uma data ISO. */
function diaMes(iso: string): string {
  const [, m, d] = iso.split('-');
  return `${d}/${m}`;
}

/** A última baixa pelo cofrinho, em texto (Nielsen 1: o que o app fez sozinho fica à vista). */
export function textoPagamentoCofrinho(u: PagamentoCofrinho): string {
  return `paga pelo cofrinho ${u.reservaNome} em ${diaMes(u.data)}${u.automatico ? ' (automático)' : ''}`;
}
