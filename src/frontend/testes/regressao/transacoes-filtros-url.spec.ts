import { TipoTransacao } from '../../src/app/core/modelos';
import { primeiroDiaMes, ultimoDiaMes } from '../../src/app/compartilhado/utilidades';
import { abrirTransacoes, paginaVazia, parametrosDaUrl } from '../apoio/transacoes';

/**
 * Regressão dos filtros de Transações pela URL: UX-1 (#31, outra tela abre Transações já filtrada) e
 * UX-5 (#35, o filtro vai para a URL: F5 e link salvo mantêm a lista, voltar do navegador não volta filtro).
 */
describe('Regressão: filtros de Transações na URL', () => {
  const hoje = new Date();
  const mesAtual = { inicio: primeiroDiaMes(hoje.getFullYear(), hoje.getMonth() + 1), fim: ultimoDiaMes(hoje.getFullYear(), hoje.getMonth() + 1) };

  it('abre já filtrada com todos os parâmetros da URL, numa busca só', async () => {
    const t = await abrirTransacoes('/transacoes?categoria=12&conta=3&tipo=despesa&inicio=2026-09-01&fim=2026-09-30&emergencias=1&busca=mercado');
    expect(t.buscas).toHaveLength(1);
    expect(t.buscas[0]).toMatchObject({
      categoriaId: 12, semCategoria: false, contaId: 3, tipo: TipoTransacao.Despesa,
      inicio: '2026-09-01', fim: '2026-09-30', somenteEmergencias: true, busca: 'mercado', pagina: 1
    });
  });

  it('categoria=sem filtra "Sem categoria"', async () => {
    const t = await abrirTransacoes('/transacoes?categoria=sem');
    expect(t.buscas[0]).toMatchObject({ semCategoria: true, categoriaId: null });
  });

  it('ignora cada parâmetro inválido sozinho e limpa a URL', async () => {
    const t = await abrirTransacoes('/transacoes?pagina=abc&busca=%20%20&emergencias=talvez&inicio=2026-02-30&tipo=xyz&categoria=999&conta=77');
    expect(t.buscas[0]).toMatchObject({
      ...mesAtual, categoriaId: null, semCategoria: false, contaId: null, tipo: null, somenteEmergencias: false, pagina: 1
    });
    expect(t.buscas[0].busca ?? '').toBe('');
    expect(t.router.url).toBe('/transacoes');
  });

  it('só uma data válida: a outra acompanha o mês dela (sem período invertido)', async () => {
    const t = await abrirTransacoes('/transacoes?inicio=2030-05-10');
    expect(t.buscas[0]).toMatchObject({ inicio: '2030-05-10', fim: '2030-05-31' });
  });

  it('mudar o filtro grava na URL com replaceUrl (voltar do navegador leva à tela anterior)', async () => {
    const t = await abrirTransacoes();
    const navegar = jest.spyOn(t.router, 'navigate');
    t.tela.filtro.tipo = TipoTransacao.Receita;
    t.tela.buscar(1);
    await t.assentar();
    expect(navegar).toHaveBeenLastCalledWith([], expect.objectContaining({ replaceUrl: true, queryParams: { tipo: 'receita' } }));
    expect(parametrosDaUrl(t.router)).toEqual({ tipo: 'receita' });
  });

  it('o período padrão (mês atual) não vai para a URL; outro período vai', async () => {
    const t = await abrirTransacoes();
    expect(t.router.url).toBe('/transacoes');
    t.tela.filtro.inicio = '2026-01-01';
    t.tela.filtro.fim = '2026-03-31';
    t.tela.buscar(1);
    await t.assentar();
    expect(parametrosDaUrl(t.router)).toEqual({ inicio: '2026-01-01', fim: '2026-03-31' });
  });

  it('busca textual vai para a URL só no Enter, não a cada tecla', async () => {
    const t = await abrirTransacoes();
    const campo = t.elemento.querySelector<HTMLInputElement>('input[type=search]')!;
    campo.value = 'uber';
    campo.dispatchEvent(new Event('input'));
    await t.assentar();
    expect(parametrosDaUrl(t.router)).toEqual({});
    expect(t.buscas).toHaveLength(1);

    campo.dispatchEvent(new KeyboardEvent('keyup', { key: 'Enter' }));
    await t.assentar();
    expect(parametrosDaUrl(t.router)).toEqual({ busca: 'uber' });
    expect(t.buscas.at(-1)).toMatchObject({ busca: 'uber', pagina: 1 });
  });

  it('página da URL é respeitada quando existe', async () => {
    const t = await abrirTransacoes('/transacoes?pagina=2', (f) => ({ ...paginaVazia(f, 120), itens: [{ id: 1 }] as never }));
    expect(t.buscas.map((b) => b.pagina)).toEqual([2]);
    expect(parametrosDaUrl(t.router)).toEqual({ pagina: '2' });
  });

  it('link antigo com página que não existe mais cai na última página, em vez de lista vazia', async () => {
    const t = await abrirTransacoes('/transacoes?categoria=sem&pagina=2', (f) => paginaVazia(f, 18));
    expect(t.buscas.map((b) => b.pagina)).toEqual([2, 1]);
    expect(parametrosDaUrl(t.router)).toEqual({ categoria: 'sem' });
  });

  it('"Limpar filtros" volta ao padrão e limpa a URL', async () => {
    const t = await abrirTransacoes('/transacoes?tipo=despesa&busca=mercado&categoria=12');
    t.tela.limparFiltros();
    await t.assentar();
    expect(t.buscas.at(-1)).toMatchObject({ ...mesAtual, tipo: null, busca: '', categoriaId: null, semCategoria: false });
    expect(t.router.url).toBe('/transacoes');
  });
});
