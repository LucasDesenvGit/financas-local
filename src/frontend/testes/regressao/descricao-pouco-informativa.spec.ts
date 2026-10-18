import { TestBed } from '@angular/core/testing';
import { of } from 'rxjs';
import { AvisosService } from '../../src/app/compartilhado/avisos.component';
import { dataIso } from '../../src/app/compartilhado/utilidades';
import { abrirTransacoes, paginaVazia, parametrosDaUrl } from '../apoio/transacoes';
import { transacao } from '../apoio/transacoes-edicao';
import { abrirPainel, painel } from '../apoio/painel';

/** Regressão do filtro "Descrição pouco informativa" (#9): URL, busca, revisada com desfazer e link do Painel. */
describe('Regressão: descrição pouco informativa', () => {
  const comItens = (url: string, extras: Record<string, unknown> = {}) => abrirTransacoes(url,
    (f) => ({ ...paginaVazia(f, 2), itens: [transacao({ id: 7, descricao: 'PIX ENVIADO' }), transacao({ id: 8, descricao: 'TED 123' })] }),
    extras);

  it('?generica=1 abre filtrada e manda poucoInformativa para a API', async () => {
    const t = await comItens('/transacoes?generica=1');
    expect(t.buscas[0]).toMatchObject({ poucoInformativa: true });
  });

  it('marcar o filtro grava generica=1 na URL; o ✕ do chip tira', async () => {
    const t = await comItens('/transacoes');
    t.tela.filtro.poucoInformativa = true;
    t.tela.buscar(1);
    await t.assentar();
    expect(parametrosDaUrl(t.router)).toEqual({ generica: '1' });
    t.elemento.querySelector<HTMLButtonElement>('[aria-label="Remover filtro: Descrição pouco informativa"]')!.click();
    await t.assentar();
    expect(parametrosDaUrl(t.router)).toEqual({});
    expect(t.buscas.at(-1)).toMatchObject({ poucoInformativa: false });
  });

  it('"marcar como revisada" manda o id, recarrega a lista e o aviso desfaz', async () => {
    const chamadas: [number[], boolean][] = [];
    const t = await comItens('/transacoes?generica=1', {
      marcarRevisada: (ids: number[], revisada: boolean) => { chamadas.push([ids, revisada]); return of({ transacoesAtualizadas: ids.length }); }
    });
    const buscasAntes = t.buscas.length;
    t.elemento.querySelector<HTMLButtonElement>('[aria-label="Marcar como revisada: TED 123"]')!.click();
    expect(chamadas).toEqual([[[8], true]]);
    expect(t.buscas.length).toBe(buscasAntes + 1);
    const aviso = TestBed.inject(AvisosService).avisos().find((a) => a.acao)!;
    expect(aviso.texto).toBe('1 transação(ões) marcada(s) como revisada(s).');
    aviso.acao!.executar();
    expect(chamadas[1]).toEqual([[8], false]);
  });

  it('marcar as selecionadas como revisadas manda todas de uma vez', async () => {
    const chamadas: number[][] = [];
    const t = await comItens('/transacoes?generica=1', {
      marcarRevisada: (ids: number[]) => { chamadas.push(ids); return of({ transacoesAtualizadas: ids.length }); }
    });
    t.tela.alternarTodas();
    t.harness.detectChanges();
    Array.from(t.elemento.querySelectorAll<HTMLButtonElement>('.barra-lote button'))
      .find((b) => b.textContent!.trim() === 'Marcar como revisadas')!.click();
    expect(chamadas).toEqual([[7, 8]]);
  });

  it('o "Revisar" do Painel abre Transações com o filtro e o mesmo período do aviso', async () => {
    const { raiz } = await abrirPainel(painel({ poucoInformativas: 3, poucoInformativasDesde: '2026-07-20' }));
    const link = raiz.querySelector<HTMLAnchorElement>('.nota-discreta a')!;
    const url = new URL(link.href, 'http://localhost');
    expect(url.pathname).toBe('/transacoes');
    expect(Object.fromEntries(url.searchParams)).toEqual({ generica: '1', inicio: '2026-07-20', fim: dataIso() });
  });
});
