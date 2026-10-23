import { expect, test } from './base';

/**
 * #173 e) (ADR 0045) e #331: a aba Open Finance de Investimentos mostra o rendimento por mês (API falsa, valores
 * fictícios), com o mês mais recente já aberto por investimento, e a grade dos investimentos recolhe, lembrada ao
 * recarregar. Sem erro nem rolagem para o lado.
 */
test('Investimentos › Open Finance: rendimento por mês com o detalhe por investimento já aberto', async ({ page, problemas }) => {
  await page.goto('/investimentos?aba=of');
  const secao = page.locator('section.rendimento-of');
  await expect(secao.getByRole('heading', { name: 'Rendimento por mês' })).toBeVisible();
  await expect(secao.locator('.resumo-rendimento-of')).toContainText('os investimentos renderam R$ 41');
  const porInvestimento = secao.getByRole('button', { name: /^Rendimento por investimento em/ }).first();
  await expect(porInvestimento).toHaveAttribute('aria-expanded', 'true');
  await expect(secao.locator('.linha-por-investimento')).toContainText('Fundo Y: sem o valor aplicado');
  await expect(secao.locator('.linha-por-investimento .trilho-investimento').first()).toBeVisible();
  const sobra = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(sobra).toBeLessThanOrEqual(1);
  expect(problemas.lista).toEqual([]);
});

test('Investimentos › Open Finance: a grade recolhe e continua recolhida ao recarregar', async ({ page, problemas }) => {
  await page.goto('/investimentos?aba=of');
  const botao = page.getByRole('button', { name: 'Investimentos do Open Finance', exact: true });
  await expect(botao).toHaveAttribute('aria-expanded', 'true');
  expect((await botao.boundingBox())!.height).toBeGreaterThanOrEqual(23.5);
  await expect(page.locator('#grade-of table')).toBeVisible();
  await botao.click();
  await expect(botao).toHaveAttribute('aria-expanded', 'false');
  await expect(page.locator('#grade-of table')).toHaveCount(0);
  // O rendimento por mês continua à vista.
  await expect(page.getByRole('heading', { name: 'Rendimento por mês' })).toBeVisible();
  await page.reload();
  await expect(page.getByRole('button', { name: 'Investimentos do Open Finance', exact: true })).toHaveAttribute('aria-expanded', 'false');
  await page.getByRole('button', { name: 'Investimentos do Open Finance', exact: true }).click();
  await expect(page.locator('#grade-of table')).toBeVisible();
  expect(problemas.lista).toEqual([]);
});
