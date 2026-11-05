-- Cenário fictício da conferência do banco (#445): o mesmo banco para a sonda em C# (que grava o gabarito conferencia.json
-- com o ConferenciaBancoService de verdade) e para o teste do Go. Nada aqui é dado real. @MES é o mês em que a sonda rodou
-- ("2026-10"), porque a fatura e os estornos são os do mês de hoje.

INSERT INTO ConexaoBancaria (Id, PluggyItemId, NomeInstituicao, Status, Ativa, UltimaAtualizacaoInstituicao, CriadoEm) VALUES
    (1, 'item-1', 'Banco Fictício', 'UPDATED', 1, '2026-10-30 02:30:00', '2025-01-01 00:00:00'),
    (2, 'item-2', 'Sem data', 'UPDATED', 1, NULL, '2025-01-01 00:00:00'),
    (3, 'item-3', 'Desligada', 'UPDATED', 0, '2026-10-30 12:00:00', '2025-01-01 00:00:00');

INSERT INTO Categoria (Id, Nome, Tipo) VALUES (1, 'Compras', 1), (2, 'Salário', 2);

INSERT INTO Conta (Id, ConexaoBancariaId, PluggyAccountId, Nome, Tipo, Origem, Moeda, SaldoInicial, Saldo, LimiteCredito, LimiteDisponivel, DiaFechamento, DiaVencimento, Cor, IncluirNoSaldo, Arquivada, CriadoEm, AtualizadoEm) VALUES
    (1, 1, 'acc-1', 'Corrente do banco', 1, 2, 'BRL', '0.0', '1500.5', NULL, NULL, NULL, NULL, '#000000', 1, 0, '2025-01-01 00:00:00', '2026-10-29 12:00:00'),
    (2, 2, 'acc-2', 'Poupança sem data', 2, 2, 'BRL', '0.0', '300.0', NULL, NULL, NULL, NULL, '#000000', 1, 0, '2025-01-01 00:00:00', '2026-10-28 02:00:00'),
    (3, NULL, NULL, 'Dinheiro', 5, 1, 'BRL', '100.0', '0.0', NULL, NULL, NULL, NULL, '#000000', 1, 0, '2025-01-01 00:00:00', '2025-01-01 00:00:00'),
    (4, NULL, NULL, 'Arquivada', 1, 1, 'BRL', '100.0', '0.0', NULL, NULL, NULL, NULL, '#000000', 1, 1, '2025-01-01 00:00:00', '2025-01-01 00:00:00'),
    (5, 1, 'acc-5', 'Cartão com fatura', 3, 2, 'BRL', '0.0', '-900.0', '5000.0', '4100.0', 5, 12, '#000000', 1, 0, '2025-01-01 00:00:00', '2026-10-29 12:00:00'),
    (6, NULL, NULL, 'Cartão manual', 3, 1, 'BRL', '0.0', '0.0', NULL, NULL, NULL, NULL, '#000000', 1, 0, '2025-01-01 00:00:00', '2025-01-01 00:00:00'),
    (7, 2, 'acc-7', 'Cartão com crédito', 3, 2, 'BRL', '0.0', '50.0', '1000.0', '1050.0', 20, 27, '#000000', 1, 0, '2025-01-01 00:00:00', '2026-10-29 12:00:00');

INSERT INTO FaturaCartao (Id, ContaId, PluggyBillId, DataVencimento, DataFechamento, ValorTotal, ValorMinimo, ValorPago, AtualizadoEm) VALUES
    (1, 5, 'bill-1', '@MES-12', '@MES-05', '400.0', '60.0', NULL, '2026-10-14 09:00:00');

