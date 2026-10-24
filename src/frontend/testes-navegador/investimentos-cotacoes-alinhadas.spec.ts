import type { Locator } from '@playwright/test';
import { expect, test } from './base';

/**
 * #377: Câmbio e Criptomoedas com as colunas retas: em cada bloco, o nome começa e o valor termina no mesmo ponto em
 * todas as linhas; entre os dois blocos, também (no computador). Folga de 0,5 px (fontes do Linux da CI, #271).
 */
async function bordas(linhas: Locator, seletor: string): Promise<{ esquerda: number; direita: number }[]> {
  const caixas = [];
  for (const linha of await linhas.all()) {
    const caixa = (await linha.locator(seletor).first().boundingBox())!;
    caixas.push({ esquerda: caixa.x, direita: caixa.x + caixa.width });
  }
  return caixas;
}
const iguais = (valores: number[]) => Math.max(...valores) - Math.min(...valores);

test('Investimentos: Câmbio e Criptomoedas com o nome e o valor alinhados em todas as linhas', async ({ page, problemas }) => {
  await page.goto('/investimentos');
  const cambio = page.getByRole('region', { name: 'Câmbio' });
  await cambio.getByRole('button', { name: /Mais \d+ moedas/ }).click();
  // Só as linhas com cotação (a de "sem cotação guardada ainda" não tem valor nem fonte).
  const linhasCambio = cambio.locator('li:visible').filter({ has: page.locator('.valor') });
  const linhasCripto = page.getByRole('region', { name: 'Criptomoedas' }).locator('li').filter({ has: page.locator('.valor') });
  await expect(linhasCambio.last()).toBeVisible();
  await expect(linhasCripto.first()).toBeVisible();

  const nomesCambio = await bordas(linhasCambio, '.cresce');
  const nomesCripto = await bordas(linhasCripto, '.cresce');
  const valoresCambio = await bordas(linhasCambio, '.valor');
  const valoresCripto = await bordas(linhasCripto, '.valor');
  expect(iguais(nomesCambio.map((c) => c.esquerda))).toBeLessThan(0.5);
  expect(iguais(nomesCripto.map((c) => c.esquerda))).toBeLessThan(0.5);
  expect(iguais(valoresCambio.map((c) => c.direita))).toBeLessThan(0.5);
  expect(iguais(valoresCripto.map((c) => c.direita))).toBeLessThan(0.5);
  if (page.viewportSize()!.width > 760) {
    // Os dois blocos ficam um embaixo do outro, com as mesmas colunas.
    expect(Math.abs(nomesCambio[0].esquerda - nomesCripto[0].esquerda)).toBeLessThan(0.5);
    expect(Math.abs(valoresCambio[0].direita - valoresCripto[0].direita)).toBeLessThan(0.5);
    const fonteCambio = await bordas(linhasCambio, '.cotacao-fonte');
    const fonteCripto = await bordas(linhasCripto, '.cotacao-fonte');
    expect(Math.abs(fonteCambio[0].direita - fonteCripto[0].direita)).toBeLessThan(0.5);
  }
  const sobra = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(sobra).toBeLessThanOrEqual(1);
  expect(problemas.lista).toEqual([]);
});
