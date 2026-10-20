import { abrirContas } from '../apoio/contas';

/**
 * Regressão #130: "Mostrar arquivadas" guarda o estado na URL (?arquivadas=1, com replaceUrl), como os filtros
 * (ADR 0018): recarregar, voltar ou abrir um link salvo mostra a tela do mesmo jeito.
 */
describe('Regressão: "Mostrar arquivadas" na URL', () => {
  const campo = (raiz: HTMLElement) => raiz.querySelector<HTMLInputElement>('label.interruptor input')!;

  it('ligar recarrega pedindo também as arquivadas e põe ?arquivadas=1 na URL, sem nova entrada no histórico', async () => {
    const t = await abrirContas();
    expect(t.cargas).toEqual([false]);
    const navegar = jest.spyOn(t.router, 'navigate');
    campo(t.raiz).click();
    await t.atualizar();
    expect(t.cargas).toEqual([false, true]);
    expect(t.router.url).toBe('/contas?arquivadas=1');
    expect(navegar.mock.calls.at(-1)![1]).toMatchObject({ replaceUrl: true });
  });

  it('desligar tira o parâmetro da URL', async () => {
    const t = await abrirContas('/contas?arquivadas=1');
    campo(t.raiz).click();
    await t.atualizar();
    expect(t.router.url).toBe('/contas');
    expect(t.cargas.at(-1)).toBe(false);
  });

  it('abrir /contas?arquivadas=1 já vem com o interruptor ligado e as arquivadas carregadas', async () => {
    const t = await abrirContas('/contas?arquivadas=1');
    expect(campo(t.raiz).checked).toBe(true);
    expect(t.cargas).toEqual([true]);
  });

  it('o ?faturas= continua funcionando junto (o merge preserva os outros parâmetros)', async () => {
    const t = await abrirContas('/contas?faturas=2');
    campo(t.raiz).click();
    await t.atualizar();
    expect(t.router.url).toContain('faturas=2');
    expect(t.router.url).toContain('arquivadas=1');
  });
});
