import { expect, test } from './base';

/**
 * #334: no Painel, "Últimas transações" vai até hoje; "Futuras (N)" troca para as parcelas já lançadas com data
 * futura. O botão tem alvo de 24 px (WCAG 2.5.8) e o cabeçalho cabe no celular. Sem erro de console nem de CSP.
 */
test('Painel › Últimas transações: "Até hoje" e "Futuras" separados', async ({ page, problemas }) => {
  await page.goto('/painel');
  const mais = page.getByRole('button', { name: /Mais análises/ });
  if ((await mais.getAttribute('aria-expanded')) !== 'true') await mais.click();
  const bloco = page.locator('.painel-bloco', { has: page.getByRole('heading', { name: /transações$/ }) });
  await expect(bloco.getByRole('heading')).toHaveText('Últimas transações');
  await expect(bloco).not.toContainText('Loja Exemplo 11/11');
  const futuras = bloco.getByRole('button', { name: 'Futuras (1)' });
  expect((await futuras.boundingBox())!.height).toBeGreaterThanOrEqual(23.5);
  await futuras.click();
  await expect(bloco.getByRole('heading')).toHaveText('Próximas transações');
  await expect(bloco).toContainText('Loja Exemplo 11/11');
  await expect(futuras).toHaveAttribute('aria-pressed', 'true');
  const sobra = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(sobra).toBeLessThanOrEqual(1);
  expect(problemas.lista).toEqual([]);
});
