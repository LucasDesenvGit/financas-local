import { expect, test } from './base';

/**
 * #335: em Metas, cada meta é uma sanfona na largura toda. Aberta, no computador, o resumo (anel, valores) fica
 * centralizado à esquerda e o plano à direita; no celular, um embaixo do outro. O título abre e fecha (aria-expanded),
 * e a escolha continua ao recarregar. A meta sem data alvo diz "sem data definida".
 */
test('Metas: cada meta em sanfona, com o resumo e o plano lado a lado', async ({ page, problemas }) => {
  await page.goto('/metas');
  const cartoes = page.locator('.cartao-meta');
  await expect(cartoes).toHaveCount(2);
  const grade = (await page.locator('.lista-metas').boundingBox())!;
  for (const c of [cartoes.nth(0), cartoes.nth(1)]) expect((await c.boundingBox())!.width).toBeGreaterThan(grade.width - 2);

  const viagem = cartoes.filter({ hasText: 'Viagem' });
  const titulo = viagem.getByRole('button', { name: /^Viagem/ });
  await expect(titulo).toHaveAttribute('aria-expanded', 'true');
  expect((await titulo.boundingBox())!.height).toBeGreaterThanOrEqual(23.5);
  const [anel, plano, resumo] = [(await viagem.locator('.anel').boundingBox())!, (await viagem.locator('.recomendado').first().boundingBox())!,
    (await viagem.locator('.meta-resumo').boundingBox())!];
  if (page.viewportSize()!.width > 900) {
    expect(plano.x).toBeGreaterThan(anel.x + anel.width);
    expect(Math.abs((anel.x + anel.width / 2) - (resumo.x + resumo.width / 2))).toBeLessThan(3);
  } else {
    expect(plano.y).toBeGreaterThan(anel.y + anel.height - 1);
  }

  await titulo.click();
  await expect(titulo).toHaveAttribute('aria-expanded', 'false');
  await expect(viagem.locator('.anel')).toBeHidden();
  await page.reload();
  await expect(page.locator('.cartao-meta').filter({ hasText: 'Viagem' }).getByRole('button', { name: /^Viagem/ }))
    .toHaveAttribute('aria-expanded', 'false');
  const sobra = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(sobra).toBeLessThanOrEqual(1);
  expect(problemas.lista).toEqual([]);
});
