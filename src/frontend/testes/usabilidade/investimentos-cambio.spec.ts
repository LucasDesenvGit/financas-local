import { TestBed } from '@angular/core/testing';
import { of, throwError } from 'rxjs';
import { ApiService } from '../../src/app/core/api.service';
import { CotacaoMoeda } from '../../src/app/core/modelos';
import { PrivacidadeService } from '../../src/app/core/privacidade.service';
import { CHAVE_MAIS_MOEDAS, InvestimentosCambioComponent, MOEDAS_CAMBIO } from '../../src/app/paginas/investimentos-cambio.component';

/**
 * #247: o câmbio em Investimentos. Dólar, euro e iene (por 100) em reais, pela PTAX de compra guardada; a data de cada
 * cotação à vista (Nielsen 1); PTAX com ⓘ (Nielsen 2); aviso quando a cotação está velha; não segue "ocultar valores"
 * (cotação é pública).
 */
const cotacao = (moeda: string, compra: number, data = '2026-10-20'): CotacaoMoeda =>
  ({ moeda, dataSolicitada: data, data, compra, venda: compra + 0.001, fonte: 'PTAX' });

function abrir(resposta: unknown, hoje = '2026-10-21', ocultar = false) {
  TestBed.configureTestingModule({
    providers: [{ provide: ApiService, useValue: { cotacoesCambio: () => resposta } }]
  });
  if (ocultar) TestBed.inject(PrivacidadeService).alternar();
  const fixture = TestBed.createComponent(InvestimentosCambioComponent);
  fixture.componentInstance.hoje = hoje;
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  const texto = (li: Element) => [...li.children].map((c) => c.textContent!.replace(/\s+/g, ' ').trim()).join(' ');
  // As três à vista; as outras ficam na lista de "Mais moedas" (#278).
  const linhas = () => [...el.querySelectorAll('.lista-cambio:not(.lista-cambio-mais) li')].map(texto);
  const mais = () => [...el.querySelectorAll('.lista-cambio-mais li')].map(texto);
  return { el, linhas, mais, fixture };
}

describe('Investimentos: câmbio do dia (#247)', () => {
  beforeEach(() => localStorage.clear());

  it('dólar, euro e iene por 100, pela PTAX de compra, com a data de cada um', () => {
    const { el, linhas } = abrir(of({ atualizadoEm: null, cotacoes: [
      cotacao('USD', 5.4321), cotacao('EUR', 6.3012), cotacao('JPY', 0.0362), cotacao('GBP', 7.1)
    ] }));
    expect(el.querySelector('h2')!.textContent).toBe('Câmbio');
    expect(linhas()).toEqual([
      'USD Dólar americano · 1 dólar R$ 5,43 PTAX de 20/10/2026',
      'EUR Euro · 1 euro R$ 6,30 PTAX de 20/10/2026',
      'JPY Iene japonês · 100 ienes R$ 3,62 PTAX de 20/10/2026'
    ]);
    // A PTAX tem o ⓘ do glossário, e não só um title= (padrão do projeto).
    expect(el.querySelector('app-termo')).not.toBeNull();
    expect(el.querySelector('.aviso-cambio')).toBeNull();
  });

  it('moeda sem cotação guardada diz isso, em vez de sumir', () => {
    const { linhas } = abrir(of({ atualizadoEm: null, cotacoes: [cotacao('USD', 5.43)] }));
    expect(linhas()[2]).toContain('sem cotação guardada ainda');
  });

  it('cotação velha (sem internet por dias): mostra o valor e avisa a data, sem esconder', () => {
    const { el, linhas } = abrir(of({ atualizadoEm: null, cotacoes: [cotacao('USD', 5.43, '2026-10-10')] }), '2026-10-21');
    expect(linhas()[0]).toContain('R$ 5,43');
    expect(el.querySelector('.aviso-cambio')!.textContent).toContain('10/10');
  });

  it('fim de semana não conta como velha (sexta → segunda)', () => {
    const { el } = abrir(of({ atualizadoEm: null, cotacoes: [cotacao('USD', 5.43, '2026-10-16')] }), '2026-10-19');
    expect(el.querySelector('.aviso-cambio')).toBeNull();
  });

  it('com "ocultar valores" ligado, a cotação continua à vista (é pública, não é dado seu)', () => {
    const { linhas } = abrir(of({ atualizadoEm: null, cotacoes: [cotacao('USD', 5.4321)] }), '2026-10-21', true);
    expect(linhas()[0]).toContain('R$ 5,43');
  });

  it('erro ao ler as cotações: avisa em texto', () => {
    const { el } = abrir(throwError(() => new Error('falhou')));
    expect(el.textContent).toContain('Não deu para ler as cotações guardadas agora.');
  });

  it('#278: as 10 moedas da PTAX, cada uma com a sua bandeira; três à vista', () => {
    expect(MOEDAS_CAMBIO.map((m) => m.codigo)).toEqual(['USD', 'EUR', 'JPY', 'GBP', 'CHF', 'CAD', 'AUD', 'DKK', 'NOK', 'SEK']);
    expect(new Set(MOEDAS_CAMBIO.map((m) => m.bandeira)).size).toBe(10);
    expect(MOEDAS_CAMBIO.filter((m) => m.principal).map((m) => m.codigo)).toEqual(['USD', 'EUR', 'JPY']);
  });

  it('#278: bandeira à direita do código, decorativa (alt vazio), do próprio site', () => {
    const { el } = abrir(of({ atualizadoEm: null, cotacoes: [cotacao('USD', 5.43)] }));
    const identificacao = el.querySelector('.lista-cambio li .moeda-identificacao')!;
    expect([...identificacao.children].map((c) => c.tagName)).toEqual(['SPAN', 'IMG']);
    const img = identificacao.querySelector('img')!;
    expect(img.getAttribute('src')).toBe('bandeiras/us.svg');
    expect(img.getAttribute('alt')).toBe('');
  });

  it('#278: "Mais moedas" abre as outras sete (padrão Disclosure) e lembra a escolha', () => {
    localStorage.clear();
    const { el, mais, fixture } = abrir(of({ atualizadoEm: null, cotacoes: [cotacao('USD', 5.43), cotacao('GBP', 7.2154)] }));
    const botao = el.querySelector<HTMLButtonElement>('.botao-mais-moedas')!;
    const lista = el.querySelector<HTMLElement>('#cambio-mais-moedas')!;
    expect(botao.getAttribute('aria-expanded')).toBe('false');
    expect(botao.getAttribute('aria-controls')).toBe('cambio-mais-moedas');
    expect(lista.hidden).toBe(true);
    expect(botao.textContent).toContain('Mais 7 moedas');

    botao.click();
    fixture.detectChanges();
    expect(botao.getAttribute('aria-expanded')).toBe('true');
    expect(lista.hidden).toBe(false);
    expect(botao.textContent).toContain('Menos moedas');
    expect(mais()).toHaveLength(7);
    expect(mais()[0]).toBe('GBP Libra esterlina · 1 libra R$ 7,22 PTAX de 20/10/2026');
    expect(localStorage.getItem(CHAVE_MAIS_MOEDAS)).toBe('1');
  });

  it('#278: abre já aberto quando a pessoa deixou assim', () => {
    localStorage.setItem(CHAVE_MAIS_MOEDAS, '1');
    const { el } = abrir(of({ atualizadoEm: null, cotacoes: [] }));
    expect(el.querySelector<HTMLElement>('#cambio-mais-moedas')!.hidden).toBe(false);
  });
});
