import { FormaCobranca, Recorrencia } from '../../src/app/core/modelos';
import { abrirContasAPagar, contaAgendada, sugestao } from '../apoio/contas-agendadas';

/**
 * #158 (ADR 0043): assinaturas no cartão e débitos automáticos em Contas a pagar. A sugestão diz como é cobrada, o
 * cadastro já vem com a forma, o texto da cobrança e a recorrência (anual também), e a conta cobrada sozinha não pede
 * "Pagar": ela se dá como paga quando a cobrança chega.
 */
describe('Regressão: assinaturas e cobranças recorrentes (#158)', () => {
  const netflix = sugestao({
    chave: 'netflix', descricao: 'NETFLIX.COM', valor: 44.9, valorMinimo: 44.9, valorMaximo: 44.9, contaId: 9,
    contaNome: 'Nubank Ultravioleta', termoBusca: 'NETFLIX', formaCobranca: FormaCobranca.CartaoCredito, textoCobranca: 'NETFLIX'
  });

  it('a sugestão do cartão diz onde cai, e o cadastro já vem com a forma e o texto da cobrança', () => {
    const { linhas, botao, clicar, tela } = abrirContasAPagar([netflix]);
    expect(linhas()[0].textContent).toContain('no cartão de crédito');
    clicar(botao('Cadastrar NETFLIX.COM'));
    expect(tela.form).toMatchObject({
      formaCobranca: FormaCobranca.CartaoCredito, textoCobranca: 'NETFLIX', contaId: 9, recorrencia: Recorrencia.Mensal
    });
  });

  it('a assinatura anual aparece como anual e é cadastrada como anual', () => {
    const anual = sugestao({ chave: 'amazon prime', descricao: 'AMAZON PRIME', recorrencia: Recorrencia.Anual,
      primeiroPagamento: '2025-11-03', ultimoPagamento: '2026-11-02', formaCobranca: FormaCobranca.CartaoCredito });
    const { linhas, botao, clicar, tela } = abrirContasAPagar([anual]);
    expect(linhas()[0].textContent).toContain('anual: pago em 11/2025 e 11/2026');
    clicar(botao('Cadastrar AMAZON PRIME'));
    expect(tela.form!.recorrencia).toBe(Recorrencia.Anual);
  });

  it('cobrada sozinha com o texto não tem "Pagar"; sem o texto, ou paga à mão, tem', () => {
    const { raiz } = abrirContasAPagar([], [
      contaAgendada({ id: 1, descricao: 'Netflix', formaCobranca: FormaCobranca.CartaoCredito, textoCobranca: 'NETFLIX' }),
      contaAgendada({ id: 2, descricao: 'Luz', formaCobranca: FormaCobranca.DebitoAutomatico, textoCobranca: null }),
      contaAgendada({ id: 3, descricao: 'Escola', formaCobranca: FormaCobranca.AMao })
    ]);
    const pagar = (nome: string) => raiz.querySelector(`[aria-label="Pagar ${nome}"]`);
    expect(pagar('Netflix')).toBeNull();
    expect(raiz.textContent).toContain('baixa sozinha quando a cobrança com “NETFLIX” chega');
    expect(pagar('Luz')).not.toBeNull();
    expect(pagar('Escola')).not.toBeNull();
  });

  it('o texto da cobrança só aparece (e só é enviado) para a cobrada sozinha', async () => {
    const { tela, raiz, atualizar, salvos } = abrirContasAPagar([]);
    tela.nova();
    await atualizar();
    const campoTexto = () => raiz.ownerDocument.querySelector('input[name="textoCobranca"]');
    expect(campoTexto()).toBeNull();
    tela.form!.formaCobranca = FormaCobranca.DebitoAutomatico;
    await atualizar();
    expect(campoTexto()).not.toBeNull();

    tela.form = { ...tela.form!, descricao: 'Escola', valor: 100, formaCobranca: FormaCobranca.AMao, textoCobranca: 'FIAP' };
    tela.salvar();
    expect(salvos.at(-1)).toMatchObject({ formaCobranca: FormaCobranca.AMao, textoCobranca: null });
  });
});
