import type { ChartConfiguration } from 'chart.js';
import { CORES_SUGERIDAS, NOMES_MESES, dataIso } from '../../compartilhado/utilidades';
import type { Meta, PlanoMetas, Reserva, SalvarMeta } from '../../core/modelos';
import type { EvolucaoDaMeta } from './metas-evolucao';
import { MesRef, PlanoCalculado, dataPeloValorMensal } from './metas-plano';

/**
 * A lógica da tela Metas (#732, ADR 0059): funções puras, sem Angular, que o componente da tela só chama. O cálculo do
 * plano de cada meta mora em `metas-plano.ts` e o do gráfico "Como está indo", em `metas-evolucao.ts`; aqui ficam o
 * que a tela decide em volta deles: a simulação na URL, as sanfonas, o formulário (limite, data alvo, "concluída"), a
 * ordem de prioridade e o gráfico.
 */

/* ------------------------------------------------------------ o que a URL e o navegador lembram */

/** Lê "?guardar=1:50,3:120" (meta:valor); o que não for número válido e ≥ 0 é ignorado. */
export function lerAjustes(texto: string | null): ReadonlyMap<number, number> {
  const ajustes = new Map<number, number>();
  for (const par of (texto ?? '').split(',')) {
    const [id, valor] = par.split(':').map(Number);
    if (Number.isInteger(id) && id > 0 && Number.isFinite(valor) && valor >= 0) ajustes.set(id, valor);
  }
  return ajustes;
}

/** O inverso de `lerAjustes`: "1:50,3:120"; sem nenhum ajuste, null (o parâmetro sai da URL). */
export function ajustesParaUrl(ajustes: ReadonlyMap<number, number>): string | null {
  return [...ajustes].map(([id, v]) => `${id}:${v}`).join(',') || null;
}

/** #149 (D): o valor que a pessoa consegue guardar este mês na meta; vazio, negativo ou inválido volta à recomendação. */
export function comAjuste(ajustes: ReadonlyMap<number, number>, id: number, valor: number | null | undefined): ReadonlyMap<number, number> {
  const novo = new Map(ajustes);
  if (valor === null || valor === undefined || !Number.isFinite(valor) || valor < 0) novo.delete(id);
  else novo.set(id, valor);
  return novo;
}

export function semAjuste(ajustes: ReadonlyMap<number, number>, id: number): ReadonlyMap<number, number> {
  const novo = new Map(ajustes);
  novo.delete(id);
  return novo;
}

/** #335: os ids das metas fechadas na sanfona, como o navegador guardou (ADR 0017); texto ruim vira "nenhuma fechada". */
export function lerFechadas(texto: string | null): ReadonlySet<number> {
  try {
    const salvo: unknown = JSON.parse(texto ?? '[]');
    return new Set(Array.isArray(salvo) ? salvo.filter((id): id is number => Number.isInteger(id)) : []);
  } catch {
    return new Set();
  }
}

/** Põe o id no conjunto, ou o tira se já está. */
export function alternarNoConjunto(conjunto: ReadonlySet<number>, id: number): ReadonlySet<number> {
  const novo = new Set(conjunto);
  if (!novo.delete(id)) novo.add(id);
  return novo;
}

/** "Fechar todas" com alguma aberta; "Abrir todas" com todas fechadas (Nielsen 3). Devolve as metas que ficam fechadas. */
export function fechadasAoAlternarTodas(ids: readonly number[], algumaAberta: boolean): ReadonlySet<number> {
  return algumaAberta ? new Set(ids) : new Set();
}

/* ------------------------------------------------------------ prioridade e prazo */

/** O mês de uma data, no formato do plano. */
export function mesDe(d: Date): MesRef {
  return { ano: d.getFullYear(), mes: d.getMonth() + 1 };
}

/** Posição na ordem de prioridade (1 = primeira) entre as metas em aberto; 0 se não está entre elas. */
export function posicaoNaOrdem(ativas: readonly Meta[], id: number): number {
  return ativas.findIndex((x) => x.id === id) + 1;
}

/** A nova ordem dos ids depois de subir (-1) ou descer (+1) a meta uma posição; null se ela não pode se mexer. */
export function ordemAposMover(ordem: readonly number[], id: number, direcao: -1 | 1): number[] | null {
  const nova = [...ordem];
  const i = nova.indexOf(id);
  const j = i + direcao;
  if (i < 0 || j < 0 || j >= nova.length) return null;
  [nova[i], nova[j]] = [nova[j], nova[i]];
  return nova;
}

/** Prazo antes do mês do plano: não há mais "manter o prazo", só a data provável. */
export function prazoVencido(dataAlvo: string | null, mesDoPlano: MesRef | null): boolean {
  if (!mesDoPlano || !dataAlvo) return false;
  const [ano, mes] = dataAlvo.split('-').map(Number);
  return ano * 12 + mes < mesDoPlano.ano * 12 + mesDoPlano.mes;
}

