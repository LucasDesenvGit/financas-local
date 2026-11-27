import { expect, test } from './base';

/**
 * #68: Configurações › Parametrização. Mudar um parâmetro de segurança do login mostra o campo da senha atual, e o
 * salvar a manda junto; a senha que falta ou está errada aparece embaixo do campo. Os outros parâmetros não pedem.
 */
test('Configurações › Parametrização: mudar o bloqueio pede a senha atual', async ({ page, problemas }) => {
  // A resposta do salvar: a lista com os valores de sempre (a tela volta a 15 depois de cada salvar).
  const salvos = [
    { chave: 'Painel.DiasProximosVencimentos', grupo: 'Painel', nome: 'Próximos vencimentos', descricao: '', tipo: 1, valor: 15,
      padrao: 15, minimo: 1, maximo: 90, unidade: 'dias', pedeSenha: false },
    { chave: 'Seguranca.MinutosBloqueio', grupo: 'Segurança', nome: 'Tempo de bloqueio', descricao: '', tipo: 1, valor: 15,
      padrao: 15, minimo: 1, maximo: 1440, unidade: 'minutos', pedeSenha: true }
  ];
  const corpos: string[] = [];
  await page.route('**/api/parametros', async (rota) => {
    const req = rota.request();
    if (req.method() !== 'PUT') return rota.fallback();
    const corpo = req.postData() ?? '';
    corpos.push(corpo);
    if (corpo.includes('"senhaAtual":"errada"')) {
      return rota.fulfill({ status: 400, contentType: 'application/json', body: JSON.stringify({ mensagem: 'A senha atual não confere.' }) });
    }
    return rota.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(salvos) });
  });
  await page.goto('/configuracoes?aba=parametrizacao');
  const senha = page.getByLabel('Senha atual');
  const salvar = page.getByRole('button', { name: 'Salvar parâmetros' });

  // Só um parâmetro comum: salva sem pedir a senha.
  await page.getByLabel('Próximos vencimentos').fill('20');
  await expect(senha).toHaveCount(0);
  await salvar.click();
  await expect(page.getByText('Parâmetros salvos.')).toBeVisible();
  expect(corpos.shift()).toBe('{"valores":{"Painel.DiasProximosVencimentos":20}}');

  // O bloqueio: o campo aparece, e salvar sem a senha avisa embaixo dele, sem chamar a API.
  await page.getByLabel('Tempo de bloqueio').fill('30');
  await expect(senha).toBeVisible();
  await expect(senha).toHaveAttribute('type', 'password');
  await expect(senha).toHaveAttribute('autocomplete', 'current-password');
  await salvar.click();
  await expect(page.getByRole('alert').filter({ hasText: 'Informe a senha atual' })).toBeVisible();
  await expect(senha).toBeFocused();
  await expect(senha).toHaveAttribute('aria-invalid', 'true');
  expect(corpos).toEqual([]);

  // Senha errada: a mensagem da API embaixo do campo; digitar de novo a apaga.
  problemas.ignorar('400');
  await senha.fill('errada');
  await salvar.click();
  await expect(page.getByRole('alert').filter({ hasText: 'A senha atual não confere.' })).toBeVisible();
  await expect(senha).toBeFocused();
  await senha.fill('Senha-Forte-123');
  await expect(page.getByText('A senha atual não confere.')).toHaveCount(0);
  await salvar.click();
  // O aviso do primeiro salvar ainda pode estar na tela: a prova é o campo sumir (a lista nova voltou).
  await expect(senha).toHaveCount(0);
  expect(corpos).toEqual([
    '{"valores":{"Seguranca.MinutosBloqueio":30},"senhaAtual":"errada"}',
    '{"valores":{"Seguranca.MinutosBloqueio":30},"senhaAtual":"Senha-Forte-123"}'
  ]);

  // Voltar ao valor salvo também esconde o campo.
  await page.getByLabel('Tempo de bloqueio').fill('45');
  await expect(senha).toBeVisible();
  await page.getByLabel('Tempo de bloqueio').fill('15');
  await expect(senha).toHaveCount(0);
});
