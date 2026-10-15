/* =====================================================================
   FinancasLocal - Estrutura completa do banco de dados (SQL Server)
   Executado pelo script scripts\3-criar-banco.ps1.
   O marcador {{PASTA_DADOS}} é substituído pelo caminho real (ex.: D:\FinancasLocal\dados\sql).
   Script idempotente: pode ser executado novamente sem perder dados.
   ===================================================================== */

IF DB_ID(N'FinancasLocal') IS NULL
BEGIN
    CREATE DATABASE [FinancasLocal]
    ON PRIMARY (NAME = N'FinancasLocal', FILENAME = N'{{PASTA_DADOS}}\FinancasLocal.mdf', SIZE = 64MB, FILEGROWTH = 64MB)
    LOG ON      (NAME = N'FinancasLocal_log', FILENAME = N'{{PASTA_DADOS}}\FinancasLocal_log.ldf', SIZE = 32MB, FILEGROWTH = 32MB)
    COLLATE Latin1_General_CI_AI;
END
GO

ALTER DATABASE [FinancasLocal] SET RECOVERY SIMPLE;
GO
ALTER DATABASE [FinancasLocal] SET PAGE_VERIFY CHECKSUM;
GO

USE [FinancasLocal];
GO

/* ------------------------------------------------------------------ */
/* Segurança / acesso                                                  */
/* ------------------------------------------------------------------ */
IF OBJECT_ID(N'dbo.Usuario', N'U') IS NULL
CREATE TABLE dbo.Usuario
(
    Id                  INT IDENTITY(1,1)  NOT NULL CONSTRAINT PK_Usuario PRIMARY KEY,
    Login               NVARCHAR(100)      NOT NULL,
    SenhaHash           NVARCHAR(500)      NOT NULL,
    CarimboSeguranca    UNIQUEIDENTIFIER   NOT NULL CONSTRAINT DF_Usuario_Carimbo DEFAULT NEWID(),
    TentativasFalhas    INT                NOT NULL CONSTRAINT DF_Usuario_Tentativas DEFAULT 0,
    BloqueadoAte        DATETIME2(0)       NULL,
    UltimoLoginEm       DATETIME2(0)       NULL,
    CriadoEm            DATETIME2(0)       NOT NULL CONSTRAINT DF_Usuario_CriadoEm DEFAULT SYSUTCDATETIME(),
    CONSTRAINT UQ_Usuario_Login UNIQUE (Login)
);
GO

IF OBJECT_ID(N'dbo.AuditoriaAcesso', N'U') IS NULL
CREATE TABLE dbo.AuditoriaAcesso
(
    Id          BIGINT IDENTITY(1,1) NOT NULL CONSTRAINT PK_AuditoriaAcesso PRIMARY KEY,
    DataHora    DATETIME2(0)         NOT NULL CONSTRAINT DF_AuditoriaAcesso_DataHora DEFAULT SYSUTCDATETIME(),
    Evento      NVARCHAR(50)         NOT NULL,
    Login       NVARCHAR(100)        NULL,
    Ip          NVARCHAR(64)         NULL,
    UserAgent   NVARCHAR(400)        NULL
);
GO

IF OBJECT_ID(N'dbo.Configuracao', N'U') IS NULL
CREATE TABLE dbo.Configuracao
(
    Chave       NVARCHAR(100)   NOT NULL CONSTRAINT PK_Configuracao PRIMARY KEY,
    Valor       NVARCHAR(MAX)   NULL,
    Protegido   BIT             NOT NULL CONSTRAINT DF_Configuracao_Protegido DEFAULT 0,
    AtualizadoEm DATETIME2(0)   NOT NULL CONSTRAINT DF_Configuracao_AtualizadoEm DEFAULT SYSUTCDATETIME()
);
GO

