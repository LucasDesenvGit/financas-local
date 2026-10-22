import type { Page } from '@playwright/test';
import { expect, test } from './base';

/**
 * #206 e #327: os orçamentos estourados num card (os 3 piores, "Ver todos", link para Transações) e os outros avisos em
 * outro, dentro das notificações da barra do topo (o sino). Os dois dá para ocultar, e a escolha sobrevive a recarregar
 * a página (localStorage, ADR 0017).
 */
async function abrirSino(page: Page) {
  const sino = page.getByRole('button', { name: /^Notificações/ });
  await sino.click();
  await expect(sino).toHaveAttribute('aria-expanded', 'true');
}

test('Notificações: o sino mostra quantas há, e o Painel não tem mais os avisos no topo', async ({ page, problemas }) => {
  await page.goto('/painel');
  const sino = page.getByRole('button', { name: 'Notificações, 5 pendentes' });
  await expect(sino).toBeVisible();
  expect((await sino.boundingBox())!.height).toBeGreaterThanOrEqual(23.5);
  await expect(page.locator('main app-painel-avisos')).toHaveCount(0);
  // Em outra tela, o sino continua lá.
  await page.goto('/transacoes');
  await abrirSino(page);
  await expect(page.getByRole('region', { name: /4 orçamentos estourados neste mês/ })).toBeVisible();
  // O card cabe na tela (também no celular).
  const caixa = (await page.locator('.card-notificacoes').boundingBox())!;
  expect(caixa.x).toBeGreaterThanOrEqual(0);
  expect(caixa.x + caixa.width).toBeLessThanOrEqual(page.viewportSize()!.width);
  // A nota das descrições pouco informativas fica no card, com "Revisar", sem entrar no contador.
  await expect(page.locator('.pouco-informativas')).toContainText('9 transações dos últimos 90 dias');
  // No computador, o card cresce com o conteúdo: sem rolagem dentro dele (cortava o menu "Ocultar").
  if (page.viewportSize()!.width > 760) {
    const rola = await page.locator('.card-notificacoes').evaluate((c) => c.scrollHeight > c.clientHeight + 1);
    expect(rola).toBe(false);
  }
  await page.keyboard.press('Escape');
  await expect(page.locator('.card-notificacoes')).toHaveCount(0);
  await expect(page.getByRole('button', { name: /^Notificações/ })).toBeFocused();
  expect(problemas.lista).toEqual([]);
});

test('Notificações: card dos orçamentos estourados leva a Transações e fica oculto depois de recarregar', async ({ page, problemas }) => {
  await page.goto('/painel');
  await abrirSino(page);
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

  // O link leva a Transações, e o card fecha (a tela nova não fica coberta).
  await card.getByRole('link', { name: /^Ver as despesas de Lazer em / }).click();
  await expect(page).toHaveURL(/\/transacoes\?.*categoria=3/);
  await expect(page.locator('.card-notificacoes')).toHaveCount(0);
  await page.goBack();

  await abrirSino(page);
  await card.getByRole('button', { name: 'Ocultar os avisos de orçamento' }).click();
  await page.getByRole('menuitem', { name: /^Ocultar até o mês que vem/ }).click();
  await expect(card.getByRole('listitem')).toHaveCount(0);
  // O foco vai para o "Mostrar" que entra no lugar do menu (WCAG 2.4.3).
  await expect(card.getByRole('button', { name: 'Mostrar os orçamentos estourados' })).toBeFocused();
  // O contador não conta mais os orçamentos ocultos.
  await expect(page.getByRole('button', { name: 'Notificações, 1 pendente' })).toBeVisible();
  await page.reload();
  await abrirSino(page);
  await expect(page.getByRole('region', { name: /4 orçamentos estourados/ })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Mostrar os orçamentos estourados' })).toHaveAttribute('aria-expanded', 'false');
  expect(problemas.lista).toEqual([]);
});

test('Notificações: os outros avisos têm card próprio, e o menu "Ocultar" funciona pelo teclado', async ({ page }) => {
  await page.goto('/painel');
  await abrirSino(page);
  const card = page.getByRole('region', { name: '1 aviso pede atenção' });
  await expect(card).toContainText('Conta "Aluguel" vencida');
  const botao = card.getByRole('button', { name: 'Ocultar os avisos' });
  await botao.focus();
  await page.keyboard.press('Enter');
  // Enter abre o menu com o foco no 1º item; Esc fecha só o menu e devolve o foco ao botão.
  await expect(page.getByRole('menuitem', { name: /^Minimizar/ })).toBeFocused();
  await page.keyboard.press('Escape');
  await expect(botao).toBeFocused();
  await expect(page.getByRole('menu')).toHaveCount(0);
  await expect(card).toBeVisible();

  // ↓ abre (o foco vai ao 1º item logo depois; esperar, senão o 2º ↓ cai no botão de novo), ↓ vai ao 2º item, Enter escolhe.
  await page.keyboard.press('ArrowDown');
  await expect(page.getByRole('menuitem', { name: /^Minimizar/ })).toBeFocused();
  await page.keyboard.press('ArrowDown');
  await expect(page.getByRole('menuitem', { name: /^Ocultar até aparecer um novo/ })).toBeFocused();
  await page.keyboard.press('Enter');
  await expect(card.getByRole('listitem')).toHaveCount(0);
  await expect(card.getByRole('button', { name: 'Mostrar os avisos' })).toBeFocused();
});

test('Notificações: o menu "Ocultar" aparece inteiro, sem ser cortado pelo card, também no celular', async ({ page }) => {
  await page.goto('/painel');
  await abrirSino(page);
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
  // Clique fora do menu (no título do card de notificações) fecha o menu, e o card continua aberto.
  await page.getByRole('region', { name: '1 aviso pede atenção' }).getByRole('button', { name: 'Ocultar os avisos' }).click();
  await expect(page.getByRole('menu')).toBeVisible();
  await page.locator('#titulo-notificacoes').click();
  await expect(page.getByRole('menu')).toHaveCount(0);
  await expect(page.locator('.card-notificacoes')).toBeVisible();
  // Clique fora do card (no pé da tela; no celular o card cobre o topo da página) fecha as notificações.
  const tela = page.viewportSize()!;
  await page.mouse.click(8, tela.height - 8);
  await expect(page.locator('.card-notificacoes')).toHaveCount(0);
});

test('Notificações: "Minimizar" continua valendo ao recarregar e em outra tela, até "Mostrar"', async ({ page }) => {
  // Três recargas e uma troca de tela: no celular emulado, numa máquina ocupada, passa dos 30 s padrão.
  test.slow();
  await page.goto('/painel');
  await abrirSino(page);
  const card = page.getByRole('region', { name: /orçamentos estourados/ });
  await card.getByRole('button', { name: 'Ocultar os avisos de orçamento' }).click();
  await page.getByRole('menuitem', { name: /^Minimizar/ }).click();
  await expect(card.getByRole('listitem')).toHaveCount(0);

  await page.reload();
  await abrirSino(page);
  await expect(card.getByRole('button', { name: 'Mostrar os orçamentos estourados' })).toBeVisible();
  await expect(card.getByRole('listitem')).toHaveCount(0);
  await page.goto('/metas');
  await abrirSino(page);
  await expect(card.getByRole('listitem')).toHaveCount(0);

  await card.getByRole('button', { name: 'Mostrar os orçamentos estourados' }).click();
  await expect(card.getByRole('listitem')).toHaveCount(3);
  await page.reload();
  await abrirSino(page);
  await expect(card.getByRole('listitem')).toHaveCount(3);
});
