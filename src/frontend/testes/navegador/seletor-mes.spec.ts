import { expect, test } from './base';

/**
 * #342: no seletor de mês do Painel, o ano do quadro abre uma página de 12 anos (como o título do calendário, #264);
 * escolher um ano volta aos meses dele, e escolher o mês leva o Painel para ele. Sem erro de console nem de CSP.
 */
test('Painel › seletor de mês: o ano abre os anos, e daí o mês escolhido', async ({ page, problemas }) => {
  await page.goto('/painel');
  const rotulo = page.locator('.seletor-mes-rotulo').first();
  await rotulo.click();
  const painel = page.locator('.seletor-mes-painel');
  const anoAtual = new Date().getFullYear();
  const botaoAno = painel.locator('.calendario-titulo-botao');
  await expect(botaoAno).toHaveText(String(anoAtual));
  expect((await botaoAno.boundingBox())!.height).toBeGreaterThanOrEqual(23.5);
  await botaoAno.click();
  await expect(painel.locator('.seletor-mes-opcao-ano')).toHaveCount(12);
  await painel.getByRole('button', { name: String(anoAtual - 3), exact: true }).click();
  await expect(painel.locator('.calendario-titulo-botao')).toHaveText(String(anoAtual - 3));
  await painel.getByRole('button', { name: `março de ${anoAtual - 3}` }).click();
  await expect(rotulo).toContainText(`março de ${anoAtual - 3}`);
  await expect(page).toHaveURL(new RegExp(`ano=${anoAtual - 3}&mes=3`));
  expect(problemas.lista).toEqual([]);
});
