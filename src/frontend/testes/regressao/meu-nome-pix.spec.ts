import { signal } from '@angular/core';
import { of } from 'rxjs';
import { GLOSSARIO } from '../../src/app/compartilhado/glossario';
import { ConfiguracoesComponent } from '../../src/app/paginas/configuracoes.component';

/**
 * #172: sem titular informado pelo banco, um Pix para outra conta sua contava como despesa. O nome do usuário (como
 * aparece nos Pix) passa a valer, e a reserva de emergência explica por que ações e FIIs ficam de fora.
 */
describe('Regressão: robustez com os dados do banco (#172)', () => {
  function salvar(meuNome: string) {
    const pedidos: (string | null)[] = [];
    const sucesso = jest.fn();
    const tela = {
      meuNome,
      ocupado: signal(false),
      api: { salvarMeuNome: (nome: string | null) => { pedidos.push(nome); return of(undefined); } },
      avisos: { sucesso, erro: jest.fn() }
    };
    ConfiguracoesComponent.prototype.salvarMeuNome.call(tela);
    return { pedidos, sucesso, tela };
  }

  it('"Seu nome como aparece nos Pix" vai sem espaços nas pontas', () => {
    const { pedidos, sucesso, tela } = salvar('  Lucas Silva ');
    expect(pedidos).toEqual(['Lucas Silva']);
    expect(tela.meuNome).toBe('Lucas Silva');
    expect(sucesso).toHaveBeenCalledWith('Nome salvo. As transferências foram revistas.');
  });

  it('nome vazio apaga (manda null)', () => {
    const { pedidos, sucesso } = salvar('   ');
    expect(pedidos).toEqual([null]);
    expect(sucesso).toHaveBeenCalledWith('Nome apagado.');
  });

  it('o ⓘ da reserva de emergência diz o que entra e por que ações e FIIs ficam de fora', () => {
    expect(GLOSSARIO.reservaEmergencia.definicao).toContain('ações, FIIs e ETFs ficam de fora');
  });
});
