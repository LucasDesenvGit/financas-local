/* =====================================================================
   FinancasLocal - Estrutura completa do banco de dados (SQLite)

   Baseline limpa: o banco trocou de motor (SQL Server -> SQLite) para que
   o projeto funcione em Windows e Linux sem depender de instância de SQL
   Server nem do IIS (a API agora sobe sozinha via Kestrel). Por isso este
   arquivo não tenta preservar o histórico de versões v4-v7 do script
   antigo (ALTER TABLE incremental) — todas as colunas que existiam no
   SQL Server já nascem aqui dentro do CREATE TABLE correspondente.

   O arquivo do banco (dados/sql/FinancasLocal.db) é criado automaticamente
   pelo SQLite na primeira conexão; não existe mais CREATE DATABASE,
   COLLATE, ALTER DATABASE nem o comando USE (SQLite não tem esses conceitos).

   Script idempotente: pode ser executado novamente sem perder dados
   (CREATE TABLE/INDEX IF NOT EXISTS).

   Importante: este script não precisa habilitar o enforcement de chaves
   estrangeiras — isso é responsabilidade de quem abre a conexão. O
   provider SQLite do EF Core já envia "PRAGMA foreign_keys = ON" em toda
   conexão por padrão; não remova esse comportamento da configuração do
   provider, senão os ON DELETE CASCADE/SET NULL abaixo deixam de valer.
   ===================================================================== */

/* ------------------------------------------------------------------ */
/* Segurança / acesso                                                  */
/* ------------------------------------------------------------------ */
CREATE TABLE IF NOT EXISTS Usuario
(
    Id                  INTEGER NOT NULL CONSTRAINT PK_Usuario PRIMARY KEY AUTOINCREMENT,
    Login               TEXT    NOT NULL,
    SenhaHash           TEXT    NOT NULL,
    -- Guid gerado pela API (Usuario.CarimboSeguranca = Guid.NewGuid()); sem DEFAULT no banco.
    CarimboSeguranca    TEXT    NOT NULL,
    TentativasFalhas    INTEGER NOT NULL CONSTRAINT DF_Usuario_Tentativas DEFAULT 0,
    BloqueadoAte        TEXT    NULL,
    UltimoLoginEm       TEXT    NULL,
    -- Preenchido pela API (Usuario.CriadoEm = DateTime.UtcNow); sem DEFAULT no banco.
    CriadoEm            TEXT    NOT NULL,
    CONSTRAINT UQ_Usuario_Login UNIQUE (Login)
);

CREATE TABLE IF NOT EXISTS AuditoriaAcesso
(
    Id          INTEGER NOT NULL CONSTRAINT PK_AuditoriaAcesso PRIMARY KEY AUTOINCREMENT,
    -- Preenchido pela API (AuditoriaAcesso.DataHora = DateTime.UtcNow); sem DEFAULT no banco.
    DataHora    TEXT    NOT NULL,
    Evento      TEXT    NOT NULL,
    Login       TEXT    NULL,
    Ip          TEXT    NULL,
    UserAgent   TEXT    NULL
);

CREATE TABLE IF NOT EXISTS Configuracao
(
    Chave        TEXT NOT NULL CONSTRAINT PK_Configuracao PRIMARY KEY,
    Valor        TEXT NULL,
    Protegido    INTEGER NOT NULL CONSTRAINT DF_Configuracao_Protegido DEFAULT 0,
    -- Preenchido pela API (Configuracao.AtualizadoEm = DateTime.UtcNow); sem DEFAULT no banco.
    AtualizadoEm TEXT NOT NULL
);

/* ------------------------------------------------------------------ */
/* Open Finance (Pluggy)                                               */
/* ------------------------------------------------------------------ */
CREATE TABLE IF NOT EXISTS ConexaoBancaria
(
    Id                           INTEGER NOT NULL CONSTRAINT PK_ConexaoBancaria PRIMARY KEY AUTOINCREMENT,
    PluggyItemId                 TEXT    NOT NULL,
    NomeInstituicao              TEXT    NULL,
    CorPrimaria                  TEXT    NULL,
    Status                       TEXT    NULL,
    StatusExecucao               TEXT    NULL,
    UltimaAtualizacaoInstituicao TEXT    NULL,
    ConsentimentoExpiraEm        TEXT    NULL,
    UltimaSincronizacaoEm        TEXT    NULL,
    UltimoErro                   TEXT    NULL,
    Ativa                        INTEGER NOT NULL CONSTRAINT DF_ConexaoBancaria_Ativa DEFAULT 1,
    -- Preenchido pela API (ConexaoBancaria.CriadoEm = DateTime.UtcNow); sem DEFAULT no banco.
    CriadoEm                     TEXT    NOT NULL,
    CONSTRAINT UQ_ConexaoBancaria_PluggyItemId UNIQUE (PluggyItemId)
);

