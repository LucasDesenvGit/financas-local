import { expect, test } from './base';

/**
 * #359: no formulário de meta, "Quanto vou guardar por mês" calcula quando a meta chega ao alvo e preenche a data alvo,
 * com a frase dizendo o mês. Sem erro de console nem de CSP.
 */
test('Metas › nova meta: o valor por mês preenche a data alvo', async ({ page, problemas }) => {
  await page.goto('/metas');
  await page.getByRole('button', { name: /Nova meta/ }).click();
  const dialogo = page.getByRole('dialog');
  await dialogo.locator('input[name="nome"]').fill('Viagem');
  await dialogo.locator('app-campo-valor[name="alvo"] input').fill('6000');
  const porMes = dialogo.locator('app-campo-valor[name="porMes"] input');
  await porMes.fill('1000');
  await porMes.blur();
  await expect(dialogo.locator('.nota-por-mes')).toContainText('(6 meses)');
  const data = new Date(new Date().getFullYear(), new Date().getMonth() + 5, 1);
  await expect(dialogo.locator('app-campo-data[name="data"] input')).toHaveValue(
    `${String(data.getMonth() + 1).padStart(2, '0')}/${data.getFullYear()}`.replace(/^/, '01/'));
  expect(problemas.lista).toEqual([]);
});
