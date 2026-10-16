import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { of } from 'rxjs';
import { ApiService } from '../../src/app/core/api.service';
import { Conta, Meta, Reserva, TipoConta } from '../../src/app/core/modelos';
import { MetasComponent } from '../../src/app/paginas/metas.component';

export const CONTAS = [
  { id: 1, nome: 'Mercado Pago', tipo: TipoConta.ContaCorrente },
  { id: 2, nome: 'Cartão Roxo', tipo: TipoConta.CartaoCredito }
] as unknown as Conta[];

export const RESERVAS = [
  { contaId: 1, nomeConta: 'Mercado Pago', nome: 'Viagem', saldo: 3250, podeTerSaldoAnterior: false, conferenciaAntiga: false },
  { contaId: 1, nomeConta: 'Mercado Pago', nome: 'Emergência', saldo: 12400, podeTerSaldoAnterior: true, conferenciaAntiga: false }
] as unknown as Reserva[];

export function meta(campos: Partial<Meta>): Meta {
  return {
    id: 1, nome: 'Viagem', valorAlvo: 10000, valorAtual: 3750, dataAlvo: null, contaId: 1, contaNome: 'Mercado Pago',
    cor: '#a7a3f5', concluida: false, percentual: 37.5, aporteMensalSugerido: null, valorInicial: 500, saldoConta: 0,
    reservaNome: 'Viagem', saldoReserva: 3250, reservaConferir: false, reservaNaoEncontrada: false, ...campos
  };
}

/** Abre Metas com a API falsa. */
export function abrirMetas(metas: Meta[] = []) {
  TestBed.configureTestingModule({
    providers: [
      provideRouter([]),
      { provide: ApiService, useValue: { metas: () => of(metas), contas: () => of(CONTAS), reservas: () => of(RESERVAS) } }
    ]
  });
  const fixture = TestBed.createComponent(MetasComponent);
  fixture.detectChanges();
  const raiz = fixture.nativeElement as HTMLElement;
  const atualizar = async () => { fixture.detectChanges(); await fixture.whenStable(); fixture.detectChanges(); };
  const escolher = async (nome: string, texto: string) => {
    const sel = raiz.querySelector<HTMLSelectElement>(`select[name=${nome}]`)!;
    sel.selectedIndex = Array.from(sel.options).findIndex((o) => o.text.startsWith(texto));
    sel.dispatchEvent(new Event('change'));
    await atualizar();
  };
  return { fixture, tela: fixture.componentInstance, raiz, atualizar, escolher };
}
