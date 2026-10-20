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
