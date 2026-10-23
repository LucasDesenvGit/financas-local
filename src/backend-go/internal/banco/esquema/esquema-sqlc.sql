-- GERADO por `go generate ./...` a partir do banco/01-estrutura.sql e do scripts/geral/2-criar-banco.sh.
-- Não edite: é só para o sqlc (veja internal/banco/esquema/esquema.go). O esquema de verdade continua no banco/.

CREATE TABLE "Usuario" (
    "Id" INTEGER NOT NULL,
    "Login" TEXT NOT NULL,
    "SenhaHash" TEXT NOT NULL,
    "CarimboSeguranca" TEXT NOT NULL,
    "TentativasFalhas" INTEGER NOT NULL,
    "BloqueadoAte" TEXT,
    "UltimoLoginEm" TEXT,
    "CriadoEm" TEXT NOT NULL
);

CREATE TABLE "AuditoriaAcesso" (
    "Id" INTEGER NOT NULL,
    "DataHora" TEXT NOT NULL,
    "Evento" TEXT NOT NULL,
    "Login" TEXT,
    "Ip" TEXT,
    "UserAgent" TEXT
);

CREATE TABLE "Configuracao" (
    "Chave" TEXT NOT NULL,
    "Valor" TEXT,
    "Protegido" INTEGER NOT NULL,
    "AtualizadoEm" TEXT NOT NULL
);

CREATE TABLE "ConexaoBancaria" (
    "Id" INTEGER NOT NULL,
    "PluggyItemId" TEXT NOT NULL,
    "NomeInstituicao" TEXT,
    "CorPrimaria" TEXT,
    "Status" TEXT,
    "StatusExecucao" TEXT,
    "UltimaAtualizacaoInstituicao" TEXT,
    "ConsentimentoExpiraEm" TEXT,
    "UltimaSincronizacaoEm" TEXT,
    "UltimoErro" TEXT,
    "Ativa" INTEGER NOT NULL,
    "CriadoEm" TEXT NOT NULL
);

CREATE TABLE "SincronizacaoLog" (
    "Id" INTEGER NOT NULL,
    "ConexaoBancariaId" INTEGER,
    "Origem" TEXT NOT NULL,
    "IniciadoEm" TEXT NOT NULL,
    "FinalizadoEm" TEXT,
    "Sucesso" INTEGER NOT NULL,
    "TransacoesNovas" INTEGER NOT NULL,
    "TransacoesAtualizadas" INTEGER NOT NULL,
    "Mensagem" TEXT
);

CREATE TABLE "Categoria" (
    "Id" INTEGER NOT NULL,
    "Nome" TEXT NOT NULL,
    "Tipo" INTEGER NOT NULL,
    "Cor" TEXT NOT NULL,
    "Icone" TEXT,
    "CategoriaPaiId" INTEGER,
    "Ativa" INTEGER NOT NULL,
    "Natureza" INTEGER
);

CREATE TABLE "CategoriaPluggy" (
    "PluggyCategoriaId" TEXT NOT NULL,
    "Descricao" TEXT NOT NULL,
    "PaiPluggyId" TEXT,
    "PaiDescricao" TEXT,
    "CategoriaId" INTEGER
);

CREATE TABLE "RegraCategorizacao" (
    "Id" INTEGER NOT NULL,
    "Texto" TEXT NOT NULL,
    "TipoCorrespondencia" INTEGER NOT NULL,
    "CategoriaId" INTEGER NOT NULL,
    "Prioridade" INTEGER NOT NULL,
    "Ativa" INTEGER NOT NULL,
    "CriadoEm" TEXT NOT NULL
);

