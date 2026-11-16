import { By } from '@angular/platform-browser';
import { TestBed } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { of } from 'rxjs';
import { ApiService } from '../../../src/app/core/api.service';
import { AvisosService } from '../../../src/app/core/avisos.service';
import { Conta, Meta, PlanoMetas, Reserva, SalvarMeta, TipoConta } from '../../../src/app/core/modelos';
import { MetasComponent } from '../../../src/app/paginas/planejamento/metas.component';
import { MetasFormularioComponent } from '../../../src/app/paginas/planejamento/metas-formulario.component';

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
    reservaNome: 'Viagem', saldoReserva: 3250, reservaConferir: false, reservaNaoEncontrada: false, prioridade: 0, ...campos
  };
}

/** Plano sem histórico suficiente (o padrão dos testes que não olham o assistente). */
export function semPlano(): PlanoMetas {
  return { disponivel: false, mesesConsiderados: 1, ano: 2026, mes: 10, margemPercentual: 10, saldoContas: 0, faturaAPagar: 0,
    saldoFimDoMes: 0, melhorMesRecente: 0, meses: [] };
}

/** API falsa de Metas; `ordens` guarda cada nova ordem de prioridade e `parametros`, cada parâmetro salvo. */
function apiFalsa(metas: Meta[], plano: PlanoMetas) {
  const ordens: number[][] = [];
  const parametros: Record<string, number>[] = [];
  /** #359: as metas criadas pelo formulário (o que vai para a API). */
  const criadas: SalvarMeta[] = [];
  /** #359: as metas editadas (id e o que vai para a API). */
  const atualizadas: { id: number; dto: SalvarMeta }[] = [];
  /** #631: os ids das metas excluídas. */
  const excluidas: number[] = [];
  /** #732: os aportes registrados (meta e valor; negativo é retirada). */
  const aportes: { id: number; valor: number }[] = [];
  const api = {
    metas: () => of(metas), contas: () => of(CONTAS), reservas: () => of(RESERVAS), planoMetas: () => of(plano),
    aportesMeta: () => of([]),
    ordenarMetas: (ids: number[]) => { ordens.push(ids); return of(undefined); },
    salvarParametros: (valores: Record<string, number>) => { parametros.push(valores); return of([]); },
    criarMeta: (dto: SalvarMeta) => { criadas.push(dto); return of(metas[0]); },
    atualizarMeta: (id: number, dto: SalvarMeta) => { atualizadas.push({ id, dto }); return of(metas[0]); },
    excluirMeta: (id: number) => { excluidas.push(id); return of(undefined); },
    aporteMeta: (id: number, valor: number) => { aportes.push({ id, valor }); return of(undefined); }
  };
  return { api, ordens, parametros, criadas, atualizadas, excluidas, aportes };
}

/**
 * Salvar ou registrar um aporte mostra "Meta salva." por 4 s, e o `whenStable` do `atualizar()` espera esse tempo. O teste
 * que continua usando a tela depois de salvar chama isto antes, para não esperar o aviso (que ele não confere).
 */
export function semTempoDeAviso(): void {
  jest.spyOn(TestBed.inject(AvisosService), 'sucesso').mockImplementation(() => undefined);
}

/** Abre Metas com a API falsa (sem URL). */
export function abrirMetas(metas: Meta[] = [], plano: PlanoMetas = semPlano()) {
  const { api, ordens, parametros, criadas, atualizadas, excluidas, aportes } = apiFalsa(metas, plano);
  TestBed.configureTestingModule({ providers: [provideRouter([]), { provide: ApiService, useValue: api }] });
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
  /** #732: a janela da meta é um componente à parte; ela edita uma cópia (`edicao()`), e "Salvar" a manda para a tela. */
  const formulario = () => fixture.debugElement.query(By.directive(MetasFormularioComponent)).componentInstance as MetasFormularioComponent;
  /** "Salvar" pelo botão do formulário. Sem esperar a tela estabilizar: o aviso "Meta salva." tem um tempo próprio. */
  const salvar = () => {
    raiz.querySelector('form.formulario')!.dispatchEvent(new Event('submit'));
    fixture.detectChanges();
  };
  return { fixture, tela: fixture.componentInstance, raiz, atualizar, escolher, formulario, salvar, ordens, parametros, criadas, atualizadas, excluidas, aportes };
}

/** Abre /metas pelo roteador de verdade (a URL é lida e gravada como no navegador), com a API falsa. */
export async function abrirMetasPelaUrl(url: string, metas: Meta[], plano: PlanoMetas) {
  const { api, parametros } = apiFalsa(metas, plano);
  TestBed.configureTestingModule({
    providers: [provideRouter([{ path: 'metas', component: MetasComponent }]), { provide: ApiService, useValue: api }]
  });
  const harness = await RouterTestingHarness.create();
  const tela = await harness.navigateByUrl(url, MetasComponent);
  const router = TestBed.inject(Router);
  const atualizar = async () => { await harness.fixture.whenStable(); harness.detectChanges(); };
  await atualizar();
  return { harness, tela, router, raiz: harness.routeNativeElement as HTMLElement, atualizar, parametros };
}
