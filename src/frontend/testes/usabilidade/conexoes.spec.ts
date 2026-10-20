import { ConfiguracoesComponent } from '../../src/app/paginas/configuracoes.component';
import { Conexao } from '../../src/app/core/modelos';

/**
 * Usabilidade do bloco Conexões (Configurações › Open Finance): a situação vem em português, não com o código da
 * Pluggy (Nielsen 2), com o tom de cor certo (verde em dia, vermelho quando precisa de ação).
 */
describe('Usabilidade: situação das conexões em português', () => {
  const situacao = (campos: Partial<Conexao>) =>
    ConfiguracoesComponent.prototype.situacao.call({}, { ativa: true, status: 'UPDATED', ...campos } as Conexao);

  it.each([
    ['UPDATED', 'Atualizada', 'ok'],
    ['UPDATING', 'Atualizando…', 'atencao'],
    ['OUTDATED', 'Desatualizada', 'atencao'],
    ['LOGIN_ERROR', 'Erro de login no banco', 'erro'],
    ['WAITING_USER_INPUT', 'Precisa de ação sua', 'erro'],
    ['WAITING_USER_ACTION', 'Precisa de ação sua', 'erro']
  ])('%s → "%s" (%s)', (status, texto, tom) => {
    expect(situacao({ status })).toEqual({ texto, tom });
  });

  it('conexão pausada diz "Pausada", seja qual for o código', () => {
    expect(situacao({ ativa: false, status: 'UPDATED' })).toEqual({ texto: 'Pausada', tom: 'neutro' });
  });

  it('código desconhecido aparece como veio, com tom de atenção (nada some)', () => {
    expect(situacao({ status: 'OUTRO_CODIGO' })).toEqual({ texto: 'OUTRO_CODIGO', tom: 'atencao' });
  });
});
