import { expect, test } from './base';

/**
 * Achado da varredura de ponta a ponta (20/10/2026), WCAG 2.1.1: editar uma transação ou uma categoria só abria com o
 * clique do mouse na linha. Agora o nome é um botão: pelo teclado, Enter abre o mesmo formulário.
 */
test('Transações: Enter no nome da transação abre a edição', async ({ page, problemas }) => {
  await page.goto('/transacoes');
  const editar = page.getByRole('button', { name: /^Editar: / }).first();
  await editar.focus();
  await page.keyboard.press('Enter');
  await expect(page.getByRole('dialog')).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('dialog')).toBeHidden();
  expect(problemas.lista).toEqual([]);
});

test('Categorias: Enter no nome da categoria abre a edição', async ({ page, problemas }) => {
  await page.goto('/categorias');
  const editar = page.getByRole('button', { name: /^Editar a categoria / }).first();
  await editar.focus();
  await page.keyboard.press('Enter');
  await expect(page.getByRole('dialog')).toBeVisible();
  expect(problemas.lista).toEqual([]);
});
