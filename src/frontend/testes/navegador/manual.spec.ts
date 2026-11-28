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

/** #820: trocar de capítulo pelo fim da página volta ao topo; link com âncora continua indo à seção. */
test('trocar de capítulo pelo fim da página volta ao topo, e a âncora ainda rola até a seção', async ({ page }) => {
  await page.goto('/manual/primeiros-passos');
  await expect(page.getByRole('heading', { level: 1 }).first()).toBeVisible();
  await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
  await expect.poll(() => page.evaluate(() => window.scrollY)).toBeGreaterThan(300);

  const proximo = page.getByRole('link', { name: /›$/ });
  const href = await proximo.getAttribute('href');
  await proximo.click();
  await expect(page).toHaveURL(new RegExp(`${href}$`));
  await expect.poll(() => page.evaluate(() => window.scrollY)).toBe(0);

  await page.goto('/manual/primeiros-passos');
  await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
  await page.getByRole('link', { name: 'Índice do manual' }).click();
  await expect(page).toHaveURL(/\/manual$/);
  await expect.poll(() => page.evaluate(() => window.scrollY)).toBe(0);

  await page.goto('/manual/configuracoes#aparencia');
  await expect(page.locator('#aparencia')).toBeVisible();
  await expect.poll(() => page.evaluate(() => window.scrollY)).toBeGreaterThan(0);
  expect(await page.locator('#aparencia').evaluate(e => e.getBoundingClientRect().top)).toBeLessThan(400);
});

/** #820: navegação só de query (a busca do Manual a cada tecla) não joga a página para o topo. */
test('digitar na busca do Manual não volta ao topo', async ({ page }) => {
  await page.goto('/manual');
  const busca = page.getByLabel('Buscar no manual');
  await busca.click();
  await page.keyboard.type('a');
  await expect(page).toHaveURL(/busca=a$/);
  await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
  const antes = await page.evaluate(() => window.scrollY);
  expect(antes).toBeGreaterThan(300);
  await page.keyboard.type('s');
  await expect(page).toHaveURL(/busca=as$/);
  await page.waitForTimeout(300);
  expect(await page.evaluate(() => window.scrollY)).toBeGreaterThan(0);
});
