import { expect, test } from './base';

/**
 * #173 b) (ADR 0044): o aviso do Painel dos trocos classificados na sincronização leva a Transações só com eles (chip
 * removível, ADR 0018) e o "Ok" dispensa o aviso.
 */
test('Painel: trocos classificados na sincronização, com "Ver em Transações" e "Ok"', async ({ page, api, problemas }) => {
  await page.goto('/painel');
  const aviso = page.locator('.aviso-trocos');
  await expect(aviso).toContainText('foram para “Investimentos” na sincronização');
  await aviso.getByRole('link', { name: 'Ver em Transações' }).click();
  await expect(page).toHaveURL(/\/transacoes\?.*de=trocos-automaticos/);
  await expect(page.getByRole('region', { name: 'Filtros ativos' })).toContainText('Trocos classificados na sincronização: 2 lançamentos');
  await page.goBack();
  await page.locator('.aviso-trocos').getByRole('button', { name: 'Ok, os trocos ficam em Investimentos' }).click();
  await expect.poll(() => api.chamadas).toContain('POST /api/carteira/conferencia/trocos-automaticos/dispensar');
  expect(problemas.lista).toEqual([]);
});
