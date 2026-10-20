import { expect, test } from './base';

/**
 * #241: regra de descrição por valor, no navegador, com a API falsa: a lista mostra a faixa de cada regra, a prévia
 * leva a faixa, e "De" maior que "Até" mostra o erro e trava o Salvar.
 */
test('Regras de descrição: faixa de valor para separar as duas contas do condomínio', async ({ page, problemas }) => {
  await page.goto('/categorias');
  await page.getByRole('tab', { name: 'Regras automáticas' }).click();
  const faixas = page.locator('td.faixa-valor');
  await expect(faixas.nth(0)).toHaveText('de R$ 800,00 a R$ 900,00');
  await expect(faixas.nth(1)).toHaveText('até R$ 250,00');

  await page.getByRole('button', { name: '+ Regra de descrição' }).click();
  const dialogo = page.getByRole('dialog');
  const faixa = dialogo.getByRole('group', { name: 'E o valor (opcional)' });
  await dialogo.getByLabel('Texto', { exact: true }).fill('CONDOMINIO');
  await dialogo.getByLabel('Mostrar como').fill('Condomínio — fundo de reserva');
  const previa = page.waitForRequest((r) => r.url().includes('/api/regras-descricao/previa')
    && r.url().includes('valorMinimo=150') && r.url().includes('valorMaximo=250'));
  await faixa.getByLabel(/^De/).fill('150');
  await faixa.getByLabel(/^Até/).fill('250');
  await previa;
  await expect(dialogo.getByRole('status')).toContainText('Vai renomear 6 transação(ões)');
  await expect(dialogo.getByRole('button', { name: 'Salvar' })).toBeEnabled();

  await faixa.getByLabel(/^De/).fill('900');
  await faixa.getByLabel(/^Até/).fill('800');
  await expect(faixa.getByRole('alert')).toHaveText('O valor "De" é maior que o "Até": troque os dois.');
  await expect(dialogo.getByRole('button', { name: 'Salvar' })).toBeDisabled();
  expect(problemas.lista).toEqual([]);
});
