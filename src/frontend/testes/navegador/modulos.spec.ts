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
  await page.route('**/api/modulos', (r) => r.fulfill({ status: 200, contentType: 'application/json', body: '[]' }));
  await page.goto('/configuracoes?aba=modulos');
  await expect(page.getByText('Nenhum módulo nesta versão do site.')).toBeVisible();
});

// Revisão do #697: a lista de módulos que não veio (o site reiniciando) mostra o erro e "Tentar de novo", e não fica
// presa até recarregar a página.
test('Configurações › Módulos: a lista que não veio, e tentar de novo', async ({ page, problemas }) => {
  problemas.ignorar('503');
  const exemplo = { id: 'exemplo', nome: 'Exemplo', descricao: 'Um módulo de exemplo.', acessaInternet: false, destinos: [], ligado: false };
  let falhar = true;
  await page.route('**/api/modulos', (rota) => falhar
    ? rota.fulfill({ status: 503, contentType: 'application/json', body: '{}' })
    : rota.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([exemplo]) }));
  await page.goto('/configuracoes?aba=modulos');
  const bloco = page.locator('section', { has: page.getByRole('heading', { name: 'Módulos' }) });
  await expect(bloco.getByRole('alert')).toContainText('Não deu para ler os módulos agora');
  await expect(bloco).not.toContainText('Nenhum módulo nesta versão');
  falhar = false;
  await bloco.getByRole('button', { name: 'Tentar de novo' }).click();
  await expect(bloco.getByRole('switch', { name: 'Exemplo' })).toBeVisible();
});

/**
 * #750: a lista que não veio aparece também no menu, para quem não abre Configurações › Módulos nem a tela de um
 * módulo. O "Tentar de novo" de lá diz "Tentando…" enquanto lê, e o grupo do módulo volta ao menu quando dá certo.
 */
test('Menu: a lista de módulos que não veio, e tentar de novo ali', async ({ page, problemas, isMobile }) => {
  problemas.ignorar('503');
  const trabalho = { id: 'trabalho', nome: 'Trabalho', descricao: 'Salário e holerites.', acessaInternet: false, destinos: [], ligado: true };
  let falhar = true;
  let responder: (() => void) | null = null;
  await page.route('**/api/modulos', async (rota) => {
    if (falhar) return rota.fulfill({ status: 503, contentType: 'application/json', body: '{}' });
    await new Promise<void>((r) => (responder = r)); // segura a resposta: dá para ver o "Tentando…"
    return rota.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([trabalho]) });
  });
  await page.goto('/painel');
  if (isMobile) await page.getByRole('button', { name: 'Menu de navegação' }).click(); // o aviso vai na gaveta
  const menu = page.getByRole('navigation', { name: 'Menu principal' });
  const aviso = menu.getByRole('status');
  await expect(aviso).toContainText('Não deu para ler os módulos: as telas deles estão fora do menu.');
  await expect(menu.getByRole('button', { name: 'Trabalho' })).toHaveCount(0);
  falhar = false;
  await aviso.getByRole('button', { name: 'Tentar de novo' }).click();
  await expect(aviso.getByRole('button', { name: 'Tentando…' })).toBeDisabled();
  await expect.poll(() => responder !== null).toBe(true);
  responder!();
  await expect(aviso).toHaveCount(0);
  await expect(menu.getByRole('button', { name: 'Trabalho' })).toBeVisible();
});
