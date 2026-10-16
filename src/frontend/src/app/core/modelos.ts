/* Tipos espelhando os DTOs da API (Financas.Api/Dtos/Dtos.cs). Datas "yyyy-MM-dd" chegam como string. */

export enum TipoCategoria { Despesa = 1, Receita = 2, Transferencia = 3 }
export enum TipoConta { ContaCorrente = 1, Poupanca = 2, CartaoCredito = 3, Investimento = 4, Dinheiro = 5, Outro = 6 }
export enum OrigemConta { Manual = 1, OpenFinance = 2 }
export enum TipoTransacao { Despesa = 1, Receita = 2, Transferencia = 3 }
export enum StatusTransacao { Efetivada = 1, Pendente = 2 }
export enum TipoCorrespondencia { Contem = 1, ComecaCom = 2, Igual = 3 }
export enum Recorrencia { Unica = 0, Mensal = 1, Anual = 2 }
export enum NaturezaGasto { Essencial = 1, EstiloDeVida = 2 }
export enum NaturezaCredito { Emprestimo = 1, Financiamento = 2 }

export const NOMES_TIPO_CONTA: Record<TipoConta, string> = {
  [TipoConta.ContaCorrente]: 'Conta corrente',
  [TipoConta.Poupanca]: 'Poupança',
  [TipoConta.CartaoCredito]: 'Cartão de crédito',
  [TipoConta.Investimento]: 'Investimento',
  [TipoConta.Dinheiro]: 'Dinheiro / carteira',
  [TipoConta.Outro]: 'Outro'
};

export const NOMES_TIPO_CATEGORIA: Record<TipoCategoria, string> = {
  [TipoCategoria.Despesa]: 'Despesa',
  [TipoCategoria.Receita]: 'Receita',
  [TipoCategoria.Transferencia]: 'Transferência'
};

export const NOMES_RECORRENCIA: Record<Recorrencia, string> = {
  [Recorrencia.Unica]: 'Única',
  [Recorrencia.Mensal]: 'Mensal',
  [Recorrencia.Anual]: 'Anual'
};

export const NOMES_CORRESPONDENCIA: Record<TipoCorrespondencia, string> = {
  [TipoCorrespondencia.Contem]: 'Contém',
  [TipoCorrespondencia.ComecaCom]: 'Começa com',
  [TipoCorrespondencia.Igual]: 'É igual a'
};

export interface EstadoAutenticacao { configurado: boolean; autenticado: boolean; login: string | null; }
export interface Mensagem { mensagem: string; }

export interface Conta {
  id: number; nome: string; tipo: TipoConta; origem: OrigemConta; numero: string | null; moeda: string;
  saldo: number; saldoInicial: number; limiteCredito: number | null; limiteDisponivel: number | null;
  diaFechamento: number | null; diaVencimento: number | null; cor: string; incluirNoSaldo: boolean; arquivada: boolean;
  conexaoBancariaId: number | null; nomeInstituicao: string | null; atualizadoEm: string;
}

export interface SalvarConta {
  nome: string; tipo: TipoConta; numero: string | null; saldoInicial: number; limiteCredito: number | null;
  diaFechamento: number | null; diaVencimento: number | null; cor: string; incluirNoSaldo: boolean; arquivada: boolean;
}

export interface Fatura {
  id: number; dataVencimento: string; dataFechamento: string | null; valorTotal: number;
  valorMinimo: number | null; valorPago: number | null; quantidadeTransacoes: number;
}

export interface Transacao {
  id: number; contaId: number; contaNome: string; openFinance: boolean; data: string; descricao: string;
  descricaoOriginal: string | null; estabelecimento: string | null; valor: number; tipo: TipoTransacao;
  status: StatusTransacao; categoriaId: number | null; categoriaNome: string | null; categoriaCor: string | null;
  parcelaNumero: number | null; parcelaTotal: number | null; ignorarRelatorios: boolean; observacao: string | null;
  emergencia: boolean;
}

export interface FiltroTransacoes {
  inicio?: string | null; fim?: string | null; contaId?: number | null; categoriaId?: number | null;
  semCategoria?: boolean; somenteEmergencias?: boolean; tipo?: TipoTransacao | null; busca?: string | null; pagina?: number; tamanhoPagina?: number;
}

export interface PaginaTransacoes {
  itens: Transacao[]; total: number; pagina: number; tamanhoPagina: number; totalReceitas: number; totalDespesas: number;
}

