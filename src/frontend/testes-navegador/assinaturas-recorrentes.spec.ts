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
