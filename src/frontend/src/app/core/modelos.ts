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

export interface EstadoAutenticacao { configurado: boolean; autenticado: boolean; login: string | null; versao?: string; }
export interface Mensagem { mensagem: string; }

export interface Conta {
  id: number; nome: string; tipo: TipoConta; origem: OrigemConta; numero: string | null; moeda: string;
  saldo: number; saldoInicial: number; limiteCredito: number | null; limiteDisponivel: number | null;
  diaFechamento: number | null; diaVencimento: number | null; cor: string; incluirNoSaldo: boolean; arquivada: boolean;
  conexaoBancariaId: number | null; nomeInstituicao: string | null; atualizadoEm: string;
  /** ADR 0024: banco que vale (escolhido, do cartão herdado da conta ou detectado pelo nome); null = não identificado. */
  banco: string | null;
  /** O banco foi escolhido pelo usuário (e não detectado). */
  bancoDefinido: boolean;
  /** Cartão: a conta a que ele pertence (de onde paga a fatura). */
  contaVinculadaId: number | null;
  /** Cartão: o que foi lançado no mês atual e a fatura que vence nele (mesma conta do Painel). */
  faturaMes: FaturaMes | null;
}

export interface SalvarConta {
  nome: string; tipo: TipoConta; numero: string | null; saldoInicial: number; limiteCredito: number | null;
  diaFechamento: number | null; diaVencimento: number | null; cor: string; incluirNoSaldo: boolean; arquivada: boolean;
  /** Código do banco (null = detectar pelo nome; 'outro' = sem banco). */
  banco: string | null;
  contaVinculadaId: number | null;
}

export type SituacaoFatura = 'paga' | 'fechada' | 'aberta';

export interface Fatura {
  id: number; dataVencimento: string; dataFechamento: string | null; valorTotal: number;
  valorMinimo: number | null; valorPago: number | null; quantidadeTransacoes: number; situacao: SituacaoFatura;
}

/** Banco que a tela de Contas sabe identificar (ADR 0024). */
export interface Banco { codigo: string; nome: string; }

export interface Transacao {
  id: number; contaId: number; contaNome: string; openFinance: boolean; data: string; descricao: string;
  descricaoOriginal: string | null; estabelecimento: string | null; valor: number; tipo: TipoTransacao;
  status: StatusTransacao; categoriaId: number | null; categoriaNome: string | null; categoriaCor: string | null;
  parcelaNumero: number | null; parcelaTotal: number | null; ignorarRelatorios: boolean; observacao: string | null;
  emergencia: boolean;
  /** #111: como o dinheiro entrou ou saiu (calculado pela API). */
  tipoLancamento: TipoLancamento;
  /** #123: o nome mostrado veio de uma regra de descrição (null = do banco ou escolhido à mão). */
  regraDescricaoId?: number | null;
}

/** #123: prévia de "usar também nas outras" (quantas transações de nome automático batem com o texto). */
export interface PreviaRenomear {
  descricaoOriginal: string | null; textoSugerido: string; quantidade: number; exemplos: string[];
}

/** #123: novo nome de uma transação: só nesta, de volta ao do banco, ou como regra para as outras e as próximas. */
export interface RenomearTransacao {
  descricao: string | null; voltarOriginal: boolean; aplicarNasOutras: boolean;
  tipoCorrespondencia: TipoCorrespondencia; texto: string | null;
}

export interface ResultadoRenomear { alteradas: number; regraId: number | null; desfazer: string | null; }

/** #123: o mesmo nome em várias transações; criarRegras = uma regra "é igual a" por descrição do banco. */
export interface RenomearEmLote { ids: number[]; descricao: string; criarRegras: boolean; }

export interface ResultadoRenomearEmLote {
  alteradas: number; regrasCriadas: number; regrasAlteradas: number; desfazer: string | null;
}

export interface PreviaRegraDescricao { quantidade: number; exemplos: string[]; }

/** #123 (ADR 0030): "quando a descrição do banco for/contiver texto, mostrar novaDescricao". */
export interface RegraDescricao {
  id: number; texto: string; tipoCorrespondencia: TipoCorrespondencia; novaDescricao: string; ativa: boolean;
  transacoes: number;
}

export interface SalvarRegraDescricao {
  texto: string; tipoCorrespondencia: TipoCorrespondencia; novaDescricao: string; ativa: boolean;
}

