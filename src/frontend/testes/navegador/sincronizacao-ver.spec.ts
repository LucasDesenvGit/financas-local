import { Page } from '@playwright/test';
import { expect, test } from './base';

/**
 * #688: o aviso "Sincronizado: X nova(s), Y atualizada(s)." da barra de cima ganha "Ver em Transações", que abre só
 * esses lançamentos, com o chip da origem (ADR 0018). Vale também com Transações já aberta: a tela relê a URL, inclusive
 * no voltar do navegador.
 */
const hoje = new Date();
const dia = (d: number) => `${hoje.getFullYear()}-${String(hoje.getMonth() + 1).padStart(2, '0')}-${String(d).padStart(2, '0')}`;

async function sincronizarPelaBarra(page: Page): Promise<void> {
  await page.route('**/api/open-finance/sincronizar', (r) => r.fulfill({ json: {
    sucesso: true, transacoesNovas: 1, transacoesAtualizadas: 1, mensagem: 'Sincronização concluída.',
    lancamentos: { ids: [1, 2], inicio: dia(1), fim: dia(2) }
  } }));
  await page.getByRole('button', { name: /^Sincronizar bancos/ }).click();
}

for (const origem of ['/painel', '/transacoes']) {
  test(`Sincronizar pela barra em ${origem}: "Ver em Transações" abre só as novas e atualizadas`, async ({ page, api, problemas }) => {
    await page.goto(origem);
    await sincronizarPelaBarra(page);
    const aviso = page.getByRole('status').filter({ hasText: 'Sincronizado: 1 nova(s), 1 atualizada(s).' });
    const ver = aviso.getByRole('button', { name: 'Ver em Transações' });
    // WCAG 2.5.8: alvo de 24 px (folga de 0,5).
    expect((await ver.boundingBox())!.height).toBeGreaterThanOrEqual(23.5);
    await ver.click();
    await expect(page).toHaveURL(`/transacoes?inicio=${dia(1)}&fim=${dia(2)}&ids=1,2&de=sincronizacao`);
    const filtros = page.getByRole('region', { name: 'Filtros ativos' });
    await expect(filtros).toContainText('Novas e atualizadas na sincronização: 2 lançamentos');
    // A lista pedida à API é a desses lançamentos, não só a URL.
    await expect.poll(() => api.chamadasComQuery.some((c) => c.startsWith('GET /api/transacoes?') && c.includes('ids=1&ids=2'))).toBe(true);
    expect(problemas.lista).toEqual([]);
  });
}

test('Transações: o voltar do navegador depois do "Ver em Transações" volta à lista de antes, sem o filtro da sincronização', async ({ page, problemas }) => {
  await page.goto('/transacoes');
  await sincronizarPelaBarra(page);
  await page.getByRole('status').getByRole('button', { name: 'Ver em Transações' }).click();
  const filtros = page.getByRole('region', { name: 'Filtros ativos' });
  await expect(filtros).toContainText('Novas e atualizadas na sincronização');
  await page.goBack();
  await expect(page).toHaveURL(/\/transacoes$/);
  await expect(page.getByText('Novas e atualizadas na sincronização')).toHaveCount(0);
  expect(problemas.lista).toEqual([]);
});
