import { abrirContas, fatura } from '../apoio/contas';

/**
 * Regressão da UX-12 (#88): "Faturas do mês" no Painel leva a /contas?faturas=ID, que abre direto o histórico de
 * faturas do cartão (detalhe por navegação, ADR 0018), sem a pessoa procurar o cartão. Desde a ADR 0024 o histórico
 * fica no próprio cartão, recolhido, em vez de numa janela.
 */
describe('Regressão: link para as faturas de um cartão', () => {
  const botao = (raiz: HTMLElement, id: number) =>
    raiz.querySelector<HTMLButtonElement>(`#cartao-${id} .botao-historico`)!;

  it('/contas?faturas=3 abre o histórico do Cartão Roxo e pede as faturas dele', async () => {
    const t = await abrirContas('/contas?faturas=3', undefined, { 3: [fatura()] });
    expect(t.pedidas).toEqual([3]);
    expect(botao(t.raiz, 3).getAttribute('aria-expanded')).toBe('true');
    expect(t.cartao(3).querySelector('.tabela-historico')).not.toBeNull();
    expect(botao(t.raiz, 2).getAttribute('aria-expanded')).toBe('false');
  });

  it('fechar o histórico tira o ?faturas= da URL (o F5 não reabre)', async () => {
    const t = await abrirContas('/contas?faturas=3');
    botao(t.raiz, 3).click();
    await t.atualizar();
    expect(botao(t.raiz, 3).getAttribute('aria-expanded')).toBe('false');
    expect(t.router.url).toBe('/contas');
  });

  it.each(['/contas?faturas=999', '/contas?faturas=1'])('%s (não existe ou não é cartão) é ignorado e a tela abre normal', async (url) => {
    const t = await abrirContas(url);
    expect(t.pedidas).toEqual([]);
    expect(t.raiz.querySelector('.botao-historico[aria-expanded="true"]')).toBeNull();
  });

  it('sem o parâmetro, nenhum histórico abre sozinho', async () => {
    const t = await abrirContas('/contas');
    expect(t.pedidas).toEqual([]);
    expect(t.raiz.querySelector('.botao-historico[aria-expanded="true"]')).toBeNull();
  });
});