/** #111 (ADR 0025): os mesmos números do enum da API. */
export enum TipoLancamento {
  Outro = 0, CompraCartao = 1, EstornoCartao = 2, PagamentoFatura = 3, PixEnviado = 4, PixRecebido = 5,
  TransferenciaEnviada = 6, TransferenciaRecebida = 7, EntreContas = 8, Boleto = 9, CompraDebito = 10, Saque = 11,
  Deposito = 12, TarifaJuros = 13, Rendimento = 14, AplicacaoResgate = 15, Salario = 16, Credito = 17, Dinheiro = 18
}

export interface FiltroTransacoes {
  inicio?: string | null; fim?: string | null; contaId?: number | null; categoriaId?: number | null;
  /** Várias categorias (cada uma inclui as subcategorias); com semCategoria, soma também as sem categoria. */
  categoriaIds?: number[] | null;
  semCategoria?: boolean; somenteEmergencias?: boolean; tipo?: TipoTransacao | null; busca?: string | null; pagina?: number; tamanhoPagina?: number;
  /** #9: só as de descrição pouco informativa ainda não revisadas. */
  poucoInformativa?: boolean;
  /** #111: grupo do filtro "Lançamento" (pix, cartao, fatura, entre-contas…). */
  lancamento?: string | null;
  /** Natureza do gasto (50/30/20), da categoria ou da categoria pai; com semNatureza, soma as sem classificação. */
  naturezas?: NaturezaGasto[] | null;
  semNatureza?: boolean;
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

/** #87 — categoria de uma transação no modo edição (automatica = voltar para as regras). */
export interface AlteracaoCategoria { id: number; categoriaId: number | null; automatica: boolean; }

/** Por itens (cada um com a sua categoria) ou todas as transações do filtro (com categoriaId/automatica). */
export interface AlterarCategoriasEmLote {
  itens?: AlteracaoCategoria[]; filtro?: FiltroTransacoes; categoriaId?: number | null; automatica?: boolean;
}

/** desfazer: chave para restaurar (vale por alguns minutos; null quando nada mudou). */
export interface ResultadoCategoriasEmLote { alteradas: number; desfazer: string | null; }

/** #39 — uso do limite de um cartão (média das últimas faturas fechadas ÷ limite). */
export interface CartaoUsoLimite { contaId: number; nome: string; limite: number; faturaMedia: number; percentual: number }

/** Sugestão para uma categoria principal; sugerido = null quando não há dados suficientes. */
export interface ItemSugestaoOrcamento {
  categoriaId: number; nome: string; cor: string; natureza: NaturezaGasto | null; mediana: number; media: number;
  ultimoMes: number; tendencia: 'alta' | 'queda' | 'estavel' | null; mesesComGasto: number; pagoNoCartaoPercentual: number;
  sugerido: number | null; atual: number | null; selecionado: boolean; motivos: string[];
}

export interface SugestaoOrcamento {
  ano: number; mes: number; mesesBase: number; mesesRenda: number; rendaMedia: number;
  parcelasCartao: number; parcelasFinanciamentos: number; parcelasEmprestimos: number; percentualPoupanca: number;
  metaPoupanca: number; disponivel: number; totalSugerido: number; faltaParaCaber: number | null;
  usoLimiteCartoes: number | null; nivelCartao: 'ok' | 'atencao' | 'alerta' | null; limiteAtencao: number; limiteAlerta: number;
  cartoes: CartaoUsoLimite[]; avisos: string[]; itens: ItemSugestaoOrcamento[];
}

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
  /** #89: cofrinho (reserva) que guarda o dinheiro da meta; com ele, o progresso usa o saldo do cofrinho. */
  reservaNome: string | null; saldoReserva: number | null; reservaConferir: boolean; reservaNaoEncontrada: boolean;
  /** #90: ordem de prioridade (1 = primeira); 0 = ainda sem ordem. */
  prioridade: number;
}

/**
 * #149 — um mês do caixa: receitas e despesas das contas de dinheiro, faturas de cartão que vencem no mês (com a parte
 * que é parcela) e a sobra = receitas − despesas − faturas.
 */
export interface MesCaixa { ano: number; mes: number; receitas: number; despesas: number; faturas: number; parcelas: number; sobra: number }

/**
 * #90/#149 — o que o plano das metas precisa (a divisão entre as metas é feita na tela, em `metas-plano.ts`).
 * `meses` começa no mês atual. `saldoFimDoMes`: saldo previsto nas contas no fim do mês, já pagas as faturas que vencem
 * nele. `melhorMesRecente`: a maior sobra de verdade entre os meses completos considerados.
 */
