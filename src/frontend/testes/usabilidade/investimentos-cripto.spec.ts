import { TestBed } from '@angular/core/testing';
import { of, throwError } from 'rxjs';
import { ApiService } from '../../src/app/core/api.service';
import { CotacaoCripto } from '../../src/app/core/modelos';
import { PrivacidadeService } from '../../src/app/core/privacidade.service';
import { InvestimentosCriptoComponent, MOEDAS_CRIPTO } from '../../src/app/paginas/investimentos-cripto.component';

/**
 * #279 (ADR 0048): criptomoedas em reais em Investimentos, pelo último negócio no Mercado Bitcoin guardado pela API.
 * Data e hora sempre à vista (Nielsen 1); variação em 24 h com o sinal em texto, não só na cor (WCAG 1.4.1); sem
 * variação quando não há dado de 24 h antes (o site não inventa número); "criptomoeda" com ⓘ (Nielsen 2); aviso
 * quando a cotação passou de 2 horas; não segue "ocultar valores" (cotação é pública).
 */
const AGORA = Date.parse('2026-10-21T12:00:00Z');
const cripto = (moeda: string, preco: number, variacao24h: number | null, dataHora = '2026-10-21T11:45:00Z'): CotacaoCripto =>
  ({ moeda, preco, dataHora, variacao24h, fonte: 'Mercado Bitcoin' });

function abrir(resposta: unknown, ocultar = false) {
  TestBed.configureTestingModule({
    providers: [{ provide: ApiService, useValue: { cotacoesCripto: () => resposta } }]
  });
  if (ocultar) TestBed.inject(PrivacidadeService).alternar();
  const fixture = TestBed.createComponent(InvestimentosCriptoComponent);
  fixture.componentInstance.agora = AGORA;
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  const linhas = () => [...el.querySelectorAll('.lista-cripto li')]
    .map((li) => [...li.children].map((c) => c.textContent!.replace(/\s+/g, ' ').trim()).join(' '));
  return { el, linhas };
}

describe('Investimentos: criptomoedas em reais (#279)', () => {
  beforeEach(() => localStorage.clear());

  it('as quatro moedas na ordem, com preço, variação em 24 h (com sinal em texto) e data e hora', () => {
    const { el, linhas } = abrir(of({ atualizadoEm: null, cotacoes: [
      cripto('BTC', 419262, 2.04), cripto('ETH', 12382, -1.37), cripto('SOL', 538.82, 0), cripto('USDT', 5.0441, null)
    ] }));
    expect(MOEDAS_CRIPTO.map((m) => m.codigo)).toEqual(['BTC', 'ETH', 'SOL', 'USDT']);
    expect(el.querySelector('h2')!.textContent).toBe('Criptomoedas');
    const [btc, eth, sol, usdt] = linhas();
    // #329: a fonte em cada linha, antes da data e hora (como o "PTAX de …" do câmbio).
    expect(btc).toMatch(/^BTC Bitcoin R\$ 419\.262,00 \+2,04% em 24 h Mercado Bitcoin · \d{2}\/10 \d{2}:45$/);
    expect(el.querySelector('.desatualizada')).toBeNull();
    expect(eth).toContain('−1,37% em 24 h');
    expect(sol).toContain('0,00% em 24 h');
    expect(usdt).toContain('R$ 5,04');
    expect(usdt).toContain('variação em 24 h ainda sem dado');
    // A cor acompanha o sinal (positivo/negativo), mas o texto já diz para que lado foi.
    const variacoes = [...el.querySelectorAll('.variacao-cripto')];
    expect(variacoes[0].classList).toContain('positivo');
    expect(variacoes[1].classList).toContain('negativo');
    expect(el.querySelector('app-termo')).not.toBeNull();
    expect(el.textContent).toContain('Confira na sua corretora');
    expect(el.querySelector('.aviso-cripto')).toBeNull();
  });

  it('ícone de cada moeda à direita do código, decorativo (alt vazio), do próprio site', () => {
    const { el } = abrir(of({ atualizadoEm: null, cotacoes: [cripto('BTC', 419262, 2.04)] }));
    const identificacao = el.querySelector('.lista-cripto li .moeda-identificacao')!;
    expect(identificacao.getAttribute('aria-hidden')).toBe('true');
    expect([...identificacao.children].map((c) => c.tagName)).toEqual(['SPAN', 'IMG']);
    const icones = [...el.querySelectorAll<HTMLImageElement>('.lista-cripto img.icone-cripto')];
    expect(icones.map((i) => i.getAttribute('src'))).toEqual(['cripto/btc.svg', 'cripto/eth.svg', 'cripto/sol.svg', 'cripto/usdt.svg']);
    expect(icones.every((i) => i.getAttribute('alt') === '')).toBe(true);
  });

  it('moeda sem cotação guardada diz isso, em vez de sumir', () => {
    const { linhas } = abrir(of({ atualizadoEm: null, cotacoes: [cripto('BTC', 419262, 2.04)] }));
    expect(linhas()).toHaveLength(4);
    expect(linhas()[3]).toContain('sem cotação guardada ainda');
  });

  it('cotação com mais de 2 horas: mostra o valor e avisa a data e hora', () => {
    const { el, linhas } = abrir(of({ atualizadoEm: null, cotacoes: [cripto('BTC', 419262, 2.04, '2026-10-21T08:00:00Z')] }));
    expect(linhas()[0]).toContain('R$ 419.262,00');
    expect(el.querySelector('.aviso-cripto')!.textContent).toContain('21/10');
    // #329: a própria linha diz que a cotação está velha.
    expect(linhas()[0]).toContain('Mercado Bitcoin');
    expect(linhas()[0]).toContain('· desatualizada');
    expect(el.querySelector('.data-cripto')!.classList).toContain('desatualizada');
  });

  it('com "ocultar valores" ligado, a cotação continua à vista (é pública, não é dado seu)', () => {
    const { linhas } = abrir(of({ atualizadoEm: null, cotacoes: [cripto('BTC', 419262, 2.04)] }), true);
    expect(linhas()[0]).toContain('R$ 419.262,00');
  });

  it('erro ao ler as cotações: avisa em texto', () => {
    const { el } = abrir(throwError(() => new Error('falhou')));
    expect(el.textContent).toContain('Não deu para ler as cotações guardadas agora.');
  });
});
