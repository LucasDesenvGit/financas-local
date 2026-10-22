import { expect, test } from './base';

/**
 * #323: Contas e cartões em outro mês, como o Painel: o seletor muda as faturas e a URL (?ano=&mes=); a nota diz que
 * saldo, dívida e limite são de hoje, e "Voltar para o mês atual" tira o mês da URL. Sem erro de console nem de CSP.
 */
test('Contas e cartões: as faturas de outro mês, e a volta ao mês atual', async ({ page, problemas }) => {
  await page.goto('/contas?ano=2025&mes=9');
  const cartao = page.locator('#cartao-3');
  await expect(cartao.locator('.valor-fatura-mes')).toContainText('Fatura de setembro/2025');
  await expect(cartao.locator('.valor-fatura-mes')).toContainText('R$ 1.234,56');
  await expect(page.locator('.nota-mes')).toContainText('Saldo, dívida total e limite continuam sendo os de hoje');
  const sobra = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(sobra).toBeLessThanOrEqual(1);

  await page.getByRole('button', { name: 'Voltar para o mês atual' }).click();
  await expect(page.locator('.nota-mes')).toHaveCount(0);
  await expect(page).toHaveURL(/\/contas$/);
  await expect(cartao.locator('.valor-fatura-mes')).toContainText('R$ 612,30');
  expect(problemas.lista).toEqual([]);
});
