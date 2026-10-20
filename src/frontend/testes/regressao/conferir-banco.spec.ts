import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { of } from 'rxjs';
import { ApiService } from '../../src/app/core/api.service';
import { ConferenciaBanco } from '../../src/app/core/modelos';
import { ConferirBancoComponent } from '../../src/app/paginas/conferir-banco.component';
import { comparar } from '../../src/app/paginas/conferir-banco';

/**
 * #172: a verificação feita à mão (19/10/2026), comparando o site com os apps dos bancos, agora é a tela "Conferir com o
 * banco". A pista com o mesmo valor da diferença vem primeiro.
 */
describe('Regressão: Conferir com o banco (#172)', () => {
  const dados: ConferenciaBanco = {
    contas: [{ id: 1, nome: 'Mercado Pago', banco: 'mercadopago', openFinance: true, saldo: 1000, pistas: [
      { texto: 'O saldo vem do banco, atualizado em 19/10/2026.', valor: null },
      { texto: 'Reservas e caixinhas desta conta (o app pode mostrar o saldo com elas)', valor: 2000, rota: '/investimentos',
        parametros: { aba: 'reservas' } }
    ] }],
    cartoes: [{ id: 2, nome: 'Gold', banco: 'nubank', faturaMes: 511.72, situacao: 'aberta', fechamento: '2026-11-03',
      vencimento: '2026-11-10', dividaTotal: 1200, pistas: [
        { texto: 'Parcelas nesta fatura (o app pode mostrá-las à parte)', valor: 59.9 },
        { texto: 'Dívida total do cartão, com as parcelas futuras, menos a fatura do mês (se o app mostra a dívida total)', valor: 688.28 }
      ] }],
    investimentos: []
  };

  async function abrir() {
    TestBed.configureTestingModule({
      providers: [provideRouter([]), { provide: ApiService, useValue: { conferenciaBanco: () => of(dados) } }]
    });
    const fixture = TestBed.createComponent(ConferirBancoComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    const linha = (id: string) => (fixture.nativeElement as HTMLElement).querySelector<HTMLElement>(`#${id}`)!;
    const digitar = async (chave: string, valor: number) => {
      fixture.componentInstance.digitar(chave, valor);
      fixture.detectChanges();
      await fixture.whenStable();
      fixture.detectChanges();
    };
    return { fixture, linha, digitar };
  }

  it('compara: igual "bate"; diferente traz a pista do mesmo valor primeiro', () => {
    expect(comparar(100, null, [])).toBeNull();
    expect(comparar(100, 100.004, [])!.bate).toBe(true);
    const r = comparar(1000, 3000, [{ texto: 'a', valor: 5 }, { texto: 'reservas', valor: 2000 }])!;
    expect(r).toMatchObject({ diferenca: 2000, bate: false });
    expect(r.pistas.map((p) => [p.texto, p.explica])).toEqual([['reservas', true], ['a', false]]);
  });

  it('o saldo do app com as reservas mostra "Explica a diferença" na pista das reservas, com o link', async () => {
    const t = await abrir();
    expect(t.linha('conta-1').textContent).toContain('digite o valor do app');
    await t.digitar('conta-1', 3000);
    const texto = t.linha('conta-1').textContent!.replace(/\s+/g, ' ');
    expect(texto).toContain('o app mostra R$ 2.000,00 a mais');
    const primeira = t.linha('conta-1').querySelector('.pistas li')!;
    expect(primeira.classList).toContain('explica');
    expect(primeira.textContent).toContain('Reservas e caixinhas');
    expect(primeira.querySelector('a')!.getAttribute('href')).toBe('/investimentos?aba=reservas');
  });

  it('o cartão: digitar a dívida total em vez da fatura é explicado; o valor certo "bate"', async () => {
    const t = await abrir();
    await t.digitar('cartao-2', 1200);
    expect(t.linha('cartao-2').querySelector('.pistas li.explica')!.textContent).toContain('Dívida total do cartão');
    await t.digitar('cartao-2', 511.72);
    expect(t.linha('cartao-2').textContent).toContain('bate');
    expect(t.linha('cartao-2').textContent).toContain('em aberto · vence 10/11');
  });

  it('cada campo diz qual valor é (leitor de tela)', async () => {
    const t = await abrir();
    const campo = t.linha('cartao-2').querySelector('input')!;
    expect(campo.getAttribute('aria-label')).toBe('Valor no app: Gold');
  });
});
