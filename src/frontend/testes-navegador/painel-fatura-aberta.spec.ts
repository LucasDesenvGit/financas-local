import type { Page } from '@playwright/test';
import { expect, test } from './base';

/**
 * Quantos cards há em cada linha da grade, pelo topo de cada um no layout (offsetTop). O getBoundingClientRect conta a
 * animação de entrada ("surgir", translateY de 4 px): o card que acabou de entrar parecia estar em outra linha.
 */
async function cardsPorLinha(page: Page, grade: string): Promise<number[]> {
  const topos = await page.locator(`${grade} .kpi`).evaluateAll((els) => els.map((e) => (e as HTMLElement).offsetTop));
  const linhas = new Map<number, number>();
  for (const y of topos) linhas.set(y, (linhas.get(y) ?? 0) + 1);
  return [...linhas.values()];
}

/**
 * #322: o interruptor "Fatura no Painel" de um cartão, em Contas e cartões, soma a fatura em aberto dele no card
 * "Fatura em aberto", no resumo de Contas e no Painel ("Situação hoje"); "ver detalhes" abre a fatura de cada cartão.
 * Sem erro de console nem bloqueio de CSP, sem alargar a página.
 */
test('Contas › "Fatura no Painel" põe a fatura em aberto no card de total, em Contas e no Painel', async ({ page, problemas }) => {
  await page.goto('/contas');
  const interruptor = page.getByRole('switch', { name: 'Fatura no Painel (Cartão Crédito)' });
  await expect(interruptor).not.toBeChecked();
  const caixa = (await interruptor.locator('xpath=..').boundingBox())!;
  expect(caixa.height).toBeGreaterThanOrEqual(23.5);
  await interruptor.click();
  await expect(interruptor).toBeChecked();
  await expect(page.locator('.kpis-contas .kpi-fatura-aberta')).toContainText('Fatura em aberto');
  if ((page.viewportSize()?.width ?? 0) > 760) expect(await cardsPorLinha(page, '.kpis-contas')).not.toContain(1);

  await page.goto('/painel');
  const card = page.locator('.kpi-fatura-aberta');
  await expect(card).toHaveCount(1);
  const detalhes = card.getByRole('button', { name: 'ver detalhes' });
  await expect(detalhes).toHaveAttribute('aria-expanded', 'false');
  expect((await detalhes.boundingBox())!.height).toBeGreaterThanOrEqual(23.5);
  await detalhes.click();
  await expect(card.getByRole('button', { name: 'esconder detalhes' })).toHaveAttribute('aria-expanded', 'true');
  await expect(card.getByRole('link', { name: 'Ver as faturas de Cartão Crédito' })).toHaveAttribute('href', '/contas?faturas=3');
  // Nenhuma linha com um card sozinho (no celular, um por linha é o normal).
  if ((page.viewportSize()?.width ?? 0) > 760) {
    expect(await cardsPorLinha(page, '.kpis.kpis-equilibrados')).not.toContain(1);
  }
  const sobra = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(sobra).toBeLessThanOrEqual(1);
  expect(problemas.lista).toEqual([]);
});
