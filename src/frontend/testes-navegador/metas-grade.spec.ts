import { expect, test } from './base';

/**
 * #335: a grade de Metas se ajusta à quantidade. Com uma meta só, o card ocupa a largura toda: no computador, o resumo
 * (anel, valores) à esquerda e o plano à direita; no celular, um embaixo do outro. Com duas, cada uma na metade.
 */
test('Metas: com uma meta só, o card ocupa a largura toda, com o resumo e o plano lado a lado', async ({ page, problemas }) => {
  await page.goto('/metas');
  const base = await page.evaluate(async () => (await fetch('/api/metas')).json());
  await page.route(/\/api\/metas$/, (r) => r.request().method() === 'GET' ? r.fulfill({ json: base.slice(0, 1) }) : r.fallback());
  await page.reload();
  const grade = page.locator('.grade-metas');
  const cartao = page.locator('.cartao-meta');
  await expect(cartao).toHaveCount(1);
  await expect(grade).toHaveClass(/metas-uma/);
  const [g, c] = [(await grade.boundingBox())!, (await cartao.boundingBox())!];
  expect(c.width).toBeGreaterThan(g.width - 2);
  const [anel, detalhe] = [(await cartao.locator('.anel').boundingBox())!, (await cartao.locator('.recomendado').first().boundingBox())!];
  if (page.viewportSize()!.width > 900) {
    expect(detalhe.x).toBeGreaterThan(anel.x + anel.width);
    // O resumo fica no meio da coluna dele: o anel centralizado entre a borda do card e a divisória.
    const coluna = (await cartao.locator('.meta-resumo').boundingBox())!;
    expect(Math.abs((anel.x + anel.width / 2) - (coluna.x + coluna.width / 2))).toBeLessThan(3);
  } else {
    expect(detalhe.y).toBeGreaterThan(anel.y + anel.height - 1);
  }
  const sobra = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(sobra).toBeLessThanOrEqual(1);
  expect(problemas.lista).toEqual([]);
});

test('Metas: com duas metas, cada uma ocupa metade da linha no computador', async ({ page, problemas }) => {
  test.skip(page.viewportSize()!.width <= 760, 'no celular, uma por linha');
  await page.goto('/metas');
  const cartoes = page.locator('.cartao-meta');
  await expect(cartoes).toHaveCount(2);
  const [a, b] = [(await cartoes.nth(0).boundingBox())!, (await cartoes.nth(1).boundingBox())!];
  expect(Math.abs(a.y - b.y)).toBeLessThan(2);
  expect(Math.abs(a.width - b.width)).toBeLessThan(2);
  const g = (await page.locator('.grade-metas').boundingBox())!;
  expect(a.width + b.width).toBeGreaterThan(g.width - 40);
  expect(problemas.lista).toEqual([]);
});
