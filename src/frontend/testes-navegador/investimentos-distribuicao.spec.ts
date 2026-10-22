import { expect, test } from './base';

/**
 * #330: sem posição aberta (sem "Rentabilidade por ativo"), a distribuição em "Mais análises" ocupa a largura toda, com a
 * rosca e a lista lado a lado no computador, em vez de meia largura com o canto vazio. No celular, um embaixo do outro.
 */
test('Investimentos › Mais análises: a distribuição sozinha usa a largura toda', async ({ page, problemas }) => {
  await page.goto('/investimentos');
  const mais = page.getByRole('button', { name: /Mais análises/ });
  if ((await mais.getAttribute('aria-expanded')) !== 'true') await mais.click();
  const bloco = page.locator('.painel-bloco', { has: page.getByRole('heading', { name: 'Distribuição' }) });
  await expect(bloco).toBeVisible();
  await expect(bloco).toContainText('Reservas');
  const grade = (await page.locator('.grade-2.grade-sozinha').boundingBox())!;
  const caixa = (await bloco.boundingBox())!;
  // O card ocupa a grade toda (não a metade).
  expect(caixa.width).toBeGreaterThan(grade.width - 2);
  const [rosca, lista] = [(await bloco.locator('canvas').boundingBox())!, (await bloco.locator('.lista-legenda').boundingBox())!];
  if (page.viewportSize()!.width > 760) {
    // Lado a lado: a lista começa à direita da rosca, na mesma faixa de altura.
    expect(lista.x).toBeGreaterThan(rosca.x + rosca.width);
    // A lista não se estica até a borda (o valor longe do nome) e o par fica no meio do card.
    expect(lista.width).toBeLessThanOrEqual(381);
    const esquerda = rosca.x - caixa.x;
    const direita = caixa.x + caixa.width - (lista.x + lista.width);
    expect(Math.abs(esquerda - direita)).toBeLessThan(60);
  } else {
    expect(lista.y).toBeGreaterThan(rosca.y + rosca.height - 1);
  }
  const sobra = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(sobra).toBeLessThanOrEqual(1);
  expect(problemas.lista).toEqual([]);
});
