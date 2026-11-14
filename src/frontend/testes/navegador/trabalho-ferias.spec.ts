import { mkdirSync } from 'node:fs';
import { join } from 'node:path';
import { expect, test } from './base';

/** #213 (ADR 0065): Trabalho › Férias (API falsa, dados fictícios): os períodos, a conta, a comparação e o agendamento. */
const trabalho = { id: 'trabalho', nome: 'Trabalho', descricao: 'd', acessaInternet: false, destinos: [], ligado: true };
const emprego = { id: 1, empregador: 'Empresa Fictícia Ltda', cnpj: null, admissao: '2024-03-11', saida: null, regime: 1,
  jornadaMensalHoras: 220, dependentes: 0, contaId: null, contaNome: null, salarios: [{ id: 1, inicio: '2024-03-01', salarioBruto: 6000 }] };
const conta = (dias: number, vendidos: number, liquido: number) => ({
  inicio: '2026-12-01', retorno: '2026-12-31', pagamento: '2026-11-29', dias, diasVendidos: vendidos, remuneracao: 6000, mediaVariaveis: 0,
  ferias: 6000 * dias / 30, tercoFerias: 2000 * dias / 30, abono: vendidos ? 2000 : 0, tercoAbono: vendidos ? 666.67 : 0,
  inss: { valor: dias === 30 ? 921.51 : 548.18 }, irrf: { valor: dias === 30 ? 1037.85 : 122.45 }, adiantamentoDecimo: 0, liquido });
const periodos = [
  { inicio: '2024-03-11', fim: '2025-03-10', fimConcessivo: '2026-03-10', completo: true, faltas: 0, diasDireito: 30,
    diasAgendados: 0, diasVendidos: 0, saldo: 30, vencida: true, vencendo: false, agendamentos: [] },
  { inicio: '2025-03-11', fim: '2026-03-10', fimConcessivo: '2027-03-10', completo: true, faltas: 0, diasDireito: 30,
    diasAgendados: 0, diasVendidos: 0, saldo: 30, vencida: false, vencendo: false, agendamentos: [] },
  { inicio: '2026-03-11', fim: '2027-03-10', fimConcessivo: '2028-03-10', completo: false, faltas: 0, diasDireito: 30,
    diasAgendados: 0, diasVendidos: 0, saldo: 30, vencida: false, vencendo: false, agendamentos: [] }];

test('Trabalho › Férias: períodos, a conta, 30 × vender 10 e agendar', async ({ page }) => {
  const json = (corpo: unknown) => ({ status: 200, contentType: 'application/json', body: JSON.stringify(corpo) });
  const pedidos: string[] = [];
  await page.route('**/api/modulos', (r) => r.fulfill(json([trabalho])));
  await page.route('**/api/trabalho/empregos', (r) => r.fulfill(json([emprego])));
  await page.route('**/api/trabalho/empregos/1/ferias', (r) => {
    pedidos.push(`${r.request().method()} ${r.request().postData() ?? ''}`.trim());
    if (r.request().method() === 'POST') {
      const agendado = { ...periodos[1], diasAgendados: 20, diasVendidos: 10, saldo: 0,
        agendamentos: [{ id: 7, inicio: '2026-12-01', dias: 20, diasVendidos: 10, adiantarDecimo: false, calculo: conta(20, 10, 7329.37) }] };
      return r.fulfill(json([periodos[0], agendado, periodos[2]]));
    }
    return r.fulfill(json(periodos));
  });
  await page.route('**/api/trabalho/empregos/1/ferias/simular**', (r) => {
    const q = new URL(r.request().url()).searchParams;
    pedidos.push(`simular ${q.get('dias')} ${q.get('vendidos')}`);
    return r.fulfill(json(q.get('dias') === '30' ? conta(30, 0, 6040.64) : conta(20, 10, 7329.37)));
  });

  await page.goto('/trabalho/ferias');
  const lista = page.locator('section', { has: page.getByRole('heading', { name: 'Períodos' }) });
  await expect(lista.locator('.periodo')).toHaveCount(3);
  await expect(lista.locator('.periodo').first()).toContainText('Vencida');
  await expect(lista).toContainText('a empresa paga essas férias em dobro');
  await expect(lista.locator('.periodo').nth(2)).toContainText('Em aquisição');

  const agendar = page.locator('section', { has: page.getByRole('heading', { name: 'Agendar' }) });
  await agendar.getByLabel('Começa em').fill('01/12/2026');
  await agendar.getByRole('button', { name: 'Comparar 30 dias × vender 10' }).click();
  await expect(agendar.locator('.comparacao')).toContainText('R$ 6.040,64');
  await expect(agendar.locator('.comparacao')).toContainText('R$ 7.329,37');
  await expect(agendar.locator('.comparacao')).toContainText('R$ 1.288,73 a mais');

  await agendar.getByLabel('Dias de férias').fill('20');
  await agendar.getByLabel('Dias vendidos').fill('10');
  await agendar.getByRole('button', { name: 'Ver a conta' }).click();
  await expect(agendar.locator('.conta')).toContainText('Cai na conta até 29/11');

  const pasta = process.env['FOTOS_PASTA'];
  if (pasta) {
    mkdirSync(pasta, { recursive: true });
    await page.screenshot({ path: join(pasta, `trabalho-ferias-${test.info().project.name}.png`), fullPage: true });
  }

  await agendar.getByRole('button', { name: 'Agendar' }).click();
  await expect(page.getByText('Férias agendadas.')).toBeVisible();
  expect(pedidos.find((p) => p.startsWith('POST'))).toBe(
    'POST {"periodoInicio":"2024-03-11","inicio":"2026-12-01","dias":20,"diasVendidos":10,"adiantarDecimo":false}');
  await expect(lista.locator('.periodo').nth(1)).toContainText('Recebe R$ 7.329,37 até 29/11');
});
