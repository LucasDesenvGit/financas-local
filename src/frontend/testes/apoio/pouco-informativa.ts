import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
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
    poucoInformativas: 0, poucoInformativasDesde: null, exemploPoucoInformativa: null, ...campos
  };
}

/** Abre o Painel com a API falsa (só GET /painel). */
export async function abrirPainel(dados: Painel) {
  TestBed.configureTestingModule({
    providers: [provideRouter([]), { provide: ApiService, useValue: { painel: () => of(dados) } }]
  });
  const fixture = TestBed.createComponent(PainelComponent);
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
  return { fixture, raiz: fixture.nativeElement as HTMLElement };
}
