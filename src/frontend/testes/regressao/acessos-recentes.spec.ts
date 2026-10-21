import { descreverEvento, origemDoAcesso } from '../../src/app/compartilhado/auditoria';

/**
 * Regressão da #284: em Configurações › Segurança › Acessos recentes, o evento aparecia como está no código
 * ("LoginSucesso") e a origem como 127.0.0.1. Agora o evento vem em português comum, com o detalhe entre parênteses,
 * e o endereço do próprio computador vira "Acesso local" (Nielsen 2, sem jargão). O desenho (data sem sobrepor o
 * evento) é conferido no navegador (estilos.spec.ts).
 */
describe('Regressão: Acessos recentes em português (#284)', () => {
  it('eventos conhecidos viram texto de gente', () => {
    expect(descreverEvento('LoginSucesso')).toBe('Entrou no site');
    expect(descreverEvento('LoginFalha')).toBe('Senha errada ao entrar');
    expect(descreverEvento('ParametrosAlterados')).toBe('Parâmetros alterados');
    expect(descreverEvento('CredencialPluggyAlterada')).toBe('Credencial da Pluggy alterada');
  });

  it('o detalhe entre parênteses continua', () => {
    expect(descreverEvento('RegraDescricaoCriada (22)')).toBe('Regra de descrição criada (22)');
    expect(descreverEvento('RenomearEmLote (5, 1 regra)')).toBe('Várias transações renomeadas (5, 1 regra)');
  });

  it('evento que não está no mapa aparece como veio (nada some)', () => {
    expect(descreverEvento('EventoNovo')).toBe('EventoNovo');
    expect(descreverEvento('EventoNovo (3)')).toBe('EventoNovo (3)');
  });

  it('endereço do próprio computador vira "Acesso local"; outro endereço aparece como veio', () => {
    expect(origemDoAcesso('127.0.0.1')).toBe('Acesso local');
    expect(origemDoAcesso('::1')).toBe('Acesso local');
    expect(origemDoAcesso('::ffff:127.0.0.1')).toBe('Acesso local');
    expect(origemDoAcesso('192.168.0.10')).toBe('192.168.0.10');
    expect(origemDoAcesso(null)).toBe('');
  });
});
