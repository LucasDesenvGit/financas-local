import { mkdirSync } from 'node:fs';
import { join } from 'node:path';
import { expect, test } from './base';

/** #10 (ADR 0067): Notícias (API falsa, dados fictícios): desligado, o aviso diz para onde acessa; ligado, a lista. */
const modulo = { id: 'noticias', nome: 'Notícias', descricao: 'As notícias do dia.', acessaInternet: true,
  destinos: ['g1.globo.com', 'feeds.bbci.co.uk', 'www.tabnews.com.br'], ligado: false };
const lista = {
  fontes: [{ id: 'g1', nome: 'G1', atualizadoEm: '2026-11-14T14:50:00', erro: null },
    { id: 'bbc', nome: 'BBC News Brasil', atualizadoEm: null, erro: 'BBC News Brasil: respondeu 500' },
    { id: 'tabnews', nome: 'TabNews', atualizadoEm: '2026-11-14T14:50:00', erro: null }],
  noticias: [
    { fonte: 'g1', fonteNome: 'G1', titulo: 'Notícia fictícia do G1', link: 'https://g1.globo.com/ficticia.ghtml', publicadaEm: '2026-11-14T12:30:00' },
    { fonte: 'tabnews', fonteNome: 'TabNews', titulo: 'Artigo fictício', link: 'https://www.tabnews.com.br/autor/artigo', publicadaEm: '2026-11-14T12:00:00' }]
};

test('Notícias desligado: o aviso diz para onde o módulo acessa', async ({ page }) => {
  await page.route('**/api/modulos', (r) => r.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([modulo]) }));
  await page.goto('/noticias');
  await expect(page.getByRole('heading', { name: 'Este módulo está desligado' })).toBeVisible();
  await expect(page.getByText('Ligado, ele acessa a internet: g1.globo.com, feeds.bbci.co.uk e www.tabnews.com.br.')).toBeVisible();
  await expect(page.locator('nav.menu')).not.toContainText('Notícias');
});

test('Notícias: a lista, a fonte que falhou e o filtro', async ({ page }) => {
  const json = (corpo: unknown) => ({ status: 200, contentType: 'application/json', body: JSON.stringify(corpo) });
  let atualizou = false;
  await page.route('**/api/modulos', (r) => r.fulfill(json([{ ...modulo, ligado: true }])));
  await page.route('**/api/noticias', (r) => r.fulfill(json(lista)));
  await page.route('**/api/noticias/atualizar', (r) => { atualizou = true; return r.fulfill(json(lista)); });

  await page.goto('/noticias');
  await expect(page.locator('nav.menu')).toContainText('Notícias');
  const link = page.getByRole('link', { name: /Notícia fictícia do G1/ });
  await expect(link).toHaveAttribute('href', 'https://g1.globo.com/ficticia.ghtml');
  await expect(link).toHaveAttribute('target', '_blank');
  await expect(link).toHaveAttribute('rel', 'noopener noreferrer');
  await expect(page.locator('.situacao-fontes .falhou')).toHaveText('BBC News Brasil: ainda não atualizado; a última tentativa falhou');

  const pasta = process.env['FOTOS_PASTA'];
  if (pasta) {
    mkdirSync(pasta, { recursive: true });
    await page.screenshot({ path: join(pasta, `noticias-${test.info().project.name}.png`), fullPage: true });
  }

  await page.getByRole('button', { name: 'TabNews' }).click();
  await expect(page.locator('.noticia')).toHaveCount(1);
  await expect(page.locator('.noticia')).toContainText('Artigo fictício');
  await page.getByRole('button', { name: 'Atualizar agora' }).click();
  await expect(page.getByText('Não deu para atualizar: BBC News Brasil. As outras fontes foram atualizadas.')).toBeVisible();
  expect(atualizou).toBe(true);
});
