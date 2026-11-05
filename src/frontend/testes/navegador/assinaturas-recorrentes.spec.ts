import { expect, test } from './base';

/**
 * #158: "Assinaturas e recorrentes" no navegador (API falsa: Escola e Luz na Conta Exemplo, Streaming no Cartão Exemplo).
 * O de-para por conta e cartão, a busca "onde cai…?" com o filtro visível e na URL, o voltar do navegador e a linha-resumo
 * de Contas e cartões, que abre a aba já filtrada.
 */
test('o de-para por cartão e conta, a busca na URL e a linha-resumo de Contas e cartões', async ({ page, problemas, isMobile }) => {
  await page.goto('/contas');
  const linha = page.getByRole('link', { name: /1 assinatura · R\$\s44,90\/mês/ });
  await expect(linha).toBeVisible();
  await linha.click();
  await expect(page).toHaveURL(/\/contas-a-pagar\?aba=recorrentes&conta=2$/);
  await expect(page.getByRole('tab', { name: 'Assinaturas e recorrentes' })).toHaveAttribute('aria-selected', 'true');
  await expect(page.getByRole('heading', { level: 2, name: 'Cartão Exemplo' })).toBeVisible();
  await expect(page.getByRole('heading', { level: 2, name: 'Conta Exemplo' })).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Remover filtro: Cartão Exemplo' })).toBeVisible();

  await page.getByRole('button', { name: 'Limpar filtros' }).click();
  await expect(page).toHaveURL(/\/contas-a-pagar\?aba=recorrentes$/);
  await expect(page.getByRole('heading', { level: 2, name: 'Conta Exemplo' })).toBeVisible();

  await page.getByLabel('Onde cai…?').fill('streaming');
  await expect(page).toHaveURL(/busca=streaming/);
  await expect(page.getByRole('heading', { level: 2, name: 'Conta Exemplo' })).toHaveCount(0);
  const ver = page.getByRole('link', { name: 'Ver as cobranças de Streaming' });
  await expect(ver).toHaveAttribute('href', /\/transacoes\?busca=STREAMING&tipo=despesa&inicio=.*&conta=2/);

  // O filtro troca a URL com replaceUrl: o voltar sai da tela, em vez de passar por cada letra digitada.
  if (!isMobile) {
    await page.goBack();
    await expect(page).toHaveURL(/\/contas$/);
  }
  expect(problemas.lista).toEqual([]);
});

/**
 * #242: cada conta ou cartão é uma sanfona. Espaço e Enter alternam (botão de verdade), o fechado sai do Tab, a escolha
 * fica depois de recarregar, e o cabeçalho tem alvo de pelo menos 24 px (WCAG 2.5.8) com o selo do banco.
 */
test('sanfona das recorrentes: teclado, lembrada ao recarregar, alvo de 24 px e selo do banco', async ({ page }) => {
  await page.goto('/contas-a-pagar?aba=recorrentes');
  const cartao = page.getByRole('button', { name: /Cartão Exemplo/ }).and(page.locator('.grupo-recorrentes-botao'));
  await expect(cartao).toHaveAttribute('aria-expanded', 'true');
  const caixa = (await cartao.boundingBox())!;
  expect(caixa.height).toBeGreaterThanOrEqual(23.5);
  await expect(cartao.locator('app-logo-banco')).toBeVisible();

  await cartao.focus();
  await page.keyboard.press('Space');
  await expect(cartao).toHaveAttribute('aria-expanded', 'false');
  await expect(page.getByRole('link', { name: 'Ver as cobranças de Streaming' })).toBeHidden();
  await page.keyboard.press('Enter');
  await expect(cartao).toHaveAttribute('aria-expanded', 'true');
  await expect(page.getByRole('link', { name: 'Ver as cobranças de Streaming' })).toBeVisible();

  await cartao.click();
  await page.reload();
  await expect(page.getByRole('button', { name: /Cartão Exemplo/ }).and(page.locator('.grupo-recorrentes-botao')))
    .toHaveAttribute('aria-expanded', 'false');
  await expect(page.getByRole('button', { name: /Conta Exemplo/ }).and(page.locator('.grupo-recorrentes-botao')))
    .toHaveAttribute('aria-expanded', 'true');
});
