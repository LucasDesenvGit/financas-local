-- Cenário fictício das cotações (#82): o mesmo banco para a sonda em C# (que grava o gabarito atualizar.json com o
-- CotacaoService de verdade, respondendo com testdata/respostas.json) e para o teste do Go. Nada aqui é dado real.

INSERT INTO Ativo (Id, Ticker, Nome, Tipo, Moeda, PrecoAtual, PrecoAnterior, PrecoAtualizadoEm, PrecoManual, FonteCotacao, Segmento, CriadoEm) VALUES
    (1, 'AAAA11', NULL, 2, 'BRL', NULL, NULL, NULL, 0, NULL, NULL, '2025-01-01 00:00:00'),
    (2, 'BBBB3', 'Nome guardado', 2, 'BRL', NULL, NULL, NULL, 0, NULL, NULL, '2025-01-01 00:00:00'),
    (3, 'CCCC3', NULL, 2, 'BRL', '77.0', NULL, NULL, 1, 'Manual', NULL, '2025-01-01 00:00:00'),
    (4, 'DDD', NULL, 2, 'USD', NULL, NULL, NULL, 0, NULL, NULL, '2025-01-01 00:00:00'),
    (5, 'EEE.L', NULL, 2, 'GBP', NULL, NULL, NULL, 0, NULL, NULL, '2025-01-01 00:00:00'),
    (6, 'FFFF3', '   ', 2, 'BRL', NULL, NULL, NULL, 0, NULL, NULL, '2025-01-01 00:00:00'),
    (7, 'GGGG3', NULL, 2, 'BRL', NULL, NULL, NULL, 0, NULL, NULL, '2025-01-01 00:00:00'),
    (8, 'HHHH3', NULL, 2, 'BRL', NULL, NULL, NULL, 0, NULL, NULL, '2025-01-01 00:00:00'),
    (9, 'IIII3', NULL, 2, 'BRL', NULL, NULL, NULL, 0, NULL, NULL, '2025-01-01 00:00:00'),
    (10, 'JJJJ3', NULL, 2, 'BRL', NULL, NULL, NULL, 0, NULL, NULL, '2025-01-01 00:00:00'),
    (11, 'KKKK3', NULL, 2, 'BRL', NULL, NULL, NULL, 0, NULL, NULL, '2025-01-01 00:00:00'),
    (12, 'LLLL3', NULL, 2, 'BRL', NULL, NULL, NULL, 0, NULL, NULL, '2025-01-01 00:00:00');

INSERT INTO OperacaoAtivo (AtivoId, Tipo, Data, Quantidade, PrecoUnitario, Taxas, Cambio, Origem, CriadoEm) VALUES
    (2, 1, '2026-01-10', '10.0', '10.0', '0.0', '1.0', 1, '2026-01-10 12:00:00'),
    (4, 1, '2026-01-10', '1.0', '100.0', '0.0', '5.0', 1, '2026-01-10 12:00:00'),
    (5, 1, '2026-01-10', '1.0', '100.0', '0.0', '7.0', 1, '2026-01-10 12:00:00');

-- O provento e o fechamento já guardados: o provento fica, o fechamento é corrigido.
INSERT INTO ProventoAtivo (AtivoId, DataCom, ValorPorCota) VALUES (2, '2026-06-01', '0.1');
INSERT INTO PrecoMensalAtivo (AtivoId, Mes, Fechamento) VALUES (2, '2026-08-01', '1.0');
INSERT INTO IndiceMensal (Codigo, Mes, Fechamento) VALUES ('IFIX', '2026-08-01', '1.0');

-- O histórico das moedas já cobre a primeira compra: nada de ir ao Banco Central.
INSERT INTO CotacaoMoeda (Moeda, Data, Compra, Venda, AtualizadoEm) VALUES
    ('USD', '2026-01-09', '5.0', '5.1', '2026-01-09 16:00:00'),
    ('GBP', '2026-01-09', '7.0', '7.1', '2026-01-09 16:00:00');
