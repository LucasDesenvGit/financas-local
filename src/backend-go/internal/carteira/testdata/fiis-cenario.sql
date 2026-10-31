-- Cenário fictício dos FIIs (#82): o mesmo banco para a sonda em C# (que grava o gabarito fiis.json com o FiiService de
-- verdade) e para o teste do Go. Nada aqui é dado real.

INSERT INTO ConexaoBancaria (Id, PluggyItemId, NomeInstituicao, Status, Ativa, CriadoEm) VALUES (1, 'item-1', 'Corretora Fictícia', 'UPDATED', 1, '2025-01-01 00:00:00');

INSERT INTO Categoria (Id, Nome, Tipo) VALUES (1, 'Compras', 1), (2, 'Rendimentos', 2), (3, 'Investimentos', 3);

INSERT INTO Conta (Id, ConexaoBancariaId, PluggyAccountId, Nome, Tipo, Origem, Moeda, SaldoInicial, Saldo, Cor, IncluirNoSaldo, Arquivada, CriadoEm, AtualizadoEm) VALUES
    (1, 1, 'acc-1', 'Conta Corretora', 1, 2, 'BRL', '0.0', '0.0', '#000000', 1, 0, '2025-01-01 00:00:00', '2025-01-01 00:00:00');

INSERT INTO Ativo (Id, Ticker, Nome, Tipo, Moeda, PrecoAtual, PrecoAnterior, PrecoManual, FonteCotacao, Segmento, CriadoEm) VALUES
    (1, 'HGLG11', 'Com sugestão de segmento', 1, 'BRL', '160.5', '158.0', 0, 'yahoo', NULL, '2025-01-01 00:00:00'),
    (2, 'XPML11', 'Sem cotação', 1, 'BRL', NULL, NULL, 0, NULL, 'Shoppings', '2025-01-01 00:00:00'),
    (3, 'ABCD11', 'Sem segmento', 1, 'BRL', '9.87', NULL, 0, 'brapi', NULL, '2025-01-01 00:00:00'),
    (4, 'ABCD12', 'Direito de subscrição', 1, 'BRL', '1.25', '0.0', 0, 'yahoo', NULL, '2025-01-01 00:00:00'),
    (5, 'ZERA11', 'Vendido por inteiro', 1, 'BRL', '60.0', '61.0', 0, 'yahoo', NULL, '2025-01-01 00:00:00'),
    (6, 'SEMO11', 'Sem operações', 1, 'BRL', '10.0', '10.0', 0, 'yahoo', NULL, '2025-01-01 00:00:00'),
    (7, 'PETR4', 'Ação fica de fora', 2, 'BRL', '30.0', '29.0', 0, 'yahoo', NULL, '2025-01-01 00:00:00'),
    (8, 'VELH11', 'Mais de 180 meses', 1, 'BRL', '100.0', '100.0', 1, NULL, 'Híbrido', '2025-01-01 00:00:00'),
    (9, 'NOVO11', 'Comprado neste mês', 1, 'BRL', NULL, NULL, 0, NULL, NULL, '2025-01-01 00:00:00'),
    (10, 'TAXA11', 'Sem cotação nem preço', 1, 'BRL', NULL, NULL, 0, NULL, NULL, '2025-01-01 00:00:00');

INSERT INTO OperacaoAtivo (Id, AtivoId, Tipo, Data, Quantidade, PrecoUnitario, Taxas, Cambio, Origem, TransacaoId, CriadoEm) VALUES
    (1, 1, 1, '2025-03-10', '10.0', '150.0', '1.25', '1.0', 1, 101, '2025-03-10 12:00:00'),
    (2, 1, 1, '2026-02-10', '5.0', '155.5', '0.0', '1.0', 3, 102, '2026-02-10 12:00:00'),
    (3, 1, 2, '2026-08-05', '3.0', '165.0', '0.5', '1.0', 1, 103, '2026-08-05 12:00:00'),
    (4, 2, 1, '2025-11-20', '7.0', '99.99', '0.0', '1.0', 1, NULL, '2025-11-20 12:00:00'),
    (5, 3, 1, '2026-01-15', '100.0', '8.5', '0.1', '1.0', 1, 104, '2026-01-15 12:00:00'),
    (6, 4, 1, '2026-03-16', '10.0', '1.0', '0.0', '1.0', 1, 105, '2026-03-16 12:00:00'),
    (7, 5, 1, '2025-12-01', '4.0', '50.0', '0.0', '1.0', 1, NULL, '2025-12-01 12:00:00'),
    (8, 5, 2, '2026-04-01', '4.0', '60.0', '0.0', '1.0', 1, NULL, '2026-04-01 12:00:00'),
    (9, 7, 1, '2026-01-05', '10.0', '25.0', '0.0', '1.0', 1, NULL, '2026-01-05 12:00:00'),
    (10, 8, 1, '2008-03-03', '2.0', '80.0', '0.0', '1.0', 1, NULL, '2008-03-03 12:00:00'),
    (11, 9, 1, '2026-10-20', '1.0', '10.0', '0.0', '1.0', 1, NULL, '2026-10-20 12:00:00'),
    (12, 10, 1, '2026-06-02', '1.0', '0.0', '0.0', '1.0', 1, NULL, '2026-06-02 12:00:00'),
    (13, 2, 2, '2026-01-10', '1.0', '100.0', '0.0', '1.0', 1, NULL, '2026-01-10 12:00:00'),
    (14, 2, 1, '2026-02-10', '1.0', '100.0', '0.0', '1.0', 1, NULL, '2026-02-10 12:00:00');

