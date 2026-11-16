import {
  Carteira, EvolucaoCarteira, OperacaoAtivo, PosicaoAtivo, Reserva, SugestaoExtrato, TipoAtivo, TipoOperacao
} from '../../../src/app/core/modelos';
import {
  LinhaImportacao, abaDaUrl, abreConferirNaUrl, alturaGraficoAtivos, avisoDeLinhasIncompletas, configAtivos, configDistribuicao,
  configEvolucao, contarReservasAConferir, corDoTipo, dadosDaCompraQueFaltou, formularioConferencia, formularioDaOperacao,
  formularioDoAtivo, formularioNovaOperacao, itensDaImportacao, linhasDaImportacao, linhasParaImportar, listaTickers,
  mensagemCotacoes, mensagemImportacaoOpenFinance, nomeTipoPluggy, operacaoParaEnviar, percentual, posicoesAbertas,
  posicoesParaMostrar, ptaxDaOperacao, quantidade, quantidadeDeFiis, resumoAtivos, resumoDistribuicao, resumoEvolucao,
  saldoInformadoEmReais, tiposPara, totalOperacao, totalSelecionadas, ativoParaEnviar
} from '../../../src/app/paginas/patrimonio/investimentos';

/**
 * #731 (ADR 0059): a regra da tela de Investimentos, em funções puras e sem `TestBed`: aba na URL, posições e totais,
 * frases-resumo, formulários, importação pelo extrato e os avisos das chamadas. A tela só chama estas funções.
 */
const posicao = (campos: Partial<PosicaoAtivo> = {}): PosicaoAtivo => ({
  id: 1, ticker: 'GARE11', nome: null, tipo: TipoAtivo.Fii, moeda: 'BRL', quantidade: 10, precoMedio: 9, custo: 90, precoAtual: 10,
  precoAtualEmReais: 10, precoAnterior: 10, variacaoDia: 0, valorMercado: 100, valorizacao: 10, valorizacaoPercentual: 11,
  lucroRealizado: 0, proventosRecebidos: 0, proventos12Meses: 0, dividendYield12Meses: null, resultadoTotal: 10,
  resultadoTotalPercentual: 11, valorSeFosseSelic: 95, diferencaSelic: 5, resultadoSelicPercentual: 5, primeiraCompra: null,
  ultimoProvento: null, precoManual: false, precoAtualizadoEm: null, fonteCotacao: null, semCotacao: false, segmento: null, ...campos
});

const reserva = (campos: Partial<Reserva> = {}): Reserva => ({
  contaId: 1, nomeConta: 'Banco', nome: 'Viagem', saldo: 300, guardado: 500, retirado: 200, movimentos: 4, primeiroMovimento: null,
  ultimoMovimento: null, saldoConferido: null, dataConferencia: null, podeTerSaldoAnterior: false, conferenciaAntiga: false, ...campos
});

const sugestao = (campos: Partial<SugestaoExtrato> = {}): SugestaoExtrato => ({
  transacaoId: 1, data: '2026-11-03', descricao: 'COMPRA MXRF11', valor: 98.5, contaNome: 'Corretora', ticker: 'MXRF11',
  tipo: TipoOperacao.Compra, quantidade: 10, precoUnitario: 9.85, fechamentoNoDia: null, origem: null, jaImportada: false,
  possivelDuplicata: false, ...campos
});

describe('Regressão: Investimentos, a aba e o "conferir" lidos da URL (#89, #170)', () => {
  const url = (params: Record<string, string>) => ({ get: (nome: string) => params[nome] ?? null });

  it('abre na aba pedida; sem aba ou com aba que não existe, no Resumo', () => {
    expect(abaDaUrl(url({ aba: 'reservas' }))).toBe('reservas');
    expect(abaDaUrl(url({ aba: 'fii' }))).toBe('fii');
    expect(abaDaUrl(url({ aba: 'outra' }))).toBe('resumo');
    expect(abaDaUrl(url({}))).toBe('resumo');
  });

  it('só "?conferir=1" abre "Conferir investimentos"', () => {
    expect(abreConferirNaUrl(url({ conferir: '1' }))).toBe(true);
    expect(abreConferirNaUrl(url({ conferir: '0' }))).toBe(false);
    expect(abreConferirNaUrl(url({}))).toBe(false);
  });
});

