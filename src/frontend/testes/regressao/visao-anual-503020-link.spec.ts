import { TestBed } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { of } from 'rxjs';
import { paramsTransacoes } from '../../src/app/compartilhado/utilidades';
import { ApiService } from '../../src/app/core/api.service';
import { DistribuicaoRenda, NaturezaGasto, PainelAnual, TipoTransacao } from '../../src/app/core/modelos';
import { AnualComponent } from '../../src/app/paginas/anual.component';
import { abrirTransacoes, parametrosDaUrl } from '../apoio/transacoes';

/**
 * Regressão: na Visão anual, cada parte de "Para onde vai a sua renda" (regra 50/30/20) leva a Transações com as
 * transações que a formam, no mesmo período do gráfico (detalhe por navegação, ADR 0018). O clique no gráfico é
 * atalho; a legenda em HTML é o caminho por teclado e toque (WCAG 2.1.1).
 */
describe('Regressão: 50/30/20 da Visão anual abre as transações de cada parte', () => {
  const resumo = { receitas: 10000, despesas: 7000, resultado: 3000, taxaPoupanca: 30 };

  function painel(distribuicao: Partial<DistribuicaoRenda> = {}): PainelAnual {
    return {
      ano: 2026, anoComparacao: 2025, mesLimite: 9, anosDisponiveis: [2026, 2025],
      resumo, resumoAnteriorMesmoPeriodo: resumo, resumoAnteriorCompleto: resumo, meses: [], categorias: [],
      distribuicao: { receitas: 10000, essenciais: 4000, estiloDeVida: 2500, naoClassificado: 500, poupanca: 3000, ...distribuicao },
      saude: {
        mesesDeReserva: null, mesesDeReservaEssencial: null, comprometimentoParcelas: null,
        parcelasMensais: 0, parcelasFinanciamentos: 0, parcelasEmprestimos: 0
      }
    } as unknown as PainelAnual;
  }

  async function abrirAnual(dados: PainelAnual) {
    // Os gráficos usam o tema, que lê prefers-color-scheme; o jsdom não tem matchMedia.
    Object.defineProperty(window, 'matchMedia', {
      configurable: true, writable: true,
      value: () => ({ matches: false, addEventListener: () => undefined, removeEventListener: () => undefined })
    });
    TestBed.configureTestingModule({
      providers: [
        provideRouter([{ path: 'anual', component: AnualComponent }, { path: 'transacoes', children: [] }]),
        { provide: ApiService, useValue: { painelAnual: () => of(dados) } }
      ]
    });
    const harness = await RouterTestingHarness.create();
    const tela = await harness.navigateByUrl('/anual', AnualComponent);
    await harness.fixture.whenStable();
    harness.detectChanges();
    return { tela, raiz: harness.routeNativeElement as HTMLElement, router: TestBed.inject(Router) };
  }

  function links(raiz: HTMLElement, router: Router) {
    return Array.from(raiz.querySelectorAll<HTMLAnchorElement>('.lista-legenda a')).map((a) => {
      const url = router.parseUrl(a.getAttribute('href')!);
      return {
        texto: a.textContent!.replace(/\s+/g, ' ').trim(),
        rota: url.root.children['primary'].segments.map((s) => s.path).join('/'),
        params: url.queryParams
      };
    });
  }

  const periodo = { inicio: '2026-01-01', fim: '2026-09-30' };

  it('paramsTransacoes leva as naturezas em ?natureza=, separadas por vírgula', () => {
    expect(paramsTransacoes({ tipo: 'despesa', naturezas: ['estilo-de-vida', 'sem'] }))
      .toEqual({ tipo: 'despesa', natureza: 'estilo-de-vida,sem' });
    expect(paramsTransacoes({ naturezas: [] })).toEqual({});
  });

  it('a legenda tem um link por parte, com o período do gráfico e o que forma cada parte', async () => {
    const { raiz, router } = await abrirAnual(painel());
    expect(links(raiz, router)).toEqual([
      { texto: 'Ver em Transações:Essenciais', rota: 'transacoes', params: { ...periodo, tipo: 'despesa', natureza: 'essencial' } },
      // A barra de estilo de vida soma o que está sem classificação; o link leva as duas coisas.
      { texto: 'Ver em Transações:Estilo de vida', rota: 'transacoes', params: { ...periodo, tipo: 'despesa', natureza: 'estilo-de-vida,sem' } },
      // Guardado = receitas − despesas: a lista traz as duas, e o "Resultado" dela é o que sobrou.
      { texto: 'Ver em Transações:Guardado', rota: 'transacoes', params: periodo }
    ]);
    expect(raiz.querySelector('.lista-legenda')!.textContent).toContain('40% da renda');
  });

  it('sem nada sem classificação, o link de estilo de vida não leva "sem classificação"', async () => {
    const { raiz, router } = await abrirAnual(painel({ estiloDeVida: 3000, naoClassificado: 0 }));
    expect(links(raiz, router)[1].params).toEqual({ ...periodo, tipo: 'despesa', natureza: 'estilo-de-vida' });
  });

  it('clicar num pedaço da barra "Você" abre as transações daquela parte; a barra de referência não navega', async () => {
    const { tela, router } = await abrirAnual(painel());
    const navegar = jest.spyOn(router, 'navigate').mockResolvedValue(true);
    expect(tela.soABarraVoce(0)).toBe(true);
    expect(tela.soABarraVoce(1)).toBe(false);

    tela.abrirParte503020({ indice: 0, serie: 1 });
    expect(navegar).toHaveBeenCalledWith(['/transacoes'],
      { queryParams: { ...periodo, tipo: 'despesa', natureza: 'estilo-de-vida,sem' } });

    navegar.mockClear();
    tela.abrirParte503020({ indice: 1, serie: 0 });
    expect(navegar).not.toHaveBeenCalled();
  });

  describe('Transações filtrada pela natureza', () => {
    it('?natureza= vira o filtro da API, numa busca só', async () => {
      const t = await abrirTransacoes('/transacoes?inicio=2026-01-01&fim=2026-09-30&tipo=despesa&natureza=estilo-de-vida,sem');
      expect(t.buscas).toHaveLength(1);
      expect(t.buscas[0]).toMatchObject({
        ...periodo, tipo: TipoTransacao.Despesa, naturezas: [NaturezaGasto.EstiloDeVida], semNatureza: true
      });
    });

    it('mostra um chip por natureza, com as palavras do gráfico', async () => {
      const t = await abrirTransacoes('/transacoes?natureza=essencial');
      const chips = Array.from(t.elemento.querySelectorAll('.chip-filtro')).map((c) => c.textContent!.replace('✕', '').trim());
      expect(chips).toEqual(['Essenciais']);
    });

    it('remover o chip tira só aquela natureza do filtro e da URL', async () => {
      const t = await abrirTransacoes('/transacoes?tipo=despesa&natureza=estilo-de-vida,sem');
      t.tela.removerFiltro('natureza:sem');
      await t.assentar();
      expect(t.buscas.at(-1)).toMatchObject({ naturezas: [NaturezaGasto.EstiloDeVida], semNatureza: false });
      expect(parametrosDaUrl(t.router)).toEqual({ tipo: 'despesa', natureza: 'estilo-de-vida' });
    });

    it('ignora natureza inválida ou repetida e "Limpar filtros" tira a natureza', async () => {
      const t = await abrirTransacoes('/transacoes?natureza=abc,essencial,constructor,essencial');
      expect(t.buscas[0]).toMatchObject({ naturezas: [NaturezaGasto.Essencial], semNatureza: false });
      expect(parametrosDaUrl(t.router)).toEqual({ natureza: 'essencial' });
      t.tela.limparFiltros();
      await t.assentar();
      expect(t.buscas.at(-1)).toMatchObject({ naturezas: [], semNatureza: false });
      expect(t.router.url).toBe('/transacoes');
    });
  });
});
