import { abrirContasAPagar, sugestao } from '../apoio/contas-agendadas';

/**
 * Regressão das sugestões de contas a pagar (#62/#63), com os pontos corrigidos na revisão do PR:
 * vencimento que já passou sem pagamento, link de pagamentos sem prender numa conta e chave do navegador no padrão.
 */
describe('Regressão: sugestões de contas a pagar', () => {
  it('pagamento esperado que não apareceu no extrato é avisado, e o cadastro usa o próximo vencimento (não um passado)', () => {
    const { raiz, botao, clicar, tela } = abrirContasAPagar([
      sugestao({ proximoVencimento: '2026-11-05', vencimentoSemPagamento: '2026-10-05', ultimoPagamento: '2026-09-05' })
    ]);
    expect(raiz.querySelector('section.sugestoes .texto-alerta')!.textContent).toContain('o de 05/10 ainda não apareceu no extrato');
    clicar(botao('Cadastrar BOLETO FIAP'));
    expect(tela.form).toMatchObject({ proximoVencimento: '2026-11-05', descricao: 'BOLETO FIAP', valor: 1450, contaId: 3, categoriaId: 5 });
  });

  it('sem vencimento perdido, não mostra aviso', () => {
    const { raiz } = abrirContasAPagar([sugestao()]);
    expect(raiz.querySelector('section.sugestoes .texto-alerta')).toBeNull();
  });

  it('"ver pagamentos" leva o período dos meses inteiros, despesa e a busca, sem filtrar por uma conta só', () => {
    const { tela } = abrirContasAPagar([]);
    expect(tela.linkPagamentos(sugestao({ primeiroPagamento: '2026-05-12', ultimoPagamento: '2026-10-08' }))).toEqual({
      inicio: '2026-05-01', fim: '2026-10-31', tipo: 'despesa', busca: 'FIAP'
    });
  });

  it('ignorar guarda a chave em financas.sugestoesContasIgnoradas e esconde a linha', () => {
    const { linhas, botao, clicar } = abrirContasAPagar([sugestao(), sugestao({ chave: 'condominio', descricao: 'CONDOMINIO' })]);
    expect(linhas()).toHaveLength(2);
    clicar(botao('Ignorar sugestão BOLETO FIAP'));
    expect(linhas()).toHaveLength(1);
    expect(JSON.parse(localStorage.getItem('financas.sugestoesContasIgnoradas')!)).toEqual(['escola fiap']);
  });

  it('se a API de sugestões falhar, a tela segue sem a seção (é complementar)', () => {
    const { raiz } = abrirContasAPagar('erro');
    expect(raiz.querySelector('section.sugestoes')).toBeNull();
    expect(raiz.querySelector('h1')!.textContent).toContain('Contas a pagar');
  });
});
