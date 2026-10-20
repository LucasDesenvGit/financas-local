import { expect, test } from './base';

/**
 * #206: no Painel, os orçamentos estourados num card (os 3 piores, "Ver todos", link para Transações) e os outros
 * avisos em outro; os dois dá para ocultar, e a escolha sobrevive a recarregar a página (localStorage, ADR 0017).
 */
test('Painel: card dos orçamentos estourados leva a Transações e fica oculto depois de recarregar', async ({ page, problemas }) => {
  await page.goto('/painel');
  const card = page.getByRole('region', { name: /4 orçamentos estourados neste mês/ });
  await expect(card).toBeVisible();
  await expect(card.getByRole('listitem')).toHaveCount(3);
  await expect(card.getByRole('listitem').first()).toContainText('348%');

  // A parte clara da barra é o limite: com 348%, perto de 100/348 da largura.
  const trilho = card.locator('.trilho-estourado').first();
  const larguras = await trilho.evaluate((t) => [t.getBoundingClientRect().width,
    (t.querySelector('.dentro-limite') as HTMLElement).getBoundingClientRect().width]);
  expect(larguras[1] / larguras[0]).toBeCloseTo(100 / 348, 1);

  await card.getByRole('button', { name: 'Ver todos os 4' }).click();
  await expect(card.getByRole('listitem')).toHaveCount(4);

  await card.getByRole('link', { name: /^Ver as despesas de Lazer em / }).click();
  await expect(page).toHaveURL(/\/transacoes\?.*categoria=3/);
  await page.goBack();

  await page.getByRole('button', { name: 'Ocultar os avisos de orçamento até o mês que vem' }).click();
  await expect(card.getByRole('listitem')).toHaveCount(0);
  await page.reload();
  await expect(page.getByRole('region', { name: /4 orçamentos estourados/ })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Mostrar os orçamentos estourados' })).toHaveAttribute('aria-expanded', 'false');
  expect(problemas.lista).toEqual([]);
});

test('Painel: os outros avisos têm card próprio, e o "Ocultar" funciona pelo teclado', async ({ page }) => {
  await page.goto('/painel');
  const card = page.getByRole('region', { name: '1 aviso pede atenção' });
  await expect(card).toContainText('Conta "Aluguel" vencida');
  const botao = card.getByRole('button', { name: 'Ocultar os avisos até aparecer um novo' });
  await botao.focus();
  await page.keyboard.press('Enter');
  await expect(card.getByRole('listitem')).toHaveCount(0);
  await expect(card.getByRole('button', { name: 'Mostrar os avisos' })).toBeFocused();
});