export interface SalvarTransacao {
  contaId: number; data: string; descricao: string; valor: number; tipo: TipoTransacao; categoriaId: number | null;
  observacao: string | null; ignorarRelatorios: boolean; status: StatusTransacao; emergencia: boolean;
}

export interface AjusteTransacao {
  descricao: string; categoriaId: number | null; observacao: string | null; ignorarRelatorios: boolean;
  emergencia: boolean;
}

export interface Transferencia {
  contaOrigemId: number; contaDestinoId: number; valor: number; data: string; descricao: string | null;
}

export interface Categoria {
  id: number; nome: string; tipo: TipoCategoria; cor: string; icone: string | null; categoriaPaiId: number | null; ativa: boolean;
  natureza: NaturezaGasto | null;
}

export interface SalvarCategoria {
  nome: string; tipo: TipoCategoria; cor: string; icone: string | null; categoriaPaiId: number | null; ativa: boolean;
  natureza: NaturezaGasto | null;
}

export interface Regra {
  id: number; texto: string; tipoCorrespondencia: TipoCorrespondencia; categoriaId: number; categoriaNome: string;
  prioridade: number; ativa: boolean;
}

export interface SalvarRegra {
  texto: string; tipoCorrespondencia: TipoCorrespondencia; categoriaId: number; prioridade: number; ativa: boolean;
}

export interface ResultadoAplicacao { transacoesAtualizadas: number; }

export interface Orcamento {
  id: number; categoriaId: number; categoriaNome: string; categoriaCor: string; ano: number; mes: number;
  valor: number; gasto: number; percentual: number; gastoEmergencia: number;
  /** ok | alerta | estourado | emergencia (passou do limite só por causa de emergências) */
  situacao: 'ok' | 'alerta' | 'estourado' | 'emergencia';
}

export interface Meta {
  id: number; nome: string; valorAlvo: number; valorAtual: number; dataAlvo: string | null; contaId: number | null;
  contaNome: string | null; cor: string; concluida: boolean; percentual: number; aporteMensalSugerido: number | null;
  valorInicial: number; saldoConta: number;
}

export interface SalvarMeta {
  nome: string; valorAlvo: number; valorAtual: number; dataAlvo: string | null; contaId: number | null;
  cor: string; concluida: boolean;
}

export interface ContaAgendada {
  id: number; descricao: string; valor: number; proximoVencimento: string; recorrencia: Recorrencia;
  categoriaId: number | null; categoriaNome: string | null; contaId: number | null; contaNome: string | null;
  ativa: boolean; ultimoPagamentoEm: string | null; diasParaVencer: number;
}

export interface SalvarContaAgendada {
  descricao: string; valor: number; proximoVencimento: string; recorrencia: Recorrencia;
  categoriaId: number | null; contaId: number | null; ativa: boolean;
}

export interface Investimento {
  id: number; nome: string; tipo: string; subtipo: string | null; codigo: string | null; emissor: string | null;
  moeda: string; saldo: number; saldoEmReais: number | null; valorAplicado: number | null; rendimento: number | null;
  taxaAnual: number | null;
  dataVencimento: string | null; status: string | null; nomeInstituicao: string | null; atualizadoEm: string;
}

export interface ValorCategoria { categoriaId: number | null; nome: string; cor: string; valor: number; }
export interface MesValores { ano: number; mes: number; receitas: number; despesas: number; despesasEmergencia: number; }

export interface OpcaoCategoria { categoriaId: number; nome: string; cor: string; motivo: string; }

export interface SugestaoCategoria {
  chave: string; exemplo: string; quantidade: number; total: number; transacaoIds: number[];
  categoriaId: number | null; categoriaNome: string | null; categoriaCor: string | null;
  confianca: 'alta' | 'media' | 'baixa' | 'nenhuma'; motivo: string | null; textoRegra: string;
  alternativas: OpcaoCategoria[];
}

