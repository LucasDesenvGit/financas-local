/* =====================================================================
   Banco fictício dos testes de contrato (#72, ADR 0049).

   DADOS INVENTADOS: nada aqui é de uma pessoa de verdade (o repositório é
   compartilhado). Montado por rodar-dotnet.sh depois de banco/01-estrutura.sql,
   das colunas do 2-criar-banco.sh e de banco/02-dados-iniciais.sql.

   Datas: o contrato roda com o relógio em 2026-10-15 01:00 UTC, que em
   Brasília ainda é 14/10/2026, 22h. Por isso nada aqui passa de 14/10/2026
   (exceto vencimentos e metas, que são futuros de propósito), e os carimbos
   (CriadoEm, AtualizadoEm) são anteriores ao início do relógio: o que for
   gravado durante o contrato aparece como "<relógio +N min>" nos esperados.

   Formato do EF Core (internal/tipos): decimal em TEXT ("-5000.0", "12.5"),
   data em "AAAA-MM-DD", data e hora em "AAAA-MM-DD hh:mm:ss[.fffffff]" (UTC),
   GUID em maiúsculas, booleano e enum em INTEGER.

   Mudou este arquivo? Regrave os esperados de todos os assuntos
   (rodar-dotnet.sh --atualizar) e confira a diferença no PR.
   ===================================================================== */

/* As datas que o 02-dados-iniciais.sql grava com a hora da montagem ('now') ficam fixas: senão, mudam a cada execução. */
UPDATE Configuracao SET AtualizadoEm = '2025-10-01 12:00:00';
UPDATE RegraCategorizacao SET CriadoEm = '2025-10-01 12:00:00';

/* ------------------------------------------------------------------ Configuração */
/* Parâmetros (ParametroService): gravados com valores fora da faixa e inválidos, para a leitura limitar ou cair no padrão. */
UPDATE Configuracao SET Valor = 'True' WHERE Chave = 'Sincronizacao.Habilitada';  -- bool.TryParse ignora maiúsculas
INSERT INTO Configuracao (Chave, Valor, Protegido, AtualizadoEm) VALUES
    ('Painel.DiasAvisoConsentimento',       '500',  0, '2025-10-01 12:00:00'),  -- acima do máximo (120)
    ('Metas.MargemSeguranca',               'dez',  0, '2025-10-01 12:00:00'),  -- não é número: vale o padrão
    ('Transacoes.MinimoLetrasDescricao',    ' 4 ',  0, '2025-10-01 12:00:00'),  -- int.TryParse aceita espaços
    ('Orcamento.CartaoAlerta',              '0',    0, '2025-10-01 12:00:00'),  -- abaixo do mínimo (10)
    ('Pluggy.ClientId',                     'cliente-ficticio-0001', 0, '2025-10-01 12:00:00'),
    ('Sincronizacao.UltimaExecucao',        '2026-10-14T09:00:00.0000000Z', 0, '2026-10-14 09:00:00'),
    -- Câmbio já atualizado depois do fechamento de hoje: o CambioHostedService não tenta a internet.
    ('Cambio.UltimaAtualizacao',            '2026-10-15T00:30:00.0000000Z', 0, '2026-10-15 00:30:00');

/* ------------------------------------------------------------------ Categorias */
INSERT INTO Categoria (Nome, Tipo, Cor, Icone, CategoriaPaiId, Ativa, Natureza) VALUES
    ('Restaurantes', 1, '#f8c09a', 'restaurante', (SELECT Id FROM Categoria WHERE Nome = 'Alimentação'), 1, 2),
    ('Padaria',      1, '#f8c09a', 'restaurante', (SELECT Id FROM Categoria WHERE Nome = 'Alimentação'), 1, 2),
    ('Academia',     1, '#e7b3ea', 'pessoal', NULL, 0, 2);  -- desativada

INSERT INTO RegraCategorizacao (Texto, TipoCorrespondencia, CategoriaId, Prioridade, Ativa, CriadoEm) VALUES
    ('SUPERMERCADO',    1, (SELECT Id FROM Categoria WHERE Nome = 'Mercado'), 100, 1, '2025-11-02 10:00:00'),
    ('STREAMFLIX',      1, (SELECT Id FROM Categoria WHERE Nome = 'Serviços e assinaturas'), 100, 1, '2025-11-02 10:00:00'),
    ('APP TRANSPORTE',  2, (SELECT Id FROM Categoria WHERE Nome = 'Transporte'), 90, 1, '2025-11-02 10:00:00'),
    ('PADARIA',         1, (SELECT Id FROM Categoria WHERE Nome = 'Padaria'), 100, 0, '2025-11-02 10:00:00');

INSERT INTO RegraDescricao (Texto, TipoCorrespondencia, NovaDescricao, ValorMinimo, ValorMaximo, Ativa, CriadoEm) VALUES
    ('PIX ENVIADO IMOBILIARIA FICTICIA', 3, 'Aluguel', '1800.0', '1900.0', 1, '2025-11-02 10:00:00'),
    ('DEBITO AUTOMATICO ENERGIA', 2, 'Conta de luz', NULL, NULL, 1, '2025-11-02 10:00:00');

INSERT INTO CategoriaPluggy (PluggyCategoriaId, Descricao, PaiPluggyId, PaiDescricao, CategoriaId) VALUES
    ('05000000', 'Groceries', NULL, NULL, (SELECT Id FROM Categoria WHERE Nome = 'Mercado')),
    ('19050000', 'Restaurants', '19000000', 'Food and drinks', (SELECT Id FROM Categoria WHERE Nome = 'Restaurantes')),
    ('99999999', 'Unknown', NULL, NULL, NULL);

/* ------------------------------------------------------------------ Open Finance e contas */
INSERT INTO ConexaoBancaria (Id, PluggyItemId, NomeInstituicao, CorPrimaria, Status, StatusExecucao, UltimaAtualizacaoInstituicao,
    ConsentimentoExpiraEm, UltimaSincronizacaoEm, UltimoErro, Ativa, CriadoEm) VALUES
    (1, 'item-ficticio-0001', 'Banco Fictício', '#820ad1', 'UPDATED', 'SUCCESS', '2026-10-14 08:55:00',
     '2026-11-05 00:00:00', '2026-10-14 09:00:00', NULL, 1, '2025-10-20 18:00:00'),
    (2, 'item-ficticio-0002', 'Banco Desconectado', NULL, 'LOGIN_ERROR', 'ERROR', '2026-03-01 08:00:00',
     '2026-03-01 00:00:00', '2026-03-01 09:00:00', 'Consentimento expirado.', 0, '2025-10-20 18:05:00');

INSERT INTO SincronizacaoLog (ConexaoBancariaId, Origem, IniciadoEm, FinalizadoEm, Sucesso, TransacoesNovas, TransacoesAtualizadas, Mensagem) VALUES
    (1, 'Automatica', '2026-10-13 09:00:00', '2026-10-13 09:00:41.25', 1, 12, 3, NULL),
    (1, 'Manual',     '2026-10-14 09:00:00', '2026-10-14 09:00:37.5',  1, 4, 0, NULL),
    (2, 'Automatica', '2026-03-01 09:00:00', '2026-03-01 09:00:05',    0, 0, 0, 'Consentimento expirado.');

INSERT INTO Conta (Id, ConexaoBancariaId, PluggyAccountId, Nome, Tipo, Origem, Numero, Moeda, SaldoInicial, Saldo, LimiteCredito,
    LimiteDisponivel, DiaFechamento, DiaVencimento, Cor, IncluirNoSaldo, Arquivada, Banco, ContaVinculadaId, CriadoEm, AtualizadoEm) VALUES
    (1, 1, 'acc-cc-0001', 'Conta Corrente Fictícia', 1, 2, '0001-9', 'BRL', '0.0', '28028.29', NULL, NULL, NULL, NULL, '#820ad1', 1, 0, NULL, NULL, '2025-10-20 18:10:00', '2026-10-14 09:00:00'),
    (2, 1, 'acc-cartao-0001', 'Cartão Fictício Platinum', 3, 2, '**** 4321', 'BRL', '0.0', '-299.9', '12000.0', '11700.1', 3, 10, '#4c1d95', 1, 0, NULL, 1, '2025-10-20 18:10:00', '2026-10-14 09:00:00'),
    (3, NULL, NULL, 'Poupança', 2, 1, NULL, 'BRL', '5000.0', '0.0', NULL, NULL, NULL, NULL, '#0f766e', 1, 0, 'caixa', NULL, '2025-10-20 18:10:00', '2026-10-14 09:00:00'),
    (4, NULL, NULL, 'Carteira (dinheiro)', 5, 1, NULL, 'BRL', '150.0', '0.0', NULL, NULL, NULL, NULL, '#ca8a04', 1, 0, NULL, NULL, '2025-10-20 18:10:00', '2026-10-14 09:00:00'),
    (5, NULL, NULL, 'Conta Antiga', 1, 1, NULL, 'BRL', '0.0', '0.0', NULL, NULL, NULL, NULL, '#64748b', 0, 1, NULL, NULL, '2025-10-20 18:10:00', '2026-10-14 09:00:00'),
    (6, NULL, NULL, 'Conta em Dólar', 1, 1, NULL, 'USD', '1200.0', '0.0', NULL, NULL, NULL, NULL, '#2563eb', 1, 0, NULL, NULL, '2025-10-20 18:10:00', '2026-10-14 09:00:00');

UPDATE Conta SET SaldoReservado = '800.0', ReservasInformadas = '[{"nome":"Caixinha viagem","valor":800.0}]' WHERE Id = 1;

/* Faturas do cartão: vencimento no dia 10, fechamento no dia 3. As já vencidas estão pagas. */
INSERT INTO FaturaCartao (ContaId, PluggyBillId, DataVencimento, DataFechamento, ValorTotal, ValorMinimo, ValorPago, AtualizadoEm) VALUES
    (2, 'bill-202512', '2025-12-10', '2025-12-03', '153.25', '22.99', '153.25', '2026-10-14 09:00:00'),
    (2, 'bill-202601', '2026-01-10', '2026-01-03', '155.35', '23.3', '155.35', '2026-10-14 09:00:00'),
    (2, 'bill-202602', '2026-02-10', '2026-02-03', '457.35', '68.6', '457.35', '2026-10-14 09:00:00'),
    (2, 'bill-202603', '2026-03-10', '2026-03-03', '459.45', '68.92', '459.45', '2026-10-14 09:00:00'),
    (2, 'bill-202604', '2026-04-10', '2026-04-03', '461.55', '69.23', '461.55', '2026-10-14 09:00:00'),
    (2, 'bill-202605', '2026-05-10', '2026-05-03', '463.65', '69.55', '463.65', '2026-10-14 09:00:00'),
    (2, 'bill-202606', '2026-06-10', '2026-06-03', '465.75', '69.86', '465.75', '2026-10-14 09:00:00'),
    (2, 'bill-202607', '2026-07-10', '2026-07-03', '467.85', '70.18', '467.85', '2026-10-14 09:00:00'),
    (2, 'bill-202608', '2026-08-10', '2026-08-03', '469.95', '70.49', '469.95', '2026-10-14 09:00:00'),
    (2, 'bill-202609', '2026-09-10', '2026-09-03', '472.05', '70.81', '472.05', '2026-10-14 09:00:00'),
    (2, 'bill-202610', '2026-10-10', '2026-10-03', '474.15', '71.12', '474.15', '2026-10-14 09:00:00'),
    (2, 'bill-202611', '2026-11-10', '2026-11-03', '299.9', '44.98', NULL, '2026-10-14 09:00:00');

