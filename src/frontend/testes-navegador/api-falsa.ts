import { Page, Route } from '@playwright/test';

/**
 * API falsa dos testes no navegador (#185, ADR 0041): responde as chamadas /api/** com dados de exemplo, sem back-end
 * e sem nenhum dado real. Um GET que ninguém simulou é anotado em `semSimulacao` e o teste falha: tela nova, endpoint
 * novo aqui.
 */
export interface ApiFalsa {
  /** "GET /api/x": chamadas sem simulação (o teste falha se houver). */
  semSimulacao: string[];
  /** Corpo dos erros que o site mandou para /api/erros/front-end (o teste falha se houver). */
  errosDoSite: string[];
  /** Todas as chamadas, para conferir uma ação ("POST /api/metas"…). */
  chamadas: string[];
}

const hoje = new Date();
/** "AAAA-MM-DD" na data local. */
const dataIso = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
const ano = hoje.getFullYear();
/** Reserva fictícia (aba Reservas de Investimentos). */
const reserva = (campos: Record<string, unknown>) => ({
  contaId: 1, nomeConta: 'Banco Fictício', nome: 'Reserva', saldo: 0, guardado: 500, retirado: 100, movimentos: 3,
  primeiroMovimento: dataIso(new Date(ano, hoje.getMonth() - 2, 5)), ultimoMovimento: dataIso(new Date(ano, hoje.getMonth(), 1)),
  saldoConferido: null, dataConferencia: null, podeTerSaldoAnterior: false, conferenciaAntiga: false, ...campos
});
const mes = hoje.getMonth() + 1;
const iso = (d: Date) => d.toISOString().slice(0, 10);
const mesesAtras = (n: number) => {
  const d = new Date(ano, hoje.getMonth() - n, 1);
  return { ano: d.getFullYear(), mes: d.getMonth() + 1 };
};

const categorias = [
  { id: 1, nome: 'Alimentação', tipo: 1, cor: '#f2b880', icone: null, categoriaPaiId: null, ativa: true, natureza: 1 },
  { id: 2, nome: 'Transporte', tipo: 1, cor: '#92c8f0', icone: null, categoriaPaiId: null, ativa: true, natureza: 1 },
  { id: 3, nome: 'Lazer', tipo: 1, cor: '#c3b1e1', icone: null, categoriaPaiId: null, ativa: true, natureza: 2 },
  { id: 4, nome: 'Salário', tipo: 2, cor: '#8fd8b0', icone: null, categoriaPaiId: null, ativa: true, natureza: null },
  { id: 5, nome: 'Transferências', tipo: 3, cor: '#b5b5c3', icone: null, categoriaPaiId: null, ativa: true, natureza: null }
];

