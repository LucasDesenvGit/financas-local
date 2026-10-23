import { Meta, MetaAporte } from '../core/modelos';
import { NOMES_MESES, pct, reaisOuNada } from '../compartilhado/utilidades';
import { MesRef, PlanoDaMeta } from './metas-plano';

/**
 * #318: "Como está indo" a meta — o que já foi guardado, mês a mês, e o caminho do plano até o alvo.
 *
 * - **Guardado** (linha cheia): o total no fim de cada mês, do mês do primeiro aporte registrado até o atual. O total de
 *   hoje é o da meta (com a conta ou o cofrinho); os meses anteriores são esse total menos os aportes que vieram
 *   depois. Cada "+ Aporte" muda a linha. Antes da tabela MetaAporte não havia data: o gráfico parte do total de hoje.
 * - **Plano** (tracejada): começa no total de hoje e soma o que o plano do assistente manda guardar em cada mês seguinte
 *   (o mês atual entra no ponto do mês seguinte), até o alvo. Fica no mês da "data provável" do plano.
 * - **Alvo**: o valor alvo, de ponta a ponta.
 * Sem plano (meta concluída ou sem histórico de caixa), só o guardado e o alvo.
 *
 * #359: a escala acompanha os aportes. Todos no mesmo dia: um ponto antes deles e um por aporte, na hora e minuto em
 * que foi registrado. Todos no mesmo mês: um ponto por dia, da véspera do primeiro aporte até o último (ou até hoje,
 * se o mês é o atual). Em meses diferentes: mês a mês, como acima. Nas escalas de dia e de hora não há linha do plano
 * (ele é mensal; fica na tabela "Plano mês a mês").
 */
export type EscalaEvolucao = 'mes' | 'dia' | 'hora';

export interface EvolucaoDaMeta {
  escala: EscalaEvolucao;
  rotulos: string[];
  guardado: (number | null)[];
  plano: (number | null)[];
  alvo: number;
  /** Aportes registrados, do mais novo ao mais antigo (a lista ao lado do gráfico). */
  aportes: MetaAporte[];
}

const indice = (m: MesRef) => m.ano * 12 + (m.mes - 1);
const doIndice = (k: number): MesRef => ({ ano: Math.floor(k / 12), mes: (k % 12) + 1 });
const mesDaData = (data: string): MesRef => {
  const [ano, mes] = data.split('-').map(Number);
  return { ano, mes };
};
const centavos = (v: number) => Math.round(v * 100) / 100;

export function rotuloMesCurto(m: MesRef): string {
  return `${NOMES_MESES[m.mes - 1].slice(0, 3).toLowerCase()}/${String(m.ano % 100).padStart(2, '0')}`;
}

/** #359: a escala do gráfico pelos aportes (ver o comentário do topo). */
export function escalaDosAportes(aportes: readonly MetaAporte[]): EscalaEvolucao {
  if (aportes.length === 0) return 'mes';
  if (aportes.every((a) => a.data === aportes[0].data)) return 'hora';
  if (aportes.every((a) => a.data.slice(0, 7) === aportes[0].data.slice(0, 7))) return 'dia';
  return 'mes';
}

/** "AAAA-MM-DD" de um Date, sem passar por UTC. */
const textoDia = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
const diaDoTexto = (s: string) => {
  const [ano, mes, dia] = s.split('-').map(Number);
  return new Date(ano, mes - 1, dia);
};

/**
 * @param hojeDia o dia de hoje (AAAA-MM-DD): na escala de dia, a linha vai até ele quando o mês dos aportes é o atual.
 */
