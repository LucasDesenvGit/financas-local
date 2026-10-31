-- Cenário fictício da conferência de investimentos (#299): o mesmo banco para a sonda em C# (que grava o gabarito
-- conferencia.json com o ConferenciaInvestimentosService de verdade) e para o teste do Go. Nada aqui é dado real.

INSERT INTO ConexaoBancaria (Id, PluggyItemId, NomeInstituicao, Status, Ativa, CriadoEm) VALUES (1, 'item-1', 'Corretora Fictícia', 'UPDATED', 1, '2025-01-01 00:00:00');

INSERT INTO Categoria (Id, Nome, Tipo) VALUES (1, 'Compras', 1), (2, 'Rendimentos', 2), (3, 'Investimentos', 3);

INSERT INTO RegraCategorizacao (Id, Texto, TipoCorrespondencia, CategoriaId, Prioridade, Ativa, CriadoEm) VALUES
    (1, 'VALOR RECEBIDO DE INVESTIMENTOS', 1, 2, 1, 1, '2026-01-01 00:00:00');

INSERT INTO Conta (Id, ConexaoBancariaId, PluggyAccountId, Nome, Tipo, Origem, Moeda, SaldoInicial, Saldo, Cor, IncluirNoSaldo, Arquivada, CriadoEm, AtualizadoEm) VALUES
    (1, 1, 'acc-1', 'Conta Corretora', 1, 2, 'BRL', '0.0', '0.0', '#000000', 1, 0, '2025-01-01 00:00:00', '2025-01-01 00:00:00');

INSERT INTO Ativo (Id, Ticker, Nome, Tipo, Moeda, PrecoManual, CriadoEm) VALUES
    (1, 'AAAA3', 'Empresa A', 2, 'BRL', 0, '2025-01-01 00:00:00'),
    (2, 'BBBB3', 'Empresa B', 2, 'BRL', 0, '2025-01-01 00:00:00'),
    (3, 'CCCC3', 'FII Fictício', 2, 'BRL', 0, '2025-01-01 00:00:00'),
    (4, 'DDDD3', 'Empresa D', 5, 'BRL', 0, '2025-01-01 00:00:00'),
    (5, 'EEEE3', 'Empresa E', 2, 'BRL', 0, '2025-01-01 00:00:00'),
    (6, 'FALT11', 'Fundo Faltando', 1, 'BRL', 0, '2025-01-01 00:00:00'),
    (7, 'LONG11', 'Fundo Longe', 1, 'BRL', 0, '2025-01-01 00:00:00'),
    (8, 'MENS11', 'Fundo Mensal', 1, 'BRL', 0, '2025-01-01 00:00:00'),
    (9, 'AAPL', 'Apple', 2, 'USD', 0, '2025-01-01 00:00:00'),
    (10, 'SOBR3', 'Cotas a mais', 2, 'BRL', 0, '2025-01-01 00:00:00'),
    (11, 'IGUA3', 'Cotas iguais', 2, 'BRL', 0, '2025-01-01 00:00:00');

INSERT INTO OperacaoAtivo (Id, AtivoId, Tipo, Data, Quantidade, PrecoUnitario, Taxas, Cambio, Origem, TransacaoId, CriadoEm) VALUES
    (1, 1, 1, '2026-03-02', '10.0', '10.0', '0.0', '1.0', 1, NULL, '2026-03-02 12:00:00'),
    (2, 2, 1, '2026-03-09', '5.0', '20.0', '0.0', '1.0', 1, NULL, '2026-03-09 12:00:00'),
    (3, 2, 1, '2026-03-09', '5.0', '20.0', '0.0', '1.0', 1, NULL, '2026-03-09 12:00:00'),
    (4, 4, 1, '2026-03-16', '10.0', '30.0', '0.5', '1.0', 3, 121, '2026-03-16 12:00:00'),
    (5, 5, 2, '2026-04-12', '1.0', '100.0', '0.5', '1.0', 1, NULL, '2026-04-12 12:00:00'),
    (6, 5, 1, '2026-01-02', '1.0', '100.0', '0.0', '1.0', 1, NULL, '2026-01-02 12:00:00'),
    (7, 3, 1, '2026-03-24', '4.0', '100.0', '0.0', '1.0', 1, NULL, '2026-03-24 12:00:00'),
    (8, 5, 1, '2026-03-30', '5.0', '100.0', '0.0', '1.0', 1, NULL, '2026-03-30 12:00:00'),
    (9, 9, 1, '2026-03-02', '1.0', '100.0', '0.0', '5.0', 1, NULL, '2026-03-02 12:00:00'),
    (10, 10, 1, '2026-01-02', '9.0', '10.0', '0.0', '1.0', 1, NULL, '2026-01-02 12:00:00'),
    (11, 11, 1, '2026-01-02', '3.0', '10.0', '0.0', '1.0', 1, NULL, '2026-01-02 12:00:00'),
    (12, 7, 1, '2026-01-02', '1.0', '10.0', '0.0', '1.0', 1, NULL, '2026-01-02 12:00:00'),
    (13, 8, 1, '2026-01-02', '1.0', '10.0', '0.0', '1.0', 1, NULL, '2026-01-02 12:00:00');

