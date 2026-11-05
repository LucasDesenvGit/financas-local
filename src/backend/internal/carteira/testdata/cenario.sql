-- Cenário fictício da carteira (#82): o mesmo banco para a sonda em C# (que grava o gabarito carteira.json com o
-- CarteiraService de verdade) e para o teste do Go. Nada aqui é dado real.
--
-- Ativos: HGLG11 (compras, venda parcial, proventos antes da compra, depois de hoje e fora dos 12 meses), ITSA4 (vendida
-- por inteiro), VOO (em dólar, com a cotação do histórico nos proventos), SAP.DE (em euro, sem cotação da moeda), XPML11
-- (preço manual), MXRF11 (FII sem preço, com o segmento sugerido), BOVA11 (sem operações), PETR4 (cotas diferentes da
-- corretora), KNRI11 (preço anterior zero, cota fracionada).
-- Open Finance: renda fixa com o banco no emissor, fundo em moeda sem cotação, a ação que já está na carteira (fora dos
-- totais), a que não está (não registrada) e a da conexão desligada (fora de tudo).

INSERT INTO ConexaoBancaria (Id, PluggyItemId, NomeInstituicao, Status, Ativa, CriadoEm) VALUES
    (1, 'item-carteira-1', 'Corretora Fictícia', 'UPDATED', 1, '2025-01-01 00:00:00'),
    (2, 'item-carteira-2', 'Banco Desligado', 'LOGIN_ERROR', 0, '2025-01-01 00:00:00');

INSERT INTO Conta (Id, ConexaoBancariaId, PluggyAccountId, Nome, Tipo, Origem, Moeda, SaldoInicial, Saldo, Cor, IncluirNoSaldo,
    Arquivada, CriadoEm, AtualizadoEm) VALUES
    (1, 1, 'acc-1', 'Conta Corretora', 1, 2, 'BRL', '0.0', '0.0', '#000000', 1, 0, '2025-01-01 00:00:00', '2025-01-01 00:00:00'),
    (2, NULL, NULL, 'Tesouro manual', 4, 1, 'BRL', '1500.5', '0.0', '#000000', 1, 0, '2025-01-01 00:00:00', '2025-01-01 00:00:00'),
    (3, NULL, NULL, 'Investimento arquivado', 4, 1, 'BRL', '999.0', '0.0', '#000000', 1, 1, '2025-01-01 00:00:00', '2025-01-01 00:00:00'),
    (4, NULL, NULL, 'Investimento fora do saldo', 4, 1, 'BRL', '999.0', '0.0', '#000000', 0, 0, '2025-01-01 00:00:00', '2025-01-01 00:00:00');

INSERT INTO Transacao (ContaId, PluggyTransactionId, Data, Descricao, DescricaoOriginal, Valor, Tipo, Status, CategoriaManual,
    IgnorarRelatorios, Emergencia, DescricaoRevisada, DescricaoManual, CriadoEm, AtualizadoEm) VALUES
    (2, NULL, '2026-03-01', 'Aporte', NULL, '250.25', 2, 1, 0, 0, 0, 0, 0, '2026-03-01 12:00:00', '2026-03-01 12:00:00'),
    (1, 'c-01', '2026-08-05', 'Dinheiro reservado Viagem', 'Dinheiro reservado Viagem', '-100.0', 1, 1, 0, 0, 0, 0, 0, '2026-08-05 12:00:00', '2026-08-05 12:00:00');

INSERT INTO Ativo (Id, Ticker, Nome, Tipo, Moeda, PrecoAtual, PrecoAnterior, PrecoAtualizadoEm, PrecoManual, FonteCotacao, Segmento, CriadoEm) VALUES
    (1, 'HGLG11', 'CSHG Logística FII', 1, 'BRL', '158.32', '157.9', '2026-10-14 21:00:00', 0, 'Yahoo', 'Logística', '2024-01-03 12:00:00'),
    (2, 'ITSA4', 'Itaúsa PN', 2, 'BRL', '10.45', '10.51', '2026-10-13 21:00:00', 0, 'Yahoo', NULL, '2024-01-03 12:00:00'),
    (3, 'VOO', 'Vanguard S&P 500 ETF', 3, 'USD', '512.37', '509.8', '2026-10-15 21:00:00', 0, 'Yahoo', NULL, '2024-01-03 12:00:00'),
    (4, 'SAP.DE', 'SAP', 2, 'EUR', '210.1', NULL, NULL, 0, NULL, NULL, '2024-01-03 12:00:00'),
    (5, 'XPML11', 'XP Malls FII', 1, 'BRL', '101.0', NULL, '2026-09-30 21:00:00', 1, 'Manual', 'Shoppings', '2024-01-03 12:00:00'),
    (6, 'MXRF11', NULL, 1, 'BRL', NULL, NULL, NULL, 0, NULL, NULL, '2024-01-03 12:00:00'),
    (7, 'BOVA11', 'Ibovespa ETF', 3, 'BRL', '120.0', '119.0', '2026-10-14 21:00:00', 0, 'Yahoo', NULL, '2024-01-03 12:00:00'),
    (8, 'PETR4', 'Petrobras PN', 2, 'BRL', '38.77', '38.5', '2026-10-14 21:00:00', 0, 'Yahoo', NULL, '2024-01-03 12:00:00'),
    (9, 'KNRI11', 'Kinea Renda', 1, 'BRL', '150.0', '0.0', '2026-10-14 21:00:00', 0, 'Yahoo', NULL, '2024-01-03 12:00:00');