const contas = [
  { id: 1, nome: 'Conta Exemplo', tipo: 1, origem: 2, numero: null, moeda: 'BRL', saldo: 3250.5, saldoInicial: 0,
    limiteCredito: null, limiteDisponivel: null, diaFechamento: null, diaVencimento: null, cor: '#a7a3f5', incluirNoSaldo: true,
    arquivada: false, conexaoBancariaId: 1, nomeInstituicao: 'Banco Exemplo', atualizadoEm: hoje.toISOString(), banco: null },
  { id: 2, nome: 'Cartão Exemplo', tipo: 3, origem: 2, numero: null, moeda: 'BRL', saldo: -820.1, saldoInicial: 0,
    limiteCredito: 5000, limiteDisponivel: 4179.9, diaFechamento: 3, diaVencimento: 10, cor: '#92c8f0', incluirNoSaldo: true,
    arquivada: false, conexaoBancariaId: 1, nomeInstituicao: 'Banco Exemplo', atualizadoEm: hoje.toISOString(), banco: null },
  // #251: cartão de crédito com a fatura do mês já paga e a próxima, ainda aberta.
  { id: 3, nome: 'Cartão Crédito', tipo: 3, origem: 2, numero: '5555 4444 3333 1234', moeda: 'BRL', saldo: -207.8, saldoInicial: 0,
    limiteCredito: 3000, limiteDisponivel: 2792.2, diaFechamento: 3, diaVencimento: 10, cor: '#d4a5e8', incluirNoSaldo: true,
    arquivada: false, conexaoBancariaId: 1, nomeInstituicao: 'Banco Exemplo', atualizadoEm: hoje.toISOString(), banco: null,
    faturaMes: { contaId: 3, cartao: 'Cartão Crédito', valorMes: 612.3, parcelasMes: 0, faturaId: 9,
      vencimento: dataIso(new Date(ano, hoje.getMonth(), 10)), situacao: 'paga', fechamento: dataIso(new Date(ano, hoje.getMonth(), 3)),
      proxima: { contaId: 3, cartao: 'Cartão Crédito', valorMes: 207.8, parcelasMes: 0, faturaId: null,
        vencimento: dataIso(new Date(ano, hoje.getMonth() + 1, 10)), situacao: 'aberta',
        fechamento: dataIso(new Date(ano, hoje.getMonth() + 1, 3)) } } },
  { id: 5, nome: 'Poupança Exemplo com um nome bem comprido, que quebra em duas linhas', tipo: 1, origem: 1, numero: null, moeda: 'BRL', saldo: 12000, saldoInicial: 12000,
    limiteCredito: null, limiteDisponivel: null, diaFechamento: null, diaVencimento: null, cor: '#8fd8b0', incluirNoSaldo: true,
    arquivada: false, conexaoBancariaId: null, nomeInstituicao: null, atualizadoEm: hoje.toISOString(), banco: null },
  { id: 4, nome: 'Cartão Segundo com um nome bem comprido, que quebra em duas linhas', tipo: 3, origem: 1, numero: null, moeda: 'BRL', saldo: -150, saldoInicial: 0,
    limiteCredito: null, limiteDisponivel: null, diaFechamento: 20, diaVencimento: 28, cor: '#f2b880', incluirNoSaldo: true,
    arquivada: false, conexaoBancariaId: null, nomeInstituicao: null, atualizadoEm: hoje.toISOString(), banco: null }
];

const transacoes = [
  { id: 1, contaId: 2, contaNome: 'Cartão Exemplo', data: iso(hoje), descricao: 'Mercado do bairro', descricaoOriginal: 'MERCADO DO BAIRRO',
    valor: -152.3, tipo: 1, status: 1, categoriaId: 1, categoriaNome: 'Alimentação', categoriaCor: '#f2b880', categoriaManual: false,
    ignorarRelatorios: false, emergencia: false, observacao: null, parcelaNumero: null, parcelaTotal: null },
  { id: 2, contaId: 1, contaNome: 'Conta Exemplo', data: iso(hoje), descricao: 'Salário', descricaoOriginal: 'SALARIO',
    valor: 5000, tipo: 2, status: 1, categoriaId: 4, categoriaNome: 'Salário', categoriaCor: '#8fd8b0', categoriaManual: false,
    ignorarRelatorios: false, emergencia: false, observacao: null, parcelaNumero: null, parcelaTotal: null }
];

const porCategoria = [
  { categoriaId: 1, nome: 'Alimentação', cor: '#f2b880', valor: 1200 },
  { categoriaId: 2, nome: 'Transporte', cor: '#92c8f0', valor: 450 },
  { categoriaId: 3, nome: 'Lazer', cor: '#c3b1e1', valor: 300 }
];
const evolucao = Array.from({ length: 12 }, (_, i) => ({ ...mesesAtras(11 - i), receitas: 5000, despesas: 3500 + i * 40 }));
const resumoAno = { receitas: 50000, despesas: 38000, resultado: 12000, taxaPoupanca: 24 };
const credito = (natureza: number) => ({ natureza, contratosAbertos: 0, contratosQuitados: 0, saldoDevedor: 0, valorContratado: 0,
  parcelaMensal: 0, totalPago: 0, parcelasEmAtraso: 0 });

