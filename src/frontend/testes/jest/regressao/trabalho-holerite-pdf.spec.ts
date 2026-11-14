import { PreviaHoleritePdf, RubricaLida, TipoLinhaHolerite as T } from '../../../src/app/core/modelos';
import {
  LinhaPrevia, TAMANHO_MAXIMO, TIPOS_LINHA_DESCONTO, TIPOS_LINHA_PROVENTO, faltaParaConfirmar, linhasDaPrevia, mesDoPdf,
  motivoDaRecusa, paraBase64, paraConfirmar, precisaEscolher, problemaDoArquivo, provasAtuais, situacaoDaLinha, textoDaProva,
  tipoEhProvento
} from '../../../src/app/paginas/trabalho/holerite-pdf';

/** #228 (ADR 0068): as regras do holerite em PDF na tela (a leitura é da API, testada no Go). Dados fictícios. */
const rubrica = (descricao: string, valor: number, provento: boolean, tipo: T, como: RubricaLida['como'] = 'dicionario'): RubricaLida =>
  ({ codigo: '001', descricao, referencia: '', valor, provento, trecho: `001 ${descricao}`, tipo, como });

const previa = (rubricas: RubricaLida[], totais: Partial<PreviaHoleritePdf> = {}): PreviaHoleritePdf => ({
  empregador: 'EMPRESA FICTÍCIA LTDA', cnpj: '12345678000195', mes: '2026-10-01', rubricas, totalVencimentos: 5000.1,
  totalDescontos: 600.3, liquido: 4399.8, salarioBase: 5000, baseInss: 5000.1, baseFgts: null, fgtsDoMes: 400, baseIrrf: null,
  provas: [], comparacoes: [], avisos: [], existente: null, ...totais
});