/** Com a sobra para as metas abaixo do valor por mês escolhido: a sobra do mês; senão, null (#359). */
export function porMesAcimaDaSobra(porMes: number | null, limiteMes: number | undefined): number | null {
  return porMes !== null && limiteMes !== undefined && porMes > limiteMes + 0.005 ? limiteMes : null;
}

/* ------------------------------------------------------------ nomes de mês e "mostrar mais" */

export function nomeMes(mes: number): string {
  const n = NOMES_MESES[mes - 1];
  return n.charAt(0).toUpperCase() + n.slice(1);
}

export function nomeMesCurto(mes: number): string {
  return NOMES_MESES[mes - 1].slice(0, 3);
}

/** #335: quantos meses do plano a tabela mostra depois de "Mostrar mais" (soma o passo, até o total do plano). */
export function mesesAposMostrarMais(atual: number, total: number, passo: number): number {
  return Math.min(total, atual + passo);
}

/* ------------------------------------------------------------ o formulário da meta */

/**
 * O que a janela da meta edita: o que vai para a API, mais o "por mês" (que vira `guardarPorMes` ao salvar) e quanto do
 * "já guardado" vem de fora do formulário (o saldo da conta ou do cofrinho, que o formulário não muda).
 */
export interface FormMeta {
  /** Null na meta nova. */
  id: number | null;
  dados: SalvarMeta;
  /** #359: quanto a pessoa diz que vai guardar por mês. */
  porMes: number | null;
  deFora: number;
}

export function formularioVazio(): FormMeta {
  return {
    id: null, porMes: null, deFora: 0,
    dados: { nome: '', valorAlvo: 0, valorAtual: 0, dataAlvo: null, contaId: null, cor: CORES_SUGERIDAS[0], concluida: false, reservaNome: null, icone: null }
  };
}

export function formularioDaMeta(m: Meta): FormMeta {
  return {
    id: m.id, porMes: m.guardarPorMes ?? null, deFora: m.valorAtual - m.valorInicial,
    dados: {
      nome: m.nome, valorAlvo: m.valorAlvo, valorAtual: m.valorInicial, dataAlvo: m.dataAlvo,
      contaId: m.contaId, cor: m.cor, concluida: m.concluida, reservaNome: m.reservaNome, icone: m.icone ?? null
    }
  };
}

/** O que vai para a API ao salvar: números de verdade, data vazia vira null e o "por mês" só vale acima de zero. */
export function dtoDaMeta(fm: FormMeta): SalvarMeta {
  return {
    ...fm.dados,
    valorAlvo: Number(fm.dados.valorAlvo),
    valorAtual: Number(fm.dados.valorAtual) || 0,
    dataAlvo: fm.dados.dataAlvo || null,
    guardarPorMes: Number(fm.porMes) > 0 ? Number(fm.porMes) : null
  };
}

/**
 * Cofrinhos da conta escolhida, com saldo (Nielsen 6: escolher, não digitar). O campo só aparece quando a conta tem
 * cofrinho (divulgação progressiva). Se a meta aponta para um que sumiu do extrato, ele continua na lista (marcado),
 * para não ser trocado sem querer.
 */
export function opcoesCofrinho(reservas: readonly Reserva[], contaId: number | null, escolhido: string | null): { nome: string; saldo: number | null }[] {
  if (contaId === null) return [];
  const opcoes = reservas.filter((r) => r.contaId === contaId)
    .map((r) => ({ nome: r.nome, saldo: r.saldo as number | null }))
    .sort((a, b) => a.nome.localeCompare(b.nome, 'pt-BR'));
  if (escolhido && !opcoes.some((o) => o.nome.toLowerCase() === escolhido.toLowerCase())) opcoes.push({ nome: escolhido, saldo: null });
  return opcoes;
}

/**
 * #316: quanto falta para o alvo no formulário: o alvo do formulário menos o já guardado (o valor inicial do formulário
 * mais o que vem da conta ou do cofrinho, que o formulário não muda).
 */
export function faltaNoForm(valorAlvo: number, valorAtual: number, deFora: number): number {
  return Math.max(0, Number(valorAlvo) - (Number(valorAtual) || 0) - deFora);
}

/**
 * #359: o maior valor por mês que dá para escolher: a maior parte da sobra para as metas nos próximos 12 meses (o mês
 * atual já com os limites), menos o que as outras metas já guardam por mês. Null sem o plano (menos de 3 meses de extrato).
 * `idEditado` é a meta que está no formulário (ela não conta entre "as outras"); null na meta nova.
 */