CREATE TABLE "Conta" (
    "Id" INTEGER NOT NULL,
    "ConexaoBancariaId" INTEGER,
    "PluggyAccountId" TEXT,
    "Nome" TEXT NOT NULL,
    "Tipo" INTEGER NOT NULL,
    "Origem" INTEGER NOT NULL,
    "Numero" TEXT,
    "Moeda" TEXT NOT NULL,
    "SaldoInicial" TEXT NOT NULL,
    "Saldo" TEXT NOT NULL,
    "LimiteCredito" TEXT,
    "LimiteDisponivel" TEXT,
    "DiaFechamento" INTEGER,
    "DiaVencimento" INTEGER,
    "Cor" TEXT NOT NULL,
    "IncluirNoSaldo" INTEGER NOT NULL,
    "Arquivada" INTEGER NOT NULL,
    "Banco" TEXT,
    "ContaVinculadaId" INTEGER,
    "CriadoEm" TEXT NOT NULL,
    "AtualizadoEm" TEXT NOT NULL,
    "SaldoReservado" TEXT,
    "ReservasInformadas" TEXT,
    "LimiteAjustadoNoApp" INTEGER,
    "LimiteDiferencaAvaliada" TEXT
);

CREATE TABLE "FaturaCartao" (
    "Id" INTEGER NOT NULL,
    "ContaId" INTEGER NOT NULL,
    "PluggyBillId" TEXT,
    "DataVencimento" TEXT NOT NULL,
    "DataFechamento" TEXT,
    "ValorTotal" TEXT NOT NULL,
    "ValorMinimo" TEXT,
    "ValorPago" TEXT,
    "AtualizadoEm" TEXT NOT NULL
);

CREATE TABLE "Transacao" (
    "Id" INTEGER NOT NULL,
    "ContaId" INTEGER NOT NULL,
    "PluggyTransactionId" TEXT,
    "Data" TEXT NOT NULL,
    "Descricao" TEXT NOT NULL,
    "DescricaoOriginal" TEXT,
    "Estabelecimento" TEXT,
    "Valor" TEXT NOT NULL,
    "Tipo" INTEGER NOT NULL,
    "Status" INTEGER NOT NULL,
    "CategoriaId" INTEGER,
    "CategoriaManual" INTEGER NOT NULL,
    "PluggyCategoriaId" TEXT,
    "FaturaCartaoId" INTEGER,
    "ParcelaNumero" INTEGER,
    "ParcelaTotal" INTEGER,
    "IgnorarRelatorios" INTEGER NOT NULL,
    "Emergencia" INTEGER NOT NULL,
    "DescricaoRevisada" INTEGER NOT NULL,
    "MeioPagamento" TEXT,
    "MesmaTitularidade" INTEGER,
    "RegraDescricaoId" INTEGER,
    "DescricaoManual" INTEGER NOT NULL,
    "TransferenciaGrupo" TEXT,
    "Observacao" TEXT,
    "CriadoEm" TEXT NOT NULL,
    "AtualizadoEm" TEXT NOT NULL
);

CREATE TABLE "Investimento" (
    "Id" INTEGER NOT NULL,
    "ConexaoBancariaId" INTEGER NOT NULL,
    "PluggyInvestmentId" TEXT NOT NULL,
    "Nome" TEXT NOT NULL,
    "Tipo" TEXT NOT NULL,
    "Subtipo" TEXT,
    "Codigo" TEXT,
    "Emissor" TEXT,
    "Moeda" TEXT NOT NULL,
    "Saldo" TEXT NOT NULL,
    "ValorAplicado" TEXT,
    "Rendimento" TEXT,
    "Quantidade" TEXT,
    "TaxaAnual" TEXT,
    "DataVencimento" TEXT,
    "DataAplicacao" TEXT,
    "Status" TEXT,
    "AtualizadoEm" TEXT NOT NULL,
    "NomePersonalizado" TEXT
);