describe('Trabalho › Holerite em PDF (#228)', () => {
  const p = previa([rubrica('SALARIO', 5000.1, true, T.Salario), rubrica('INSS', 500.1, false, T.Inss),
    rubrica('Seguro de vida', 100.1, false, T.OutroDesconto, 'desconhecida'), rubrica('ASSIST', 0.1, false, T.PlanoDeSaude, 'escolha')]);

  it('a prova real, refeita com o que a pessoa corrigiu, em centavos', () => {
    const linhas = linhasDaPrevia(p);
    expect(provasAtuais(p, linhas).map((x) => [x.descricao, x.bateu, x.diferenca])).toEqual([
      ['Total de vencimentos', true, 0], ['Total de descontos', true, 0], ['Líquido', true, 0]]);
    // Deixar uma linha de fora e trocar outra de lado: a prova mostra qual total e por quanto.
    const mudadas: LinhaPrevia[] = linhas.map((l, i) => (i === 2 ? { ...l, ignorar: true } : i === 3 ? { ...l, tipo: T.OutroProvento } : l));
    expect(provasAtuais(p, mudadas).map((x) => [x.descricao, x.encontrado, x.diferenca, x.bateu])).toEqual([
      ['Total de vencimentos', 5000.2, 0.1, false], ['Total de descontos', 500.1, -100.2, false], ['Líquido', 4500.1, 100.3, false]]);
    // Sem o valor, a linha não soma; sem o total no PDF, não há prova dele.
    const semValor = linhas.map((l, i) => (i === 0 ? { ...l, valor: null } : l));
    expect(provasAtuais(previa(p.rubricas, { totalVencimentos: null, liquido: null }), semValor)).toEqual([
      { descricao: 'Total de descontos', impresso: 600.3, encontrado: 600.3, diferenca: 0, bateu: true }]);
  });

  it('o texto da prova diz se bate, ou de que lado ficou a diferença', () => {
    const base = { descricao: 'Líquido', impresso: 10, encontrado: 10, diferenca: 0, bateu: true };
    expect(textoDaProva(base)).toBe('bate com o PDF');
    expect(textoDaProva({ ...base, diferenca: 0.5, bateu: false })).toBe('a mais que o impresso');
    expect(textoDaProva({ ...base, diferenca: -0.5, bateu: false })).toBe('a menos que o impresso');
  });

  it('a situação de cada linha, em texto', () => {
    const [salario, , seguro, assist] = linhasDaPrevia(p);
    expect(situacaoDaLinha(salario)).toBe('reconhecido');
    expect(situacaoDaLinha(assist)).toBe('reconhecido pelo que você escolheu antes');
    expect(situacaoDaLinha(seguro)).toBe('escolha o tipo');
    expect(situacaoDaLinha({ ...seguro, tipo: T.PlanoDeSaude })).toBe('você escolheu: o site lembra no próximo mês');
    expect(situacaoDaLinha({ ...salario, ignorar: true })).toBe('fica de fora');
    expect(precisaEscolher(seguro)).toBe(true);
    expect(precisaEscolher({ ...seguro, tipo: T.PlanoDeSaude })).toBe(false);
    expect(precisaEscolher({ ...seguro, ignorar: true })).toBe(false);
    expect(precisaEscolher(salario)).toBe(false);
  });

  it('confirmar pede o mês, uma linha e o valor de cada uma', () => {
    const linhas = linhasDaPrevia(p);
    expect(faltaParaConfirmar(linhas, null)).toBe('Escolha o mês do holerite.');
    expect(faltaParaConfirmar(linhas.map((l) => ({ ...l, ignorar: true })), '2026-10-01')).toBe('Deixe pelo menos uma linha no holerite.');
    expect(faltaParaConfirmar(linhas.map((l, i) => (i === 1 ? { ...l, valor: null } : l)), '2026-10-01')).toBe('Cada linha precisa de um valor.');
    expect(faltaParaConfirmar(linhas.map((l, i) => (i === 1 ? { ...l, valor: 0 } : l)), '2026-10-01')).toBe('Cada linha precisa de um valor.');
    // A linha de fora sem valor não atrapalha.
    expect(faltaParaConfirmar(linhas.map((l, i) => (i === 1 ? { ...l, valor: null, ignorar: true } : l)), '2026-10-01')).toBeNull();
  });

  it('o corpo do "Confirmar": só as linhas que ficam, como a pessoa deixou, e o rodapé do PDF', () => {
    const linhas = linhasDaPrevia(p).map((l, i) => (i === 2 ? { ...l, tipo: '12' as unknown as T, valor: 99.9 } : i === 3 ? { ...l, ignorar: true } : l));
    expect(paraConfirmar(p, linhas, '2026-09-01')).toEqual({
      mes: '2026-09-01', salarioBase: 5000, baseInss: 5000.1, baseFgts: null, fgtsDoMes: 400,
      rubricas: [
        { codigo: '001', descricao: 'SALARIO', referencia: '', valor: 5000.1, provento: true, tipo: T.Salario },
        { codigo: '001', descricao: 'INSS', referencia: '', valor: 500.1, provento: false, tipo: T.Inss },
        { codigo: '001', descricao: 'Seguro de vida', referencia: '', valor: 99.9, provento: false, tipo: T.PlanoDeSaude }]
    });
  });

  it('os tipos da linha, dos dois lados', () => {
    expect(TIPOS_LINHA_PROVENTO.every((t) => tipoEhProvento(t.valor))).toBe(true);
    expect(TIPOS_LINHA_DESCONTO.some((t) => tipoEhProvento(t.valor))).toBe(false);
    expect(tipoEhProvento(T.Dsr)).toBe(true);
    expect(tipoEhProvento(T.ValeTransporte)).toBe(false);
    expect(TIPOS_LINHA_DESCONTO.map((t) => t.valor)).toContain(T.Adiantamento);
  });

  it('o arquivo: base64, o limite e a recusa da API', () => {
    expect(paraBase64(new TextEncoder().encode('%PDF-1.4'))).toBe('JVBERi0xLjQ=');
    const grande = new Uint8Array(0x8000 * 2 + 3).fill(65);
    expect(atob(paraBase64(grande))).toBe('A'.repeat(0x8000 * 2 + 3));
    expect(problemaDoArquivo('a.pdf', TAMANHO_MAXIMO)).toBeNull();
    expect(problemaDoArquivo('a.pdf', TAMANHO_MAXIMO + 1)).toBe('a.pdf: o PDF passa do limite de 700 KB. Mande só o holerite.');
    expect(problemaDoArquivo('a.pdf', 0)).toBe('a.pdf: o arquivo está vazio.');
    expect(motivoDaRecusa({ error: { mensagem: 'x', motivo: 'senha' } })).toBe('senha');
    expect(motivoDaRecusa({ error: { motivo: 'semTexto' } })).toBe('semTexto');
    expect(motivoDaRecusa({ error: { motivo: 'outro' } })).toBeNull();
    expect(motivoDaRecusa({ error: null })).toBeNull();
    expect(motivoDaRecusa(null)).toBeNull();
    expect(mesDoPdf(p)).toBe('2026-10-01');
    expect(mesDoPdf({ ...p, mes: null })).toBeNull();
  });
});