export interface ResumoAno { receitas: number; despesas: number; resultado: number; taxaPoupanca: number; }
export interface MesComparativo {
  mes: number; receitas: number; despesas: number; receitasAnterior: number; despesasAnterior: number;
  patrimonioEstimado: number | null; futuro: boolean;
}
export interface CategoriaComparativa {
  categoriaId: number | null; nome: string; cor: string; natureza: NaturezaGasto | null; valor: number; valorAnterior: number; meses: number[];
}
export interface DistribuicaoRenda { receitas: number; essenciais: number; estiloDeVida: number; naoClassificado: number; poupanca: number; }
export interface SaudeFinanceira {
  patrimonioAtual: number; reservaLiquida: number; gastoMensalMedio: number; gastoEssencialMensalMedio: number;
  mesesDeReserva: number | null; mesesDeReservaEssencial: number | null; rendaMensalMedia: number; parcelasMensais: number;
  comprometimentoParcelas: number | null; mesesAnalisados: number;
  parcelasFinanciamentos: number; parcelasEmprestimos: number; dividaFinanciamentos: number; dividaEmprestimos: number;
}
export interface PainelAnual {
  ano: number; anoComparacao: number; mesLimite: number; anosDisponiveis: number[];
  resumo: ResumoAno; resumoAnteriorMesmoPeriodo: ResumoAno; resumoAnteriorCompleto: ResumoAno;
  meses: MesComparativo[]; categorias: CategoriaComparativa[]; distribuicao: DistribuicaoRenda; saude: SaudeFinanceira;
}

export interface ProjecaoMes {
  ano: number; mes: number; situacao: 'realizado' | 'em andamento' | 'projetado'; receitas: number; despesas: number;
  parcelas: number; resultado: number; saldoProjetado: number | null;
}
export interface ProjecaoCategoria { categoriaId: number | null; nome: string; cor: string; mediaMensal: number; ultimoMes: number; }
export interface Projecao {
  mesesConsiderados: number; mediaReceitas: number; mediaDespesas: number; saldoDisponivelAtual: number;
  meses: ProjecaoMes[]; categorias: ProjecaoCategoria[];
}

export interface Painel {
  saldoContas: number; dividaCartoes: number; totalInvestido: number; patrimonioLiquido: number;
  rendaVariavel: number; patrimonioSemRendaVariavel: number;
  receitasMes: number; despesasMes: number; gastosPorCategoria: ValorCategoria[]; evolucao: MesValores[];
  orcamentos: Orcamento[]; proximasContas: ContaAgendada[]; ultimasTransacoes: Transacao[]; alertas: string[];
  ultimaSincronizacao: string | null;
  despesasEmergenciaMes: number;
  dividaFinanciamentos: number; dividaEmprestimos: number;
}

export interface Emprestimo {
  id: number; natureza: NaturezaCredito; produto: string; modalidade: string | null; modalidadeDescricao: string;
  numeroContrato: string | null; nomeInstituicao: string | null; moeda: string; valorContratado: number; saldoDevedor: number;
  valorParcela: number | null; parcelaMensal: number; parcelasTotal: number | null; parcelasPagas: number | null;
  parcelasAVencer: number | null; parcelasEmAtraso: number; periodicidade: string | null; sistemaAmortizacao: string | null;
  taxaJuros: number | null; periodicidadeTaxa: string | null; indexador: string | null; cet: number | null;
  dataContratacao: string | null; dataVencimentoFinal: string | null; proximoVencimento: string | null;
  totalPago: number; ultimoPagamento: string | null; quitado: boolean; atualizadoEm: string;
}
export interface ResumoCredito {
  natureza: NaturezaCredito; contratosAbertos: number; contratosQuitados: number; saldoDevedor: number;
  valorContratado: number; parcelaMensal: number; totalPago: number; parcelasEmAtraso: number;
}
export interface PainelEmprestimos {
  financiamentos: ResumoCredito; emprestimos: ResumoCredito; contratos: Emprestimo[];
  pagamentosPorMes: { ano: number; mes: number; financiamentos: number; emprestimos: number }[];
}

export interface ConfiguracaoPluggy {
  clientId: string | null; secretConfigurado: boolean; intervaloHoras: number; diasRetroativos: number; habilitada: boolean;
}

export interface SalvarConfiguracaoPluggy {
  clientId: string; clientSecret: string | null; intervaloHoras: number; diasRetroativos: number; habilitada: boolean;
}

export interface Conexao {
  id: number; pluggyItemId: string; nomeInstituicao: string | null; corPrimaria: string | null; status: string | null;
  statusExecucao: string | null; ultimaAtualizacaoInstituicao: string | null; consentimentoExpiraEm: string | null;
  ultimaSincronizacaoEm: string | null; ultimoErro: string | null; ativa: boolean; quantidadeContas: number;
}

export interface SincronizacaoLog {
  id: number; conexaoBancariaId: number | null; nomeInstituicao: string | null; origem: string; iniciadoEm: string;
  finalizadoEm: string | null; sucesso: boolean; transacoesNovas: number; transacoesAtualizadas: number; mensagem: string | null;
}