CREATE TABLE "Emprestimo" (
    "Id" INTEGER NOT NULL,
    "ConexaoBancariaId" INTEGER NOT NULL,
    "PluggyLoanId" TEXT NOT NULL,
    "Natureza" INTEGER NOT NULL,
    "Modalidade" TEXT,
    "Produto" TEXT NOT NULL,
    "NumeroContrato" TEXT,
    "Moeda" TEXT NOT NULL,
    "ValorContratado" TEXT NOT NULL,
    "SaldoDevedor" TEXT NOT NULL,
    "ValorParcela" TEXT,
    "ParcelasTotal" INTEGER,
    "ParcelasPagas" INTEGER,
    "ParcelasAVencer" INTEGER,
    "ParcelasEmAtraso" INTEGER,
    "Periodicidade" TEXT,
    "SistemaAmortizacao" TEXT,
    "TaxaJuros" TEXT,
    "PeriodicidadeTaxa" TEXT,
    "Indexador" TEXT,
    "Cet" TEXT,
    "DataContratacao" TEXT,
    "DataPrimeiraParcela" TEXT,
    "DataVencimentoFinal" TEXT,
    "AtualizadoEm" TEXT NOT NULL
);

CREATE TABLE "PagamentoEmprestimo" (
    "Id" INTEGER NOT NULL,
    "EmprestimoId" INTEGER NOT NULL,
    "Data" TEXT NOT NULL,
    "Valor" TEXT NOT NULL,
    "Extraordinario" INTEGER NOT NULL
);

CREATE TABLE "Orcamento" (
    "Id" INTEGER NOT NULL,
    "CategoriaId" INTEGER NOT NULL,
    "Ano" INTEGER NOT NULL,
    "Mes" INTEGER NOT NULL,
    "Valor" TEXT NOT NULL
);

CREATE TABLE "Meta" (
    "Id" INTEGER NOT NULL,
    "Nome" TEXT NOT NULL,
    "ValorAlvo" TEXT NOT NULL,
    "ValorAtual" TEXT NOT NULL,
    "DataAlvo" TEXT,
    "ContaId" INTEGER,
    "Cor" TEXT NOT NULL,
    "Concluida" INTEGER NOT NULL,
    "ReservaNome" TEXT,
    "Prioridade" INTEGER NOT NULL,
    "Icone" TEXT,
    "GuardarPorMes" TEXT,
    "CriadoEm" TEXT NOT NULL
);

CREATE TABLE "ContaAgendada" (
    "Id" INTEGER NOT NULL,
    "Descricao" TEXT NOT NULL,
    "Valor" TEXT NOT NULL,
    "ProximoVencimento" TEXT NOT NULL,
    "Recorrencia" INTEGER NOT NULL,
    "CategoriaId" INTEGER,
    "ContaId" INTEGER,
    "Ativa" INTEGER NOT NULL,
    "UltimoPagamentoEm" TEXT,
    "DataLimite" TEXT,
    "ChaveExtrato" TEXT,
    "CriadoEm" TEXT NOT NULL,
    "FormaCobranca" INTEGER NOT NULL,
    "TextoCobranca" TEXT,
    "Observacao" TEXT
);

CREATE TABLE "Ativo" (
    "Id" INTEGER NOT NULL,
    "Ticker" TEXT NOT NULL,
    "Nome" TEXT,
    "Tipo" INTEGER NOT NULL,
    "Moeda" TEXT NOT NULL,
    "PrecoAtual" TEXT,
    "PrecoAnterior" TEXT,
    "PrecoAtualizadoEm" TEXT,
    "PrecoManual" INTEGER NOT NULL,
    "FonteCotacao" TEXT,
    "Segmento" TEXT,
    "CriadoEm" TEXT NOT NULL
);

CREATE TABLE "OperacaoAtivo" (
    "Id" INTEGER NOT NULL,
    "AtivoId" INTEGER NOT NULL,
    "Tipo" INTEGER NOT NULL,
    "Data" TEXT NOT NULL,
    "Quantidade" TEXT NOT NULL,
    "PrecoUnitario" TEXT NOT NULL,
    "Taxas" TEXT NOT NULL,
    "Cambio" TEXT NOT NULL,
    "Observacao" TEXT,
    "Origem" INTEGER NOT NULL,
    "PluggyTransacaoId" TEXT,
    "TransacaoId" INTEGER,
    "CriadoEm" TEXT NOT NULL
);

