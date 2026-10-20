import { TestBed } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { of } from 'rxjs';
import { ApiService } from '../../src/app/core/api.service';
import { Banco, Conta, Fatura, OrigemConta, SalvarConta, TipoConta } from '../../src/app/core/modelos';
import { ContasComponent } from '../../src/app/paginas/contas.component';

export const BANCOS: Banco[] = [
  { codigo: 'nubank', nome: 'Nubank' }, { codigo: 'itau', nome: 'Itaú' }, { codigo: 'mercadopago', nome: 'Mercado Pago' },
  { codigo: 'outro', nome: 'Outro (sem banco)' }
];

export function conta(campos: Partial<Conta> = {}): Conta {
  return {
    id: 1, nome: 'Conta corrente', tipo: TipoConta.ContaCorrente, origem: OrigemConta.OpenFinance, numero: null, moeda: 'BRL',
    saldo: 1000, saldoInicial: 0, limiteCredito: null, limiteDisponivel: null, diaFechamento: null, diaVencimento: null,
    cor: '#a7a3f5', incluirNoSaldo: true, arquivada: false, conexaoBancariaId: null, nomeInstituicao: null,
    atualizadoEm: '2026-10-17T10:00:00Z', banco: null, bancoDefinido: false, contaVinculadaId: null, faturaMes: null,
    ...campos
  };
}

/** Nubank: conta corrente e o cartão dela (banco herdado da conta), e um cartão manual sem banco. */
export const CONTAS: Conta[] = [
  conta({ id: 1, nome: 'Nubank conta', banco: 'nubank', saldo: 2500 }),
  conta({ id: 2, nome: 'Ultravioleta', tipo: TipoConta.CartaoCredito, banco: 'nubank', contaVinculadaId: 1, saldo: -2950,
    limiteCredito: 5000, limiteDisponivel: 2050, diaFechamento: 3, diaVencimento: 10, numero: '5555 4444 3333 1234',
    faturaMes: { contaId: 2, cartao: 'Ultravioleta', valorMes: 820, parcelasMes: 100, faturaId: 30, vencimento: '2026-10-10',
      situacao: 'fechada', banco: 'nubank' } }),
  conta({ id: 3, nome: 'Cartão Roxo', tipo: TipoConta.CartaoCredito, origem: OrigemConta.Manual, saldo: -1530,
    limiteCredito: 2000, cor: '#d4a5e8' })
];

export function fatura(campos: Partial<Fatura> = {}): Fatura {
  return { id: 1, dataVencimento: '2026-10-10', dataFechamento: '2026-10-03', valorTotal: 1530, valorMinimo: null,
    valorPago: null, quantidadeTransacoes: 12, situacao: 'fechada', ...campos };
}

/** Abre /contas pelo roteador (a URL é lida e gravada), com a API falsa; guarda faturas pedidas e contas salvas. */
export async function abrirContas(url = '/contas', contas: Conta[] = CONTAS,
  faturasPorConta: Record<number, Fatura[]> = {}, extras: Record<string, unknown> = {}) {
  const pedidas: number[] = [];
  const salvas: { id: number | null; dto: SalvarConta }[] = [];
  /** Cada carga de contas: true quando pediu também as arquivadas. */
  const cargas: boolean[] = [];
  TestBed.configureTestingModule({
    providers: [
      provideRouter([{ path: 'contas', component: ContasComponent }, { path: 'transacoes', children: [] }]),
      {
        provide: ApiService,
        useValue: {
          contas: (arquivadas?: boolean) => { cargas.push(!!arquivadas); return of(contas); },
          bancos: () => of(BANCOS),
          faturas: (id: number) => { pedidas.push(id); return of(faturasPorConta[id] ?? []); },
          criarConta: (dto: SalvarConta) => { salvas.push({ id: null, dto }); return of(conta()); },
          atualizarConta: (id: number, dto: SalvarConta) => { salvas.push({ id, dto }); return of(conta()); },
          ...extras
        }
      }
    ]
  });
  const harness = await RouterTestingHarness.create();
  const tela = await harness.navigateByUrl(url, ContasComponent);
  const atualizar = async () => { await harness.fixture.whenStable(); harness.detectChanges(); };
  await atualizar();
  const raiz = harness.routeNativeElement as HTMLElement;
  const cartao = (id: number) => raiz.querySelector<HTMLElement>(`#cartao-${id}`)!;
  return { harness, tela, raiz, atualizar, pedidas, salvas, cargas, cartao, router: TestBed.inject(Router) };
}
