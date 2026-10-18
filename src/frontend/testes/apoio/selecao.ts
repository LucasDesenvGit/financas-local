import { FiltroTransacoes, PaginaTransacoes } from '../../src/app/core/modelos';
import { abrirTransacoes } from './transacoes';
import { transacao } from './transacoes-edicao';

/** 6 transações em páginas de 4: a página 1 tem os ids 1–4 e a página 2, os ids 5–6. */
export function seisTransacoes(f: FiltroTransacoes): PaginaTransacoes {
  const todas = [1, 2, 3, 4, 5, 6].map((id) => transacao({ id, descricao: `PIX ENVIADO ${id}`, data: `2026-10-${10 + id}`, valor: -id * 10 }));
  const pagina = f.pagina ?? 1;
  return { itens: todas.slice((pagina - 1) * 4, pagina * 4), total: 6, pagina, tamanhoPagina: 4, totalReceitas: 0, totalDespesas: 60 };
}

/** Abre Transações com as 6 transações e atalhos para as caixas de seleção e a barra. */
export async function abrirSelecao(url = '/transacoes') {
  const t = await abrirTransacoes(url, seisTransacoes);
  const caixas = () => Array.from(t.elemento.querySelectorAll<HTMLInputElement>('tbody .col-check input[type="checkbox"]'));
  const todas = () => t.elemento.querySelector<HTMLInputElement>('thead .col-check input[type="checkbox"]')!;
  /** Clique na caixa da linha `i` (o navegador marca/desmarca; o componente lê o estado), opcionalmente com Shift. */
  const clicar = async (i: number, shift = false) => {
    caixas()[i].dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, shiftKey: shift }));
    await t.assentar();
  };
  const ids = () => [...t.tela.selecionadas()].sort((a, b) => a - b);
  const barra = () => t.elemento.querySelector<HTMLElement>('.barra-lote-espaco')!;
  return { ...t, caixas, todas, clicar, ids, barra };
}
