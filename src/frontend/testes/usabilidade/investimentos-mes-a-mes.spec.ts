import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { of } from 'rxjs';
import { ApiService } from '../../src/app/core/api.service';
import { Fii, MesAMesFii, PainelFiis } from '../../src/app/core/modelos';
import { PrivacidadeService } from '../../src/app/core/privacidade.service';
import { InvestimentosFiisComponent } from '../../src/app/paginas/investimentos-fiis.component';

/**
 * #173: "Mês a mês" dos FIIs, como no app da corretora.
 * - Nielsen 2 (Match Between the System and the Real World): os mesmos números e nomes do app — aportes, rendimentos,
 *   valorização — com o mês mais recente primeiro.
 * - Shneiderman (overview first): o gráfico e a frase-resumo do último mês antes da tabela.
 * - WCAG 1.1.1 (Non-text Content): o gráfico tem a frase-resumo, que respeita "ocultar valores".
 */
const MESES = ['jan', 'fev', 'mar', 'abr', 'mai', 'jun', 'jul', 'ago', 'set', 'out', 'nov', 'dez'];

/** Os últimos `n` meses até o atual, do mais antigo ao mais recente (como a API devolve). */
function meses(n: number, campos: (i: number) => Partial<MesAMesFii> = () => ({})): MesAMesFii[] {
  const hoje = new Date();
  return Array.from({ length: n }, (_, i) => {
    const d = new Date(hoje.getFullYear(), hoje.getMonth() - (n - 1 - i), 1);
    return { ano: d.getFullYear(), mes: d.getMonth() + 1, aportes: 0, subscricoes: 0, resgates: 0, rendimentosRecebidos: 0,
      rendimentosEstimados: 0, valorizacao: 0, saldo: 1000, ...campos(i) };
  });
}

const rotulo = (m: MesAMesFii) => `${MESES[m.mes - 1]}/${String(m.ano).slice(2)}`;

