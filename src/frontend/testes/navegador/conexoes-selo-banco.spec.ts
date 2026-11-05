import { expect, test } from './base';

/**
 * #376: em Configurações › Conexões, o selo do banco (o mesmo de Contas e cartões) no lugar da bolinha colorida; banco
 * sem selo, o ícone neutro. Sem erro de console nem de CSP, e sem rolagem para o lado no celular.
 */
test('Configurações: cada conexão com o selo do banco no lugar da bolinha', async ({ page, problemas }) => {
  await page.goto('/configuracoes');
  const linhas = page.locator('table tbody tr').filter({ has: page.locator('app-logo-banco') });
  const nubank = linhas.filter({ hasText: 'Nubank' });
  await expect(nubank.locator('svg.logo-banco')).toBeVisible();
  await expect(nubank.locator('.ponto')).toHaveCount(0);
  const outro = linhas.filter({ hasText: 'Banco Exemplo' });
  await expect(outro.locator('.logo-generico')).toBeVisible();
  // O selo fica à esquerda do nome, com o tamanho combinado (28 px; folga de 0,5, #271).
  const [selo, texto] = [(await nubank.locator('app-logo-banco').boundingBox())!, (await nubank.getByText('Nubank').boundingBox())!];
  expect(selo.width).toBeGreaterThanOrEqual(27.5);
  expect(texto.x).toBeGreaterThanOrEqual(selo.x + selo.width);
  const sobra = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(sobra).toBeLessThanOrEqual(1);
  expect(problemas.lista).toEqual([]);
});
