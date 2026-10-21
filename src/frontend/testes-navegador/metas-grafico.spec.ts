import { expect, test } from './base';

/**
 * #318: "Como está indo" abre o gráfico da meta (canvas com alternativa em texto, WCAG 1.1.1) e a lista dos últimos
 * aportes, sem erro de console nem bloqueio de CSP, e sem alargar o cartão no celular (API falsa: 4 aportes da Viagem).
 */
test('Metas › Como está indo: o gráfico da meta e os últimos aportes', async ({ page, problemas }) => {
  await page.goto('/metas');
  const cartao = page.locator('.cartao-meta').filter({ hasText: 'Viagem' });
  const botao = cartao.getByRole('button', { name: 'Como está indo' });
  await expect(botao).toHaveAttribute('aria-expanded', 'false');
  await botao.click();
  await expect(botao).toHaveAttribute('aria-expanded', 'true');

  const area = cartao.locator('#evolucao-meta-1');
  await expect(area.locator('canvas')).toBeVisible();
  await expect(area.locator('[role="img"]')).toHaveAttribute('aria-label', /4 aportes registrados/);
  await expect(area.locator('.lista-aportes li')).toHaveCount(4);
  await expect(area.locator('.lista-aportes li').first()).toContainText('+R$ 700,00');

  const [c, g] = [await cartao.boundingBox(), await area.locator('canvas').boundingBox()];
  expect(g!.x + g!.width).toBeLessThanOrEqual(c!.x + c!.width + 0.5);
  const sobra = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(sobra).toBeLessThanOrEqual(1);

  await botao.click();
  await expect(area.locator('canvas')).toHaveCount(0);
  expect(problemas.lista).toEqual([]);
});
