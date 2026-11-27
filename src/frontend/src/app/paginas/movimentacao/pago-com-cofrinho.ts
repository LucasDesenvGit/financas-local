import { dataIso } from '../../compartilhado/utilidades';
import { AvisoContaAgendada, Conta, ContaAgendada, FormaCobranca, OrigemConta, PagamentoCofrinho, Reserva } from '../../core/modelos';

/**
 * #691 (ADR 0063): "Pago com" um cofrinho nas contas a pagar. O dinheiro sai direto de uma reserva da conta do Open
 * Finance, e o app lança a despesa: no vencimento (débito automático) ou pelo "Pagar" (à mão).
 */

/** Opção do campo "Pago com": um cofrinho da conta, com o saldo (null = não está mais na lista de reservas). */
export interface OpcaoCofrinho { nome: string; saldo: number | null }

/**
 * #719: o texto da opção é só o nome, como no app do banco. O rótulo do campo já diz que é um cofrinho, e o nome quase
 * sempre já tem a palavra ("Cofrinho da Virada"): o prefixo a repetia (Nielsen 8, sem informação a mais).
 */
export function textoOpcaoCofrinho(o: OpcaoCofrinho): string {
  return o.saldo === null ? `${o.nome} (não encontrado nas reservas)` : o.nome;
}

/** O cofrinho só existe em conta do Open Finance, e a conta cobrada no cartão não sai dele. */
export function aceitaCofrinho(conta: Conta | undefined, forma: FormaCobranca): boolean {
  return conta?.origem === OrigemConta.OpenFinance && forma !== FormaCobranca.CartaoCredito;
}

/** O mesmo, para o formulário: a conta é a escolhida nele (`contaId`) entre as contas da tela. */
export function formAceitaCofrinho(fm: { contaId: number | null; formaCobranca: FormaCobranca }, contas: Conta[]): boolean {
  return aceitaCofrinho(contas.find((c) => c.id === fm.contaId), fm.formaCobranca);
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
export function diaMes(iso: string): string {
  const [, m, d] = iso.split('-');
  return `${d}/${m}`;
}

/** A última baixa pelo cofrinho, em texto (Nielsen 1: o que o app fez sozinho fica à vista). */
export function textoPagamentoCofrinho(u: PagamentoCofrinho): string {
  return `paga pelo cofrinho ${u.reservaNome} em ${diaMes(u.data)}${u.automatico ? ' (automático)' : ''}`;
}

/** O texto da confirmação de "desfazer": o lançamento sai e o vencimento volta (Nielsen 5). */
export function textoDesfazerPagamento(c: ContaAgendada, u: PagamentoCofrinho): string {
  return `Apagar a despesa de ${c.descricao} paga pelo cofrinho ${u.reservaNome} e voltar o vencimento? `
    + (u.automatico ? 'O app não lança de novo sozinho este vencimento: use “Pagar” se ele foi pago.' : '');
}

/** O texto da confirmação de "lançar pelo cofrinho": a despesa do último pagamento que ficou sem ela. */
export function textoLancarPeloCofrinho(c: ContaAgendada, data: string): string {
  return `Lançar a despesa de ${c.descricao} de ${diaMes(data)}, no valor da conta a pagar, saindo do cofrinho ${c.reservaNome}? `
    + 'Se o valor foi outro, ajuste depois em Transações.';
}

/**
 * O "Pagar" aberto: a conta, a data inicial e o cofrinho já escolhido (`null` = o "Pagar" de sempre; texto, mesmo vazio =
 * "foi paga com um cofrinho?", aberto pelo aviso).
 */
export interface AberturaPagamento { conta: ContaAgendada; data: string; cofrinho: string | null }

/** "Foi paga com um cofrinho": a data do vencimento e o primeiro cofrinho da conta já escolhidos. */
export function aberturaPeloAviso(c: ContaAgendada, a: AvisoContaAgendada, reservas: Reserva[], hoje = dataIso()): AberturaPagamento {
  return { conta: c, data: a.data ?? hoje, cofrinho: opcoesCofrinho(reservas, c.contaId, null)[0]?.nome ?? '' };
}

/** O que a janela do "Pagar" guarda enquanto a pessoa escolhe. */
export interface EscolhaDoPagamento { data: string; lancar: boolean; cofrinho: string | null; guardarCofrinho: boolean }

/** A caixa "Lançar a despesa" já vem marcada onde o app pode lançar; no cofrinho, o lançamento é certo. */
export function lancarNoPagar(a: AberturaPagamento, contas: Conta[]): boolean {
  return a.cofrinho !== null || podeLancar(a.conta, contas);
}

/** O que vai para a API ao confirmar. Com cofrinho, a despesa é sempre lançada; sem, só onde o app pode lançar. */
export interface PagamentoParaApi {
  lancar: boolean; data: string; cofrinho?: { reservaNome: string; guardarCofrinho: boolean }
}

export function corpoDoPagamento(conta: ContaAgendada, contas: Conta[], e: EscolhaDoPagamento): PagamentoParaApi {
  const nome = e.cofrinho?.trim();
  const cofrinho = nome ? { reservaNome: nome, guardarCofrinho: e.guardarCofrinho } : undefined;
  return { lancar: cofrinho ? true : e.lancar && podeLancar(conta, contas), data: e.data, cofrinho };
}
