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
 */
export interface EvolucaoDaMeta {
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

export function evolucaoDaMeta(meta: Meta, aportes: readonly MetaAporte[], plano: PlanoDaMeta | null, hoje: MesRef): EvolucaoDaMeta {
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
  return { rotulos, guardado, plano: linhaPlano, alvo: meta.valorAlvo, aportes: [...aportes].reverse() };
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
    partes.push(`${n} ${n === 1 ? 'aporte registrado' : 'aportes registrados'} desde ${rotuloMesCurto(mesDaData(ev.aportes[n - 1].data))}.`);
  }
  if (meta.concluida || meta.valorAtual >= meta.valorAlvo) partes.push('Meta alcançada.');
  else if (plano?.dataProvavel) partes.push(`No plano, chega ao alvo em ${rotuloMesCurto(mesDaData(plano.dataProvavel))}.`);
  else if (plano) partes.push('No plano, o alvo passa de 10 anos.');
  return partes.join(' ');
}