export function limiteDoPorMes(plano: PlanoMetas | null, calculado: PlanoCalculado | null, percentual: number,
  ativas: readonly Meta[], idEditado: number | null): number | null {
  if (!plano?.disponivel || !calculado) return null;
  const parte = percentual / 100;
  const maior = Math.max(calculado.limiteMes, ...plano.meses.slice(1, 12).map((m) => Math.max(0, m.sobra * parte)));
  const outras = ativas.filter((m) => m.id !== idEditado).reduce((s, m) => s + (m.guardarPorMes ?? 0), 0);
  return Math.max(0, Math.round((maior - outras) * 100) / 100);
}

export function mensagemLimitePorMes(limite: number | null): string {
  if (limite === null) return '';
  return limite > 0
    ? `Passa do que sobra para as metas: informe até R$ ${limite.toLocaleString('pt-BR', { minimumFractionDigits: 2, maximumFractionDigits: 2 })} por mês (já descontado o que as outras metas guardam).`
    : 'Não sobra nada para as metas nos próximos meses: deixe em branco e o plano decide.';
}

export function porMesAcimaDoLimite(limite: number | null, porMes: number | null): boolean {
  return limite !== null && Number(porMes) > limite;
}

/** #359: com o valor por mês do formulário, em quantos meses e em que mês a meta chega ao alvo. */
export function previsaoPorMes(fm: FormMeta, hoje: MesRef): { meses: number; data: string } | null {
  return dataPeloValorMensal(faltaNoForm(fm.dados.valorAlvo, fm.dados.valorAtual, fm.deFora), Number(fm.porMes) || 0, hoje);
}

/**
 * #359: mudou o valor por mês, o alvo ou o já guardado: a data alvo acompanha (só com o valor por mês preenchido e dentro
 * do limite). Devolve a data nova, ou null quando a data não deve mudar.
 */
export function dataAlvoPeloPorMes(fm: FormMeta, limite: number | null, hoje: MesRef): string | null {
  if (!fm.porMes || porMesAcimaDoLimite(limite, fm.porMes)) return null;
  return previsaoPorMes(fm, hoje)?.data ?? null;
}

/**
 * #316: aumentar o alvo de uma meta concluída para acima do que já foi guardado reabre a meta. Antes, "Concluída"
 * ficava marcada para sempre (a API grava ao chegar no alvo), e a meta continuava concluída com 1%. Devolve o valor de
 * "Concluída" depois da mudança e se a meta voltou para o plano (para a tela avisar).
 */
export function aoMudarOAlvo(fm: FormMeta, valorNovo: number): { concluida: boolean; reaberta: boolean } {
  const reaberta = fm.dados.concluida && Number(valorNovo) > Number(fm.dados.valorAlvo)
    && faltaNoForm(valorNovo, fm.dados.valorAtual, fm.deFora) > 0;
  return { concluida: reaberta ? false : fm.dados.concluida, reaberta };
}

/* ------------------------------------------------------------ "Como está indo" */

/** O mês e o dia de hoje para o gráfico: o mês é o do plano (o caixa que a API calculou), se houver; senão, o do relógio. */
export function hojeDaEvolucao(mesDoPlano: MesRef | null, agora: Date): { hoje: MesRef; hojeDia: string } {
  return {
    hoje: mesDoPlano ? { ano: mesDoPlano.ano, mes: mesDoPlano.mes } : mesDe(agora),
    hojeDia: dataIso(agora)
  };
}

/** #318: o gráfico de linhas de "Como está indo" (guardado, plano e alvo), na cor da meta. */
export function graficoDaEvolucao(cor: string, ev: EvolucaoDaMeta): ChartConfiguration<'line'> {
  // O <app-grafico> põe a linha cheia no padrão (preenchida até a base, com pontos) e as tracejadas sem nada (#292).
  return {
    type: 'line',
    data: {
      labels: ev.rotulos,
      datasets: [
        { label: 'Guardado', data: ev.guardado, borderColor: cor, tension: 0.25 },
        // Sem plano (meta concluída, ou sem histórico de caixa), a série sai: a legenda não mostra linha que não existe.
        ...(ev.plano.some((v) => v !== null)
          ? [{ label: 'Plano', data: ev.plano, borderColor: cor, borderDash: [6, 4], tension: 0.25 }] : []),
        { label: 'Alvo', data: ev.rotulos.map(() => ev.alvo), borderColor: '#a9b4c6', borderDash: [2, 3], borderWidth: 1.5 }
      ]
    },
    options: {
      plugins: { legend: { position: 'bottom' } },
      scales: {
        x: { ticks: { maxTicksLimit: 6, maxRotation: 0 } },
        y: { beginAtZero: true, ticks: { callback: (v) => Number(v).toLocaleString('pt-BR', { notation: 'compact' }) } }
      }
    }
  };
}
