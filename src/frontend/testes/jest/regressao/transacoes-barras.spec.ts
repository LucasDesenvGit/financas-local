import { TestBed } from '@angular/core/testing';
import { of } from 'rxjs';
import { ConfirmacaoService } from '../../../src/app/compartilhado/ui/confirmacao.service';
import { abrirEdicao, ITENS } from '../apoio/transacoes-edicao';
import { abrirTransacoes, paginaVazia } from '../apoio/transacoes';

/**
 * #475 (ADR 0059): a barra "Editando categorias" e a das selecionadas são componentes de apresentação; a tela é quem
 * grava. Aqui, pelos controles das barras (não pelos métodos da tela), que cada um chega ao lugar certo.
 */
describe('Regressão: barras de edição e de seleção de Transações (#475)', () => {
  afterEach(() => jest.restoreAllMocks());

  /** Escolhe a opção `indice` de um select da página, como o usuário faria. */
  async function escolher(t: { elemento: HTMLElement; assentar(): Promise<void> }, seletor: string, indice: number) {
    const s = t.elemento.querySelector<HTMLSelectElement>(seletor)!;
    s.selectedIndex = indice;
    s.dispatchEvent(new Event('change'));
    await t.assentar();
  }

  it('"todas do filtro": a escolha no select da barra vai para a API, e o select volta a "escolher" depois', async () => {
    const t = await abrirEdicao('/transacoes?editar=1&categoria=sem', undefined, 240);
    jest.spyOn(TestBed.inject(ConfirmacaoService), 'confirmar').mockResolvedValueOnce(true);
    await escolher(t, '.todas-do-filtro select', 1);
    expect(t.tela.categoriaTodas).toBe('auto');
    t.botao('Aplicar a todas as 240').click();
    await t.assentar();
    await t.assentar();
    expect(t.lotes).toHaveLength(1);
    expect(t.lotes[0]).toMatchObject({ categoriaId: null, automatica: true, filtro: { semCategoria: true } });
    expect(t.tela.categoriaTodas).toBeNull();
    expect(t.elemento.querySelector<HTMLSelectElement>('.todas-do-filtro select')!.selectedIndex).toBe(0);
  });

  it('"Descartar" da barra pergunta e limpa as trocas pendentes', async () => {
    const t = await abrirEdicao();
    await t.escolher(1, '20');
    const confirmar = jest.spyOn(TestBed.inject(ConfirmacaoService), 'confirmar').mockResolvedValueOnce(true);
    t.botao('Descartar').click();
    await t.assentar();
    expect(confirmar).toHaveBeenCalled();
    expect(t.tela.pendentes().size).toBe(0);
  });

  it('no modo edição, a categoria escolhida na barra das selecionadas vira troca pendente', async () => {
    const t = await abrirEdicao();
    t.tela.alternar(1);
    t.tela.alternar(3);
    await t.assentar();
    await escolher(t, '.barra-lote select', 2);
    expect(t.tela.categoriaLote).toBe(20);
    t.botao('Trocar nas selecionadas').click();
    t.harness.detectChanges();   // sem whenStable: o aviso "troca(s) marcada(s)" deixa um timer pendente
    expect([...t.tela.pendentes()]).toEqual([[1, '20'], [3, '20']]);
    expect(t.tela.selecionadas().size).toBe(0);
  });

  it('fora do modo edição, "Aplicar categoria" e "Marcar emergência" da barra mandam as selecionadas para a API', async () => {
    const categorizadas: [number[], number | null][] = [];
    const emergencias: [number[], boolean][] = [];
    const t = await abrirTransacoes('/transacoes', (f) => ({ ...paginaVazia(f, ITENS.length), itens: ITENS }), {
      categorizarEmLote: (ids: number[], cat: number | null) => { categorizadas.push([ids, cat]); return of({ transacoesAtualizadas: ids.length }); },
      marcarEmergencia: (ids: number[], e: boolean) => { emergencias.push([ids, e]); return of({ transacoesAtualizadas: ids.length }); }
    });
    const botao = (texto: string) => [...t.elemento.querySelectorAll<HTMLButtonElement>('.barra-lote button')]
      .find((b) => b.textContent!.trim() === texto)!;
    t.tela.alternar(2);
    await t.assentar();
    await escolher(t, '.barra-lote select', 1);
    // Sem whenStable depois dos cliques: o aviso de sucesso deixa um timer de 10 s pendente.
    botao('Aplicar categoria').click();
    t.harness.detectChanges();
    expect(categorizadas).toEqual([[[2], 12]]);

    t.tela.alternar(1);
    t.harness.detectChanges();
    botao('Marcar emergência').click();
    t.harness.detectChanges();
    expect(emergencias).toEqual([[[1], true]]);
  });

  it('Esc na barra das selecionadas limpa a seleção', async () => {
    const t = await abrirEdicao('/transacoes');
    t.tela.alternar(1);
    await t.assentar();
    t.elemento.querySelector('.barra-lote')!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    await t.assentar();
    expect(t.tela.selecionadas().size).toBe(0);
  });
});