INSERT INTO Transacao (Id, ContaId, Data, Descricao, DescricaoOriginal, Valor, Tipo, Status, CategoriaId, CategoriaManual, IgnorarRelatorios, Emergencia, DescricaoRevisada, DescricaoManual, ParcelaNumero, ParcelaTotal, TransferenciaGrupo, CriadoEm, AtualizadoEm) VALUES
    -- Pendentes: um na conta do banco, dois na manual (com sinais diferentes), um e dois nos cartões.
    (1, 1, '@MES-02', 'Mercado', NULL, '-50.0', 1, 2, 1, 0, 0, 0, 0, 0, NULL, NULL, NULL, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (2, 3, '@MES-02', 'Entrada', NULL, '30.0', 2, 2, 2, 0, 0, 0, 0, 0, NULL, NULL, NULL, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (3, 3, '@MES-02', 'Saída', NULL, '-100.0', 1, 2, 1, 0, 0, 0, 0, 0, NULL, NULL, NULL, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (4, 5, '@MES-03', 'Compra pendente', NULL, '-20.0', 1, 2, 1, 0, 0, 0, 0, 0, NULL, NULL, NULL, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (5, 6, '@MES-03', 'Compra um', NULL, '-11.0', 1, 2, 1, 0, 0, 0, 0, 0, NULL, NULL, NULL, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (6, 6, '@MES-03', 'Compra dois', NULL, '-12.0', 1, 2, 1, 0, 0, 0, 0, 0, NULL, NULL, NULL, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    -- Estorno no mês; crédito que não é despesa e transferência ficam de fora.
    (7, 5, '@MES-04', 'Estorno da loja', NULL, '35.5', 1, 1, 1, 0, 0, 0, 0, 0, NULL, NULL, NULL, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (8, 5, '@MES-04', 'Pagamento recebido', NULL, '100.0', 3, 1, NULL, 0, 0, 0, 0, 0, NULL, NULL, NULL, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (9, 5, '@MES-04', 'Transferência', NULL, '77.0', 1, 1, 1, 0, 0, 0, 0, 0, NULL, NULL, '00000000-0000-4000-8000-000000000001', '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    -- Compras repetidas: o mesmo dia, valor e texto (com espaços e sem diferença de maiúsculas); outro dia não repete, parcela
    -- não conta.
    (10, 5, '@MES-04', 'Padaria ', NULL, '-9.5', 1, 1, 1, 0, 0, 0, 0, 0, NULL, NULL, NULL, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (11, 5, '@MES-04', 'PADARIA', NULL, '-9.5', 1, 1, 1, 0, 0, 0, 0, 0, NULL, NULL, NULL, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (12, 5, '@MES-04', 'padaria', NULL, '-9.5', 1, 1, 1, 0, 0, 0, 0, 0, NULL, NULL, NULL, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (13, 5, '@MES-06', 'Padaria', NULL, '-9.5', 1, 1, 1, 0, 0, 0, 0, 0, NULL, NULL, NULL, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (14, 5, '@MES-04', 'Parcelado', NULL, '-40.0', 1, 1, 1, 0, 0, 0, 0, 0, 1, 2, NULL, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (15, 5, '@MES-04', 'Parcelado', NULL, '-40.0', 1, 1, 1, 0, 0, 0, 0, 0, 2, 2, NULL, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (16, 6, '@MES-07', 'Assinatura', NULL, '-5.0', 1, 1, 1, 0, 0, 0, 0, 0, NULL, NULL, NULL, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (17, 6, '@MES-07', 'Assinatura', NULL, '-5.0', 1, 1, 1, 0, 0, 0, 0, 0, NULL, NULL, NULL, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (18, 6, '@MES-08', 'Curso', NULL, '-30.0', 1, 1, 1, 0, 0, 0, 0, 0, NULL, NULL, NULL, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (19, 6, '@MES-08', 'Curso', NULL, '-30.0', 1, 1, 1, 0, 0, 0, 0, 0, NULL, NULL, NULL, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    -- Reservas pelo extrato: uma sem conferência, uma conferida.
    (20, 1, '2026-09-10', 'Aplicação em Caixinha viagem', NULL, '-200.0', 3, 1, NULL, 0, 0, 0, 0, 0, NULL, NULL, NULL, '2026-01-01 12:00:00', '2026-01-01 12:00:00'),
    (21, 1, '2026-09-11', 'Aplicação em Caixinha casa', NULL, '-300.0', 3, 1, NULL, 0, 0, 0, 0, 0, NULL, NULL, NULL, '2026-01-01 12:00:00', '2026-01-01 12:00:00');

INSERT INTO ReservaConferencia (ContaId, Nome, SaldoConferido, DataConferencia, AtualizadoEm) VALUES
    (1, 'Caixinha casa', '310.25', '2026-09-20', '2026-09-20 12:00:00');

INSERT INTO Investimento (Id, ConexaoBancariaId, PluggyInvestmentId, Nome, Tipo, Subtipo, Codigo, Emissor, Moeda, Saldo, NomePersonalizado, AtualizadoEm) VALUES
    (1, 1, 'i-1', 'CDB BANCO FICTICIO', 'FIXED_INCOME', 'CDB', NULL, 'Banco Fictício S.A.', 'BRL', '1000.0', NULL, '2026-10-29 12:00:00'),
    (2, 2, 'i-2', 'Fundo sem data', 'MUTUAL_FUND', NULL, NULL, NULL, 'BRL', '2500.75', 'Meu fundo', '2026-10-28 02:00:00'),
    (3, 1, 'i-3', 'Apple', 'EQUITY', 'STOCK', 'AAPL', NULL, 'USD', '100.0', NULL, '2026-10-29 12:00:00'),
    (4, 1, 'i-4', 'Sem cotação', 'EQUITY', 'STOCK', 'XPTO', NULL, 'JPY', '100.0', NULL, '2026-10-29 12:00:00'),
    (5, 3, 'i-5', 'Da conexão desligada', 'EQUITY', 'STOCK', 'PETR4', NULL, 'BRL', '999.0', NULL, '2026-10-29 12:00:00');

INSERT INTO CotacaoMoeda (Moeda, Data, Compra, Venda, AtualizadoEm) VALUES
    ('USD', '2026-10-01', '5.1', '5.2', '2026-10-01 12:00:00');

INSERT INTO Ativo (Id, Ticker, Nome, Tipo, Moeda, PrecoAtual, PrecoManual, CriadoEm) VALUES
    (1, 'PETR4', 'Com cotação', 2, 'BRL', '30.5', 0, '2025-01-01 00:00:00'),
    (2, 'VALE3', 'Sem cotação', 2, 'BRL', NULL, 0, '2025-01-01 00:00:00'),
    (3, 'AAPL', 'No exterior', 2, 'USD', '200.0', 0, '2025-01-01 00:00:00'),
    (4, 'ITSA4', 'Vendido', 2, 'BRL', '10.0', 0, '2025-01-01 00:00:00');

INSERT INTO OperacaoAtivo (Id, AtivoId, Tipo, Data, Quantidade, PrecoUnitario, Taxas, Cambio, Origem, CriadoEm) VALUES
    (1, 1, 1, '2026-01-05', '10.0', '25.0', '0.5', '1.0', 1, '2026-01-05 12:00:00'),
    (2, 2, 1, '2026-01-05', '3.0', '60.0', '0.0', '1.0', 1, '2026-01-05 12:00:00'),
    (3, 3, 1, '2026-01-05', '0.5', '180.0', '0.0', '5.0', 1, '2026-01-05 12:00:00'),
    (4, 4, 1, '2026-01-05', '2.0', '10.0', '0.0', '1.0', 1, '2026-01-05 12:00:00'),
    (5, 4, 2, '2026-02-05', '2.0', '11.0', '0.0', '1.0', 1, '2026-02-05 12:00:00');