/* ------------------------------------------------------------------ */
/* Open Finance (Pluggy)                                               */
/* ------------------------------------------------------------------ */
IF OBJECT_ID(N'dbo.ConexaoBancaria', N'U') IS NULL
CREATE TABLE dbo.ConexaoBancaria
(
    Id                          INT IDENTITY(1,1) NOT NULL CONSTRAINT PK_ConexaoBancaria PRIMARY KEY,
    PluggyItemId                NVARCHAR(64)      NOT NULL,
    NomeInstituicao             NVARCHAR(150)     NULL,
    CorPrimaria                 NVARCHAR(9)       NULL,
    Status                      NVARCHAR(40)      NULL,
    StatusExecucao              NVARCHAR(60)      NULL,
    UltimaAtualizacaoInstituicao DATETIME2(0)     NULL,
    ConsentimentoExpiraEm       DATETIME2(0)      NULL,
    UltimaSincronizacaoEm       DATETIME2(0)      NULL,
    UltimoErro                  NVARCHAR(2000)    NULL,
    Ativa                       BIT               NOT NULL CONSTRAINT DF_ConexaoBancaria_Ativa DEFAULT 1,
    CriadoEm                    DATETIME2(0)      NOT NULL CONSTRAINT DF_ConexaoBancaria_CriadoEm DEFAULT SYSUTCDATETIME(),
    CONSTRAINT UQ_ConexaoBancaria_PluggyItemId UNIQUE (PluggyItemId)
);
GO

IF OBJECT_ID(N'dbo.SincronizacaoLog', N'U') IS NULL
CREATE TABLE dbo.SincronizacaoLog
(
    Id                      BIGINT IDENTITY(1,1) NOT NULL CONSTRAINT PK_SincronizacaoLog PRIMARY KEY,
    ConexaoBancariaId       INT                  NULL,
    Origem                  NVARCHAR(20)         NOT NULL,
    IniciadoEm              DATETIME2(0)         NOT NULL,
    FinalizadoEm            DATETIME2(0)         NULL,
    Sucesso                 BIT                  NOT NULL CONSTRAINT DF_SincronizacaoLog_Sucesso DEFAULT 0,
    TransacoesNovas         INT                  NOT NULL CONSTRAINT DF_SincronizacaoLog_Novas DEFAULT 0,
    TransacoesAtualizadas   INT                  NOT NULL CONSTRAINT DF_SincronizacaoLog_Atualizadas DEFAULT 0,
    Mensagem                NVARCHAR(2000)       NULL,
    CONSTRAINT FK_SincronizacaoLog_ConexaoBancaria FOREIGN KEY (ConexaoBancariaId)
        REFERENCES dbo.ConexaoBancaria (Id) ON DELETE SET NULL
);
GO
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'IX_SincronizacaoLog_IniciadoEm')
    CREATE INDEX IX_SincronizacaoLog_IniciadoEm ON dbo.SincronizacaoLog (IniciadoEm DESC);
GO

/* ------------------------------------------------------------------ */
/* Categorias                                                          */
/* Tipo: 1 = Despesa, 2 = Receita, 3 = Transferência                   */
/* ------------------------------------------------------------------ */
IF OBJECT_ID(N'dbo.Categoria', N'U') IS NULL
CREATE TABLE dbo.Categoria
(
    Id              INT IDENTITY(1,1) NOT NULL CONSTRAINT PK_Categoria PRIMARY KEY,
    Nome            NVARCHAR(80)      NOT NULL,
    Tipo            TINYINT           NOT NULL,
    Cor             NVARCHAR(9)       NOT NULL CONSTRAINT DF_Categoria_Cor DEFAULT N'#64748b',
    Icone           NVARCHAR(40)      NULL,
    CategoriaPaiId  INT               NULL,
    Ativa           BIT               NOT NULL CONSTRAINT DF_Categoria_Ativa DEFAULT 1,
    CONSTRAINT UQ_Categoria_Nome UNIQUE (Nome),
    CONSTRAINT CK_Categoria_Tipo CHECK (Tipo IN (1, 2, 3)),
    CONSTRAINT FK_Categoria_CategoriaPai FOREIGN KEY (CategoriaPaiId) REFERENCES dbo.Categoria (Id)
);
GO

/* Natureza do gasto (v4): 1 = Essencial, 2 = Estilo de vida. Usada na regra 50/30/20. */
IF COL_LENGTH(N'dbo.Categoria', N'Natureza') IS NULL
    ALTER TABLE dbo.Categoria ADD Natureza TINYINT NULL
        CONSTRAINT CK_Categoria_Natureza CHECK (Natureza IN (1, 2));
GO

