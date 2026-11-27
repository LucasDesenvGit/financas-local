/** #214: as regras da tela Trabalho › 13º salário, sem Angular (ADR 0059). O cálculo é da API. */
import { ParcelaDecimo } from '../../core/modelos';

/** O ano da URL (?ano=2026), ou o padrão quando falta ou não é um ano. */
export function anoDaUrl(valor: string | null, padrao: number): number {
  const ano = Number(valor);
  return valor && Number.isInteger(ano) && ano >= 1900 && ano <= 2200 ? ano : padrao;
}

/**
 * O que o formulário da 1ª parcela mostra: só o que a pessoa informou. A metade calculada, o 30/11 padrão e a data que
 * veio com as férias ficam de fora, senão "Gravar" os gravaria como informados (revisão do #700; a data, #750).
 */
export function camposDaPrimeira(p: ParcelaDecimo): { em: string | null; valor: number | null } {
  return { em: p.dataInformada ? p.data : null, valor: p.valorInformado ? p.valor : null };
}