INSERT INTO OperacaoAtivo (Id, AtivoId, Tipo, Data, Quantidade, PrecoUnitario, Taxas, Cambio, Observacao, Origem, PluggyTransacaoId, TransacaoId, CriadoEm) VALUES
    (1, 1, 1, '2024-11-10', '10.0', '160.25', '0.0', '1.0', NULL, 1, NULL, NULL, '2024-11-10 12:00:00'),
    (2, 1, 1, '2026-02-10', '7.0', '155.83', '1.27', '1.0', 'Segunda compra', 1, NULL, NULL, '2026-02-10 12:00:00'),
    (3, 1, 2, '2026-06-15', '3.0', '161.11', '0.33', '1.0', NULL, 1, NULL, NULL, '2026-06-15 12:00:00'),
    (4, 2, 1, '2025-01-10', '100.0', '9.87', '0.5', '1.0', NULL, 1, NULL, NULL, '2025-01-10 12:00:00'),
    (5, 2, 2, '2026-06-15', '100.0', '10.92', '0.5', '1.0', 'Venda total', 3, NULL, NULL, '2026-06-15 12:00:00'),
    (6, 3, 1, '2026-03-05', '2.5', '480.1', '1.0', '5.4213', 'Compra no exterior', 1, NULL, NULL, '2026-03-05 12:00:00'),
    (7, 3, 2, '2026-07-01', '0.75', '495.2', '0.5', '5.3322', NULL, 1, NULL, NULL, '2026-07-01 12:00:00'),
    (8, 4, 1, '2026-04-01', '3.0', '190.0', '0.0', '6.1', NULL, 1, NULL, NULL, '2026-04-01 12:00:00'),
    (9, 5, 1, '2026-04-01', '20.0', '99.5', '0.0', '1.0', NULL, 2, 'pluggy-op-1', NULL, '2026-04-01 12:00:00'),
    (10, 6, 1, '2026-05-01', '33.0', '9.77', '0.0', '1.0', NULL, 1, NULL, NULL, '2026-05-01 12:00:00'),
    (11, 8, 1, '2026-01-15', '7.0', '33.33', '0.07', '1.0', NULL, 1, NULL, NULL, '2026-01-15 12:00:00'),
    (12, 9, 1, '2026-01-15', '1.333333', '147.77', '0.01', '1.0', NULL, 1, NULL, NULL, '2026-01-15 12:00:00'),
    (13, 9, 1, '2026-02-15', '0.666667', '149.13', '0.0', '1.0', NULL, 1, NULL, NULL, '2026-02-15 12:00:00');

INSERT INTO ProventoAtivo (AtivoId, DataCom, ValorPorCota) VALUES
    (1, '2024-10-31', '1.05'), (1, '2024-12-31', '1.1'), (1, '2026-07-31', '1.1'), (1, '2026-08-31', '1.1'), (1, '2026-09-30', '1.15'),
    (1, '2099-01-31', '9.0'),
    (2, '2025-08-20', '0.0185'), (2, '2026-08-20', '0.0185'),
    (3, '2026-03-20', '1.7'), (3, '2026-06-20', '1.72'), (3, '2026-09-20', '1.71'),
    (4, '2026-05-15', '2.2'),
    (5, '2026-09-30', '0.92'),
    (7, '2026-09-30', '0.5'),
    (9, '2026-09-30', '0.97');

