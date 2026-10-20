import { TestBed } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { of } from 'rxjs';
import { ApiService } from '../../src/app/core/api.service';
import { Painel } from '../../src/app/core/modelos';
import { PainelComponent } from '../../src/app/paginas/painel.component';

/** Painel mínimo: tudo zerado; os testes mudam só o que importa. */
export function painel(campos: Partial<Painel> = {}): Painel {
  return {
    saldoContas: 0, dividaCartoes: 0, totalInvestido: 0, patrimonioLiquido: 0, rendaVariavel: 0,
    patrimonioSemRendaVariavel: 0, receitasMes: 0, despesasMes: 0, gastosPorCategoria: [], evolucao: [], orcamentos: [],
    proximasContas: [], ultimasTransacoes: [], alertas: [], ultimaSincronizacao: null, despesasEmergenciaMes: 0,
    dividaFinanciamentos: 0, dividaEmprestimos: 0, diasProximosVencimentos: 15, faturasDoMes: [], totalFaturasDoMes: 0,
    poucoInformativas: 0, poucoInformativasDesde: null, exemploPoucoInformativa: null, creditosCorretoraComoReceita: 0,
    totalCreditosCorretoraComoReceita: 0, ...campos
  };
}

/**
 * Abre o Painel pelo roteador de verdade (o mês vai para a URL), com a API falsa (só GET /painel). `dados` pode ser
 * fixo ou depender do mês pedido; `chamadas` guarda cada (ano, mês) buscado.
 */
export async function abrirPainel(dados: Painel | ((ano: number, mes: number) => Painel), url = '/painel') {
  const chamadas: [number, number][] = [];
  const responder = typeof dados === 'function' ? dados : () => dados;
  TestBed.configureTestingModule({
    providers: [
      provideRouter([{ path: 'painel', component: PainelComponent }]),
      { provide: ApiService, useValue: { painel: (ano: number, mes: number) => { chamadas.push([ano, mes]); return of(responder(ano, mes)); } } }
    ]
  });
  const harness = await RouterTestingHarness.create();
  const tela = await harness.navigateByUrl(url, PainelComponent);
  await harness.fixture.whenStable();
  harness.detectChanges();
  const raiz = harness.routeNativeElement as HTMLElement;
  const atualizar = async () => { await harness.fixture.whenStable(); harness.detectChanges(); };
  return { harness, tela, raiz, chamadas, atualizar, router: TestBed.inject(Router) };
}
