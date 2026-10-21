import { FiltroTransacoes, PaginaTransacoes, Transacao } from '../../src/app/core/modelos';
import { abrirTransacoes, paginaVazia, parametrosDaUrl } from '../apoio/transacoes';

/**
 * #244: ordenar a lista de Transações pelo cabeçalho (Data, Categoria, Conta, Valor; Descrição não). A ordem vai para a
 * API (a lista é paginada lá), para a URL (F5 e link salvo mantêm) e volta à página 1. Cabeçalho é botão, e o th da
 * coluna que vale tem aria-sort (WAI-ARIA APG, Sortable Table; WCAG 2.1.1).
 */
const UMA = {
  id: 1, contaId: 3, contaNome: 'Nubank', openFinance: false, data: '2026-10-10', descricao: 'Mercado do bairro',
  descricaoOriginal: null, estabelecimento: null, valor: -50, tipo: 1, status: 1, categoriaId: 12, categoriaNome: 'Mercado',
  categoriaCor: '#f2d98b', parcelaNumero: null, parcelaTotal: null, ignorarRelatorios: false, observacao: null,
  emergencia: false, tipoLancamento: 0
} as unknown as Transacao;

const comUma = (f: FiltroTransacoes): PaginaTransacoes => ({ ...paginaVazia(f, 120), itens: [UMA] });

function cabecalho(elemento: HTMLElement, rotulo: string) {
  const botao = [...elemento.querySelectorAll<HTMLButtonElement>('thead .botao-ordenar')]
    .find((b) => b.textContent!.trim().startsWith(rotulo));
  return { botao: botao!, th: botao?.closest('th') ?? null };
}

describe('Regressão: ordenar Transações pelo cabeçalho (#244)', () => {
  it('padrão: data, mais recente primeiro; sem ordem na URL nem na API; só Data tem aria-sort', async () => {
    const t = await abrirTransacoes('/transacoes', comUma);
    expect(t.buscas.at(-1)!.ordem ?? null).toBeNull();
    expect(cabecalho(t.elemento, 'Data').th!.getAttribute('aria-sort')).toBe('descending');
    expect(cabecalho(t.elemento, 'Valor').th!.hasAttribute('aria-sort')).toBe(false);
    expect(parametrosDaUrl(t.router)['ordem']).toBeUndefined();
    // Descrição não ordena.
    const thDescricao = [...t.elemento.querySelectorAll('thead th')].find((th) => th.textContent!.trim() === 'Descrição')!;
    expect(thDescricao.querySelector('button')).toBeNull();
  });

  it('clicar em Valor ordena pelo maior primeiro, na API e na URL, e volta à página 1; clicar de novo inverte', async () => {
    const t = await abrirTransacoes('/transacoes?pagina=2', comUma);
    expect(t.buscas.at(-1)!.pagina).toBe(2);

    cabecalho(t.elemento, 'Valor').botao.click();
    await t.assentar();
    expect(t.buscas.at(-1)).toMatchObject({ ordem: 'valor', direcao: 'desc', pagina: 1 });
    expect(parametrosDaUrl(t.router)).toMatchObject({ ordem: 'valor', direcao: 'desc' });
    expect(parametrosDaUrl(t.router)['pagina']).toBeUndefined();
    expect(cabecalho(t.elemento, 'Valor').th!.getAttribute('aria-sort')).toBe('descending');
    expect(cabecalho(t.elemento, 'Data').th!.hasAttribute('aria-sort')).toBe(false);

    cabecalho(t.elemento, 'Valor').botao.click();
    await t.assentar();
    expect(t.buscas.at(-1)).toMatchObject({ ordem: 'valor', direcao: 'asc' });
    expect(cabecalho(t.elemento, 'Valor').th!.getAttribute('aria-sort')).toBe('ascending');
  });

  it('Categoria e Conta começam de A a Z', async () => {
    const t = await abrirTransacoes('/transacoes', comUma);
    cabecalho(t.elemento, 'Conta').botao.click();
    await t.assentar();
    expect(t.buscas.at(-1)).toMatchObject({ ordem: 'conta', direcao: 'asc' });
    cabecalho(t.elemento, 'Categoria').botao.click();
    await t.assentar();
    expect(t.buscas.at(-1)).toMatchObject({ ordem: 'categoria', direcao: 'asc' });
  });

  it('a ordem da URL vale ao abrir (F5, link salvo); ordem inválida é ignorada', async () => {
    const t = await abrirTransacoes('/transacoes?ordem=conta&direcao=desc', comUma);
    expect(t.buscas.at(-1)).toMatchObject({ ordem: 'conta', direcao: 'desc' });
    expect(cabecalho(t.elemento, 'Conta').th!.getAttribute('aria-sort')).toBe('descending');
  });

  it('ordem inválida na URL fica com o padrão', async () => {
    const t = await abrirTransacoes('/transacoes?ordem=descricao;drop&direcao=xyz', comUma);
    expect(t.buscas.at(-1)!.ordem ?? null).toBeNull();
    expect(cabecalho(t.elemento, 'Data').th!.getAttribute('aria-sort')).toBe('descending');
  });
});