IF OBJECT_ID(N'dbo.CategoriaPluggy', N'U') IS NULL
CREATE TABLE dbo.CategoriaPluggy
(
    PluggyCategoriaId   NVARCHAR(20)    NOT NULL CONSTRAINT PK_CategoriaPluggy PRIMARY KEY,
    Descricao           NVARCHAR(150)   NOT NULL,
    PaiPluggyId         NVARCHAR(20)    NULL,
    PaiDescricao        NVARCHAR(150)   NULL,
    CategoriaId         INT             NULL,
    CONSTRAINT FK_CategoriaPluggy_Categoria FOREIGN KEY (CategoriaId) REFERENCES dbo.Categoria (Id) ON DELETE SET NULL
);
GO

/* TipoCorrespondencia: 1 = Contém, 2 = Começa com, 3 = Igual */
IF OBJECT_ID(N'dbo.RegraCategorizacao', N'U') IS NULL
CREATE TABLE dbo.RegraCategorizacao
(
    Id                  INT IDENTITY(1,1) NOT NULL CONSTRAINT PK_RegraCategorizacao PRIMARY KEY,
    Texto               NVARCHAR(200)     NOT NULL,
    TipoCorrespondencia TINYINT           NOT NULL CONSTRAINT DF_Regra_TipoCorrespondencia DEFAULT 1,
    CategoriaId         INT               NOT NULL,
    Prioridade          INT               NOT NULL CONSTRAINT DF_Regra_Prioridade DEFAULT 100,
    Ativa               BIT               NOT NULL CONSTRAINT DF_Regra_Ativa DEFAULT 1,
    CriadoEm            DATETIME2(0)      NOT NULL CONSTRAINT DF_Regra_CriadoEm DEFAULT SYSUTCDATETIME(),
    CONSTRAINT CK_Regra_TipoCorrespondencia CHECK (TipoCorrespondencia IN (1, 2, 3)),
    CONSTRAINT FK_RegraCategorizacao_Categoria FOREIGN KEY (CategoriaId) REFERENCES dbo.Categoria (Id) ON DELETE CASCADE
);
GO

/* ------------------------------------------------------------------ */
/* Contas                                                              */
/* Tipo: 1 Conta corrente, 2 Poupança, 3 Cartão de crédito,            */
/*       4 Investimento, 5 Dinheiro/Carteira, 6 Outro                  */
/* Origem: 1 Manual, 2 Open Finance (Pluggy)                           */
/* ------------------------------------------------------------------ */
IF OBJECT_ID(N'dbo.Conta', N'U') IS NULL
CREATE TABLE dbo.Conta
(
    Id                  INT IDENTITY(1,1) NOT NULL CONSTRAINT PK_Conta PRIMARY KEY,
    ConexaoBancariaId   INT               NULL,
    PluggyAccountId     NVARCHAR(64)      NULL,
    Nome                NVARCHAR(120)     NOT NULL,
    Tipo                TINYINT           NOT NULL,
    Origem              TINYINT           NOT NULL,
    Numero              NVARCHAR(60)      NULL,
    Moeda               CHAR(3)           NOT NULL CONSTRAINT DF_Conta_Moeda DEFAULT 'BRL',
    SaldoInicial        DECIMAL(18,2)     NOT NULL CONSTRAINT DF_Conta_SaldoInicial DEFAULT 0,
    Saldo               DECIMAL(18,2)     NOT NULL CONSTRAINT DF_Conta_Saldo DEFAULT 0,
    LimiteCredito       DECIMAL(18,2)     NULL,
    LimiteDisponivel    DECIMAL(18,2)     NULL,
    DiaFechamento       TINYINT           NULL,
    DiaVencimento       TINYINT           NULL,
    Cor                 NVARCHAR(9)       NOT NULL CONSTRAINT DF_Conta_Cor DEFAULT N'#0f766e',
    IncluirNoSaldo      BIT               NOT NULL CONSTRAINT DF_Conta_IncluirNoSaldo DEFAULT 1,
    Arquivada           BIT               NOT NULL CONSTRAINT DF_Conta_Arquivada DEFAULT 0,
    CriadoEm            DATETIME2(0)      NOT NULL CONSTRAINT DF_Conta_CriadoEm DEFAULT SYSUTCDATETIME(),
    AtualizadoEm        DATETIME2(0)      NOT NULL CONSTRAINT DF_Conta_AtualizadoEm DEFAULT SYSUTCDATETIME(),
    CONSTRAINT CK_Conta_Tipo CHECK (Tipo BETWEEN 1 AND 6),
    CONSTRAINT CK_Conta_Origem CHECK (Origem IN (1, 2)),
    CONSTRAINT CK_Conta_DiaFechamento CHECK (DiaFechamento IS NULL OR DiaFechamento BETWEEN 1 AND 31),
    CONSTRAINT CK_Conta_DiaVencimento CHECK (DiaVencimento IS NULL OR DiaVencimento BETWEEN 1 AND 31),
    CONSTRAINT FK_Conta_ConexaoBancaria FOREIGN KEY (ConexaoBancariaId) REFERENCES dbo.ConexaoBancaria (Id) ON DELETE CASCADE
);
GO
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'UX_Conta_PluggyAccountId')
    CREATE UNIQUE INDEX UX_Conta_PluggyAccountId ON dbo.Conta (PluggyAccountId) WHERE PluggyAccountId IS NOT NULL;
