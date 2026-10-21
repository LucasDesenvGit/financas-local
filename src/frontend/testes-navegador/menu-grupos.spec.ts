import { expect, test } from './base';

/**
 * Menu em grupos (#227, ADR 0046), no navegador de verdade: o que o Jest não alcança (Enter e Espaço nativos do botão,
 * a ordem do Tab, o tamanho desenhado do cabeçalho e a gaveta do celular).
 */
test('teclado: Enter e Espaço abrem e fecham o grupo, e tela de grupo fechado sai do Tab', async ({ page, isMobile, problemas }) => {
  test.skip(isMobile, 'no celular, a gaveta é conferida no outro teste');
  await page.goto('/painel');
  const menu = page.getByRole('navigation', { name: 'Menu principal' });
  const patrimonio = menu.getByRole('button', { name: 'Patrimônio' });
  await expect(patrimonio).toHaveAttribute('aria-expanded', 'true');

  await patrimonio.focus();
  await page.keyboard.press('Enter');
  await expect(patrimonio).toHaveAttribute('aria-expanded', 'false');
  await expect(menu.getByRole('link', { name: 'Investimentos' })).toBeHidden();
  // Tab sai do cabeçalho fechado direto para o próximo grupo, sem passar pelas telas escondidas.
  await page.keyboard.press('Tab');
  await expect(menu.getByRole('button', { name: 'Análises' })).toBeFocused();

  await patrimonio.focus();
  await page.keyboard.press('Space');
  await expect(patrimonio).toHaveAttribute('aria-expanded', 'true');
  await page.keyboard.press('Tab');
  await expect(menu.getByRole('link', { name: 'Investimentos' })).toBeFocused();

  // Alvo de pelo menos 24×24 px (WCAG 2.5.8).
  const caixa = (await patrimonio.boundingBox())!;
  expect(caixa.height).toBeGreaterThanOrEqual(23.5);
  expect(caixa.width).toBeGreaterThanOrEqual(23.5);
  expect(problemas.lista).toEqual([]);
});

test('o grupo da tela aberta abre sozinho e a escolha volta ao recarregar', async ({ page, isMobile }) => {
  test.skip(isMobile, 'no celular, a gaveta é conferida no outro teste');
  await page.goto('/painel');
  const menu = page.getByRole('navigation', { name: 'Menu principal' });
  await menu.getByRole('button', { name: 'Ajustes' }).click();
  await expect(menu.getByRole('button', { name: 'Ajustes' })).toHaveAttribute('aria-expanded', 'false');

  await page.reload();
  await expect(menu.getByRole('button', { name: 'Ajustes' })).toHaveAttribute('aria-expanded', 'false');

  await page.goto('/configuracoes');
  await expect(menu.getByRole('button', { name: 'Ajustes' })).toHaveAttribute('aria-expanded', 'true');
  await expect(menu.getByRole('link', { name: 'Configurações' })).toHaveClass(/ativo/);
});

test('celular: tocar no grupo não fecha a gaveta; tocar numa tela fecha', async ({ page, isMobile, problemas }) => {
  test.skip(!isMobile, 'só no projeto de celular');
  await page.goto('/painel');
  const botaoMenu = page.getByRole('button', { name: 'Menu de navegação' });
  await botaoMenu.click();
  const menu = page.getByRole('navigation', { name: 'Menu principal' });
  const planejamento = menu.getByRole('button', { name: 'Planejamento' });
  await planejamento.click();
  await expect(planejamento).toHaveAttribute('aria-expanded', 'false');
  await expect(botaoMenu).toHaveAttribute('aria-expanded', 'true');
  await planejamento.click();
  await menu.getByRole('link', { name: 'Metas' }).click();
  await expect(page.getByRole('heading', { level: 1 })).toHaveText(/Metas/);
  await expect(botaoMenu).toHaveAttribute('aria-expanded', 'false');
  expect(problemas.lista).toEqual([]);
});