/* Transações: 12 meses (nov/2025 a out/2026), com transferências entre contas (mesmo TransferenciaGrupo), pagamento de
   fatura, compra parcelada, estorno no cartão, gasto de emergência, uma sem categoria, uma pendente e um centavo. */
INSERT INTO Transacao (ContaId, PluggyTransactionId, Data, Descricao, DescricaoOriginal, Estabelecimento, Valor, Tipo, Status, CategoriaId,
    CategoriaManual, FaturaCartaoId, ParcelaNumero, ParcelaTotal, IgnorarRelatorios, Emergencia, DescricaoRevisada, MeioPagamento,
    MesmaTitularidade, RegraDescricaoId, DescricaoManual, TransferenciaGrupo, Observacao, CriadoEm, AtualizadoEm) VALUES
    (5, NULL, '2025-11-03', 'Lançamento da conta antiga', NULL, NULL, '-10.0', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Outros gastos'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2025-11-03 09:00:00', '2025-11-03 09:00:00'),
    (1, 'tx-202511-sal', '2025-11-05', 'SALARIO EMPRESA FICTICIA LTDA', 'SALARIO EMPRESA FICTICIA LTDA', NULL, '6500.0', 2, 1, (SELECT Id FROM Categoria WHERE Nome = 'Salário'), 0, NULL, NULL, NULL, 0, 0, 0, 'TED', NULL, NULL, 0, NULL, NULL, '2025-11-05 09:00:00', '2025-11-05 09:00:00'),
    (1, 'tx-202511-poup', '2025-11-06', 'TRANSFERENCIA PARA POUPANCA', 'TRANSFERENCIA PARA POUPANCA', NULL, '-500.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transferências'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, 1, NULL, 0, '00000000-0000-4000-8000-000000000001', NULL, '2025-11-06 09:00:00', '2025-11-06 09:00:00'),
    (3, NULL, '2025-11-06', 'Transferência da conta corrente', NULL, NULL, '500.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transferências'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, '00000000-0000-4000-8000-000000000001', NULL, '2025-11-06 09:00:00', '2025-11-06 09:00:00'),
    (1, 'tx-202511-cdb', '2025-11-07', 'APLICACAO CDB BANCO FICTICIO', 'APLICACAO CDB BANCO FICTICIO', NULL, '-1000.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Investimentos'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2025-11-07 09:00:00', '2025-11-07 09:00:00'),
    (1, 'tx-202511-alug', '2025-11-08', 'Aluguel', 'PIX ENVIADO IMOBILIARIA FICTICIA', NULL, '-1850.0', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Moradia'), 0, NULL, NULL, NULL, 0, 0, 0, 'PIX', NULL, 1, 0, NULL, NULL, '2025-11-08 09:00:00', '2025-11-08 09:00:00'),
    (1, 'tx-202511-luz', '2025-11-12', 'Conta de luz', 'DEBITO AUTOMATICO ENERGIA FICTICIA', NULL, '-150.0', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Moradia'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, 2, 0, NULL, NULL, '2025-11-12 09:00:00', '2025-11-12 09:00:00'),
    (1, 'tx-202511-merc', '2025-11-15', 'SUPERMERCADO EXEMPLO', 'SUPERMERCADO EXEMPLO', 'Supermercado Exemplo', '-420.5', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Mercado'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2025-11-15 09:00:00', '2025-11-15 09:00:00'),
    (2, 'tx-202511-c18', '2025-11-18', 'RESTAURANTE SABOR FICTICIO', 'RESTAURANTE SABOR FICTICIO', 'Restaurante Sabor', '-85.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Restaurantes'), 0, 1, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2025-11-18 09:00:00', '2025-11-18 09:00:00'),
    (2, 'tx-202511-c20', '2025-11-20', 'ASSINATURA STREAMFLIX', 'ASSINATURA STREAMFLIX', NULL, '-39.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Serviços e assinaturas'), 0, 1, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2025-11-20 09:00:00', '2025-11-20 09:00:00'),
    (2, 'tx-202511-c22', '2025-11-22', 'APP TRANSPORTE FICTICIO *VIAGEM', 'APP TRANSPORTE FICTICIO *VIAGEM', NULL, '-27.45', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transporte'), 0, 1, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2025-11-22 09:00:00', '2025-11-22 09:00:00'),
    (1, 'tx-202511-tar', '2025-11-25', 'TARIFA PACOTE DE SERVICOS', 'TARIFA PACOTE DE SERVICOS', NULL, '-32.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Tarifas bancárias'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2025-11-25 09:00:00', '2025-11-25 09:00:00'),
    (1, 'tx-202512-sal', '2025-12-05', 'SALARIO EMPRESA FICTICIA LTDA', 'SALARIO EMPRESA FICTICIA LTDA', NULL, '6500.0', 2, 1, (SELECT Id FROM Categoria WHERE Nome = 'Salário'), 0, NULL, NULL, NULL, 0, 0, 0, 'TED', NULL, NULL, 0, NULL, NULL, '2025-12-05 09:00:00', '2025-12-05 09:00:00'),
    (1, 'tx-202512-poup', '2025-12-06', 'TRANSFERENCIA PARA POUPANCA', 'TRANSFERENCIA PARA POUPANCA', NULL, '-500.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transferências'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, 1, NULL, 0, '00000000-0000-4000-8000-000000000002', NULL, '2025-12-06 09:00:00', '2025-12-06 09:00:00'),
    (3, NULL, '2025-12-06', 'Transferência da conta corrente', NULL, NULL, '500.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transferências'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, '00000000-0000-4000-8000-000000000002', NULL, '2025-12-06 09:00:00', '2025-12-06 09:00:00'),
    (1, 'tx-202512-cdb', '2025-12-07', 'APLICACAO CDB BANCO FICTICIO', 'APLICACAO CDB BANCO FICTICIO', NULL, '-1000.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Investimentos'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2025-12-07 09:00:00', '2025-12-07 09:00:00'),
    (1, 'tx-202512-alug', '2025-12-08', 'Aluguel', 'PIX ENVIADO IMOBILIARIA FICTICIA', NULL, '-1850.0', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Moradia'), 0, NULL, NULL, NULL, 0, 0, 0, 'PIX', NULL, 1, 0, NULL, NULL, '2025-12-08 09:00:00', '2025-12-08 09:00:00'),
    (1, 'tx-202512-pgfat', '2025-12-10', 'PAGAMENTO FATURA CARTAO', 'PAGAMENTO FATURA CARTAO', NULL, '-153.25', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Pagamento de fatura'), 0, NULL, NULL, NULL, 0, 0, 0, 'PAGAMENTO_FATURA', 1, NULL, 0, '00000000-0000-4000-8000-000000000003', NULL, '2025-12-10 09:00:00', '2025-12-10 09:00:00'),
    (2, 'tx-202512-pgrec', '2025-12-10', 'PAGAMENTO RECEBIDO', 'PAGAMENTO RECEBIDO', NULL, '153.25', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Pagamento de fatura'), 0, 1, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, '00000000-0000-4000-8000-000000000003', NULL, '2025-12-10 09:00:00', '2025-12-10 09:00:00'),
    (1, 'tx-202512-luz', '2025-12-12', 'Conta de luz', 'DEBITO AUTOMATICO ENERGIA FICTICIA', NULL, '-157.35', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Moradia'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, 2, 0, NULL, NULL, '2025-12-12 09:00:00', '2025-12-12 09:00:00'),
    (1, 'tx-202512-merc', '2025-12-15', 'SUPERMERCADO EXEMPLO', 'SUPERMERCADO EXEMPLO', 'Supermercado Exemplo', '-433.77', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Mercado'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2025-12-15 09:00:00', '2025-12-15 09:00:00'),
    (2, 'tx-202512-c18', '2025-12-18', 'RESTAURANTE SABOR FICTICIO', 'RESTAURANTE SABOR FICTICIO', 'Restaurante Sabor', '-88.0', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Restaurantes'), 0, 2, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2025-12-18 09:00:00', '2025-12-18 09:00:00'),
    (2, 'tx-202512-c20', '2025-12-20', 'ASSINATURA STREAMFLIX', 'ASSINATURA STREAMFLIX', NULL, '-39.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Serviços e assinaturas'), 0, 2, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2025-12-20 09:00:00', '2025-12-20 09:00:00'),
    (2, 'tx-202512-c22', '2025-12-22', 'APP TRANSPORTE FICTICIO *VIAGEM', 'APP TRANSPORTE FICTICIO *VIAGEM', NULL, '-27.45', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transporte'), 0, 2, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2025-12-22 09:00:00', '2025-12-22 09:00:00'),
    (1, 'tx-202512-tar', '2025-12-25', 'TARIFA PACOTE DE SERVICOS', 'TARIFA PACOTE DE SERVICOS', NULL, '-32.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Tarifas bancárias'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2025-12-25 09:00:00', '2025-12-25 09:00:00'),
    (2, 'tx-202601-movel', '2026-01-04', 'LOJA DE MOVEIS FICTICIA 1/10', 'LOJA DE MOVEIS FICTICIA 1/10', NULL, '-299.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Compras'), 0, 3, 1, 10, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-01-04 09:00:00', '2026-01-04 09:00:00'),
    (1, 'tx-202601-sal', '2026-01-05', 'SALARIO EMPRESA FICTICIA LTDA', 'SALARIO EMPRESA FICTICIA LTDA', NULL, '6500.0', 2, 1, (SELECT Id FROM Categoria WHERE Nome = 'Salário'), 0, NULL, NULL, NULL, 0, 0, 0, 'TED', NULL, NULL, 0, NULL, NULL, '2026-01-05 09:00:00', '2026-01-05 09:00:00'),
    (1, 'tx-202601-poup', '2026-01-06', 'TRANSFERENCIA PARA POUPANCA', 'TRANSFERENCIA PARA POUPANCA', NULL, '-500.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transferências'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, 1, NULL, 0, '00000000-0000-4000-8000-000000000004', NULL, '2026-01-06 09:00:00', '2026-01-06 09:00:00'),
    (3, NULL, '2026-01-06', 'Transferência da conta corrente', NULL, NULL, '500.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transferências'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, '00000000-0000-4000-8000-000000000004', NULL, '2026-01-06 09:00:00', '2026-01-06 09:00:00'),
    (1, 'tx-202601-cdb', '2026-01-07', 'APLICACAO CDB BANCO FICTICIO', 'APLICACAO CDB BANCO FICTICIO', NULL, '-1000.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Investimentos'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-01-07 09:00:00', '2026-01-07 09:00:00'),
    (1, 'tx-202601-alug', '2026-01-08', 'Aluguel', 'PIX ENVIADO IMOBILIARIA FICTICIA', NULL, '-1850.0', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Moradia'), 0, NULL, NULL, NULL, 0, 0, 0, 'PIX', NULL, 1, 0, NULL, NULL, '2026-01-08 09:00:00', '2026-01-08 09:00:00'),
    (1, 'tx-202601-pgfat', '2026-01-10', 'PAGAMENTO FATURA CARTAO', 'PAGAMENTO FATURA CARTAO', NULL, '-155.35', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Pagamento de fatura'), 0, NULL, NULL, NULL, 0, 0, 0, 'PAGAMENTO_FATURA', 1, NULL, 0, '00000000-0000-4000-8000-000000000005', NULL, '2026-01-10 09:00:00', '2026-01-10 09:00:00'),
    (2, 'tx-202601-pgrec', '2026-01-10', 'PAGAMENTO RECEBIDO', 'PAGAMENTO RECEBIDO', NULL, '155.35', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Pagamento de fatura'), 0, 2, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, '00000000-0000-4000-8000-000000000005', NULL, '2026-01-10 09:00:00', '2026-01-10 09:00:00'),
    (1, 'tx-202601-luz', '2026-01-12', 'Conta de luz', 'DEBITO AUTOMATICO ENERGIA FICTICIA', NULL, '-164.7', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Moradia'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, 2, 0, NULL, NULL, '2026-01-12 09:00:00', '2026-01-12 09:00:00'),
    (1, 'tx-202601-merc', '2026-01-15', 'SUPERMERCADO EXEMPLO', 'SUPERMERCADO EXEMPLO', 'Supermercado Exemplo', '-447.04', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Mercado'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-01-15 09:00:00', '2026-01-15 09:00:00'),
    (2, 'tx-202601-c18', '2026-01-18', 'RESTAURANTE SABOR FICTICIO', 'RESTAURANTE SABOR FICTICIO', 'Restaurante Sabor', '-90.1', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Restaurantes'), 0, 3, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2026-01-18 09:00:00', '2026-01-18 09:00:00'),
    (2, 'tx-202601-c20', '2026-01-20', 'ASSINATURA STREAMFLIX', 'ASSINATURA STREAMFLIX', NULL, '-39.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Serviços e assinaturas'), 0, 3, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2026-01-20 09:00:00', '2026-01-20 09:00:00'),
    (2, 'tx-202601-c22', '2026-01-22', 'APP TRANSPORTE FICTICIO *VIAGEM', 'APP TRANSPORTE FICTICIO *VIAGEM', NULL, '-27.45', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transporte'), 0, 3, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2026-01-22 09:00:00', '2026-01-22 09:00:00'),
    (1, 'tx-202601-tar', '2026-01-25', 'TARIFA PACOTE DE SERVICOS', 'TARIFA PACOTE DE SERVICOS', NULL, '-32.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Tarifas bancárias'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-01-25 09:00:00', '2026-01-25 09:00:00'),
    (2, 'tx-202602-movel', '2026-02-04', 'LOJA DE MOVEIS FICTICIA 2/10', 'LOJA DE MOVEIS FICTICIA 2/10', NULL, '-299.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Compras'), 0, 4, 2, 10, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-02-04 09:00:00', '2026-02-04 09:00:00'),
    (1, 'tx-202602-sal', '2026-02-05', 'SALARIO EMPRESA FICTICIA LTDA', 'SALARIO EMPRESA FICTICIA LTDA', NULL, '6500.0', 2, 1, (SELECT Id FROM Categoria WHERE Nome = 'Salário'), 0, NULL, NULL, NULL, 0, 0, 0, 'TED', NULL, NULL, 0, NULL, NULL, '2026-02-05 09:00:00', '2026-02-05 09:00:00'),
    (1, 'tx-202602-poup', '2026-02-06', 'TRANSFERENCIA PARA POUPANCA', 'TRANSFERENCIA PARA POUPANCA', NULL, '-500.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transferências'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, 1, NULL, 0, '00000000-0000-4000-8000-000000000006', NULL, '2026-02-06 09:00:00', '2026-02-06 09:00:00'),
    (3, NULL, '2026-02-06', 'Transferência da conta corrente', NULL, NULL, '500.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transferências'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, '00000000-0000-4000-8000-000000000006', NULL, '2026-02-06 09:00:00', '2026-02-06 09:00:00'),
    (1, 'tx-202602-cdb', '2026-02-07', 'APLICACAO CDB BANCO FICTICIO', 'APLICACAO CDB BANCO FICTICIO', NULL, '-1000.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Investimentos'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-02-07 09:00:00', '2026-02-07 09:00:00'),
    (1, 'tx-202602-alug', '2026-02-08', 'Aluguel', 'PIX ENVIADO IMOBILIARIA FICTICIA', NULL, '-1850.0', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Moradia'), 0, NULL, NULL, NULL, 0, 0, 0, 'PIX', NULL, 1, 0, NULL, NULL, '2026-02-08 09:00:00', '2026-02-08 09:00:00'),
    (1, 'tx-202602-pgfat', '2026-02-10', 'PAGAMENTO FATURA CARTAO', 'PAGAMENTO FATURA CARTAO', NULL, '-457.35', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Pagamento de fatura'), 0, NULL, NULL, NULL, 0, 0, 0, 'PAGAMENTO_FATURA', 1, NULL, 0, '00000000-0000-4000-8000-000000000007', NULL, '2026-02-10 09:00:00', '2026-02-10 09:00:00'),
    (2, 'tx-202602-pgrec', '2026-02-10', 'PAGAMENTO RECEBIDO', 'PAGAMENTO RECEBIDO', NULL, '457.35', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Pagamento de fatura'), 0, 3, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, '00000000-0000-4000-8000-000000000007', NULL, '2026-02-10 09:00:00', '2026-02-10 09:00:00'),
    (1, 'tx-202602-luz', '2026-02-12', 'Conta de luz', 'DEBITO AUTOMATICO ENERGIA FICTICIA', NULL, '-172.05', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Moradia'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, 2, 0, NULL, NULL, '2026-02-12 09:00:00', '2026-02-12 09:00:00'),
    (1, 'tx-202602-merc', '2026-02-15', 'SUPERMERCADO EXEMPLO', 'SUPERMERCADO EXEMPLO', 'Supermercado Exemplo', '-460.31', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Mercado'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-02-15 09:00:00', '2026-02-15 09:00:00'),
    (2, 'tx-202602-c18', '2026-02-18', 'RESTAURANTE SABOR FICTICIO', 'RESTAURANTE SABOR FICTICIO', 'Restaurante Sabor', '-92.2', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Restaurantes'), 0, 4, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2026-02-18 09:00:00', '2026-02-18 09:00:00'),
    (2, 'tx-202602-c20', '2026-02-20', 'ASSINATURA STREAMFLIX', 'ASSINATURA STREAMFLIX', NULL, '-39.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Serviços e assinaturas'), 0, 4, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2026-02-20 09:00:00', '2026-02-20 09:00:00'),
    (2, 'tx-202602-c22', '2026-02-22', 'APP TRANSPORTE FICTICIO *VIAGEM', 'APP TRANSPORTE FICTICIO *VIAGEM', NULL, '-27.45', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transporte'), 0, 4, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2026-02-22 09:00:00', '2026-02-22 09:00:00'),
    (1, 'tx-202602-tar', '2026-02-25', 'TARIFA PACOTE DE SERVICOS', 'TARIFA PACOTE DE SERVICOS', NULL, '-32.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Tarifas bancárias'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-02-25 09:00:00', '2026-02-25 09:00:00'),
    (2, 'tx-202603-movel', '2026-03-04', 'LOJA DE MOVEIS FICTICIA 3/10', 'LOJA DE MOVEIS FICTICIA 3/10', NULL, '-299.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Compras'), 0, 5, 3, 10, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-03-04 09:00:00', '2026-03-04 09:00:00'),
    (1, 'tx-202603-sal', '2026-03-05', 'SALARIO EMPRESA FICTICIA LTDA', 'SALARIO EMPRESA FICTICIA LTDA', NULL, '6500.0', 2, 1, (SELECT Id FROM Categoria WHERE Nome = 'Salário'), 0, NULL, NULL, NULL, 0, 0, 0, 'TED', NULL, NULL, 0, NULL, NULL, '2026-03-05 09:00:00', '2026-03-05 09:00:00'),
    (1, 'tx-202603-poup', '2026-03-06', 'TRANSFERENCIA PARA POUPANCA', 'TRANSFERENCIA PARA POUPANCA', NULL, '-500.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transferências'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, 1, NULL, 0, '00000000-0000-4000-8000-000000000008', NULL, '2026-03-06 09:00:00', '2026-03-06 09:00:00'),
    (3, NULL, '2026-03-06', 'Transferência da conta corrente', NULL, NULL, '500.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transferências'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, '00000000-0000-4000-8000-000000000008', NULL, '2026-03-06 09:00:00', '2026-03-06 09:00:00'),
    (1, 'tx-202603-cdb', '2026-03-07', 'APLICACAO CDB BANCO FICTICIO', 'APLICACAO CDB BANCO FICTICIO', NULL, '-1000.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Investimentos'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-03-07 09:00:00', '2026-03-07 09:00:00'),
    (1, 'tx-202603-alug', '2026-03-08', 'Aluguel', 'PIX ENVIADO IMOBILIARIA FICTICIA', NULL, '-1850.0', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Moradia'), 0, NULL, NULL, NULL, 0, 0, 0, 'PIX', NULL, 1, 0, NULL, NULL, '2026-03-08 09:00:00', '2026-03-08 09:00:00'),
    (1, 'tx-202603-pgfat', '2026-03-10', 'PAGAMENTO FATURA CARTAO', 'PAGAMENTO FATURA CARTAO', NULL, '-459.45', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Pagamento de fatura'), 0, NULL, NULL, NULL, 0, 0, 0, 'PAGAMENTO_FATURA', 1, NULL, 0, '00000000-0000-4000-8000-000000000009', NULL, '2026-03-10 09:00:00', '2026-03-10 09:00:00'),
    (2, 'tx-202603-pgrec', '2026-03-10', 'PAGAMENTO RECEBIDO', 'PAGAMENTO RECEBIDO', NULL, '459.45', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Pagamento de fatura'), 0, 4, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, '00000000-0000-4000-8000-000000000009', NULL, '2026-03-10 09:00:00', '2026-03-10 09:00:00'),
    (1, 'tx-202603-luz', '2026-03-12', 'Conta de luz', 'DEBITO AUTOMATICO ENERGIA FICTICIA', NULL, '-179.4', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Moradia'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, 2, 0, NULL, NULL, '2026-03-12 09:00:00', '2026-03-12 09:00:00'),
    (1, 'tx-202603-merc', '2026-03-15', 'SUPERMERCADO EXEMPLO', 'SUPERMERCADO EXEMPLO', 'Supermercado Exemplo', '-473.58', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Mercado'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-03-15 09:00:00', '2026-03-15 09:00:00'),
    (2, 'tx-202603-c18', '2026-03-18', 'RESTAURANTE SABOR FICTICIO', 'RESTAURANTE SABOR FICTICIO', 'Restaurante Sabor', '-94.3', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Restaurantes'), 0, 5, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2026-03-18 09:00:00', '2026-03-18 09:00:00'),
    (2, 'tx-202603-c20', '2026-03-20', 'ASSINATURA STREAMFLIX', 'ASSINATURA STREAMFLIX', NULL, '-39.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Serviços e assinaturas'), 0, 5, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2026-03-20 09:00:00', '2026-03-20 09:00:00'),
    (2, 'tx-202603-c22', '2026-03-22', 'APP TRANSPORTE FICTICIO *VIAGEM', 'APP TRANSPORTE FICTICIO *VIAGEM', NULL, '-27.45', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transporte'), 0, 5, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2026-03-22 09:00:00', '2026-03-22 09:00:00'),
    (1, 'tx-202603-tar', '2026-03-25', 'TARIFA PACOTE DE SERVICOS', 'TARIFA PACOTE DE SERVICOS', NULL, '-32.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Tarifas bancárias'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-03-25 09:00:00', '2026-03-25 09:00:00'),
    (2, 'tx-202604-movel', '2026-04-04', 'LOJA DE MOVEIS FICTICIA 4/10', 'LOJA DE MOVEIS FICTICIA 4/10', NULL, '-299.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Compras'), 0, 6, 4, 10, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-04-04 09:00:00', '2026-04-04 09:00:00'),
    (1, 'tx-202604-sal', '2026-04-05', 'SALARIO EMPRESA FICTICIA LTDA', 'SALARIO EMPRESA FICTICIA LTDA', NULL, '6500.0', 2, 1, (SELECT Id FROM Categoria WHERE Nome = 'Salário'), 0, NULL, NULL, NULL, 0, 0, 0, 'TED', NULL, NULL, 0, NULL, NULL, '2026-04-05 09:00:00', '2026-04-05 09:00:00'),
    (1, 'tx-202604-poup', '2026-04-06', 'TRANSFERENCIA PARA POUPANCA', 'TRANSFERENCIA PARA POUPANCA', NULL, '-500.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transferências'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, 1, NULL, 0, '00000000-0000-4000-8000-000000000010', NULL, '2026-04-06 09:00:00', '2026-04-06 09:00:00'),
    (3, NULL, '2026-04-06', 'Transferência da conta corrente', NULL, NULL, '500.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transferências'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, '00000000-0000-4000-8000-000000000010', NULL, '2026-04-06 09:00:00', '2026-04-06 09:00:00'),
    (1, 'tx-202604-cdb', '2026-04-07', 'APLICACAO CDB BANCO FICTICIO', 'APLICACAO CDB BANCO FICTICIO', NULL, '-1000.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Investimentos'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-04-07 09:00:00', '2026-04-07 09:00:00'),
    (1, 'tx-202604-alug', '2026-04-08', 'Aluguel', 'PIX ENVIADO IMOBILIARIA FICTICIA', NULL, '-1850.0', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Moradia'), 0, NULL, NULL, NULL, 0, 0, 0, 'PIX', NULL, 1, 0, NULL, NULL, '2026-04-08 09:00:00', '2026-04-08 09:00:00'),
    (1, 'tx-202604-pgfat', '2026-04-10', 'PAGAMENTO FATURA CARTAO', 'PAGAMENTO FATURA CARTAO', NULL, '-461.55', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Pagamento de fatura'), 0, NULL, NULL, NULL, 0, 0, 0, 'PAGAMENTO_FATURA', 1, NULL, 0, '00000000-0000-4000-8000-000000000011', NULL, '2026-04-10 09:00:00', '2026-04-10 09:00:00'),
    (2, 'tx-202604-pgrec', '2026-04-10', 'PAGAMENTO RECEBIDO', 'PAGAMENTO RECEBIDO', NULL, '461.55', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Pagamento de fatura'), 0, 5, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, '00000000-0000-4000-8000-000000000011', NULL, '2026-04-10 09:00:00', '2026-04-10 09:00:00'),
    (1, 'tx-202604-luz', '2026-04-12', 'Conta de luz', 'DEBITO AUTOMATICO ENERGIA FICTICIA', NULL, '-186.75', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Moradia'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, 2, 0, NULL, NULL, '2026-04-12 09:00:00', '2026-04-12 09:00:00'),
    (1, 'tx-202604-merc', '2026-04-15', 'SUPERMERCADO EXEMPLO', 'SUPERMERCADO EXEMPLO', 'Supermercado Exemplo', '-486.85', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Mercado'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-04-15 09:00:00', '2026-04-15 09:00:00'),
    (2, 'tx-202604-c18', '2026-04-18', 'RESTAURANTE SABOR FICTICIO', 'RESTAURANTE SABOR FICTICIO', 'Restaurante Sabor', '-96.4', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Restaurantes'), 0, 6, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2026-04-18 09:00:00', '2026-04-18 09:00:00'),
    (2, 'tx-202604-c20', '2026-04-20', 'ASSINATURA STREAMFLIX', 'ASSINATURA STREAMFLIX', NULL, '-39.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Serviços e assinaturas'), 0, 6, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2026-04-20 09:00:00', '2026-04-20 09:00:00'),
    (2, 'tx-202604-c22', '2026-04-22', 'APP TRANSPORTE FICTICIO *VIAGEM', 'APP TRANSPORTE FICTICIO *VIAGEM', NULL, '-27.45', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transporte'), 0, 6, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2026-04-22 09:00:00', '2026-04-22 09:00:00'),
    (1, 'tx-202604-tar', '2026-04-25', 'TARIFA PACOTE DE SERVICOS', 'TARIFA PACOTE DE SERVICOS', NULL, '-32.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Tarifas bancárias'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-04-25 09:00:00', '2026-04-25 09:00:00'),
    (2, 'tx-202605-movel', '2026-05-04', 'LOJA DE MOVEIS FICTICIA 5/10', 'LOJA DE MOVEIS FICTICIA 5/10', NULL, '-299.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Compras'), 0, 7, 5, 10, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-05-04 09:00:00', '2026-05-04 09:00:00'),
    (1, 'tx-202605-sal', '2026-05-05', 'SALARIO EMPRESA FICTICIA LTDA', 'SALARIO EMPRESA FICTICIA LTDA', NULL, '6500.0', 2, 1, (SELECT Id FROM Categoria WHERE Nome = 'Salário'), 0, NULL, NULL, NULL, 0, 0, 0, 'TED', NULL, NULL, 0, NULL, NULL, '2026-05-05 09:00:00', '2026-05-05 09:00:00'),
    (1, 'tx-202605-poup', '2026-05-06', 'TRANSFERENCIA PARA POUPANCA', 'TRANSFERENCIA PARA POUPANCA', NULL, '-500.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transferências'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, 1, NULL, 0, '00000000-0000-4000-8000-000000000012', NULL, '2026-05-06 09:00:00', '2026-05-06 09:00:00'),
    (3, NULL, '2026-05-06', 'Transferência da conta corrente', NULL, NULL, '500.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transferências'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, '00000000-0000-4000-8000-000000000012', NULL, '2026-05-06 09:00:00', '2026-05-06 09:00:00'),
    (1, 'tx-202605-cdb', '2026-05-07', 'APLICACAO CDB BANCO FICTICIO', 'APLICACAO CDB BANCO FICTICIO', NULL, '-1000.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Investimentos'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-05-07 09:00:00', '2026-05-07 09:00:00'),
    (1, 'tx-202605-alug', '2026-05-08', 'Aluguel', 'PIX ENVIADO IMOBILIARIA FICTICIA', NULL, '-1850.0', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Moradia'), 0, NULL, NULL, NULL, 0, 0, 0, 'PIX', NULL, 1, 0, NULL, NULL, '2026-05-08 09:00:00', '2026-05-08 09:00:00'),
    (1, 'tx-202605-pgfat', '2026-05-10', 'PAGAMENTO FATURA CARTAO', 'PAGAMENTO FATURA CARTAO', NULL, '-463.65', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Pagamento de fatura'), 0, NULL, NULL, NULL, 0, 0, 0, 'PAGAMENTO_FATURA', 1, NULL, 0, '00000000-0000-4000-8000-000000000013', NULL, '2026-05-10 09:00:00', '2026-05-10 09:00:00'),
    (2, 'tx-202605-pgrec', '2026-05-10', 'PAGAMENTO RECEBIDO', 'PAGAMENTO RECEBIDO', NULL, '463.65', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Pagamento de fatura'), 0, 6, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, '00000000-0000-4000-8000-000000000013', NULL, '2026-05-10 09:00:00', '2026-05-10 09:00:00'),
    (1, 'tx-202605-luz', '2026-05-12', 'Conta de luz', 'DEBITO AUTOMATICO ENERGIA FICTICIA', NULL, '-194.1', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Moradia'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, 2, 0, NULL, NULL, '2026-05-12 09:00:00', '2026-05-12 09:00:00'),
    (1, 'tx-202605-merc', '2026-05-15', 'SUPERMERCADO EXEMPLO', 'SUPERMERCADO EXEMPLO', 'Supermercado Exemplo', '-500.12', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Mercado'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-05-15 09:00:00', '2026-05-15 09:00:00'),
    (2, 'tx-202605-c18', '2026-05-18', 'RESTAURANTE SABOR FICTICIO', 'RESTAURANTE SABOR FICTICIO', 'Restaurante Sabor', '-98.5', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Restaurantes'), 0, 7, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2026-05-18 09:00:00', '2026-05-18 09:00:00'),
    (2, 'tx-202605-c20', '2026-05-20', 'ASSINATURA STREAMFLIX', 'ASSINATURA STREAMFLIX', NULL, '-39.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Serviços e assinaturas'), 0, 7, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2026-05-20 09:00:00', '2026-05-20 09:00:00'),
    (2, 'tx-202605-c22', '2026-05-22', 'APP TRANSPORTE FICTICIO *VIAGEM', 'APP TRANSPORTE FICTICIO *VIAGEM', NULL, '-27.45', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transporte'), 0, 7, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2026-05-22 09:00:00', '2026-05-22 09:00:00'),
    (1, 'tx-202605-tar', '2026-05-25', 'TARIFA PACOTE DE SERVICOS', 'TARIFA PACOTE DE SERVICOS', NULL, '-32.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Tarifas bancárias'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-05-25 09:00:00', '2026-05-25 09:00:00'),
    (2, 'tx-202606-movel', '2026-06-04', 'LOJA DE MOVEIS FICTICIA 6/10', 'LOJA DE MOVEIS FICTICIA 6/10', NULL, '-299.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Compras'), 0, 8, 6, 10, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-06-04 09:00:00', '2026-06-04 09:00:00'),
    (1, 'tx-202606-sal', '2026-06-05', 'SALARIO EMPRESA FICTICIA LTDA', 'SALARIO EMPRESA FICTICIA LTDA', NULL, '6500.0', 2, 1, (SELECT Id FROM Categoria WHERE Nome = 'Salário'), 0, NULL, NULL, NULL, 0, 0, 0, 'TED', NULL, NULL, 0, NULL, NULL, '2026-06-05 09:00:00', '2026-06-05 09:00:00'),
    (1, 'tx-202606-poup', '2026-06-06', 'TRANSFERENCIA PARA POUPANCA', 'TRANSFERENCIA PARA POUPANCA', NULL, '-500.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transferências'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, 1, NULL, 0, '00000000-0000-4000-8000-000000000014', NULL, '2026-06-06 09:00:00', '2026-06-06 09:00:00'),
    (3, NULL, '2026-06-06', 'Transferência da conta corrente', NULL, NULL, '500.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transferências'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, '00000000-0000-4000-8000-000000000014', NULL, '2026-06-06 09:00:00', '2026-06-06 09:00:00'),
    (1, 'tx-202606-cdb', '2026-06-07', 'APLICACAO CDB BANCO FICTICIO', 'APLICACAO CDB BANCO FICTICIO', NULL, '-1000.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Investimentos'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-06-07 09:00:00', '2026-06-07 09:00:00'),
    (1, 'tx-202606-alug', '2026-06-08', 'Aluguel', 'PIX ENVIADO IMOBILIARIA FICTICIA', NULL, '-1850.0', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Moradia'), 0, NULL, NULL, NULL, 0, 0, 0, 'PIX', NULL, 1, 0, NULL, NULL, '2026-06-08 09:00:00', '2026-06-08 09:00:00'),
    (1, 'tx-202606-pgfat', '2026-06-10', 'PAGAMENTO FATURA CARTAO', 'PAGAMENTO FATURA CARTAO', NULL, '-465.75', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Pagamento de fatura'), 0, NULL, NULL, NULL, 0, 0, 0, 'PAGAMENTO_FATURA', 1, NULL, 0, '00000000-0000-4000-8000-000000000015', NULL, '2026-06-10 09:00:00', '2026-06-10 09:00:00'),
    (2, 'tx-202606-pgrec', '2026-06-10', 'PAGAMENTO RECEBIDO', 'PAGAMENTO RECEBIDO', NULL, '465.75', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Pagamento de fatura'), 0, 7, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, '00000000-0000-4000-8000-000000000015', NULL, '2026-06-10 09:00:00', '2026-06-10 09:00:00'),
    (1, 'tx-202606-luz', '2026-06-12', 'Conta de luz', 'DEBITO AUTOMATICO ENERGIA FICTICIA', NULL, '-201.45', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Moradia'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, 2, 0, NULL, NULL, '2026-06-12 09:00:00', '2026-06-12 09:00:00'),
    (1, 'tx-202606-merc', '2026-06-15', 'SUPERMERCADO EXEMPLO', 'SUPERMERCADO EXEMPLO', 'Supermercado Exemplo', '-513.39', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Mercado'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-06-15 09:00:00', '2026-06-15 09:00:00'),
    (2, 'tx-202606-c18', '2026-06-18', 'RESTAURANTE SABOR FICTICIO', 'RESTAURANTE SABOR FICTICIO', 'Restaurante Sabor', '-100.6', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Restaurantes'), 0, 8, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2026-06-18 09:00:00', '2026-06-18 09:00:00'),
    (2, 'tx-202606-c20', '2026-06-20', 'ASSINATURA STREAMFLIX', 'ASSINATURA STREAMFLIX', NULL, '-39.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Serviços e assinaturas'), 0, 8, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2026-06-20 09:00:00', '2026-06-20 09:00:00'),
    (2, 'tx-202606-c22', '2026-06-22', 'APP TRANSPORTE FICTICIO *VIAGEM', 'APP TRANSPORTE FICTICIO *VIAGEM', NULL, '-27.45', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transporte'), 0, 8, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2026-06-22 09:00:00', '2026-06-22 09:00:00'),
    (1, 'tx-202606-tar', '2026-06-25', 'TARIFA PACOTE DE SERVICOS', 'TARIFA PACOTE DE SERVICOS', NULL, '-32.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Tarifas bancárias'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-06-25 09:00:00', '2026-06-25 09:00:00'),
    (2, 'tx-202607-movel', '2026-07-04', 'LOJA DE MOVEIS FICTICIA 7/10', 'LOJA DE MOVEIS FICTICIA 7/10', NULL, '-299.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Compras'), 0, 9, 7, 10, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-07-04 09:00:00', '2026-07-04 09:00:00'),
    (1, 'tx-202607-sal', '2026-07-05', 'SALARIO EMPRESA FICTICIA LTDA', 'SALARIO EMPRESA FICTICIA LTDA', NULL, '6500.0', 2, 1, (SELECT Id FROM Categoria WHERE Nome = 'Salário'), 0, NULL, NULL, NULL, 0, 0, 0, 'TED', NULL, NULL, 0, NULL, NULL, '2026-07-05 09:00:00', '2026-07-05 09:00:00'),
    (1, 'tx-202607-poup', '2026-07-06', 'TRANSFERENCIA PARA POUPANCA', 'TRANSFERENCIA PARA POUPANCA', NULL, '-500.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transferências'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, 1, NULL, 0, '00000000-0000-4000-8000-000000000016', NULL, '2026-07-06 09:00:00', '2026-07-06 09:00:00'),
    (3, NULL, '2026-07-06', 'Transferência da conta corrente', NULL, NULL, '500.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transferências'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, '00000000-0000-4000-8000-000000000016', NULL, '2026-07-06 09:00:00', '2026-07-06 09:00:00'),
    (1, 'tx-202607-cdb', '2026-07-07', 'APLICACAO CDB BANCO FICTICIO', 'APLICACAO CDB BANCO FICTICIO', NULL, '-1000.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Investimentos'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-07-07 09:00:00', '2026-07-07 09:00:00'),
    (1, 'tx-202607-alug', '2026-07-08', 'Aluguel', 'PIX ENVIADO IMOBILIARIA FICTICIA', NULL, '-1850.0', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Moradia'), 0, NULL, NULL, NULL, 0, 0, 0, 'PIX', NULL, 1, 0, NULL, NULL, '2026-07-08 09:00:00', '2026-07-08 09:00:00'),
    (2, 'tx-roupa', '2026-07-09', 'LOJA DE ROUPAS FICTICIA', 'LOJA DE ROUPAS FICTICIA', NULL, '-249.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Compras'), 0, 9, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2026-07-09 09:00:00', '2026-07-09 09:00:00'),
    (1, 'tx-202607-pgfat', '2026-07-10', 'PAGAMENTO FATURA CARTAO', 'PAGAMENTO FATURA CARTAO', NULL, '-467.85', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Pagamento de fatura'), 0, NULL, NULL, NULL, 0, 0, 0, 'PAGAMENTO_FATURA', 1, NULL, 0, '00000000-0000-4000-8000-000000000017', NULL, '2026-07-10 09:00:00', '2026-07-10 09:00:00'),
    (2, 'tx-202607-pgrec', '2026-07-10', 'PAGAMENTO RECEBIDO', 'PAGAMENTO RECEBIDO', NULL, '467.85', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Pagamento de fatura'), 0, 8, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, '00000000-0000-4000-8000-000000000017', NULL, '2026-07-10 09:00:00', '2026-07-10 09:00:00'),
    (1, 'tx-202607-luz', '2026-07-12', 'Conta de luz', 'DEBITO AUTOMATICO ENERGIA FICTICIA', NULL, '-208.8', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Moradia'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, 2, 0, NULL, NULL, '2026-07-12 09:00:00', '2026-07-12 09:00:00'),
    (1, 'tx-202607-merc', '2026-07-15', 'SUPERMERCADO EXEMPLO', 'SUPERMERCADO EXEMPLO', 'Supermercado Exemplo', '-526.66', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Mercado'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-07-15 09:00:00', '2026-07-15 09:00:00'),
    (2, 'tx-roupa-est', '2026-07-16', 'ESTORNO LOJA DE ROUPAS FICTICIA', 'ESTORNO LOJA DE ROUPAS FICTICIA', NULL, '249.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Compras'), 0, 9, NULL, NULL, 1, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-07-16 09:00:00', '2026-07-16 09:00:00'),
    (2, 'tx-202607-c18', '2026-07-18', 'RESTAURANTE SABOR FICTICIO', 'RESTAURANTE SABOR FICTICIO', 'Restaurante Sabor', '-102.7', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Restaurantes'), 0, 9, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2026-07-18 09:00:00', '2026-07-18 09:00:00'),
    (2, 'tx-202607-c20', '2026-07-20', 'ASSINATURA STREAMFLIX', 'ASSINATURA STREAMFLIX', NULL, '-39.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Serviços e assinaturas'), 0, 9, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2026-07-20 09:00:00', '2026-07-20 09:00:00'),
    (2, 'tx-202607-c22', '2026-07-22', 'APP TRANSPORTE FICTICIO *VIAGEM', 'APP TRANSPORTE FICTICIO *VIAGEM', NULL, '-27.45', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transporte'), 0, 9, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2026-07-22 09:00:00', '2026-07-22 09:00:00'),
    (1, 'tx-202607-tar', '2026-07-25', 'TARIFA PACOTE DE SERVICOS', 'TARIFA PACOTE DE SERVICOS', NULL, '-32.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Tarifas bancárias'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-07-25 09:00:00', '2026-07-25 09:00:00'),
    (1, 'tx-farm', '2026-08-03', 'FARMACIA FICTICIA', 'FARMACIA FICTICIA', NULL, '-320.47', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Saúde'), 0, NULL, NULL, NULL, 0, 1, 0, NULL, NULL, NULL, 0, NULL, 'Remédio de urgência', '2026-08-03 09:00:00', '2026-08-03 09:00:00'),
    (2, 'tx-202608-movel', '2026-08-04', 'LOJA DE MOVEIS FICTICIA 8/10', 'LOJA DE MOVEIS FICTICIA 8/10', NULL, '-299.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Compras'), 0, 10, 8, 10, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-08-04 09:00:00', '2026-08-04 09:00:00'),
    (1, 'tx-202608-sal', '2026-08-05', 'SALARIO EMPRESA FICTICIA LTDA', 'SALARIO EMPRESA FICTICIA LTDA', NULL, '6500.0', 2, 1, (SELECT Id FROM Categoria WHERE Nome = 'Salário'), 0, NULL, NULL, NULL, 0, 0, 0, 'TED', NULL, NULL, 0, NULL, NULL, '2026-08-05 09:00:00', '2026-08-05 09:00:00'),
    (1, 'tx-202608-poup', '2026-08-06', 'TRANSFERENCIA PARA POUPANCA', 'TRANSFERENCIA PARA POUPANCA', NULL, '-500.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transferências'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, 1, NULL, 0, '00000000-0000-4000-8000-000000000018', NULL, '2026-08-06 09:00:00', '2026-08-06 09:00:00'),
    (3, NULL, '2026-08-06', 'Transferência da conta corrente', NULL, NULL, '500.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transferências'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, '00000000-0000-4000-8000-000000000018', NULL, '2026-08-06 09:00:00', '2026-08-06 09:00:00'),
    (1, 'tx-202608-cdb', '2026-08-07', 'APLICACAO CDB BANCO FICTICIO', 'APLICACAO CDB BANCO FICTICIO', NULL, '-1000.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Investimentos'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-08-07 09:00:00', '2026-08-07 09:00:00'),
    (1, 'tx-202608-alug', '2026-08-08', 'Aluguel', 'PIX ENVIADO IMOBILIARIA FICTICIA', NULL, '-1850.0', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Moradia'), 0, NULL, NULL, NULL, 0, 0, 0, 'PIX', NULL, 1, 0, NULL, NULL, '2026-08-08 09:00:00', '2026-08-08 09:00:00'),
    (1, 'tx-202608-pgfat', '2026-08-10', 'PAGAMENTO FATURA CARTAO', 'PAGAMENTO FATURA CARTAO', NULL, '-469.95', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Pagamento de fatura'), 0, NULL, NULL, NULL, 0, 0, 0, 'PAGAMENTO_FATURA', 1, NULL, 0, '00000000-0000-4000-8000-000000000019', NULL, '2026-08-10 09:00:00', '2026-08-10 09:00:00'),
    (2, 'tx-202608-pgrec', '2026-08-10', 'PAGAMENTO RECEBIDO', 'PAGAMENTO RECEBIDO', NULL, '469.95', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Pagamento de fatura'), 0, 9, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, '00000000-0000-4000-8000-000000000019', NULL, '2026-08-10 09:00:00', '2026-08-10 09:00:00'),
    (1, 'tx-202608-luz', '2026-08-12', 'Conta de luz', 'DEBITO AUTOMATICO ENERGIA FICTICIA', NULL, '-216.15', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Moradia'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, 2, 0, NULL, NULL, '2026-08-12 09:00:00', '2026-08-12 09:00:00'),
    (1, 'tx-202608-merc', '2026-08-15', 'SUPERMERCADO EXEMPLO', 'SUPERMERCADO EXEMPLO', 'Supermercado Exemplo', '-539.93', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Mercado'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-08-15 09:00:00', '2026-08-15 09:00:00'),
    (2, 'tx-202608-c18', '2026-08-18', 'RESTAURANTE SABOR FICTICIO', 'RESTAURANTE SABOR FICTICIO', 'Restaurante Sabor', '-104.8', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Restaurantes'), 0, 10, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2026-08-18 09:00:00', '2026-08-18 09:00:00'),
    (2, 'tx-202608-c20', '2026-08-20', 'ASSINATURA STREAMFLIX', 'ASSINATURA STREAMFLIX', NULL, '-39.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Serviços e assinaturas'), 0, 10, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2026-08-20 09:00:00', '2026-08-20 09:00:00'),
    (2, 'tx-202608-c22', '2026-08-22', 'APP TRANSPORTE FICTICIO *VIAGEM', 'APP TRANSPORTE FICTICIO *VIAGEM', NULL, '-27.45', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transporte'), 0, 10, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2026-08-22 09:00:00', '2026-08-22 09:00:00'),
    (1, 'tx-202608-tar', '2026-08-25', 'TARIFA PACOTE DE SERVICOS', 'TARIFA PACOTE DE SERVICOS', NULL, '-32.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Tarifas bancárias'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-08-25 09:00:00', '2026-08-25 09:00:00'),
    (2, 'tx-202609-movel', '2026-09-04', 'LOJA DE MOVEIS FICTICIA 9/10', 'LOJA DE MOVEIS FICTICIA 9/10', NULL, '-299.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Compras'), 0, 11, 9, 10, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-09-04 09:00:00', '2026-09-04 09:00:00'),
    (1, 'tx-202609-sal', '2026-09-05', 'SALARIO EMPRESA FICTICIA LTDA', 'SALARIO EMPRESA FICTICIA LTDA', NULL, '6500.0', 2, 1, (SELECT Id FROM Categoria WHERE Nome = 'Salário'), 0, NULL, NULL, NULL, 0, 0, 0, 'TED', NULL, NULL, 0, NULL, NULL, '2026-09-05 09:00:00', '2026-09-05 09:00:00'),
    (1, 'tx-202609-poup', '2026-09-06', 'TRANSFERENCIA PARA POUPANCA', 'TRANSFERENCIA PARA POUPANCA', NULL, '-500.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transferências'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, 1, NULL, 0, '00000000-0000-4000-8000-000000000020', NULL, '2026-09-06 09:00:00', '2026-09-06 09:00:00'),
    (3, NULL, '2026-09-06', 'Transferência da conta corrente', NULL, NULL, '500.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transferências'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, '00000000-0000-4000-8000-000000000020', NULL, '2026-09-06 09:00:00', '2026-09-06 09:00:00'),
    (1, 'tx-202609-cdb', '2026-09-07', 'APLICACAO CDB BANCO FICTICIO', 'APLICACAO CDB BANCO FICTICIO', NULL, '-1000.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Investimentos'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-09-07 09:00:00', '2026-09-07 09:00:00'),
    (1, 'tx-202609-alug', '2026-09-08', 'Aluguel', 'PIX ENVIADO IMOBILIARIA FICTICIA', NULL, '-1850.0', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Moradia'), 0, NULL, NULL, NULL, 0, 0, 0, 'PIX', NULL, 1, 0, NULL, NULL, '2026-09-08 09:00:00', '2026-09-08 09:00:00'),
    (1, 'tx-202609-pgfat', '2026-09-10', 'PAGAMENTO FATURA CARTAO', 'PAGAMENTO FATURA CARTAO', NULL, '-472.05', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Pagamento de fatura'), 0, NULL, NULL, NULL, 0, 0, 0, 'PAGAMENTO_FATURA', 1, NULL, 0, '00000000-0000-4000-8000-000000000021', NULL, '2026-09-10 09:00:00', '2026-09-10 09:00:00'),
    (2, 'tx-202609-pgrec', '2026-09-10', 'PAGAMENTO RECEBIDO', 'PAGAMENTO RECEBIDO', NULL, '472.05', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Pagamento de fatura'), 0, 10, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, '00000000-0000-4000-8000-000000000021', NULL, '2026-09-10 09:00:00', '2026-09-10 09:00:00'),
    (1, 'tx-202609-luz', '2026-09-12', 'Conta de luz', 'DEBITO AUTOMATICO ENERGIA FICTICIA', NULL, '-223.5', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Moradia'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, 2, 0, NULL, NULL, '2026-09-12 09:00:00', '2026-09-12 09:00:00'),
    (1, 'tx-pix-sem-cat', '2026-09-14', 'PIX ENVIADO', 'PIX ENVIADO', NULL, '-50.0', 1, 1, NULL, 0, NULL, NULL, NULL, 0, 0, 0, 'PIX', NULL, NULL, 0, NULL, NULL, '2026-09-14 09:00:00', '2026-09-14 09:00:00'),
    (1, 'tx-202609-merc', '2026-09-15', 'SUPERMERCADO EXEMPLO', 'SUPERMERCADO EXEMPLO', 'Supermercado Exemplo', '-553.2', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Mercado'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-09-15 09:00:00', '2026-09-15 09:00:00'),
    (2, 'tx-202609-c18', '2026-09-18', 'RESTAURANTE SABOR FICTICIO', 'RESTAURANTE SABOR FICTICIO', 'Restaurante Sabor', '-106.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Restaurantes'), 0, 11, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2026-09-18 09:00:00', '2026-09-18 09:00:00'),
    (2, 'tx-202609-c20', '2026-09-20', 'ASSINATURA STREAMFLIX', 'ASSINATURA STREAMFLIX', NULL, '-39.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Serviços e assinaturas'), 0, 11, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2026-09-20 09:00:00', '2026-09-20 09:00:00'),
    (6, NULL, '2026-09-20', 'ONLINE STORE (USD)', NULL, NULL, '-45.99', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Compras'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-09-20 09:00:00', '2026-09-20 09:00:00'),
    (2, 'tx-202609-c22', '2026-09-22', 'APP TRANSPORTE FICTICIO *VIAGEM', 'APP TRANSPORTE FICTICIO *VIAGEM', NULL, '-27.45', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transporte'), 0, 11, NULL, NULL, 0, 0, 0, 'CARTAO', NULL, NULL, 0, NULL, NULL, '2026-09-22 09:00:00', '2026-09-22 09:00:00'),
    (1, 'tx-202609-tar', '2026-09-25', 'TARIFA PACOTE DE SERVICOS', 'TARIFA PACOTE DE SERVICOS', NULL, '-32.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Tarifas bancárias'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-09-25 09:00:00', '2026-09-25 09:00:00'),
    (1, 'tx-centavo', '2026-09-30', 'RENDIMENTO POUPANCA', 'RENDIMENTO POUPANCA', NULL, '0.01', 2, 1, (SELECT Id FROM Categoria WHERE Nome = 'Rendimentos'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-09-30 09:00:00', '2026-09-30 09:00:00'),
    (2, 'tx-202610-movel', '2026-10-04', 'LOJA DE MOVEIS FICTICIA 10/10', 'LOJA DE MOVEIS FICTICIA 10/10', NULL, '-299.9', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Compras'), 0, 12, 10, 10, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-10-04 09:00:00', '2026-10-04 09:00:00'),
    (1, 'tx-202610-sal', '2026-10-05', 'SALARIO EMPRESA FICTICIA LTDA', 'SALARIO EMPRESA FICTICIA LTDA', NULL, '6500.0', 2, 1, (SELECT Id FROM Categoria WHERE Nome = 'Salário'), 0, NULL, NULL, NULL, 0, 0, 0, 'TED', NULL, NULL, 0, NULL, NULL, '2026-10-05 09:00:00', '2026-10-05 09:00:00'),
    (1, 'tx-202610-poup', '2026-10-06', 'TRANSFERENCIA PARA POUPANCA', 'TRANSFERENCIA PARA POUPANCA', NULL, '-500.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transferências'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, 1, NULL, 0, '00000000-0000-4000-8000-000000000022', NULL, '2026-10-06 09:00:00', '2026-10-06 09:00:00'),
    (3, NULL, '2026-10-06', 'Transferência da conta corrente', NULL, NULL, '500.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Transferências'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, '00000000-0000-4000-8000-000000000022', NULL, '2026-10-06 09:00:00', '2026-10-06 09:00:00'),
    (1, 'tx-202610-cdb', '2026-10-07', 'APLICACAO CDB BANCO FICTICIO', 'APLICACAO CDB BANCO FICTICIO', NULL, '-1000.0', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Investimentos'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-10-07 09:00:00', '2026-10-07 09:00:00'),
    (1, 'tx-202610-alug', '2026-10-08', 'Aluguel', 'PIX ENVIADO IMOBILIARIA FICTICIA', NULL, '-1850.0', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Moradia'), 0, NULL, NULL, NULL, 0, 0, 0, 'PIX', NULL, 1, 0, NULL, NULL, '2026-10-08 09:00:00', '2026-10-08 09:00:00'),
    (1, 'tx-202610-pgfat', '2026-10-10', 'PAGAMENTO FATURA CARTAO', 'PAGAMENTO FATURA CARTAO', NULL, '-474.15', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Pagamento de fatura'), 0, NULL, NULL, NULL, 0, 0, 0, 'PAGAMENTO_FATURA', 1, NULL, 0, '00000000-0000-4000-8000-000000000023', NULL, '2026-10-10 09:00:00', '2026-10-10 09:00:00'),
    (2, 'tx-202610-pgrec', '2026-10-10', 'PAGAMENTO RECEBIDO', 'PAGAMENTO RECEBIDO', NULL, '474.15', 3, 1, (SELECT Id FROM Categoria WHERE Nome = 'Pagamento de fatura'), 0, 11, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, '00000000-0000-4000-8000-000000000023', NULL, '2026-10-10 09:00:00', '2026-10-10 09:00:00'),
    (4, NULL, '2026-10-11', 'Feira', NULL, NULL, '-60.0', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Mercado'), 1, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-10-11 09:00:00', '2026-10-11 09:00:00'),
    (1, 'tx-202610-luz', '2026-10-12', 'Conta de luz', 'DEBITO AUTOMATICO ENERGIA FICTICIA', NULL, '-230.85', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Moradia'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, 2, 0, NULL, NULL, '2026-10-12 09:00:00', '2026-10-12 09:00:00'),
    (4, NULL, '2026-10-12', 'Padaria do bairro', NULL, NULL, '-12.75', 1, 1, (SELECT Id FROM Categoria WHERE Nome = 'Padaria'), 1, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 1, NULL, NULL, '2026-10-12 09:00:00', '2026-10-12 09:00:00'),
    (1, 'tx-pix-rec', '2026-10-13', 'PIX RECEBIDO JOAO FICTICIO', 'PIX RECEBIDO JOAO FICTICIO', NULL, '120.35', 2, 1, (SELECT Id FROM Categoria WHERE Nome = 'Outras receitas'), 0, NULL, NULL, NULL, 0, 0, 0, 'PIX', 0, NULL, 0, NULL, NULL, '2026-10-13 09:00:00', '2026-10-13 09:00:00'),
    (1, 'tx-pend', '2026-10-14', 'COMPRA PENDENTE MERCADINHO', 'COMPRA PENDENTE MERCADINHO', NULL, '-18.9', 1, 2, (SELECT Id FROM Categoria WHERE Nome = 'Mercado'), 0, NULL, NULL, NULL, 0, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, '2026-10-14 09:00:00', '2026-10-14 09:00:00');

/* ------------------------------------------------------------------ Planejamento */
INSERT INTO Orcamento (CategoriaId, Ano, Mes, Valor) VALUES
    ((SELECT Id FROM Categoria WHERE Nome = 'Mercado'),      2026, 9,  '600.0'),
    ((SELECT Id FROM Categoria WHERE Nome = 'Alimentação'),  2026, 9,  '400.0'),
    ((SELECT Id FROM Categoria WHERE Nome = 'Lazer'),        2026, 9,  '200.0'),
    ((SELECT Id FROM Categoria WHERE Nome = 'Mercado'),      2026, 10, '650.0'),
    ((SELECT Id FROM Categoria WHERE Nome = 'Alimentação'),  2026, 10, '400.0'),
    ((SELECT Id FROM Categoria WHERE Nome = 'Transporte'),   2026, 10, '150.5'),
    ((SELECT Id FROM Categoria WHERE Nome = 'Restaurantes'), 2026, 10, '120.0');

INSERT INTO Meta (Id, Nome, ValorAlvo, ValorAtual, DataAlvo, ContaId, Cor, Concluida, ReservaNome, Prioridade, Icone, CriadoEm) VALUES
    (1, 'Reserva de emergência', '30000.0', '12500.0', '2027-12-31', 3, '#0f766e', 0, NULL, 1, 'escudo', '2025-11-01 12:00:00'),
    (2, 'Viagem de férias', '8000.0', '1500.5', '2027-06-30', 1, '#2563eb', 0, 'Caixinha viagem', 2, 'aviao', '2025-12-01 12:00:00'),
    (3, 'Notebook novo', '4500.0', '4500.0', '2026-08-31', NULL, '#9333ea', 1, NULL, 0, NULL, '2025-11-15 12:00:00');

INSERT INTO MetaAporte (MetaId, Data, Valor, CriadoEm) VALUES
    (1, '2026-08-06', '500.0', '2026-08-06 12:00:00'),
    (1, '2026-09-06', '500.0', '2026-09-06 12:00:00'),
    (1, '2026-10-06', '500.0', '2026-10-06 12:00:00'),
    (2, '2026-09-20', '1600.5', '2026-09-20 12:00:00'),
    (2, '2026-10-02', '-100.0', '2026-10-02 12:00:00'),
    (3, '2026-08-30', '4500.0', '2026-08-30 12:00:00');

INSERT INTO ContaAgendada (Descricao, Valor, ProximoVencimento, Recorrencia, CategoriaId, ContaId, Ativa, UltimoPagamentoEm, DataLimite,
    ChaveExtrato, FormaCobranca, TextoCobranca, Observacao, CriadoEm) VALUES
    ('Internet', '119.9', '2026-10-20', 1, (SELECT Id FROM Categoria WHERE Nome = 'Serviços e assinaturas'), 1, 1, '2026-09-20', NULL, NULL, 1, NULL, NULL, '2025-11-01 12:00:00'),
    ('Seguro do carro', '2100.0', '2026-10-18', 0, (SELECT Id FROM Categoria WHERE Nome = 'Seguros'), 1, 1, NULL, NULL, NULL, 1, NULL, 'Parcela única', '2026-09-01 12:00:00'),
    ('IPVA', '1450.0', '2027-01-15', 2, (SELECT Id FROM Categoria WHERE Nome = 'Impostos e taxas'), 1, 1, '2026-01-15', NULL, NULL, 1, NULL, NULL, '2025-11-01 12:00:00'),
    ('Streaming', '39.9', '2026-10-20', 1, (SELECT Id FROM Categoria WHERE Nome = 'Serviços e assinaturas'), 2, 1, '2026-09-20', NULL, NULL, 3, 'STREAMFLIX', NULL, '2025-11-01 12:00:00'),
    ('Curso de inglês', '350.0', '2026-10-25', 1, (SELECT Id FROM Categoria WHERE Nome = 'Educação'), 1, 1, '2026-09-25', '2026-12-25', NULL, 1, NULL, NULL, '2026-01-10 12:00:00'),
    ('Academia (cancelada)', '99.0', '2026-05-10', 1, NULL, 1, 0, '2026-04-10', NULL, NULL, 1, NULL, NULL, '2025-11-01 12:00:00');

INSERT INTO ReservaConferencia (ContaId, Nome, SaldoConferido, DataConferencia, AtualizadoEm) VALUES
    (1, 'Caixinha viagem', '800.0', '2026-09-30', '2026-09-30 20:00:00');

/* ------------------------------------------------------------------ Investimentos e empréstimos do Open Finance */
INSERT INTO Investimento (ConexaoBancariaId, PluggyInvestmentId, Nome, Tipo, Subtipo, Codigo, Emissor, Moeda, Saldo, ValorAplicado,
    Rendimento, Quantidade, TaxaAnual, DataVencimento, DataAplicacao, Status, AtualizadoEm) VALUES
    (1, 'inv-cdb-0001', 'CDB Banco Fictício 110% CDI', 'FIXED_INCOME', 'CDB', NULL, 'Banco Fictício', 'BRL', '12873.42', '12000.0',
     '873.42', NULL, '110.0', '2028-11-07', '2025-11-07', 'ACTIVE', '2026-10-14 09:00:00'),
    (1, 'inv-fundo-0001', 'Fundo DI Fictício', 'MUTUAL_FUND', 'INVESTMENT_FUND', NULL, NULL, 'BRL', '2530.07', '2500.0',
     '30.07', '1843.12345678', NULL, NULL, '2026-06-01', 'ACTIVE', '2026-10-14 09:00:00');

INSERT INTO HistoricoInvestimento (PluggyInvestmentId, ConexaoBancariaId, Nome, Tipo, Data, Saldo, ValorAplicado, AtualizadoEm) VALUES
    ('inv-cdb-0001', 1, 'CDB Banco Fictício 110% CDI', 'FIXED_INCOME', '2026-08-31', '12510.2', '11000.0', '2026-08-31 09:00:00'),
    ('inv-cdb-0001', 1, 'CDB Banco Fictício 110% CDI', 'FIXED_INCOME', '2026-09-30', '12688.73', '11000.0', '2026-09-30 09:00:00'),
    ('inv-cdb-0001', 1, 'CDB Banco Fictício 110% CDI', 'FIXED_INCOME', '2026-10-14', '12873.42', '12000.0', '2026-10-14 09:00:00'),
    ('inv-resgatado-0001', 1, 'LCI Fictícia (resgatada)', 'FIXED_INCOME', '2026-05-31', '3050.0', '3000.0', '2026-05-31 09:00:00');

INSERT INTO Emprestimo (Id, ConexaoBancariaId, PluggyLoanId, Natureza, Modalidade, Produto, NumeroContrato, Moeda, ValorContratado,
    SaldoDevedor, ValorParcela, ParcelasTotal, ParcelasPagas, ParcelasAVencer, ParcelasEmAtraso, Periodicidade, SistemaAmortizacao,
    TaxaJuros, PeriodicidadeTaxa, Indexador, Cet, DataContratacao, DataPrimeiraParcela, DataVencimentoFinal, AtualizadoEm) VALUES
    (1, 1, 'loan-veiculo-0001', 2, 'FINANCIAMENTO_AQUISICAO_BENS_VEICULOS_AUTOMOTORES', 'Financiamento de veículo', 'CTR-0001', 'BRL',
     '45000.0', '28750.33', '1250.0', 48, 18, 30, 0, 'MENSAL', 'PRICE', '0.0149', 'MENSAL', 'PRE_FIXADO', '0.0175',
     '2025-04-10', '2025-05-10', '2029-04-10', '2026-10-14 09:00:00'),
    (2, 1, 'loan-pessoal-0001', 1, 'CREDITO_PESSOAL_SEM_CONSIGNACAO', 'Crédito pessoal', NULL, 'BRL',
     '5000.0', '0.0', '480.25', 12, 12, 0, 0, 'MENSAL', NULL, '0.0399', 'MENSAL', NULL, NULL,
     '2025-09-15', '2025-10-15', '2026-09-15', '2026-10-14 09:00:00');

INSERT INTO PagamentoEmprestimo (EmprestimoId, Data, Valor, Extraordinario) VALUES
    (1, '2025-11-10', '1250.0', 0),
    (1, '2025-12-10', '1250.0', 0),
    (1, '2026-01-10', '1250.0', 0),
    (1, '2026-02-10', '1250.0', 0),
    (1, '2026-03-10', '1250.0', 0),
    (1, '2026-04-10', '1250.0', 0),
    (1, '2026-05-10', '1250.0', 0),
    (1, '2026-06-10', '1250.0', 0),
    (1, '2026-07-10', '1250.0', 0),
    (1, '2026-08-10', '1250.0', 0),
    (1, '2026-09-10', '1250.0', 0),
    (1, '2026-10-10', '1250.0', 0),
    (1, '2026-06-20', '2000.0', 1);

/* ------------------------------------------------------------------ Carteira de renda variável */
INSERT INTO Ativo (Id, Ticker, Nome, Tipo, Moeda, PrecoAtual, PrecoAnterior, PrecoAtualizadoEm, PrecoManual, FonteCotacao, Segmento, CriadoEm) VALUES
    (1, 'HGLG11', 'CSHG Logística FII', 1, 'BRL', '158.32', '157.9', '2026-10-14 21:00:00', 0, 'yahoo', 'Logística', '2025-11-03 12:00:00'),
    (2, 'ITSA4', 'Itaúsa PN', 2, 'BRL', '10.45', '10.51', '2026-10-14 21:00:00', 0, 'yahoo', NULL, '2025-11-03 12:00:00'),
    (3, 'IVVB11', 'iShares S&P 500', 3, 'BRL', '345.1', '343.87', '2026-10-14 21:00:00', 0, 'yahoo', NULL, '2026-02-10 12:00:00'),
    (4, 'VOO', 'Vanguard S&P 500 ETF', 3, 'USD', '512.37', '509.8', '2026-10-14 21:00:00', 0, 'yahoo', NULL, '2026-03-05 12:00:00'),
    (5, 'XPML11', 'XP Malls FII', 1, 'BRL', '101.0', NULL, '2026-09-30 21:00:00', 1, NULL, 'Shoppings', '2026-04-01 12:00:00');

INSERT INTO OperacaoAtivo (AtivoId, Tipo, Data, Quantidade, PrecoUnitario, Taxas, Cambio, Observacao, Origem, PluggyTransacaoId, TransacaoId, CriadoEm) VALUES
    (1, 1, '2025-11-10', '10.0', '160.25', '0.0', '1.0', NULL, 1, NULL, NULL, '2025-11-10 12:00:00'),
    (1, 1, '2026-02-10', '5.0', '155.8', '1.2', '1.0', NULL, 1, NULL, NULL, '2026-02-10 12:00:00'),
    (2, 1, '2025-11-10', '100.0', '9.87', '0.5', '1.0', NULL, 1, NULL, NULL, '2025-11-10 12:00:00'),
    (2, 2, '2026-06-15', '30.0', '10.92', '0.5', '1.0', 'Venda parcial', 1, NULL, NULL, '2026-06-15 12:00:00'),
    (3, 1, '2026-02-10', '3.0', '330.0', '0.0', '1.0', NULL, 1, NULL, NULL, '2026-02-10 12:00:00'),
    (4, 1, '2026-03-05', '2.5', '480.1', '1.0', '5.4213', 'Compra no exterior', 1, NULL, NULL, '2026-03-05 12:00:00'),
    (5, 1, '2026-04-01', '20.0', '99.5', '0.0', '1.0', NULL, 1, NULL, NULL, '2026-04-01 12:00:00');

INSERT INTO ProventoAtivo (AtivoId, DataCom, ValorPorCota) VALUES
    (1, '2026-07-31', '1.1'), (1, '2026-08-31', '1.1'), (1, '2026-09-30', '1.15'),
    (2, '2026-08-20', '0.0185'), (5, '2026-09-30', '0.92');

INSERT INTO PrecoMensalAtivo (AtivoId, Mes, Fechamento) VALUES
    (1, '2026-07-01', '156.4'), (1, '2026-08-01', '157.12'), (1, '2026-09-01', '157.9'),
    (2, '2026-07-01', '10.2'), (2, '2026-08-01', '10.38'), (2, '2026-09-01', '10.51'),
    (4, '2026-08-01', '498.33'), (4, '2026-09-01', '509.8');

INSERT INTO IndiceMensal (Codigo, Mes, Fechamento) VALUES
    ('IFIX', '2026-07-01', '3301.25'), ('IFIX', '2026-08-01', '3345.8'), ('IFIX', '2026-09-01', '3389.07');

/* Selic diária (% ao dia) e câmbio já gravados: o contrato não depende da internet. */
INSERT INTO TaxaSelicDiaria (Data, Taxa) VALUES
    ('2026-10-01', '0.055131'), ('2026-10-02', '0.055131'), ('2026-10-05', '0.055131'), ('2026-10-06', '0.055131'),
    ('2026-10-07', '0.055131'), ('2026-10-08', '0.055131'), ('2026-10-09', '0.055131'), ('2026-10-13', '0.055131'),
    ('2026-10-14', '0.055131');

INSERT INTO CotacaoMoeda (Moeda, Data, Compra, Venda, AtualizadoEm) VALUES
    ('USD', '2026-03-05', '5.4207', '5.4213', '2026-03-05 16:00:00'),
    ('USD', '2026-10-09', '5.3871', '5.3877', '2026-10-09 16:00:00'),
    ('USD', '2026-10-13', '5.4012', '5.4018', '2026-10-13 16:00:00'),
    ('USD', '2026-10-14', '5.4123', '5.4129', '2026-10-14 16:00:00'),
    ('EUR', '2026-10-14', '6.2981', '6.2995', '2026-10-14 16:00:00');

INSERT INTO CotacaoCripto (Moeda, DataHora, Preco, AtualizadoEm) VALUES
    ('BTC', '2026-10-13 21:00:00', '612345.67', '2026-10-13 21:00:05'),
    ('BTC', '2026-10-14 21:00:00', '618900.12', '2026-10-14 21:00:05');

/* ------------------------------------------------------------------ Erros do sistema (17-erros, #298) */
/* Datas em UTC, todas antes de 14/10: o aviso que o Go grava na partida (relógio de teste) fica fora dos filtros
   por período dos casos. O de 01/07 tem mais de 90 dias (Diagnostico.DiasRetencaoErros): a limpeza da partida o
   apaga. O 2 e o 7 empatam na última ocorrência (desempate pelo Id) e caem em 06/10 no horário de Brasília. */
INSERT INTO ErroSistema (Id, PrimeiraEm, UltimaEm, Ocorrencias, Nivel, Origem, Categoria, Mensagem, Detalhe, Rota, Usuario, Assinatura, Resolvido, ResolvidoEm) VALUES
    (1, '2026-10-01 12:00:00', '2026-10-06 14:30:00', 3, 4, 'API', 'Erro',
        'Erro não tratado em /api/transacoes: Sequence contains no elements',
        'System.InvalidOperationException: Sequence contains no elements
   at Financas.Api.Servicos.Exemplo.Calcular()', 'GET /api/transacoes', 'ana.teste', '0A1B2C3D4E5F60718293A4B5C6D7E8F9', 0, NULL),
    (2, '2026-10-07 02:30:00', '2026-10-07 02:30:00', 1, 3, 'Open Finance', 'Financas.Api.Servicos.Pluggy.SincronizacaoService',
        'Conexão Banco Exemplo: o consentimento vence em 5 dias.', NULL, NULL, NULL, '1111111111111111AAAAAAAAAAAAAAAA', 0, NULL),
    (3, '2026-09-18 09:00:00', '2026-09-20 09:00:00', 2, 4, 'Investimentos', 'Financas.Api.Servicos.Investimentos.CotacaoService',
        'Não foi possível atualizar a cotação de EXMP11.', NULL, NULL, NULL, '2222222222222222BBBBBBBBBBBBBBBB', 1, '2026-09-21 10:00:00.5'),
    (4, '2026-10-03 03:00:00', '2026-10-03 03:00:00', 1, 5, 'Câmbio', 'Financas.Api.Servicos.Cambio.CambioHostedService',
        'Falha na atualização automática do câmbio.', 'System.Net.Http.HttpRequestException: Name or service not known', NULL, NULL,
        '3333333333333333CCCCCCCCCCCCCCCC', 0, NULL),
    (5, '2026-07-01 08:00:00', '2026-07-01 08:00:00', 1, 3, 'Sistema', 'Financas.Api.Data.EsquemaBanco',
        'Aviso antigo, apagado pela limpeza.', NULL, NULL, NULL, '4444444444444444DDDDDDDDDDDDDDDD', 0, NULL),
    (6, '2026-10-05 18:00:00', '2026-10-05 18:00:00.25', 1, 4, 'Front-end', 'Front-end',
        'TypeError: Cannot read properties of undefined', 'TypeError: Cannot read properties of undefined
    at main-ABCDEFGH.js:1:2345', 'tela /painel', 'ana.teste', '5555555555555555EEEEEEEEEEEEEEEE', 0, NULL),
    (7, '2026-10-06 20:00:00', '2026-10-07 02:30:00', 4, 4, 'Open Finance', 'Financas.Api.Servicos.Pluggy.PluggyClient',
        'A Pluggy respondeu 503 ao buscar as transações.', NULL, NULL, NULL, '6666666666666666FFFFFFFFFFFFFFFF', 0, NULL);