function abrir(mesAMes: MesAMesFii[]) {
  const painel = {
    resumo: { valorMercado: 1000, custo: 900, valorizacao: 100, valorizacaoPercentual: 11.1, renda12Meses: 0, rendaMediaMensal: 0,
      rendaMensalEstimada: 0, dy12Meses: null, yieldOnCost12Meses: null, recebidoExtrato12Meses: 0, rentabilidadeTotal: null,
      rentabilidadeSelic: null, rentabilidadeIfix: null, quantidadeFiis: 1 },
    fiis: [{ id: 1, ticker: 'MXRF11', nome: null, segmento: null, segmentoSugerido: false, quantidade: 100, precoMedio: 9,
      custo: 900, precoAtual: 10, variacaoDia: null, valorMercado: 1000, valorizacao: 100, valorizacaoPercentual: 11.1,
      ultimoRendimento: null, dataComUltimo: null, rendimentosPorCota12Meses: 0, dyMensal: null, dy12Meses: null,
      yieldOnCost12Meses: null, yieldOnCostMensal: null, rendaMensalEstimada: 0, proventos12Meses: 0, proventosTotal: 0,
      recebidoExtrato12Meses: 0, pesoCarteira: 100, fonteCotacao: null, precoManual: false } as Fii],
    lotes: [], proventos: [], evolucao: [], rentabilidade: [], segmentos: [], historicos: [], segmentosDisponiveis: [], mesAMes
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
  const secao = () => raiz.querySelector<HTMLElement>('section.mes-a-mes');
  const linhas = () => Array.from(secao()!.querySelectorAll<HTMLTableRowElement>('tbody tr'));
  const atualizar = () => fixture.detectChanges();
  return { fixture, tela: fixture.componentInstance, raiz, secao, linhas, atualizar };
}

describe('Usabilidade: Investimentos › FIIs › Mês a mês (#173)', () => {
  it('#173 c): "por fundo" abre e fecha o detalhe do mês, com aria-expanded; o crédito sem fundo aparece como tal', () => {
    const t = abrir(meses(2, (i) => (i === 1 ? {
      rendimentosRecebidos: 25, porFii: [{ ticker: 'MXRF11', valor: 10 }, { ticker: 'HGLG11', valor: 12 }, { ticker: null, valor: 3 }]
    } : {})));
    const botao = t.linhas()[0].querySelector<HTMLButtonElement>('button.link')!;
    expect(botao.textContent!.trim()).toBe('por fundo');
    expect(botao.getAttribute('aria-expanded')).toBe('false');
    botao.click();
    t.atualizar();
    expect(botao.getAttribute('aria-expanded')).toBe('true');
    const lista = t.secao()!.querySelector('#' + botao.getAttribute('aria-controls'))!;
    expect(lista.textContent!.replace(/\s+/g, ' ')).toMatch(/MXRF11: R\$\s10,00HGLG11: R\$\s12,00sem fundo identificado: R\$\s3,00/);
    botao.click();
    t.atualizar();
    expect(t.secao()!.querySelector('.linha-por-fii')).toBeNull();
    // Mês sem o detalhe não tem o botão.
    expect(t.linhas()[1].querySelector('button.link')).toBeNull();
  });

  it('o gráfico mostra só o que a carteira rendeu (rendimentos e valorização), na escala deles; os aportes ficam na tabela', () => {
    // Com aporte de milhares no mesmo eixo, os rendimentos (dezenas de reais) viravam tracinhos invisíveis.
    const t = abrir(meses(3, (i) => ({ aportes: i === 0 ? 7000 : 0, rendimentosRecebidos: 60 + i, valorizacao: i === 1 ? -20 : 15 })));
    const series = t.tela.graficoMesAMes().data.datasets.map((d) => d.label);
    expect(series).toEqual(['Rendimentos', 'Valorização']);
    expect(t.linhas().at(-1)!.textContent).toContain('7.000,00');
  });

  it('as colunas têm os nomes do app: Aportes, Rendimentos, Valorização e Saldo no fim', () => {
    const t = abrir(meses(3));
    expect(t.secao()!.querySelector('h2')!.textContent).toBe('Mês a mês');
    const ths = Array.from(t.secao()!.querySelectorAll('thead th'));
    const cabecalhos = ths.map((th) => th.textContent!.replace('ⓘ', '').replace(/\s+/g, ' ').trim());
    expect(cabecalhos).toEqual(['Mês', 'Aportes', 'Rendimentos', 'Valorização', 'Saldo no fim']);
    // Termo com definição no contexto (ⓘ do glossário), não só em title=.
    expect(ths[3].querySelector('app-termo button')).not.toBeNull();
  });

  it('na tabela, o mês mais recente vem primeiro, marcado "até hoje"', () => {
    const dados = meses(3);
    const t = abrir(dados);
    const primeira = t.linhas()[0].querySelector('th')!.textContent!;
    expect(primeira).toContain(rotulo(dados[2]));
    expect(primeira).toContain('até hoje');
    expect(t.linhas()[2].querySelector('th')!.textContent).toContain(rotulo(dados[0]));
  });

  it('subscrição e vendas aparecem à parte, embaixo dos aportes', () => {
    const t = abrir(meses(1, () => ({ aportes: 500, subscricoes: 40, resgates: 120 })));
    const aportes = t.linhas()[0].querySelectorAll('td')[0].textContent!;
    expect(aportes).toContain('em subscrição');
    expect(aportes).toContain('em vendas');
  });

  it('no mês atual, o provento com data-com no mês aparece como "a receber"; nos anteriores, não', () => {
    const t = abrir(meses(2, () => ({ rendimentosEstimados: 12 })));
    expect(t.linhas()[0].querySelectorAll('td')[1].textContent).toContain('a receber (data-com)');
    expect(t.linhas()[1].querySelectorAll('td')[1].textContent).not.toContain('a receber');
  });

  it('valorização negativa fica marcada como perda; positiva, como ganho', () => {
    const t = abrir(meses(2, (i) => ({ valorizacao: i === 0 ? 25 : -30 })));
    expect(t.linhas()[0].querySelectorAll('td')[2].classList).toContain('negativo');
    expect(t.linhas()[1].querySelectorAll('td')[2].classList).toContain('positivo');
  });

  it('a frase-resumo diz os três números do último mês e, com "ocultar valores", não mostra R$', () => {
    const dados = meses(2, (i) => (i === 1 ? { aportes: 500, rendimentosRecebidos: 42, valorizacao: -30 } : {}));
    const t = abrir(dados);
    const resumo = t.tela.resumoMesAMes()!;
    expect(resumo).toContain(rotulo(dados[1]));
    expect(resumo).toMatch(/R\$\s?500 de aportes/);
    expect(resumo).toMatch(/R\$\s?42 de rendimentos/);
    expect(resumo).toMatch(/caíram R\$\s?30/);

    TestBed.inject(PrivacidadeService).oculto.set(true);
    const oculto = t.tela.resumoMesAMes()!;
    expect(oculto).not.toContain('R$');
    expect(oculto).toContain('houve aportes');
    expect(oculto).toContain('caíram de preço');
  });

  it('12 meses por padrão; "24 meses" mostra o período maior, com o estado no botão', () => {
    const t = abrir(meses(20));
    expect(t.linhas().length).toBe(12);
    const botoes = Array.from(t.secao()!.querySelectorAll<HTMLButtonElement>('.segmentado button'));
    expect(botoes[0].getAttribute('aria-pressed')).toBe('true');
    botoes[1].click();
    t.atualizar();
    expect(t.linhas().length).toBe(20);
    expect(botoes[1].getAttribute('aria-pressed')).toBe('true');
  });

  it('sem nenhum mês, a seção não aparece', () => {
    const t = abrir([]);
    expect(t.secao()).toBeNull();
  });
});