INSERT INTO PrecoMensalAtivo (AtivoId, Mes, Fechamento) VALUES (8, '2026-05-01', '50.0');

INSERT INTO Investimento (ConexaoBancariaId, PluggyInvestmentId, Nome, Tipo, Subtipo, Codigo, Emissor, Moeda, Saldo, Quantidade, AtualizadoEm) VALUES
    (1, 'i-1', 'Fundo Faltando', 'EQUITY', 'STOCK', 'FALT11', NULL, 'BRL', '1000.0', '20.0', '2026-05-20 12:00:00'),
    (1, 'i-2', 'Fundo Longe', 'EQUITY', 'STOCK', 'LONG11', NULL, 'BRL', '1000.0', '6.0', '2026-05-20 12:00:00'),
    (1, 'i-3', 'Fundo Mensal', 'EQUITY', 'STOCK', 'MENS11', NULL, 'BRL', '1000.0', '2.0', '2026-05-20 12:00:00'),
    (1, 'i-4', 'Cotas a mais', 'EQUITY', 'STOCK', 'SOBR3', NULL, 'BRL', '100.0', '5.0', '2026-05-20 12:00:00'),
    (1, 'i-5', 'Cotas iguais', 'EQUITY', 'STOCK', 'IGUA3', NULL, 'BRL', '100.0', '3.0', '2026-05-20 12:00:00'),
    (1, 'i-6', 'Empresa D', 'EQUITY', 'STOCK', 'DDDD3', 'Fundo de Investimento Imobiliário D', 'BRL', '100.0', '10.0', '2026-05-20 12:00:00');

INSERT INTO Transacao (Id, ContaId, Data, Descricao, DescricaoOriginal, Valor, Tipo, Status, CategoriaId, CategoriaManual, IgnorarRelatorios, Emergencia, DescricaoRevisada, DescricaoManual, CriadoEm, AtualizadoEm) VALUES
    (101, 1, '2026-03-02', 'Compra de Renda Variável', NULL, '-102.0', 1, 1, NULL, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (102, 1, '2026-03-03', 'Valor recebido de Investimentos', NULL, '1.5', 2, 1, 2, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (103, 1, '2026-03-06', 'Valor recebido de Investimentos', NULL, '0.4', 2, 1, 2, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (111, 1, '2026-03-10', 'Compra de Renda Variável', NULL, '-204.0', 1, 1, NULL, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (112, 1, '2026-03-11', 'Valor recebido de Investimentos', NULL, '3.9', 2, 1, 2, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (113, 1, '2026-03-11', 'Valor recebido de Investimentos', NULL, '50.0', 2, 1, 2, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (121, 1, '2026-03-16', 'Compra de Renda Variável', NULL, '-305.5', 1, 1, NULL, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (122, 1, '2026-03-17', 'Valor recebido de Investimentos', NULL, '5.0', 2, 1, 2, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (131, 1, '2026-05-05', 'Compra de Renda Variável', NULL, '-1010.0', 1, 1, NULL, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (132, 1, '2026-05-06', 'Valor recebido de Investimentos', NULL, '10.0', 2, 1, 2, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (133, 1, '2026-05-05', 'Compra de Renda Variável', NULL, '-77.0', 1, 1, NULL, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (141, 1, '2026-04-14', 'Valor recebido de Investimentos', NULL, '99.5', 2, 1, 2, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (151, 1, '2026-04-20', 'Valor recebido de Investimentos', NULL, '12.0', 3, 1, 3, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (161, 1, '2026-03-24', 'Compra de Renda Variável', NULL, '-408.0', 1, 1, NULL, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (162, 1, '2026-03-24', 'Valor recebido de Investimentos', NULL, '7.0', 3, 1, 3, 1, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (171, 1, '2026-03-30', 'Compra de Renda Variável', NULL, '-510.0', 1, 1, NULL, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (172, 1, '2026-03-31', 'Valor recebido de Investimentos', NULL, '9.9', 2, 1, 2, 1, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (181, 1, '2026-04-20', 'Resgate de Investimentos', NULL, '30.0', 2, 1, NULL, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (182, 1, '2026-04-20', 'Fundo de Investimentos', NULL, '31.0', 2, 1, NULL, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (191, 1, '2026-03-02', 'Venda de Renda Variável', NULL, '-1.0', 1, 1, NULL, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (192, 1, '2026-03-02', 'Pix', 'Compra de renda variavel', '-1.0', 1, 1, NULL, 0, 0, 0, 0, 0, '2026-01-01 12:00:00', '2026-01-01 12:00:00');
