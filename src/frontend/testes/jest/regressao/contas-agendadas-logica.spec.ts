import { Conta, FormaCobranca, OrigemConta, Recorrencia, Reserva, TipoConta } from '../../../src/app/core/modelos';
import {
  bancoDe, chavesIgnoradasDe, chavesParaGuardar, corpoDoAviso, corpoParaSalvar, dataLimitePelosRestantes, ehCartao, formularioDaConta,
  formularioDaSugestao, formularioVazio, limiteInvalido, restantesDoForm, separarSugestoes, textoDiasParaVencer, totalProximos30,
  totalVencido
} from '../../../src/app/paginas/movimentacao/contas-agendadas';
import {
  aberturaPeloAviso, corpoDoPagamento, lancarNoPagar, textoDesfazerPagamento, textoLancarPeloCofrinho
} from '../../../src/app/paginas/movimentacao/pago-com-cofrinho';
import { contaAgendada, sugestao } from '../apoio/contas-agendadas';

/**
 * #777 (ADR 0059): as regras de Contas a pagar que saíram da tela para funções puras, testadas sem o `TestBed`. Os
 * que a tela já cobria pela interface (a data limite, "é a mesma", o texto do "Pagar") seguem nos testes dela.
 */
describe('Regressão: regras de Contas a pagar em funções puras (#777)', () => {
  const conta = (campos: Partial<Conta>) => ({ id: 7, origem: OrigemConta.OpenFinance, ...campos } as Conta);
  const reserva = (campos: Partial<Reserva>) => ({ contaId: 7, nome: 'Emergência', saldo: 900, ...campos } as Reserva);

  describe('lista e totais', () => {
    const itens = [
      contaAgendada({ id: 1, valor: 100, diasParaVencer: -3 }),
      contaAgendada({ id: 2, valor: 50, diasParaVencer: 0 }),
      contaAgendada({ id: 3, valor: 70, diasParaVencer: 30 }),
      contaAgendada({ id: 4, valor: 900, diasParaVencer: 31 }),
      contaAgendada({ id: 5, valor: 400, diasParaVencer: -1, ativa: false })
    ];

    it('"Vencidas" soma só as ativas que já passaram; "Próximos 30 dias", as de hoje a 30 dias', () => {
      expect(totalVencido(itens)).toBe(100);
      expect(totalProximos30(itens)).toBe(120);
    });

    it('o texto do vencimento: vencida há Nd, vence hoje ou em N dias', () => {
      expect(textoDiasParaVencer(-3)).toBe('vencida há 3d');
      expect(textoDiasParaVencer(0)).toBe('vence hoje');
      expect(textoDiasParaVencer(25)).toBe('em 25 dias');
    });

    it('o banco e o cartão vêm da conta escolhida; sem conta, selo neutro', () => {
      const contas = [conta({ id: 3, banco: 'itau', tipo: TipoConta.ContaCorrente }), conta({ id: 4, banco: 'nubank', tipo: TipoConta.CartaoCredito })];
      expect(bancoDe(contas, 3)).toBe('itau');
      expect(bancoDe(contas, null)).toBeNull();
      expect(bancoDe(contas, 99)).toBeNull();
      expect(ehCartao(contas, 4)).toBe(true);
      expect(ehCartao(contas, 3)).toBe(false);
      expect(ehCartao(contas, null)).toBe(false);
    });
  });

  describe('data limite do formulário', () => {
    const form = (campos = {}) => ({ ...formularioVazio('2026-11-10'), ...campos });

    it('a data limite antes do vencimento é inválida, só na conta ativa que se repete', () => {
      expect(limiteInvalido(form({ dataLimite: '2026-10-01' }))).toBe(true);
      expect(limiteInvalido(form({ dataLimite: '2026-11-10' }))).toBe(false);
      expect(limiteInvalido(form({ dataLimite: '2026-10-01', ativa: false }))).toBe(false);
      expect(limiteInvalido(form({ dataLimite: '2026-10-01', recorrencia: Recorrencia.Unica }))).toBe(false);
      expect(limiteInvalido(form())).toBe(false);
    });

    it('os pagamentos que faltam contam o próximo; sem data limite, ou com data antes do vencimento, não há conta', () => {
      expect(restantesDoForm(form({ dataLimite: '2027-01-10' }))).toBe(3);
      expect(restantesDoForm(form({ dataLimite: '2026-11-10' }))).toBe(1);
      expect(restantesDoForm(form({ dataLimite: '2026-10-01' }))).toBeNull();
      expect(restantesDoForm(form())).toBeNull();
    });

    it('"faltam N" vira a data do N-ésimo vencimento; valor vazio, zero ou quebrado apaga a data', () => {
      expect(dataLimitePelosRestantes(form(), 3)).toBe('2027-01-10');
      expect(dataLimitePelosRestantes(form(), '3')).toBe('2027-01-10');
      expect(dataLimitePelosRestantes(form(), 1)).toBe('2026-11-10');
      expect(dataLimitePelosRestantes(form({ recorrencia: Recorrencia.Anual }), 2)).toBe('2027-11-10');
      expect(dataLimitePelosRestantes(form(), 0)).toBeNull();
      expect(dataLimitePelosRestantes(form(), null)).toBeNull();
      expect(dataLimitePelosRestantes(form(), 'abc')).toBeNull();
      expect(dataLimitePelosRestantes(form({ proximoVencimento: '' }), 3)).toBeNull();
    });
  });

  describe('o que vai para a API ao salvar', () => {
    const contas = [conta({ id: 7 }), conta({ id: 9, origem: OrigemConta.Manual })];

    it('conta única sai sem data limite; o texto da cobrança só vale para a cobrada sozinha', () => {
      const base = { ...formularioVazio('2026-11-10'), descricao: 'Escola', valor: 100, dataLimite: '2027-06-10' };
      expect(corpoParaSalvar({ ...base, recorrencia: Recorrencia.Unica }, contas).dataLimite).toBeNull();
      expect(corpoParaSalvar(base, contas).dataLimite).toBe('2027-06-10');
      expect(corpoParaSalvar({ ...base, textoCobranca: 'FIAP' }, contas).textoCobranca).toBeNull();
      expect(corpoParaSalvar({ ...base, formaCobranca: FormaCobranca.DebitoAutomatico, textoCobranca: ' FIAP ' }, contas).textoCobranca)
        .toBe('FIAP');
      expect(corpoParaSalvar({ ...base, formaCobranca: FormaCobranca.DebitoAutomatico, textoCobranca: '  ' }, contas).textoCobranca)
        .toBeNull();
    });

    it('o valor vira número, e a observação em branco vira nula', () => {
      expect(corpoParaSalvar({ ...formularioVazio(), valor: '12.5' as unknown as number, observacao: ' ok ' }, contas))
        .toMatchObject({ valor: 12.5, observacao: 'ok' });
      expect(corpoParaSalvar({ ...formularioVazio(), observacao: '  ' }, contas).observacao).toBeNull();
    });

    it('o cofrinho só vale em conta do Open Finance, fora do cartão', () => {
      const base = { ...formularioVazio(), reservaNome: ' Emergência ' };
      expect(corpoParaSalvar({ ...base, contaId: 7 }, contas).reservaNome).toBe('Emergência');
      expect(corpoParaSalvar({ ...base, contaId: 9 }, contas).reservaNome).toBeNull();
      expect(corpoParaSalvar({ ...base, contaId: 7, formaCobranca: FormaCobranca.CartaoCredito }, contas).reservaNome).toBeNull();
      expect(corpoParaSalvar({ ...base, contaId: null }, contas).reservaNome).toBeNull();
    });

    it('o formulário da conta traz o que ela tem, e o da sugestão guarda a chave do extrato (#205)', () => {
      const c = contaAgendada({ id: 3, descricao: 'Luz', reservaNome: 'Emergência', contaId: 7, dataLimite: '2027-01-10' });
      expect(formularioDaConta(c)).toMatchObject({ descricao: 'Luz', reservaNome: 'Emergência', contaId: 7, dataLimite: '2027-01-10' });
      expect(formularioDaConta(c).chaveExtrato).toBeUndefined();
      expect(formularioDaSugestao(sugestao({ chaveExtrato: 'LIVEPIX', recorrencia: undefined, formaCobranca: undefined })))
        .toMatchObject({ chaveExtrato: 'LIVEPIX', recorrencia: Recorrencia.Mensal, formaCobranca: FormaCobranca.AMao, reservaNome: null });
    });

    it('os avisos "Atualizar o valor" e "Atualizar o catálogo" mudam só o valor ou só a conta', () => {
      const c = contaAgendada({ id: 4, valor: 44.9, contaId: 3, reservaNome: 'Emergência' });
      const aviso = { contaAgendadaId: 4, descricao: 'Streaming', texto: '', valorAnterior: 44.9, valorNovo: 49.9, contaNovaId: 9,
        contaNovaNome: 'Nubank', data: null };
      expect(corpoDoAviso(c, { ...aviso, tipo: 'aumentou' })).toMatchObject({ valor: 49.9, contaId: 3, reservaNome: 'Emergência' });
      expect(corpoDoAviso(c, { ...aviso, tipo: 'mudou-de-cartao' })).toMatchObject({ valor: 44.9, contaId: 9 });
      expect(corpoDoAviso(c, { ...aviso, tipo: 'aumentou', valorNovo: null }).valor).toBe(44.9);
    });
  });

  describe('sugestões ignoradas, guardadas só neste navegador', () => {
    it('separa as visíveis das ignoradas pela chave', () => {
      const lista = [sugestao({ chave: 'a' }), sugestao({ chave: 'b' }), sugestao({ chave: 'c' })];
      const r = separarSugestoes(lista, ['b', 'zzz']);
      expect(r.visiveis.map((s) => s.chave)).toEqual(['a', 'c']);
      expect(r.ignoradas.map((s) => s.chave)).toEqual(['b']);
    });

    it('lê só lista de texto; o resto (vazio, JSON quebrado, outro tipo) vira lista vazia', () => {
      expect(chavesIgnoradasDe('["a","b"]')).toEqual(['a', 'b']);
      expect(chavesIgnoradasDe('["a",1,null]')).toEqual(['a']);
      expect(chavesIgnoradasDe(null)).toEqual([]);
      expect(chavesIgnoradasDe('{"a":1}')).toEqual([]);
      expect(chavesIgnoradasDe('{quebrado')).toEqual([]);
    });

    it('guarda sem repetir e só as 200 últimas', () => {
      expect(chavesParaGuardar(['a', 'b', 'a'])).toEqual(['a', 'b']);
      const muitas = Array.from({ length: 250 }, (_, i) => `k${i}`);
      const guardadas = chavesParaGuardar(muitas);
      expect(guardadas).toHaveLength(200);
      expect(guardadas[0]).toBe('k50');
      expect(guardadas.at(-1)).toBe('k249');
    });
  });

  describe('a janela "Pagar"', () => {
    const contas = [conta({ id: 7 }), conta({ id: 9, origem: OrigemConta.Manual })];
    const escolha = { data: '2026-11-10', lancar: true, cofrinho: null, guardarCofrinho: true };

    it('a caixa de lançar já vem marcada onde o app pode lançar, e sempre no "foi paga com um cofrinho"', () => {
      const manual = contaAgendada({ contaId: 9 });
      const aberta = contaAgendada({ contaId: 7 });
      expect(lancarNoPagar({ conta: manual, data: '', cofrinho: null }, contas)).toBe(true);
      expect(lancarNoPagar({ conta: aberta, data: '', cofrinho: null }, contas)).toBe(false);
      expect(lancarNoPagar({ conta: aberta, data: '', cofrinho: '' }, contas)).toBe(true);
    });

    it('o aviso "foi paga com um cofrinho" abre com a data do vencimento e o primeiro cofrinho da conta', () => {
      const c = contaAgendada({ contaId: 7 });
      const reservas = [reserva({ nome: 'Viagem' }), reserva({ nome: 'Emergência' }), reserva({ contaId: 8, nome: 'Outra' })];
      const aviso = { contaAgendadaId: 1, descricao: 'Luz', tipo: 'nao-veio' as const, texto: '', valorAnterior: null, valorNovo: null,
        contaNovaId: null, contaNovaNome: null, data: '2026-10-15' };
      expect(aberturaPeloAviso(c, aviso, reservas)).toEqual({ conta: c, data: '2026-10-15', cofrinho: 'Emergência' });
      expect(aberturaPeloAviso(c, { ...aviso, data: null }, [], '2026-11-22')).toEqual({ conta: c, data: '2026-11-22', cofrinho: '' });
    });

    it('sem cofrinho, só lança onde o app pode; com cofrinho, a despesa é sempre lançada e o nome vai sem espaços', () => {
      expect(corpoDoPagamento(contaAgendada({ contaId: 9 }), contas, escolha)).toEqual({ lancar: true, data: '2026-11-10', cofrinho: undefined });
      expect(corpoDoPagamento(contaAgendada({ contaId: 7 }), contas, escolha).lancar).toBe(false);
      expect(corpoDoPagamento(contaAgendada({ contaId: 9 }), contas, { ...escolha, lancar: false }).lancar).toBe(false);
      expect(corpoDoPagamento(contaAgendada({ contaId: 7 }), contas, { ...escolha, lancar: false, cofrinho: ' Emergência ', guardarCofrinho: false }))
        .toEqual({ lancar: true, data: '2026-11-10', cofrinho: { reservaNome: 'Emergência', guardarCofrinho: false } });
      expect(corpoDoPagamento(contaAgendada({ contaId: 9 }), contas, { ...escolha, cofrinho: '  ' }).cofrinho).toBeUndefined();
    });

    it('os textos das confirmações dizem a conta, o cofrinho e o dia', () => {
      const u = { transacaoId: 1, data: '2026-11-05', valor: 120, reservaNome: 'Emergência', automatico: true, podeDesfazer: true };
      const c = contaAgendada({ descricao: 'Condomínio', reservaNome: 'Emergência' });
      expect(textoDesfazerPagamento(c, u)).toContain('Apagar a despesa de Condomínio paga pelo cofrinho Emergência');
      expect(textoDesfazerPagamento(c, u)).toContain('use “Pagar”');
      expect(textoDesfazerPagamento(c, { ...u, automatico: false })).not.toContain('use “Pagar”');
      expect(textoLancarPeloCofrinho(c, '2026-11-08')).toContain('Lançar a despesa de Condomínio de 08/11');
    });
  });
});
