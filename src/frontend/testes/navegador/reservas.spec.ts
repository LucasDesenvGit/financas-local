import { expect, test } from './base';

/**
 * #237: "conferir saldo" só some da reserva que o banco detalha pelo nome e da linha com o restante do total do banco.
 * A reserva fora da lista detalhada e as da conta em que o banco manda só o total continuam conferíveis (API falsa).
 */
test('Investimentos › Reservas: "conferir saldo" só onde o valor não veio do banco pelo nome', async ({ page, problemas }) => {
  await page.goto('/investimentos?aba=reservas');
  const linha = (nome: string) => page.locator('table.tabela tbody tr')
    .filter({ has: page.locator('strong', { hasText: new RegExp(`^${nome}$`) }) });
  const conferir = (nome: string) => linha(nome).getByRole('button', { name: 'conferir saldo' });

  await expect(linha('Viagem')).toContainText('informado pelo banco');
  await expect(conferir('Viagem')).toHaveCount(0);

  await expect(linha('Presentes')).not.toContainText('informado pelo banco');
  await expect(linha('Presentes')).toContainText('não está na lista de reservas que a instituição manda');
  await expect(conferir('Presentes')).toBeVisible();

  await expect(linha('Carro')).not.toContainText('informado pelo banco');
  await expect(conferir('Carro')).toBeVisible();

  await expect(linha('Reservado')).toContainText('restante do total do banco');
  await expect(conferir('Reservado')).toHaveCount(0);

  await conferir('Carro').click();
  await expect(page.getByRole('dialog', { name: 'Conferir saldo · Carro' })).toBeVisible();
  const sobra = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(sobra).toBeLessThanOrEqual(1);
  expect(problemas.lista).toEqual([]);
});

/**
 * #691, parte 2: "registrar saída" de um cofrinho, e a conferência abaixo da estimativa com a opção de registrar a
 * diferença como gasto (só com a data de hoje).
 */
test('Investimentos › Reservas: registrar saída e a diferença como gasto (#691)', async ({ page, api, problemas }) => {
  await page.goto('/investimentos?aba=reservas');
  await expect(page.getByRole('button', { name: 'Registrar saída do cofrinho Reservado' })).toHaveCount(0);
  await page.getByRole('button', { name: 'Registrar saída do cofrinho Carro' }).click();
  const saida = page.getByRole('dialog', { name: 'Registrar saída · Carro' });
  await saida.getByLabel('Descrição').fill('Compra no Mercado Livre');
  await saida.getByLabel('Valor').fill('45,90');
  await saida.getByRole('button', { name: 'Registrar' }).click();
  await expect.poll(() => api.chamadas.includes('POST /api/reservas/saida')).toBe(true);

  const linha = page.locator('table.tabela tbody tr').filter({ has: page.locator('strong', { hasText: /^Carro$/ }) });
  await linha.getByRole('button', { name: 'conferir saldo' }).click();
  const conferencia = page.getByRole('dialog', { name: 'Conferir saldo · Carro' });
  const diferenca = conferencia.getByLabel(/Registrar a diferença .* como gasto do cofrinho/);
  await expect(diferenca).toHaveCount(0);
  await conferencia.getByLabel('Saldo na instituição').fill('350');
  await expect(diferenca).toBeVisible();
  await expect(diferenca).not.toBeChecked();
  expect(problemas.lista).toEqual([]);
});
