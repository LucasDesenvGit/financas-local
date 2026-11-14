import { SituacaoHolerite, TipoLancamentoHolerite as T } from '../../../src/app/core/modelos';
import {
  ehPorHora, ehProvento, lancamentoNovo, paraSalvar, rotuloTipo, textoSituacao, trocarTipo
} from '../../../src/app/paginas/trabalho/holerites';
import { ajudaDaUrl } from '../../../src/app/paginas/manual/manual-conteudo';

/** #211 (ADR 0065): as regras da tela Trabalho › Holerites (o cálculo é da API, testado no Go). */
describe('Trabalho › Holerites (#211)', () => {
  it('cada tipo nasce com os padrões dele', () => {
    expect(lancamentoNovo(T.HoraExtra)).toMatchObject({ percentual: 50, incideInss: false, recorrente: false });
    expect(lancamentoNovo(T.AdicionalNoturno).percentual).toBe(20);
    expect(lancamentoNovo(T.OutroProvento)).toMatchObject({ percentual: null, incideInss: true, incideIrrf: true });
    expect(lancamentoNovo(T.PensaoAlimenticia)).toMatchObject({ deduzIrrf: true, recorrente: true });
    expect(lancamentoNovo(T.PlanoDeSaude)).toMatchObject({ deduzIrrf: false, recorrente: true });
    expect(lancamentoNovo(T.OutroDesconto)).toMatchObject({ deduzIrrf: false, recorrente: false });
  });

  it('trocar o tipo guarda a descrição e o "repetir" e recomeça o resto', () => {
    const he = { ...lancamentoNovo(T.HoraExtra), descricao: 'HE', quantidade: 10, recorrente: true };
    expect(trocarTipo(he, T.OutroProvento)).toMatchObject({ tipo: T.OutroProvento, descricao: 'HE', quantidade: null,
      percentual: null, recorrente: true, incideInss: true });
  });

  it('o corpo do PUT leva só os campos do tipo', () => {
    const lista = [
      { ...lancamentoNovo(T.HoraExtra), id: 7, quantidade: 10, valor: 99, descricao: '  HE  ' },
      { ...lancamentoNovo(T.ValeTransporte), quantidade: 3, percentual: 6, valor: 400 }
    ];
    expect(paraSalvar(lista)).toEqual([
      { tipo: T.HoraExtra, descricao: 'HE', quantidade: 10, percentual: 50, valor: null, incideInss: false, incideIrrf: false,
        deduzIrrf: false, recorrente: false },
      { tipo: T.ValeTransporte, descricao: '', quantidade: null, percentual: null, valor: 400, incideInss: false,
        incideIrrf: false, deduzIrrf: false, recorrente: false }
    ]);
  });

  it('textos e grupos de tipo', () => {
    expect(rotuloTipo(T.ValeAlimentacao)).toBe('VR/VA');
    expect(rotuloTipo(99 as T)).toBe('—');
    expect(ehPorHora(T.AdicionalNoturno)).toBe(true);
    expect(ehPorHora(T.OutroProvento)).toBe(false);
    expect(ehProvento(T.OutroProvento)).toBe(true);
    expect(ehProvento(T.ValeTransporte)).toBe(false);
    expect(textoSituacao(SituacaoHolerite.Fechado)).toBe('Fechado');
    expect(textoSituacao(SituacaoHolerite.Aberto)).toBe('Aberto');
  });

  it('a Ajuda da barra leva à seção da tela do módulo (dois trechos na rota)', () => {
    expect(ajudaDaUrl('/trabalho/holerites?holerite=3')).toEqual({ capitulo: 'trabalho', secao: 'holerites' });
    expect(ajudaDaUrl('/trabalho/salario')).toEqual({ capitulo: 'trabalho', secao: 'salario' });
    expect(ajudaDaUrl('/transacoes?categoria=1')).toEqual({ capitulo: 'transacoes' });
    expect(ajudaDaUrl('/manual/contas')).toBeNull();
  });
});
