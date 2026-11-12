import { Conta, FormaCobranca, OrigemConta, Reserva } from '../../../src/app/core/modelos';
import {
  aceitaCofrinho, baixaSozinha, cofrinhoEncontrado, mostrarPagar, opcoesCofrinho, podeLancar, textoPagamentoCofrinho
} from '../../../src/app/paginas/movimentacao/pago-com-cofrinho';
import { abrirContasAPagar, contaAgendada } from '../apoio/contas-agendadas';
import { diferencaComoGasto, podeRegistrarSaida } from '../../../src/app/paginas/patrimonio/reserva-conferencia';

/**
 * #691 (ADR 0063): no Mercado Pago, o débito automático pago com um cofrinho sai direto da reserva e não chega pelo
 * Open Finance. A conta a pagar marca "Pago com" o cofrinho, e o app lança a despesa.
 */
describe('#691: pago com o cofrinho', () => {
  const reserva = (campos: Partial<Reserva>): Reserva => ({
    contaId: 7, nomeConta: 'Mercado Pago', nome: 'Emergência', saldo: 900, guardado: 900, retirado: 0, movimentos: 3,
    primeiroMovimento: null, ultimoMovimento: null, saldoConferido: null, dataConferencia: null,
    podeTerSaldoAnterior: false, conferenciaAntiga: false, ...campos
  });
  const conta = (origem: OrigemConta, id = 7) => ({ id, origem } as Conta);

  it('o cofrinho só vale em conta do Open Finance e fora do cartão', () => {
    expect(aceitaCofrinho(conta(OrigemConta.OpenFinance), FormaCobranca.DebitoAutomatico)).toBe(true);
    expect(aceitaCofrinho(conta(OrigemConta.OpenFinance), FormaCobranca.AMao)).toBe(true);
    expect(aceitaCofrinho(conta(OrigemConta.OpenFinance), FormaCobranca.CartaoCredito)).toBe(false);
    expect(aceitaCofrinho(conta(OrigemConta.Manual), FormaCobranca.AMao)).toBe(false);
    expect(aceitaCofrinho(undefined, FormaCobranca.AMao)).toBe(false);
  });

  it('as opções são os cofrinhos da conta, pelo nome, sem o restante do banco nem o Méli Dólar; o que sumiu fica, marcado', () => {
    const reservas = [reserva({ nome: 'Viagem' }), reserva({ nome: 'Emergência' }), reserva({ contaId: 8, nome: 'Outra' }),
      reserva({ nome: 'Reservado', restanteDoBanco: true }), reserva({ nome: 'Méli Dólar', moeda: 'USD' })];
    expect(opcoesCofrinho(reservas, 7, null).map((o) => o.nome)).toEqual(['Emergência', 'Viagem']);
    expect(opcoesCofrinho(reservas, 7, 'emergência')).toHaveLength(2);
    expect(opcoesCofrinho(reservas, 7, 'Antigo').at(-1)).toEqual({ nome: 'Antigo', saldo: null });
    expect(opcoesCofrinho(reservas, null, 'Antigo')).toEqual([]);
  });

  it('avisa o cofrinho que não está mais nas reservas da conta, sem diferenciar maiúsculas', () => {
    const reservas = [reserva({})];
    expect(cofrinhoEncontrado(contaAgendada({ contaId: 7, reservaNome: 'EMERGÊNCIA' }), reservas)).toBe(true);
    expect(cofrinhoEncontrado(contaAgendada({ contaId: 7, reservaNome: 'Viagem' }), reservas)).toBe(false);
    expect(cofrinhoEncontrado(contaAgendada({ contaId: 7, reservaNome: null }), reservas)).toBe(true);
  });

  it('débito automático pelo cofrinho dá baixa sozinho; o "Pagar" só aparece se ficou vencida (baixa desfeita)', () => {
    const debito = contaAgendada({ formaCobranca: FormaCobranca.DebitoAutomatico, reservaNome: 'Emergência', diasParaVencer: 3 });
    expect(baixaSozinha(debito)).toBe(true);
    expect(mostrarPagar(debito)).toBe(false);
    expect(mostrarPagar({ ...debito, diasParaVencer: -2 })).toBe(true);
    expect(mostrarPagar({ ...debito, ativa: false, diasParaVencer: -2 })).toBe(false);
    const aMao = contaAgendada({ reservaNome: 'Emergência' });
    expect(baixaSozinha(aMao)).toBe(false);
    expect(mostrarPagar(aMao)).toBe(true);
  });

  it('o "Pagar" lança a despesa na conta manual e, no Open Finance, só pelo cofrinho', () => {
    const contas = [conta(OrigemConta.OpenFinance, 7), conta(OrigemConta.Manual, 9)];
    expect(podeLancar(contaAgendada({ contaId: 7, reservaNome: 'Emergência' }), contas)).toBe(true);
    expect(podeLancar(contaAgendada({ contaId: 7 }), contas)).toBe(false);
    expect(podeLancar(contaAgendada({ contaId: 9 }), contas)).toBe(true);
    expect(podeLancar(contaAgendada({ contaId: null }), contas)).toBe(false);
  });

  it('o texto da última baixa diz de onde saiu, quando e se foi sozinha', () => {
    const u = { transacaoId: 1, data: '2026-11-05', valor: 120, reservaNome: 'Emergência', automatico: true, podeDesfazer: true };
    expect(textoPagamentoCofrinho(u)).toBe('paga pelo cofrinho Emergência em 05/11 (automático)');
    expect(textoPagamentoCofrinho({ ...u, automatico: false })).toBe('paga pelo cofrinho Emergência em 05/11');
  });

  it('editar pelo aviso "Atualizar o valor" mantém o cofrinho (não volta para o saldo da conta)', () => {
    const { tela, salvos } = abrirContasAPagar([], [contaAgendada({ id: 4, reservaNome: 'Emergência', contaId: 7 })]);
    tela.atualizarPeloAviso({ contaAgendadaId: 4, descricao: 'Escola', tipo: 'aumentou', texto: '', valorAnterior: 1450,
      valorNovo: 1500, contaNovaId: null, contaNovaNome: null, data: null });
    expect(salvos[0]).toMatchObject({ valor: 1500, reservaNome: 'Emergência' });
  });
});


