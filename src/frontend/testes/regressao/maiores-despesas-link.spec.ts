import { TestBed } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { of } from 'rxjs';
import { AvisosService } from '../../src/app/compartilhado/avisos.component';
import { paramsTransacoes } from '../../src/app/compartilhado/utilidades';
import { ApiService } from '../../src/app/core/api.service';
import { TipoTransacao, Transacao } from '../../src/app/core/modelos';
import { RelatoriosComponent } from '../../src/app/paginas/relatorios.component';
import { abrirTransacoes, paginaVazia, parametrosDaUrl } from '../apoio/transacoes';
import { transacao } from '../apoio/transacoes-edicao';

/**
 * Regressão da #141: em Relatórios, cada uma das "Maiores despesas" leva a Transações no dia e na conta dela, com
 * ?transacao=id para a tela destacar a linha, rolar até ela e pôr o foco nela (detalhe por navegação, ADR 0018).
 */
describe('Regressão: Maiores despesas levam à transação em Transações', () => {
  beforeEach(() => {
    // O jsdom não implementa scrollIntoView.
    Element.prototype.scrollIntoView = jest.fn();
  });

  async function abrirRelatorios(maiores: Transacao[]) {
    // Os gráficos usam o tema, que lê prefers-color-scheme; o jsdom não tem matchMedia.
    Object.defineProperty(window, 'matchMedia', {
      configurable: true, writable: true,
      value: () => ({ matches: false, addEventListener: () => undefined, removeEventListener: () => undefined })
    });
    TestBed.configureTestingModule({
      providers: [
        provideRouter([{ path: 'relatorios', component: RelatoriosComponent }]),
        {
          provide: ApiService, useValue: {
            relatorioFluxo: () => of([]), relatorioCategorias: () => of([]), maioresDespesas: () => of(maiores)
          }
        }
      ]
    });
    const harness = await RouterTestingHarness.create();
    await harness.navigateByUrl('/relatorios?inicio=2026-09-01&fim=2026-09-30', RelatoriosComponent);
    await harness.fixture.whenStable();
    harness.detectChanges();
    return harness.routeNativeElement as HTMLElement;
  }

  it('paramsTransacoes leva o id da transação em ?transacao=', () => {
    expect(paramsTransacoes({ inicio: '2026-09-12', fim: '2026-09-12', tipo: 'despesa', contaId: 4, transacaoId: 42 }))
      .toEqual({ inicio: '2026-09-12', fim: '2026-09-12', tipo: 'despesa', conta: '4', transacao: '42' });
  });

  it('cada despesa é um link para Transações no dia e na conta dela, com o id', async () => {
    const raiz = await abrirRelatorios([transacao({ id: 42, contaId: 4, data: '2026-09-12T00:00:00', valor: -900 })]);
    const link = raiz.querySelector<HTMLAnchorElement>('.lista-maiores a')!;
    const url = TestBed.inject(Router).parseUrl(link.getAttribute('href')!);
    expect(url.root.children['primary'].segments.map((s) => s.path)).toEqual(['transacoes']);
    expect(url.queryParams).toEqual({ inicio: '2026-09-12', fim: '2026-09-12', tipo: 'despesa', conta: '4', transacao: '42' });
    // O nome do link diz para onde vai e contém o texto visível (descrição), para quem usa leitor de tela.
    expect(link.textContent!.replace(/\s+/g, ' ').trim()).toMatch(/^Ver em Transações:.*PADARIA PAO QUENTE/);
  });

  it('?transacao=id destaca a linha, rola até ela, põe o foco nela e fica na URL', async () => {
    const itens = [transacao({ id: 7 }), transacao({ id: 42 })];
    const t = await abrirTransacoes('/transacoes?inicio=2026-10-10&fim=2026-10-10&tipo=despesa&conta=3&transacao=42',
      (f) => ({ ...paginaVazia(f, itens.length), itens }));
    await t.assentar();
    expect(t.buscas).toHaveLength(1);
    expect(t.buscas[0]).toMatchObject({ inicio: '2026-10-10', fim: '2026-10-10', tipo: TipoTransacao.Despesa, contaId: 3 });
    const linha = t.elemento.querySelector<HTMLElement>('tr[data-transacao="42"]')!;
    expect(linha.classList).toContain('destacada');
    expect(linha.getAttribute('aria-current')).toBe('true');
    expect(t.elemento.querySelectorAll('tr.destacada')).toHaveLength(1);
    expect(linha.scrollIntoView).toHaveBeenCalled();
    expect(document.activeElement).toBe(linha);
    expect(parametrosDaUrl(t.router)).toMatchObject({ transacao: '42' });
  });

  it('trocar o filtro tira o destaque e o ?transacao= da URL', async () => {
    const itens = [transacao({ id: 42 })];
    const t = await abrirTransacoes('/transacoes?inicio=2026-10-10&fim=2026-10-10&transacao=42',
      (f) => ({ ...paginaVazia(f, itens.length), itens }));
    t.tela.limparFiltros();
    await t.assentar();
    expect(t.elemento.querySelector('tr.destacada')).toBeNull();
    expect(parametrosDaUrl(t.router)).not.toHaveProperty('transacao');
  });

  it('transação que não está na lista: avisa e tira o ?transacao= (o F5 não repete o aviso)', async () => {
    // No protótipo: o TestBed ainda não foi montado, e o aviso some sozinho antes do whenStable terminar.
    const info = jest.spyOn(AvisosService.prototype, 'info');
    const t = await abrirTransacoes('/transacoes?inicio=2026-10-10&fim=2026-10-10&transacao=99',
      (f) => ({ ...paginaVazia(f, 1), itens: [transacao({ id: 42 })] }));
    expect(info).toHaveBeenCalledTimes(1);
    expect(info).toHaveBeenCalledWith(expect.stringContaining('não aparece neste filtro'));
    expect(t.elemento.querySelector('tr.destacada')).toBeNull();
    expect(parametrosDaUrl(t.router)).toEqual({ inicio: '2026-10-10', fim: '2026-10-10' });
  });

  it.each(['abc', '0', '-3', '1.5'])('?transacao=%s é ignorado e os outros filtros continuam valendo', async (valor) => {
    const t = await abrirTransacoes(`/transacoes?tipo=despesa&transacao=${valor}`,
      (f) => ({ ...paginaVazia(f, 1), itens: [transacao({ id: 42 })] }));
    expect(t.buscas[0]).toMatchObject({ tipo: TipoTransacao.Despesa });
    expect(t.elemento.querySelector('tr.destacada')).toBeNull();
    expect(parametrosDaUrl(t.router)).toEqual({ tipo: 'despesa' });
  });
});
