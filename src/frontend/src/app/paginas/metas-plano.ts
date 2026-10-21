import { Meta, PlanoMetas } from '../core/modelos';

/**
 * Plano das metas (#90, #149, ADR 0035): quanto guardar em cada meta, mês a mês, a partir do caixa que a API calcula
 * (receitas − despesas − faturas que vencem no mês). Fica na tela para recalcular na hora quando o usuário muda o
 * percentual ou diz quanto consegue guardar (Nielsen 1 e 3), e para ser testado (o back-end não tem testes).
 *
 * Mês a mês, a partir do atual:
 *  1. capacidade = max(0, sobra × percentual da sobra que vai para as metas);
 *  2. no mês atual, a capacidade também não passa do saldo previsto nas contas no fim do mês (já pagas as faturas) nem
 *     do que de fato sobrou no melhor mês recente. Havendo sobra e saldo, recomenda pelo menos um mínimo pequeno;
 *  3. cada meta com prazo pede o que falta × capacidade do mês ÷ capacidade somada até o prazo (mês apertado pede menos,
 *     folgado pede mais), sem passar de 2× a divisão igual, para não ter picos. O que não coube vai para os meses
 *     seguintes;
 *  4. com folga, todas as metas com prazo recebem o pedido, e o resto vai para as sem prazo (ou de prazo vencido) pela
 *     prioridade; no aperto, decide a prioridade, e a meta sem prazo leva até uma parte justa;
 *  5. o valor que o usuário diz que consegue guardar numa meta este mês vale no lugar da recomendação, e o resto do plano
 *     é refeito sobre o que ainda falta;
 *  6. a partir do mês seguinte, meses com valores parecidos viram um degrau com o mesmo valor (o total não muda).
 * Julgamento próprio, não validado com uso real: o mínimo (R$ 50), o teto (2×) e a tolerância dos degraus.
 */

/** Mínimo recomendado no mês quando há sobra e dinheiro na conta. */
export const MINIMO_DO_MES = 50;
/** Teto mensal de uma meta com prazo, em vezes a divisão igual até o prazo. */
export const TETO_DIVISAO_IGUAL = 2;
/** Dois meses ficam no mesmo degrau quando a diferença é de até R$ 20 ou 15% do valor do degrau. */
const TOLERANCIA_REAIS = 20;
const TOLERANCIA_PERCENTUAL = 0.15;
const MESES_NA_TABELA = 12;

export interface MesRef { ano: number; mes: number }

export interface MesDoPlano extends MesRef { sobra: number; faturas: number; guardar: number }

/** Meses seguidos (a partir do próximo) com o mesmo valor a guardar; `faturas` é a média das faturas desses meses. */
export interface Degrau { inicio: MesRef; fim: MesRef; valor: number; faturas: number }

export interface PlanoDaMeta {
  metaId: number;
  falta: number;
  /** Quanto guardar este mês (o recomendado ou o que o usuário escolheu). */
  recomendado: number;
  /** O valor deste mês veio do usuário ("consigo guardar"), não da recomendação. */
  ajustado: boolean;
  /** Mês (AAAA-MM-01) em que a meta é alcançada nesse plano; null se passa do horizonte. */
  dataProvavel: string | null;
  /** null sem data-alvo. */
  noPrazo: boolean | null;
  /** Quanto os meses seguintes precisam, em média, para cumprir o prazo quando o plano não cumpre. */
  paraManterPrazo: number | null;
  meses: MesDoPlano[];
  /** #318: quanto guardar em cada mês, do atual até o da data provável (ou o horizonte inteiro): o gráfico da meta. */
  ateOAlvo: number[];
  degraus: Degrau[];
  /** O plano conta com as faturas caindo: o degrau maior começa em `aPartir`, com faturas bem menores. */
  dependeDaFatura: { aPartir: MesRef; antes: number; depois: number } | null;
}

export type LimiteDoMes = 'saldo' | 'historico' | 'minimo' | null;

export interface PlanoCalculado {
  sobraMes: number;
  /** Quanto do mês atual pode ir para as metas, já com os limites. */
  limiteMes: number;
  limitadoPor: LimiteDoMes;
  totalRecomendado: number;
  metas: PlanoDaMeta[];
}

const centavos = (v: number) => Math.round(v * 100) / 100;