export interface ResultadoSincronizacao {
  sucesso: boolean; transacoesNovas: number; transacoesAtualizadas: number; mensagem: string | null;
}

export interface CategoriaPluggy {
  pluggyCategoriaId: string; descricao: string; paiDescricao: string | null; categoriaId: number | null;
  categoriaNome: string | null; quantidadeTransacoes: number;
}

export interface Auditoria {
  id: number; dataHora: string; evento: string; login: string | null; ip: string | null; userAgent: string | null;
}

/* ----------------------------- Carteira de investimentos ----------------------------- */
export enum TipoAtivo { Fii = 1, Acao = 2, Etf = 3, Bdr = 4, Outro = 5 }
export enum TipoOperacao { Compra = 1, Venda = 2 }
export enum OrigemOperacao { Manual = 1, OpenFinance = 2, Extrato = 3 }

export const NOMES_ORIGEM_OPERACAO: Record<OrigemOperacao, string> = {
  [OrigemOperacao.Manual]: 'manual',
  [OrigemOperacao.OpenFinance]: 'Open Finance',
  [OrigemOperacao.Extrato]: 'extrato'
};

export const NOMES_TIPO_ATIVO: Record<TipoAtivo, string> = {
  [TipoAtivo.Fii]: 'FII',
  [TipoAtivo.Acao]: 'Ação',
  [TipoAtivo.Etf]: 'ETF',
  [TipoAtivo.Bdr]: 'BDR',
  [TipoAtivo.Outro]: 'Outro'
};

export interface SelicResumo {
  metaAnual: number | null; acumulado12Meses: number | null; acumuladoAno: number | null; atualizadaAte: string | null;
}

/** Preço médio, preço atual/anterior e último provento na moeda do ativo; custo, valores e resultados em reais. */
export interface PosicaoAtivo {
  id: number; ticker: string; nome: string | null; tipo: TipoAtivo; moeda: string;
  quantidade: number; precoMedio: number; custo: number;
  precoAtual: number | null; precoAtualEmReais: number | null; precoAnterior: number | null; variacaoDia: number | null;
  valorMercado: number; valorizacao: number; valorizacaoPercentual: number | null;
  lucroRealizado: number; proventosRecebidos: number; proventos12Meses: number; dividendYield12Meses: number | null;
  resultadoTotal: number; resultadoTotalPercentual: number | null;
  valorSeFosseSelic: number; diferencaSelic: number; resultadoSelicPercentual: number | null;
  primeiraCompra: string | null; ultimoProvento: { dataCom: string; valorPorCota: number } | null;
  precoManual: boolean; precoAtualizadoEm: string | null; fonteCotacao: string | null; semCotacao: boolean;
  segmento: string | null;
}

export interface TotaisRendaVariavel {
  valorMercado: number; custo: number; valorizacao: number; valorizacaoPercentual: number | null;
  lucroRealizado: number; proventosRecebidos: number; proventos12Meses: number; resultadoTotal: number;
  valorSeFosseSelic: number; diferencaSelic: number; resultadoTotalPercentual: number | null;
  resultadoSelicPercentual: number | null;
}

export interface EvolucaoCarteira { ano: number; mes: number; aplicado: number; aplicadoNaSelic: number; valorMercado: number | null }

export interface InvestimentoComparado {
  id: number; nome: string; tipo: string; subtipo: string | null; codigo: string | null; emissor: string | null;
  nomeInstituicao: string | null; moeda: string; saldo: number; saldoEmReais: number | null;
  valorAplicado: number | null; dataAplicacao: string | null;
  dataVencimento: string | null; taxaAnual: number | null; rentabilidade: number | null; selicNoPeriodo: number | null;
  percentualDaSelic: number | null; rendaVariavel: boolean; jaNaCarteira: boolean; atualizadoEm: string;
}

export interface Carteira {
  selic: SelicResumo; rendaVariavel: TotaisRendaVariavel; posicoes: PosicaoAtivo[]; evolucao: EvolucaoCarteira[];
  openFinance: InvestimentoComparado[]; totalOpenFinance: number; totalInvestimentosManuais: number; totalGeral: number;
  distribuicao: { nome: string; valor: number }[]; cotacoesAtualizadasEm: string | null; cotacoesDesatualizadas: boolean;
  openFinanceNaoRegistrados: number; moedasSemCotacao: string[];
}

