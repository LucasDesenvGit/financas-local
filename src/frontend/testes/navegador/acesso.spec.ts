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

  test('sem usuário criado, o link do gerenciador pede para criar um; sem o link, só explica (#68)', async ({ page }) => {
    await page.unrouteAll();
    await ligarApiFalsa(page, { '/api/auth/estado': { configurado: false, autenticado: false, login: null } });
    await page.goto('/painel');
    await expect(page).toHaveURL(/\/entrar/);
    await expect(page.getByText(/abra o site pelo gerenciador/)).toBeVisible();
    await expect(page.getByRole('button', { name: /Criar e entrar/ })).toHaveCount(0);

    await page.goto('/entrar?configurar=123.abc');
    await expect(page.getByRole('button', { name: /Criar e entrar/ })).toBeVisible();
    await expect(page).not.toHaveURL(/configurar=/);
    const pedido = page.waitForRequest((r) => r.url().endsWith('/api/auth/configurar'));
    await page.getByLabel('Usuário').fill('teste');
    await page.getByLabel('Senha', { exact: true }).fill('Senha-Forte-123');
    await page.getByLabel('Confirmar senha').fill('Senha-Forte-123');
    await page.getByRole('button', { name: /Criar e entrar/ }).click();
    expect((await pedido).headers()['x-configurar-token']).toBe('123.abc');
  });
});
