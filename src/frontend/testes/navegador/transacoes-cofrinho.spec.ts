import { expect, test } from './base';

/**
 * #758 (ADR 0063): o "+ Lançamento" de Transações lança a compra paga direto de um cofrinho. A Conta Exemplo (Open
 * Finance) tem o cofrinho "Emergência" na API falsa: escolhida no grupo do cofrinho, aparece o campo "Cofrinho", e o
 * POST leva o nome dele.
 */
test('Transações: lançamento pago direto de um cofrinho (#758)', async ({ page, problemas }) => {
  await page.goto('/transacoes');
  await page.getByRole('button', { name: '+ Lançamento' }).click();
  const dialogo = page.getByRole('dialog');
  await expect(dialogo.getByLabel('Cofrinho')).toHaveCount(0);

  await dialogo.getByLabel('Conta').selectOption({ label: 'Conta Exemplo' });
  await dialogo.getByLabel('Cofrinho').selectOption({ label: 'Emergência' });
  await expect(dialogo.getByText('O saldo da conta não muda')).toBeVisible();
  await dialogo.getByLabel('Valor').fill('89,90');
  await dialogo.getByLabel('Descrição').fill('Compra no Mercado Livre');

  const pedido = page.waitForRequest((r) => r.method() === 'POST' && new URL(r.url()).pathname === '/api/transacoes');
  await dialogo.getByRole('button', { name: 'Salvar' }).click();
  expect((await pedido).postDataJSON()).toMatchObject({ contaId: 1, reservaNome: 'Emergência', valor: 89.9, tipo: 1 });
  expect(problemas.lista).toEqual([]);
});
