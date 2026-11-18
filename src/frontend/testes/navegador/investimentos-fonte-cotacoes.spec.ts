import { expect, test } from './base';

/**
 * #763 (ADR 0072): a janela "Fonte das cotações" só explica de onde vêm as cotações (o Yahoo Finance). A brapi saiu: não
 * há campo de token nem chamada a /api/carteira/configuracao (a API falsa não a simula mais, e a chamada reprovaria o teste).
 */
test('Investimentos: a fonte das cotações mostra só o Yahoo, sem token', async ({ page }) => {
  await page.goto('/investimentos?aba=rv');
  await page.getByRole('button', { name: 'fonte das cotações' }).click();
  const janela = page.getByRole('dialog', { name: 'Fonte das cotações' });
  await expect(janela).toContainText('Yahoo Finance');
  await expect(janela).not.toContainText(/brapi|token/i);
  await expect(janela.getByRole('textbox')).toHaveCount(0);
  await janela.getByRole('button', { name: 'Fechar' }).last().click();
  await expect(janela).toBeHidden();
});
