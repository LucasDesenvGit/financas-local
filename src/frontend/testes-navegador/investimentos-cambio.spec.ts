import { expect, test } from './base';

/**
 * #247: o câmbio na aba Resumo de Investimentos (API falsa: USD 5,4321, EUR 6,3012, JPY 0,0362 de compra). Dólar,
 * euro e iene por 100, com a data; a PTAX tem o ⓘ do glossário (abre por clique).
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