describe('Regressão: Investimentos, textos e números', () => {
  it('percentual leva sinal só no positivo e traço sem valor', () => {
    expect(percentual(1.5)).toBe('+1,50%');
    expect(percentual(-1.5)).toBe('-1,50%');
    expect(percentual(0)).toBe('0,00%');
    expect(percentual(1.5, false)).toBe('1,50%');
    expect(percentual(null)).toBe('—');
    expect(percentual(undefined)).toBe('—');
  });

  it('a quantidade de cotas some com "Ocultar valores"', () => {
    expect(quantidade(1234.5, false)).toBe('1.234,5');
    expect(quantidade(1234.5, true)).toBe('•••');
  });

  it('listaTickers junta com vírgula e "e"', () => {
    expect(listaTickers([])).toBe('');
    expect(listaTickers(['GARE11'])).toBe('GARE11');
    expect(listaTickers(['GARE11', 'XPLG11'])).toBe('GARE11 e XPLG11');
    expect(listaTickers(['GARE11', 'XPLG11', 'XPML11'])).toBe('GARE11, XPLG11 e XPML11');
  });

  it('nomeTipoPluggy reconhece o FII pelo subtipo ou pelo código, e cai no tipo da corretora', () => {
    expect(nomeTipoPluggy('EQUITY', 'REAL_ESTATE_FUND')).toBe('FII');
    expect(nomeTipoPluggy('EQUITY', null, 'XPLG11')).toBe('FII');
    expect(nomeTipoPluggy('EQUITY', null, 'PETR4')).toBe('Ações');
    expect(nomeTipoPluggy('FIXED_INCOME')).toBe('Renda fixa');
    expect(nomeTipoPluggy('DESCONHECIDO')).toBe('DESCONHECIDO');
  });

  it('ativo no exterior não pode ser FII nem BDR', () => {
    expect(tiposPara('BRL')).toContain(TipoAtivo.Fii);
    expect(tiposPara(undefined)).toContain(TipoAtivo.Bdr);
    expect(tiposPara('USD')).toEqual([TipoAtivo.Acao, TipoAtivo.Etf, TipoAtivo.Outro]);
  });

  it('a cor do tipo repete a paleta depois da última', () => {
    expect(corDoTipo(0)).toBe('#a7a3f5');
    expect(corDoTipo(8)).toBe(corDoTipo(0));
  });
});

describe('Regressão: Investimentos, posições e totais', () => {
  const posicoes = [posicao({ id: 1 }), posicao({ id: 2, ticker: 'PETR4', tipo: TipoAtivo.Acao }), posicao({ id: 3, quantidade: 0 })];

  it('abertas são as com cotas; encerradas só entram quando pedido', () => {
    expect(posicoesAbertas(posicoes).map((p) => p.id)).toEqual([1, 2]);
    expect(posicoesParaMostrar(posicoes, false).map((p) => p.id)).toEqual([1, 2]);
    expect(posicoesParaMostrar(posicoes, true).map((p) => p.id)).toEqual([1, 2, 3]);
    expect(posicoesAbertas(undefined)).toEqual([]);
  });

  it('conta os FIIs abertos e as reservas que pedem conferência', () => {
    expect(quantidadeDeFiis(posicoes)).toBe(1);
    expect(contarReservasAConferir([reserva(), reserva({ podeTerSaldoAnterior: true }), reserva({ conferenciaAntiga: true })])).toBe(2);
    expect(contarReservasAConferir(undefined)).toBe(0);
  });

  it('o gráfico de ativos cresce 36 px por barra, com o mínimo de 220', () => {
    expect(alturaGraficoAtivos(2)).toBe(220);
    expect(alturaGraficoAtivos(10)).toBe(360);
  });
});

