import { expect, test } from './base';

/**
 * #244: ordenar Transações pelo cabeçalho, no navegador. A ordem vai na chamada da API (a lista é paginada lá) e na URL;
 * o cabeçalho é botão (teclado) com alvo de 24 px, e o th da coluna que vale tem aria-sort.
 */
test('Transações: ordenar pelo cabeçalho, por clique e por teclado', async ({ page, api }) => {
  await page.goto('/transacoes');
  const tabela = page.locator('table.tabela').first();
  const valor = tabela.getByRole('button', { name: /^Valor/ });
  await expect(valor).toBeVisible();
  const caixa = (await valor.boundingBox())!;
  // 24 px pedidos no CSS; com as fontes do Linux da CI a caixa sai 23,99997 (arredondamento): mesma folga do telas.spec.
  expect(caixa.height).toBeGreaterThanOrEqual(23.5);
  await expect(tabela.getByRole('columnheader', { name: /^Data/ })).toHaveAttribute('aria-sort', 'descending');

  await valor.click();
  await expect(page).toHaveURL(/ordem=valor/);
  await expect(page).toHaveURL(/direcao=desc/);
  await expect.poll(() => api.chamadasComQuery.some((c) => c.startsWith('GET /api/transacoes?') && c.includes('ordem=valor')
    && c.includes('direcao=desc'))).toBe(true);
  await expect(tabela.getByRole('columnheader', { name: /^Valor/ })).toHaveAttribute('aria-sort', 'descending');
  await expect(tabela.getByRole('columnheader', { name: /^Data/ })).not.toHaveAttribute('aria-sort', /.+/);

  // Teclado: Tab até "Conta" e Enter (o botão é de verdade).
  await tabela.getByRole('button', { name: /^Conta/ }).focus();
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/ordem=conta/);
  await expect(tabela.getByRole('columnheader', { name: /^Conta/ })).toHaveAttribute('aria-sort', 'ascending');

  // F5 mantém a ordem.
  await page.reload();
  await expect(page.locator('table.tabela').first().getByRole('columnheader', { name: /^Conta/ }))
    .toHaveAttribute('aria-sort', 'ascending');
});
