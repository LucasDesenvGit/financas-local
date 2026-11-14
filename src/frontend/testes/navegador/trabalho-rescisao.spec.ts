import { mkdirSync } from 'node:fs';
import { join } from 'node:path';
import { expect, test } from './base';

/** #215 (ADR 0065): Trabalho › Rescisão (API falsa, dados fictícios): simular, comparar os tipos e registrar. */
const trabalho = { id: 'trabalho', nome: 'Trabalho', descricao: 'd', acessaInternet: false, destinos: [], ligado: true };
const emprego = { id: 1, empregador: 'Empresa Fictícia Ltda', cnpj: null, admissao: '2024-03-11', saida: null, regime: 1,
  jornadaMensalHoras: 220, dependentes: 0, contaId: null, contaNome: null, salarios: [{ id: 1, inicio: '2024-03-01', salarioBruto: 6000 }] };
const calculo = { tipo: 1, nomeDoTipo: 'Dispensa sem justa causa', aviso: 2, saida: '2026-12-15', dataProjetada: '2027-01-20', diasDeAviso: 36,
  remuneracao: 6000, totalVerbas: 30866.67, totalDescontos: 4275.21, liquido: 26591.46, seguroDesemprego: true,
  verbas: [{ descricao: 'Saldo de salário', referencia: '15 dias', valor: 3000 }, { descricao: 'Aviso prévio indenizado', referencia: '36 dias', valor: 7200 },
    { descricao: '13º proporcional', referencia: '12/12', valor: 6000 }],
  descontos: [{ descricao: 'INSS', referencia: 'sobre o saldo de salário', valor: 248.6 }],
  fgts: { saldo: 10000, depositoRescisao: 1296, percentualMulta: 40, multa: 4518.4, percentualSaque: 100, sacavel: 15814.4 } };
const comparacao = [
  { tipo: 1, nome: 'Dispensa sem justa causa', aviso: 2, liquido: 26591.46, multa: 4518.4, sacavel: 15814.4, seguroDesemprego: true },
  { tipo: 2, nome: 'Pedido de demissão', aviso: 1, liquido: 19391.46, multa: 0, sacavel: 0, seguroDesemprego: false },
  { tipo: 3, nome: 'Acordo (art. 484-A)', aviso: 2, liquido: 22991.46, multa: 2201.6, sacavel: 11008, seguroDesemprego: false },
  { tipo: 4, nome: 'Justa causa', aviso: 0, liquido: 10751.4, multa: 0, sacavel: 0, seguroDesemprego: false },
  { tipo: 5, nome: 'Fim do contrato de experiência', aviso: 0, liquido: 19391.46, multa: 0, sacavel: 10720, seguroDesemprego: false }];

test('Trabalho › Rescisão: simular, comparar e registrar', async ({ page }) => {
  const json = (corpo: unknown, status = 200) => ({ status, contentType: 'application/json', body: JSON.stringify(corpo) });
  const pedidos: string[] = [];
  await page.route('**/api/modulos', (r) => r.fulfill(json([trabalho])));
  await page.route('**/api/trabalho/empregos', (r) => r.fulfill(json([emprego])));
  await page.route('**/api/trabalho/empregos/1/rescisao', (r) => {
    if (r.request().method() === 'POST') {
      pedidos.push(r.request().postData() ?? '');
      return r.fulfill(json(calculo, 201));
    }
    return r.fulfill(json({ registrada: null }));
  });
  await page.route('**/api/trabalho/empregos/1/rescisao/simular**', (r) => {
    pedidos.push(new URL(r.request().url()).search);
    return r.fulfill(json(calculo));
  });
  await page.route('**/api/trabalho/empregos/1/rescisao/comparar**', (r) => r.fulfill(json(comparacao)));

  await page.goto('/trabalho/rescisao');
  const simular = page.locator('section', { has: page.getByRole('heading', { name: 'Simular' }) });
  await simular.getByLabel('Data de saída').fill('15/12/2026');
  await simular.getByRole('button', { name: 'Comparar os tipos' }).click();
  await expect(simular.locator('table tbody tr')).toHaveCount(5);
  await expect(simular.locator('table tbody tr').first()).toContainText('e o seguro-desemprego');
  await simular.getByRole('button', { name: 'Simular' }).click();
  expect(pedidos.at(-1)).toBe('?tipo=1&aviso=2&saida=2026-12-15');
  const conta = page.locator('section', { has: page.getByRole('heading', { name: 'A conta' }) });
  await expect(conta).toContainText('R$ 26.591,46');
  await expect(conta).toContainText('Multa de 40% sobre o saldo');
  await expect(conta).toContainText('Para sacar: R$ 15.814,40.');
  await expect(conta).toContainText('20/01/2027');

  // Justa causa: só "sem aviso".
  await simular.getByLabel('Tipo').selectOption({ label: 'Justa causa' });
  await expect(simular.getByLabel('Aviso prévio').locator('option')).toHaveText(['Sem aviso prévio']);
  await simular.getByLabel('Tipo').selectOption({ label: 'Dispensa sem justa causa' });
  await simular.getByRole('button', { name: 'Simular' }).click();

  const pasta = process.env['FOTOS_PASTA'];
  if (pasta) {
    mkdirSync(pasta, { recursive: true });
    await page.screenshot({ path: join(pasta, `trabalho-rescisao-${test.info().project.name}.png`), fullPage: true });
  }

  await conta.getByRole('button', { name: 'Registrar esta rescisão' }).click();
  await page.getByRole('alertdialog').getByRole('button', { name: 'Registrar' }).click();
  await expect(page.getByText('Rescisão registrada: o emprego foi fechado.')).toBeVisible();
  expect(JSON.parse(pedidos.find((p) => p.startsWith('{'))!)).toEqual({ tipo: 1, aviso: 2, saida: '2026-12-15' });
  await expect(page.getByRole('heading', { name: 'Rescisão registrada' })).toBeVisible();
});
