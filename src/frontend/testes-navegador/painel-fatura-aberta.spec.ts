import { expect, test } from './base';

/**
 * #322: o interruptor "Fatura no Painel" de um cartão, em Contas e cartões, põe a fatura em aberto dele em "Situação
 * hoje" no Painel (a escolha fica neste navegador). Sem erro de console nem bloqueio de CSP, sem alargar a página.
 */
test('Contas › "Fatura no Painel" mostra a fatura em aberto do cartão no Painel', async ({ page, problemas }) => {
  await page.goto('/contas');
  const interruptor = page.getByRole('switch', { name: 'Fatura no Painel (Cartão Crédito)' });
  await expect(interruptor).not.toBeChecked();
  const caixa = (await interruptor.locator('xpath=..').boundingBox())!;
  expect(caixa.height).toBeGreaterThanOrEqual(23.5);
  await interruptor.click();
  await expect(interruptor).toBeChecked();

  await page.goto('/painel');
  const card = page.locator('.kpi-fatura-aberta');
  await expect(card).toHaveCount(1);
  await expect(card).toContainText('Fatura aberta · Cartão Crédito');
  await expect(card.getByRole('link', { name: 'Ver as faturas de Cartão Crédito' })).toHaveAttribute('href', '/contas?faturas=3');
  const sobra = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(sobra).toBeLessThanOrEqual(1);
  expect(problemas.lista).toEqual([]);
});
