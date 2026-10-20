import { expect, test } from './base';

/**
 * #158: Contas a pagar no navegador, com a API falsa: a conta cobrada sozinha diz onde cai e não tem "Pagar"; as
 * sugestões do cartão (mensal e anual) dizem como são cobradas; o formulário mostra o texto da cobrança só para a
 * cobrada sozinha.
 */
test('Contas a pagar: forma de cobrança, baixa sozinha e sugestões do cartão', async ({ page, problemas }) => {
  await page.goto('/contas-a-pagar');
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Contas a pagar');

  await expect(page.getByRole('button', { name: 'Pagar Escola' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Pagar Luz' })).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Pagar Streaming' })).toHaveCount(0);
  await expect(page.getByText('baixa sozinha quando a cobrança com “STREAMING” chega')).toBeVisible();

  const sugestoes = page.locator('section.sugestoes');
  await expect(sugestoes).toContainText('no cartão de crédito');
  await expect(sugestoes).toContainText('anual: pago em');

  await page.getByRole('button', { name: 'Cadastrar MUSICA APP' }).click();
  const dialogo = page.getByRole('dialog');
  await expect(dialogo.getByLabel('Como é cobrada')).toBeVisible();
  await expect(dialogo.getByLabel('Texto da cobrança no extrato')).toHaveValue('MUSICA');
  await dialogo.getByLabel('Como é cobrada').selectOption({ label: 'paga à mão (boleto, Pix)' });
  await expect(dialogo.getByLabel('Texto da cobrança no extrato')).toHaveCount(0);
  expect(problemas.lista).toEqual([]);
});