GO

IF OBJECT_ID(N'dbo.FaturaCartao', N'U') IS NULL
CREATE TABLE dbo.FaturaCartao
(
    Id              INT IDENTITY(1,1) NOT NULL CONSTRAINT PK_FaturaCartao PRIMARY KEY,
    ContaId         INT               NOT NULL,
    PluggyBillId    NVARCHAR(64)      NULL,
    DataVencimento  DATE              NOT NULL,
    DataFechamento  DATE              NULL,
    ValorTotal      DECIMAL(18,2)     NOT NULL,
    ValorMinimo     DECIMAL(18,2)     NULL,
    ValorPago       DECIMAL(18,2)     NULL,
    AtualizadoEm    DATETIME2(0)      NOT NULL CONSTRAINT DF_FaturaCartao_AtualizadoEm DEFAULT SYSUTCDATETIME(),
    CONSTRAINT FK_FaturaCartao_Conta FOREIGN KEY (ContaId) REFERENCES dbo.Conta (Id) ON DELETE CASCADE
);
GO
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'UX_FaturaCartao_PluggyBillId')
    CREATE UNIQUE INDEX UX_FaturaCartao_PluggyBillId ON dbo.FaturaCartao (PluggyBillId) WHERE PluggyBillId IS NOT NULL;
GO
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'IX_FaturaCartao_Conta_Vencimento')
    CREATE INDEX IX_FaturaCartao_Conta_Vencimento ON dbo.FaturaCartao (ContaId, DataVencimento DESC);
GO

/* ------------------------------------------------------------------ */
/* Transações                                                          */
/* Valor: negativo = saída, positivo = entrada                         */
/* Tipo: 1 Despesa, 2 Receita, 3 Transferência                         */
/* Status: 1 Efetivada, 2 Pendente                                     */
/* ------------------------------------------------------------------ */
IF OBJECT_ID(N'dbo.Transacao', N'U') IS NULL
CREATE TABLE dbo.Transacao
(
    Id                  BIGINT IDENTITY(1,1) NOT NULL CONSTRAINT PK_Transacao PRIMARY KEY,
    ContaId             INT                  NOT NULL,
    PluggyTransactionId NVARCHAR(64)         NULL,
    Data                DATE                 NOT NULL,
    Descricao           NVARCHAR(300)        NOT NULL,
    DescricaoOriginal   NVARCHAR(300)        NULL,
    Estabelecimento     NVARCHAR(200)        NULL,
    Valor               DECIMAL(18,2)        NOT NULL,
    Tipo                TINYINT              NOT NULL,
    Status              TINYINT              NOT NULL CONSTRAINT DF_Transacao_Status DEFAULT 1,
    CategoriaId         INT                  NULL,
    CategoriaManual     BIT                  NOT NULL CONSTRAINT DF_Transacao_CategoriaManual DEFAULT 0,
    PluggyCategoriaId   NVARCHAR(20)         NULL,
    FaturaCartaoId      INT                  NULL,
    ParcelaNumero       SMALLINT             NULL,
    ParcelaTotal        SMALLINT             NULL,
    IgnorarRelatorios   BIT                  NOT NULL CONSTRAINT DF_Transacao_Ignorar DEFAULT 0,
    TransferenciaGrupo  UNIQUEIDENTIFIER     NULL,
    Observacao          NVARCHAR(1000)       NULL,
    CriadoEm            DATETIME2(0)         NOT NULL CONSTRAINT DF_Transacao_CriadoEm DEFAULT SYSUTCDATETIME(),
    AtualizadoEm        DATETIME2(0)         NOT NULL CONSTRAINT DF_Transacao_AtualizadoEm DEFAULT SYSUTCDATETIME(),
    CONSTRAINT CK_Transacao_Tipo CHECK (Tipo IN (1, 2, 3)),
    CONSTRAINT CK_Transacao_Status CHECK (Status IN (1, 2)),
    CONSTRAINT FK_Transacao_Conta FOREIGN KEY (ContaId) REFERENCES dbo.Conta (Id) ON DELETE CASCADE,
    CONSTRAINT FK_Transacao_Categoria FOREIGN KEY (CategoriaId) REFERENCES dbo.Categoria (Id) ON DELETE SET NULL,
    /* Sem ação em cascata para evitar múltiplos caminhos (Conta -> Fatura -> Transação); tratado na API */
    CONSTRAINT FK_Transacao_FaturaCartao FOREIGN KEY (FaturaCartaoId) REFERENCES dbo.FaturaCartao (Id)
);
GO
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'UX_Transacao_PluggyTransactionId')
    CREATE UNIQUE INDEX UX_Transacao_PluggyTransactionId ON dbo.Transacao (PluggyTransactionId) WHERE PluggyTransactionId IS NOT NULL;
