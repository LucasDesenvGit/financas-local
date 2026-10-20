import { Conta, TipoConta } from '../core/modelos';

/**
 * #171: o limite usado que o banco informa (aprovado − disponível) costuma ser a dívida do cartão. Bem maior que ela,
 * ou você reduziu o limite no app (por segurança) ou há compromissos que ainda não entraram na dívida (compras em
 * processamento, parcelas). O site não sabe qual: pergunta.
 *
 * Limiares (julgamento próprio, issue #171): mais de 2× a dívida e mais de R$ 1.000 além dela. Depois de um "não", a
 * pergunta volta quando a diferença muda mais de 20% (e de R$ 100).
 */
export const LIMITE_USADO_VEZES_DIVIDA = 2;
export const LIMITE_DIFERENCA_MINIMA = 1000;
export const LIMITE_MUDANCA_RELEVANTE = 0.2;

export type EstadoLimite = 'perguntar' | 'ajustado' | 'nao' | null;

/** Tudo o que se deve no cartão hoje (o saldo do cartão é negativo quando há dívida). */
export function dividaDoCartao(c: Conta): number {
  return Math.max(0, -c.saldo);
}

/** Limite usado segundo o banco (aprovado − disponível); null sem os dois valores. */
export function limiteUsado(c: Conta): number | null {
  return c.limiteCredito === null || c.limiteDisponivel === null ? null : c.limiteCredito - c.limiteDisponivel;
}

/** Quanto do limite usado vai além da dívida do cartão; null sem os dois limites. */
export function diferencaDoLimite(c: Conta): number | null {
  const usado = limiteUsado(c);
  return usado === null ? null : usado - dividaDoCartao(c);
}

export function estadoDoLimite(c: Conta): EstadoLimite {
  if (c.tipo !== TipoConta.CartaoCredito) return null;
  const diferenca = diferencaDoLimite(c);
  if (diferenca === null) return null;
  if (c.limiteAjustadoNoApp === true) return 'ajustado';
  const divida = dividaDoCartao(c);
  const grande = diferenca + divida > LIMITE_USADO_VEZES_DIVIDA * divida && diferenca > LIMITE_DIFERENCA_MINIMA;
  if (c.limiteAjustadoNoApp === false) {
    const avaliada = c.limiteDiferencaAvaliada ?? 0;
    const mudou = Math.abs(diferenca - avaliada) > Math.max(100, Math.abs(avaliada) * LIMITE_MUDANCA_RELEVANTE);
    return mudou && grande ? 'perguntar' : 'nao';
  }
  return grande ? 'perguntar' : null;
}
