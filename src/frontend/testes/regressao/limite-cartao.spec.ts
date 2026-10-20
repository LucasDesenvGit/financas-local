import { OrigemConta, TipoConta } from '../../src/app/core/modelos';
import { diferencaDoLimite, estadoDoLimite } from '../../src/app/paginas/limite-cartao';
import { conta } from '../apoio/contas';

/**
 * #171: com o limite reduzido no app, o limite usado (aprovado − disponível) fica ~15× a dívida. O site pergunta, e a
 * resposta muda o que a diferença é: limite tirado por você ("sim") ou compromisso ("não").
 */
describe('Regressão: limite do cartão reduzido no app (#171)', () => {
  // Aprovado R$ 10.150, deixado em R$ 1.000 no app, dívida R$ 511,72: limite usado R$ 9.661,72.
  const cartao = (campos = {}) => conta({ id: 4, nome: 'Gold', tipo: TipoConta.CartaoCredito, saldo: -511.72,
    limiteCredito: 10150, limiteDisponivel: 488.28, ...campos });

  it('limite usado bem maior que a dívida: pergunta', () => {
    expect(diferencaDoLimite(cartao())).toBeCloseTo(9150);
    expect(estadoDoLimite(cartao())).toBe('perguntar');
  });

  it('limite usado igual à dívida (ou diferença pequena): não pergunta', () => {
    expect(estadoDoLimite(cartao({ limiteDisponivel: 9638.28 }))).toBeNull();
    // Mais de 2× a dívida, mas só R$ 800 além dela.
    expect(estadoDoLimite(cartao({ saldo: -300, limiteDisponivel: 10150 - 1100 }))).toBeNull();
  });

  it('"sim" fica guardado; "não" só volta a perguntar se a diferença mudar', () => {
    expect(estadoDoLimite(cartao({ limiteAjustadoNoApp: true }))).toBe('ajustado');
    expect(estadoDoLimite(cartao({ limiteAjustadoNoApp: false, limiteDiferencaAvaliada: 9150 }))).toBe('nao');
    expect(estadoDoLimite(cartao({ limiteAjustadoNoApp: false, limiteDiferencaAvaliada: 9000 }))).toBe('nao');
    expect(estadoDoLimite(cartao({ limiteAjustadoNoApp: false, limiteDiferencaAvaliada: 4000 }))).toBe('perguntar');
  });

  it('cartão manual e conta que não é cartão não perguntam', () => {
    expect(estadoDoLimite(cartao({ origem: OrigemConta.Manual, limiteDisponivel: null }))).toBeNull();
    expect(estadoDoLimite(conta({ limiteCredito: 10150, limiteDisponivel: 0 }))).toBeNull();
  });
});