GO
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'IX_Transacao_Conta_Data')
    CREATE INDEX IX_Transacao_Conta_Data ON dbo.Transacao (ContaId, Data DESC) INCLUDE (Valor, Tipo, CategoriaId, Status);
GO
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'IX_Transacao_Data')
    CREATE INDEX IX_Transacao_Data ON dbo.Transacao (Data DESC) INCLUDE (Valor, Tipo, CategoriaId, IgnorarRelatorios, ContaId);
GO
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'IX_Transacao_Categoria')
    CREATE INDEX IX_Transacao_Categoria ON dbo.Transacao (CategoriaId, Data);
GO
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'IX_Transacao_FaturaCartao')
    CREATE INDEX IX_Transacao_FaturaCartao ON dbo.Transacao (FaturaCartaoId) WHERE FaturaCartaoId IS NOT NULL;
GO

/* ------------------------------------------------------------------ */
/* Investimentos (somente leitura, vindos do Open Finance)             */
/* ------------------------------------------------------------------ */
IF OBJECT_ID(N'dbo.Investimento', N'U') IS NULL
CREATE TABLE dbo.Investimento
(
    Id                  INT IDENTITY(1,1) NOT NULL CONSTRAINT PK_Investimento PRIMARY KEY,
    ConexaoBancariaId   INT               NOT NULL,
    PluggyInvestmentId  NVARCHAR(64)      NOT NULL,
    Nome                NVARCHAR(200)     NOT NULL,
    Tipo                NVARCHAR(30)      NOT NULL,
    Subtipo             NVARCHAR(60)      NULL,
    Codigo              NVARCHAR(60)      NULL,
    Emissor             NVARCHAR(200)     NULL,
    Moeda               CHAR(3)           NOT NULL CONSTRAINT DF_Investimento_Moeda DEFAULT 'BRL',
    Saldo               DECIMAL(18,2)     NOT NULL,
    ValorAplicado       DECIMAL(18,2)     NULL,
    Rendimento          DECIMAL(18,2)     NULL,
    Quantidade          DECIMAL(28,8)     NULL,
    TaxaAnual           DECIMAL(9,4)      NULL,
    DataVencimento      DATE              NULL,
    Status              NVARCHAR(30)      NULL,
    AtualizadoEm        DATETIME2(0)      NOT NULL CONSTRAINT DF_Investimento_AtualizadoEm DEFAULT SYSUTCDATETIME(),
    CONSTRAINT UQ_Investimento_PluggyInvestmentId UNIQUE (PluggyInvestmentId),
    CONSTRAINT FK_Investimento_ConexaoBancaria FOREIGN KEY (ConexaoBancariaId) REFERENCES dbo.ConexaoBancaria (Id) ON DELETE CASCADE
);
GO

