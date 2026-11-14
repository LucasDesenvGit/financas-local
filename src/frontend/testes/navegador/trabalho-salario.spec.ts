import { mkdirSync } from 'node:fs';
import { join } from 'node:path';
import { Page } from '@playwright/test';
import { expect, test } from './base';

/**
 * #210 (ADRs 0064 e 0065): o módulo Trabalho (API falsa, dados fictícios). Desligado, o grupo não aparece no menu e a
 * rota mostra o aviso com "Ligar Trabalho"; ligado, a tela do Salário mostra o emprego, a estimativa do mês com "como
 * calculei" e o histórico, e grava o aumento.
 */
const trabalho = { id: 'trabalho', nome: 'Trabalho', descricao: 'O salário pelo bruto, com o INSS e o IRRF estimados.',
  acessaInternet: false, destinos: [], ligado: false };
const emprego = { id: 1, empregador: 'Empresa Fictícia Ltda', cnpj: '12345678000190', admissao: '2024-03-11', saida: null,
  regime: 1, jornadaMensalHoras: 220, dependentes: 0, contaId: null, contaNome: null,
  salarios: [{ id: 2, inicio: '2026-06-01', salarioBruto: 6000 }, { id: 1, inicio: '2024-03-01', salarioBruto: 5000 }] };
const estimativa = { mes: '2026-11-01', salarioBruto: 6000, vigenciaDe: '2026-06-01', semEncargos: false, liquido: 4973.39,
  inss: { base: 6000, valor: 641.51, noTeto: false, fonte: 'Portaria Interministerial MPS/MF nº 13, de 09/01/2026', parcelas: [
    { de: 0, ate: 1621, aliquota: 0.075, valor: 121.58 }, { de: 1621, ate: 2902.84, aliquota: 0.09, valor: 115.37 },
    { de: 2902.84, ate: 4354.27, aliquota: 0.12, valor: 174.17 }, { de: 4354.27, ate: 6000, aliquota: 0.14, valor: 230.40 }] },
  irrf: { rendimento: 6000, deducoesLegais: 641.51, simplificado: 607.2, deducaoUsada: 'legal', base: 5358.49, aliquota: 0.275,
    parcelaADeduzir: 908.73, impostoDaTabela: 564.85, reducaoLei15270: 179.75, valor: 385.10,
    fonte: 'Lei nº 15.191/2025 (tabela mensal) e Lei nº 15.270/2025 (redução até R$ 7.350,00)' } };

async function json(page: Page, padrao: string, corpo: unknown, pedidos?: string[]) {
  await page.route(padrao, (rota) => {
    pedidos?.push(`${rota.request().method()} ${new URL(rota.request().url()).pathname} ${rota.request().postData() ?? ''}`.trim());
    return rota.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(corpo) });
  });
}

test('Trabalho desligado: fora do menu, o aviso na rota e ligar ali mesmo', async ({ page }) => {
  let ligado = false;
  await page.route('**/api/modulos', (rota) => rota.fulfill({ status: 200, contentType: 'application/json',
    body: JSON.stringify([{ ...trabalho, ligado }]) }));
  await page.route('**/api/modulos/trabalho', (rota) => {
    ligado = true;
    return rota.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ ...trabalho, ligado: true }) });
  });
  const pedidos: string[] = [];
  await json(page, '**/api/trabalho/empregos', [], pedidos);

  await page.goto('/trabalho/salario');
  await expect(page.getByRole('heading', { name: 'Este módulo está desligado' })).toBeVisible();
  await expect(page.getByText('Ele não acessa a internet')).toBeVisible();
  await expect(page.locator('nav.menu')).not.toContainText('Trabalho');
  expect(pedidos, 'desligado, a tela não chama a API do módulo').toEqual([]);

  await page.getByRole('button', { name: 'Ligar Trabalho' }).click();
  // A tela aparece sem recarregar: sem emprego, já abre o cadastro.
  await expect(page.getByRole('heading', { name: 'Cadastrar emprego' })).toBeVisible();
  await expect(page.locator('nav.menu')).toContainText('Trabalho');
  expect(pedidos).toEqual(['GET /api/trabalho/empregos']);
});

test('Trabalho › Salário: o mês, como calculei e o aumento', async ({ page }) => {
  await json(page, '**/api/modulos', [{ ...trabalho, ligado: true }]);
  const pedidos: string[] = [];
  await page.route('**/api/trabalho/empregos', (rota) => rota.fulfill({ status: 200, contentType: 'application/json',
    body: JSON.stringify([emprego]) }));
  await json(page, '**/api/trabalho/empregos/1/estimativa**', estimativa);
  await json(page, '**/api/trabalho/empregos/1/salarios', { ...emprego, salarios: [{ id: 3, inicio: '2026-12-01', salarioBruto: 6500 },
    ...emprego.salarios] }, pedidos);

  await page.goto('/trabalho/salario');
  await expect(page.getByRole('heading', { name: 'Empresa Fictícia Ltda' })).toBeVisible();
  await expect(page.getByText('12.345.678/0001-90')).toBeVisible();
  const mes = page.locator('section', { has: page.getByRole('heading', { name: 'O mês' }) });
  await expect(mes).toContainText('R$ 4.973,39');
  await expect(mes).toContainText('641,51');
  await page.getByText('Como calculei').click();
  await expect(mes).toContainText('7,5% de R$ 0,00 até R$ 1.621,00: R$ 121,58');
  await expect(mes).toContainText('Redução da Lei 15.270/2025');
  await expect(mes).toContainText('Portaria Interministerial MPS/MF nº 13');

  const historico = page.locator('section', { has: page.getByRole('heading', { name: 'Histórico de salários' }) });
  await expect(historico.locator('tbody tr')).toHaveCount(2);
  await expect(historico).toContainText('+20,0%');

  const pasta = process.env['FOTOS_PASTA'];
  if (pasta) {
    mkdirSync(pasta, { recursive: true });
    await page.screenshot({ path: join(pasta, `trabalho-salario-${test.info().project.name}.png`), fullPage: true });
  }

  await historico.getByLabel('Aumento a partir de (o mês)').fill('01/12/2026');
  await historico.getByLabel('Salário bruto novo').fill('6500');
  await historico.getByRole('button', { name: 'Gravar aumento' }).click();
  await expect(page.getByText('Salário gravado a partir de dezembro de 2026.')).toBeVisible();
  await expect(historico.locator('tbody tr')).toHaveCount(3);
  expect(pedidos).toEqual(['POST /api/trabalho/empregos/1/salarios {"inicio":"2026-12-01","salarioBruto":6500}']);
});