/** Respostas dos GET, pelo caminho sem a query. */
const respostas: Record<string, unknown> = {
  '/api/auth/estado': { configurado: true, autenticado: true, login: 'teste', versao: '0.0.0-teste' },
  '/api/painel': {
    saldoContas: 3250.5, dividaCartoes: 820.1, totalInvestido: 10000, patrimonioLiquido: 12430.4, rendaVariavel: 4000,
    patrimonioSemRendaVariavel: 8430.4, receitasMes: 5000, despesasMes: 1950, gastosPorCategoria: porCategoria, evolucao,
    orcamentos: [], proximasContas: [], ultimasTransacoes: transacoes, alertas: [], ultimaSincronizacao: hoje.toISOString(),
    despesasEmergenciaMes: 0, dividaFinanciamentos: 0, dividaEmprestimos: 0, diasProximosVencimentos: 7,
    faturasDoMes: [
      { contaId: 2, cartao: 'Cartão Exemplo', valorMes: 820.1, parcelasMes: 0, faturaId: null, vencimento: null, situacao: null },
      { contaId: 4, cartao: 'Cartão Segundo com um nome bem comprido, que quebra em duas linhas', valorMes: 150, parcelasMes: 0, faturaId: null, vencimento: null, situacao: null }
    ],
    totalFaturasDoMes: 970.1, poucoInformativas: 0, poucoInformativasDesde: null, exemploPoucoInformativa: null,
    creditosCorretoraComoReceita: 0, totalCreditosCorretoraComoReceita: 0,
    trocosAutomaticos: { quantidade: 2, total: 5.4, ids: [1, 2], desde: dataIso(new Date(ano, hoje.getMonth(), 1)), ate: dataIso(hoje) }
  },
  '/api/relatorios/categorias': porCategoria,
  '/api/relatorios/fluxo': evolucao,
  '/api/relatorios/maiores-despesas': transacoes.filter((t) => t.valor < 0),
  '/api/relatorios/anual': {
    ano, anoComparacao: ano - 1, mesLimite: mes, anosDisponiveis: [ano - 1, ano],
    resumo: resumoAno, resumoAnteriorMesmoPeriodo: resumoAno, resumoAnteriorCompleto: resumoAno,
    meses: Array.from({ length: 12 }, (_, i) => ({ mes: i + 1, receitas: 5000, despesas: 3800, receitasAnterior: 4800,
      despesasAnterior: 3700, patrimonioEstimado: 10000 + i * 500, futuro: i + 1 > mes })),
    categorias: [],
    distribuicao: { receitas: 50000, essenciais: 22000, estiloDeVida: 12000, naoClassificado: 4000, poupanca: 12000 },
    saude: { patrimonioAtual: 12430, reservaLiquida: 8000, gastoMensalMedio: 3800, gastoEssencialMensalMedio: 2200,
      mesesDeReserva: 2.1, mesesDeReservaEssencial: 3.6, rendaMensalMedia: 5000, parcelasMensais: 0, comprometimentoParcelas: 0,
      mesesAnalisados: 12, parcelasFinanciamentos: 0, parcelasEmprestimos: 0, dividaFinanciamentos: 0, dividaEmprestimos: 0 }
  },
  '/api/relatorios/projecao': {
    mesesConsiderados: 6, mediaReceitas: 5000, mediaDespesas: 3800, saldoDisponivelAtual: 3250.5,
    meses: Array.from({ length: 6 }, (_, i) => ({ ...mesesAtras(-i), receitas: 5000, despesasAVista: 3300, parcelas: 500,
      despesas: 3800, resultado: 1200, saldoProjetado: 3250 + 1200 * (i + 1), atual: i === 0 })),
    categorias: []
  },
  '/api/contas': contas,
  '/api/contas/bancos': [],
  // #172: "Conferir com o banco".
  '/api/conferencia-banco': {
    contas: [{ id: 1, nome: 'Conta Corrente', banco: 'nubank', openFinance: true, saldo: 1000, pistas: [
      { texto: 'O saldo vem do banco, atualizado em 19/10/2026. O que entrou ou saiu depois ainda não chegou.', valor: null },
      { texto: 'Reservas e caixinhas desta conta (o app pode mostrar o saldo com elas)', valor: 2000, rota: '/investimentos',
        parametros: { aba: 'reservas' } }
    ] }],
    cartoes: [{ id: 2, nome: 'Cartão Roxo', banco: 'nubank', faturaMes: 511.72, situacao: 'aberta', fechamento: null,
      vencimento: '2026-11-10', dividaTotal: 1200, pistas: [
        { texto: 'Parcelas nesta fatura (o app pode mostrá-las à parte)', valor: 59.9 }
      ] }],
    investimentos: [{ chave: 'of-1', nome: 'CDB · Nubank', origem: 'Open Finance', saldo: 1500, pistas: [] }]
  },
  '/api/transacoes': { itens: transacoes, total: transacoes.length, pagina: 1, tamanhoPagina: 50, totalReceitas: 5000, totalDespesas: 152.3 },
  '/api/transacoes/sugestoes-categoria': [],
  // A edição de uma transação pede as sugestões dela (teclado.spec.ts abre a primeira).
  '/api/transacoes/1/sugestoes-categoria': [],
  '/api/transacoes/2/sugestoes-categoria': [],
  '/api/categorias': categorias,
  '/api/regras': [],
  // #241: as duas contas do condomínio, separadas pelo valor.
  '/api/regras-descricao': [
    { id: 1, texto: 'CONDOMINIO', tipoCorrespondencia: 1, novaDescricao: 'Condomínio — taxa', ativa: true, transacoes: 6,
      valorMinimo: 800, valorMaximo: 900 },
    { id: 2, texto: 'CONDOMINIO', tipoCorrespondencia: 1, novaDescricao: 'Condomínio — fundo de reserva', ativa: true,
      transacoes: 6, valorMinimo: null, valorMaximo: 250 }
  ],
  '/api/regras-descricao/previa': { quantidade: 6, exemplos: ['PAGAMENTO CONDOMINIO EDIF SOLAR'] },
  '/api/orcamentos': [],
  '/api/metas': [],
  '/api/reservas': [],
  '/api/metas/plano': { disponivel: false, mesesConsiderados: 1, ano, mes, margemPercentual: 10, saldoContas: 0, faturaAPagar: 0,
    saldoFimDoMes: 0, melhorMesRecente: 0, meses: [] },
  // #158: uma conta paga à mão, um débito automático e uma assinatura no cartão (baixa sozinha), e duas sugestões.
  '/api/contas-agendadas': [
    { id: 1, descricao: 'Escola', valor: 1450, proximoVencimento: dataIso(new Date(ano, hoje.getMonth() + 1, 10)),
      recorrencia: 1, categoriaId: null, categoriaNome: 'Educação', contaId: 1, contaNome: 'Conta Exemplo', ativa: true,
      ultimoPagamentoEm: null, diasParaVencer: 20, dataLimite: null, pagamentosRestantes: null, encerrada: false,
      formaCobranca: 1, textoCobranca: null, observacao: null },
    { id: 2, descricao: 'Luz', valor: 180, proximoVencimento: dataIso(new Date(ano, hoje.getMonth() + 1, 15)),
      recorrencia: 1, categoriaId: null, categoriaNome: 'Moradia', contaId: 1, contaNome: 'Conta Exemplo', ativa: true,
      ultimoPagamentoEm: null, diasParaVencer: 25, dataLimite: null, pagamentosRestantes: null, encerrada: false,
      formaCobranca: 2, textoCobranca: 'ENERGIA', observacao: null },
    { id: 3, descricao: 'Streaming', valor: 44.9, proximoVencimento: dataIso(new Date(ano, hoje.getMonth() + 1, 12)),
      recorrencia: 1, categoriaId: null, categoriaNome: 'Lazer', contaId: 2, contaNome: 'Cartão Exemplo', ativa: true,
      ultimoPagamentoEm: null, diasParaVencer: 22, dataLimite: null, pagamentosRestantes: null, encerrada: false,
      formaCobranca: 3, textoCobranca: 'STREAMING', observacao: 'conta da família' }
  ],
  // #158: um aviso de aumento, com a ação "Atualizar o valor".
  '/api/contas-agendadas/avisos': [
    { contaAgendadaId: 3, descricao: 'Streaming', tipo: 'aumentou', texto: 'Streaming aumentou na última cobrança.',
      valorAnterior: 44.9, valorNovo: 49.9, contaNovaId: null, contaNovaNome: null, data: null }
  ],
  '/api/contas-agendadas/sugestoes': [
    { chave: 'musica#2', descricao: 'MUSICA APP', valor: 21.9, valorVariavel: false, valorMinimo: 21.9, valorMaximo: 21.9,
      proximoVencimento: dataIso(new Date(ano, hoje.getMonth() + 1, 3)), diaHabitual: 3, meses: 6, pagamentos: 6,
      ultimoPagamento: dataIso(new Date(ano, hoje.getMonth(), 3)), primeiroPagamento: dataIso(new Date(ano, hoje.getMonth() - 5, 3)),
      categoriaId: null, categoriaNome: null, contaId: 2, contaNome: 'Cartão Exemplo', termoBusca: 'MUSICA',
      vencimentoSemPagamento: null, formaCobranca: 3, textoCobranca: 'MUSICA', recorrencia: 1,
      chaveExtrato: 'MUSICA', pareceSerId: null, pareceSerNome: null },
    { chave: 'nuvem', descricao: 'NUVEM ANUAL', valor: 119, valorVariavel: false, valorMinimo: 119, valorMaximo: 119,
      proximoVencimento: dataIso(new Date(ano + 1, hoje.getMonth(), 5)), diaHabitual: 5, meses: 2, pagamentos: 2,
      ultimoPagamento: dataIso(new Date(ano, hoje.getMonth(), 5)), primeiroPagamento: dataIso(new Date(ano - 1, hoje.getMonth(), 5)),
      categoriaId: null, categoriaNome: null, contaId: 2, contaNome: 'Cartão Exemplo', termoBusca: 'NUVEM',
      vencimentoSemPagamento: null, formaCobranca: 3, textoCobranca: 'NUVEM', recorrencia: 2,
      chaveExtrato: 'NUVEM', pareceSerId: null, pareceSerNome: null },
    // #205: parecida com "Escola" (valor, dia e conta), com outro nome: "parece ser Escola" e "é a mesma".
    { chave: 'PESSOA EXEMPLO', descricao: 'PIX ENVIADO PESSOA EXEMPLO', valor: 1450, valorVariavel: false, valorMinimo: 1450,
      valorMaximo: 1450, proximoVencimento: dataIso(new Date(ano, hoje.getMonth() + 1, 10)), diaHabitual: 10, meses: 6, pagamentos: 6,
      ultimoPagamento: dataIso(new Date(ano, hoje.getMonth(), 10)), primeiroPagamento: dataIso(new Date(ano, hoje.getMonth() - 5, 10)),
      categoriaId: null, categoriaNome: null, contaId: 1, contaNome: 'Conta Exemplo', termoBusca: 'PESSOA',
      vencimentoSemPagamento: null, formaCobranca: 1, textoCobranca: 'PESSOA', recorrencia: 1,
      chaveExtrato: 'PESSOA EXEMPLO', pareceSerId: 1, pareceSerNome: 'Escola' }
  ],
  '/api/investimentos': [],
  '/api/investimentos/historico': {
    desde: dataIso(new Date(ano, hoje.getMonth() - 1, 20)),
    meses: [
      { ...mesesAtras(1), desde: dataIso(new Date(ano, hoje.getMonth() - 1, 20)), rendimento: 12.3, saldo: 5012.3,
        porInvestimento: [{ nome: 'Caixinha viagem', rendimento: 10, saldo: 4010 }, { nome: 'CDB · Banco X', rendimento: 2.3, saldo: 1002.3 }] },
      { ...mesesAtras(0), desde: null, rendimento: 40.5, saldo: 5552.8,
        porInvestimento: [{ nome: 'Caixinha viagem', rendimento: 33, saldo: 4543 }, { nome: 'Fundo Y', rendimento: null, saldo: 1009.8 }] }
    ]
  },
  '/api/emprestimos': { financiamentos: credito(1), emprestimos: credito(2), contratos: [], pagamentosPorMes: [] },
  '/api/cambio/moedas': [],
  '/api/carteira': {
    selic: { metaAnual: 15, acumulado12Meses: 14.2, acumuladoAno: 11.5, atualizadaAte: iso(hoje) },
    rendaVariavel: { valorMercado: 0, custo: 0, valorizacao: 0, valorizacaoPercentual: null, lucroRealizado: 0, proventosRecebidos: 0,
      proventos12Meses: 0, resultadoTotal: 0, valorSeFosseSelic: 0, diferencaSelic: 0, resultadoTotalPercentual: null,
      resultadoSelicPercentual: null },
    posicoes: [], evolucao: [], openFinance: [], totalOpenFinance: 0, totalInvestimentosManuais: 0, totalGeral: 0, distribuicao: [],
    cotacoesAtualizadasEm: null, cotacoesDesatualizadas: false, openFinanceNaoRegistrados: 0, moedasSemCotacao: [],
    // #237: numa conta, o banco detalha uma reserva pelo nome e outra fica fora da lista; na outra, manda só o total.
    reservas: [
      reserva({ contaId: 1, nomeConta: 'Banco Fictício', nome: 'Viagem', saldo: 1200, informadoPeloBanco: true }),
      reserva({ contaId: 1, nomeConta: 'Banco Fictício', nome: 'Presentes', saldo: 150, foraDaListaDoBanco: true }),
      reserva({ contaId: 2, nomeConta: 'Carteira Digital', nome: 'Carro', saldo: 400, podeTerSaldoAnterior: true }),
      reserva({ contaId: 2, nomeConta: 'Carteira Digital', nome: 'Reservado', saldo: 600, guardado: 0, retirado: 0, movimentos: 0,
        primeiroMovimento: null, ultimoMovimento: null, restanteDoBanco: true, totalDoBanco: 1000 })
    ],
    totalReservas: 2350, cotasDiferentes: []
  },
  // #173: um FII fictício e seis meses de "Mês a mês" (aporte, subscrição, venda, rendimentos e valorização dos dois sinais).
  '/api/carteira/fiis': {
    resumo: { valorMercado: 1020, custo: 980, valorizacao: 40, valorizacaoPercentual: 4.08, renda12Meses: 60, rendaMediaMensal: 10,
      rendaMensalEstimada: 10, dy12Meses: 5.9, yieldOnCost12Meses: 6.1, recebidoExtrato12Meses: 60, rentabilidadeTotal: 10.2,
      rentabilidadeSelic: 8.5, rentabilidadeIfix: 3.1, quantidadeFiis: 1 },
    fiis: [{ id: 1, ticker: 'MXRF11', nome: 'Fundo de Teste', segmento: 'Papel (recebíveis)', segmentoSugerido: false,
      quantidade: 100, precoMedio: 9.8, custo: 980, precoAtual: 10.2, variacaoDia: 0.3, valorMercado: 1020, valorizacao: 40,
      valorizacaoPercentual: 4.08, ultimoRendimento: 0.1, dataComUltimo: '2026-09-30', rendimentosPorCota12Meses: 0.6,
      dyMensal: 0.98, dy12Meses: 5.9, yieldOnCost12Meses: 6.1, yieldOnCostMensal: 1.02, rendaMensalEstimada: 10,
      proventos12Meses: 60, proventosTotal: 60, recebidoExtrato12Meses: 60, pesoCarteira: 100, fonteCotacao: 'Yahoo',
      precoManual: false }],
    lotes: [], proventos: [], evolucao: [], rentabilidade: [], segmentos: [{ nome: 'Papel (recebíveis)', valor: 1020 }],
    historicos: [], segmentosDisponiveis: ['Papel (recebíveis)'],
    mesAMes: [
      { ...mesesAtras(5), aportes: 980, subscricoes: 0, resgates: 0, rendimentosRecebidos: 0, rendimentosEstimados: 0, valorizacao: -12.5, saldo: 967.5,
        idsAportes: [101] },
      { ...mesesAtras(4), aportes: 0, subscricoes: 0, resgates: 0, rendimentosRecebidos: 10, rendimentosEstimados: 0, valorizacao: 8, saldo: 975.5 },
      { ...mesesAtras(3), aportes: 0, subscricoes: 15, resgates: 0, rendimentosRecebidos: 10, rendimentosEstimados: 0, valorizacao: -5.5, saldo: 985 },
      { ...mesesAtras(2), aportes: 0, subscricoes: 0, resgates: 20, rendimentosRecebidos: 10, rendimentosEstimados: 0, valorizacao: 15, saldo: 980 },
      { ...mesesAtras(1), aportes: 0, subscricoes: 0, resgates: 0, rendimentosRecebidos: 10, rendimentosEstimados: 0, valorizacao: 22, saldo: 1002 },
      { ...mesesAtras(0), aportes: 0, subscricoes: 0, resgates: 0, rendimentosRecebidos: 10, rendimentosEstimados: 10, valorizacao: 18, saldo: 1020,
        porFii: [{ ticker: 'MXRF11', valor: 8 }, { ticker: null, valor: 2 }], idsRendimentos: [201, 202] }
    ]
  },
  '/api/carteira/extrato/sugestoes': [],
  '/api/carteira/conferencia': { tipos: [], cotas: [], comprasSemOperacao: [], trocos: [], rendimentos: [], categoriaDoTroco: null,
    taxas: [{ data: dataIso(new Date(ano, hoje.getMonth() - 1, 8)), conta: 'Nubank', debitado: 1010.4, trocos: 10.1, valorDasCotas: 1000,
      taxa: 0.3, operacoes: [{ operacaoId: 31, ticker: 'MXRF11', quantidade: 100, taxa: 0.3 }] }] },
  '/api/carteira/configuracao': { tokenBrapiConfigurado: false },
  '/api/parametros': [],
  '/api/parametros/sistema': { versao: '0.0.0-teste', dotNet: '10.0', sistemaOperacional: 'Teste', iniciadoEm: hoje.toISOString(),
    caminhoBanco: '/teste/FinancasLocal.db', tamanhoBancoBytes: 1024, pastaChaves: '/teste/chaves', contas: 2, transacoes: 2,
    conexoesAtivas: 1, ultimaSincronizacao: hoje.toISOString() },
  '/api/open-finance/configuracao': { clientId: null, secretConfigurado: false, intervaloHoras: 6, diasRetroativos: 30, habilitada: true },
  '/api/open-finance/conexoes': [],
  '/api/open-finance/status': { ultimaSincronizacao: hoje.toISOString(), emExecucao: false, conexoesAtivas: 1, conexoesComErro: 0,
    consentimentosVencendo: 0 },
  '/api/open-finance/logs': [],
  '/api/open-finance/categorias': [],
  '/api/auth/auditoria': [],
  '/api/erros': { itens: [], total: 0, pagina: 1, tamanhoPagina: 50, abertos: 0, origens: [] }
};

/** Liga a API falsa na página. `alterar` troca respostas para um teste ("/api/auth/estado" deslogado…). */
export async function ligarApiFalsa(page: Page, alterar: Record<string, unknown> = {}): Promise<ApiFalsa> {
  const api: ApiFalsa = { semSimulacao: [], errosDoSite: [], chamadas: [] };
  const tabela = { ...respostas, ...alterar };
  await page.route('**/api/**', async (route: Route) => {
    const req = route.request();
    const caminho = new URL(req.url()).pathname;
    api.chamadas.push(`${req.method()} ${caminho}`);
    if (req.method() === 'POST' && caminho === '/api/erros/front-end') {
      api.errosDoSite.push(req.postData() ?? '');
      return route.fulfill({ status: 204 });
    }
    if (req.method() !== 'GET') return route.fulfill({ status: 200, contentType: 'application/json', body: '{}' });
    if (!(caminho in tabela)) {
      api.semSimulacao.push(`GET ${caminho}`);
      return route.fulfill({ status: 404, contentType: 'application/json', body: '{"mensagem":"sem simulação"}' });
    }
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(tabela[caminho]) });
  });
  return api;
}