describe('Regressão: Investimentos, frases-resumo (UX-9)', () => {
  const ev = (valorMercado: number | null, aplicadoNaSelic: number): EvolucaoCarteira =>
    ({ ano: 2026, mes: 10, aplicado: 1000, aplicadoNaSelic, valorMercado });

  it('evolução: compara o último mês com valor de mercado; "praticamente o mesmo" abaixo de 0,5%', () => {
    expect(resumoEvolucao([], false)).toBeNull();
    expect(resumoEvolucao([ev(null, 1000)], false)).toBeNull();
    expect(resumoEvolucao([ev(1002, 1000)], false)).toBe('Sua renda variável vale hoje praticamente o mesmo que esse dinheiro teria na Selic.');
    const mais = resumoEvolucao([ev(1100, 1000)], false)!;
    expect(mais).toContain('10% mais do que esse dinheiro teria na Selic');
    expect(mais).toMatch(/\(R\$\s1\.100 contra R\$\s1\.000\)/);
    expect(resumoEvolucao([ev(900, 1000)], true)).toBe('Sua renda variável vale hoje 10% menos do que esse dinheiro teria na Selic.');
  });

  it('distribuição: o maior tipo e a parte dele no total', () => {
    const d = { totalGeral: 200, distribuicao: [{ nome: 'Reservas', valor: 50 }, { nome: 'Renda fixa', valor: 150 }] } as Carteira;
    expect(resumoDistribuicao(d)).toBe('Renda fixa concentra 75% dos seus investimentos.');
    expect(resumoDistribuicao({ ...d, totalGeral: 0 })).toBeNull();
    expect(resumoDistribuicao(null)).toBeNull();
  });

  it('ativos: quantos renderam mais que a Selic', () => {
    const a = (r: number, s: number) => posicao({ resultadoTotalPercentual: r, resultadoSelicPercentual: s });
    expect(resumoAtivos([])).toBeNull();
    expect(resumoAtivos([a(8, 5)])).toBe('Seu ativo rendeu mais que a Selic no mesmo período.');
    expect(resumoAtivos([a(8, 5), a(9, 5)])).toBe('Todos os 2 ativos renderam mais que a Selic no mesmo período.');
    expect(resumoAtivos([a(1, 5), a(2, 5)])).toBe('Nenhum ativo rendeu mais que a Selic no mesmo período.');
    expect(resumoAtivos([a(8, 5), a(1, 5), a(9, 5)])).toBe('2 de 3 ativos renderam mais que a Selic no mesmo período.');
  });
});

describe('Regressão: Investimentos, a configuração dos gráficos', () => {
  it('evolução: três séries, com o valor de mercado só como ponto', () => {
    const c = configEvolucao([{ ano: 2026, mes: 1, aplicado: 10, aplicadoNaSelic: 11, valorMercado: null },
      { ano: 2026, mes: 2, aplicado: 20, aplicadoNaSelic: 22, valorMercado: 25 }]);
    expect(c.data.labels).toEqual(['jan/26', 'fev/26']);
    expect(c.data.datasets.map((d) => d.data)).toEqual([[10, 20], [11, 22], [null, 25]]);
    expect(c.data.datasets[2]).toMatchObject({ showLine: false });
  });

  it('distribuição: uma fatia por tipo, com as cores da paleta', () => {
    const c = configDistribuicao([{ nome: 'A', valor: 1 }, { nome: 'B', valor: 2 }]);
    expect(c.data.labels).toEqual(['A', 'B']);
    expect(c.data.datasets[0].backgroundColor).toEqual([corDoTipo(0), corDoTipo(1)]);
  });

  it('ativos: o resultado é verde quando ganhou da Selic e rosa quando perdeu', () => {
    const c = configAtivos([posicao({ resultadoTotalPercentual: 8, resultadoSelicPercentual: 5 }),
      posicao({ ticker: 'PETR4', resultadoTotalPercentual: 1, resultadoSelicPercentual: 5 })]);
    expect(c.data.labels).toEqual(['GARE11', 'PETR4']);
    expect(c.data.datasets[0].backgroundColor).toEqual(['#8fd5b4', '#f5a3b0']);
    expect(c.data.datasets[1].data).toEqual([5, 5]);
  });
});

