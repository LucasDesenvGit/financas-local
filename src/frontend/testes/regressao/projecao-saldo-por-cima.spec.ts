import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { of } from 'rxjs';
import { linhaPorCimaDasBarras } from '../../src/app/compartilhado/grafico-misto';
import { ApiService } from '../../src/app/core/api.service';
import { Projecao } from '../../src/app/core/modelos';
import { ProjecaoComponent } from '../../src/app/paginas/projecao.component';

/**
 * Regressão (mesmo defeito da #282, na Projeção): a linha do "Saldo previsto" ficava atrás das barras. Com o padrão do
 * <app-grafico> (grafico-misto.ts), ela vem por cima, com um ponto em cada mês.
 */
describe('Regressão: Projeção com a linha do saldo por cima das barras', () => {
  it('a linha tem order menor que as barras e um ponto em cada mês', () => {
    Object.defineProperty(window, 'matchMedia', {
      configurable: true, writable: true,
      value: () => ({ matches: false, addEventListener: () => undefined, removeEventListener: () => undefined })
    });
    const dados: Projecao = {
      mesesConsiderados: 6, mediaReceitas: 1000, mediaDespesas: 800, saldoDisponivelAtual: 500, categorias: [],
      meses: [
        { ano: 2026, mes: 9, situacao: 'realizado', receitas: 1000, despesas: 900, parcelas: 0, resultado: 100, saldoProjetado: null },
        { ano: 2026, mes: 10, situacao: 'projetado', receitas: 1000, despesas: 800, parcelas: 0, resultado: 200, saldoProjetado: 700 }
      ]
    };
    TestBed.configureTestingModule({
      providers: [provideRouter([]), { provide: ApiService, useValue: { projecao: () => of(dados) } }]
    });
    const fixture = TestBed.createComponent(ProjecaoComponent);
    fixture.detectChanges();
    // A configuração como vai para o Chart.js: o <app-grafico> aplica o padrão de barras com linha.
    const conjuntos = linhaPorCimaDasBarras(fixture.componentInstance.grafico()).data.datasets as unknown as
      { label: string; order: number; pointRadius?: number }[];
    const linha = conjuntos.find((c) => c.label === 'Saldo previsto')!;
    expect(conjuntos.filter((c) => c !== linha).every((b) => b.order > linha.order)).toBe(true);
    expect(linha.pointRadius).toBeGreaterThan(0);
  });
});
