import {
  TEXTO_MUITOS, avisoDaSincronizacao, paramsDaSincronizacao, textoDaConexaoAdicionada
} from '../../../src/app/compartilhado/aviso-sincronizacao';
import { ResultadoSincronizacao } from '../../../src/app/core/modelos';
import { urlVeioDeFora } from '../../../src/app/paginas/movimentacao/transacoes';

/**
 * #688: o aviso "Sincronizado: X nova(s), Y atualizada(s)." ganha "Ver em Transações", que abre só esses lançamentos
 * (Nielsen 1; Shneiderman, feedback informativo). Sem nada novo, o aviso fica sem ação; com mais de 200, a ação abre o
 * período, e o aviso diz por quê.
 */
describe('Regressão: aviso da sincronização com "Ver em Transações" (#688)', () => {
  const resultado = (r: Partial<ResultadoSincronizacao> = {}): ResultadoSincronizacao =>
    ({ sucesso: true, transacoesNovas: 2, transacoesAtualizadas: 1, mensagem: 'Sincronização concluída.', lancamentos: null, ...r });
  const lancamentos = { ids: [12, 40, 41], inicio: '2026-10-02', fim: '2026-10-08' };

  it('com novas e atualizadas, a ação leva aos ids, no período deles, com o chip da sincronização', () => {
    const aviso = avisoDaSincronizacao(resultado({ lancamentos }));
    expect(aviso.texto).toBe('Sincronizado: 2 nova(s), 1 atualizada(s).');
    expect(aviso.verEmTransacoes).toEqual({ inicio: '2026-10-02', fim: '2026-10-08', ids: '12,40,41', de: 'sincronizacao' });
  });

  it('sem nada novo nem atualizado, o aviso fica como era, sem ação', () => {
    const aviso = avisoDaSincronizacao(resultado({ transacoesNovas: 0, transacoesAtualizadas: 0 }));
    expect(aviso).toEqual({ texto: 'Sincronizado: 0 nova(s), 0 atualizada(s).', verEmTransacoes: null });
  });

  it('com mais de 200 (a lista vem vazia), a ação abre o período sem o filtro de ids, e o aviso diz por quê', () => {
    const aviso = avisoDaSincronizacao(resultado({ transacoesNovas: 350, lancamentos: { ids: [], inicio: '2025-10-15', fim: '2026-10-14' } }));
    expect(aviso.texto).toBe('Sincronizado: 350 nova(s), 1 atualizada(s).' + TEXTO_MUITOS);
    expect(aviso.verEmTransacoes).toEqual({ inicio: '2025-10-15', fim: '2026-10-14' });
  });

  it('o "Adicionar" conexão mantém o próprio texto e ganha a mesma ação', () => {
    const r = resultado({ transacoesNovas: 3, lancamentos });
    const aviso = avisoDaSincronizacao(r, textoDaConexaoAdicionada(r));
    expect(aviso.texto).toBe('Conexão adicionada: 3 transação(ões) importada(s).');
    expect(aviso.verEmTransacoes).toEqual(paramsDaSincronizacao(lancamentos));
  });

  it('resposta sem o campo (versão antiga da API) não quebra: sem ação', () => {
    expect(paramsDaSincronizacao(undefined)).toBeNull();
  });
});

/**
 * #688 (ADR 0018): com Transações aberta, a URL pode mudar por fora (o "Ver em Transações" do aviso, o voltar do
 * navegador): a tela relê o filtro. Não relê na navegação que a abriu nem na URL que ela mesma gravou.
 */
describe('Regressão: Transações relê a URL que mudou por fora (#688)', () => {
  const gravada = '/transacoes?inicio=2026-11-01&fim=2026-11-30';
  it.each([
    ['a navegação que abriu a tela', 1, '/transacoes?ids=1,2&de=sincronizacao', false],
    ['a URL que a própria tela gravou', 2, gravada, false],
    ['o "Ver em Transações" com a tela aberta', 3, '/transacoes?ids=1,2&de=sincronizacao', true],
    ['o voltar do navegador para a lista de antes', 4, '/transacoes', true]
  ])('%s', (_nome, navegacao, url, rele) => {
    expect(urlVeioDeFora(navegacao, url, 1, gravada)).toBe(rele);
  });
});