CREATE TABLE "ProventoAtivo" (
    "Id" INTEGER NOT NULL,
    "AtivoId" INTEGER NOT NULL,
    "DataCom" TEXT NOT NULL,
    "ValorPorCota" TEXT NOT NULL
);

CREATE TABLE "TaxaSelicDiaria" (
    "Data" TEXT NOT NULL,
    "Taxa" TEXT NOT NULL
);

CREATE TABLE "PrecoMensalAtivo" (
    "AtivoId" INTEGER NOT NULL,
    "Mes" TEXT NOT NULL,
    "Fechamento" TEXT NOT NULL
);

CREATE TABLE "IndiceMensal" (
    "Codigo" TEXT NOT NULL,
    "Mes" TEXT NOT NULL,
    "Fechamento" TEXT NOT NULL
);

CREATE TABLE "CotacaoMoeda" (
    "Moeda" TEXT NOT NULL,
    "Data" TEXT NOT NULL,
    "Compra" TEXT NOT NULL,
    "Venda" TEXT NOT NULL,
    "AtualizadoEm" TEXT NOT NULL
);

CREATE TABLE "ReservaConferencia" (
    "Id" INTEGER NOT NULL,
    "ContaId" INTEGER NOT NULL,
    "Nome" TEXT NOT NULL,
    "SaldoConferido" TEXT NOT NULL,
    "DataConferencia" TEXT NOT NULL,
    "AtualizadoEm" TEXT NOT NULL
);

CREATE TABLE "ErroSistema" (
    "Id" INTEGER NOT NULL,
    "PrimeiraEm" TEXT NOT NULL,
    "UltimaEm" TEXT NOT NULL,
    "Ocorrencias" INTEGER NOT NULL,
    "Nivel" INTEGER NOT NULL,
    "Origem" TEXT NOT NULL,
    "Categoria" TEXT NOT NULL,
    "Mensagem" TEXT NOT NULL,
    "Detalhe" TEXT,
    "Rota" TEXT,
    "Usuario" TEXT,
    "Assinatura" TEXT NOT NULL,
    "Resolvido" INTEGER NOT NULL,
    "ResolvidoEm" TEXT
);

CREATE TABLE "RegraDescricao" (
    "Id" INTEGER NOT NULL,
    "Texto" TEXT NOT NULL,
    "TipoCorrespondencia" INTEGER NOT NULL,
    "NovaDescricao" TEXT NOT NULL,
    "ValorMinimo" TEXT,
    "ValorMaximo" TEXT,
    "Ativa" INTEGER NOT NULL,
    "CriadoEm" TEXT NOT NULL
);

CREATE TABLE "HistoricoInvestimento" (
    "Id" INTEGER NOT NULL,
    "PluggyInvestmentId" TEXT NOT NULL,
    "ConexaoBancariaId" INTEGER NOT NULL,
    "Nome" TEXT NOT NULL,
    "Tipo" TEXT NOT NULL,
    "Data" TEXT NOT NULL,
    "Saldo" TEXT NOT NULL,
    "ValorAplicado" TEXT,
    "AtualizadoEm" TEXT NOT NULL
);

CREATE TABLE "CotacaoCripto" (
    "Moeda" TEXT NOT NULL,
    "DataHora" TEXT NOT NULL,
    "Preco" TEXT NOT NULL,
    "AtualizadoEm" TEXT NOT NULL
);

CREATE TABLE "MetaAporte" (
    "Id" INTEGER NOT NULL,
    "MetaId" INTEGER NOT NULL,
    "Data" TEXT NOT NULL,
    "Valor" TEXT NOT NULL,
    "CriadoEm" TEXT NOT NULL
);