export interface PlanoMetas {
  disponivel: boolean; mesesConsiderados: number; ano: number; mes: number; margemPercentual: number;
  saldoContas: number; faturaAPagar: number; saldoFimDoMes: number; melhorMesRecente: number; meses: MesCaixa[];
}

export interface SalvarMeta {
  nome: string; valorAlvo: number; valorAtual: number; dataAlvo: string | null; contaId: number | null;
  cor: string; concluida: boolean; reservaNome: string | null;
}

export interface ContaAgendada {
  id: number; descricao: string; valor: number; proximoVencimento: string; recorrencia: Recorrencia;
  categoriaId: number | null; categoriaNome: string | null; contaId: number | null; contaNome: string | null;
  ativa: boolean; ultimoPagamentoEm: string | null; diasParaVencer: number;
  /** #69: até quando se repete (null = sem fim); pagamentos que faltam até lá, contando o próximo. */
  dataLimite: string | null; pagamentosRestantes: number | null;
  /** Inativa por ter passado da data limite ao pagar. */
  encerrada: boolean;
}

export interface SalvarContaAgendada {
  descricao: string; valor: number; proximoVencimento: string; recorrencia: Recorrencia;
  categoriaId: number | null; contaId: number | null; ativa: boolean;
  /** #69: último vencimento que ainda se repete (null = sem fim). */
  dataLimite: string | null;
}

/** Pagamento mensal achado no extrato que ainda não está em Contas a pagar. */
export interface SugestaoContaAgendada {
  chave: string; descricao: string; valor: number; valorVariavel: boolean; valorMinimo: number; valorMaximo: number;
  proximoVencimento: string; diaHabitual: number; meses: number; pagamentos: number;
  ultimoPagamento: string; primeiroPagamento: string;
  categoriaId: number | null; categoriaNome: string | null; contaId: number; contaNome: string; termoBusca: string;
  /** Vencimento esperado que já passou sem o pagamento aparecer no extrato (null quando não há). */
  vencimentoSemPagamento: string | null;
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
  diasProximosVencimentos: number;
  /** UX-12/#144: a fatura de cada cartão que fecha no mês escolhido. */
  faturasDoMes: FaturaMes[] | null; totalFaturasDoMes: number;
  /** #9: transações recentes com descrição pouco informativa ainda não revisadas (aviso discreto). */
  poucoInformativas: number; poucoInformativasDesde: string | null; exemploPoucoInformativa: string | null;
}

export interface FaturaMes {
  /** #144: <c>valorMes</c> é o total do banco na fatura fechada; na aberta, a soma das transações do ciclo. */
  contaId: number; cartao: string; valorMes: number; parcelasMes: number;
  /** Fatura que fecha no mês (<c>faturaId</c> só depois de importada; vencimento estimado enquanto não). */
  faturaId: number | null; vencimento: string | null; situacao: SituacaoFatura | null;
  /** #144: dia em que a fatura fecha; null = cartão sem fechamento conhecido (valor é o do mês do calendário). */
  fechamento?: string | null;
  /** ADR 0024: banco do cartão, para o selo. */
  banco?: string | null;
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
  clientId: string; clientSecret: string | null;
  /** Opcionais: a aba Parametrização cuida deles. */
  intervaloHoras?: number; diasRetroativos?: number; habilitada?: boolean;
}

