import { expect, test } from './base';

/**
 * #254: Sugestões de categoria. O motivo não fica embaixo dos chips; regra com aviso da API e regras iguais com
 * categorias diferentes vêm desmarcadas, com o porquê em texto; "Aplicar as de confiança alta" não cria regra em conflito.
 */
test('Sugestões de categoria: chips abaixo do motivo, sem cobrir o texto', async ({ page }) => {
  await page.goto('/transacoes');
  await page.getByRole('button', { name: /Sugestões de categoria/ }).click();
  const linha = page.locator('.tabela-sugestoes tbody tr').first();
  const motivo = await linha.locator('td:nth-child(4) small.texto-suave').first().boundingBox();
  const chips = await linha.locator('.sugestoes-chips').boundingBox();
  // O topo dos chips fica abaixo do fim do motivo, com folga (antes, o margin-top negativo os puxava para cima dele).
  expect(chips!.y).toBeGreaterThanOrEqual(motivo!.y + motivo!.height + 2);
});

test('Sugestões de categoria: regras em conflito ou com aviso vêm desmarcadas, com o motivo', async ({ page, api }) => {
  await page.goto('/transacoes');
  await page.getByRole('button', { name: /Sugestões de categoria/ }).click();
  const linhas = page.locator('.tabela-sugestoes tbody tr');
  const marcar = (i: number) => linhas.nth(i).getByRole('checkbox');

  // Mesmo "MERCADOLIVRE" para Alimentação e para Lazer: nenhuma das duas vem marcada.
  await expect(marcar(0)).not.toBeChecked();
  await expect(marcar(2)).not.toBeChecked();
  // A da API com aviso também não, e o aviso aparece em texto.
  await expect(marcar(1)).not.toBeChecked();
  await expect(linhas.nth(1)).toContainText('também aparece em 4 transações de outras categorias');
  // Sem conflito nem aviso: marcada.
  await expect(marcar(3)).toBeChecked();

  // Marcar as duas do MERCADOLIVRE mostra o conflito nas duas, com a categoria da outra.
  await marcar(0).check();
  await marcar(2).check();
  await expect(linhas.nth(0)).toContainText('Outra linha cria esta regra para Lazer');
  await expect(linhas.nth(2)).toContainText('Outra linha cria esta regra para Alimentação');

  // "Aplicar as 4 de confiança alta": categoriza todas, mas só cria a regra do IFOOD.
  page.once('dialog', (d) => {
    expect(d.message()).toContain('Sem regra (o mesmo texto com categorias diferentes): "MERCADOLIVRE"');
    void d.accept();
  });
  await page.getByRole('button', { name: /Aplicar as 4 de confiança alta/ }).click();
  await expect.poll(() => api.chamadas.filter((c) => c === 'POST /api/regras').length).toBe(1);
  await expect.poll(() => api.chamadas.filter((c) => c === 'PUT /api/transacoes/categoria-em-lote').length).toBe(4);
});
