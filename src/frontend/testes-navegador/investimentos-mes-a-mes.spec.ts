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

/**
 * #173 d): detalhe por navegação (ADR 0018). "ver lançamentos" nos rendimentos do mês abre Transações com o mês e só os
 * lançamentos que formam o número; o filtro aparece num chip removível (Nielsen 1 e 3) e o voltar leva de novo aos FIIs.
 */
test('Investimentos › FIIs › Mês a mês: "ver lançamentos" abre Transações filtrada e o voltar funciona', async ({ page, problemas }) => {
  await page.goto('/investimentos?aba=fii');
  const tabela = page.locator('section.mes-a-mes').getByRole('table');
  // O mês mais recente (primeira linha) tem dois créditos de rendimento.
  const pedido = page.waitForRequest((r) => r.url().includes('/api/transacoes?') && r.url().includes('ids=201') && r.url().includes('ids=202'));
  await tabela.getByRole('link', { name: /^Ver em Transações os rendimentos de FIIs de/ }).first().click();
  await pedido;

  // O mês é o atual (o período padrão de Transações): na URL, só os ids e a origem.
  await expect(page).toHaveURL(/\/transacoes\?.*ids=201(,|%2C)202/);
  await expect(page).toHaveURL(/de=fii-rendimentos/);
  const filtros = page.getByRole('region', { name: 'Filtros ativos' });
  await expect(filtros).toContainText('Rendimentos de FIIs: 2 lançamentos');

  // O ✕ tira só esse filtro (o mês continua), e a URL acompanha.
  await filtros.getByRole('button', { name: 'Remover filtro: Rendimentos de FIIs: 2 lançamentos' }).click();
  await expect(page).not.toHaveURL(/ids=/);
  // Era o único filtro: a faixa "Filtrando:" some junto (Nielsen 8).
  await expect(filtros).toHaveCount(0);

  await page.goBack();
  await expect(page).toHaveURL(/\/investimentos\?aba=fii/);
  // O mês dos aportes também tem o link (compras e vendas daquele mês).
  await expect(page.locator('section.mes-a-mes').getByRole('link', { name: /^Ver em Transações as compras e vendas de FIIs de/ }))
    .toHaveCount(1);
  expect(problemas.lista).toEqual([]);
});
