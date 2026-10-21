import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { of } from 'rxjs';
import { ApiService } from '../../src/app/core/api.service';
import { PainelAnual } from '../../src/app/core/modelos';
import { AnualComponent } from '../../src/app/paginas/anual.component';

/**
 * Regressão: no "Resultado acumulado" da Visão anual, o preenchimento ia da linha até o zero (`fill: true`). Nos meses
 * com acumulado negativo, a faixa ficava acima da linha e parecia invertida. Agora vai até a base (`fill: 'start'`).
 */
describe('Regressão: Visão anual, preenchimento do resultado acumulado embaixo da linha', () => {
  it('a série do ano atual preenche até a base do gráfico, não até o zero', async () => {
    Object.defineProperty(window, 'matchMedia', {
      configurable: true, writable: true,
      value: () => ({ matches: false, addEventListener: () => undefined, removeEventListener: () => undefined })
    });
    const resumo = { receitas: 0, despesas: 0, resultado: 0, taxaPoupanca: 0 };
    const dados = {
      ano: 2026, anoComparacao: 2025, mesLimite: 9, anosDisponiveis: [2026], resumo,
      resumoAnteriorMesmoPeriodo: resumo, resumoAnteriorCompleto: resumo, meses: [], categorias: [],
      distribuicao: { receitas: 0, essenciais: 0, estiloDeVida: 0, naoClassificado: 0, poupanca: 0 },
      saude: { mesesDeReserva: null, mesesDeReservaEssencial: null, comprometimentoParcelas: null, parcelasMensais: 0,
        parcelasFinanciamentos: 0, parcelasEmprestimos: 0 }
    } as unknown as PainelAnual;
    TestBed.configureTestingModule({
      providers: [provideRouter([{ path: 'anual', component: AnualComponent }]),
        { provide: ApiService, useValue: { painelAnual: () => of(dados) } }]
    });
    const harness = await RouterTestingHarness.create();
    const tela = await harness.navigateByUrl('/anual', AnualComponent);
    const [atual] = tela.graficoAcumulado().data.datasets as unknown as { fill: unknown }[];
    expect(atual.fill).toBe('start');
  });
});