/** #691, parte 2: a diferença da conferência que pode virar gasto do cofrinho, e onde cabe "registrar saída". */
describe('#691: diferença da conferência e saída do cofrinho', () => {
  const reserva = (campos: Partial<Reserva>): Reserva => ({
    contaId: 7, nomeConta: 'Mercado Pago', nome: 'Emergência', saldo: 900, guardado: 900, retirado: 0, movimentos: 3,
    primeiroMovimento: null, ultimoMovimento: null, saldoConferido: null, dataConferencia: null,
    podeTerSaldoAnterior: false, conferenciaAntiga: false, moeda: 'BRL', ...campos
  });
  const hoje = '2026-11-11';

  it('só abaixo da estimativa, com a data de hoje, numa reserva em reais e conferível', () => {
    expect(diferencaComoGasto(reserva({}), 850.45, hoje, hoje)).toBe(49.55);
    expect(diferencaComoGasto(reserva({}), 900, hoje, hoje)).toBe(0);
    expect(diferencaComoGasto(reserva({}), 950, hoje, hoje)).toBe(0);
    expect(diferencaComoGasto(reserva({}), null, hoje, hoje)).toBe(0);
    expect(diferencaComoGasto(reserva({}), 850, '2026-11-10', hoje)).toBe(0);
    expect(diferencaComoGasto(reserva({ moeda: 'USD' }), 850, hoje, hoje)).toBe(0);
    expect(diferencaComoGasto(reserva({ informadoPeloBanco: true }), 850, hoje, hoje)).toBe(0);
    expect(diferencaComoGasto(reserva({ restanteDoBanco: true }), 850, hoje, hoje)).toBe(0);
  });

  it('"registrar saída" em toda reserva em reais, menos no restante do banco', () => {
    expect(podeRegistrarSaida(reserva({}))).toBe(true);
    expect(podeRegistrarSaida(reserva({ informadoPeloBanco: true }))).toBe(true);
    expect(podeRegistrarSaida(reserva({ moeda: 'USD' }))).toBe(false);
    expect(podeRegistrarSaida(reserva({ restanteDoBanco: true }))).toBe(false);
  });
});
