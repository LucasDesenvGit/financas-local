import { mkdirSync } from 'node:fs';
import { join } from 'node:path';
import { expect, test } from './base';

/**
 * Manual objetivo (#817, ADR 0071): "Mais detalhes" fechado, que abre pelo teclado (Enter e Espaço, WCAG 2.1.1),
 * e a página sem rolagem para o lado, também no celular. Com FOTOS_PASTA, guarda o print da seção aberta e fechada.
 */
test('"Mais detalhes" abre pelo teclado e a seção cabe na tela', async ({ page }, info) => {
  await page.goto('/manual/primeiros-passos');
  const secao = page.locator('#conhecendo-a-tela');
  await expect(secao.getByRole('heading', { name: 'Conhecendo a tela' })).toBeVisible();
  const resumo = secao.getByText('Mais detalhes');
  const detalhes = secao.locator('details');
  await expect(detalhes).not.toHaveAttribute('open', '');

  const pasta = process.env['FOTOS_PASTA'];
  if (pasta) { mkdirSync(pasta, { recursive: true }); await secao.screenshot({ path: join(pasta, `secao-fechada-${info.project.name}.png`) }); }

  await resumo.focus();
  await page.keyboard.press('Enter');
  await expect(detalhes).toHaveAttribute('open', '');
  await expect(secao.getByText('Grupos do menu:')).toBeVisible();
  await page.keyboard.press('Space');
  await expect(detalhes).not.toHaveAttribute('open', '');
  await page.keyboard.press('Space');
  await expect(detalhes).toHaveAttribute('open', '');
  if (pasta) await secao.screenshot({ path: join(pasta, `secao-aberta-${info.project.name}.png`) });

  const sobra = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  expect(sobra).toBeLessThanOrEqual(0);
});
