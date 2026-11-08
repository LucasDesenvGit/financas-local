import { expect, test } from './base';

/**
 * #384: "Meus FIIs" com o total da carteira (a API falsa tem um FII fictício): o valor ao lado do título, e a linha de
 * total no fim da tabela, com cada célula embaixo da coluna dela, sem rolagem para o lado na página, também no celular.
 */
test('Investimentos › FIIs › Meus FIIs: total ao lado do título e linha de total alinhada com as colunas', async ({ page, problemas }) => {
  await page.goto('/investimentos?aba=fii');
  const secao = page.locator('section.meus-fiis');
  await expect(secao.getByRole('heading', { name: 'Meus FIIs' })).toBeVisible();
  await expect(secao.locator('.total-topo')).toContainText('1.020,00');
  await expect(secao.locator('.total-topo')).toContainText('investido R$ 980,00');

  const total = secao.locator('tfoot tr');
  await expect(total.getByRole('rowheader')).toContainText('Total');
  await expect(total).toContainText('+4,08%');

  // Cada célula do total fica embaixo da coluna dela: a primeira ocupa as três primeiras colunas (Fundo, Segmento, Cotas).
  const caixas = await secao.locator('table').evaluate((tabela) => {
    const borda = (el: Element) => { const r = el.getBoundingClientRect(); return { esquerda: r.left, direita: r.right }; };
    return {
      cabecalho: Array.from(tabela.querySelectorAll('thead th')).map(borda),
      total: Array.from(tabela.querySelectorAll('tfoot tr > *')).map(borda)
    };
  });
  expect(caixas.cabecalho).toHaveLength(11);
  expect(caixas.total).toHaveLength(9);
  expect(Math.abs(caixas.total[0].esquerda - caixas.cabecalho[0].esquerda)).toBeLessThan(2);
  expect(Math.abs(caixas.total[0].direita - caixas.cabecalho[2].direita)).toBeLessThan(2);
  for (let i = 1; i < caixas.total.length; i++) {
    expect(Math.abs(caixas.total[i].direita - caixas.cabecalho[i + 2].direita)).toBeLessThan(2);
  }

  // A tabela rola dentro do bloco; a página não passa da largura da tela (WCAG 1.4.10).
  const larguras = await page.evaluate(() => ({ pagina: document.documentElement.scrollWidth, janela: window.innerWidth }));
  expect(larguras.pagina).toBeLessThanOrEqual(larguras.janela);
  expect(problemas.lista).toEqual([]);
});
