import { TestBed } from '@angular/core/testing';
import { AvisosService } from '../../src/app/compartilhado/avisos.component';
import { PaginaTransacoes, TipoCorrespondencia, TipoTransacao } from '../../src/app/core/modelos';
import { abrirRenomear, pix } from '../apoio/renomear';
import { abrirTransacoes, paginaVazia } from '../apoio/transacoes';
import { transacao } from '../apoio/transacoes-edicao';

/**
 * Regressão de "Renomear" (#123, ADR 0030): o que vai para a API em cada escolha, o "Desfazer" e o nome do banco
 * embaixo na lista.
 */
describe('Regressão: renomear transação', () => {
  it('com outras iguais, "também nas outras" já vem marcado e manda a regra "igual à descrição do banco"', async () => {
    const t = await abrirRenomear();
    expect(t.previas[0]).toEqual({ texto: 'PIX ENVIADO 12345 JOAO', tipo: TipoCorrespondencia.Igual });
    expect(t.tela.escopo).toBe('outras');
    t.tela.nome = 'Aluguel do apartamento';
    t.tela.salvar();
    expect(t.pedidos).toEqual([{ descricao: 'Aluguel do apartamento', voltarOriginal: false, aplicarNasOutras: true,
      tipoCorrespondencia: TipoCorrespondencia.Igual, texto: 'PIX ENVIADO 12345 JOAO' }]);
    expect(t.eventos.renomeada).toBe(1);
  });

  it('sem outras iguais, fica "só nesta" e não manda texto de regra', async () => {
    const t = await abrirRenomear(pix(), 0);
    expect(t.tela.escopo).toBe('so-esta');
    t.tela.nome = '  Presente da Ana  ';
    t.tela.salvar();
    expect(t.pedidos[0]).toMatchObject({ descricao: 'Presente da Ana', aplicarNasOutras: false, texto: null });
  });

  it('"contiver" sugere o texto sem os números e pede outra prévia', async () => {
    const t = await abrirRenomear();
    t.tela.tipo = TipoCorrespondencia.Contem;
    t.tela.mudarTipo();
    // A prévia espera 300 ms de calma antes de pedir (debounce).
    await new Promise((fim) => setTimeout(fim, 350));
    await t.assentar();
    expect(t.tela.texto).toBe('PIX ENVIADO JOAO');
    expect(t.previas.at(-1)).toEqual({ texto: 'PIX ENVIADO JOAO', tipo: TipoCorrespondencia.Contem });
  });

  it('"Voltar à descrição do banco" só aparece quando o nome foi trocado e manda voltarOriginal', async () => {
    const semTroca = await abrirRenomear();
    expect(semTroca.botao('Voltar à descrição do banco')).toBeUndefined();
    TestBed.resetTestingModule();
    const t = await abrirRenomear(pix({ descricao: 'Aluguel' }));
    t.botao('Voltar à descrição do banco').click();
    expect(t.pedidos[0]).toMatchObject({ voltarOriginal: true, aplicarNasOutras: false, descricao: null });
  });

  it('o aviso tem "Desfazer", que manda a chave e recarrega a lista', async () => {
    const t = await abrirRenomear();
    t.tela.nome = 'Aluguel';
    t.tela.salvar();
    const aviso = TestBed.inject(AvisosService).avisos().find((a) => a.acao)!;
    expect(aviso.texto).toBe('Renomeada, junto com mais 3 transação(ões). As próximas já chegam com esse nome.');
    aviso.acao!.executar();
    expect(t.desfeitos).toEqual(['chave-9']);
    expect(t.eventos.renomeada).toBe(2);
  });

  it('lançamento manual (sem descrição do banco) não oferece regra', async () => {
    const t = await abrirRenomear(pix({ descricaoOriginal: null, openFinance: false }));
    expect(t.previas).toHaveLength(0);
    expect(t.raiz.querySelector('fieldset')).toBeNull();
    t.tela.nome = 'Feira';
    t.tela.salvar();
    expect(t.pedidos[0]).toMatchObject({ aplicarNasOutras: false });
  });

  it('na lista, o nome do banco aparece embaixo só quando o nome foi trocado', async () => {
    const itens = [transacao({ id: 1, descricao: 'Aluguel', descricaoOriginal: 'PIX ENVIADO 123' }), transacao({ id: 2 })];
    const t = await abrirTransacoes('/transacoes', (f): PaginaTransacoes => ({ ...paginaVazia(f, 2), itens }));
    const linhas = Array.from(t.elemento.querySelectorAll('tbody tr'));
    expect(linhas[0].querySelector('.descricao-original')!.textContent!.trim()).toBe('do banco: PIX ENVIADO 123');
    expect(linhas[1].querySelector('.descricao-original')).toBeNull();
  });

  it('pagamento de fatura (fora dos gastos) aparece com o valor em cor neutra, nem verde nem vermelho', async () => {
    const itens = [transacao({ id: 1, valor: 1626.42, tipo: TipoTransacao.Transferencia, ignorarRelatorios: true })];
    const t = await abrirTransacoes('/transacoes', (f): PaginaTransacoes => ({ ...paginaVazia(f, 1), itens }));
    const valor = t.elemento.querySelector('tbody td.valor')!;
    expect(valor.classList.contains('fora-dos-totais')).toBe(true);
    expect(valor.classList.contains('positivo')).toBe(false);
  });
});
