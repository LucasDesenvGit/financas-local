import { TestBed } from '@angular/core/testing';
import { of, throwError } from 'rxjs';
import { ApiService } from '../../src/app/core/api.service';
import { CotacaoMoeda } from '../../src/app/core/modelos';
import { PrivacidadeService } from '../../src/app/core/privacidade.service';
import { InvestimentosCambioComponent } from '../../src/app/paginas/investimentos-cambio.component';

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
  const linhas = () => [...el.querySelectorAll('.lista-cambio li')]
    .map((li) => [...li.children].map((c) => c.textContent!.replace(/\s+/g, ' ').trim()).join(' '));
  return { el, linhas };
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
});
