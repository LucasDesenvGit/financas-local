import { Page } from '@playwright/test';
import { expect, test } from './base';

/**
 * #332: "Reportar para o Claude", em Configurações › Erros, com um erro fictício: copia o texto para colar no Claude
 * Code, ou, sem a área de transferência, mostra o texto selecionado na linha. Sem erro de console nem bloqueio de CSP.
 */
async function umErro(page: Page) {
  await page.route(/\/api\/erros(\?|$)/, (r) => r.request().method() === 'GET' ? r.fulfill({ json: {
    itens: [{ id: 7, primeiraEm: '2026-10-16T12:00:00Z', ultimaEm: '2026-10-16T15:30:00Z', ocorrencias: 3, nivel: 4,
      origem: 'API', mensagem: 'Erro de teste na tela de metas', rota: 'GET /api/metas', usuario: null, resolvido: false,
      resolvidoEm: null }],
    total: 1, pagina: 1, tamanhoPagina: 25, abertos: 1, origens: ['API']
  } }) : r.fallback());
  await page.route(/\/api\/erros\/7$/, (r) => r.fulfill({ json: { id: 7, categoria: 'Erro', detalhe: 'pilha de teste' } }));
}

test('Configurações › Erros: "Reportar para o Claude" copia o erro e avisa', async ({ page, problemas }) => {
  await page.context().grantPermissions(['clipboard-read', 'clipboard-write']);
  await umErro(page);
  await page.goto('/configuracoes?aba=erros');
  const botao = page.getByRole('button', { name: /^Reportar para o Claude o erro de/ });
  expect((await botao.boundingBox())!.height).toBeGreaterThanOrEqual(23.5);
  await botao.click();
  await expect(page.getByText('Copiado. Cole no Claude Code.')).toBeVisible();
  const copiado = await page.evaluate(() => navigator.clipboard.readText());
  expect(copiado).toMatch(/^\/reportar-erro\r?\n/); // no Windows, a área de transferência devolve \r\n
  expect(copiado).toContain('- Mensagem: Erro de teste na tela de metas');
  expect(copiado).toContain('pilha de teste');
  expect(problemas.lista).toEqual([]);
});

test('Configurações › Erros: sem a área de transferência, o texto aparece selecionado na linha', async ({ page, problemas }) => {
  await page.addInitScript(() => Object.defineProperty(navigator, 'clipboard', { configurable: true, value: undefined }));
  await umErro(page);
  await page.goto('/configuracoes?aba=erros');
  await page.getByRole('button', { name: /^Reportar para o Claude o erro de/ }).click();
  const caixa = page.getByRole('textbox', { name: /copie o texto abaixo/ });
  await expect(caixa).toBeFocused();
  await expect(caixa).toHaveValue(/^\/reportar-erro\n/);
  const sobra = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(sobra).toBeLessThanOrEqual(1);
  await page.getByRole('button', { name: 'Fechar', exact: true }).click();
  await expect(caixa).toHaveCount(0);
  expect(problemas.lista).toEqual([]);
});
