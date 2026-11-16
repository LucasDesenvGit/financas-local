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

/**
 * #274 e #379: o selo da cobrança fica numa coluna, e o nome, a conta (com o selo do banco) e a forma de cobrança ficam
 * empilhados ao lado, começando no mesmo ponto; a conta não encosta no nome.
 */
test('Contas a pagar: nome, conta e forma de cobrança alinhados depois do selo', async ({ page }) => {
  await page.goto('/contas-a-pagar');
  const linha = page.locator('table').first().locator('tbody tr').filter({ has: page.locator('.conta-da-cobranca') }).first();
  const [selo, nome, conta, forma] = await Promise.all(['app-selo-servico', '.nome-cobranca', '.conta-da-cobranca', '.forma-cobranca']
    .map(async (s) => (await linha.locator(s).first().boundingBox())!));
  expect(Math.abs(conta.x - nome.x)).toBeLessThan(0.5);
  expect(Math.abs(forma.x - nome.x)).toBeLessThan(0.5);
  expect(nome.x - (selo.x + selo.width)).toBeGreaterThanOrEqual(6);
  // A conta fica embaixo do nome, sem encostar nele.
  expect(conta.y).toBeGreaterThanOrEqual(nome.y + nome.height - 0.5);
});

/**
 * #691 (ADR 0063): a conta paga com o cofrinho diz de onde sai o dinheiro e a última baixa que o app deu, com "ajustar
 * valor" e "desfazer"; o formulário mostra o "Pago com" só em conta do Open Finance, fora do cartão.
 */
test('Contas a pagar: pago com o cofrinho (#691)', async ({ page, api, problemas }) => {
  await page.goto('/contas-a-pagar');
  const condominio = page.locator('tbody tr').filter({ hasText: 'Condomínio' });
  await expect(condominio.getByText('cofrinho Emergência (Conta Exemplo)')).toBeVisible();
  await expect(page.getByText(/paga pelo cofrinho Emergência em \d\d\/\d\d \(automático\)/)).toBeVisible();
  await expect(page.getByRole('button', { name: 'Pagar Condomínio' })).toHaveCount(0);

  await page.getByRole('button', { name: 'Ajustar o valor pago de Condomínio' }).click();
  const ajuste = page.getByRole('dialog');
  await ajuste.getByRole('button', { name: 'Salvar' }).click();
  await expect.poll(() => api.chamadas.includes('PUT /api/contas-agendadas/4/pagamento-cofrinho')).toBe(true);

  await page.getByRole('button', { name: '+ Nova conta' }).click();
  const dialogo = page.getByRole('dialog');
  await expect(dialogo.getByLabel('Pago com')).toHaveCount(0);
  await dialogo.getByLabel('Conta de pagamento (opcional)').selectOption({ label: 'Conta Exemplo' });
  await dialogo.getByLabel('Pago com').selectOption({ label: 'Cofrinho Emergência' });
  await expect(dialogo).toContainText('o “Pagar” lança a despesa e baixa o cofrinho');
  await dialogo.getByLabel('Pago com').selectOption({ label: 'Outro cofrinho…' });
  await expect(dialogo.getByLabel('Nome do cofrinho')).toBeVisible();
  await dialogo.getByLabel('Como é cobrada').selectOption({ label: 'no cartão de crédito' });
  await expect(dialogo.getByLabel('Pago com')).toHaveCount(0);
  expect(problemas.lista).toEqual([]);
});

/**
 * #691, parte 2: o aviso "não veio" pergunta se foi paga com um cofrinho e abre o "Pagar" com ele escolhido; o
 * pagamento sem despesa oferece "lançar pelo cofrinho".
 */
test('Contas a pagar: foi paga com um cofrinho e o pagamento sem despesa (#691)', async ({ page, api, problemas }) => {
  await page.goto('/contas-a-pagar');
  await page.getByRole('button', { name: 'Foi paga com um cofrinho: Luz' }).click();
  const dialogo = page.getByRole('dialog', { name: 'Pagar · Luz' });
  await expect(dialogo.getByLabel('Qual cofrinho')).toHaveValue(/Emergência/);
  await expect(dialogo.getByLabel('Usar este cofrinho nas próximas (o app lança no vencimento)')).toBeChecked();
  await dialogo.getByRole('button', { name: 'Confirmar pagamento' }).click();
  await expect.poll(() => api.chamadas.includes('POST /api/contas-agendadas/2/pagar')).toBe(true);

  await expect(page.getByText(/o pagamento de \d\d\/\d\d está sem a despesa/)).toBeVisible();
  await page.getByRole('button', { name: 'Lançar pelo cofrinho a despesa de Internet' }).click();
  await page.getByRole('alertdialog', { name: 'Lançar pelo cofrinho' }).getByRole('button', { name: 'Lançar', exact: true }).click();
  await expect.poll(() => api.chamadas.includes('POST /api/reservas/saida')).toBe(true);
  expect(problemas.lista).toEqual([]);
});
