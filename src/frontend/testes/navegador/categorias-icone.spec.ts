import { expect, test } from './base';

/**
 * #336: as categorias com o ícone na cor delas no lugar da bolinha, e a rosca do Painel com o total no centro. Sem erro
 * de console nem de CSP, e sem rolagem para o lado no celular.
 */
test('Painel: rosca com o total no centro e a lista de categorias com o ícone', async ({ page, problemas }) => {
  await page.goto('/painel');
  const lista = page.locator('.lista-categorias li');
  await expect(lista.first().locator('app-icone-categoria svg')).toBeVisible();
  await expect(page.locator('.lista-categorias .ponto')).toHaveCount(0);
  const centro = page.locator('.rosca-centro');
  await expect(centro).toContainText('em despesas');
  const [caixaCentro, caixaGrafico] = [(await centro.locator('strong').boundingBox())!, (await page.locator('.rosca canvas').boundingBox())!];
  // O total fica no meio da rosca (folga de alguns px pela altura da linha de baixo).
  expect(Math.abs((caixaCentro.x + caixaCentro.width / 2) - (caixaGrafico.x + caixaGrafico.width / 2))).toBeLessThan(2);
  // O ícone é um alvo decorativo, mas do tamanho combinado: 28 px.
  const icone = (await lista.first().locator('.icone-categoria').boundingBox())!;
  expect(icone.width).toBeGreaterThanOrEqual(27.5);
  const sobra = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(sobra).toBeLessThanOrEqual(1);
  expect(problemas.lista).toEqual([]);
});

test('Categorias e regras: cada categoria com o ícone, sem bolinha', async ({ page, problemas }) => {
  await page.goto('/categorias');
  await expect(page.locator('main app-icone-categoria').first()).toBeVisible();
  await expect(page.locator('main .lista-simples .ponto')).toHaveCount(0);
  expect(problemas.lista).toEqual([]);
});
