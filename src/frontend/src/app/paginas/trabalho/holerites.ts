import { LancamentoHolerite, SituacaoHolerite, TipoLancamentoHolerite as T } from '../../core/modelos';

/** #211 (ADR 0065): as regras da tela Trabalho › Holerites, sem Angular (ADR 0059). O cálculo é da API. */

export const TIPOS_PROVENTO: { valor: T; rotulo: string }[] = [
  { valor: T.HoraExtra, rotulo: 'Horas extras' },
  { valor: T.AdicionalNoturno, rotulo: 'Adicional noturno' },
  { valor: T.OutroProvento, rotulo: 'Outro provento (bônus, PLR, comissão…)' }
];

export const TIPOS_DESCONTO: { valor: T; rotulo: string }[] = [
  { valor: T.ValeTransporte, rotulo: 'Vale-transporte' },
  { valor: T.ValeAlimentacao, rotulo: 'VR/VA' },
  { valor: T.PlanoDeSaude, rotulo: 'Plano de saúde' },
  { valor: T.PensaoAlimenticia, rotulo: 'Pensão alimentícia' },
  { valor: T.OutroDesconto, rotulo: 'Outro desconto' }
];

export function rotuloTipo(t: T): string {
  return [...TIPOS_PROVENTO, ...TIPOS_DESCONTO].find((x) => x.valor === t)?.rotulo ?? '—';
}

/** Hora extra e adicional noturno vão por horas e percentual; o resto, por valor. */
export function ehPorHora(t: T): boolean {
  return t === T.HoraExtra || t === T.AdicionalNoturno;
}

export function ehProvento(t: T): boolean {
  return t < T.ValeTransporte;
}

/** Um lançamento novo do tipo, com os padrões dele (hora extra a 50%, adicional noturno a 20%, bônus com INSS e IR). */
export function lancamentoNovo(tipo: T): LancamentoHolerite {
  return {
    tipo, descricao: '', quantidade: null, valor: null,
    percentual: tipo === T.HoraExtra ? 50 : tipo === T.AdicionalNoturno ? 20 : null,
    incideInss: tipo === T.OutroProvento, incideIrrf: tipo === T.OutroProvento,
    deduzIrrf: tipo === T.PensaoAlimenticia, recorrente: tipo === T.PlanoDeSaude || tipo === T.PensaoAlimenticia
  };
}

/** Trocar o tipo mantém a descrição e a marca de recorrente e recomeça o resto com os padrões do tipo novo. */
export function trocarTipo(l: LancamentoHolerite, tipo: T): LancamentoHolerite {
  return { ...lancamentoNovo(tipo), descricao: l.descricao, recorrente: l.recorrente };
}

/** O corpo do PUT: sem o id, e só os campos que valem para o tipo (a API normaliza o resto). */
export function paraSalvar(lista: LancamentoHolerite[]): LancamentoHolerite[] {
  return lista.map((l) => ({
    tipo: Number(l.tipo), descricao: l.descricao.trim(),
    quantidade: ehPorHora(l.tipo) ? l.quantidade : null,
    percentual: ehPorHora(l.tipo) ? l.percentual : null,
    valor: ehPorHora(l.tipo) ? null : l.valor,
    incideInss: l.incideInss, incideIrrf: l.incideIrrf, deduzIrrf: l.deduzIrrf, recorrente: l.recorrente
  }));
}

export function textoSituacao(s: SituacaoHolerite): string {
  return s === SituacaoHolerite.Fechado ? 'Fechado' : 'Aberto';
}
