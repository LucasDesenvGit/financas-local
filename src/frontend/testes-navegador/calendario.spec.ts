import { expect, test } from './base';

/**
 * #264: no navegador, o calendário do campo de data troca de nível pelo título (dias → meses → anos) e volta ao
 * escolher; meses e anos com alvo de pelo menos 24 × 24 px (WCAG 2.5.8), também no celular; nada passa da tela.
 */
test('calendário: título leva a meses e anos, escolher volta aos dias', async ({ page, problemas }) => {
  await page.goto('/transacoes');
  const campoDe = page.locator('app-campo-data').first();
  await campoDe.getByRole('button', { name: 'Escolher a data no calendário' }).click();
  const calendario = page.getByRole('dialog');
  const titulo = calendario.locator('.calendario-topo strong');
  await expect(titulo).toHaveText(/ de \d{4}$/);

  await calendario.locator('.calendario-titulo-botao').click();
  await expect(titulo).toHaveText(/^\d{4}$/);
  const celulas = calendario.locator('.calendario-celula');
  await expect(celulas).toHaveCount(12);
  for (const c of await celulas.all()) {
    const caixa = (await c.boundingBox())!;
    expect(caixa.width).toBeGreaterThanOrEqual(24);
    expect(caixa.height).toBeGreaterThanOrEqual(24);
  }

  await calendario.locator('.calendario-titulo-botao').click();
  await expect(titulo).toHaveText(/^\d{4}–\d{4}$/);
  await calendario.getByRole('button', { name: 'Anos anteriores' }).click();
  const ano = Number((await celulas.first().textContent())!.trim());
  await celulas.first().click();
  await expect(titulo).toHaveText(String(ano));
  await calendario.getByRole('button', { name: `março de ${ano}` }).click();
  await expect(titulo).toHaveText(`março de ${ano}`);

  const caixa = (await calendario.boundingBox())!;
  expect(caixa.x).toBeGreaterThanOrEqual(0);
  expect(caixa.x + caixa.width).toBeLessThanOrEqual(page.viewportSize()!.width);
  expect(problemas.lista).toEqual([]);
});
