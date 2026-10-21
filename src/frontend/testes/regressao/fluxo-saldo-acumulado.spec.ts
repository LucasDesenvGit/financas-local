import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { of } from 'rxjs';
import { linhaPorCimaDasBarras } from '../../src/app/compartilhado/grafico-misto';
import { ApiService } from '../../src/app/core/api.service';
import { MesValores } from '../../src/app/core/modelos';
import { RelatoriosComponent } from '../../src/app/paginas/relatorios.component';

/**
 * Regressão da #282: no Fluxo de caixa mensal, a linha do saldo acumulado ficava atrás das barras (no Chart.js, o
 * primeiro conjunto é desenhado por cima; `order` menor vem por cima). Agora ela vem na frente, com um ponto por mês,
 * o valor de cada mês aparece na tabela e "Emergências" sai da legenda quando não houve nenhuma.
 */
const mes = (m: number, receitas: number, despesas: number, despesasEmergencia = 0) =>
  ({ ano: 2026, mes: m, receitas, despesas, despesasEmergencia }) as MesValores;

async function abrir(fluxo: MesValores[]) {
  Object.defineProperty(window, 'matchMedia', {
    configurable: true, writable: true,
    value: () => ({ matches: false, addEventListener: () => undefined, removeEventListener: () => undefined })
  });
  TestBed.configureTestingModule({
    providers: [
      provideRouter([{ path: 'relatorios', component: RelatoriosComponent }]),
      { provide: ApiService, useValue: { relatorioFluxo: () => of(fluxo), relatorioCategorias: () => of([]), maioresDespesas: () => of([]) } }
    ]
  });
  const harness = await RouterTestingHarness.create();
  const tela = await harness.navigateByUrl('/relatorios?inicio=2026-05-01&fim=2026-07-31', RelatoriosComponent);
  await harness.fixture.whenStable();
  harness.detectChanges();
  return { tela, raiz: harness.routeNativeElement as HTMLElement };
}

describe('Regressão: linha do saldo acumulado por cima das barras (#282)', () => {
  it('a linha tem order menor que as barras e um ponto em cada mês', async () => {
    const { tela } = await abrir([mes(5, 11000, 10900), mes(6, 7000, 9000), mes(7, 13000, 11000)]);
    // A configuração como vai para o Chart.js: o <app-grafico> aplica o padrão de barras com linha.
    const conjuntos = linhaPorCimaDasBarras(tela.graficoFluxo()).data.datasets as unknown as { label: string; order: number; pointRadius?: number; data: number[] }[];
    const linha = conjuntos.find((c) => c.label === 'Saldo acumulado')!;
    const barras = conjuntos.filter((c) => c !== linha);
    expect(barras.every((b) => b.order > linha.order)).toBe(true);
    expect(linha.pointRadius).toBeGreaterThan(0);
    expect(linha.data).toEqual([100, -1900, 100]);
  });

  it('a tabela mostra o saldo acumulado de cada mês', async () => {
    const { raiz } = await abrir([mes(5, 11000, 10900), mes(6, 7000, 9000)]);
    const cabecalho = [...raiz.querySelectorAll('.tabela-meses th')].map((th) => th.textContent!.trim());
    expect(cabecalho.at(-1)).toBe('Saldo acumulado');
    const celulas = [...raiz.querySelectorAll('.tabela-meses .saldo-acumulado')];
    expect(celulas.map((c) => c.textContent!.replace(/\s+/g, ' ').trim())).toEqual(['R$ 100,00', '−R$ 1.900,00']);
    expect(celulas[1].classList).toContain('negativo');
  });

  it('"Emergências" só aparece na legenda quando houve alguma no período', async () => {
    const filtro = (tela: RelatoriosComponent) => {
      const f = tela.graficoFluxo().options!.plugins!.legend!.labels!.filter!;
      return (texto: string) => f({ text: texto } as never, {} as never);
    };
    const sem = await abrir([mes(5, 1000, 900)]);
    expect(filtro(sem.tela)('Emergências')).toBe(false);
    expect(filtro(sem.tela)('Receitas')).toBe(true);
    TestBed.resetTestingModule();
    const com = await abrir([mes(5, 1000, 900, 50)]);
    expect(filtro(com.tela)('Emergências')).toBe(true);
    // O "como ler" acompanha a legenda: o laranja só é citado quando houve emergência.
    expect(com.tela.comoLerFluxo()).toContain('laranja');
    expect(sem.tela.comoLerFluxo()).not.toContain('laranja');
  });
});
