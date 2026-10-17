import { TestBed } from '@angular/core/testing';
import { AvisosService } from '../../src/app/compartilhado/avisos.component';
import { TipoOperacao } from '../../src/app/core/modelos';
import { abrirConferencia, conferencia, rendimento, troco } from '../apoio/conferencia';

/**
 * Regressão de "Conferir investimentos" (ADR 0027): o GARE11 entrou como ação, a compra de 15/06 ficou fora do
 * histórico da corretora e o troco das compras contava como rendimento. Cada correção vai para a lista certa da API.
 */
describe('Regressão: Conferir investimentos', () => {
  it('a compra que faltou vem com o débito sugerido e vai ligada a ele, com cotas e preço', async () => {
    const t = await abrirConferencia();
    t.botao('Corrigir selecionados').click();
    t.fixture.detectChanges();
    expect(t.correcoes).toHaveLength(1);
    expect(t.correcoes[0].compras).toEqual([
      { transacaoId: 1177, ticker: 'GARE11', tipo: TipoOperacao.Compra, quantidade: 72, precoUnitario: 8.1826, taxas: 0 }
    ]);
    expect(t.correcoes[0].ativosParaFii).toEqual([10]);
  });

  it('troco vai na lista de trocos e provento na de rendimentos (nunca os dois juntos)', async () => {
    const t = await abrirConferencia(conferencia({
      tipos: [], cotas: [], trocos: [troco({ transacaoId: 1 }), troco({ transacaoId: 2, vendaTicker: 'BTLG12', vendaQuantidade: 2,
        vendaData: '2026-05-07', compraData: null, compraValor: null })], rendimentos: [rendimento({ transacaoId: 3 })]
    }));
    t.botao('Corrigir selecionados').click();
    t.fixture.detectChanges();
    expect(t.correcoes[0]).toMatchObject({ trocos: [1, 2], rendimentos: [3], compras: [], ativosParaFii: [] });
  });

  it('trocar o débito recalcula o preço sem o troco', async () => {
    const t = await abrirConferencia();
    const linha = t.tela.cotas()[0];
    linha.debitoId = 1176;
    t.tela.recalcular(linha);
    expect(linha.preco).toBeCloseTo((558.96 - 0.78) / 72, 4);
  });

  it('desmarcado não vai; sem nada marcado, o botão fica desabilitado', async () => {
    const t = await abrirConferencia();
    for (const l of [...t.tela.tipos(), ...t.tela.cotas(), ...t.tela.trocos(), ...t.tela.rendimentos()]) l.marcada = false;
    t.fixture.detectChanges();
    expect(t.botao('Corrigir selecionados').disabled).toBe(true);
  });

  it('compra marcada sem débito não é enviada: pede o débito', async () => {
    const t = await abrirConferencia();
    t.tela.cotas()[0].debitoId = null;
    const erro = jest.spyOn(TestBed.inject(AvisosService), 'erro');
    t.tela.corrigir();
    expect(t.correcoes).toHaveLength(0);
    expect(erro).toHaveBeenCalledWith('Escolha o débito e informe cotas e preço de GARE11.');
  });

  it('sem categoria de transferência, o troco fica desmarcado e travado', async () => {
    const t = await abrirConferencia(conferencia({ categoriaDoTroco: null }));
    expect(t.tela.trocos()[0].marcada).toBe(false);
    expect(t.raiz.querySelector<HTMLInputElement>('#tabela-trocos input[type=checkbox]')!.disabled).toBe(true);
    expect(t.raiz.querySelector('[role=alert]')!.textContent).toContain('Crie uma categoria do tipo transferência');
  });

  it('o aviso tem "Desfazer", que manda a chave da correção e recarrega a tela', async () => {
    const t = await abrirConferencia();
    t.botao('Corrigir selecionados').click();
    t.fixture.detectChanges();
    expect(t.eventos).toMatchObject({ corrigido: 1, fechar: 1 });
    const aviso = TestBed.inject(AvisosService).avisos().find((a) => a.acao)!;
    expect(aviso.texto).toBe('Corrigido: 1 ativo(s) agora FII, 1 compra(s) lançada(s), 1 troco(s)/venda(s) em Investimentos, '
      + '1 rendimento(s) de volta à receita.');
    aviso.acao!.executar();
    expect(t.desfeitas).toEqual(['chave-1']);
    expect(t.eventos.corrigido).toBe(2);
  });

  it('sem débito que explique a diferença, oferece lançar à mão com as cotas que faltam', async () => {
    const t = await abrirConferencia(conferencia({ comprasSemOperacao: [], cotas: [{ ...conferencia().cotas[0],
      debitoSugeridoId: null, precoSugerido: null, fechamentoNoDia: null }] }));
    t.botao('Lançar a compra à mão').click();
    expect(t.eventos.lancarAMao).toEqual([{ ticker: 'GARE11', quantidade: 72 }]);
  });
});
