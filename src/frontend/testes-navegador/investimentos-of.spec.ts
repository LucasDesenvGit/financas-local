import { expect, test } from './base';

/**
 * #173 e) (ADR 0045): a aba Open Finance de Investimentos mostra o rendimento por mês (API falsa, valores fictícios), com o
 * detalhe por investimento sob demanda, sem erro nem rolagem para o lado.
 */
test('Investimentos › Open Finance: rendimento por mês com detalhe por investimento', async ({ page, problemas }) => {
  await page.goto('/investimentos?aba=of');
  const secao = page.locator('section.rendimento-of');
  await expect(secao.getByRole('heading', { name: 'Rendimento por mês' })).toBeVisible();
  await expect(secao.locator('.resumo-rendimento-of')).toContainText('os investimentos renderam R$ 41');
  const porInvestimento = secao.getByRole('button', { name: /^Rendimento por investimento em/ }).first();
  await porInvestimento.click();
  await expect(porInvestimento).toHaveAttribute('aria-expanded', 'true');
  await expect(secao.locator('.linha-por-investimento')).toContainText('Fundo Y: sem o valor aplicado');
  const sobra = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(sobra).toBeLessThanOrEqual(1);
  expect(problemas.lista).toEqual([]);
});
