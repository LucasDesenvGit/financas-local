/** #214: as regras da tela Trabalho › 13º salário, sem Angular (ADR 0059). O cálculo é da API. */

/** O ano da URL (?ano=2026), ou o padrão quando falta ou não é um ano. */
export function anoDaUrl(valor: string | null, padrao: number): number {
  const ano = Number(valor);
  return valor && Number.isInteger(ano) && ano >= 1900 && ano <= 2200 ? ano : padrao;
}
