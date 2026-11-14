import { mkdirSync } from 'node:fs';
import { join } from 'node:path';
import { expect, test } from './base';

/**
 * #211 (ADR 0065): Trabalho › Holerites (API falsa, dados fictícios). A lista dos meses, o holerite com os lançamentos,
 * "Salvar e calcular" manda o que vale para cada tipo, e "Fechar o mês" congela (só depois de salvar).
 */
const trabalho = { id: 'trabalho', nome: 'Trabalho', descricao: 'Salário e holerite.', acessaInternet: false, destinos: [], ligado: true };
const emprego = { id: 1, empregador: 'Empresa Fictícia Ltda', cnpj: null, admissao: '2024-03-11', saida: null, regime: 1,
  jornadaMensalHoras: 220, dependentes: 0, contaId: null, contaNome: null, salarios: [{ id: 1, inicio: '2024-03-01', salarioBruto: 5500 }] };
const calculo = { mes: '2026-10-01', salarioContratual: 5500, diasTrabalhados: 30, valorHora: 25, baseInss: 6045.37, fgtsDoMes: 483.63,
  semEncargos: false, inss: null, irrf: null, totalProventos: 7045.37, totalDescontos: 1992.24, liquido: 5053.13,
  proventos: [{ tipo: 0, descricao: 'Salário', referencia: '30 dias', valor: 5500 },
    { tipo: 1, descricao: 'Horas extras', referencia: '10 h a 50%', valor: 375 },
    { tipo: 2, descricao: 'Adicional noturno', referencia: '20 h a 20%', valor: 100 },
    { tipo: 3, descricao: 'PLR', referencia: '', valor: 1000 },
    { tipo: 4, descricao: 'DSR sobre horas extras e adicional noturno', referencia: '÷ 27 dias úteis × 4 de descanso', valor: 70.37 }],
  descontos: [{ tipo: 20, descricao: 'INSS', referencia: '', valor: 647.87 }, { tipo: 21, descricao: 'IRRF', referencia: '', valor: 264.37 },
    { tipo: 10, descricao: 'Vale-transporte', referencia: '6% do salário (o limite)', valor: 330 },
    { tipo: 12, descricao: 'Plano de saúde', referencia: '', valor: 250 },
    { tipo: 13, descricao: 'Pensão alimentícia', referencia: '', valor: 500 }] };
const lancamentos = [
  { id: 1, tipo: 1, descricao: '', quantidade: 10, percentual: 50, valor: null, incideInss: true, incideIrrf: true, deduzIrrf: false, recorrente: false },
  { id: 2, tipo: 12, descricao: '', quantidade: null, percentual: null, valor: 250, incideInss: false, incideIrrf: false, deduzIrrf: false, recorrente: true }];
const holerite = { id: 5, empregoId: 1, mes: '2026-10-01', situacao: 1, diasUteis: 27, diasDescanso: 4, fechadoEm: null,
  lancamentos, calculo, aviso: null };

