import { AvisoRescisao as A, TipoRescisao as T } from '../../core/modelos';

/** #215: as regras da tela Trabalho › Rescisão, sem Angular (ADR 0059). As da CLT e o cálculo são da API. */

export const TIPOS: { valor: T; rotulo: string }[] = [
  { valor: T.SemJustaCausa, rotulo: 'Dispensa sem justa causa' },
  { valor: T.PedidoDemissao, rotulo: 'Pedido de demissão' },
  { valor: T.Acordo, rotulo: 'Acordo (art. 484-A)' },
  { valor: T.JustaCausa, rotulo: 'Justa causa' },
  { valor: T.FimExperiencia, rotulo: 'Fim do contrato de experiência' }
];

/** Os avisos que fazem sentido para o tipo (justa causa e fim da experiência não têm). */
export function avisosDoTipo(tipo: T): { valor: A; rotulo: string }[] {
  if (tipo === T.JustaCausa || tipo === T.FimExperiencia) return [{ valor: A.Sem, rotulo: 'Sem aviso prévio' }];
  if (tipo === T.PedidoDemissao) {
    return [{ valor: A.Trabalhado, rotulo: 'Vou cumprir o aviso' }, { valor: A.Indenizado, rotulo: 'Não vou cumprir (a empresa desconta)' }];
  }
  return [{ valor: A.Indenizado, rotulo: 'Indenizado (a empresa paga)' }, { valor: A.Trabalhado, rotulo: 'Trabalhado' }];
}

/** O aviso padrão do tipo: o primeiro que faz sentido. */
export function avisoPadrao(tipo: T): A {
  return avisosDoTipo(tipo)[0].valor;
}

/** O que muda no FGTS, em palavras. */
export function textoDoFgts(multa: number, saque: number): string {
  if (!multa && !saque) return 'Sem multa e sem saque: o saldo fica na conta do FGTS.';
  if (!multa) return `Sem multa; saca ${saque}% do saldo.`;
  return `Multa de ${multa}% sobre o saldo; saca ${saque}% do saldo e a multa.`;
}
