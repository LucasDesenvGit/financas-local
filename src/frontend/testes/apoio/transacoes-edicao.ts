import { of } from 'rxjs';
import {
  AlterarCategoriasEmLote, FiltroTransacoes, SalvarRegra, StatusTransacao, TipoTransacao, Transacao
} from '../../src/app/core/modelos';
import { abrirTransacoes, paginaVazia } from './transacoes';

/** Transação de exemplo (despesa do Open Finance, categoria Mercado). */
export function transacao(campos: Partial<Transacao> = {}): Transacao {
  return {
    id: 1, contaId: 3, contaNome: 'Nubank', openFinance: true, data: '2026-10-10', descricao: 'PADARIA PAO QUENTE',
    descricaoOriginal: 'PADARIA PAO QUENTE', estabelecimento: null, valor: -25, tipo: TipoTransacao.Despesa,
    status: StatusTransacao.Efetivada, categoriaId: 12, categoriaNome: 'Mercado', categoriaCor: '#f2d98b',
    parcelaNumero: null, parcelaTotal: null, ignorarRelatorios: false, observacao: null, emergencia: false, ...campos
  };
}

/** Três transações: duas iguais (padaria) e uma sem categoria. */
export const ITENS: Transacao[] = [
  transacao({ id: 1 }),
  transacao({ id: 2, data: '2026-10-09' }),
  transacao({ id: 3, descricao: 'PIX ENVIADO', descricaoOriginal: 'PIX ENVIADO', categoriaId: null, categoriaNome: null, categoriaCor: null })
];

/**
 * Abre Transações (roteador de verdade, URL lida e gravada) com a API falsa do modo edição. Guarda os lotes
 * enviados, os "Desfazer" e as regras criadas.
 */
export async function abrirEdicao(url = '/transacoes?editar=1', itens: Transacao[] = ITENS, total = itens.length) {
  const lotes: AlterarCategoriasEmLote[] = [];
  const desfeitos: string[] = [];
  const regras: SalvarRegra[] = [];
  const categorizadosAntigo: number[][] = [];
  const t = await abrirTransacoes(url, (f: FiltroTransacoes) => ({ ...paginaVazia(f, total), itens }), {
    alterarCategoriasEmLote: (dto: AlterarCategoriasEmLote) => {
      lotes.push(JSON.parse(JSON.stringify(dto)));
      return of({ alteradas: dto.itens?.length ?? total, desfazer: 'chave-1' });
    },
    desfazerCategoriasEmLote: (chave: string) => { desfeitos.push(chave); return of({ transacoesAtualizadas: 2 }); },
    criarRegra: (dto: SalvarRegra) => { regras.push(dto); return of({ id: 9, ...dto }); },
    categorizarEmLote: (ids: number[]) => { categorizadosAntigo.push(ids); return of({ transacoesAtualizadas: ids.length }); }
  });
  const selectDa = (id: number) => {
    const linha = Array.from(t.elemento.querySelectorAll('tbody tr'))[itens.findIndex((x) => x.id === id)];
    return linha.querySelector<HTMLSelectElement>('select.select-categoria')!;
  };
  /** Escolhe a opção pelo valor ('auto' ou o id) como o usuário faria. */
  const escolher = async (id: number, valor: string) => {
    const s = selectDa(id);
    s.value = valor;
    s.dispatchEvent(new Event('change'));
    await t.assentar();
  };
  const botao = (texto: string) =>
    Array.from(t.elemento.querySelectorAll<HTMLButtonElement>('button')).find((b) => b.textContent!.trim().startsWith(texto))!;
  return { ...t, lotes, desfeitos, regras, categorizadosAntigo, selectDa, escolher, botao };
}