/* ------------------------------------------------------------------ */
/* Planejamento                                                        */
/* ------------------------------------------------------------------ */
IF OBJECT_ID(N'dbo.Orcamento', N'U') IS NULL
CREATE TABLE dbo.Orcamento
(
    Id          INT IDENTITY(1,1) NOT NULL CONSTRAINT PK_Orcamento PRIMARY KEY,
    CategoriaId INT               NOT NULL,
    Ano         SMALLINT          NOT NULL,
    Mes         TINYINT           NOT NULL,
    Valor       DECIMAL(18,2)     NOT NULL,
    CONSTRAINT UQ_Orcamento_Categoria_Ano_Mes UNIQUE (CategoriaId, Ano, Mes),
    CONSTRAINT CK_Orcamento_Mes CHECK (Mes BETWEEN 1 AND 12),
    CONSTRAINT CK_Orcamento_Valor CHECK (Valor >= 0),
    CONSTRAINT FK_Orcamento_Categoria FOREIGN KEY (CategoriaId) REFERENCES dbo.Categoria (Id) ON DELETE CASCADE
);
GO

IF OBJECT_ID(N'dbo.Meta', N'U') IS NULL
CREATE TABLE dbo.Meta
(
    Id          INT IDENTITY(1,1) NOT NULL CONSTRAINT PK_Meta PRIMARY KEY,
    Nome        NVARCHAR(120)     NOT NULL,
    ValorAlvo   DECIMAL(18,2)     NOT NULL,
    ValorAtual  DECIMAL(18,2)     NOT NULL CONSTRAINT DF_Meta_ValorAtual DEFAULT 0,
    DataAlvo    DATE              NULL,
    ContaId     INT               NULL,
    Cor         NVARCHAR(9)       NOT NULL CONSTRAINT DF_Meta_Cor DEFAULT N'#0f766e',
    Concluida   BIT               NOT NULL CONSTRAINT DF_Meta_Concluida DEFAULT 0,
    CriadoEm    DATETIME2(0)      NOT NULL CONSTRAINT DF_Meta_CriadoEm DEFAULT SYSUTCDATETIME(),
    CONSTRAINT CK_Meta_ValorAlvo CHECK (ValorAlvo > 0),
    CONSTRAINT FK_Meta_Conta FOREIGN KEY (ContaId) REFERENCES dbo.Conta (Id) ON DELETE SET NULL
);
GO

/* Recorrencia: 0 Única, 1 Mensal, 2 Anual */
IF OBJECT_ID(N'dbo.ContaAgendada', N'U') IS NULL
CREATE TABLE dbo.ContaAgendada
(
    Id                  INT IDENTITY(1,1) NOT NULL CONSTRAINT PK_ContaAgendada PRIMARY KEY,
    Descricao           NVARCHAR(150)     NOT NULL,
    Valor               DECIMAL(18,2)     NOT NULL,
    ProximoVencimento   DATE              NOT NULL,
    Recorrencia         TINYINT           NOT NULL CONSTRAINT DF_ContaAgendada_Recorrencia DEFAULT 1,
    CategoriaId         INT               NULL,
    ContaId             INT               NULL,
    Ativa               BIT               NOT NULL CONSTRAINT DF_ContaAgendada_Ativa DEFAULT 1,
    UltimoPagamentoEm   DATE              NULL,
    CriadoEm            DATETIME2(0)      NOT NULL CONSTRAINT DF_ContaAgendada_CriadoEm DEFAULT SYSUTCDATETIME(),
    CONSTRAINT CK_ContaAgendada_Recorrencia CHECK (Recorrencia IN (0, 1, 2)),
    CONSTRAINT CK_ContaAgendada_Valor CHECK (Valor > 0),
    CONSTRAINT FK_ContaAgendada_Categoria FOREIGN KEY (CategoriaId) REFERENCES dbo.Categoria (Id) ON DELETE SET NULL,
    CONSTRAINT FK_ContaAgendada_Conta FOREIGN KEY (ContaId) REFERENCES dbo.Conta (Id) ON DELETE SET NULL
);
GO
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'IX_ContaAgendada_ProximoVencimento')
    CREATE INDEX IX_ContaAgendada_ProximoVencimento ON dbo.ContaAgendada (Ativa, ProximoVencimento);
GO

