import { expect, test } from './base';

/**
 * #475 (ADR 0059): as janelas de lançamento e de transferência de Transações são componentes de apresentação. A tela
 * é quem grava: aqui, o caminho inteiro pelo navegador, do botão ao pedido que chega na API.
 */
test('Transações: "+ Lançamento" grava pela janela, na primeira conta manual', async ({ page, problemas }) => {
  await page.goto('/transacoes');
  await page.getByRole('button', { name: '+ Lançamento' }).click();
  const janela = page.getByRole('dialog', { name: 'Novo lançamento' });
  await expect(janela).toBeVisible();
  await janela.getByLabel('Descrição').fill('Feira do sábado');
  await janela.getByLabel('Valor').fill('42,50');
  const pedido = page.waitForRequest((r) => r.method() === 'POST' && new URL(r.url()).pathname === '/api/transacoes');
  await janela.getByRole('button', { name: 'Salvar' }).click();
  expect((await pedido).postDataJSON()).toMatchObject({ descricao: 'Feira do sábado', valor: 42.5, contaId: 5, tipo: 1 });
  await expect(janela).toBeHidden();
  expect(problemas.lista).toEqual([]);
});

test('Transações: "Cancelar" na edição fecha sem gravar', async ({ page, api, problemas }) => {
  await page.goto('/transacoes');
  await page.getByRole('button', { name: /^Editar: / }).first().click();
  const janela = page.getByRole('dialog', { name: 'Editar transação' });
  await janela.getByLabel('Descrição').fill('outra coisa');
  await janela.getByRole('button', { name: 'Cancelar' }).click();
  await expect(janela).toBeHidden();
  expect(api.chamadas.filter((c) => !c.startsWith('GET '))).toEqual([]);
  expect(problemas.lista).toEqual([]);
});

test('Transações: "Transferência" grava pela janela', async ({ page, problemas }) => {
  await page.goto('/transacoes');
  await page.getByRole('button', { name: '⇄ Transferência' }).click();
  const janela = page.getByRole('dialog', { name: 'Transferência entre contas' });
  await janela.locator('select[name=origem]').selectOption({ label: 'Conta Exemplo' });
  await janela.locator('select[name=destino]').selectOption({ index: 3 });
  await janela.getByLabel('Valor').fill('100');
  const pedido = page.waitForRequest((r) => r.method() === 'POST' && new URL(r.url()).pathname === '/api/transacoes/transferencia');
  await janela.getByRole('button', { name: 'Transferir' }).click();
  expect((await pedido).postDataJSON()).toMatchObject({ contaOrigemId: 1, valor: 100 });
  await expect(janela).toBeHidden();
  expect(problemas.lista).toEqual([]);
});
