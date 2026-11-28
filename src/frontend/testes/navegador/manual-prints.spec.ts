import { expect, test } from './base';

/**
 * Prints do Manual (#752, ADR 0071), no navegador de verdade: a imagem do tema aberto carrega (o servidor serve `.webp` e a
 * CSP deixa), tem o tamanho da própria imagem e, no celular, não passa da largura da tela (max-width: 100%).
 */
for (const tema of ['claro', 'escuro'] as const) {
  test(`o print da seção aparece no tema ${tema}`, async ({ page, isMobile }) => {
    await page.emulateMedia({ colorScheme: tema === 'escuro' ? 'dark' : 'light' });
    await page.goto('/manual/configuracoes#aparencia');
    const img = page.locator('#aparencia img.print-manual');
    await expect(img).toBeVisible();
    await expect(img).toHaveAttribute('src', `/imagens/manual/aparencia-${tema}.webp`);
    await expect(img).toHaveAttribute('alt', /paletas de cores/);
    await expect.poll(() => img.evaluate((e: HTMLImageElement) => e.complete && e.naturalWidth)).toBeGreaterThan(0);

    const caixa = (await img.boundingBox())!;
    const largura = Number(await img.getAttribute('width'));
    const janela = page.viewportSize()!.width;
    if (isMobile) {
      expect(caixa.width).toBeLessThanOrEqual(janela);
      const sobra = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
      expect(sobra, 'a página rola para o lado').toBeLessThanOrEqual(1);
    } else {
      expect(Math.round(caixa.width)).toBe(largura);
    }
  });
}
