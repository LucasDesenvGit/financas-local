import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { of } from 'rxjs';
import { ApiService } from '../../src/app/core/api.service';
import { Conta, TipoConta } from '../../src/app/core/modelos';
import { ContasComponent } from '../../src/app/paginas/contas.component';

const CONTAS = [
  { id: 1, nome: 'Conta corrente', tipo: TipoConta.ContaCorrente, origem: 1, saldo: 1000, arquivada: false, cor: '#a7a3f5', incluirNoSaldo: true },
  { id: 2, nome: 'Cartão Roxo', tipo: TipoConta.CartaoCredito, origem: 1, saldo: -1530, limiteCredito: 2000, arquivada: false, cor: '#d4a5e8', incluirNoSaldo: true }
] as unknown as Conta[];

async function abrir(url: string) {
  const pedidas: number[] = [];
  TestBed.configureTestingModule({
    providers: [
      provideRouter([{ path: 'contas', component: ContasComponent }]),
      {
        provide: ApiService,
        useValue: {
          contas: () => of(CONTAS),
          faturas: (id: number) => { pedidas.push(id); return of([]); }
        }
      }
    ]
  });
  const harness = await RouterTestingHarness.create();
  await harness.navigateByUrl(url, ContasComponent);
  harness.detectChanges();
  return { pedidas, raiz: harness.routeNativeElement as HTMLElement };
}

/**
 * Regressão da UX-12 (#88): "Faturas do mês" no Painel leva a /contas?faturas=ID, que abre direto as faturas do
 * cartão (detalhe por navegação, ADR 0018), sem a pessoa procurar o cartão e clicar em "faturas".
 */
describe('Regressão: link para as faturas de um cartão', () => {
  it('/contas?faturas=2 abre as faturas do Cartão Roxo', async () => {
    const t = await abrir('/contas?faturas=2');
    expect(t.pedidas).toEqual([2]);
    expect(t.raiz.querySelector('.modal')!.getAttribute('aria-label')).toBe('Faturas · Cartão Roxo');
  });

  it('id que não existe é ignorado e a tela abre normal', async () => {
    const t = await abrir('/contas?faturas=999');
    expect(t.pedidas).toEqual([]);
    expect(t.raiz.querySelector('.modal')).toBeNull();
  });

  it('sem o parâmetro, nenhuma janela abre sozinha', async () => {
    const t = await abrir('/contas');
    expect(t.pedidas).toEqual([]);
  });
});