test('Trabalho › Holerites: o mês, os lançamentos e fechar', async ({ page }) => {
  const pedidos: string[] = [];
  const responder = (corpo: unknown) => ({ status: 200, contentType: 'application/json', body: JSON.stringify(corpo) });
  await page.route('**/api/modulos', (r) => r.fulfill(responder([trabalho])));
  await page.route('**/api/trabalho/empregos', (r) => r.fulfill(responder([emprego])));
  await page.route('**/api/trabalho/empregos/1/holerites', (r) => r.fulfill(responder([
    { id: 5, mes: '2026-10-01', situacao: 1, totalProventos: 7045.37, totalDescontos: 1992.24, liquido: 5053.13 },
    { id: 4, mes: '2026-09-01', situacao: 2, totalProventos: 5500, totalDescontos: 761.98, liquido: 4738.02 }])));
  await page.route('**/api/trabalho/holerites/5', (r) => {
    pedidos.push(`${r.request().method()} ${r.request().postData() ?? ''}`.trim());
    return r.fulfill(responder(holerite));
  });
  const credito = { id: 31, data: '2026-11-05', descricao: 'CREDITO FOLHA', contaId: 1, conta: 'Conta salário', valor: 5053.13, livre: 5053.13 };
  await page.route('**/api/trabalho/holerites/5/recebimentos', (r) => {
    if (r.request().method() === 'POST') {
      pedidos.push(`ligar ${r.request().postData()}`);
      return r.fulfill(responder({ liquido: 5053.13, recebido: 5053.13, diferenca: 0, sugestoes: [], outros: [],
        recebimentos: [{ ...credito, livre: 0, valorLigado: 5053.13 }] }));
    }
    return r.fulfill(responder({ liquido: 5053.13, recebido: 0, diferenca: -5053.13, recebimentos: [], outros: [],
      sugestoes: [{ ...credito, motivos: ['Valor perto do líquido do holerite', 'O banco marcou como salário'] }] }));
  });
  await page.route('**/api/trabalho/holerites/5/fechar', (r) => {
    pedidos.push('fechar');
    return r.fulfill(responder({ ...holerite, situacao: 2, fechadoEm: '2026-11-13T23:00:00' }));
  });

  await page.goto('/trabalho/holerites?holerite=5');
  await expect(page.getByRole('heading', { name: 'Holerites', level: 1 })).toBeVisible();
  const lista = page.locator('section', { has: page.getByRole('heading', { name: 'Empresa Fictícia Ltda' }) });
  await expect(lista.getByRole('button', { name: /outubro de 2026/ })).toHaveAttribute('aria-current', 'true');
  await expect(lista).toContainText('setembro de 2026');
  const holeriteBloco = page.locator('section', { has: page.getByRole('heading', { name: /outubro de 2026 Aberto/ }) });
  await expect(holeriteBloco.locator('table')).toContainText('DSR sobre horas extras e adicional noturno');
  await expect(holeriteBloco.locator('tfoot')).toContainText('R$ 5.053,13');
  await expect(holeriteBloco).toContainText('FGTS do mês');

  const pasta = process.env['FOTOS_PASTA'];
  if (pasta) {
    mkdirSync(pasta, { recursive: true });
    await page.screenshot({ path: join(pasta, `trabalho-holerites-${test.info().project.name}.png`), fullPage: true });
  }

  // #212: o crédito sugerido, com o motivo, e ligar.
  const recebido = page.locator('section.recebimento');
  await expect(recebido).toContainText('Valor perto do líquido do holerite · O banco marcou como salário');
  await recebido.getByRole('button', { name: /^Ligar/ }).click();
  await expect(recebido).toContainText('Recebido igual ao líquido estimado.');
  expect(pedidos).toContain('ligar {"transacaoId":31,"valor":null}');

  // Mexeu: "Fechar o mês" espera o salvar.
  await holeriteBloco.getByRole('button', { name: '+ Lançamento' }).click();
  await expect(holeriteBloco.getByRole('button', { name: 'Fechar o mês' })).toBeDisabled();
  const novo = holeriteBloco.locator('fieldset.lancamento').nth(2);
  await novo.getByLabel('Tipo').selectOption({ label: 'Outro provento (bônus, PLR, comissão…)' });
  await novo.getByLabel('Descrição (opcional)').fill('Bônus');
  await novo.getByLabel('Valor').fill('800');
  await holeriteBloco.getByRole('button', { name: 'Salvar e calcular' }).click();
  await expect(page.getByText('Holerite calculado.')).toBeVisible();
  const enviado = JSON.parse(pedidos.find((p) => p.startsWith('PUT'))!.slice(4));
  expect(enviado.lancamentos[2]).toEqual({ tipo: 3, descricao: 'Bônus', quantidade: null, percentual: null, valor: 800,
    incideInss: true, incideIrrf: true, deduzIrrf: false, recorrente: false });
  expect(enviado.diasUteis).toBe(27);

  await holeriteBloco.getByRole('button', { name: 'Fechar o mês' }).click();
  await expect(page.getByText('Holerite de outubro de 2026 fechado: os números ficam como estão.')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Reabrir' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Salvar e calcular' })).toHaveCount(0);
});

test('Trabalho › Holerites: sem emprego, leva ao Salário', async ({ page }) => {
  await page.route('**/api/modulos', (r) => r.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([trabalho]) }));
  await page.route('**/api/trabalho/empregos', (r) => r.fulfill({ status: 200, contentType: 'application/json', body: '[]' }));
  await page.goto('/trabalho/holerites');
  await expect(page.getByRole('link', { name: 'Trabalho › Salário' })).toHaveAttribute('href', '/trabalho/salario');
});
