import { TestBed } from '@angular/core/testing';
import { of } from 'rxjs';
import { ApiService } from '../../src/app/core/api.service';
import { HistoricoInvestimentos } from '../../src/app/core/modelos';
import { PrivacidadeService } from '../../src/app/core/privacidade.service';
import { RendimentoInvestimentosOfComponent } from '../../src/app/paginas/rendimento-investimentos-of.component';

/**
 * #173 e) (ADR 0045): rendimento por mês dos investimentos do Open Finance (valores fictícios). Shneiderman (a frase do
 * último mês antes da tabela; o detalhe por investimento sob demanda), Nielsen 1 (o mês que começou no meio diz desde
 * quando conta) e 10 (sem histórico, diz por quê e quando aparece), WCAG 4.1.2 (botão com aria-expanded e nome).
 */
describe('Usabilidade: rendimento por mês dos investimentos do Open Finance', () => {
  const historico = (campos: Partial<HistoricoInvestimentos> = {}): HistoricoInvestimentos => ({
    desde: '2025-09-20',
    meses: [
      { ano: 2025, mes: 9, desde: '2025-09-20', rendimento: 12.3, saldo: 5012.3,
        porInvestimento: [{ nome: 'Caixinha viagem', rendimento: 10, saldo: 4010 }, { nome: 'CDB · Banco X', rendimento: 2.3, saldo: 1002.3 }] },
      { ano: 2025, mes: 10, desde: null, rendimento: 40.5, saldo: 5552.8,
        porInvestimento: [{ nome: 'Caixinha viagem', rendimento: 33, saldo: 4543 }, { nome: 'Fundo Y', rendimento: null, saldo: 1009.8 }] }
    ],
    ...campos
  });

  afterEach(() => localStorage.clear());

  async function abrir(dados: HistoricoInvestimentos, ocultar = false) {
    TestBed.configureTestingModule({ providers: [{ provide: ApiService, useValue: { historicoInvestimentos: () => of(dados) } }] });
    if (ocultar) TestBed.inject(PrivacidadeService).alternar();
    const fixture = TestBed.createComponent(RendimentoInvestimentosOfComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    return { fixture, raiz: fixture.nativeElement as HTMLElement };
  }
  const texto = (el: Element | null) => (el?.textContent ?? '').replace(/\s+/g, ' ').trim();

  it('sem histórico ainda, diz por quê e quando o rendimento aparece', async () => {
    const { raiz } = await abrir({ desde: null, meses: [] });
    expect(texto(raiz.querySelector('.rendimento-of p'))).toContain('A partir da próxima sincronização, o site guarda o saldo');
    expect(raiz.querySelector('table')).toBeNull();
  });

  it('a frase diz o último mês; a tabela começa pelo mais recente, e o mês que começou no meio diz desde quando', async () => {
    const { raiz } = await abrir(historico());
    expect(texto(raiz.querySelector('.resumo-rendimento-of'))).toBe('Em out/25, os investimentos renderam R$ 41.');
    const linhas = Array.from(raiz.querySelectorAll('tbody > tr')).map((tr) => texto(tr.querySelector('th')));
    expect(linhas).toEqual(['out/25', 'set/25 desde 20/09']);
    expect(texto(raiz.querySelector('.nota'))).toContain('O histórico começou em 20/09/2025');
  });

  it('"por investimento" abre o detalhe do mês, com aria-expanded; fundo sem valor aplicado diz isso', async () => {
    const { fixture, raiz } = await abrir(historico());
    const botao = raiz.querySelector<HTMLButtonElement>('[aria-label="Rendimento por investimento em out/25"]')!;
    expect(botao.getAttribute('aria-expanded')).toBe('false');
    botao.click();
    fixture.detectChanges();
    expect(botao.getAttribute('aria-expanded')).toBe('true');
    const itens = Array.from(raiz.querySelectorAll('#rend-of-2025-10 li')).map(texto);
    expect(itens).toEqual(['Caixinha viagem: R$ 33,00', 'Fundo Y: sem o valor aplicado']);
  });

  it('"ocultar valores" tira o R$ da frase', async () => {
    const { raiz } = await abrir(historico(), true);
    expect(texto(raiz.querySelector('.resumo-rendimento-of'))).toBe('Em out/25, os investimentos renderam.');
  });
});