export interface ResultadoImportacaoCarteira {
  ativosCriados: number; operacoesImportadas: number; semHistorico: string[]; comLancamentoManual: string[];
  erros: string[]; mensagem: string | null;
}

/** Preço, taxas e preço atual na moeda do ativo; valorTotal, valorAtual e Selic em reais. */
export interface OperacaoAtivo {
  id: number; ativoId: number; ticker: string; tipo: TipoOperacao; data: string; quantidade: number;
  precoUnitario: number; taxas: number; moeda: string; cambio: number; valorTotal: number; observacao: string | null;
  precoAtual: number | null; variacaoPreco: number | null; valorAtual: number | null; valorSeFosseSelic: number;
  origem: OrigemOperacao;
}

/** Preço e taxas na moeda do ativo; cambio = R$ por 1 unidade da moeda (ignorado em BRL). */
export interface SalvarOperacao {
  ticker: string; tipoAtivo: TipoAtivo | null; tipo: TipoOperacao; data: string; quantidade: number | null;
  precoUnitario: number | null; taxas: number; observacao: string | null; moeda: string; cambio: number | null;
}

/* ----------------------------- Câmbio ----------------------------- */
export interface MoedaCambio { codigo: string; nome: string; }
export interface CotacaoMoeda {
  moeda: string; dataSolicitada: string; data: string; compra: number; venda: number; fonte: string;
}

export interface AtualizarAtivo {
  nome: string | null; tipo: TipoAtivo; segmento: string | null; precoManual: boolean; precoAtual: number | null;
}

export interface ResultadoCotacoes { atualizados: number; proventosNovos: number; falhas: string[] }

/* ----------------------------- FIIs ----------------------------- */
export interface Fii {
  id: number; ticker: string; nome: string | null; segmento: string | null; segmentoSugerido: boolean;
  quantidade: number; precoMedio: number; custo: number; precoAtual: number | null; variacaoDia: number | null;
  valorMercado: number; valorizacao: number; valorizacaoPercentual: number | null;
  ultimoRendimento: number | null; dataComUltimo: string | null; rendimentosPorCota12Meses: number;
  dyMensal: number | null; dy12Meses: number | null; yieldOnCost12Meses: number | null; yieldOnCostMensal: number | null;
  rendaMensalEstimada: number; proventos12Meses: number; proventosTotal: number; recebidoExtrato12Meses: number;
  pesoCarteira: number; fonteCotacao: string | null; precoManual: boolean;
}

export interface LoteFii {
  operacaoId: number; ativoId: number; ticker: string; data: string; quantidade: number; precoPago: number;
  valorPago: number; precoAtual: number | null; valorAtual: number | null; variacaoPercentual: number | null;
  dyNaCompra: number | null; yieldOnCost12Meses: number | null; dyAtual12Meses: number | null;
  rendaMensalLote: number; proventosRecebidosLote: number; origem: OrigemOperacao;
}

export interface ProventoMes { ano: number; mes: number; total: number; recebidoExtrato: number; porAtivo: { ticker: string; valor: number }[] }

export interface ResumoFiis {
  valorMercado: number; custo: number; valorizacao: number; valorizacaoPercentual: number | null;
  renda12Meses: number; rendaMediaMensal: number; rendaMensalEstimada: number; dy12Meses: number | null;
  yieldOnCost12Meses: number | null; recebidoExtrato12Meses: number; rentabilidadeTotal: number | null;
  rentabilidadeSelic: number | null; rentabilidadeIfix: number | null; quantidadeFiis: number;
}

export interface PainelFiis {
  resumo: ResumoFiis; fiis: Fii[]; lotes: LoteFii[]; proventos: ProventoMes[];
  evolucao: { ano: number; mes: number; aplicado: number; valorMercado: number | null }[];
  rentabilidade: { ano: number; mes: number; carteira: number; selic: number; ifix: number | null }[];
  segmentos: { nome: string; valor: number }[];
  historicos: { ticker: string; pontos: { ano: number; mes: number; fechamento: number }[] }[];
  segmentosDisponiveis: string[];
}

export interface SugestaoExtrato {
  transacaoId: number; data: string; descricao: string; valor: number; contaNome: string; ticker: string;
  tipo: TipoOperacao; quantidade: number | null; precoUnitario: number | null; fechamentoNoDia: number | null;
  origem: string | null; jaImportada: boolean; possivelDuplicata: boolean;
}

export interface ImportarExtratoItem {
  transacaoId: number; ticker: string; tipo: TipoOperacao; quantidade: number; precoUnitario: number; taxas: number;
}
