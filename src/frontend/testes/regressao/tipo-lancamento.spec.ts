import { TipoLancamento, TipoTransacao, Transacao } from '../../src/app/core/modelos';
import { abrirTransacoes, paginaVazia, parametrosDaUrl } from '../apoio/transacoes';
import { transacao } from '../apoio/transacoes-edicao';

/** Regressão do tipo de lançamento (#111): filtro na URL, chip e os avisos de gasto. */
describe('Regressão: tipo de lançamento', () => {
  const com = (itens: Transacao[], url = '/transacoes') =>
    abrirTransacoes(url, (f) => ({ ...paginaVazia(f, itens.length), itens }));

  it('?lancamento=pix abre filtrada, manda o grupo para a API e mostra o chip "Pix"', async () => {
    const t = await com([], '/transacoes?lancamento=pix');
    expect(t.buscas[0]).toMatchObject({ lancamento: 'pix' });
    const chips = Array.from(t.elemento.querySelectorAll('.chip-filtro')).map((c) => c.textContent!.replace('✕', '').trim());
    expect(chips).toEqual(['Pix']);
  });

  it('grupo que não existe é ignorado e sai da URL', async () => {
    const t = await com([], '/transacoes?lancamento=foguete');
    expect(t.buscas[0].lancamento ?? null).toBeNull();
    expect(t.router.url).toBe('/transacoes');
  });

  it('escolher o filtro grava ?lancamento= na URL; o ✕ do chip tira', async () => {
    const t = await com([]);
    t.tela.filtro.lancamento = 'fatura';
    t.tela.buscar(1);
    await t.assentar();
    expect(parametrosDaUrl(t.router)).toEqual({ lancamento: 'fatura' });
    t.elemento.querySelector<HTMLButtonElement>('[aria-label="Remover filtro: Pagamento de fatura"]')!.click();
    await t.assentar();
    expect(parametrosDaUrl(t.router)).toEqual({});
    expect(t.buscas.at(-1)!.lancamento ?? null).toBeNull();
  });

  it('pagamento de fatura ou transferência entre suas contas nos totais avisa "somando nos gastos/nas receitas"', async () => {
    const t = await com([
      transacao({ id: 1, tipoLancamento: TipoLancamento.PagamentoFatura, tipo: TipoTransacao.Despesa, valor: -1210 }),
      transacao({ id: 2, tipoLancamento: TipoLancamento.EntreContas, tipo: TipoTransacao.Despesa, valor: -500 }),
      transacao({ id: 3, tipoLancamento: TipoLancamento.PagamentoFatura, tipo: TipoTransacao.Transferencia, ignorarRelatorios: true }),
      transacao({ id: 4, tipoLancamento: TipoLancamento.PixEnviado, tipo: TipoTransacao.Despesa }),
      // O crédito do pagamento no cartão, sem categoria de transferência, entra como receita.
      transacao({ id: 5, tipoLancamento: TipoLancamento.PagamentoFatura, tipo: TipoTransacao.Receita, valor: 1210 })
    ]);
    const avisos = Array.from(t.elemento.querySelectorAll('tbody tr')).map((tr) =>
      tr.querySelector('.etiqueta-alerta')?.textContent?.trim() ?? tr.querySelector('.etiqueta-suave')?.textContent?.trim() ?? '');
    expect(avisos).toEqual(['somando nos gastos', 'somando nos gastos', 'fora dos gastos', '', 'somando nas receitas']);
  });

  it('tipo desconhecido (API antiga ou valor novo) aparece como "Outro"', async () => {
    const t = await com([transacao({ id: 1, tipoLancamento: 99 as TipoLancamento })]);
    expect(t.elemento.querySelector('.etiqueta-lancamento')!.textContent!.trim()).toBe('Outro');
  });
});
