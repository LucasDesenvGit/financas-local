import { expect, test } from './base';

/** Celular (#185, #131): a gaveta do menu abre pelo ☰, leva a outra tela e fecha pelo ✕ e pelo Esc. */
test.describe('celular', () => {
  test.skip(({ isMobile }) => !isMobile, 'só no projeto de celular');

  test('a gaveta abre, navega e fecha', async ({ page, problemas }) => {
    await page.goto('/painel');
    const menu = page.getByRole('button', { name: 'Menu de navegação' });
    await menu.click();
    await expect(menu).toHaveAttribute('aria-expanded', 'true');
    await page.getByRole('navigation').getByRole('link', { name: 'Orçamentos' }).click();
    await expect(page.getByRole('heading', { level: 1 })).toHaveText(/Orçamentos/);

    await menu.click();
    await page.getByRole('button', { name: 'Fechar menu' }).click();
    await expect(menu).toHaveAttribute('aria-expanded', 'false');

    await menu.click();
    await page.keyboard.press('Escape');
    await expect(menu).toHaveAttribute('aria-expanded', 'false');
    expect(problemas.lista).toEqual([]);
  });

  test('nenhuma tela passa da largura do celular (sem rolagem para o lado)', async ({ page }) => {
    for (const rota of ['/painel', '/transacoes', '/investimentos', '/configuracoes']) {
      await page.goto(rota);
      await page.waitForLoadState('networkidle');
      const sobra = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
      expect(sobra, `${rota} rola para o lado`).toBeLessThanOrEqual(1);
    }
  });
});
