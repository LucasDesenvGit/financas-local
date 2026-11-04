import { expect, test } from './base';

/**
 * #631 (ADR 0057): o "Excluir" do card da meta pergunta pela janela do site, e não pelo confirm() do navegador.
 * A janela é um alertdialog com o nome da meta, o foco começa no Cancelar, Cancelar e Esc não apagam nada, e
 * "Excluir" (vermelho) manda o DELETE. Sem erro de console nem de CSP. Nenhum diálogo nativo do navegador pode abrir.
 */
test('Metas › Excluir no card pede confirmação no modal do site', async ({ page, api, problemas }) => {
  const nativos: string[] = [];
  page.on('dialog', async (d) => { nativos.push(d.message()); await d.dismiss(); });
  await page.goto('/metas');

  const excluir = page.getByRole('button', { name: 'Excluir a meta Viagem' });
  await excluir.click();
  const janela = page.getByRole('alertdialog', { name: 'Excluir meta' });
  await expect(janela).toBeVisible();
  await expect(janela).toContainText('Excluir a meta “Viagem”?');
  await expect(janela.getByRole('button', { name: 'Cancelar' })).toBeFocused();
  await expect(janela.getByRole('button', { name: 'Excluir', exact: true })).toHaveClass(/botao-perigo/);

  // Cancelar e Esc fecham sem apagar.
  await janela.getByRole('button', { name: 'Cancelar' }).click();
  await expect(janela).toBeHidden();
  await excluir.click();
  await expect(janela).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(janela).toBeHidden();
  expect(api.chamadas.filter((c) => c.startsWith('DELETE'))).toEqual([]);

  // Confirmando, apaga a meta 1.
  await excluir.click();
  await janela.getByRole('button', { name: 'Excluir', exact: true }).click();
  await expect(janela).toBeHidden();
  await expect.poll(() => api.chamadas.filter((c) => c.startsWith('DELETE'))).toEqual(['DELETE /api/metas/1']);

  expect(nativos).toEqual([]);
  expect(problemas.lista).toEqual([]);
});