describe('Regressão: Investimentos, avisos das chamadas', () => {
  it('cotações: erro com os ativos sem cotação; sucesso só quando não é a atualização automática', () => {
    expect(mensagemCotacoes({ atualizados: 0, proventosNovos: 0, falhas: ['AAPL', 'PETR4'] }, true))
      .toEqual({ tipo: 'erro', texto: 'Sem cotação para AAPL, PETR4. Tente de novo mais tarde ou informe a cotação manualmente.' });
    expect(mensagemCotacoes({ atualizados: 3, proventosNovos: 0, falhas: [] }, true)).toBeNull();
    expect(mensagemCotacoes({ atualizados: 3, proventosNovos: 0, falhas: [] }, false))
      .toEqual({ tipo: 'sucesso', texto: '3 cotação(ões) atualizada(s)' });
    expect(mensagemCotacoes({ atualizados: 3, proventosNovos: 2, falhas: [] }, false)!.texto)
      .toBe('3 cotação(ões) atualizada(s) · 2 provento(s) novo(s)');
  });

  describe('compras do Open Finance', () => {
    const resultado = (campos = {}) => ({ ativosCriados: 0, operacoesImportadas: 0, semHistorico: [], comLancamentoManual: [], erros: [],
      mensagem: null, ...campos });

    it('a mensagem da API vira erro', () => {
      expect(mensagemImportacaoOpenFinance(resultado({ mensagem: 'Sem conexão' }), true)).toEqual({ tipo: 'erro', texto: 'Sem conexão' });
    });

    it('a importação automática sem nada novo fica calada; a manual avisa mesmo assim', () => {
      expect(mensagemImportacaoOpenFinance(resultado(), true)).toBeNull();
      expect(mensagemImportacaoOpenFinance(resultado(), false)).toEqual({ tipo: 'sucesso', texto: '0 compra(s)/venda(s) registrada(s)' });
      expect(mensagemImportacaoOpenFinance(resultado({ operacoesImportadas: 2, ativosCriados: 1 }), true))
        .toEqual({ tipo: 'sucesso', texto: '2 compra(s)/venda(s) registrada(s) · 1 ativo(s) novo(s)' });
    });

    it('falha em algum ativo vira erro, com as outras partes', () => {
      const m = mensagemImportacaoOpenFinance(resultado({ operacoesImportadas: 1, semHistorico: ['A'], comLancamentoManual: ['B'], erros: ['C'] }), true)!;
      expect(m.tipo).toBe('erro');
      expect(m.texto).toContain('sem histórico na corretora: A');
      expect(m.texto).toContain('ignorados por já terem lançamentos seus: B');
      expect(m.texto).toContain('falha em C');
    });
  });
});

describe('Regressão: Investimentos, o formulário da operação', () => {
  const operacao = (campos: Partial<OperacaoAtivo> = {}): OperacaoAtivo => ({
    id: 7, ativoId: 1, ticker: 'AAPL', tipo: TipoOperacao.Venda, data: '2026-10-01', quantidade: 3, precoUnitario: 180, taxas: 1,
    moeda: 'USD', cambio: 5.4, valorTotal: 0, observacao: 'x', precoAtual: null, variacaoPreco: null, valorAtual: null,
    valorSeFosseSelic: 0, origem: 1, ...campos
  });

  it('a operação nova começa em compra, hoje, em reais; a de um ativo já lançado herda a moeda', () => {
    expect(formularioNovaOperacao('', undefined, '2026-11-16')).toEqual({
      ticker: '', tipoAtivo: null, tipo: TipoOperacao.Compra, data: '2026-11-16', quantidade: null, precoUnitario: null, taxas: 0,
      observacao: null, moeda: 'BRL', cambio: null
    });
    expect(formularioNovaOperacao('AAPL', 'USD', '2026-11-16').moeda).toBe('USD');
  });

  it('editar traz os dados da operação; o câmbio de uma operação em reais não aparece', () => {
    expect(formularioDaOperacao(operacao())).toMatchObject({ ticker: 'AAPL', tipo: TipoOperacao.Venda, moeda: 'USD', cambio: 5.4 });
    expect(formularioDaOperacao(operacao({ moeda: 'BRL', cambio: 1 })).cambio).toBeNull();
  });

  it('o total soma as taxas na compra e desconta na venda', () => {
    const f = { ...formularioNovaOperacao('X', undefined, 'h'), quantidade: 10, precoUnitario: 5, taxas: 2 };
    expect(totalOperacao(f)).toBe(52);
    expect(totalOperacao({ ...f, tipo: TipoOperacao.Venda })).toBe(48);
    expect(totalOperacao({ ...f, quantidade: null, precoUnitario: null, taxas: 0 })).toBe(0);
  });

  it('para a API: ticker em maiúsculas sem espaços, observação vazia vira null e câmbio só fora do real', () => {
    const f = { ...formularioNovaOperacao(' mxrf11 ', undefined, 'h'), observacao: '   ', cambio: 5 };
    expect(operacaoParaEnviar(f)).toMatchObject({ ticker: 'MXRF11', observacao: null, cambio: null });
    expect(operacaoParaEnviar({ ...f, moeda: 'USD', observacao: ' ok ' })).toMatchObject({ observacao: 'ok', cambio: 5 });
  });

  it('a PTAX sugerida é a de venda na compra e a de compra na venda', () => {
    const c = { data: '2026-11-10', compra: 5.43, venda: 5.44 };
    expect(ptaxDaOperacao(c, TipoOperacao.Compra)).toEqual({ data: '2026-11-10', valor: 5.44, tipo: 'venda' });
    expect(ptaxDaOperacao(c, TipoOperacao.Venda)).toEqual({ data: '2026-11-10', valor: 5.43, tipo: 'compra' });
  });

  it('a compra que faltou leva as cotas e a observação', () => {
    expect(dadosDaCompraQueFaltou(4)).toEqual({ quantidade: 4, observacao: 'Compra que faltou no Open Finance' });
  });
});

