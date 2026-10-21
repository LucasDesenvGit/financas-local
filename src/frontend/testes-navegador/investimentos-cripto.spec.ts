import { expect, test } from './base';

/**
 * #279 (ADR 0048): criptomoedas em Investimentos, logo abaixo do Câmbio (API falsa: BTC +2,04%, ETH −1,37%, SOL sem
 * variação ainda, USDT sem cotação). A tela só lê o que a API guardou: nada sai do navegador para a internet.
 */
test('Investimentos: criptomoedas em reais, com variação em 24 h e o ⓘ de "criptomoeda"', async ({ page, problemas }) => {
  await page.goto('/investimentos');
  const bloco = page.getByRole('region', { name: 'Criptomoedas' });
  await expect(bloco).toBeVisible();
  const linhas = bloco.getByRole('listitem');
  await expect(linhas).toHaveCount(4);
  await expect(linhas.nth(0)).toContainText('Bitcoin');
  await expect(linhas.nth(0)).toContainText('R$ 419.262,00');
  await expect(linhas.nth(0)).toContainText('+2,04% em 24 h');
  await expect(linhas.nth(1)).toContainText('−1,37% em 24 h');
  await expect(linhas.nth(2)).toContainText('variação em 24 h ainda sem dado');
  await expect(linhas.nth(3)).toContainText('sem cotação guardada ainda');

  // Logo abaixo do Câmbio.
  const cambio = await page.getByRole('region', { name: 'Câmbio' }).boundingBox();
  const cripto = await bloco.boundingBox();
  expect(cripto!.y).toBeGreaterThan(cambio!.y);

  await bloco.getByRole('button', { name: 'O que é Criptomoeda?' }).click();
  await expect(page.getByText('Dinheiro digital negociado pela internet')).toBeVisible();
  expect(problemas.lista).toEqual([]);
});