export enum TipoParametro { Inteiro = 1, Booleano = 2 }
/** Booleanos usam 0/1 em valor e padrao. */
export interface Parametro {
  chave: string; grupo: string; nome: string; descricao: string; tipo: TipoParametro;
  valor: number; padrao: number; minimo: number | null; maximo: number | null; unidade: string | null;
}
export interface InfoSistema {
  versao: string; dotNet: string; sistemaOperacional: string; iniciadoEm: string; caminhoBanco: string;
  tamanhoBancoBytes: number | null; pastaChaves: string; contas: number; transacoes: number; conexoesAtivas: number;
  ultimaSincronizacao: string | null;
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

/** Status leve da sincronização para a barra superior (UX-8). */
export interface StatusSincronizacao {
  ultimaSincronizacao: string | null; emExecucao: boolean; conexoesAtivas: number;
  conexoesComErro: number; consentimentosVencendo: number;
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
  openFinanceNaoRegistrados: number; moedasSemCotacao: string[]; reservas: Reserva[]; totalReservas: number;
  /** Ativos com cotas diferentes da posição da corretora ("Conferir investimentos" mostra o que fazer). */
  cotasDiferentes: string[];
}

/**
 * Reserva ("cofrinho") que a instituição não informa pelo Open Finance, estimada pelos lançamentos
 * "Dinheiro reservado/retirado {nome}" da conta. Com conferência: saldo conferido + movimentos depois da data.
 */
export interface Reserva {
  contaId: number; nomeConta: string; nome: string; saldo: number; guardado: number; retirado: number;
  movimentos: number; primeiroMovimento: string | null; ultimoMovimento: string | null;
  saldoConferido: number | null; dataConferencia: string | null;
  /** Sem conferência e o 1º lançamento está no início da janela importada: provavelmente já havia saldo antes. */
  podeTerSaldoAnterior: boolean;
  /** Conferida há mais de 30 dias. */
  conferenciaAntiga: boolean;
  /** #168: o valor veio da instituição pelo Open Finance (não é estimado nem precisa de conferência). */
  informadoPeloBanco?: boolean;
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

/* ----------------------------- Conferir investimentos (ADR 0027) ----------------------------- */
export interface ConferenciaInvestimentos {
  tipos: TipoParaCorrigir[]; cotas: CotasDiferentes[]; comprasSemOperacao: CompraSemOperacao[];
  trocos: CreditoDaCorretora[]; rendimentos: RendimentoComoInvestimento[]; categoriaDoTroco: { id: number; nome: string } | null;
}
/** Ativo cadastrado como ação ou "outro" que é FII (motivo: o nome do fundo ou o emissor da corretora). */
export interface TipoParaCorrigir { ativoId: number; ticker: string; nome: string | null; tipoAtual: TipoAtivo; motivo: string; }
/** Cotas na corretora × na carteira; quando faltam, o débito de compra sugerido e o preço (débito − troco ÷ cotas). */
export interface CotasDiferentes {
  ativoId: number; ticker: string; naCorretora: number; naCarteira: number; posicaoEm: string;
  debitoSugeridoId: number | null; precoSugerido: number | null; fechamentoNoDia: number | null;
}
/** Débito de compra de renda variável que nenhuma operação explica (troco: o que voltou dele). */
export interface CompraSemOperacao { transacaoId: number; data: string; valor: number; troco: number; conta: string; descricao: string; }
/** Troco de compra (compra*) ou valor de venda (venda*) contando como receita. */
export interface CreditoDaCorretora {
  transacaoId: number; data: string; valor: number; descricao: string; conta: string; categoria: string | null;
  compraData: string | null; compraValor: number | null; vendaTicker: string | null; vendaQuantidade: number | null;
  vendaData: string | null;
}
/** Provento numa categoria de transferência; volta para a categoria automática. */
export interface RendimentoComoInvestimento {
  transacaoId: number; data: string; valor: number; descricao: string; conta: string; categoria: string | null;
  categoriaAutomatica: string;
}
export interface CorrigirInvestimentos {
  ativosParaFii: number[]; compras: ImportarExtratoItem[]; trocos: number[]; rendimentos: number[];
}
export interface ResultadoCorrecaoInvestimentos {
  tipos: number; compras: number; trocos: number; rendimentos: number; desfazer: string | null;
}

/* ----------------------------- Erros do sistema (#11) ----------------------------- */
/** Os mesmos números do LogLevel do .NET. */
export enum NivelErro { Aviso = 3, Erro = 4, Critico = 5 }

export const NOMES_NIVEL_ERRO: Record<NivelErro, string> = {
  [NivelErro.Aviso]: 'Aviso',
  [NivelErro.Erro]: 'Erro',
  [NivelErro.Critico]: 'Crítico'
};

export type SituacaoErros = 'abertos' | 'resolvidos' | 'todos';

export interface FiltroErros {
  situacao: SituacaoErros; origem: string | null; nivel: NivelErro | null; inicio: string | null; fim: string | null;
  pagina: number; tamanhoPagina: number;
}

/** Datas em UTC (com "Z"). */
export interface ErroSistema {
  id: number; primeiraEm: string; ultimaEm: string; ocorrencias: number; nivel: NivelErro; origem: string;
  mensagem: string; rota: string | null; usuario: string | null; resolvido: boolean; resolvidoEm: string | null;
}

export interface ErroSistemaDetalhe { id: number; categoria: string; detalhe: string | null; }

export interface PaginaErros {
  itens: ErroSistema[]; total: number; pagina: number; tamanhoPagina: number;
  /** Total em aberto, sem filtro. */
  abertos: number;
  origens: string[];
}

export interface ErroFrontEnd { mensagem: string; detalhe: string | null; rota: string | null; }
