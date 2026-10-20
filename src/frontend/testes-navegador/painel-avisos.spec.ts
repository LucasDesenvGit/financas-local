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

  await card.getByRole('button', { name: 'Ocultar os avisos de orçamento' }).click();
  await page.getByRole('menuitem', { name: /^Ocultar até o mês que vem/ }).click();
  await expect(card.getByRole('listitem')).toHaveCount(0);
  // O foco vai para o "Mostrar" que entra no lugar do menu (WCAG 2.4.3).
  await expect(card.getByRole('button', { name: 'Mostrar os orçamentos estourados' })).toBeFocused();
  await page.reload();
  await expect(page.getByRole('region', { name: /4 orçamentos estourados/ })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Mostrar os orçamentos estourados' })).toHaveAttribute('aria-expanded', 'false');
  expect(problemas.lista).toEqual([]);
});

test('Painel: os outros avisos têm card próprio, e o menu "Ocultar" funciona pelo teclado', async ({ page }) => {
  await page.goto('/painel');
  const card = page.getByRole('region', { name: '1 aviso pede atenção' });
  await expect(card).toContainText('Conta "Aluguel" vencida');
  const botao = card.getByRole('button', { name: 'Ocultar os avisos' });
  await botao.focus();
  await page.keyboard.press('Enter');
  // Enter abre o menu com o foco no 1º item; Esc fecha e devolve o foco ao botão.
  await expect(page.getByRole('menuitem', { name: /^Minimizar/ })).toBeFocused();
  await page.keyboard.press('Escape');
  await expect(botao).toBeFocused();
  await expect(page.getByRole('menu')).toHaveCount(0);

  // ↓ abre (o foco vai ao 1º item logo depois; esperar, senão o 2º ↓ cai no botão de novo), ↓ vai ao 2º item, Enter escolhe.
  await page.keyboard.press('ArrowDown');
  await expect(page.getByRole('menuitem', { name: /^Minimizar/ })).toBeFocused();
  await page.keyboard.press('ArrowDown');
  await expect(page.getByRole('menuitem', { name: /^Ocultar até aparecer um novo/ })).toBeFocused();
  await page.keyboard.press('Enter');
  await expect(card.getByRole('listitem')).toHaveCount(0);
  await expect(card.getByRole('button', { name: 'Mostrar os avisos' })).toBeFocused();
});

test('Painel: o menu "Ocultar" aparece inteiro, sem ser cortado pelo card, também no celular', async ({ page }) => {
  await page.goto('/painel');
  const card = page.getByRole('region', { name: /orçamentos estourados/ });
  await card.getByRole('button', { name: 'Ocultar os avisos de orçamento' }).click();
  const menu = page.getByRole('menu');
  await expect(menu).toBeVisible();
  const caixa = (await menu.boundingBox())!;
  expect(caixa.x).toBeGreaterThanOrEqual(0);
  expect(caixa.x + caixa.width).toBeLessThanOrEqual(page.viewportSize()!.width);
  // O último item é clicável no ponto dele: nada por cima, nada cortado.
  await page.getByRole('menuitem', { name: /^Ocultar até o mês que vem/ }).click();
  await expect(card.getByRole('listitem')).toHaveCount(0);
  // Clique fora fecha um menu aberto.
  await page.getByRole('region', { name: '1 aviso pede atenção' }).getByRole('button', { name: 'Ocultar os avisos' }).click();
  await expect(page.getByRole('menu')).toBeVisible();
  await page.getByRole('heading', { level: 1 }).click();
  await expect(page.getByRole('menu')).toHaveCount(0);
});

test('Painel: "Minimizar" continua valendo ao recarregar e ao voltar ao Painel, até "Mostrar"', async ({ page }) => {
  await page.goto('/painel');
  const card = page.getByRole('region', { name: /orçamentos estourados/ });
  await card.getByRole('button', { name: 'Ocultar os avisos de orçamento' }).click();
  await page.getByRole('menuitem', { name: /^Minimizar/ }).click();
  await expect(card.getByRole('listitem')).toHaveCount(0);

  await page.reload();
  await expect(card.getByRole('button', { name: 'Mostrar os orçamentos estourados' })).toBeVisible();
  await expect(card.getByRole('listitem')).toHaveCount(0);
  await page.goto('/metas');
  await page.goto('/painel');
  await expect(card.getByRole('listitem')).toHaveCount(0);

  await card.getByRole('button', { name: 'Mostrar os orçamentos estourados' }).click();
  await expect(card.getByRole('listitem')).toHaveCount(3);
  await page.reload();
  await expect(card.getByRole('listitem')).toHaveCount(3);
});
