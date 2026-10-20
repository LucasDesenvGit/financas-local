import { expect, test, TELAS } from './base';

/**
 * Regressão no navegador (#185): cada tela do menu abre no build de produção, com a CSP da API, sem erro no console,
 * sem exceção e sem chamada que a API falsa não conhece. Roda em computador e em celular (projetos do
 * playwright.config.ts).
 */
for (const { rota, titulo } of TELAS) {
  test(`${rota} abre sem erro`, async ({ page, problemas }) => {
    await page.goto(rota);
    await expect(page.getByRole('heading', { level: 1 }).first()).toHaveText(titulo);
    await page.waitForLoadState('networkidle');
    expect(problemas.lista).toEqual([]);
  });
}

test('o endereço desconhecido volta para o Painel', async ({ page }) => {
  await page.goto('/nao-existe');
  await expect(page).toHaveURL(/\/painel$/);
});

test('o Manual abre o capítulo pelo card inteiro', async ({ page }) => {
  await page.goto('/manual');
  await page.locator('.grade-capitulos li').first().click({ position: { x: 20, y: 80 } });
  await expect(page).toHaveURL(/\/manual\/primeiros-passos$/);
});

test('o menu leva de uma tela a outra, e o voltar do navegador funciona', async ({ page, isMobile }) => {
  test.skip(isMobile, 'no celular o menu é a gaveta (celular.spec.ts)');
  await page.goto('/painel');
  await page.getByRole('navigation').getByRole('link', { name: 'Metas' }).click();
  await expect(page.getByRole('heading', { level: 1 })).toHaveText(/Metas/);
  await page.goBack();
  await expect(page).toHaveURL(/\/painel$/);
});
