import type { Page } from '@playwright/test';
import { expect, test } from './base';

/** Um plano do assistente com 10 anos de caixa igual (para a API falsa, que não tem histórico de caixa). */
async function comPlano(page: Page) {
  const hoje = new Date();
  const meses = Array.from({ length: 120 }, (_, k) => {
    const d = new Date(hoje.getFullYear(), hoje.getMonth() + k, 1);
    return { ano: d.getFullYear(), mes: d.getMonth() + 1, receitas: 10000, despesas: 4000, faturas: 2500, parcelas: 0, sobra: 3500 };
  });
  await page.route(/\/api\/metas\/plano$/, (r) => r.fulfill({ json: {
    disponivel: true, mesesConsiderados: 6, ano: hoje.getFullYear(), mes: hoje.getMonth() + 1, margemPercentual: 10,
    saldoContas: 9000, faturaAPagar: 0, saldoFimDoMes: 9000, melhorMesRecente: 5000, meses
  } }));
}

/**
 * #335: no card da meta, "Consigo guardar outro valor" fica na linha de "Este mês: guarde"; sem a lista "Depois:", o
 * plano mês a mês já vem aberto, com 5 meses e "Mostrar mais". Sem erro de console nem rolagem para o lado.
 */
test('Metas: o plano mês a mês aberto com 5 meses, e "Consigo guardar" na linha do valor do mês', async ({ page, problemas }) => {
  await comPlano(page);
  await page.goto('/metas');
  const viagem = page.locator('.cartao-meta').filter({ hasText: 'Viagem' });
  const linha = viagem.locator('.recomendado');
  await expect(linha).toContainText('Este mês: guarde');
  const [texto, botao] = [(await linha.locator('span').first().boundingBox())!, (await linha.getByRole('button', { name: 'Consigo guardar outro valor' }).boundingBox())!];
  if (page.viewportSize()!.width > 900) expect(Math.abs(texto.y - botao.y)).toBeLessThan(6);
  await expect(viagem.locator('.degraus')).toHaveCount(0);
  // A seta da barra fica na linha do nome, também no celular (não sozinha numa linha).
  const [nome, seta] = [(await viagem.locator('.meta-nome').boundingBox())!, (await viagem.locator('.meta-seta').boundingBox())!];
  expect(Math.abs((nome.y + nome.height / 2) - (seta.y + seta.height / 2))).toBeLessThan(12);
  await expect(viagem.getByRole('button', { name: /Plano mês a mês/ })).toHaveAttribute('aria-expanded', 'true');
  await expect(viagem.locator('.tabela-plano tbody tr')).toHaveCount(5);
  await viagem.getByRole('button', { name: 'Mostrar mais 5 meses' }).click();
  await expect(viagem.locator('.tabela-plano tbody tr')).toHaveCount(10);
  const sobra = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(sobra).toBeLessThanOrEqual(1);
  expect(problemas.lista).toEqual([]);
});