INSERT INTO Investimento (ConexaoBancariaId, PluggyInvestmentId, Nome, Tipo, Subtipo, Codigo, Emissor, Moeda, Saldo, ValorAplicado,
    Rendimento, Quantidade, TaxaAnual, DataVencimento, DataAplicacao, Status, NomePersonalizado, AtualizadoEm) VALUES
    (1, 'inv-1', 'CDB - NU FINANCEIRA S.A. - SOCIEDADE DE CREDITO', 'FIXED_INCOME', 'CDB', NULL, 'NU FINANCEIRA S.A.', 'BRL', '12873.42', '12000.0',
     '873.42', NULL, '110.0', '2028-11-07', '2025-11-07', 'ACTIVE', NULL, '2026-10-14 09:00:00'),
    (1, 'inv-2', 'Caixinha', 'FIXED_INCOME', 'CDB', NULL, 'Banco Fictício', 'BRL', '999.9', '0.0', NULL, NULL, NULL, NULL, '2026-10-29', 'ACTIVE',
     '  Viagem de férias ', '2026-10-14 09:00:00'),
    (1, 'inv-3', 'Fundo Global', 'MUTUAL_FUND', 'INVESTMENT_FUND', NULL, NULL, 'GBP', '500.0', '400.0', NULL, NULL, NULL, NULL, '2025-06-01', 'ACTIVE', NULL, '2026-10-14 09:00:00'),
    (1, 'inv-4', 'Fundo em dólar', 'MUTUAL_FUND', 'INVESTMENT_FUND', NULL, NULL, 'USD', '100.5', '90.0', NULL, NULL, NULL, NULL, '2023-06-01', 'ACTIVE', NULL, '2026-10-14 09:00:00'),
    (1, 'inv-5', 'Previdência', 'SECURITY', 'PENSION', NULL, NULL, 'BRL', '3000.0', NULL, NULL, NULL, NULL, NULL, NULL, 'ACTIVE', NULL, '2026-10-14 09:00:00'),
    (1, 'inv-6', 'COE Fictício', 'coe', NULL, NULL, NULL, 'BRL', '700.0', '650.0', NULL, NULL, NULL, NULL, '2026-01-02', 'ACTIVE', NULL, '2026-10-14 09:00:00'),
    (1, 'inv-7', 'Outra coisa', 'OTHER', NULL, NULL, NULL, 'BRL', '20.0', NULL, NULL, NULL, NULL, NULL, NULL, 'ACTIVE', NULL, '2026-10-14 09:00:00'),
    -- O mesmo tipo com saldos que, pelo texto, ficariam em outra ordem ("100.5" antes de "9.5").
    (1, 'inv-8', 'Renda fixa pequena', 'FIXED_INCOME', 'LCI', NULL, NULL, 'BRL', '9.5', NULL, NULL, NULL, NULL, NULL, NULL, 'ACTIVE', NULL, '2026-10-14 09:00:00'),
    (1, 'inv-9', 'HGLG11', 'EQUITY', 'REAL_ESTATE_FUND', 'hglg11', NULL, 'BRL', '2850.0', '2700.0', NULL, '18.0', NULL, NULL, '2026-01-02', 'ACTIVE', NULL, '2026-10-14 09:00:00'),
    (1, 'inv-10', 'PETROBRAS PN', 'EQUITY', 'STOCK', 'PETR4', NULL, 'BRL', '387.7', NULL, NULL, '10.0', NULL, NULL, NULL, 'ACTIVE', NULL, '2026-10-14 09:00:00'),
    (1, 'inv-11', 'BOVA11', 'ETF', NULL, NULL, NULL, 'BRL', '600.0', NULL, NULL, '5.0', NULL, NULL, NULL, 'ACTIVE', NULL, '2026-10-14 09:00:00'),
    (1, 'inv-12', 'ACAO SEM CODIGO VALIDO', 'EQUITY', 'STOCK', NULL, NULL, 'BRL', '42.0', NULL, NULL, NULL, NULL, NULL, NULL, 'ACTIVE', NULL, '2026-10-14 09:00:00'),
    (1, 'inv-13', 'WEGE3', 'EQUITY', 'STOCK', 'WEGE3', NULL, 'BRL', '0.0', NULL, NULL, '4.0', NULL, NULL, NULL, 'ACTIVE', NULL, '2026-10-14 09:00:00'),
    (2, 'inv-20', 'Desligado', 'EQUITY', 'STOCK', 'ITSA4', NULL, 'BRL', '5000.0', '4000.0', NULL, '100.0', NULL, NULL, '2020-01-01', 'ACTIVE', NULL, '2026-10-14 09:00:00');

INSERT INTO ReservaConferencia (ContaId, Nome, SaldoConferido, DataConferencia, AtualizadoEm) VALUES
    (1, 'Viagem', '300.0', '2026-08-01', '2026-08-01 12:00:00');

-- O dólar e o euro: só o dólar tem cotação; a libra e o euro ficam sem.
INSERT INTO CotacaoMoeda (Moeda, Data, Compra, Venda, AtualizadoEm) VALUES
    ('USD', '2026-03-05', '5.4207', '5.4213', '2026-03-05 16:00:00'),
    ('USD', '2026-06-18', '5.3111', '5.3117', '2026-06-18 16:00:00'),
    ('USD', '2026-09-18', '5.3871', '5.3877', '2026-09-18 16:00:00'),
    ('USD', '2026-10-28', '5.4123', '5.4129', '2026-10-28 16:00:00');
