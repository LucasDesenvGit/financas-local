import type { Page } from '@playwright/test';
import { expect, test } from './base';

/** Um plano do assistente com 10 anos de caixa igual (para a API falsa, que não tem histórico de caixa). */
async function comPlano(page: Page) {
  const hoje = new Date();
  const meses = Array.from({ length: 120 }, (_, k) => {
    const d = new Date(hoje.getFullYear(), hoje.getMonth() + k, 1);
    return { ano: d.getFullYear(), mes: d.getMonth() + 1, receitas: 10000, despesas: 4000, faturas: 2500, parcelas: 0, sobra: 3500 };
  });
  await page.route(/\/api\/metas\/plano$/, (r) => r.fulfill({ json: {
    disponivel: true, mesesConsiderados: 6, ano: hoje.getFullYear(), mes: hoje.getMonth() + 1, margemPercentual: 10,
    saldoContas: 9000, faturaAPagar: 0, saldoFimDoMes: 9000, melhorMesRecente: 5000, meses
  } }));
}

/**
 * #335: no card da meta, "Consigo guardar outro valor" fica na linha de "Este mês: guarde"; sem a lista "Depois:", o
 * plano mês a mês já vem aberto, com 5 meses e "Mostrar mais". Sem erro de console nem rolagem para o lado.
 */
test('Metas: o plano mês a mês aberto com 5 meses, e "Consigo guardar" na linha do valor do mês', async ({ page, problemas }) => {
  await comPlano(page);
  await page.goto('/metas');
  const viagem = page.locator('.cartao-meta').filter({ hasText: 'Viagem' });
  const linha = viagem.locator('.recomendado');
  await expect(linha).toContainText('Este mês: guarde');
  const [texto, botao] = [(await linha.locator('span').first().boundingBox())!, (await linha.getByRole('button', { name: 'Consigo guardar outro valor' }).boundingBox())!];
  if (page.viewportSize()!.width > 900) expect(Math.abs(texto.y - botao.y)).toBeLessThan(6);
  await expect(viagem.locator('.degraus')).toHaveCount(0);
  // A seta da barra fica na linha do nome, também no celular (não sozinha numa linha).
  const [nome, seta] = [(await viagem.locator('.meta-nome').boundingBox())!, (await viagem.locator('.meta-seta').boundingBox())!];
  expect(Math.abs((nome.y + nome.height / 2) - (seta.y + seta.height / 2))).toBeLessThan(12);
  await expect(viagem.getByRole('button', { name: /Plano mês a mês/ })).toHaveAttribute('aria-expanded', 'true');
  await expect(viagem.locator('.tabela-plano tbody tr')).toHaveCount(5);
  await viagem.getByRole('button', { name: 'Mostrar mais 5 meses' }).click();
  await expect(viagem.locator('.tabela-plano tbody tr')).toHaveCount(10);
  const sobra = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(sobra).toBeLessThanOrEqual(1);
  expect(problemas.lista).toEqual([]);
});

/**
 * #359: a recomendação do topo é uma caixa em destaque, numa linha só no computador (quebra só quando não cabe), com o
 * valor em negrito.
 */
test('Metas: o aviso "Recomendo guardar" numa caixa e numa linha só', async ({ page, problemas }) => {
  await comPlano(page);
  await page.goto('/metas');
  const aviso = page.locator('.aviso-recomendacao');
  await expect(aviso).toContainText('Recomendo guardar');
  await expect(aviso.locator('strong').first()).toHaveText(/R\$/);
  const { altura, linha } = await aviso.evaluate((el) => {
    const css = getComputedStyle(el);
    return { altura: el.clientHeight - parseFloat(css.paddingTop) - parseFloat(css.paddingBottom), linha: parseFloat(css.lineHeight) };
  });
  if (page.viewportSize()!.width > 900) expect(altura).toBeLessThan(linha * 1.5);
  expect(problemas.lista).toEqual([]);
});

/**
 * #359: valor por mês maior que a sobra para as metas não passa: o erro aparece ao digitar e "Salvar" fica desabilitado.
 * Valor gravado antes do bloqueio mostra o aviso amarelo (caixa com a barra à esquerda e o essencial em negrito).
 */
test('Metas: valor por mês acima da sobra é bloqueado no formulário, e o aviso amarelo mostra o que importa', async ({ page, problemas }) => {
  await comPlano(page);
  const ano = new Date().getFullYear();
  await page.route(/\/api\/metas(\?|$)/, (r) => r.request().method() === 'GET' ? r.fulfill({ json: [
    { id: 6, nome: 'Teste', valorAlvo: 50000, valorAtual: 0, dataAlvo: `${ano}-12-01`, contaId: null, contaNome: null, cor: '#f0d58c',
      concluida: false, percentual: 0, aporteMensalSugerido: null, valorInicial: 0, saldoConta: 0, reservaNome: null, saldoReserva: null,
      reservaConferir: false, reservaNaoEncontrada: false, prioridade: 1, icone: null, guardarPorMes: 22200 }
  ] }) : r.fallback());
  await page.goto('/metas');
  const avisos = page.locator('.cartao-meta .aviso-prazo');
  await expect(avisos.first()).toContainText('Você escolheu guardar');
  await expect(avisos.first().locator('strong').first()).toHaveText(/R\$\s22\.200,00 por mês/);
  const estilo = await avisos.first().evaluate((el) => getComputedStyle(el).borderLeftWidth);
  expect(estilo).toBe('4px');

  await page.getByRole('button', { name: /Nova meta/ }).click();
  const dialogo = page.getByRole('dialog');
  await dialogo.locator('input[name="nome"]').fill('Outra');
  await dialogo.locator('app-campo-valor[name="alvo"] input').fill('1000');
  // A outra meta já guarda R$ 22.200 (mais que a sobra): não sobra nada para esta.
  await dialogo.locator('app-campo-valor[name="porMes"] input').fill('100');
  await expect(dialogo.locator('app-campo-valor[name="porMes"] .campo-valor-mensagem')).toContainText('Não sobra nada para as metas');
  await expect(dialogo.getByRole('button', { name: 'Salvar' })).toBeDisabled();
  await expect(dialogo.locator('.nota-por-mes')).toHaveCount(0);
  expect(problemas.lista).toEqual([]);
});
