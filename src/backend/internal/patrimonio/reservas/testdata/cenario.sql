-- Cenário fictício das reservas (#82): o mesmo banco para a sonda em C# (que grava o gabarito listar.json com o
-- ReservaService de verdade) e para o teste do Go. Nada aqui é dado real.
--
-- Conta 1: o extrato de reservas de um banco tipo Mercado Pago (a "Viagem" com conferência antiga, a "Casa" que
--          ficaria negativa, o nome em outra caixa, a descrição renomeada, o pendente, o sem nome) e o Méli Dólar
--          (com e sem a quantidade na descrição, o "Méli" com acento, que o LIKE achava só depois da #567, a conferência no
--          meio).
-- Conta 2: Open Finance com a conexão desligada: os lançamentos ficam de fora; a conferência, não.
-- Conta 3: arquivada. Conta 6: fora do saldo. Conta 8: manual. Nada delas aparece.
-- Conta 4: o banco informa só o total reservado: a linha "Reservado (restante)", porque já há uma reserva "Reservado".
-- Conta 5: o banco informa a lista detalhada: a "Caixa" com o nome do extrato, a "Nova", e a "Outra" fora da lista.
-- Conta 7: só a conferência do Méli Dólar, sem lançamentos.

INSERT INTO ConexaoBancaria (Id, PluggyItemId, NomeInstituicao, Status, Ativa, CriadoEm) VALUES
    (1, 'item-reservas-1', 'Banco Fictício', 'UPDATED', 1, '2026-01-01 00:00:00'),
    (2, 'item-reservas-2', 'Banco Desligado', 'LOGIN_ERROR', 0, '2026-01-01 00:00:00');

INSERT INTO Conta (Id, ConexaoBancariaId, PluggyAccountId, Nome, Tipo, Origem, Moeda, SaldoInicial, Saldo, Cor, IncluirNoSaldo,
    Arquivada, CriadoEm, AtualizadoEm) VALUES
    (1, 1, 'acc-1', 'Conta Mercado', 1, 2, 'BRL', '0.0', '0.0', '#000000', 1, 0, '2026-01-01 00:00:00', '2026-01-01 00:00:00'),
    (2, 2, 'acc-2', 'Conta Desligada', 1, 2, 'BRL', '0.0', '0.0', '#000000', 1, 0, '2026-01-01 00:00:00', '2026-01-01 00:00:00'),
    (3, 1, 'acc-3', 'Conta Arquivada', 1, 2, 'BRL', '0.0', '0.0', '#000000', 1, 1, '2026-01-01 00:00:00', '2026-01-01 00:00:00'),
    (4, 1, 'acc-4', 'Conta Total', 1, 2, 'BRL', '0.0', '0.0', '#000000', 1, 0, '2026-01-01 00:00:00', '2026-01-01 00:00:00'),
    (5, 1, 'acc-5', 'Conta Lista', 1, 2, 'BRL', '0.0', '0.0', '#000000', 1, 0, '2026-01-01 00:00:00', '2026-01-01 00:00:00'),
    (6, 1, 'acc-6', 'Conta Fora', 1, 2, 'BRL', '0.0', '0.0', '#000000', 0, 0, '2026-01-01 00:00:00', '2026-01-01 00:00:00'),
    (7, 1, 'acc-7', 'Conta Dólar', 1, 2, 'BRL', '0.0', '0.0', '#000000', 1, 0, '2026-01-01 00:00:00', '2026-01-01 00:00:00'),
    (8, NULL, NULL, 'Carteira', 5, 1, 'BRL', '0.0', '0.0', '#000000', 1, 0, '2026-01-01 00:00:00', '2026-01-01 00:00:00');

UPDATE Conta SET SaldoReservado = '500.0' WHERE Id = 4;
UPDATE Conta SET SaldoReservado = '300.0', ReservasInformadas = '[{"nome":"Caixa","valor":250.555},{"nome":"Nova","valor":49.0}]'
    WHERE Id = 5;

