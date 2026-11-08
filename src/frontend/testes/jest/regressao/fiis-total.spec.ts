import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { of } from 'rxjs';
import { ApiService } from '../../../src/app/core/api.service';
import { Fii, PainelFiis } from '../../../src/app/core/modelos';
import { PrivacidadeService } from '../../../src/app/core/privacidade.service';
import { InvestimentosFiisComponent } from '../../../src/app/paginas/patrimonio/investimentos-fiis.component';
import { totalDosFiis } from '../../../src/app/paginas/patrimonio/fiis-total';

/**
 * #384: "Meus FIIs" com o total da carteira, na linha do fim da tabela e ao lado do título.
 * - Shneiderman (overview first): o valor da carteira antes da linha de cada fundo.
 * - Nielsen 1 (Visibility of System Status): o fundo sem cotação fica fora do valor atual, e o total diz isso.
 * - WCAG 1.4.1 (Use of Color): a valorização tem o sinal em texto, além da cor.
 */
function fii(campos: Partial<Fii>): Fii {
  return {
    id: 1, ticker: 'MXRF11', nome: null, segmento: null, segmentoSugerido: false, quantidade: 100, precoMedio: 10, custo: 1000,
    precoAtual: 11, variacaoDia: null, valorMercado: 1100, valorizacao: 100, valorizacaoPercentual: 10, ultimoRendimento: null,
    dataComUltimo: null, rendimentosPorCota12Meses: 0, dyMensal: null, dy12Meses: null, yieldOnCost12Meses: null,
    yieldOnCostMensal: null, rendaMensalEstimada: 10, proventos12Meses: 0, proventosTotal: 0, recebidoExtrato12Meses: 0,
    pesoCarteira: 100, fonteCotacao: null, precoManual: false, ...campos
  };
}

const DOIS_FUNDOS = [
  fii({ id: 1, ticker: 'MXRF11', custo: 1000, valorMercado: 1100, valorizacao: 100, rendaMensalEstimada: 10 }),
  fii({ id: 2, ticker: 'HGLG11', custo: 500.1, valorMercado: 450.2, valorizacao: -49.9, rendaMensalEstimada: 4.2 })
];

function abrir(fiis: Fii[]) {
  const painel = {
    resumo: { valorMercado: 0, custo: 0, valorizacao: 0, valorizacaoPercentual: null, renda12Meses: 0, rendaMediaMensal: 0,
      rendaMensalEstimada: 0, dy12Meses: null, yieldOnCost12Meses: null, recebidoExtrato12Meses: 0, rentabilidadeTotal: null,
      rentabilidadeSelic: null, rentabilidadeIfix: null, quantidadeFiis: fiis.length },
    fiis, lotes: [], proventos: [], evolucao: [], rentabilidade: [], segmentos: [], historicos: [], segmentosDisponiveis: [],
    mesAMes: []
  } as PainelFiis;
  // Os gráficos usam o tema, que lê prefers-color-scheme; o jsdom não tem matchMedia.
  Object.defineProperty(window, 'matchMedia', {
    configurable: true, writable: true,
    value: () => ({ matches: false, addEventListener: () => undefined, removeEventListener: () => undefined })
  });
  TestBed.configureTestingModule({ providers: [provideRouter([]), { provide: ApiService, useValue: { fiis: () => of(painel) } }] });
  const fixture = TestBed.createComponent(InvestimentosFiisComponent);
  fixture.detectChanges();
  const raiz = fixture.nativeElement as HTMLElement;
  const secao = () => raiz.querySelector<HTMLElement>('section.meus-fiis')!;
  // O <br> não vira espaço no textContent: troca por um, como o leitor de tela lê.
  const texto = (el: Element | null) => {
    const copia = el?.cloneNode(true) as Element | undefined;
    copia?.querySelectorAll('br').forEach((br) => br.replaceWith(' '));
    return (copia?.textContent ?? '').replace(/\s+/g, ' ').trim();
  };
  const celulasTotal = () => Array.from(secao().querySelectorAll('tfoot tr > *')).map(texto);
  return { fixture, raiz, secao, texto, celulasTotal };
}