INSERT INTO ProventoAtivo (AtivoId, DataCom, ValorPorCota) VALUES
    (1, '2025-02-28', '1.1'), (1, '2025-03-31', '1.1'), (1, '2025-04-30', '1.1'), (1, '2025-10-31', '1.12'), (1, '2025-12-30', '1.12'),
    (1, '2026-02-27', '1.13'), (1, '2026-03-31', '1.13'), (1, '2026-07-31', '1.1'), (1, '2026-08-31', '1.1'), (1, '2026-09-30', '1.15'),
    (1, '2026-11-30', '9.99'),
    (2, '2026-05-29', '0.92'), (2, '2026-06-30', '0.9'),
    (3, '2026-03-31', '0.085'),
    (5, '2026-01-30', '0.5'),
    (7, '2026-06-30', '1.0'),
    (8, '2008-04-30', '0.8'), (8, '2026-09-30', '0.8');

INSERT INTO PrecoMensalAtivo (AtivoId, Mes, Fechamento) VALUES
    (1, '2025-12-01', '152.0'), (1, '2026-01-01', '153.0'), (1, '2026-03-01', '156.0'), (1, '2026-10-01', '159.9'),
    (1, '2023-01-01', '120.0'), (1, '2023-10-01', '121.0'), (1, '2023-11-01', '122.0'), (1, '2023-12-01', '123.0'),
    (3, '2026-02-01', '8.7'),
    (9, '2026-10-01', '11.5');

INSERT INTO IndiceMensal (Codigo, Mes, Fechamento) VALUES
    ('IFIX', '2011-10-01', '1000.0'), ('IFIX', '2025-12-01', '3300.0'), ('IFIX', '2026-08-01', '3400.0'), ('IFIX', '2026-09-01', '3450.0'),
    ('IBOV', '2026-09-01', '1.0');

INSERT INTO Transacao (Id, ContaId, Data, Descricao, DescricaoOriginal, Valor, Tipo, Status, CategoriaId, CategoriaManual, IgnorarRelatorios, Emergencia, DescricaoRevisada, DescricaoManual, CriadoEm, AtualizadoEm) VALUES
    (101, 1, '2025-03-10', 'Compra de cotas', NULL, '-1501.25', 3, 1, 3, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (102, 1, '2026-02-10', 'Compra de cotas', NULL, '-777.5', 3, 1, 3, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (103, 1, '2026-08-05', 'Venda de cotas', NULL, '494.5', 3, 1, 3, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (104, 1, '2026-01-15', 'Compra de cotas', NULL, '-860.0', 3, 1, 3, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (105, 1, '2026-03-16', 'Compra de cotas', NULL, '-10.0', 3, 1, 3, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    -- Com o código: 12 cotas × 1,10.
    (201, 1, '2026-09-10', 'RENDIMENTOS HGLG11', NULL, '13.2', 2, 1, 2, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    -- Sem o código: 12 cotas × 1,15, ligado ao fundo pelo valor.
    (202, 1, '2026-10-05', 'Valor recebido de Investimentos', NULL, '13.8', 2, 1, 2, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    -- Não bate com provento nenhum: fica sem fundo.
    (203, 1, '2026-07-10', 'Valor recebido de Investimentos', NULL, '7.77', 2, 1, 2, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    -- Ação: fora dos recebidos.
    (204, 1, '2026-07-10', 'RENDIMENTOS PETR4', NULL, '10.0', 2, 1, 2, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    -- 7 cotas × 0,92: o primeiro pega o provento, e o segundo, igual, fica sem.
    (205, 1, '2026-06-10', 'Valor recebido de Investimentos', NULL, '6.44', 2, 1, 2, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (206, 1, '2026-06-12', 'Valor recebido de Investimentos', NULL, '6.44', 2, 1, 2, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    -- Depois dos 40 dias da data-com: fica sem fundo.
    (207, 1, '2026-08-20', 'Valor recebido de Investimentos', NULL, '6.3', 2, 1, 2, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    -- 100 cotas × 0,085, com 0,04 de diferença: dentro da tolerância.
    (208, 1, '2026-04-10', 'Valor recebido de Investimentos', NULL, '8.54', 2, 1, 2, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    -- Com o código e o valor certo (15 cotas × 1,10): guarda o provento, e o sem código do lado fica sem fundo.
    (209, 1, '2026-08-10', 'RENDIMENTOS HGLG11', NULL, '16.5', 2, 1, 2, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (210, 1, '2026-08-11', 'Valor recebido de Investimentos', NULL, '16.5', 2, 1, 2, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    -- Dois créditos do mesmo fundo no mês: uma linha só.
    (212, 1, '2026-08-12', 'RENDIMENTOS HGLG11', NULL, '1.0', 2, 1, 2, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    -- Dois fundos pagam o mesmo mês com valores parecidos: fica o mais perto do valor.
    (211, 1, '2026-02-10', 'Valor recebido de Investimentos', NULL, '2.0', 2, 1, 2, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00');
