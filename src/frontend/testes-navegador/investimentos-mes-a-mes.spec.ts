import { expect, test } from './base';

/**
 * #173: a seção "Mês a mês" da aba FIIs no navegador, com a API falsa (um FII e seis meses fictícios): o gráfico explicado
 * com a frase-resumo, a tabela com o mês mais recente primeiro e o seletor de 12/24 meses, sem erro nem rolagem para o lado.
 */
test('Investimentos › FIIs › Mês a mês: gráfico com frase-resumo, tabela e seletor', async ({ page, problemas }) => {
  await page.goto('/investimentos?aba=fii');
  const secao = page.locator('section.mes-a-mes');
  await expect(secao.getByRole('heading', { name: 'Mês a mês' })).toBeVisible();

  const tabela = secao.getByRole('table');
  await expect(tabela.getByRole('columnheader')).toHaveText([/Mês/, /Aportes/, /Rendimentos/, /Valorização/, /Saldo no fim/]);
  // Seis meses, o mais recente primeiro (Nielsen 2: como no app).
  await expect(tabela.locator('tbody tr').first()).toContainText('1.020');

  const doze = secao.getByRole('button', { name: '12 meses' });
  const vinteQuatro = secao.getByRole('button', { name: '24 meses' });
  await expect(doze).toHaveAttribute('aria-pressed', 'true');
  await vinteQuatro.click();
  await expect(vinteQuatro).toHaveAttribute('aria-pressed', 'true');

  // #173 c): o detalhe por fundo do mês, sob demanda.
  const porFundo = tabela.getByRole('button', { name: /^Rendimentos por fundo em/ }).first();
  await porFundo.click();
  await expect(porFundo).toHaveAttribute('aria-expanded', 'true');
  await expect(secao.locator('.linha-por-fii')).toContainText('sem fundo identificado');

  // Nada passa da largura da tela (WCAG 1.4.10), também no celular.
  const larguras = await page.evaluate(() => ({ pagina: document.documentElement.scrollWidth, janela: window.innerWidth }));
  expect(larguras.pagina).toBeLessThanOrEqual(larguras.janela);
  expect(problemas.lista).toEqual([]);
});
