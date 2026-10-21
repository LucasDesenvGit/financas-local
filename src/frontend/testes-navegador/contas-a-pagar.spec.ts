import { expect, test } from './base';

/**
 * #158: Contas a pagar no navegador, com a API falsa: a conta cobrada sozinha diz onde cai e não tem "Pagar"; as
 * sugestões do cartão (mensal e anual) dizem como são cobradas; o formulário mostra o texto da cobrança só para a
 * cobrada sozinha.
 */
test('Contas a pagar: forma de cobrança, baixa sozinha e sugestões do cartão', async ({ page, problemas }) => {
  await page.goto('/contas-a-pagar');
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Contas a pagar');

  await expect(page.getByRole('button', { name: 'Pagar Escola' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Pagar Luz' })).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Pagar Streaming' })).toHaveCount(0);
  await expect(page.getByText('baixa sozinha quando a cobrança com “STREAMING” chega')).toBeVisible();

  const sugestoes = page.locator('section.sugestoes');
  await expect(sugestoes).toContainText('no cartão de crédito');
  await expect(sugestoes).toContainText('anual: pago em');

  await page.getByRole('button', { name: 'Cadastrar MUSICA APP' }).click();
  const dialogo = page.getByRole('dialog');
  await expect(dialogo.getByLabel('Como é cobrada')).toBeVisible();
  await expect(dialogo.getByLabel('Texto da cobrança no extrato')).toHaveValue('MUSICA');
  await dialogo.getByLabel('Como é cobrada').selectOption({ label: 'paga à mão (boleto, Pix)' });
  await expect(dialogo.getByLabel('Texto da cobrança no extrato')).toHaveCount(0);
  expect(problemas.lista).toEqual([]);
});

test('Contas a pagar: aviso de aumento com "Atualizar o valor" (#158)', async ({ page, api, problemas }) => {
  await page.goto('/contas-a-pagar');
  const avisos = page.locator('section.avisos-recorrentes');
  await expect(avisos).toContainText('Streaming aumentou na última cobrança.');
  await avisos.getByRole('button', { name: 'Atualizar o valor de Streaming' }).click();
  await expect.poll(() => api.chamadas.some((c) => c.startsWith('PUT /api/contas-agendadas/3'))).toBe(true);
  expect(problemas.lista).toEqual([]);
});

test('Contas a pagar: sugestão parecida com outra conta a pagar tem "é a mesma" (#205)', async ({ page, api, problemas }) => {
  await page.goto('/contas-a-pagar');
  const sugestoes = page.locator('section.sugestoes');
  await expect(sugestoes).toContainText('parece ser Escola, que já está na lista');
  const botao = sugestoes.getByRole('button', { name: 'é a mesma: PIX ENVIADO PESSOA EXEMPLO é Escola' });
  // Por teclado (WCAG 2.1.1): foco no botão e Enter.
  await botao.focus();
  await page.keyboard.press('Enter');
  await expect.poll(() => api.chamadas.includes('POST /api/contas-agendadas/1/ligar-extrato')).toBe(true);
  await expect(sugestoes).not.toContainText('PIX ENVIADO PESSOA EXEMPLO');
  expect(problemas.lista).toEqual([]);
});

/**
 * #265: cada cobrança tem um selo antes do nome: o da marca, quando o serviço é conhecido (YouTube, na API falsa), ou o
 * ícone da categoria num quadrado neutro. Sem imagem de fora (a CSP continua; erro de CSP derruba o teste).
 */
test('Contas a pagar: selo de cada cobrança, da marca ou da categoria', async ({ page, problemas }) => {
  await page.goto('/contas-a-pagar');
  const linhas = page.locator('table').first().locator('tbody tr');
  await expect(linhas.first()).toBeVisible();
  for (const linha of await linhas.all()) await expect(linha.locator('app-selo-servico')).toBeVisible();

  const youtube = page.locator('section.sugestoes tbody tr').filter({ hasText: 'Google YouTubePremium' });
  await expect(youtube.locator('app-selo-servico svg.logo-banco rect')).toHaveAttribute('fill', '#FF0000');

  await page.getByRole('tab', { name: 'Assinaturas e recorrentes' }).click();
  await expect(page.locator('.grupo-recorrentes tbody tr app-selo-servico').first()).toBeVisible();
  expect(problemas.lista).toEqual([]);
});

/** #274: o selo do banco (com o nome da conta) não encosta no nome da cobrança. */
test('Contas a pagar: espaço entre o nome da cobrança e o selo do banco', async ({ page }) => {
  await page.goto('/contas-a-pagar');
  const linha = page.locator('table').first().locator('tbody tr').filter({ has: page.locator('.conta-da-cobranca') }).first();
  const nome = (await linha.locator('.nome-cobranca').boundingBox())!;
  const conta = (await linha.locator('.conta-da-cobranca').boundingBox())!;
  // Na mesma linha, a conta começa pelo menos 6 px depois do fim do nome (antes, 0 a 3 px).
  if (Math.abs(conta.y - nome.y) < nome.height) expect(conta.x - (nome.x + nome.width)).toBeGreaterThanOrEqual(6);
});