describe('Regressão: Meus FIIs com o total da carteira (#384)', () => {
  it('soma o investido, o valor atual, a valorização (R$ e %), a renda e o yield on cost da carteira', () => {
    const t = totalDosFiis(DOIS_FUNDOS);
    expect(t.fundos).toBe(2);
    expect(t.investido).toBe(1500.1);
    expect(t.valorAtual).toBe(1550.2);
    expect(t.valorizacao).toBe(50.1);
    expect(t.valorizacaoPercentual).toBeCloseTo((50.1 / 1500.1) * 100, 10);
    expect(t.rendaMensal).toBe(14.2);
    expect(t.yieldOnCostMensal).toBeCloseTo((14.2 / 1500.1) * 100, 10);
    expect(t.semCotacao).toBe(0);
  });

  it('o fundo sem cotação entra no investido e na renda, mas fica fora do valor atual e da valorização', () => {
    // A API conta o fundo sem cotação pelo preço médio (valor de mercado = custo); o total não usa esse valor.
    const semCotacao = fii({ id: 3, ticker: 'XPLG11', custo: 300, precoAtual: null, valorMercado: 300, valorizacao: 0, rendaMensalEstimada: 3 });
    const t = totalDosFiis([...DOIS_FUNDOS, semCotacao]);
    expect(t.semCotacao).toBe(1);
    expect(t.investido).toBe(1800.1);
    expect(t.valorAtual).toBe(1550.2);
    expect(t.valorizacao).toBe(50.1);
    expect(t.valorizacaoPercentual).toBeCloseTo((50.1 / 1500.1) * 100, 10);
    expect(t.rendaMensal).toBe(17.2);
  });

  it('sem custo, a valorização e o yield on cost em % ficam sem número (e não dividem por zero)', () => {
    const t = totalDosFiis([fii({ custo: 0, valorMercado: 0, valorizacao: 0, rendaMensalEstimada: 0, precoAtual: null })]);
    expect(t.valorizacaoPercentual).toBeNull();
    expect(t.yieldOnCostMensal).toBeNull();
  });

  it('a linha de total fica no fim da tabela, alinhada com as 11 colunas, com o sinal da valorização em texto', () => {
    const tela = abrir(DOIS_FUNDOS);
    const colunas = tela.secao().querySelectorAll('thead th').length;
    const ocupadas = Array.from(tela.secao().querySelectorAll<HTMLTableCellElement>('tfoot tr > *'))
      .reduce((soma, c) => soma + c.colSpan, 0);
    expect(colunas).toBe(11);
    expect(ocupadas).toBe(colunas);

    const [fundo, investido, atual, valorizacao, , , yoc, renda, peso] = tela.celulasTotal();
    expect(fundo).toBe('Total 2 fundos');
    expect(investido).toContain('1.500,10');
    expect(atual).toContain('1.550,20');
    expect(valorizacao).toContain('+3,34%');
    expect(valorizacao).toContain('+R$');
    expect(yoc).toBe('0,95% ao mês');
    expect(renda).toContain('14,20');
    expect(peso).toBe('100%');
  });

  it('o valor atual e o investido aparecem ao lado do título, e o aviso do fundo sem cotação também', () => {
    const semCotacao = fii({ id: 3, ticker: 'XPLG11', custo: 300, precoAtual: null, valorMercado: 300, valorizacao: 0 });
    const tela = abrir([...DOIS_FUNDOS, semCotacao]);
    const topo = tela.texto(tela.secao().querySelector('.total-topo'));
    expect(topo).toContain('1.550,20');
    expect(topo).toContain('investido R$ 1.800,10');
    expect(topo).toContain('1 fundo sem cotação, fora do valor atual');
    expect(tela.celulasTotal()[2]).toContain('1 fundo sem cotação, fora do valor atual');
  });

  it('com "ocultar valores", o total esconde os reais e mostra só os percentuais', () => {
    const tela = abrir(DOIS_FUNDOS);
    TestBed.inject(PrivacidadeService).oculto.set(true);
    tela.fixture.detectChanges();
    const linha = tela.celulasTotal().join(' | ');
    const topo = tela.texto(tela.secao().querySelector('.total-topo'));
    expect(linha).not.toMatch(/\d{1,3},\d{2}(?!%)/);
    expect(linha).toContain('+3,34%');
    expect(topo).not.toMatch(/\d,\d{2}/);
    expect(topo).toContain('R$ •••••');
  });
});
