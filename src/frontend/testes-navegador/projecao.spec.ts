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
