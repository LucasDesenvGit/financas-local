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
const ano = hoje.getFullYear();
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
  { id: 3, nome: 'Poupança Exemplo com um nome bem comprido, que quebra em duas linhas', tipo: 1, origem: 1, numero: null, moeda: 'BRL', saldo: 12000, saldoInicial: 12000,
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
    creditosCorretoraComoReceita: 0, totalCreditosCorretoraComoReceita: 0
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
  '/api/transacoes': { itens: transacoes, total: transacoes.length, pagina: 1, tamanhoPagina: 50, totalReceitas: 5000, totalDespesas: 152.3 },
  '/api/transacoes/sugestoes-categoria': [],
  '/api/categorias': categorias,
  '/api/regras': [],
  '/api/regras-descricao': [],
  '/api/orcamentos': [],
  '/api/metas': [],
  '/api/reservas': [],
  '/api/metas/plano': { disponivel: false, mesesConsiderados: 1, ano, mes, margemPercentual: 10, saldoContas: 0, faturaAPagar: 0,
    saldoFimDoMes: 0, melhorMesRecente: 0, meses: [] },
  '/api/contas-agendadas': [],
  '/api/contas-agendadas/sugestoes': [],
  '/api/investimentos': [],
  '/api/emprestimos': { financiamentos: credito(1), emprestimos: credito(2), contratos: [], pagamentosPorMes: [] },
  '/api/cambio/moedas': [],
  '/api/carteira': {
    selic: { metaAnual: 15, acumulado12Meses: 14.2, acumuladoAno: 11.5, atualizadaAte: iso(hoje) },
    rendaVariavel: { valorMercado: 0, custo: 0, valorizacao: 0, valorizacaoPercentual: null, lucroRealizado: 0, proventosRecebidos: 0,
      proventos12Meses: 0, resultadoTotal: 0, valorSeFosseSelic: 0, diferencaSelic: 0, resultadoTotalPercentual: null,
      resultadoSelicPercentual: null },
    posicoes: [], evolucao: [], openFinance: [], totalOpenFinance: 0, totalInvestimentosManuais: 0, totalGeral: 0, distribuicao: [],
    cotacoesAtualizadasEm: null, cotacoesDesatualizadas: false, openFinanceNaoRegistrados: 0, moedasSemCotacao: [], reservas: [],
    totalReservas: 0, cotasDiferentes: []
  },
  '/api/carteira/fiis': {
    resumo: { valorMercado: 0, custo: 0, valorizacao: 0, valorizacaoPercentual: null, renda12Meses: 0, rendaMediaMensal: 0,
      rendaMensalEstimada: 0, dy12Meses: null, yieldOnCost12Meses: null, recebidoExtrato12Meses: 0, rentabilidadeTotal: null,
      rentabilidadeSelic: null, rentabilidadeIfix: null, quantidadeFiis: 0 },
    fiis: [], lotes: [], proventos: [], evolucao: [], rentabilidade: [], segmentos: [], historicos: [], segmentosDisponiveis: [],
    mesAMes: []
  },
  '/api/carteira/extrato/sugestoes': [],
  '/api/carteira/conferencia': { tipos: [], cotas: [], comprasSemOperacao: [], trocos: [], rendimentos: [], categoriaDoTroco: null },
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
