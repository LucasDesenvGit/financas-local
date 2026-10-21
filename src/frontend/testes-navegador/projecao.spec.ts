import { expect, test } from './base';

/**
 * #257: na tabela "Mês a mês" da Projeção, valor negativo largo fica numa linha só: o "−" não quebra sozinho em cima do
 * número (no celular a coluna é estreita).
 */
test('Projeção: valor negativo não quebra a linha entre o sinal e o número', async ({ page }) => {
  await page.goto('/projecao');
  const celulas = page.locator('.tabela td.valor').filter({ hasText: '−' });
  await expect(celulas.first()).toBeVisible();
  const linhasPorCelula = await celulas.evaluateAll((tds) => tds.map((td) => {
    // Só o texto do valor (o primeiro nó de texto com "R$"), sem o "em parcelas" que pode vir embaixo.
    const no = [...td.childNodes].find((n) => n.nodeType === Node.TEXT_NODE && n.textContent!.includes('R$'))!;
    const intervalo = document.createRange();
    intervalo.selectNodeContents(no);
    return new Set([...intervalo.getClientRects()].map((r) => Math.round(r.top))).size;
  }));
  expect(linhasPorCelula.length).toBeGreaterThan(0);
  expect(linhasPorCelula.every((n) => n === 1)).toBe(true);
});

test('#312: "Mês a mês" sem rolagem lateral num notebook (1366 px): o "Saldo previsto" aparece inteiro', async ({ page, problemas }, info) => {
  test.skip(info.project.name !== 'computador', 'no celular a tabela rola para o lado de propósito (tabela-rolagem)');
  await page.setViewportSize({ width: 1366, height: 900 });
  await page.goto('/projecao');
  const bloco = page.locator('section').filter({ has: page.getByRole('heading', { name: 'Mês a mês' }) });
  const rolagem = bloco.locator('.tabela-rolagem');
  await expect(rolagem).toBeVisible();
  const medidas = await rolagem.evaluate((el) => ({ visivel: el.clientWidth, conteudo: el.scrollWidth }));
  expect(medidas.conteudo, 'a tabela cabe sem rolar para o lado').toBeLessThanOrEqual(medidas.visivel + 1);
  // "em parcelas" numa linha só: a célula de despesas não vira três linhas.
  const parcelas = bloco.locator('td small', { hasText: 'em parcelas' }).first();
  const altura = await parcelas.evaluate((el) => el.getBoundingClientRect().height);
  const linha = await parcelas.evaluate((el) => parseFloat(getComputedStyle(el).lineHeight) || el.getBoundingClientRect().height);
  expect(altura).toBeLessThan(linha * 1.5);
  expect(problemas.lista).toEqual([]);
});

test('#314: "Gasto médio por categoria" na largura toda, em duas colunas no computador e uma no celular', async ({ page, problemas }, info) => {
  await page.goto('/projecao');
  const mesAMes = page.locator('section').filter({ has: page.getByRole('heading', { name: 'Mês a mês' }) });
  const categorias = page.locator('section').filter({ has: page.getByRole('heading', { name: 'Gasto médio por categoria' }) });
  await expect(categorias).toBeVisible();
  // Mesma largura da tabela "Mês a mês" (a grade inteira), sem sobrar a metade da direita.
  const larguraTabela = (await mesAMes.boundingBox())!.width;
  const larguraCategorias = (await categorias.boundingBox())!.width;
  expect(Math.abs(larguraTabela - larguraCategorias)).toBeLessThan(2);
  const colunas = await categorias.locator('.lista-categorias-projecao').evaluate((ul) => getComputedStyle(ul).columnCount);
  expect(colunas).toBe(info.project.name === 'celular' ? '1' : '2');
  // A porcentagem diz o que compara.
  await expect(categorias.locator('.variacao').first()).toContainText('último mês');
  expect(problemas.lista).toEqual([]);
});