export function evolucaoDaMeta(meta: Meta, aportes: readonly MetaAporte[], plano: PlanoDaMeta | null, hoje: MesRef,
  hojeDia?: string): EvolucaoDaMeta {
  const escala = escalaDosAportes(aportes);
  const antes = centavos(Math.max(0, meta.valorAtual - aportes.reduce((s, a) => s + a.valor, 0)));
  const sohGuardado = (rotulos: string[], guardado: number[]): EvolucaoDaMeta => ({
    escala, rotulos, guardado, plano: rotulos.map(() => null), alvo: meta.valorAlvo, aportes: [...aportes].reverse()
  });
  if (escala === 'hora') {
    let acumulado = antes;
    const guardado = [antes, ...aportes.map((a) => (acumulado = centavos(Math.max(0, acumulado + a.valor))))];
    return sohGuardado(['Antes', ...aportes.map((a, i) => a.hora ?? `${i + 1}º aporte`)], guardado);
  }
  if (escala === 'dia') {
    const ultimoAporte = aportes.reduce((m, a) => (a.data > m ? a.data : m), aportes[0].data);
    const fim = hojeDia && hojeDia.slice(0, 7) === ultimoAporte.slice(0, 7) && hojeDia > ultimoAporte ? hojeDia : ultimoAporte;
    const dia = diaDoTexto(aportes.reduce((m, a) => (a.data < m ? a.data : m), aportes[0].data));
    dia.setDate(dia.getDate() - 1);
    const rotulos: string[] = [];
    const guardado: number[] = [];
    for (; textoDia(dia) <= fim; dia.setDate(dia.getDate() + 1)) {
      const d = textoDia(dia);
      rotulos.push(`${d.slice(8, 10)}/${d.slice(5, 7)}`);
      guardado.push(centavos(Math.max(0, meta.valorAtual - aportes.filter((a) => a.data > d).reduce((s, a) => s + a.valor, 0))));
    }
    return sohGuardado(rotulos, guardado);
  }
  const atual = indice(hoje);
  const primeiro = aportes.length ? Math.min(...aportes.map((a) => indice(mesDaData(a.data)))) : atual;
  const inicio = Math.min(primeiro, atual);
  const guardar = plano?.ateOAlvo ?? [];
  // Até o alvo: o mês atual mais os meses do plano (o último é o da data provável).
  const fim = atual + Math.max(guardar.length - 1, 0);

  const rotulos: string[] = [];
  const guardado: (number | null)[] = [];
  const linhaPlano: (number | null)[] = [];
  let acumulado = meta.valorAtual;
  for (let k = inicio; k <= fim; k++) {
    rotulos.push(rotuloMesCurto(doIndice(k)));
    if (k <= atual) {
      const depois = aportes.filter((a) => indice(mesDaData(a.data)) > k).reduce((s, a) => s + a.valor, 0);
      guardado.push(centavos(Math.max(0, meta.valorAtual - depois)));
    } else {
      guardado.push(null);
    }
    if (!plano || guardar.length === 0 || k < atual) {
      linhaPlano.push(null);
    } else if (k === atual) {
      // Só um mês no plano: o alvo chega este mês, e o ponto já mostra o fim dele.
      linhaPlano.push(centavos(Math.min(meta.valorAlvo, guardar.length === 1 ? acumulado + guardar[0] : acumulado)));
    } else {
      acumulado += k === atual + 1 ? guardar[0] + guardar[1] : guardar[k - atual];
      linhaPlano.push(centavos(Math.min(meta.valorAlvo, acumulado)));
    }
  }
  return { escala, rotulos, guardado, plano: linhaPlano, alvo: meta.valorAlvo, aportes: [...aportes].reverse() };
}

/** #359: a linha "como ler" do gráfico, pela escala. */
export function comoLerEvolucao(escala: EscalaEvolucao): string {
  if (escala === 'hora') return 'Linha cheia: o total guardado antes dos aportes e depois de cada um, na hora em que foi registrado. Pontilhada: o valor alvo.';
  if (escala === 'dia') return 'Linha cheia: o total guardado no fim de cada dia, desde a véspera do primeiro aporte. Pontilhada: o valor alvo. O plano fica no "Plano mês a mês".';
  return 'Linha cheia: o total guardado no fim de cada mês; cada aporte muda o mês dele. Tracejada: o plano até o alvo. Pontilhada: o valor alvo.';
}

/** Frase-resumo do gráfico (WCAG 1.1.1); com "Ocultar valores", só percentuais. */
export function resumoEvolucao(meta: Meta, ev: EvolucaoDaMeta, plano: PlanoDaMeta | null, oculto: boolean): string {
  const valor = reaisOuNada(meta.valorAtual, oculto);
  const alvo = reaisOuNada(meta.valorAlvo, oculto);
  const partes = [valor && alvo ? `Guardado: ${valor} de ${alvo} (${pct(meta.percentual)}).` : `Guardado: ${pct(meta.percentual)} do alvo.`];
  if (ev.aportes.length === 0) {
    partes.push('Ainda sem aporte registrado: cada “+ Aporte” vira um ponto da linha.');
  } else {
    const n = ev.aportes.length;
    const primeiro = ev.aportes[n - 1].data;
    const quando = ev.escala === 'hora' ? `em ${primeiro.slice(8, 10)}/${primeiro.slice(5, 7)}/${primeiro.slice(0, 4)}`
      : ev.escala === 'dia' ? `desde ${primeiro.slice(8, 10)}/${primeiro.slice(5, 7)}` : `desde ${rotuloMesCurto(mesDaData(primeiro))}`;
    partes.push(`${n} ${n === 1 ? 'aporte registrado' : 'aportes registrados'} ${quando}.`);
  }
  if (meta.concluida || meta.valorAtual >= meta.valorAlvo) partes.push('Meta alcançada.');
  else if (plano?.dataProvavel) partes.push(`No plano, chega ao alvo em ${rotuloMesCurto(mesDaData(plano.dataProvavel))}.`);
  else if (plano) partes.push('No plano, o alvo passa de 10 anos.');
  return partes.join(' ');
}
