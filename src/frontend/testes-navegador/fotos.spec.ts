import { mkdirSync } from 'node:fs';
import { join } from 'node:path';
import { expect, test } from './base';

/**
 * Prints das telas, para conferir a olho (skill "testar-no-navegador", #185). Só roda quando pedido:
 *   FOTOS=painel,metas,investimentos?aba=fii FOTOS_PASTA=<pasta> npx playwright test fotos
 * Rotas sem a barra do começo: o Git Bash troca "/painel" por "C:/Program Files/Git/painel".
 * Para cada rota: tema claro e escuro, no computador e no celular (os dois projetos). Os dados são os da API falsa;
 * nada real aparece.
 */
const rotas = (process.env['FOTOS'] ?? '').split(',').map((r) => r.trim().replace(/^\/+/, '')).filter(Boolean)
  .map((r) => '/' + r);
const pasta = process.env['FOTOS_PASTA'] ?? 'testes-navegador/fotos';

test.skip(rotas.length === 0, 'só com FOTOS=/rota1,/rota2');

for (const rota of rotas) {
  for (const tema of ['claro', 'escuro'] as const) {
    test(`foto ${rota} (${tema})`, async ({ page }, info) => {
      await page.emulateMedia({ colorScheme: tema === 'escuro' ? 'dark' : 'light' });
      await page.goto(rota);
      await expect(page.getByRole('heading', { level: 1 }).first()).toBeVisible();
      await page.waitForLoadState('networkidle');
      mkdirSync(pasta, { recursive: true });
      const nome = `${rota.replace(/[^a-z0-9]+/gi, '-').replace(/^-|-$/g, '') || 'inicio'}-${tema}-${info.project.name}.png`;
      await page.screenshot({ path: join(pasta, nome), fullPage: true });
    });
  }
}