export function planejarMetas(caixa: PlanoMetas, metas: readonly Meta[], percentual: number,
  ajustes: ReadonlyMap<number, number> = new Map()): PlanoCalculado {
  const n = caixa.meses.length;
  const parte = Math.min(Math.max(percentual, 0), 100) / 100;
  const capacidade = caixa.meses.map((m) => Math.max(0, m.sobra * parte));
  const mesDoIndice = (k: number): MesRef => {
    const total = caixa.ano * 12 + (caixa.mes - 1) + k;
    return { ano: Math.floor(total / 12), mes: (total % 12) + 1 };
  };
  const indicePrazo = (m: Meta): number | null => {
    if (!m.dataAlvo) return null;
    const [ano, mes] = m.dataAlvo.split('-').map(Number);
    return (ano - caixa.ano) * 12 + mes - caixa.mes;
  };

  // Passo 2: limites do mês atual.
  let limitadoPor: LimiteDoMes = null;
  let limite = capacidade[0] ?? 0;
  const peloSaldo = Math.max(0, caixa.saldoFimDoMes) * parte;
  if (peloSaldo < limite) { limite = peloSaldo; limitadoPor = 'saldo'; }
  const peloHistorico = Math.max(0, caixa.melhorMesRecente);
  if (peloHistorico < limite) { limite = peloHistorico; limitadoPor = 'historico'; }
  const minimo = Math.min(MINIMO_DO_MES, capacidade[0] ?? 0, peloSaldo);
  if (limite < minimo) { limite = minimo; limitadoPor = 'minimo'; }
  if (n > 0) capacidade[0] = limite;

  const ativas = metas.filter((m) => !m.concluida && m.valorAlvo - m.valorAtual > 0);
  const faltaInicial = new Map(ativas.map((m) => [m.id, m.valorAlvo - m.valorAtual]));
  const falta = new Map(faltaInicial);
  const prazo = new Map(ativas.map((m) => [m.id, indicePrazo(m)]));
  const teto = new Map(ativas.map((m) => {
    const p = prazo.get(m.id);
    return [m.id, p !== null && p !== undefined && p >= 0 ? TETO_DIVISAO_IGUAL * faltaInicial.get(m.id)! / (p + 1) : null];
  }));
  const alocado = new Map(ativas.map((m) => [m.id, new Array<number>(n).fill(0)]));
  const restante = capacidade.slice();
  const dar = (id: number, k: number, valor: number) => {
    alocado.get(id)![k] += valor;
    restante[k] -= valor;
    falta.set(id, falta.get(id)! - valor);
  };

  // Passo 5: o que o usuário disse que consegue guardar este mês.
  const ajustadas = new Set<number>();
  for (const m of ativas) {
    const v = ajustes.get(m.id);
    if (v === undefined || v === null || !Number.isFinite(v) || v < 0 || n === 0) continue;
    ajustadas.add(m.id);
    const valor = Math.min(v, falta.get(m.id)!);
    alocado.get(m.id)![0] = valor;
    restante[0] = Math.max(0, restante[0] - valor);
    falta.set(m.id, falta.get(m.id)! - valor);
  }

  for (let k = 0; k < n; k++) {
    const livres = ativas.filter((m) => !(k === 0 && ajustadas.has(m.id)));
    // Passo 3: pedido de cada meta com prazo.
    const pedidos = new Map<number, number>();
    for (const m of livres) {
      const f = falta.get(m.id)!;
      const p = prazo.get(m.id);
      if (f <= 0 || p === null || p === undefined || p < k) continue;
      const ate = Math.min(p, n - 1);
      let soma = 0;
      for (let i = k; i <= ate; i++) soma += capacidade[i];
      let pedido = soma > 0 ? f * capacidade[k] / soma : (k === ate ? f : 0);
      const t = teto.get(m.id);
      if (t !== null && t !== undefined) pedido = Math.min(pedido, t);
      pedidos.set(m.id, Math.min(f, pedido));
    }
    // Passo 4: com folga, todas recebem o pedido; no aperto, decide a prioridade.
    const totalPedido = [...pedidos.values()].reduce((s, v) => s + v, 0);
    const folga = restante[k] >= totalPedido;
    const abertas = livres.filter((m) => falta.get(m.id)! > 0).length;
    const parteJusta = abertas > 0 ? capacidade[k] / abertas : 0;
    for (const m of livres) {
      const f = falta.get(m.id)!;
      if (f <= 0 || restante[k] <= 0) continue;
      const limiteMeta = pedidos.get(m.id) ?? (folga ? 0 : parteJusta);
      const valor = Math.min(limiteMeta, restante[k], f);
      if (valor > 0) dar(m.id, k, valor);
    }
    for (const m of livres) {
      if (restante[k] <= 0) break;
      const p = prazo.get(m.id);
      if (falta.get(m.id)! <= 0 || (p !== null && p !== undefined && p >= k)) continue;
      dar(m.id, k, Math.min(restante[k], falta.get(m.id)!));
    }
    // Mínimo do mês atual: quando o pedido das metas com prazo ficou abaixo dele (a capacidade está nos meses seguintes).
    if (k === 0 && ajustadas.size === 0) {
      const total = ativas.reduce((s, m) => s + alocado.get(m.id)![0], 0);
      const primeira = ativas.find((m) => falta.get(m.id)! > 0);
      if (primeira && total < minimo - 0.005) dar(primeira.id, 0, Math.min(minimo - total, falta.get(primeira.id)!));
    }
  }

  const planos = ativas.map((m): PlanoDaMeta => {
    const a = alocado.get(m.id)!;
    const degraus = suavizar(a, caixa, mesDoIndice);
    const total = faltaInicial.get(m.id)!;
    let acumulado = 0;
    let indiceProvavel: number | null = null;
    for (let k = 0; k < n; k++) {
      acumulado += a[k];
      if (acumulado >= total - 0.005) { indiceProvavel = k; break; }
    }
    const p = prazo.get(m.id) ?? null;
    const noPrazo = p === null ? null : indiceProvavel !== null && indiceProvavel <= p;
    const paraManterPrazo = noPrazo === false && p !== null && p >= 1 ? centavos((total - a[0]) / p) : null;
    const provavel = indiceProvavel === null ? null : mesDoIndice(indiceProvavel);
    return {
      metaId: m.id,
      falta: centavos(total),
      recomendado: centavos(a[0] ?? 0),
      ajustado: ajustadas.has(m.id),
      dataProvavel: provavel ? `${provavel.ano}-${String(provavel.mes).padStart(2, '0')}-01` : null,
      noPrazo,
      paraManterPrazo,
      meses: caixa.meses.slice(0, MESES_NA_TABELA).map((x, k) => ({
        ano: x.ano, mes: x.mes, sobra: x.sobra, faturas: x.faturas, guardar: centavos(a[k])
      })),
      ateOAlvo: a.slice(0, indiceProvavel === null ? n : indiceProvavel + 1).map(centavos),
      degraus,
      dependeDaFatura: dependeDaFatura(degraus)
    };
  });

  return {
    sobraMes: caixa.meses[0]?.sobra ?? 0,
    limiteMes: centavos(capacidade[0] ?? 0),
    limitadoPor,
    totalRecomendado: centavos(planos.reduce((s, p) => s + p.recomendado, 0)),
    metas: planos
  };
}

