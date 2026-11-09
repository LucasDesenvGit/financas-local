import { expect, test } from './base';

/** A impressão digital tem 95 caracteres sem espaço: no celular, ela tem de quebrar em vez de empurrar a tela. */
const impressao = Array.from({ length: 32 }, (_, i) => (i * 7 % 256).toString(16).toUpperCase().padStart(2, '0')).join(':');

/**
 * #328 (ADR 0061): o quadro "Acesso pelo celular" em Configurações › Segurança (API falsa, IPs fictícios). Ligar pede
 * confirmação e mostra o endereço, os avisos, os passos e os QR codes (#677); desligar volta ao começo. Sem erro de CSP nem rolagem para o lado.
 */
test('Configurações › Segurança: ligar e desligar o acesso pelo celular', async ({ page, problemas }) => {
  const ligada = { ligado: true, ip: '192.168.0.10', endereco: 'https://192.168.0.10:5443', ipIndisponivel: false,
    impressaoDigital: impressao, avisos: ['O Windows marcou esta rede como Pública, e nela a porta continua fechada.'],
    enderecos: [{ ip: '192.168.0.10', interface: 'Wi-Fi', rede: '192.168.0.0/24' }] };
  const pedidos: string[] = [];
  await page.route('**/api/celular/ligar', (rota) => {
    pedidos.push(rota.request().postData() ?? '');
    return rota.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ligada) });
  });
  await page.route('**/api/celular/desligar', (rota) => rota.fulfill({ status: 200, contentType: 'application/json',
    body: JSON.stringify({ ...ligada, ligado: false, ip: null, endereco: null, impressaoDigital: null, avisos: [] }) }));

  await page.goto('/configuracoes?aba=seguranca');
  const quadro = page.locator('section.acesso-celular');
  await expect(quadro.getByRole('heading', { name: 'Acesso pelo celular' })).toBeVisible();
  const escolha = quadro.getByLabel('Endereço deste computador na rede');
  await expect(escolha).toHaveValue('192.168.0.10');
  await escolha.selectOption('10.0.0.20');

  await quadro.getByRole('button', { name: 'Ligar o acesso pelo celular' }).click();
  const janela = page.getByRole('alertdialog');
  await expect(janela).toContainText('10.0.0.20');
  await janela.getByRole('button', { name: 'Ligar' }).click();
  await expect(quadro.getByRole('status')).toContainText('https://192.168.0.10:5443');
  expect(pedidos).toEqual(['{"ip":"10.0.0.20"}']);
  await expect(quadro.locator('.aviso-inline')).toContainText('Pública');
  await expect(quadro.locator('.impressao-digital')).toHaveText(impressao);
  await expect(quadro.getByRole('link', { name: 'Baixar o certificado neste computador' })).toHaveAttribute('href', '/api/celular/autoridade');
  // #677: os dois QR codes aparecem, inteiros e no tamanho em que a câmera lê (168 px), sem erro de CSP.
  for (const nome of ['certificado', 'site']) {
    const qr = quadro.getByRole('img', { name: new RegExp(`QR code do endereço do ${nome}`) });
    await expect(qr).toBeVisible();
    const caixa = (await qr.boundingBox())!;
    expect(caixa.width, `QR do ${nome}`).toBeGreaterThanOrEqual(167.5);
    expect(caixa.height, `QR do ${nome}`).toBeGreaterThanOrEqual(167.5);
  }
  const sobra = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(sobra, 'a tela rola para o lado').toBeLessThanOrEqual(1);

  await quadro.getByRole('button', { name: 'Desligar o acesso pelo celular' }).click();
  await expect(quadro.getByRole('button', { name: 'Ligar o acesso pelo celular' })).toBeVisible();
  expect(problemas.lista).toEqual([]);
});