/* ------------------------------------------------------------------ */
/* Carteira de renda variável (v5)                                     */
/* Ativo.Tipo: 1 FII, 2 Ação, 3 ETF, 4 BDR, 5 Outro                     */
/* OperacaoAtivo.Tipo: 1 Compra, 2 Venda                                */
/* ------------------------------------------------------------------ */
IF COL_LENGTH(N'dbo.Investimento', N'DataAplicacao') IS NULL
    ALTER TABLE dbo.Investimento ADD DataAplicacao DATE NULL;
GO

IF OBJECT_ID(N'dbo.Ativo', N'U') IS NULL
CREATE TABLE dbo.Ativo
(
    Id                  INT IDENTITY(1,1) NOT NULL CONSTRAINT PK_Ativo PRIMARY KEY,
    Ticker              NVARCHAR(12)      NOT NULL,
    Nome                NVARCHAR(150)     NULL,
    Tipo                TINYINT           NOT NULL CONSTRAINT DF_Ativo_Tipo DEFAULT 1,
    PrecoAtual          DECIMAL(18,4)     NULL,
    PrecoAnterior       DECIMAL(18,4)     NULL,
    PrecoAtualizadoEm   DATETIME2(0)      NULL,
    PrecoManual         BIT               NOT NULL CONSTRAINT DF_Ativo_PrecoManual DEFAULT 0,
    FonteCotacao        NVARCHAR(20)      NULL,
    CriadoEm            DATETIME2(0)      NOT NULL CONSTRAINT DF_Ativo_CriadoEm DEFAULT SYSUTCDATETIME(),
    CONSTRAINT UQ_Ativo_Ticker UNIQUE (Ticker),
    CONSTRAINT CK_Ativo_Tipo CHECK (Tipo BETWEEN 1 AND 5)
);
GO

IF OBJECT_ID(N'dbo.OperacaoAtivo', N'U') IS NULL
CREATE TABLE dbo.OperacaoAtivo
(
    Id              INT IDENTITY(1,1) NOT NULL CONSTRAINT PK_OperacaoAtivo PRIMARY KEY,
    AtivoId         INT               NOT NULL,
    Tipo            TINYINT           NOT NULL,
    Data            DATE              NOT NULL,
    Quantidade      DECIMAL(18,6)     NOT NULL,
    PrecoUnitario   DECIMAL(18,6)     NOT NULL,
    Taxas           DECIMAL(18,2)     NOT NULL CONSTRAINT DF_OperacaoAtivo_Taxas DEFAULT 0,
    Observacao      NVARCHAR(300)     NULL,
    CriadoEm        DATETIME2(0)      NOT NULL CONSTRAINT DF_OperacaoAtivo_CriadoEm DEFAULT SYSUTCDATETIME(),
    CONSTRAINT CK_OperacaoAtivo_Tipo CHECK (Tipo IN (1, 2)),
    CONSTRAINT CK_OperacaoAtivo_Quantidade CHECK (Quantidade > 0),
    CONSTRAINT CK_OperacaoAtivo_Preco CHECK (PrecoUnitario >= 0),
    CONSTRAINT CK_OperacaoAtivo_Taxas CHECK (Taxas >= 0),
    CONSTRAINT FK_OperacaoAtivo_Ativo FOREIGN KEY (AtivoId) REFERENCES dbo.Ativo (Id) ON DELETE CASCADE
);
GO
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'IX_OperacaoAtivo_Ativo_Data')
    CREATE INDEX IX_OperacaoAtivo_Ativo_Data ON dbo.OperacaoAtivo (AtivoId, Data);
GO

/* Proventos (rendimentos/dividendos) por cota, pela data com. */
IF OBJECT_ID(N'dbo.ProventoAtivo', N'U') IS NULL
CREATE TABLE dbo.ProventoAtivo
(
    Id              INT IDENTITY(1,1) NOT NULL CONSTRAINT PK_ProventoAtivo PRIMARY KEY,
    AtivoId         INT               NOT NULL,
    DataCom         DATE              NOT NULL,
    ValorPorCota    DECIMAL(18,8)     NOT NULL,
    CONSTRAINT UQ_ProventoAtivo UNIQUE (AtivoId, DataCom),
    CONSTRAINT FK_ProventoAtivo_Ativo FOREIGN KEY (AtivoId) REFERENCES dbo.Ativo (Id) ON DELETE CASCADE
);
GO

