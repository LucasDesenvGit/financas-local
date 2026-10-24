import { TestBed } from '@angular/core/testing';
import { IconeCategoriaComponent, iconeDaCategoria } from '../../src/app/compartilhado/icone-categoria.component';
import { IconesCategoria } from '../../src/app/compartilhado/icones';
import { COR_OUTRAS, fatiasRosca } from '../../src/app/compartilhado/rosca-categorias';
import { abrirPainel, painel } from '../apoio/painel';

/**
 * #336: o ícone da categoria (na cor dela) no lugar da bolinha, e a rosca de categorias com as fatias menores que 3%
 * juntas em "Outras".
 */
describe('Regressão: ícone da categoria e rosca de categorias (#336)', () => {
  it('o ícone vem pelo nome, sem acento nem maiúscula; categoria sem ícone conhecido: null', () => {
    expect(iconeDaCategoria('Alimentação')).toBe(IconesCategoria['ALIMENTACAO']);
    expect(iconeDaCategoria('serviços e assinaturas')).toBe(IconesCategoria['SERVICOS E ASSINATURAS']);
    expect(iconeDaCategoria('Academia')).toBeNull();
    expect(iconeDaCategoria(null)).toBeNull();
  });

  it('sem ícone conhecido, mostra a primeira letra; é decorativo (aria-hidden) e leva a cor da categoria', () => {
    const f = TestBed.createComponent(IconeCategoriaComponent);
    f.componentRef.setInput('nome', 'academia');
    f.componentRef.setInput('cor', '#f2d98b');
    f.detectChanges();
    const caixa = (f.nativeElement as HTMLElement).querySelector<HTMLElement>('.icone-categoria')!;
    expect(caixa.getAttribute('aria-hidden')).toBe('true');
    expect(caixa.textContent!.trim()).toBe('A');
    expect(caixa.style.getPropertyValue('--cor')).toBe('#f2d98b');
    f.componentRef.setInput('nome', 'Mercado');
    f.detectChanges();
    expect(caixa.querySelector('.inicial')).toBeNull();
    expect(caixa.querySelector('app-icone')).not.toBeNull();
  });

  it('as fatias menores que 3% ficam juntas em "Outras", no fim, e cada fatia sabe de qual categoria é', () => {
    const itens = [
      { nome: 'Moradia', valor: 2000, cor: '#d4a5e8' },
      { nome: 'Mercado', valor: 1500, cor: '#f2d98b' },
      { nome: 'Pets', valor: 60, cor: '#c9b3f5' },
      { nome: 'Lazer', valor: 400, cor: '#f3b3d3' },
      { nome: 'Doações', valor: 40, cor: '#b5cf9a' }
    ];
    const f = fatiasRosca(itens);
    // Total 4.000: Pets (1,5%) e Doações (1%) viram "Outras" = 100.
    expect(f.rotulos).toEqual(['Moradia', 'Mercado', 'Lazer', 'Outras']);
    expect(f.valores).toEqual([2000, 1500, 400, 100]);
    expect(f.cores[3]).toBe(COR_OUTRAS);
    expect(f.indices).toEqual([0, 1, 3, -1]);
  });

  it('sem fatia pequena, não há "Outras"; valor zero ou negativo não vira fatia', () => {
    const f = fatiasRosca([{ nome: 'A', valor: 10, cor: '#000000' }, { nome: 'B', valor: 0, cor: '#000000' }]);
    expect(f.rotulos).toEqual(['A']);
  });
});

describe('Regressão: Painel com o ícone das categorias e a rosca nova (#336)', () => {
  beforeEach(() => {
    localStorage.clear();
    // O gráfico usa o tema, que lê matchMedia (o jsdom não tem).
    Object.defineProperty(window, 'matchMedia', {
      configurable: true, writable: true,
      value: () => ({ matches: false, addEventListener: () => undefined, removeEventListener: () => undefined })
    });
  });

  const gastos = Array.from({ length: 10 }, (_, k) => ({ categoriaId: k + 1, nome: `Categoria ${k + 1}`, cor: '#92c8f0', valor: 1000 - k * 100 }))
    .concat([{ categoriaId: 99, nome: 'Pets', cor: '#c9b3f5', valor: 20 }]);

  it('cada categoria da lista tem o ícone e a barrinha; "Mostrar todas" abre o resto; o total vai no centro', async () => {
    const total = gastos.reduce((s, g) => s + g.valor, 0);
    const { raiz, atualizar } = await abrirPainel(painel({ gastosPorCategoria: gastos, despesasMes: total }));
    const linhas = () => raiz.querySelectorAll('.lista-categorias li');
    expect(linhas()).toHaveLength(8);
    expect(linhas()[0].querySelector('app-icone-categoria')).not.toBeNull();
    expect(linhas()[0].querySelector('.trilho .progresso')).not.toBeNull();
    expect(raiz.querySelector('.ponto')).toBeNull();
    expect(raiz.querySelector('.rosca-centro')!.textContent!.replace(/\s+/g, ' ')).toContain('em despesas');
    const botao = raiz.querySelector<HTMLButtonElement>('.mais-categorias')!;
    expect(botao.textContent!.trim()).toBe('Mostrar todas as 11 categorias');
    expect(botao.getAttribute('aria-expanded')).toBe('false');
    botao.click();
    await atualizar();
    expect(linhas()).toHaveLength(11);
    expect(botao.getAttribute('aria-expanded')).toBe('true');
  });

  it('a rosca junta as pequenas em "Outras"; clicar em "Outras" abre as despesas do mês, e numa categoria, a dela', async () => {
    const total = gastos.reduce((s, g) => s + g.valor, 0);
    const { tela, router } = await abrirPainel(painel({ gastosPorCategoria: gastos, despesasMes: total }), '/painel?mes=2026-10');
    const cfg = tela.graficoCategorias();
    expect(cfg.data.labels![cfg.data.labels!.length - 1]).toBe('Outras');
    const navegar = jest.spyOn(router, 'navigate').mockResolvedValue(true);
    tela.abrirCategoria(cfg.data.labels!.length - 1);
    expect(navegar.mock.calls[0][1]!.queryParams).not.toHaveProperty('categoria');
    expect(navegar.mock.calls[0][1]!.queryParams).toMatchObject({ tipo: 'despesa' });
    tela.abrirCategoria(0);
    expect(navegar.mock.calls[1][1]!.queryParams).toMatchObject({ categoria: '1' });
  });
});
