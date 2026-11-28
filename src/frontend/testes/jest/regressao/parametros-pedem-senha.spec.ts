import { Parametro, TipoParametro } from '../../../src/app/core/modelos';
import { mudaParametroComSenha } from '../../../src/app/paginas/ajustes/parametrizacao';

// #68: mudar um parâmetro de segurança do login pede a senha atual; os outros, não.
function parametro(chave: string, valor: number, pedeSenha: boolean): Parametro {
  return { chave, grupo: pedeSenha ? 'Segurança' : 'Painel', nome: chave, descricao: '', tipo: TipoParametro.Inteiro,
    valor, padrao: valor, minimo: 1, maximo: 1440, unidade: null, pedeSenha };
}

const lista = [parametro('Painel.DiasProximosVencimentos', 15, false), parametro('Seguranca.MinutosBloqueio', 15, true)];

describe('mudaParametroComSenha', () => {
  it.each([
    ['nada mudou', { 'Painel.DiasProximosVencimentos': 15, 'Seguranca.MinutosBloqueio': 15 }, false],
    ['só um parâmetro que não é de segurança', { 'Painel.DiasProximosVencimentos': 20, 'Seguranca.MinutosBloqueio': 15 }, false],
    ['o tempo de bloqueio', { 'Painel.DiasProximosVencimentos': 15, 'Seguranca.MinutosBloqueio': 1 }, true],
    ['o campo de número devolve texto', { 'Painel.DiasProximosVencimentos': 15, 'Seguranca.MinutosBloqueio': '15' as unknown as number }, false],
    ['o campo foi apagado', { 'Painel.DiasProximosVencimentos': 15, 'Seguranca.MinutosBloqueio': null as unknown as number }, true]
  ])('%s', (_nome, valores, esperado) => {
    expect(mudaParametroComSenha(lista, valores)).toBe(esperado);
  });
});
