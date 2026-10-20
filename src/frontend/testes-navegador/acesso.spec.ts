import { ligarApiFalsa } from './api-falsa';
import { expect, test } from './base';

/** Entrada no site (#185): sem sessão vai para "entrar"; o primeiro acesso pede para criar o usuário. */
test.describe('acesso', () => {
  test('sem sessão, qualquer tela leva à entrada, e entrar leva ao Painel', async ({ page, problemas }) => {
    await page.unrouteAll();
    const api = await ligarApiFalsa(page, { '/api/auth/estado': { configurado: true, autenticado: false, login: null } });
    await page.goto('/metas');
    await expect(page).toHaveURL(/\/entrar/);
    await page.getByLabel(/Usuário/).fill('teste');
    await page.getByLabel(/Senha/).first().fill('uma senha qualquer');
    await page.getByRole('button', { name: /Entrar/ }).click();
    await expect.poll(() => api.chamadas).toContain('POST /api/auth/entrar');
    expect(problemas.lista).toEqual([]);
  });

  test('sem usuário criado, a entrada pede para criar um', async ({ page }) => {
    await page.unrouteAll();
    await ligarApiFalsa(page, { '/api/auth/estado': { configurado: false, autenticado: false, login: null } });
    await page.goto('/painel');
    await expect(page).toHaveURL(/\/entrar/);
    await expect(page.getByRole('button', { name: /Criar|Começar|Cadastrar/ })).toBeVisible();
  });
});
