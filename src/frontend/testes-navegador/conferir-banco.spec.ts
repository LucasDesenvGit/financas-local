import { expect, test } from './base';

/**
 * #172: "Conferir com o banco" no navegador: a tela abre pelo botão de Contas e cartões, o valor digitado é comparado na
 * hora e a pista do mesmo valor da diferença vem primeiro, com o link para o detalhe.
 */
test('abre por Contas e cartões, compara o valor do app e mostra a pista que explica', async ({ page, problemas }) => {
  await page.goto('/contas');
  await page.getByRole('link', { name: 'Conferir com o banco' }).click();
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Conferir com o banco');

  const conta = page.getByRole('textbox', { name: 'Valor no app: Conta Corrente' });
  await conta.fill('3.000,00');
  await conta.blur();
  const linha = page.locator('#conta-1');
  await expect(linha).toContainText('a mais');
  await expect(linha.locator('.pistas li').first()).toContainText('Explica a diferença');
  await expect(linha.locator('.pistas li').first().getByRole('link', { name: 'ver' })).toHaveAttribute('href', '/investimentos?aba=reservas');

  const cartao = page.getByRole('textbox', { name: 'Valor no app: Cartão Roxo' });
  await cartao.fill('511,72');
  await cartao.blur();
  await expect(page.locator('#cartao-2')).toContainText('bate');
  expect(problemas.lista).toEqual([]);
});