CREATE TABLE IF NOT EXISTS SincronizacaoLog
(
    Id                      INTEGER NOT NULL CONSTRAINT PK_SincronizacaoLog PRIMARY KEY AUTOINCREMENT,
    ConexaoBancariaId       INTEGER NULL,
    Origem                  TEXT    NOT NULL,
    IniciadoEm              TEXT    NOT NULL,
    FinalizadoEm            TEXT    NULL,
    Sucesso                 INTEGER NOT NULL CONSTRAINT DF_SincronizacaoLog_Sucesso DEFAULT 0,
    TransacoesNovas         INTEGER NOT NULL CONSTRAINT DF_SincronizacaoLog_Novas DEFAULT 0,
    TransacoesAtualizadas   INTEGER NOT NULL CONSTRAINT DF_SincronizacaoLog_Atualizadas DEFAULT 0,
    Mensagem                TEXT    NULL,
    CONSTRAINT FK_SincronizacaoLog_ConexaoBancaria FOREIGN KEY (ConexaoBancariaId)
        REFERENCES ConexaoBancaria (Id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS IX_SincronizacaoLog_IniciadoEm ON SincronizacaoLog (IniciadoEm DESC);

/* ------------------------------------------------------------------ */
/* Categorias                                                          */
/* Tipo: 1 = Despesa, 2 = Receita, 3 = Transferência                   */
/* Natureza (só para despesas): 1 = Essencial, 2 = Estilo de vida.     */
/* Usada na regra 50/30/20.                                            */
/* ------------------------------------------------------------------ */
CREATE TABLE IF NOT EXISTS Categoria
(
    Id              INTEGER NOT NULL CONSTRAINT PK_Categoria PRIMARY KEY AUTOINCREMENT,
    Nome            TEXT    NOT NULL,
    Tipo            INTEGER NOT NULL,
    Cor             TEXT    NOT NULL CONSTRAINT DF_Categoria_Cor DEFAULT '#64748b',
    Icone           TEXT    NULL,
    CategoriaPaiId  INTEGER NULL,
    Ativa           INTEGER NOT NULL CONSTRAINT DF_Categoria_Ativa DEFAULT 1,
    Natureza        INTEGER NULL CONSTRAINT CK_Categoria_Natureza CHECK (Natureza IN (1, 2)),
    CONSTRAINT UQ_Categoria_Nome UNIQUE (Nome),
    CONSTRAINT CK_Categoria_Tipo CHECK (Tipo IN (1, 2, 3)),
    CONSTRAINT FK_Categoria_CategoriaPai FOREIGN KEY (CategoriaPaiId) REFERENCES Categoria (Id)
);

CREATE TABLE IF NOT EXISTS CategoriaPluggy
(
    PluggyCategoriaId   TEXT    NOT NULL CONSTRAINT PK_CategoriaPluggy PRIMARY KEY,
    Descricao           TEXT    NOT NULL,
    PaiPluggyId         TEXT    NULL,
    PaiDescricao        TEXT    NULL,
    CategoriaId         INTEGER NULL,
    CONSTRAINT FK_CategoriaPluggy_Categoria FOREIGN KEY (CategoriaId) REFERENCES Categoria (Id) ON DELETE SET NULL
);

/* TipoCorrespondencia: 1 = Contém, 2 = Começa com, 3 = Igual */
CREATE TABLE IF NOT EXISTS RegraCategorizacao
(
    Id                  INTEGER NOT NULL CONSTRAINT PK_RegraCategorizacao PRIMARY KEY AUTOINCREMENT,
    Texto               TEXT    NOT NULL,
    TipoCorrespondencia INTEGER NOT NULL CONSTRAINT DF_Regra_TipoCorrespondencia DEFAULT 1,
    CategoriaId         INTEGER NOT NULL,
    Prioridade          INTEGER NOT NULL CONSTRAINT DF_Regra_Prioridade DEFAULT 100,
    Ativa               INTEGER NOT NULL CONSTRAINT DF_Regra_Ativa DEFAULT 1,
    -- Preenchido pela API (RegraCategorizacao.CriadoEm = DateTime.UtcNow); sem DEFAULT no banco.
    CriadoEm            TEXT    NOT NULL,
    CONSTRAINT CK_Regra_TipoCorrespondencia CHECK (TipoCorrespondencia IN (1, 2, 3)),
    CONSTRAINT FK_RegraCategorizacao_Categoria FOREIGN KEY (CategoriaId) REFERENCES Categoria (Id) ON DELETE CASCADE
);

/* ------------------------------------------------------------------ */
/* Contas                                                              */
/* Tipo: 1 Conta corrente, 2 Poupança, 3 Cartão de crédito,            */
/*       4 Investimento, 5 Dinheiro/Carteira, 6 Outro                  */
/* Origem: 1 Manual, 2 Open Finance (Pluggy)                           */
/* Valores monetários ficam em TEXT (não REAL/NUMERIC) para não perder */
/* precisão decimal; é o mapeamento padrão do EF Core SQLite p/ decimal*/
/* ------------------------------------------------------------------ */
CREATE TABLE IF NOT EXISTS Conta
(
    Id                  INTEGER NOT NULL CONSTRAINT PK_Conta PRIMARY KEY AUTOINCREMENT,
    ConexaoBancariaId   INTEGER NULL,
    PluggyAccountId     TEXT    NULL,
    Nome                TEXT    NOT NULL,
    Tipo                INTEGER NOT NULL,
    Origem              INTEGER NOT NULL,
    Numero              TEXT    NULL,
    Moeda               TEXT    NOT NULL CONSTRAINT DF_Conta_Moeda DEFAULT 'BRL',
    SaldoInicial        TEXT    NOT NULL CONSTRAINT DF_Conta_SaldoInicial DEFAULT '0',
    Saldo               TEXT    NOT NULL CONSTRAINT DF_Conta_Saldo DEFAULT '0',
    LimiteCredito       TEXT    NULL,
    LimiteDisponivel    TEXT    NULL,
    DiaFechamento       INTEGER NULL,
    DiaVencimento       INTEGER NULL,
    Cor                 TEXT    NOT NULL CONSTRAINT DF_Conta_Cor DEFAULT '#0f766e',
    IncluirNoSaldo      INTEGER NOT NULL CONSTRAINT DF_Conta_IncluirNoSaldo DEFAULT 1,
    Arquivada           INTEGER NOT NULL CONSTRAINT DF_Conta_Arquivada DEFAULT 0,
    -- ADR 0024: banco escolhido pelo usuário (código do BancoService; NULL = detectar pelo nome).
    Banco               TEXT    NULL,
    -- ADR 0024: cartão → conta a que pertence (de onde paga a fatura). Sem FK: a API limpa ao excluir a conta.
    ContaVinculadaId    INTEGER NULL,
    -- CriadoEm/AtualizadoEm preenchidos pela API (DateTime.UtcNow); sem DEFAULT no banco.
    CriadoEm            TEXT    NOT NULL,
    AtualizadoEm        TEXT    NOT NULL,
    CONSTRAINT CK_Conta_Tipo CHECK (Tipo BETWEEN 1 AND 6),
    CONSTRAINT CK_Conta_Origem CHECK (Origem IN (1, 2)),
    CONSTRAINT CK_Conta_DiaFechamento CHECK (DiaFechamento IS NULL OR DiaFechamento BETWEEN 1 AND 31),
    CONSTRAINT CK_Conta_DiaVencimento CHECK (DiaVencimento IS NULL OR DiaVencimento BETWEEN 1 AND 31),
    CONSTRAINT FK_Conta_ConexaoBancaria FOREIGN KEY (ConexaoBancariaId) REFERENCES ConexaoBancaria (Id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS UX_Conta_PluggyAccountId ON Conta (PluggyAccountId) WHERE PluggyAccountId IS NOT NULL;

CREATE TABLE IF NOT EXISTS FaturaCartao
(
    Id              INTEGER NOT NULL CONSTRAINT PK_FaturaCartao PRIMARY KEY AUTOINCREMENT,
    ContaId         INTEGER NOT NULL,
    PluggyBillId    TEXT    NULL,
    DataVencimento  TEXT    NOT NULL,
    DataFechamento  TEXT    NULL,
    ValorTotal      TEXT    NOT NULL,
    ValorMinimo     TEXT    NULL,
    ValorPago       TEXT    NULL,
    -- Preenchido pela API (FaturaCartao.AtualizadoEm = DateTime.UtcNow); sem DEFAULT no banco.
    AtualizadoEm    TEXT    NOT NULL,
    CONSTRAINT FK_FaturaCartao_Conta FOREIGN KEY (ContaId) REFERENCES Conta (Id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS UX_FaturaCartao_PluggyBillId ON FaturaCartao (PluggyBillId) WHERE PluggyBillId IS NOT NULL;
CREATE INDEX IF NOT EXISTS IX_FaturaCartao_Conta_Vencimento ON FaturaCartao (ContaId, DataVencimento DESC);

/* ------------------------------------------------------------------ */
/* Transações                                                          */
/* Valor: negativo = saída, positivo = entrada                         */
/* Tipo: 1 Despesa, 2 Receita, 3 Transferência                         */
/* Status: 1 Efetivada, 2 Pendente                                     */
/* ------------------------------------------------------------------ */
CREATE TABLE IF NOT EXISTS Transacao
(
    Id                  INTEGER NOT NULL CONSTRAINT PK_Transacao PRIMARY KEY AUTOINCREMENT,
    ContaId             INTEGER NOT NULL,
    PluggyTransactionId TEXT    NULL,
    Data                TEXT    NOT NULL,
    Descricao           TEXT    NOT NULL,
    DescricaoOriginal   TEXT    NULL,
    Estabelecimento     TEXT    NULL,
    Valor               TEXT    NOT NULL,
    Tipo                INTEGER NOT NULL,
    Status              INTEGER NOT NULL CONSTRAINT DF_Transacao_Status DEFAULT 1,
    CategoriaId         INTEGER NULL,
    CategoriaManual     INTEGER NOT NULL CONSTRAINT DF_Transacao_CategoriaManual DEFAULT 0,
    PluggyCategoriaId   TEXT    NULL,
    FaturaCartaoId      INTEGER NULL,
    ParcelaNumero       INTEGER NULL,
    ParcelaTotal        INTEGER NULL,
    IgnorarRelatorios   INTEGER NOT NULL CONSTRAINT DF_Transacao_Ignorar DEFAULT 0,
    -- Marcação de gasto de emergência (destacado nos relatórios e orçamentos).
    Emergencia          INTEGER NOT NULL CONSTRAINT DF_Transacao_Emergencia DEFAULT 0,
    -- #9: o usuário já olhou a descrição pouco informativa ("PIX ENVIADO") e ela sai do filtro.
    DescricaoRevisada   INTEGER NOT NULL CONSTRAINT DF_Transacao_DescricaoRevisada DEFAULT 0,
    -- #111 (ADR 0025): tipo de operação informado pelo banco no Open Finance (operationType/paymentMethod da Pluggy).
    MeioPagamento       TEXT    NULL,
    -- #111: quem pagou e quem recebeu têm o mesmo documento (transferência entre contas suas). Só o sim/não é
    -- guardado, nunca o CPF/CNPJ nem o nome. NULL = o banco não informou.
    MesmaTitularidade   INTEGER NULL,
    -- #123 (ADR 0030): o nome mostrado em Descricao veio de uma regra de descrição (RegraDescricao.Id) ou foi
    -- escolhido à mão (DescricaoManual = 1: as regras não mexem mais). DescricaoOriginal continua sendo a do banco.
    RegraDescricaoId    INTEGER NULL,
    DescricaoManual     INTEGER NOT NULL CONSTRAINT DF_Transacao_DescricaoManual DEFAULT 0,
    -- Guid gerado pela API quando aplicável (Transacao.TransferenciaGrupo); sem DEFAULT no banco.
    TransferenciaGrupo  TEXT    NULL,
    Observacao          TEXT    NULL,
    -- CriadoEm/AtualizadoEm preenchidos pela API (DateTime.UtcNow); sem DEFAULT no banco.
    CriadoEm            TEXT    NOT NULL,
    AtualizadoEm        TEXT    NOT NULL,
    CONSTRAINT CK_Transacao_Tipo CHECK (Tipo IN (1, 2, 3)),
    CONSTRAINT CK_Transacao_Status CHECK (Status IN (1, 2)),
    CONSTRAINT FK_Transacao_Conta FOREIGN KEY (ContaId) REFERENCES Conta (Id) ON DELETE CASCADE,
    CONSTRAINT FK_Transacao_Categoria FOREIGN KEY (CategoriaId) REFERENCES Categoria (Id) ON DELETE SET NULL,
    /* Sem ação em cascata para evitar múltiplos caminhos (Conta -> Fatura -> Transação); tratado na API */
    CONSTRAINT FK_Transacao_FaturaCartao FOREIGN KEY (FaturaCartaoId) REFERENCES FaturaCartao (Id)
);
CREATE UNIQUE INDEX IF NOT EXISTS UX_Transacao_PluggyTransactionId ON Transacao (PluggyTransactionId) WHERE PluggyTransactionId IS NOT NULL;
-- Sem INCLUDE(...): SQLite não tem índice de cobertura; mantém só as colunas-chave.
CREATE INDEX IF NOT EXISTS IX_Transacao_Conta_Data ON Transacao (ContaId, Data DESC);
CREATE INDEX IF NOT EXISTS IX_Transacao_Data ON Transacao (Data DESC);
CREATE INDEX IF NOT EXISTS IX_Transacao_Categoria ON Transacao (CategoriaId, Data);
CREATE INDEX IF NOT EXISTS IX_Transacao_FaturaCartao ON Transacao (FaturaCartaoId) WHERE FaturaCartaoId IS NOT NULL;

/* ------------------------------------------------------------------ */
/* Investimentos (somente leitura, vindos do Open Finance)             */
/* ------------------------------------------------------------------ */
CREATE TABLE IF NOT EXISTS Investimento
(
    Id                  INTEGER NOT NULL CONSTRAINT PK_Investimento PRIMARY KEY AUTOINCREMENT,
    ConexaoBancariaId   INTEGER NOT NULL,
    PluggyInvestmentId  TEXT    NOT NULL,
    Nome                TEXT    NOT NULL,
    Tipo                TEXT    NOT NULL,
    Subtipo             TEXT    NULL,
    Codigo              TEXT    NULL,
    Emissor             TEXT    NULL,
    Moeda               TEXT    NOT NULL CONSTRAINT DF_Investimento_Moeda DEFAULT 'BRL',
    Saldo               TEXT    NOT NULL,
    ValorAplicado       TEXT    NULL,
    Rendimento          TEXT    NULL,
    Quantidade          TEXT    NULL,
    TaxaAnual           TEXT    NULL,
    DataVencimento      TEXT    NULL,
    DataAplicacao       TEXT    NULL,
    Status              TEXT    NULL,
    -- Preenchido pela API (Investimento.AtualizadoEm = DateTime.UtcNow); sem DEFAULT no banco.
    AtualizadoEm        TEXT    NOT NULL,
    CONSTRAINT UQ_Investimento_PluggyInvestmentId UNIQUE (PluggyInvestmentId),
    CONSTRAINT FK_Investimento_ConexaoBancaria FOREIGN KEY (ConexaoBancariaId) REFERENCES ConexaoBancaria (Id) ON DELETE CASCADE
);

/* ------------------------------------------------------------------ */
/* Empréstimos e financiamentos (somente leitura, do Open Finance)     */
/* Natureza: 1 Empréstimo, 2 Financiamento                             */
/* Taxas (TaxaJuros, Cet) em fração: 0.0975 = 9,75%                    */
/* ------------------------------------------------------------------ */
CREATE TABLE IF NOT EXISTS Emprestimo
(
    Id                  INTEGER NOT NULL CONSTRAINT PK_Emprestimo PRIMARY KEY AUTOINCREMENT,
    ConexaoBancariaId   INTEGER NOT NULL,
    PluggyLoanId        TEXT    NOT NULL,
    Natureza            INTEGER NOT NULL CONSTRAINT CK_Emprestimo_Natureza CHECK (Natureza IN (1, 2)),
    Modalidade          TEXT    NULL,
    Produto             TEXT    NOT NULL,
    NumeroContrato      TEXT    NULL,
    Moeda               TEXT    NOT NULL CONSTRAINT DF_Emprestimo_Moeda DEFAULT 'BRL',
    ValorContratado     TEXT    NOT NULL,
    SaldoDevedor        TEXT    NOT NULL,
    ValorParcela        TEXT    NULL,
    ParcelasTotal       INTEGER NULL,
    ParcelasPagas       INTEGER NULL,
    ParcelasAVencer     INTEGER NULL,
    ParcelasEmAtraso    INTEGER NULL,
    Periodicidade       TEXT    NULL,
    SistemaAmortizacao  TEXT    NULL,
    TaxaJuros           TEXT    NULL,
    PeriodicidadeTaxa   TEXT    NULL,
    Indexador           TEXT    NULL,
    Cet                 TEXT    NULL,
    DataContratacao     TEXT    NULL,
    DataPrimeiraParcela TEXT    NULL,
    DataVencimentoFinal TEXT    NULL,
    -- Preenchido pela API (Emprestimo.AtualizadoEm = DateTime.UtcNow); sem DEFAULT no banco.
    AtualizadoEm        TEXT    NOT NULL,
    CONSTRAINT UQ_Emprestimo_PluggyLoanId UNIQUE (PluggyLoanId),
    CONSTRAINT FK_Emprestimo_ConexaoBancaria FOREIGN KEY (ConexaoBancariaId) REFERENCES ConexaoBancaria (Id) ON DELETE CASCADE
);

/* Pagamentos efetuados de cada contrato (recarregados a cada sincronização). */
CREATE TABLE IF NOT EXISTS PagamentoEmprestimo
(
    Id              INTEGER NOT NULL CONSTRAINT PK_PagamentoEmprestimo PRIMARY KEY AUTOINCREMENT,
    EmprestimoId    INTEGER NOT NULL,
    Data            TEXT    NOT NULL,
    Valor           TEXT    NOT NULL,
    Extraordinario  INTEGER NOT NULL CONSTRAINT DF_PagamentoEmprestimo_Extraordinario DEFAULT 0,
    CONSTRAINT FK_PagamentoEmprestimo_Emprestimo FOREIGN KEY (EmprestimoId) REFERENCES Emprestimo (Id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS IX_PagamentoEmprestimo_EmprestimoId_Data ON PagamentoEmprestimo (EmprestimoId, Data);

/* ------------------------------------------------------------------ */
/* Planejamento                                                        */
/* ------------------------------------------------------------------ */
CREATE TABLE IF NOT EXISTS Orcamento
(
    Id          INTEGER NOT NULL CONSTRAINT PK_Orcamento PRIMARY KEY AUTOINCREMENT,
    CategoriaId INTEGER NOT NULL,
    Ano         INTEGER NOT NULL,
    Mes         INTEGER NOT NULL,
    Valor       TEXT    NOT NULL,
    CONSTRAINT UQ_Orcamento_Categoria_Ano_Mes UNIQUE (CategoriaId, Ano, Mes),
    CONSTRAINT CK_Orcamento_Mes CHECK (Mes BETWEEN 1 AND 12),
    -- Valor fica em TEXT (decimal como string); CAST para comparação numérica correta.
    CONSTRAINT CK_Orcamento_Valor CHECK (CAST(Valor AS REAL) >= 0),
    CONSTRAINT FK_Orcamento_Categoria FOREIGN KEY (CategoriaId) REFERENCES Categoria (Id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS Meta
(
    Id          INTEGER NOT NULL CONSTRAINT PK_Meta PRIMARY KEY AUTOINCREMENT,
    Nome        TEXT    NOT NULL,
    ValorAlvo   TEXT    NOT NULL,
    ValorAtual  TEXT    NOT NULL CONSTRAINT DF_Meta_ValorAtual DEFAULT '0',
    DataAlvo    TEXT    NULL,
    ContaId     INTEGER NULL,
    Cor         TEXT    NOT NULL CONSTRAINT DF_Meta_Cor DEFAULT '#0f766e',
    Concluida   INTEGER NOT NULL CONSTRAINT DF_Meta_Concluida DEFAULT 0,
    -- Cofrinho (reserva do extrato, ADR 0010) da conta que guarda o dinheiro da meta: identificado pelo nome,
    -- como no ReservaService. Bancos antigos: coluna via 2-criar-banco.sh.
    ReservaNome TEXT    NULL,
    -- Ordem de prioridade (1 = primeira) para o assistente de metas dividir a sobra; 0 = ainda sem ordem.
    -- Bancos antigos: coluna via 2-criar-banco.sh.
    Prioridade  INTEGER NOT NULL CONSTRAINT DF_Meta_Prioridade DEFAULT 0,
    -- #316: ícone escolhido para a meta (nome da lista do front-end); NULL = o padrão (cofrinho).
    -- Bancos antigos: coluna via 2-criar-banco.sh.
    Icone       TEXT    NULL,
    -- #359: quanto a pessoa disse que vai guardar por mês (o plano das metas usa esse valor); NULL = o plano decide.
    -- Bancos antigos: coluna via 2-criar-banco.sh.
    GuardarPorMes TEXT  NULL,
    -- Preenchido pela API (Meta.CriadoEm = DateTime.UtcNow); sem DEFAULT no banco.
    CriadoEm    TEXT    NOT NULL,
    CONSTRAINT CK_Meta_ValorAlvo CHECK (CAST(ValorAlvo AS REAL) > 0),
    CONSTRAINT FK_Meta_Conta FOREIGN KEY (ContaId) REFERENCES Conta (Id) ON DELETE SET NULL
);

/* Recorrencia: 0 Única, 1 Mensal, 2 Anual */
CREATE TABLE IF NOT EXISTS ContaAgendada
(
    Id                  INTEGER NOT NULL CONSTRAINT PK_ContaAgendada PRIMARY KEY AUTOINCREMENT,
    Descricao           TEXT    NOT NULL,
    Valor               TEXT    NOT NULL,
    ProximoVencimento   TEXT    NOT NULL,
    Recorrencia         INTEGER NOT NULL CONSTRAINT DF_ContaAgendada_Recorrencia DEFAULT 1,
    CategoriaId         INTEGER NULL,
    ContaId             INTEGER NULL,
    Ativa               INTEGER NOT NULL CONSTRAINT DF_ContaAgendada_Ativa DEFAULT 1,
    UltimoPagamentoEm   TEXT    NULL,
    -- #69: último vencimento que ainda se repete (NULL = sem fim). Ao pagar e passar dele, a conta é encerrada.
    DataLimite          TEXT    NULL,
    -- #205: chave da descrição do banco (a do agrupamento das sugestões do extrato) do pagamento que é esta conta.
    ChaveExtrato        TEXT    NULL,
    -- Preenchido pela API (ContaAgendada.CriadoEm = DateTime.UtcNow); sem DEFAULT no banco.
    CriadoEm            TEXT    NOT NULL,
    CONSTRAINT CK_ContaAgendada_Recorrencia CHECK (Recorrencia IN (0, 1, 2)),
    CONSTRAINT CK_ContaAgendada_Valor CHECK (CAST(Valor AS REAL) > 0),
    CONSTRAINT FK_ContaAgendada_Categoria FOREIGN KEY (CategoriaId) REFERENCES Categoria (Id) ON DELETE SET NULL,
    CONSTRAINT FK_ContaAgendada_Conta FOREIGN KEY (ContaId) REFERENCES Conta (Id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS IX_ContaAgendada_ProximoVencimento ON ContaAgendada (Ativa, ProximoVencimento);

/* ------------------------------------------------------------------ */
/* Carteira de renda variável                                          */
/* Ativo.Tipo: 1 FII, 2 Ação, 3 ETF, 4 BDR, 5 Outro                     */
/* OperacaoAtivo.Tipo: 1 Compra, 2 Venda                                */
/* OperacaoAtivo.Origem: 1 Manual, 2 Open Finance, 3 Extrato            */
/* ------------------------------------------------------------------ */
CREATE TABLE IF NOT EXISTS Ativo
(
    Id                  INTEGER NOT NULL CONSTRAINT PK_Ativo PRIMARY KEY AUTOINCREMENT,
    Ticker              TEXT    NOT NULL,
    Nome                TEXT    NULL,
    Tipo                INTEGER NOT NULL CONSTRAINT DF_Ativo_Tipo DEFAULT 1,
    -- Moeda do ativo (ISO 4217): BRL = B3; outra = ativo no exterior. Bancos antigos: coluna via 2-criar-banco.sh.
    Moeda               TEXT    NOT NULL CONSTRAINT DF_Ativo_Moeda DEFAULT 'BRL',
    PrecoAtual          TEXT    NULL,
    PrecoAnterior       TEXT    NULL,
    PrecoAtualizadoEm   TEXT    NULL,
    PrecoManual         INTEGER NOT NULL CONSTRAINT DF_Ativo_PrecoManual DEFAULT 0,
    FonteCotacao        TEXT    NULL,
    Segmento            TEXT    NULL,
    -- Preenchido pela API (Ativo.CriadoEm = DateTime.UtcNow); sem DEFAULT no banco.
    CriadoEm            TEXT    NOT NULL,
    CONSTRAINT UQ_Ativo_Ticker UNIQUE (Ticker),
    CONSTRAINT CK_Ativo_Tipo CHECK (Tipo BETWEEN 1 AND 5)
);

CREATE TABLE IF NOT EXISTS OperacaoAtivo
(
    Id                  INTEGER NOT NULL CONSTRAINT PK_OperacaoAtivo PRIMARY KEY AUTOINCREMENT,
    AtivoId             INTEGER NOT NULL,
    Tipo                INTEGER NOT NULL,
    Data                TEXT    NOT NULL,
    Quantidade          TEXT    NOT NULL,
    PrecoUnitario       TEXT    NOT NULL,
    Taxas               TEXT    NOT NULL CONSTRAINT DF_OperacaoAtivo_Taxas DEFAULT '0',
    -- Reais por 1 unidade da moeda do ativo na operação (1 para BRL). Bancos antigos: coluna via 2-criar-banco.sh.
    Cambio              TEXT    NOT NULL CONSTRAINT DF_OperacaoAtivo_Cambio DEFAULT '1',
    Observacao          TEXT    NULL,
    Origem              INTEGER NOT NULL CONSTRAINT DF_OperacaoAtivo_Origem DEFAULT 1,
    -- Id da movimentação do investimento na Pluggy (operações importadas do Open Finance).
    PluggyTransacaoId   TEXT    NULL,
    -- Transação do extrato que originou a operação (importação pelo extrato).
    TransacaoId         INTEGER NULL,
    -- Preenchido pela API (OperacaoAtivo.CriadoEm = DateTime.UtcNow); sem DEFAULT no banco.
    CriadoEm            TEXT    NOT NULL,
    CONSTRAINT CK_OperacaoAtivo_Tipo CHECK (Tipo IN (1, 2)),
    CONSTRAINT CK_OperacaoAtivo_Quantidade CHECK (CAST(Quantidade AS REAL) > 0),
    CONSTRAINT CK_OperacaoAtivo_Preco CHECK (CAST(PrecoUnitario AS REAL) >= 0),
    CONSTRAINT CK_OperacaoAtivo_Taxas CHECK (CAST(Taxas AS REAL) >= 0),
    CONSTRAINT CK_OperacaoAtivo_Origem CHECK (Origem BETWEEN 1 AND 3),
    CONSTRAINT FK_OperacaoAtivo_Ativo FOREIGN KEY (AtivoId) REFERENCES Ativo (Id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS IX_OperacaoAtivo_Ativo_Data ON OperacaoAtivo (AtivoId, Data);
CREATE UNIQUE INDEX IF NOT EXISTS UX_OperacaoAtivo_PluggyTransacaoId ON OperacaoAtivo (PluggyTransacaoId) WHERE PluggyTransacaoId IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS UX_OperacaoAtivo_TransacaoId ON OperacaoAtivo (TransacaoId) WHERE TransacaoId IS NOT NULL;

/* Proventos (rendimentos/dividendos) por cota, pela data com. */
CREATE TABLE IF NOT EXISTS ProventoAtivo
(
    Id              INTEGER NOT NULL CONSTRAINT PK_ProventoAtivo PRIMARY KEY AUTOINCREMENT,
    AtivoId         INTEGER NOT NULL,
    DataCom         TEXT    NOT NULL,
    ValorPorCota    TEXT    NOT NULL,
    CONSTRAINT UQ_ProventoAtivo UNIQUE (AtivoId, DataCom),
    CONSTRAINT FK_ProventoAtivo_Ativo FOREIGN KEY (AtivoId) REFERENCES Ativo (Id) ON DELETE CASCADE
);

/* Taxa Selic diária (% ao dia), série 11 do SGS/Banco Central — cache local. */
CREATE TABLE IF NOT EXISTS TaxaSelicDiaria
(
    Data    TEXT NOT NULL CONSTRAINT PK_TaxaSelicDiaria PRIMARY KEY,
    Taxa    TEXT NOT NULL
);

/* Fechamento mensal de cada ativo (para evolução do patrimônio e rentabilidade). Mes = 1º dia do mês. */
CREATE TABLE IF NOT EXISTS PrecoMensalAtivo
(
    AtivoId     INTEGER NOT NULL,
    Mes         TEXT    NOT NULL,
    Fechamento  TEXT    NOT NULL,
    CONSTRAINT PK_PrecoMensalAtivo PRIMARY KEY (AtivoId, Mes),
    CONSTRAINT FK_PrecoMensalAtivo_Ativo FOREIGN KEY (AtivoId) REFERENCES Ativo (Id) ON DELETE CASCADE
);

/* Fechamento mensal de índices de referência (ex.: IFIX). */
CREATE TABLE IF NOT EXISTS IndiceMensal
(
    Codigo      TEXT NOT NULL,
    Mes         TEXT NOT NULL,
    Fechamento  TEXT NOT NULL,
    CONSTRAINT PK_IndiceMensal PRIMARY KEY (Codigo, Mes)
);

/* Cotação PTAX de fechamento (reais por 1 unidade da moeda), Banco Central/Olinda — guardada para uso offline
   e atualizada automaticamente pela API (CambioHostedService). */
CREATE TABLE IF NOT EXISTS CotacaoMoeda
(
    Moeda   TEXT NOT NULL,
    Data    TEXT NOT NULL,
    Compra  TEXT NOT NULL,
    Venda   TEXT NOT NULL,
    AtualizadoEm TEXT NOT NULL,
    CONSTRAINT PK_CotacaoMoeda PRIMARY KEY (Moeda, Data)
);

/* Saldo real de uma reserva ("cofrinho") conferido pelo usuário. A instituição não informa esse saldo pelo
   Open Finance: a API o estima pelos lançamentos "Dinheiro reservado/retirado {Nome}" da conta, a partir
   desta conferência (lançamentos até DataConferencia já estão em SaldoConferido). */
CREATE TABLE IF NOT EXISTS ReservaConferencia
(
    Id              INTEGER NOT NULL CONSTRAINT PK_ReservaConferencia PRIMARY KEY AUTOINCREMENT,
    ContaId         INTEGER NOT NULL,
    Nome            TEXT    NOT NULL,
    SaldoConferido  TEXT    NOT NULL,
    DataConferencia TEXT    NOT NULL,
    -- Preenchido pela API (ReservaConferencia.AtualizadoEm = DateTime.UtcNow); sem DEFAULT no banco.
    AtualizadoEm    TEXT    NOT NULL,
    CONSTRAINT CK_ReservaConferencia_Saldo CHECK (CAST(SaldoConferido AS REAL) >= 0),
    CONSTRAINT FK_ReservaConferencia_Conta FOREIGN KEY (ContaId) REFERENCES Conta (Id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS UX_ReservaConferencia_Conta_Nome ON ReservaConferencia (ContaId, Nome);

/* Erros e avisos do sistema (#11, ADR 0022), gravados pela API (RegistroErrosLoggerProvider) sem dados sensíveis:
   tokens, senhas, query strings, CPF e valores em R$ são removidos antes de gravar. A mesma ocorrência
   repetida enquanto está em aberto vira uma linha só (Ocorrencias/UltimaEm). Apagados depois dos dias definidos em
   Parametrização (Diagnostico.DiasRetencaoErros). */
CREATE TABLE IF NOT EXISTS ErroSistema
(
    Id           INTEGER NOT NULL CONSTRAINT PK_ErroSistema PRIMARY KEY AUTOINCREMENT,
    -- PrimeiraEm/UltimaEm/ResolvidoEm em UTC, preenchidos pela API; sem DEFAULT no banco.
    PrimeiraEm   TEXT    NOT NULL,
    UltimaEm     TEXT    NOT NULL,
    Ocorrencias  INTEGER NOT NULL CONSTRAINT DF_ErroSistema_Ocorrencias DEFAULT 1,
    -- 3 Aviso, 4 Erro, 5 Crítico (os mesmos números do LogLevel do .NET).
    Nivel        INTEGER NOT NULL,
    Origem       TEXT    NOT NULL,
    Categoria    TEXT    NOT NULL,
    Mensagem     TEXT    NOT NULL,
    Detalhe      TEXT    NULL,
    Rota         TEXT    NULL,
    Usuario      TEXT    NULL,
    Assinatura   TEXT    NOT NULL,
    Resolvido    INTEGER NOT NULL CONSTRAINT DF_ErroSistema_Resolvido DEFAULT 0,
    ResolvidoEm  TEXT    NULL,
    CONSTRAINT CK_ErroSistema_Nivel CHECK (Nivel IN (3, 4, 5)),
    CONSTRAINT CK_ErroSistema_Ocorrencias CHECK (Ocorrencias >= 1)
);
CREATE INDEX IF NOT EXISTS IX_ErroSistema_Resolvido_UltimaEm ON ErroSistema (Resolvido, UltimaEm);
CREATE INDEX IF NOT EXISTS IX_ErroSistema_Assinatura ON ErroSistema (Assinatura, Resolvido);

/* ------------------------------------------------------------------ */
/* Regras de descrição (#123, ADR 0030)                                */
/* ------------------------------------------------------------------ */
/* "Quando a descrição do banco for/contiver X, mostrar Y": valem nas transações existentes e nas que chegam pela
   sincronização, menos nas que tiveram a descrição escolhida à mão. Separadas de RegraCategorizacao porque lá a
   categoria é obrigatória (NOT NULL) e o SQLite não deixa mudar isso sem recriar a tabela. */
CREATE TABLE IF NOT EXISTS RegraDescricao
(
    Id                  INTEGER NOT NULL CONSTRAINT PK_RegraDescricao PRIMARY KEY AUTOINCREMENT,
    -- Comparado com a descrição do banco (Transacao.DescricaoOriginal), sem acento e sem diferença de maiúsculas.
    Texto               TEXT    NOT NULL,
    TipoCorrespondencia INTEGER NOT NULL CONSTRAINT DF_RegraDescricao_TipoCorrespondencia DEFAULT 3,
    NovaDescricao       TEXT    NOT NULL,
    -- #241: a regra só vale com o valor da transação (sem o sinal) nesta faixa; NULL = sem piso ou sem teto.
    ValorMinimo         TEXT    NULL,
    ValorMaximo         TEXT    NULL,
    Ativa               INTEGER NOT NULL CONSTRAINT DF_RegraDescricao_Ativa DEFAULT 1,
    -- Preenchido pela API (RegraDescricao.CriadoEm = DateTime.UtcNow); sem DEFAULT no banco.
    CriadoEm            TEXT    NOT NULL,
    CONSTRAINT CK_RegraDescricao_TipoCorrespondencia CHECK (TipoCorrespondencia IN (1, 2, 3))
);

/* ------------------------------------------------------------------ */
/* Histórico de saldo dos investimentos do Open Finance (#173 e)       */
/* ------------------------------------------------------------------ */
/* ADR 0045: um registro por investimento e dia (a última sincronização do dia vale), para o rendimento de cada mês da
   caixinha/CDB. Só renda fixa, fundos e outros em reais (a renda variável tem a carteira própria). Sem chave estrangeira
   para Investimento, de propósito: o investimento resgatado sai de Investimento (a instituição para de mandar) e o
   histórico dele fica. */
CREATE TABLE IF NOT EXISTS HistoricoInvestimento
(
    Id                  INTEGER NOT NULL CONSTRAINT PK_HistoricoInvestimento PRIMARY KEY AUTOINCREMENT,
    PluggyInvestmentId  TEXT    NOT NULL,
    ConexaoBancariaId   INTEGER NOT NULL,
    Nome                TEXT    NOT NULL,
    Tipo                TEXT    NOT NULL,
    Data                TEXT    NOT NULL,
    Saldo               TEXT    NOT NULL,
    ValorAplicado       TEXT    NULL,
    -- Preenchido pela API (HistoricoInvestimento.AtualizadoEm = DateTime.UtcNow); sem DEFAULT no banco.
    AtualizadoEm        TEXT    NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS UX_HistoricoInvestimento_Investimento_Data ON HistoricoInvestimento (PluggyInvestmentId, Data);

/* ------------------------------------------------------------------ */
/* Cotação de criptomoedas (#279, ADR 0048)                            */
/* ------------------------------------------------------------------ */
/* Último negócio em reais no Mercado Bitcoin (API pública), gravado a cada 30 minutos pela API (CriptoHostedService)
   e guardado por 30 dias, para a tela funcionar sem internet e para a variação em 24 h. Cripto negocia 24 h por dia:
   a chave é moeda + data e hora da fonte (UTC). */
CREATE TABLE IF NOT EXISTS CotacaoCripto
(
    Moeda        TEXT NOT NULL,
    DataHora     TEXT NOT NULL,
    Preco        TEXT NOT NULL,
    -- Preenchido pela API (CotacaoCripto.AtualizadoEm = DateTime.UtcNow); sem DEFAULT no banco.
    AtualizadoEm TEXT NOT NULL,
    CONSTRAINT PK_CotacaoCripto PRIMARY KEY (Moeda, DataHora)
);

/* ------------------------------------------------------------------ */
/* Aportes das metas (#318)                                            */
/* ------------------------------------------------------------------ */
/* Um registro por "+ aporte" (ou retirada, valor negativo), com a data em Brasília: é o histórico do gráfico "Como
   está indo" da meta. Meta.ValorAtual continua sendo o total; antes desta tabela, os aportes não tinham data. */
CREATE TABLE IF NOT EXISTS MetaAporte
(
    Id       INTEGER NOT NULL CONSTRAINT PK_MetaAporte PRIMARY KEY AUTOINCREMENT,
    MetaId   INTEGER NOT NULL,
    Data     TEXT    NOT NULL,
    Valor    TEXT    NOT NULL,
    -- Preenchido pela API (MetaAporte.CriadoEm = DateTime.UtcNow); sem DEFAULT no banco.
    CriadoEm TEXT    NOT NULL,
    CONSTRAINT FK_MetaAporte_Meta FOREIGN KEY (MetaId) REFERENCES Meta (Id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS IX_MetaAporte_Meta_Data ON MetaAporte (MetaId, Data);
