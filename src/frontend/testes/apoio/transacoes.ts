import { TestBed } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { of } from 'rxjs';
import { ApiService } from '../../src/app/core/api.service';
import { Categoria, Conta, FiltroTransacoes, PaginaTransacoes } from '../../src/app/core/modelos';
import { TransacoesComponent } from '../../src/app/paginas/transacoes.component';

export const CONTAS = [
  { id: 3, nome: 'Nubank', arquivada: false },
  { id: 4, nome: 'Itaú', arquivada: false }
] as unknown as Conta[];

export const CATEGORIAS = [
  { id: 12, nome: 'Mercado', cor: '#f2d98b', categoriaPaiId: null, ativa: true, tipo: 1, icone: null, natureza: null },
  { id: 20, nome: 'Lazer', cor: '#f3b3d3', categoriaPaiId: null, ativa: true, tipo: 1, icone: null, natureza: null },
  { id: 21, nome: 'Cinema', cor: '#d4a5e8', categoriaPaiId: 20, ativa: true, tipo: 1, icone: null, natureza: null }
] as unknown as Categoria[];

/** Página vazia do tamanho pedido; os testes trocam por outra quando precisam de total/itens. */
export function paginaVazia(f: FiltroTransacoes, total = 0): PaginaTransacoes {
  return { itens: [], total, pagina: f.pagina ?? 1, tamanhoPagina: f.tamanhoPagina ?? 50, totalReceitas: 0, totalDespesas: 0 };
}

export interface TelaTransacoes {
  harness: RouterTestingHarness;
  tela: TransacoesComponent;
  router: Router;
  /** Cópia de cada filtro enviado à API, na ordem. */
  buscas: FiltroTransacoes[];
  elemento: HTMLElement;
  /** Espera a navegação/URL assentar e redesenha. */
  assentar(): Promise<void>;
}

/**
 * Abre /transacoes pelo roteador de verdade (para a URL ser lida e gravada como no navegador), com a API falsa.
 * `resposta` define o que a API devolve para cada filtro (padrão: página vazia).
 */
export async function abrirTransacoes(
  url = '/transacoes',
  resposta: (f: FiltroTransacoes) => PaginaTransacoes = (f) => paginaVazia(f),
  /** Métodos a mais da API falsa (ex.: os do modo edição). */
  extras: Record<string, unknown> = {}
): Promise<TelaTransacoes> {
  const buscas: FiltroTransacoes[] = [];
  const api = {
    contas: () => of(CONTAS),
    categorias: () => of(CATEGORIAS),
    transacoes: (f: FiltroTransacoes) => {
      buscas.push({ ...f });
      return of(resposta({ ...f }));
    },
    ...extras
  };
  TestBed.configureTestingModule({
    providers: [
      provideRouter([{ path: 'transacoes', component: TransacoesComponent }, { path: 'painel', children: [] }]),
      { provide: ApiService, useValue: api }
    ]
  });
  // A tela rola para o topo ao trocar de página; o jsdom não implementa scrollTo.
  window.scrollTo = jest.fn() as unknown as typeof window.scrollTo;

  const harness = await RouterTestingHarness.create();
  const tela = await harness.navigateByUrl(url, TransacoesComponent);
  const router = TestBed.inject(Router);
  const assentar = async () => {
    await harness.fixture.whenStable();
    harness.detectChanges();
  };
  await assentar();
  return { harness, tela, router, buscas, elemento: harness.routeNativeElement as HTMLElement, assentar };
}

/** Parâmetros da URL atual como objeto (ordem não importa). */
export function parametrosDaUrl(router: Router): Record<string, string> {
  const params = router.parseUrl(router.url).queryParams;
  return Object.fromEntries(Object.entries(params).map(([k, v]) => [k, String(v)]));
}
