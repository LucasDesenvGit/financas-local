import { expect, test } from './base';

/**
 * #191 (ADR 0064): Configurações › Módulos (API falsa, com dois módulos fictícios). Um interruptor por módulo, com o
 * que ele faz e para onde acessa a internet; ligar e desligar chamam a API na hora, e o erro volta o interruptor.
 */
test('Configurações › Módulos: ligar e desligar pelo interruptor', async ({ page, problemas }) => {
  const exemplo = { id: 'exemplo', nome: 'Exemplo', descricao: 'Um módulo de exemplo.', acessaInternet: false, destinos: [], ligado: false };
  const daRede = { id: 'darede', nome: 'Da rede', descricao: 'Busca coisas na internet.', acessaInternet: true,
    destinos: ['a.example.com', 'b.example.com'], ligado: false };
  await page.route('**/api/modulos', (rota) => rota.fulfill({ status: 200, contentType: 'application/json',
    body: JSON.stringify([exemplo, daRede]) }));
  const pedidos: string[] = [];
  await page.route('**/api/modulos/*', (rota) => {
    const req = rota.request();
    pedidos.push(`${req.method()} ${new URL(req.url()).pathname} ${req.postData()}`);
    const ligado = JSON.parse(req.postData() ?? '{}').ligado;
    if (req.url().endsWith('/darede')) {
      return rota.fulfill({ status: 500, contentType: 'application/json', body: JSON.stringify({ mensagem: 'Falhou ao gravar.' }) });
    }
    return rota.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ ...exemplo, ligado }) });
  });

  await page.goto('/configuracoes?aba=modulos');
  const bloco = page.locator('section', { has: page.getByRole('heading', { name: 'Módulos' }) });
  const interruptor = bloco.getByRole('switch', { name: 'Exemplo' });
  await expect(interruptor).not.toBeChecked();
  await expect(bloco).toContainText('Não acessa a internet.');
  await expect(bloco).toContainText('Acessa a internet: a.example.com e b.example.com.');

  await interruptor.check();
  await expect(page.getByText('Exemplo ligado.')).toBeVisible();
  await expect(interruptor).toBeChecked();
  await interruptor.uncheck();
  await expect(page.getByText('Exemplo desligado. Os dados dele ficam guardados.')).toBeVisible();
  expect(pedidos).toEqual(['PUT /api/modulos/exemplo {"ligado":true}', 'PUT /api/modulos/exemplo {"ligado":false}']);

  // A API falhou: o interruptor volta ao que está gravado e o erro aparece.
  problemas.ignorar('500');
  const daRedeInterruptor = bloco.getByRole('switch', { name: 'Da rede' });
  await daRedeInterruptor.click();
  await expect(page.getByText('Falhou ao gravar.')).toBeVisible();
  await expect(daRedeInterruptor).not.toBeChecked();
});

test('Configurações › Módulos: sem módulo nesta versão', async ({ page }) => {
  await page.goto('/configuracoes?aba=modulos');
  await expect(page.getByText('Nenhum módulo nesta versão do site.')).toBeVisible();
});
