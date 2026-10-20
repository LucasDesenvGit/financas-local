import { signal } from '@angular/core';
import { of } from 'rxjs';
import { Conexao, InvestimentoComparado } from '../../src/app/core/modelos';
import { ConfiguracoesComponent } from '../../src/app/paginas/configuracoes.component';
import { InvestimentosComponent } from '../../src/app/paginas/investimentos.component';
import { abrirPainel, painel } from '../apoio/painel';

/**
 * #170: a caixinha do Nubank aparecia como "CDB - NU FINANCEIRA S.A. - SOCIEDADE…" e as conexões todas como
 * "MeuPluggy"; os trocos das compras de FII voltavam a contar como rendimento sem ninguém avisar.
 */
describe('Regressão: investimentos com nome reconhecível (#170)', () => {
  const caixinha = { id: 1, nome: 'CDB - NU FINANCEIRA S.A. - SOCIEDADE DE CREDITO', nomeExibicao: 'CDB · Nubank',
    nomePersonalizado: null } as InvestimentoComparado;

  function renomear(nome: string) {
    const pedidos: [number, string | null][] = [];
    const sucesso = jest.fn();
    const tela = {
      formNome: { investimento: caixinha, nome } as { investimento: InvestimentoComparado; nome: string } | null,
      salvando: signal(false),
      api: { renomearInvestimento: (id: number, n: string | null) => { pedidos.push([id, n]); return of(undefined); } },
      avisos: { sucesso, erro: jest.fn() },
      carregar: jest.fn()
    };
    InvestimentosComponent.prototype.salvarNome.call(tela);
    return { pedidos, sucesso, tela };
  }

  it('renomear manda o nome sem espaços nas pontas, fecha e recarrega', () => {
    const { pedidos, sucesso, tela } = renomear('  Caixinha viagem ');
    expect(pedidos).toEqual([[1, 'Caixinha viagem']]);
    expect(tela.formNome).toBeNull();
    expect(tela.carregar).toHaveBeenCalled();
    expect(sucesso).toHaveBeenCalledWith('Investimento renomeado.');
  });

  it('nome vazio volta ao automático (manda null)', () => {
    const { pedidos, sucesso } = renomear('   ');
    expect(pedidos).toEqual([[1, null]]);
    expect(sucesso).toHaveBeenCalledWith('O investimento voltou ao nome automático.');
  });

  it('a conexão mostra o banco; o conector só quando não há banco', () => {
    const nome = (campos: Partial<Conexao>) =>
      ConfiguracoesComponent.prototype.nomeConexao.call({}, { nomeInstituicao: 'MeuPluggy', nomeExibicao: null, ...campos } as Conexao);
    expect(nome({ nomeExibicao: 'Nubank' })).toBe('Nubank');
    expect(nome({})).toBe('MeuPluggy');
    expect(nome({ nomeInstituicao: null })).toBe('Instituição');
  });

  it('o Painel avisa dos trocos contando como rendimento e leva a Conferir investimentos', async () => {
    const { raiz } = await abrirPainel(painel({ creditosCorretoraComoReceita: 3, totalCreditosCorretoraComoReceita: 12.5 }));
    const aviso = Array.from(raiz.querySelectorAll('.nota-discreta')).find((p) => p.textContent!.includes('troco'))!;
    // O ⓘ é o botão do glossário que explica "troco".
    expect(aviso.textContent!.replace('ⓘ', '').replace(/\s+/g, ' ')).toContain('3 créditos de troco ou venda de FIIs e ações (R$ 12,50)');
    expect(aviso.querySelector('a[href^="/investimentos"]')!.getAttribute('href')).toBe('/investimentos?conferir=1');
  });

  it('sem trocos, o Painel não mostra o aviso', async () => {
    const { raiz } = await abrirPainel(painel());
    expect(Array.from(raiz.querySelectorAll('.nota-discreta')).some((p) => p.textContent!.includes('troco'))).toBe(false);
  });
});
