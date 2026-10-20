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

  /**
   * Achados da varredura de ponta a ponta (20/10/2026): nada faz a tela rolar para o lado (WCAG 1.4.10; a dica
   * escondida do ícone na borda direita fazia Categorias rolar 3 px) e todo botão ou link solto tem 24 × 24 px
   * (WCAG 2.5.8; link dentro de frase é exceção da própria regra).
   */
  test(`${rota}: sem rolagem para o lado e alvos de 24 px`, async ({ page }) => {
    await page.goto(rota);
    await page.waitForLoadState('networkidle');
    const sobra = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
    expect(sobra, 'a tela rola para o lado').toBeLessThanOrEqual(1);
    const pequenos = await page.evaluate(() => [...document.querySelectorAll('button, a[href]')].flatMap((el) => {
      const r = el.getBoundingClientRect();
      const st = getComputedStyle(el);
      if (!r.width || !r.height || st.visibility === 'hidden' || st.display === 'inline') return [];
      return r.width < 23.5 || r.height < 23.5
        ? [`${Math.round(r.width)}×${Math.round(r.height)} ${el.tagName.toLowerCase()}.${[...el.classList].join('.')} "${(el.textContent ?? '').trim().slice(0, 30)}"`]
        : [];
    }));
    expect(pequenos).toEqual([]);

    // A dica do ícone mais perto da borda direita, aberta, também não sai da tela.
    const dicas = page.locator('.com-dica:visible');
    const direitas = await dicas.evaluateAll((els) => els.map((e) => e.getBoundingClientRect().right));
    if (direitas.length) {
      await dicas.nth(direitas.indexOf(Math.max(...direitas))).hover();
      const sobraComDica = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
      expect(sobraComDica, 'a dica aberta faz a tela rolar para o lado').toBeLessThanOrEqual(1);
    }
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