/**
 * Passo 6: a partir do mês seguinte, junta meses com valores parecidos num degrau com o mesmo valor (a média; o
 * arredondamento fica no último mês do degrau, para o total não mudar). Altera `a` e devolve os degraus.
 */
function suavizar(a: number[], caixa: PlanoMetas, mesDoIndice: (k: number) => MesRef): Degrau[] {
  let ultimo = 0;
  for (let k = a.length - 1; k >= 1; k--) if (a[k] > 0.005) { ultimo = k; break; }
  const degraus: Degrau[] = [];
  for (let i = 1; i <= ultimo;) {
    let j = i;
    let soma = a[i];
    while (j + 1 <= ultimo) {
      const media = soma / (j - i + 1);
      if (Math.abs(a[j + 1] - media) > Math.max(TOLERANCIA_REAIS, TOLERANCIA_PERCENTUAL * media)) break;
      j++;
      soma += a[j];
    }
    const quantos = j - i + 1;
    const valor = centavos(soma / quantos);
    for (let k = i; k < j; k++) a[k] = valor;
    a[j] = centavos(soma - valor * (quantos - 1));
    let faturas = 0;
    for (let k = i; k <= j; k++) faturas += caixa.meses[k].faturas;
    degraus.push({ inicio: mesDoIndice(i), fim: mesDoIndice(j), valor, faturas: centavos(faturas / quantos) });
    i = j + 1;
  }
  return degraus;
}

/** O degrau mais alto vale pelo menos o dobro do primeiro porque as faturas caem pelo menos 20%: o plano depende disso. */
function dependeDaFatura(degraus: Degrau[]): PlanoDaMeta['dependeDaFatura'] {
  if (degraus.length < 2) return null;
  const primeiro = degraus[0];
  const maior = degraus.slice(1).reduce((x, d) => (d.valor > x.valor ? d : x));
  if (maior.valor < 2 * Math.max(primeiro.valor, 1) || maior.faturas > 0.8 * primeiro.faturas) return null;
  return { aPartir: maior.inicio, antes: primeiro.faturas, depois: maior.faturas };
}
