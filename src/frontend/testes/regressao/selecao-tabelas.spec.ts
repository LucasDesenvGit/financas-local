import { TestBed } from '@angular/core/testing';
import { AvisosService } from '../../src/app/compartilhado/avisos.component';
import { abrirSelecao } from '../apoio/selecao';

/**
 * Regressão #133: seleção de vários registros em Transações — Shift+clique para intervalo, estado parcial no
 * "selecionar todas", a seleção que continua ao trocar de página e é limpa (com aviso) ao trocar o filtro, e a ação em
 * lote só nas selecionadas.
 */
describe('Regressão: seleção de várias transações', () => {
  it('Shift+clique seleciona o intervalo entre a última caixa usada e esta', async () => {
    const t = await abrirSelecao();
    await t.clicar(0);
    await t.clicar(2, true);
    expect(t.ids()).toEqual([1, 2, 3]);
    // Shift+clique desmarcando aplica o "desmarcar" ao intervalo entre a última usada (3ª) e esta (2ª)
    await t.clicar(1, true);
    expect(t.ids()).toEqual([1]);
  });

  it('"Selecionar todas desta página" fica parcial com só algumas marcadas; clicar nele marca todas da página', async () => {
    const t = await abrirSelecao();
    expect(t.todas().indeterminate).toBe(false);
    await t.clicar(1);
    expect(t.todas().indeterminate).toBe(true);
    expect(t.todas().checked).toBe(false);
    t.todas().click();
    await t.assentar();
    expect(t.ids()).toEqual([1, 2, 3, 4]);
    expect(t.todas().checked).toBe(true);
    expect(t.todas().indeterminate).toBe(false);
    t.todas().click();
    await t.assentar();
    expect(t.ids()).toEqual([]);
  });

  it('trocar de página mantém a seleção e a barra diz quantas estão em outras páginas', async () => {
    const t = await abrirSelecao();
    await t.clicar(0);
    await t.clicar(1);
    t.tela.buscar(2);
    await t.assentar();
    expect(t.ids()).toEqual([1, 2]);
    await t.clicar(0);           // id 5, na página 2
    expect(t.elemento.querySelector('.contagem-selecao')!.textContent!.trim()).toBe('3 selecionadas (2 em outras páginas)');
  });

  it('trocar o filtro limpa a seleção, com o aviso "Seleção limpa ao trocar o filtro."', async () => {
    const t = await abrirSelecao();
    const info = jest.spyOn(TestBed.inject(AvisosService), 'info');
    await t.clicar(0);
    t.tela.filtro.busca = 'mercado';
    t.tela.buscar(1);
    await t.assentar();
    expect(t.ids()).toEqual([]);
    expect(info).toHaveBeenCalledWith('Seleção limpa ao trocar o filtro.');
  });

  it('Esc na tabela limpa a seleção', async () => {
    const t = await abrirSelecao();
    await t.clicar(0);
    t.caixas()[0].dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    await t.assentar();
    expect(t.ids()).toEqual([]);
  });

  it('clicar na célula da caixa não abre a edição da transação', async () => {
    const t = await abrirSelecao();
    const editar = jest.spyOn(t.tela, 'editar');
    (t.caixas()[0].closest('td') as HTMLElement).click();
    await t.assentar();
    expect(editar).not.toHaveBeenCalled();
  });

  it('a ação em lote vale só para as selecionadas (inclusive as de outras páginas) e limpa a seleção depois', async () => {
    let enviados: number[] = [];
    const t = await abrirSelecao();
    (t.tela as unknown as { api: { marcarEmergencia: unknown } }).api.marcarEmergencia = (ids: number[]) => {
      enviados = ids;
      return { subscribe: (o: { next: (r: unknown) => void }) => o.next({ transacoesAtualizadas: ids.length }) };
    };
    await t.clicar(0);
    t.tela.buscar(2);
    await t.assentar();
    await t.clicar(1);           // id 6
    t.tela.marcarEmergenciaLote(true);
    await t.assentar();
    expect(enviados.sort()).toEqual([1, 6]);
    expect(t.ids()).toEqual([]);
  });
});
