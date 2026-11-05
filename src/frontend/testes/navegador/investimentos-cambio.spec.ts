import { expect, test } from './base';

/**
 * #247: o câmbio na aba Resumo de Investimentos (API falsa: USD 5,4321, EUR 6,3012, JPY 0,0362, GBP 7,2154 e
 * CHF 6,7012 de compra). Dólar, euro e iene por 100, com a data; a PTAX tem o ⓘ do glossário (abre por clique).
 * #278: a bandeira de cada moeda (arquivo do próprio site, sem bloqueio de CSP) e "Mais moedas" com as outras da PTAX.
 */
test('Investimentos: câmbio do dólar, do euro e do iene (por 100), com a data e o ⓘ da PTAX', async ({ page, problemas }) => {
  await page.goto('/investimentos');
  const bloco = page.getByRole('region', { name: 'Câmbio' });
  await expect(bloco).toBeVisible();
  const linhas = bloco.getByRole('listitem');
  await expect(linhas).toHaveCount(3);
  await expect(linhas.nth(0)).toContainText('Dólar americano');
  await expect(linhas.nth(0)).toContainText('R$ 5,43');
  await expect(linhas.nth(1)).toContainText('R$ 6,30');
  await expect(linhas.nth(2)).toContainText('100 ienes');
  await expect(linhas.nth(2)).toContainText('R$ 3,62');
  await expect(linhas.nth(0)).toContainText(/PTAX de \d{2}\/\d{2}\/\d{4}/);

  // O ⓘ da PTAX abre a definição por clique (funciona no toque e no teclado, não só no title=).
  await bloco.getByRole('button', { name: /PTAX/ }).click();
  await expect(page.getByText('A cotação oficial do dólar')).toBeVisible();
  expect(problemas.lista).toEqual([]);
});

test('#278: bandeira de cada moeda, à direita do código, carregada do próprio site', async ({ page, problemas }) => {
  await page.goto('/investimentos');
  const bloco = page.getByRole('region', { name: 'Câmbio' });
  const bandeiras = bloco.locator('li:visible img.bandeira');
  await expect(bandeiras).toHaveCount(3);
  for (const img of await bandeiras.all()) {
    // A imagem carregou de verdade (largura natural > 0): o arquivo existe e a CSP deixou.
    await expect.poll(() => img.evaluate((el: HTMLImageElement) => el.complete && el.naturalWidth)).toBeGreaterThan(0);
    await expect(img).toHaveAttribute('alt', '');
  }
  const primeira = bloco.locator('li').first();
  const codigo = await primeira.locator('.codigo-moeda').boundingBox();
  const bandeira = await primeira.locator('img.bandeira').boundingBox();
  expect(bandeira!.x, 'a bandeira fica à direita do código').toBeGreaterThan(codigo!.x + codigo!.width - 0.5);
  expect(problemas.lista).toEqual([]);
});

test('#278: "Mais moedas" mostra as outras da PTAX, pelo teclado também, e lembra a escolha', async ({ page, problemas }) => {
  await page.goto('/investimentos');
  const bloco = page.getByRole('region', { name: 'Câmbio' });
  const botao = bloco.getByRole('button', { name: /Mais 7 moedas/ });
  await expect(botao).toHaveAttribute('aria-expanded', 'false');
  await botao.focus();
  await page.keyboard.press('Enter');
  const menos = bloco.getByRole('button', { name: 'Menos moedas' });
  await expect(menos).toHaveAttribute('aria-expanded', 'true');
  await expect(bloco.getByRole('listitem')).toHaveCount(10);
  await expect(bloco.getByRole('listitem').filter({ hasText: 'Libra esterlina' })).toContainText('R$ 7,22');
  await expect(bloco.getByRole('listitem').filter({ hasText: 'Coroa sueca' })).toContainText('sem cotação guardada ainda');

  // A escolha fica lembrada neste navegador (ADR 0017).
  await page.reload();
  await expect(page.getByRole('region', { name: 'Câmbio' }).getByRole('listitem')).toHaveCount(10);
  expect(problemas.lista).toEqual([]);
});
