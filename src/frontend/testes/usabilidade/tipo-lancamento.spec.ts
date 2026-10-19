import { of } from 'rxjs';
import { Conta, TipoConta, TipoLancamento, TipoTransacao } from '../../src/app/core/modelos';
import { abrirTransacoes, paginaVazia } from '../apoio/transacoes';
import { transacao } from '../apoio/transacoes-edicao';

/**
 * Usabilidade do tipo de lançamento em Transações (#111, ADR 0025). Nielsen 2: "Pix enviado", "Compra no cartão",
 * na linguagem de quem usa; Nielsen 6: reconhecer na própria linha, sem abrir a transação; WCAG 1.4.1 e 1.1.1: o
 * ícone é decorativo e o texto sempre aparece; Nielsen 10: o termo tem ⓘ do glossário.
 */
describe('Usabilidade: tipo de lançamento', () => {
  const CONTAS = [
    { id: 3, nome: 'Nubank conta', tipo: TipoConta.ContaCorrente, banco: 'nubank', arquivada: false },
    { id: 4, nome: 'Itaú Click', tipo: TipoConta.CartaoCredito, banco: 'itau', arquivada: false }
  ] as unknown as Conta[];
  const ITENS = [
    transacao({ id: 1, descricao: 'JOAO SILVA', contaId: 3, contaNome: 'Nubank conta', tipoLancamento: TipoLancamento.PixEnviado }),
    transacao({ id: 2, descricao: 'MERCADO LIVRE', contaId: 4, contaNome: 'Itaú Click', tipoLancamento: TipoLancamento.CompraCartao }),
    transacao({ id: 3, descricao: 'PAGAMENTO FATURA', contaId: 3, contaNome: 'Nubank conta', tipoLancamento: TipoLancamento.PagamentoFatura,
      tipo: TipoTransacao.Transferencia, ignorarRelatorios: true })
  ];
  const abrir = () => abrirTransacoes('/transacoes', (f) => ({ ...paginaVazia(f, ITENS.length), itens: ITENS }),
    { contas: () => of(CONTAS) });
  const linha = (raiz: HTMLElement, i: number) => raiz.querySelectorAll<HTMLElement>('tbody tr')[i];

  it('cada linha diz o tipo em texto, com ícone decorativo', async () => {
    const t = await abrir();
    const etiquetas = Array.from(t.elemento.querySelectorAll('.etiqueta-lancamento')).map((e) => e.textContent!.trim());
    expect(etiquetas).toEqual(['Pix enviado', 'Compra no cartão', 'Pagamento de fatura']);
    expect(t.elemento.querySelector('.etiqueta-lancamento app-icone svg')!.getAttribute('aria-hidden')).toBe('true');
  });

  it('o que não é gasto aparece escrito ("fora dos gastos"), não só na cor', async () => {
    const t = await abrir();
    expect(linha(t.elemento, 2).textContent).toContain('fora dos gastos');
    expect(linha(t.elemento, 0).textContent).not.toContain('fora dos gastos');
  });

  it('filtro "Lançamento" tem rótulo ligado ao campo e ⓘ do glossário', async () => {
    const t = await abrir();
    const select = t.elemento.querySelector<HTMLSelectElement>('#filtro-lancamento')!;
    const rotulo = t.elemento.querySelector(`label[for="${select.id}"]`)!;
    expect(rotulo.textContent).toContain('Lançamento');
    expect(rotulo.querySelector('[aria-label="O que é Tipo de lançamento?"]')).not.toBeNull();
    expect(Array.from(select.options).map((o) => o.textContent!.trim())).toContain('Pagamento de fatura');
  });

  it('a coluna Conta mostra o banco (selo decorativo) e diz quando é cartão', async () => {
    const t = await abrir();
    const conta = (i: number) => linha(t.elemento, i).querySelector('.conta-transacao')!;
    // O texto do selo (sigla desenhada) é aria-hidden; o nome vem no texto ao lado.
    const nome = (i: number) => conta(i).querySelector(':scope > span')!.textContent!.replace(/\s+/g, ' ').trim();
    expect(nome(1)).toBe('Itaú Click cartão de crédito');
    expect(nome(0)).toBe('Nubank conta');
    expect(conta(1).querySelector('app-logo-banco svg')!.getAttribute('aria-hidden')).toBe('true');
  });
});
