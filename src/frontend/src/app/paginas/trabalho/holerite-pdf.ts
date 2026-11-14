import {
  ComoRubrica, ConfirmarHoleritePdf, PreviaHoleritePdf, RubricaLida, TipoLinhaHolerite as T
} from '../../core/modelos';

/** #228 (ADR 0068): as regras do holerite em PDF na tela Trabalho › Holerites, sem Angular (ADR 0059). A leitura é da API. */

/** O PDF vai em base64 num pedido de até 1 MB: 700 KB de arquivo (ADR 0068). */
export const TAMANHO_MAXIMO = 700 * 1024;

export const TIPOS_LINHA_PROVENTO: { valor: T; rotulo: string }[] = [
  { valor: T.Salario, rotulo: 'Salário' },
  { valor: T.HoraExtra, rotulo: 'Horas extras' },
  { valor: T.Dsr, rotulo: 'DSR (descanso remunerado)' },
  { valor: T.AdicionalNoturno, rotulo: 'Adicional noturno' },
  { valor: T.OutroProvento, rotulo: 'Outro provento' }
];

export const TIPOS_LINHA_DESCONTO: { valor: T; rotulo: string }[] = [
  { valor: T.Inss, rotulo: 'INSS' },
  { valor: T.Irrf, rotulo: 'IRRF' },
  { valor: T.ValeTransporte, rotulo: 'Vale-transporte' },
  { valor: T.ValeAlimentacao, rotulo: 'VR/VA' },
  { valor: T.PlanoDeSaude, rotulo: 'Plano de saúde' },
  { valor: T.PensaoAlimenticia, rotulo: 'Pensão alimentícia' },
  { valor: T.Adiantamento, rotulo: 'Adiantamento' },
  { valor: T.OutroDesconto, rotulo: 'Outro desconto' }
];

/** Até 9, provento; de 10 em diante, desconto (como na API). */
export function tipoEhProvento(t: T): boolean {
  return t < 10;
}

/** Uma linha da prévia como a pessoa deixou: o tipo e o valor que dá para corrigir, e se ela fica de fora. */
export interface LinhaPrevia {
  rubrica: RubricaLida;
  tipo: T;
  valor: number | null;
  ignorar: boolean;
}

export function linhasDaPrevia(p: PreviaHoleritePdf): LinhaPrevia[] {
  return p.rubricas.map((r) => ({ rubrica: r, tipo: r.tipo, valor: r.valor, ignorar: false }));
}

/** A situação da linha em texto, não só cor (Nielsen 1). A pessoa mudou o tipo: "você escolheu", e o site lembra. */
export function situacaoDaLinha(l: LinhaPrevia): string {
  if (l.ignorar) return 'fica de fora';
  if (l.tipo !== l.rubrica.tipo) return 'você escolheu: o site lembra no próximo mês';
  const textos: Record<ComoRubrica, string> = {
    dicionario: 'reconhecido',
    escolha: 'reconhecido pelo que você escolheu antes',
    desconhecida: 'escolha o tipo'
  };
  return textos[l.rubrica.como];
}

export function precisaEscolher(l: LinhaPrevia): boolean {
  return !l.ignorar && l.rubrica.como === 'desconhecida' && l.tipo === l.rubrica.tipo;
}

const centavos = (v: number): number => Math.round(v * 100);

/** A prova de novo, com o que a pessoa corrigiu: as somas contra os totais impressos no PDF. */
export interface ProvaAtual { descricao: string; impresso: number; encontrado: number; diferenca: number; bateu: boolean; }

export function provasAtuais(p: PreviaHoleritePdf, linhas: LinhaPrevia[]): ProvaAtual[] {
  let proventos = 0;
  let descontos = 0;
  for (const l of linhas) {
    if (l.ignorar || l.valor === null) continue;
    if (tipoEhProvento(l.tipo)) proventos += centavos(l.valor);
    else descontos += centavos(l.valor);
  }
  const provas: ProvaAtual[] = [];
  const prova = (descricao: string, impresso: number | null, encontrado: number): void => {
    if (impresso === null) return;
    const diferenca = encontrado - centavos(impresso);
    provas.push({ descricao, impresso, encontrado: encontrado / 100, diferenca: diferenca / 100, bateu: diferenca === 0 });
  };
  prova('Total de vencimentos', p.totalVencimentos, proventos);
  prova('Total de descontos', p.totalDescontos, descontos);
  prova('Líquido', p.liquido, proventos - descontos);
  return provas;
}

/** O fim da frase da prova: "bate com o PDF" ou, depois do valor da diferença, "a mais que o impresso" (WCAG 3.3.1:
 * qual total e por quanto). */
export function textoDaProva(p: ProvaAtual): string {
  if (p.bateu) return 'bate com o PDF';
  return p.diferenca > 0 ? 'a mais que o impresso' : 'a menos que o impresso';
}

/** O que falta para confirmar (vazio: pode). */
export function faltaParaConfirmar(linhas: LinhaPrevia[], mes: string | null): string | null {
  if (!mes) return 'Escolha o mês do holerite.';
  const valendo = linhas.filter((l) => !l.ignorar);
  if (!valendo.length) return 'Deixe pelo menos uma linha no holerite.';
  if (valendo.some((l) => l.valor === null || l.valor <= 0)) return 'Cada linha precisa de um valor.';
  return null;
}

/** O corpo do "Confirmar": as linhas que ficam, com o tipo e o valor como a pessoa deixou, e o rodapé do PDF. */
export function paraConfirmar(p: PreviaHoleritePdf, linhas: LinhaPrevia[], mes: string): ConfirmarHoleritePdf {
  return {
    mes,
    rubricas: linhas.filter((l) => !l.ignorar).map((l) => ({
      codigo: l.rubrica.codigo, descricao: l.rubrica.descricao, referencia: l.rubrica.referencia, valor: l.valor ?? 0,
      provento: l.rubrica.provento, tipo: Number(l.tipo)
    })),
    salarioBase: p.salarioBase, baseInss: p.baseInss, baseFgts: p.baseFgts, fgtsDoMes: p.fgtsDoMes
  };
}

/** O conteúdo do arquivo em base64, para o JSON do pedido. */
export function paraBase64(bytes: Uint8Array): string {
  let texto = '';
  const pedaco = 0x8000;
  for (let i = 0; i < bytes.length; i += pedaco) {
    texto += String.fromCharCode(...bytes.subarray(i, i + pedaco));
  }
  return btoa(texto);
}

/** Recusa antes de mandar: o que não é PDF pelo tamanho ou pelo nome (a API confere o conteúdo). */
export function problemaDoArquivo(nome: string, tamanho: number): string | null {
  if (tamanho > TAMANHO_MAXIMO) return `${nome}: o PDF passa do limite de 700 KB. Mande só o holerite.`;
  if (tamanho === 0) return `${nome}: o arquivo está vazio.`;
  return null;
}

/** A recusa da API pede a senha? (400 com o motivo "senha".) */
export function motivoDaRecusa(erro: unknown): 'senha' | 'semTexto' | null {
  const corpo = (erro as { error?: { motivo?: string } } | null)?.error;
  return corpo?.motivo === 'senha' || corpo?.motivo === 'semTexto' ? corpo.motivo : null;
}

/** "AAAA-MM-01" do mês escolhido. */
export function mesDoPdf(p: PreviaHoleritePdf): string | null {
  return p.mes ? p.mes.slice(0, 10) : null;
}