describe('Regressão: Investimentos, o formulário do ativo', () => {
  it('copia os campos editáveis e manda o nome sem espaços (vazio vira null)', () => {
    const p = posicao({ nome: 'Fundo', segmento: 'Logística', precoManual: true, precoAtual: 10 });
    const f = formularioDoAtivo(p);
    expect(f).toEqual({ nome: 'Fundo', tipo: TipoAtivo.Fii, segmento: 'Logística', precoManual: true, precoAtual: 10 });
    expect(ativoParaEnviar({ ...f, nome: '  Novo  ' }).nome).toBe('Novo');
    expect(ativoParaEnviar({ ...f, nome: '  ' }).nome).toBeNull();
  });
});

describe('Regressão: Investimentos, a conferência do saldo de uma reserva', () => {
  it('o Méli Dólar começa em reais com a estimativa; as outras, com o saldo conferido ou a estimativa', () => {
    const meli = reserva({ moeda: 'USD', saldo: 209.6, saldoConferido: 10.4 });
    expect(formularioConferencia(meli, '2026-10-19')).toMatchObject({ saldo: 209.6, moeda: 'BRL', registrarDiferenca: false });
    expect(formularioConferencia(reserva({ saldoConferido: 280 }), 'h').saldo).toBe(280);
    expect(formularioConferencia(reserva(), 'h').saldo).toBe(300);
  });

  it('em dólar, o saldo é comparado em reais pela PTAX; sem PTAX, não dá para comparar', () => {
    const meli = reserva({ moeda: 'USD', cotacao: 5 });
    expect(saldoInformadoEmReais(meli, 'USD', 10)).toBe(50);
    expect(saldoInformadoEmReais(meli, 'BRL', 52)).toBe(52);
    expect(saldoInformadoEmReais({ ...meli, cotacao: null }, 'USD', 10)).toBeNull();
    expect(saldoInformadoEmReais(reserva(), 'BRL', 70)).toBe(70);
  });
});

describe('Regressão: Investimentos, a importação pelo extrato', () => {
  it('só vem marcada a linha nova, que não é duplicata e já tem cotas e preço', () => {
    const linhas = linhasDaImportacao([
      sugestao({ transacaoId: 1 }), sugestao({ transacaoId: 2, jaImportada: true }), sugestao({ transacaoId: 3, possivelDuplicata: true }),
      sugestao({ transacaoId: 4, quantidade: null }), sugestao({ transacaoId: 5, precoUnitario: null })
    ]);
    expect(linhas.map((l) => l.selecionada)).toEqual([true, false, false, false, false]);
    expect(linhas.every((l) => l.taxas === 0)).toBe(true);
    expect(totalSelecionadas(linhas)).toBe(1);
  });

  it('importa as marcadas que ainda não foram importadas', () => {
    const linhas: LinhaImportacao[] = [
      { ...sugestao({ transacaoId: 1 }), selecionada: true, taxas: 0 },
      { ...sugestao({ transacaoId: 2, jaImportada: true }), selecionada: true, taxas: 0 },
      { ...sugestao({ transacaoId: 3 }), selecionada: false, taxas: 0 }
    ];
    expect(linhasParaImportar(linhas).map((l) => l.transacaoId)).toEqual([1]);
  });

  it('avisa as marcadas sem código, cotas ou preço; completas seguem para a API em maiúsculas', () => {
    const marcada = (campos: Partial<SugestaoExtrato>): LinhaImportacao => ({ ...sugestao(campos), selecionada: true, taxas: 0 });
    expect(avisoDeLinhasIncompletas([marcada({}), marcada({ ticker: '', quantidade: 0 }), marcada({ ticker: 'VISC11', precoUnitario: null })]))
      .toBe('Preencha cotas e preço de sem código, VISC11.');
    const completa = marcada({ ticker: ' mxrf11 ' });
    expect(avisoDeLinhasIncompletas([completa])).toBeNull();
    expect(itensDaImportacao([completa])).toEqual([
      { transacaoId: 1, ticker: 'MXRF11', tipo: TipoOperacao.Compra, quantidade: 10, precoUnitario: 9.85, taxas: 0 }
    ]);
  });
});
