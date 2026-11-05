import { expect, test } from './base';

/**
 * #330: sem posição aberta (sem "Rentabilidade por ativo"), a distribuição em "Mais análises" ocupa a largura toda, com a
 * rosca e a lista lado a lado no computador, em vez de meia largura com o canto vazio. No celular, um embaixo do outro.
 */
test('Investimentos › Mais análises: a distribuição sozinha usa a largura toda', async ({ page, problemas }) => {
  await page.goto('/investimentos');
  const mais = page.getByRole('button', { name: /Mais análises/ });
  if ((await mais.getAttribute('aria-expanded')) !== 'true') await mais.click();
  const bloco = page.locator('.painel-bloco', { has: page.getByRole('heading', { name: 'Distribuição' }) });
  await expect(bloco).toBeVisible();
  await expect(bloco).toContainText('Reservas');
  const grade = (await page.locator('.grade-2.grade-sozinha').boundingBox())!;
  const caixa = (await bloco.boundingBox())!;
  // O card ocupa a grade toda (não a metade).
  expect(caixa.width).toBeGreaterThan(grade.width - 2);
  const [rosca, lista] = [(await bloco.locator('canvas').boundingBox())!, (await bloco.locator('.lista-legenda').boundingBox())!];
  if (page.viewportSize()!.width > 760) {
    // Lado a lado: a lista começa à direita da rosca, na mesma faixa de altura.
    expect(lista.x).toBeGreaterThan(rosca.x + rosca.width);
    // A lista não se estica até a borda (o valor longe do nome) e o par fica no meio do card.
    expect(lista.width).toBeLessThanOrEqual(381);
    const esquerda = rosca.x - caixa.x;
    const direita = caixa.x + caixa.width - (lista.x + lista.width);
    expect(Math.abs(esquerda - direita)).toBeLessThan(60);
  } else {
    expect(lista.y).toBeGreaterThan(rosca.y + rosca.height - 1);
  }
  const sobra = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(sobra).toBeLessThanOrEqual(1);
  expect(problemas.lista).toEqual([]);
});

/** Uma posição aberta (ação), com o mínimo que a tela e o gráfico de rentabilidade usam. */
const posicao = (id: number, ticker: string, resultado: number, selic: number) => ({
  id, ticker, nome: null, tipo: 2, moeda: 'BRL', quantidade: 10, precoMedio: 100, custo: 1000, precoAtual: 105,
  precoAtualEmReais: 105, precoAnterior: 104, variacaoDia: 0.5, valorMercado: 1050, valorizacao: 50, valorizacaoPercentual: 5,
  lucroRealizado: 0, proventosRecebidos: 30, proventos12Meses: 30, dividendYield12Meses: 9, resultadoTotal: 80,
  resultadoTotalPercentual: resultado, valorSeFosseSelic: 1060, diferencaSelic: 20, resultadoSelicPercentual: selic,
  primeiraCompra: '2025-01-10', ultimoProvento: null, precoManual: false, precoAtualizadoEm: null, fonteCotacao: 'brapi',
  semCotacao: false, segmento: null
});

test('Investimentos › Mais análises: com ativos em carteira, os dois cards lado a lado, como antes', async ({ page, problemas }) => {
  // A carteira da API falsa, com ativos abertos e três tipos na distribuição (o caso do print da issue).
  await page.goto('/investimentos');
  const base = await page.evaluate(async () => (await fetch('/api/carteira')).json());
  await page.route(/\/api\/carteira$/, (r) => r.fulfill({ json: {
    ...base, posicoes: [posicao(1, 'GARE11', 8, 5), posicao(2, 'VGIC11', 7, 5), posicao(3, 'VGHF11', -6, 5)],
    totalGeral: 44020, distribuicao: [{ nome: 'Renda fixa', valor: 17514 }, { nome: 'Fundos Imobiliários', valor: 15668 },
      { nome: 'Reservas', valor: 10840 }]
  } }));
  await page.reload();
  const mais = page.getByRole('button', { name: /Mais análises/ });
  if ((await mais.getAttribute('aria-expanded')) !== 'true') await mais.click();
  const distribuicao = page.locator('.painel-bloco', { has: page.getByRole('heading', { name: 'Distribuição' }) });
  const rentabilidade = page.locator('.painel-bloco', { has: page.getByRole('heading', { name: /Rentabilidade por ativo/ }) });
  await expect(rentabilidade).toBeVisible();
  // Nada do arranjo de "sozinha": a grade de duas colunas e a lista embaixo da rosca.
  await expect(page.locator('.grade-sozinha')).toHaveCount(0);
  await expect(distribuicao).not.toHaveClass(/distribuicao-larga/);
  const [rosca, lista] = [(await distribuicao.locator('canvas').boundingBox())!, (await distribuicao.locator('.lista-legenda').boundingBox())!];
  expect(lista.y).toBeGreaterThan(rosca.y + rosca.height - 1);
  await expect(distribuicao.locator('.lista-legenda li')).toHaveCount(3);
  if (page.viewportSize()!.width > 760) {
    const [d, r] = [(await distribuicao.boundingBox())!, (await rentabilidade.boundingBox())!];
    expect(Math.abs(d.y - r.y)).toBeLessThan(2);
    expect(Math.abs(d.width - r.width)).toBeLessThan(2);
  }
  expect(problemas.lista).toEqual([]);
});
