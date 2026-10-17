import { TestBed } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { of } from 'rxjs';
import { ApiService } from '../../src/app/core/api.service';
import { ErroSistema, FiltroErros, NivelErro, PaginaErros } from '../../src/app/core/modelos';
import { ErrosComponent } from '../../src/app/paginas/erros.component';

/** Erro de exemplo: falha da API repetida 3 vezes, em aberto. */
export function erro(campos: Partial<ErroSistema> = {}): ErroSistema {
  return {
    id: 1, primeiraEm: '2026-10-16T12:00:00Z', ultimaEm: '2026-10-16T15:30:00Z', ocorrencias: 3, nivel: NivelErro.Erro,
    origem: 'API', mensagem: "Erro não tratado em /api/metas: String 'x' was not recognized as a valid DateOnly.",
    rota: 'GET /api/metas', usuario: 'lucas', resolvido: false, resolvidoEm: null, ...campos
  };
}

/**
 * Abre a aba de erros pelo roteador de verdade (a URL é lida e gravada como no navegador), com a API falsa.
 * Guarda cada filtro buscado, cada detalhe pedido e cada mudança de situação.
 */
export async function abrirErros(url = '/configuracoes?aba=erros', itens: ErroSistema[] = [erro()]) {
  const buscas: FiltroErros[] = [];
  const detalhes: number[] = [];
  const situacoes: [number, boolean][] = [];
  const resolvidosEmLote: FiltroErros[] = [];
  const api = {
    erros: (f: FiltroErros) => {
      buscas.push({ ...f });
      const p: PaginaErros = { itens, total: itens.length, pagina: f.pagina, tamanhoPagina: f.tamanhoPagina,
        abertos: itens.filter((i) => !i.resolvido).length, origens: ['API', 'Open Finance'] };
      return of(p);
    },
    erroDetalhe: (id: number) => {
      detalhes.push(id);
      return of({ id, categoria: 'Erro', detalhe: 'System.FormatException: x\n   at Metas.Listar()' });
    },
    situacaoErro: (id: number, resolvido: boolean) => { situacoes.push([id, resolvido]); return of(undefined); },
    resolverErros: (f: FiltroErros) => { resolvidosEmLote.push({ ...f }); return of({ quantidade: itens.length }); }
  };
  TestBed.configureTestingModule({
    providers: [
      provideRouter([{ path: 'configuracoes', component: ErrosComponent }]),
      { provide: ApiService, useValue: api }
    ]
  });
  const harness = await RouterTestingHarness.create();
  const tela = await harness.navigateByUrl(url, ErrosComponent);
  const router = TestBed.inject(Router);
  const assentar = async () => {
    await harness.fixture.whenStable();
    harness.detectChanges();
  };
  await assentar();
  const raiz = harness.routeNativeElement as HTMLElement;
  return { harness, tela, router, raiz, assentar, buscas, detalhes, situacoes, resolvidosEmLote };
}
