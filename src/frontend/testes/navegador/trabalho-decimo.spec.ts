import { mkdirSync } from 'node:fs';
import { join } from 'node:path';
import { expect, test } from './base';

/** #214 (ADR 0065): Trabalho › 13º salário (API falsa, dados fictícios): as parcelas, "como calculei" e a 1ª informada. */
const trabalho = { id: 'trabalho', nome: 'Trabalho', descricao: 'd', acessaInternet: false, destinos: [], ligado: true };
const emprego = { id: 1, empregador: 'Empresa Fictícia Ltda', cnpj: null, admissao: '2024-03-11', saida: null, regime: 1,
  jornadaMensalHoras: 220, dependentes: 0, contaId: null, contaNome: null, salarios: [{ id: 1, inicio: '2024-03-01', salarioBruto: 6000 }] };
const decimo = { ano: 2026, avos: 12, salarioBase: 6000, mediaVariaveis: 0, total: 6000, liquido: 4973.39, saida: null,
  primeira: { valor: 3000, data: '2026-11-30', informada: false }, segunda: { valor: 1973.39, data: '2026-12-20', informada: false },
  inss: { base: 6000, valor: 641.51, noTeto: false, parcelas: [], fonte: '' },
  irrf: { rendimento: 6000, deducoesLegais: 641.51, simplificado: 0, deducaoUsada: 'legal', base: 5358.49, aliquota: 0.275,
    parcelaADeduzir: 908.73, impostoDaTabela: 564.85, reducaoLei15270: 179.75, valor: 385.10, fonte: '' } };

test('Trabalho › 13º salário: as parcelas e a 1ª que veio nas férias', async ({ page }) => {
  const json = (corpo: unknown) => ({ status: 200, contentType: 'application/json', body: JSON.stringify(corpo) });
  await page.route('**/api/modulos', (r) => r.fulfill(json([trabalho])));
  await page.route('**/api/trabalho/empregos', (r) => r.fulfill(json([emprego])));
  const pedidos: string[] = [];
  await page.route('**/api/trabalho/empregos/1/decimo-terceiro/*', (r) => {
    pedidos.push(`${r.request().method()} ${new URL(r.request().url()).pathname} ${r.request().postData() ?? ''}`.trim());
    if (r.request().method() === 'PUT') {
      return r.fulfill(json({ ...decimo, primeira: { valor: 2800, data: '2026-07-10', informada: true },
        segunda: { valor: 2173.39, data: '2026-12-20', informada: false } }));
    }
    return r.fulfill(json(decimo));
  });

  await page.goto('/trabalho/decimo-terceiro?ano=2026');
  await expect(page.getByRole('heading', { name: '13º salário', level: 1 })).toBeVisible();
  const parcelas = page.locator('section', { has: page.getByRole('heading', { name: 'Empresa Fictícia Ltda' }) });
  await expect(parcelas).toContainText('1ª parcela, até 30/11');
  await expect(parcelas).toContainText('R$ 1.973,39');
  await expect(parcelas).toContainText('R$ 4.973,39');
  await page.getByText('Como calculei').click();
  await expect(parcelas).toContainText('12/12 avos');

  const pasta = process.env['FOTOS_PASTA'];
  if (pasta) {
    mkdirSync(pasta, { recursive: true });
    await page.screenshot({ path: join(pasta, `trabalho-decimo-${test.info().project.name}.png`), fullPage: true });
  }

  const form = page.locator('section', { has: page.getByRole('heading', { name: 'A 1ª parcela que veio' }) });
  await form.getByLabel('Recebida em').fill('10/07/2026');
  await form.getByLabel('Valor recebido').fill('2800');
  await form.getByRole('button', { name: 'Gravar' }).click();
  await expect(page.getByText('1ª parcela gravada: a 2ª foi recalculada.')).toBeVisible();
  await expect(parcelas).toContainText('1ª parcela, recebida em 10/07');
  expect(pedidos.at(-1)).toBe('PUT /api/trabalho/empregos/1/decimo-terceiro/2026 {"primeiraParcelaEm":"2026-07-10","primeiraParcelaValor":2800}');

  await page.getByRole('button', { name: 'Ano anterior' }).click();
  await expect(page).toHaveURL(/ano=2025/);
  expect(pedidos.at(-1)).toBe('GET /api/trabalho/empregos/1/decimo-terceiro/2025');
});
