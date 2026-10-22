import { TestBed } from '@angular/core/testing';
import { bancoNoTexto, visualBanco } from '../../src/app/compartilhado/bancos';
import { IconeCobrancaPadrao, IconesCategoria } from '../../src/app/compartilhado/icones';
import { SeloServicoComponent } from '../../src/app/compartilhado/selo-servico.component';
import { SERVICOS, servicoDe } from '../../src/app/compartilhado/servicos';
import { TIPOS_LANCAMENTO } from '../../src/app/compartilhado/tipos-lancamento';
import { TipoLancamento } from '../../src/app/core/modelos';

/**
 * #325: o selo da transação usa também o tipo do lançamento (as etiquetas), além do nome. "Transferência Recebida|Fulano"
 * (Pix recebido) e "Pagamento de fatura … Itau Uniclass" ficavam sem selo; o KeetaBR não era reconhecido.
 */
function selo(entradas: Record<string, unknown>) {
  const fixture = TestBed.createComponent(SeloServicoComponent);
  for (const [nome, valor] of Object.entries(entradas)) fixture.componentRef.setInput(nome, valor);
  fixture.detectChanges();
  return fixture.componentInstance;
}
const PIX = SERVICOS.find((s) => s.chave === 'pix')!.visual;

describe('Regressão: selo pelo tipo do lançamento e Keeta (#325)', () => {
  it('reconhece o KeetaBR', () => {
    expect(servicoDe('KeetaBR')?.chave).toBe('keeta');
    expect(servicoDe('KEETA*PEDIDO 123')?.chave).toBe('keeta');
  });

  it('Pix enviado ou recebido sem serviço pelo nome: o selo do Pix, antes da categoria', () => {
    expect(selo({ nome: 'Transferência Recebida|Pessoa Exemplo', tipo: TipoLancamento.PixRecebido }).visual()).toBe(PIX);
    expect(selo({ nome: 'Pessoa Exemplo', tipo: TipoLancamento.PixEnviado, categoria: 'Transferências' }).visual()).toBe(PIX);
    // Serviço pelo nome vem antes: Pix para a Netflix é Netflix.
    expect(selo({ nome: 'Netflix', tipo: TipoLancamento.PixEnviado }).visual()!.nome).toBe('Netflix');
  });

  it('pagamento de fatura: o logo do banco citado no texto; sem banco, o ícone da etiqueta', () => {
    expect(selo({ nome: 'Pagamento de fatura Fatura Paga Itau Uniclass', tipo: TipoLancamento.PagamentoFatura }).visual())
      .toBe(visualBanco('itau'));
    const semBanco = selo({ nome: 'Pagamento de fatura', tipo: TipoLancamento.PagamentoFatura });
    expect(semBanco.visual()).toBeNull();
    expect(semBanco.iconeCategoria()).toBe(TIPOS_LANCAMENTO[TipoLancamento.PagamentoFatura].icone);
  });

  it('sem marca: a categoria vem antes do ícone do tipo; sem os dois, o recibo', () => {
    expect(selo({ nome: 'Restaurante', tipo: TipoLancamento.CompraCartao, categoria: 'Alimentação' }).iconeCategoria())
      .toBe(IconesCategoria['ALIMENTACAO']);
    expect(selo({ nome: 'Loja', tipo: TipoLancamento.CompraCartao }).iconeCategoria()).toBe(TIPOS_LANCAMENTO[TipoLancamento.CompraCartao].icone);
    expect(selo({ nome: 'Algo', tipo: TipoLancamento.Outro }).iconeCategoria()).toBe(IconeCobrancaPadrao);
    expect(selo({ nome: 'Algo' }).iconeCategoria()).toBe(IconeCobrancaPadrao);
  });

  it('o banco no texto só casa com a palavra inteira ou a sequência certa', () => {
    expect(bancoNoTexto('FATURA PAGA ITAU UNICLASS')).toBe('itau');
    expect(bancoNoTexto('Pagamento fatura Nubank')).toBe('nubank');
    expect(bancoNoTexto('PAGTO CARTAO BANCO INTER')).toBe('inter');
    expect(bancoNoTexto('INTERNET FIBRA')).toBeNull();
    expect(bancoNoTexto('FATURA OUROCARD')).toBe('bb');
    expect(bancoNoTexto(null, 'PAGAMENTO C6 BANK')).toBe('c6');
    expect(bancoNoTexto('Pagamento de fatura')).toBeNull();
  });
});