INSERT INTO Transacao (ContaId, PluggyTransactionId, Data, Descricao, DescricaoOriginal, Valor, Tipo, Status, CategoriaManual,
    IgnorarRelatorios, Emergencia, DescricaoRevisada, DescricaoManual, CriadoEm, AtualizadoEm) VALUES
    -- Conta 1: o primeiro lançamento importado é de 01/08 (o começo da janela).
    (1, 'r-01', '2026-08-01', 'PIX RECEBIDO', 'PIX RECEBIDO', '1000.0', 2, 1, 0, 0, 0, 0, 0, '2026-08-01 12:00:00', '2026-08-01 12:00:00'),
    (1, 'r-02', '2026-08-05', 'Dinheiro reservado Viagem', 'Dinheiro reservado Viagem', '-100.00', 1, 1, 0, 0, 0, 0, 0, '2026-08-05 12:00:00', '2026-08-05 12:00:00'),
    (1, 'r-03', '2026-08-20', 'Viagem (renomeada)', 'dinheiro RESERVADO viagem', '-50.5', 1, 1, 0, 0, 0, 0, 1, '2026-08-20 12:00:00', '2026-08-20 12:00:00'),
    (1, 'r-04', '2026-09-01', 'Dinheiro retirado Viagem', 'Dinheiro retirado Viagem', '30.0', 2, 1, 0, 0, 0, 0, 0, '2026-09-01 12:00:00', '2026-09-01 12:00:00'),
    (1, 'r-05', '2026-09-10', 'Dinheiro reservado Casa', 'Dinheiro reservado Casa', '-200.0', 1, 1, 0, 0, 0, 0, 0, '2026-09-10 12:00:00', '2026-09-10 12:00:00'),
    (1, 'r-06', '2026-09-15', 'Dinheiro retirado Casa', 'Dinheiro retirado Casa', '250.0', 2, 1, 0, 0, 0, 0, 0, '2026-09-15 12:00:00', '2026-09-15 12:00:00'),
    (1, 'r-07', '2026-09-16', 'Dinheiro reservado ', 'Dinheiro reservado ', '-5.0', 1, 1, 0, 0, 0, 0, 0, '2026-09-16 12:00:00', '2026-09-16 12:00:00'),
    (1, 'r-08', '2026-09-17', 'Dinheiro reservado Casa', 'Dinheiro reservado Casa', '-999.0', 1, 2, 0, 0, 0, 0, 0, '2026-09-17 12:00:00', '2026-09-17 12:00:00'),
    (1, 'r-09', '2026-09-18', 'DINHEIRO RESERVADO Emergência', NULL, '-10.333', 1, 1, 0, 0, 0, 0, 0, '2026-09-18 12:00:00', '2026-09-18 12:00:00'),
    (1, 'r-10', '2026-09-20', 'Compra de Meli Dólar MUSD 10,50', 'Compra de Meli Dólar MUSD 10,50', '-57.0', 1, 1, 0, 0, 0, 0, 0, '2026-09-20 12:00:00', '2026-09-20 12:00:00'),
    (1, 'r-11', '2026-09-26', 'Compra de Meli Dolar', 'Compra de Meli Dolar', '-10.0', 1, 1, 0, 0, 0, 0, 0, '2026-09-26 12:00:00', '2026-09-26 12:00:00'),
    (1, 'r-12', '2026-10-01', 'Venda de Meli Dolar', 'Venda de Meli Dolar', '27.03', 2, 1, 0, 0, 0, 0, 0, '2026-10-01 12:00:00', '2026-10-01 12:00:00'),
    (1, 'r-13', '2026-10-02', 'Compra de Méli Dólar MUSD 1.000', 'Compra de Méli Dólar MUSD 1.000', '-5000.0', 1, 1, 0, 0, 0, 0, 0, '2026-10-02 12:00:00', '2026-10-02 12:00:00'),
    (1, 'r-14', '2026-10-03', 'Venda de meli dolar MUSD 2.5', 'Venda de meli dolar MUSD 2.5', '13.0', 2, 1, 0, 0, 0, 0, 0, '2026-10-03 12:00:00', '2026-10-03 12:00:00'),
    -- O mesmo saldo: o desempate é pelo nome, na ordem da cultura ("árvore" antes de "Zebra"; pelo código, ao contrário).
    (1, 'r-15', '2026-10-05', 'Dinheiro reservado Zebra', 'Dinheiro reservado Zebra', '-7.0', 1, 1, 0, 0, 0, 0, 0, '2026-10-05 12:00:00', '2026-10-05 12:00:00'),
    (1, 'r-16', '2026-10-06', 'Dinheiro reservado árvore', 'Dinheiro reservado árvore', '-7.0', 1, 1, 0, 0, 0, 0, 0, '2026-10-06 12:00:00', '2026-10-06 12:00:00'),
    -- Conta 2 (conexão desligada), 3 (arquivada), 6 (fora do saldo), 8 (manual): nada aparece.
    (2, 'r-20', '2026-09-01', 'Dinheiro reservado Viagem', 'Dinheiro reservado Viagem', '-70.0', 1, 1, 0, 0, 0, 0, 0, '2026-09-01 12:00:00', '2026-09-01 12:00:00'),
    (3, 'r-30', '2026-09-01', 'Dinheiro reservado Viagem', 'Dinheiro reservado Viagem', '-70.0', 1, 1, 0, 0, 0, 0, 0, '2026-09-01 12:00:00', '2026-09-01 12:00:00'),
    (6, 'r-60', '2026-09-01', 'Dinheiro reservado Viagem', 'Dinheiro reservado Viagem', '-70.0', 1, 1, 0, 0, 0, 0, 0, '2026-09-01 12:00:00', '2026-09-01 12:00:00'),
    (8, NULL, '2026-09-01', 'Dinheiro reservado Viagem', NULL, '-70.0', 1, 1, 0, 0, 0, 0, 0, '2026-09-01 12:00:00', '2026-09-01 12:00:00'),
    -- Conta 4: só o total do banco.
    (4, 'r-40', '2026-10-10', 'Dinheiro reservado Reserva A', 'Dinheiro reservado Reserva A', '-100.0', 1, 1, 0, 0, 0, 0, 0, '2026-10-10 12:00:00', '2026-10-10 12:00:00'),
    (4, 'r-41', '2026-10-11', 'Dinheiro reservado Reservado', 'Dinheiro reservado Reservado', '-50.0', 1, 1, 0, 0, 0, 0, 0, '2026-10-11 12:00:00', '2026-10-11 12:00:00'),
    -- Conta 5: a lista detalhada do banco.
    (5, 'r-50', '2026-10-10', 'Dinheiro reservado caixa', 'Dinheiro reservado caixa', '-100.0', 1, 1, 0, 0, 0, 0, 0, '2026-10-10 12:00:00', '2026-10-10 12:00:00'),
    (5, 'r-51', '2026-10-12', 'Dinheiro reservado Outra', 'Dinheiro reservado Outra', '-20.0', 1, 1, 0, 0, 0, 0, 0, '2026-10-12 12:00:00', '2026-10-12 12:00:00');

