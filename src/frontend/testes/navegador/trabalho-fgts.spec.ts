import { mkdirSync } from 'node:fs';
import { join } from 'node:path';
import { expect, test } from './base';

/** #216 (ADR 0066): Trabalho › FGTS (API falsa, dados fictícios): o saldo, a conferência com o app e lançar um saque. */
const trabalho = { id: 'trabalho', nome: 'Trabalho', descricao: 'd', acessaInternet: false, destinos: [], ligado: true };
const emprego = { id: 1, empregador: 'Empresa Fictícia Ltda', cnpj: null, admissao: '2026-01-10', saida: null, regime: 1,
  jornadaMensalHoras: 220, dependentes: 0, contaId: null, contaNome: null, salarios: [{ id: 1, inicio: '2026-01-01', salarioBruto: 3000 }] };
const meses = ['01', '02', '03', '04', '05', '06', '07', '08', '09', '10', '11'].map((m, i) => ({
  mes: `2026-${m}-01`, correcao: i ? 1.5 : 0, deposito: i ? 240 : 168, distribuicao: 0, saques: 0, saldo: 168 + i * 241.5, conferido: m === '11' }));
const fgts = { saldo: 3000, ate: '2026-11-01', meses,
  conferencias: [{ data: '2026-11-10', informado: 3000, estimado: 2598.99, diferenca: 401.01 }],
  lancamentos: [{ id: 1, data: '2026-11-10', tipo: 1, valor: 3000, motivo: null }] };

test('Trabalho › FGTS: o saldo, a conferência e um saque', async ({ page }) => {
  const json = (corpo: unknown) => ({ status: 200, contentType: 'application/json', body: JSON.stringify(corpo) });
  const pedidos: string[] = [];
  await page.route('**/api/modulos', (r) => r.fulfill(json([trabalho])));
  await page.route('**/api/trabalho/empregos', (r) => r.fulfill(json([emprego])));
  await page.route('**/api/trabalho/empregos/1/fgts', (r) => r.fulfill(json(fgts)));
  await page.route('**/api/trabalho/empregos/1/fgts/lancamentos', (r) => {
    pedidos.push(r.request().postData() ?? '');
    return r.fulfill(json({ ...fgts, saldo: 2500, lancamentos: [...fgts.lancamentos,
      { id: 2, data: '2026-11-11', tipo: 2, valor: 500, motivo: 2 }] }));
  });

  await page.goto('/trabalho/fgts');
  const saldo = page.locator('section', { has: page.getByRole('heading', { name: 'Empresa Fictícia Ltda' }) });
  await expect(saldo.locator('.saldo')).toHaveText('R$ 3.000,00');
  await expect(saldo).toContainText('a mais R$ 401,01');
  await expect(page.locator('table tbody tr')).toHaveCount(11);
  await expect(page.locator('table tbody tr').first()).toContainText('conferido');

  const pasta = process.env['FOTOS_PASTA'];
  if (pasta) {
    mkdirSync(pasta, { recursive: true });
    await page.screenshot({ path: join(pasta, `trabalho-fgts-${test.info().project.name}.png`), fullPage: true });
  }

  const form = page.locator('section', { has: page.getByRole('heading', { name: 'Lançar' }) });
  await form.getByLabel('O quê').selectOption({ label: 'Saque' });
  await form.getByLabel('Motivo do saque').selectOption({ label: 'Saque-aniversário' });
  await expect(form).toContainText('o saldo fica');
  await form.getByLabel('Data', { exact: true }).fill('11/11/2026');
  await form.getByLabel('Valor').fill('500');
  await form.getByRole('button', { name: 'Lançar' }).click();
  await expect(page.getByText('Lançado: o saldo foi recalculado.')).toBeVisible();
  expect(JSON.parse(pedidos[0])).toEqual({ data: '2026-11-11', tipo: 2, valor: 500, motivo: 2 });
  await expect(saldo).toContainText('Saque (saque-aniversário)');
});
