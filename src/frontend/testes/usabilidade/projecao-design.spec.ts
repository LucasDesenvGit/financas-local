import { TestBed } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';
import { of } from 'rxjs';
import { ApiService } from '../../src/app/core/api.service';
import { PrivacidadeService } from '../../src/app/core/privacidade.service';
import { Projecao, ProjecaoMes } from '../../src/app/core/modelos';
import { ProjecaoComponent } from '../../src/app/paginas/projecao.component';

/**
 * #314: Projeção.
 * - O mês em que o saldo vira ganha um selo em texto e entra na frase-resumo (Shneiderman, overview first), sem valores
 *   com "ocultar valores".
 * - "Gasto médio por categoria" diz o que a porcentagem compara ("último mês +34%") e leva a Transações com a categoria
 *   e os mesmos meses da média (detalhe por navegação, ADR 0018); nada fica só num title=.
 */
const mes = (m: number, saldoProjetado: number | null, situacao: ProjecaoMes['situacao'] = 'projetado'): ProjecaoMes =>
  ({ ano: 2026, mes: m, situacao, receitas: 1000, despesas: 800, parcelas: 0, resultado: 200, saldoProjetado });

function abrir(dados: Projecao, ocultar = false) {
  Object.defineProperty(window, 'matchMedia', {
    configurable: true, writable: true,
    value: () => ({ matches: false, addEventListener: () => undefined, removeEventListener: () => undefined })
  });
  TestBed.configureTestingModule({
    providers: [provideRouter([]), { provide: ApiService, useValue: { projecao: () => of(dados) } }]
  });
  if (ocultar) TestBed.inject(PrivacidadeService).alternar();
  const fixture = TestBed.createComponent(ProjecaoComponent);
  fixture.detectChanges();
  return { tela: fixture.componentInstance, el: fixture.nativeElement as HTMLElement };
}

const base: Projecao = {
  mesesConsiderados: 6, mediaReceitas: 1000, mediaDespesas: 800, saldoDisponivelAtual: -1000,
  meses: [mes(9, null, 'realizado'), mes(10, -343.87, 'em andamento'), mes(11, 616.13), mes(12, 1647.91)],
  categorias: [
    { categoriaId: 7, nome: 'Mercado', cor: '#aaa', mediaMensal: 882.67, ultimoMes: 1182.78 },
    { categoriaId: null, nome: 'Sem categoria', cor: '#bbb', mediaMensal: 100, ultimoMes: 17 }
  ]
};

describe('Projeção: virada do saldo e gasto por categoria (#314)', () => {
  beforeEach(() => localStorage.clear());

  it('o primeiro mês em que o saldo troca de sinal ganha o selo, em texto', () => {
    const { tela, el } = abrir(base);
    expect(tela.virada()?.mes.mes).toBe(11);
    expect(tela.virada()?.positivo).toBe(true);
    const selos = [...el.querySelectorAll('.selo-virada')];
    expect(selos).toHaveLength(1);
    expect(selos[0].textContent!.trim()).toBe('saldo volta ao positivo');
    expect(selos[0].closest('tr')!.textContent).toContain('nov/26');
  });

  it('saldo que fica negativo também é destacado; sem troca de sinal, nenhum selo', () => {
    const negativo = abrir({ ...base, saldoDisponivelAtual: 500, meses: [mes(10, 200), mes(11, -50), mes(12, -300)] });
    expect(negativo.el.querySelector('.selo-virada')!.textContent!.trim()).toBe('saldo fica negativo');
    TestBed.resetTestingModule();
    const sempre = abrir({ ...base, saldoDisponivelAtual: 500, meses: [mes(10, 600), mes(11, 700)] });
    expect(sempre.el.querySelector('.selo-virada')).toBeNull();
  });

  it('a frase-resumo diz o mês da virada, também com "ocultar valores" (sem valor)', () => {
    const { tela } = abrir(base, true);
    expect(tela.resumoGrafico()).toContain('O saldo volta ao positivo em novembro/2026.');
    expect(tela.resumoGrafico()).not.toMatch(/R\$/);
  });

  it('a porcentagem diz o que compara, sem depender de title=', () => {
    const { el } = abrir(base);
    const variacoes = [...el.querySelectorAll('.lista-categorias-projecao .variacao')];
    expect(variacoes[0].textContent!.replace(/\s+/g, ' ').trim()).toBe('último mês +34%');
    expect(variacoes[0].getAttribute('title')).toBeNull();
    // Abaixo da média: o menos tipográfico, como no resto do site.
    expect(variacoes[1].textContent!.replace(/\s+/g, ' ').trim()).toBe('último mês −83%');
    expect(el.textContent).toContain('em vermelho, mais de 10% acima; em verde, mais de 10% abaixo');
  });

  it('o nome da categoria leva a Transações com a categoria e os meses da média (e "Sem categoria" também)', () => {
    const { el } = abrir(base);
    const links = [...el.querySelectorAll<HTMLAnchorElement>('.lista-categorias-projecao a')];
    expect(links.map((a) => a.textContent!.trim())).toEqual(['Mercado', 'Sem categoria']);
    const url = TestBed.inject(Router).parseUrl(links[0].getAttribute('href')!);
    expect(url.root.children['primary'].segments.map((s) => s.path)).toEqual(['transacoes']);
    const hoje = new Date();
    const inicio = new Date(hoje.getFullYear(), hoje.getMonth() - 6, 1);
    const fim = new Date(hoje.getFullYear(), hoje.getMonth(), 0);
    const iso = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
    expect(url.queryParams).toEqual({ categoria: '7', inicio: iso(inicio), fim: iso(fim), tipo: 'despesa' });
    expect(TestBed.inject(Router).parseUrl(links[1].getAttribute('href')!).queryParams['categoria']).toBe('sem');
    expect(links[0].getAttribute('aria-label')).toBe('Ver as transações de Mercado dos meses da média');
  });
});