INSERT INTO ReservaConferencia (ContaId, Nome, SaldoConferido, DataConferencia, AtualizadoEm) VALUES
    (1, 'VIAGEM', '120.0', '2026-08-25', '2026-08-25 12:00:00'),
    (1, 'Méli Dólar', '10.12345678', '2026-09-25', '2026-09-25 12:00:00'),
    (2, 'Poupança antiga', '33.3', '2026-10-20', '2026-10-20 12:00:00'),
    (3, 'Arquivada', '10.0', '2026-10-20', '2026-10-20 12:00:00'),
    (7, 'meli dolar', '2.0', '2026-10-25', '2026-10-25 12:00:00');

-- O dólar: o histórico cobre o começo do Méli Dólar (20/09), e o GarantirHistorico não vai à internet.
INSERT INTO CotacaoMoeda (Moeda, Data, Compra, Venda, AtualizadoEm) VALUES
    ('USD', '2026-09-15', '5.30', '5.31', '2026-09-15 16:00:00'),
    ('USD', '2026-09-19', '5.3123', '5.3129', '2026-09-19 16:00:00'),
    ('USD', '2026-10-01', '5.4567', '5.4573', '2026-10-01 16:00:00'),
    ('USD', '2026-10-28', '5.5011', '5.5017', '2026-10-28 16:00:00');