/* Taxa Selic diária (% ao dia), série 11 do SGS/Banco Central — cache local. */
IF OBJECT_ID(N'dbo.TaxaSelicDiaria', N'U') IS NULL
CREATE TABLE dbo.TaxaSelicDiaria
(
    Data    DATE           NOT NULL CONSTRAINT PK_TaxaSelicDiaria PRIMARY KEY,
    Taxa    DECIMAL(12,8)  NOT NULL
);
GO

/* ------------------------------------------------------------------ */
/* Carteira (v6): origem das operações, segmento dos FIIs, histórico   */
/* mensal de cotações e de índices (IFIX)                              */
/* OperacaoAtivo.Origem: 1 Manual, 2 Open Finance, 3 Extrato           */
/* ------------------------------------------------------------------ */
IF COL_LENGTH(N'dbo.OperacaoAtivo', N'Origem') IS NULL
    ALTER TABLE dbo.OperacaoAtivo ADD Origem TINYINT NOT NULL CONSTRAINT DF_OperacaoAtivo_Origem DEFAULT 1;
GO
IF COL_LENGTH(N'dbo.OperacaoAtivo', N'PluggyTransacaoId') IS NULL
    ALTER TABLE dbo.OperacaoAtivo ADD PluggyTransacaoId NVARCHAR(100) NULL;
GO
IF COL_LENGTH(N'dbo.OperacaoAtivo', N'TransacaoId') IS NULL
    ALTER TABLE dbo.OperacaoAtivo ADD TransacaoId BIGINT NULL;
GO
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'UX_OperacaoAtivo_PluggyTransacaoId')
    CREATE UNIQUE INDEX UX_OperacaoAtivo_PluggyTransacaoId ON dbo.OperacaoAtivo (PluggyTransacaoId) WHERE PluggyTransacaoId IS NOT NULL;
GO
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'UX_OperacaoAtivo_TransacaoId')
    CREATE UNIQUE INDEX UX_OperacaoAtivo_TransacaoId ON dbo.OperacaoAtivo (TransacaoId) WHERE TransacaoId IS NOT NULL;
GO
IF NOT EXISTS (SELECT 1 FROM sys.check_constraints WHERE name = N'CK_OperacaoAtivo_Origem')
    ALTER TABLE dbo.OperacaoAtivo ADD CONSTRAINT CK_OperacaoAtivo_Origem CHECK (Origem BETWEEN 1 AND 3);
GO

IF COL_LENGTH(N'dbo.Ativo', N'Segmento') IS NULL
    ALTER TABLE dbo.Ativo ADD Segmento NVARCHAR(40) NULL;
GO

/* Fechamento mensal de cada ativo (para evolução do patrimônio e rentabilidade). Mes = 1º dia do mês. */
IF OBJECT_ID(N'dbo.PrecoMensalAtivo', N'U') IS NULL
CREATE TABLE dbo.PrecoMensalAtivo
(
    AtivoId     INT            NOT NULL,
    Mes         DATE           NOT NULL,
    Fechamento  DECIMAL(18,4)  NOT NULL,
    CONSTRAINT PK_PrecoMensalAtivo PRIMARY KEY (AtivoId, Mes),
    CONSTRAINT FK_PrecoMensalAtivo_Ativo FOREIGN KEY (AtivoId) REFERENCES dbo.Ativo (Id) ON DELETE CASCADE
);
GO

/* Fechamento mensal de índices de referência (ex.: IFIX). */
IF OBJECT_ID(N'dbo.IndiceMensal', N'U') IS NULL
CREATE TABLE dbo.IndiceMensal
(
    Codigo      NVARCHAR(12)   NOT NULL,
    Mes         DATE           NOT NULL,
    Fechamento  DECIMAL(18,4)  NOT NULL,
    CONSTRAINT PK_IndiceMensal PRIMARY KEY (Codigo, Mes)
);
GO

/* ------------------------------------------------------------------ */
/* Transações (v7): marcação de gasto de emergência                    */
/* ------------------------------------------------------------------ */
IF COL_LENGTH(N'dbo.Transacao', N'Emergencia') IS NULL
    ALTER TABLE dbo.Transacao ADD Emergencia BIT NOT NULL CONSTRAINT DF_Transacao_Emergencia DEFAULT 0;
GO
