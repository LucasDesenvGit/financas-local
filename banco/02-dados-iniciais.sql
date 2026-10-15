/* =====================================================================
   FinancasLocal - Dados iniciais (categorias padrão e configurações)
   Idempotente: só insere o que ainda não existe.
   ===================================================================== */
USE [FinancasLocal];
GO

DECLARE @Categorias TABLE (Nome NVARCHAR(80), Tipo TINYINT, Cor NVARCHAR(9), Icone NVARCHAR(40));
INSERT INTO @Categorias (Nome, Tipo, Cor, Icone) VALUES
    (N'Alimentação',            1, N'#f8c09a', N'restaurante'),
    (N'Mercado',                1, N'#f2d98b', N'carrinho'),
    (N'Moradia',                1, N'#d4a5e8', N'casa'),
    (N'Transporte',             1, N'#92c8f0', N'carro'),
    (N'Saúde',                  1, N'#f4a89a', N'saude'),
    (N'Educação',               1, N'#8fd8d8', N'livro'),
    (N'Lazer',                  1, N'#f3b3d3', N'lazer'),
    (N'Compras',                1, N'#c9b3f5', N'sacola'),
    (N'Serviços e assinaturas', 1, N'#a7a3f5', N'assinatura'),
    (N'Viagens',                1, N'#9fd2f6', N'aviao'),
    (N'Impostos e taxas',       1, N'#c4bcb4', N'imposto'),
    (N'Tarifas bancárias',      1, N'#b8b0a8', N'banco'),
    (N'Seguros',                1, N'#a9b4c6', N'escudo'),
    (N'Doações',                1, N'#f5a3b0', N'coracao'),
    (N'Pets',                   1, N'#e3c48f', N'pet'),
    (N'Cuidados pessoais',      1, N'#e7b3ea', N'pessoal'),
    (N'Juros e empréstimos',    1, N'#f19c9c', N'juros'),
    (N'Outros gastos',          1, N'#b4bccb', N'outros'),
    (N'Salário',                2, N'#8fd5b4', N'salario'),
    (N'Rendimentos',            2, N'#a8dcb9', N'grafico'),
    (N'Outras receitas',        2, N'#b5e3c4', N'entrada'),
    (N'Transferências',         3, N'#9ccfc6', N'transferencia'),
    (N'Pagamento de fatura',    3, N'#a9c9d6', N'cartao'),
    (N'Investimentos',          3, N'#8fd8c8', N'investimento');

INSERT INTO dbo.Categoria (Nome, Tipo, Cor, Icone)
SELECT c.Nome, c.Tipo, c.Cor, c.Icone
FROM @Categorias c
WHERE NOT EXISTS (SELECT 1 FROM dbo.Categoria x WHERE x.Nome = c.Nome);
GO

DECLARE @Config TABLE (Chave NVARCHAR(100), Valor NVARCHAR(MAX));
INSERT INTO @Config (Chave, Valor) VALUES
    (N'Sincronizacao.IntervaloHoras',     N'6'),
    (N'Sincronizacao.DiasRetroativos',    N'30'),
    (N'Sincronizacao.DiasPrimeiraCarga',  N'365'),
    (N'Sincronizacao.Habilitada',         N'true');

INSERT INTO dbo.Configuracao (Chave, Valor, Protegido)
SELECT c.Chave, c.Valor, 0
FROM @Config c
WHERE NOT EXISTS (SELECT 1 FROM dbo.Configuracao x WHERE x.Chave = c.Chave);
GO

/* Paleta pastel (v2): atualiza as cores padrão antigas, sem mexer nas que você personalizou. */
DECLARE @Cores TABLE (Nome NVARCHAR(80), CorAntiga NVARCHAR(9), CorNova NVARCHAR(9));
INSERT INTO @Cores (Nome, CorAntiga, CorNova) VALUES
    (N'Alimentação', N'#ea580c', N'#f8c09a'),
    (N'Mercado', N'#d97706', N'#f2d98b'),
    (N'Moradia', N'#7c3aed', N'#d4a5e8'),
    (N'Transporte', N'#2563eb', N'#92c8f0'),
    (N'Saúde', N'#dc2626', N'#f4a89a'),
    (N'Educação', N'#0891b2', N'#8fd8d8'),
    (N'Lazer', N'#db2777', N'#f3b3d3'),
    (N'Compras', N'#9333ea', N'#c9b3f5'),
    (N'Serviços e assinaturas', N'#4f46e5', N'#a7a3f5'),
    (N'Viagens', N'#0284c7', N'#9fd2f6'),
    (N'Impostos e taxas', N'#57534e', N'#c4bcb4'),
    (N'Tarifas bancárias', N'#78716c', N'#b8b0a8'),
    (N'Seguros', N'#475569', N'#a9b4c6'),
    (N'Doações', N'#e11d48', N'#f5a3b0'),
    (N'Pets', N'#a16207', N'#e3c48f'),
    (N'Cuidados pessoais', N'#c026d3', N'#e7b3ea'),
    (N'Juros e empréstimos', N'#b91c1c', N'#f19c9c'),
    (N'Outros gastos', N'#64748b', N'#b4bccb'),
    (N'Salário', N'#16a34a', N'#8fd5b4'),
    (N'Rendimentos', N'#15803d', N'#a8dcb9'),
    (N'Outras receitas', N'#22c55e', N'#b5e3c4'),
    (N'Transferências', N'#0f766e', N'#9ccfc6'),
    (N'Pagamento de fatura', N'#115e59', N'#a9c9d6'),
    (N'Investimentos', N'#0d9488', N'#8fd8c8');

UPDATE c SET c.Cor = x.CorNova
FROM dbo.Categoria c
JOIN @Cores x ON x.Nome = c.Nome AND x.CorAntiga = c.Cor;
GO

/* Natureza padrão das categorias de despesa (v4) — só preenche onde ainda não foi definida. */
UPDATE dbo.Categoria SET Natureza = 1
WHERE Natureza IS NULL AND Tipo = 1 AND Nome IN (N'Mercado', N'Moradia', N'Transporte', N'Saúde', N'Educação',
    N'Impostos e taxas', N'Tarifas bancárias', N'Seguros', N'Juros e empréstimos');
UPDATE dbo.Categoria SET Natureza = 2
WHERE Natureza IS NULL AND Tipo = 1 AND Nome IN (N'Alimentação', N'Lazer', N'Compras', N'Serviços e assinaturas',
    N'Viagens', N'Doações', N'Pets', N'Cuidados pessoais', N'Outros gastos');
GO

/* Regras (v5): transferências para corretoras viram "Investimentos" (não contam como despesa).
   Rode "Reaplicar regras" em Categorias e regras para corrigir transações antigas. */
DECLARE @IdInvest INT = (SELECT Id FROM dbo.Categoria WHERE Nome = N'Investimentos');
IF @IdInvest IS NOT NULL
BEGIN
    DECLARE @Corretoras TABLE (Texto NVARCHAR(200));
    INSERT INTO @Corretoras (Texto) VALUES
        (N'XP INVEST'), (N'CLEAR CORRETORA'), (N'RICO INVEST'), (N'NU INVEST'), (N'NUINVEST'),
        (N'INTER DTVM'), (N'BTG PACTUAL CTVM'), (N'GENIAL INVEST'), (N'TORO INVEST'), (N'ORAMA'),
        (N'MODAL DTVM'), (N'AGORA INVEST'), (N'AVENUE SECURITIES'), (N'WARREN'), (N'B3 S.A');
    INSERT INTO dbo.RegraCategorizacao (Texto, TipoCorrespondencia, CategoriaId, Prioridade, Ativa)
    SELECT c.Texto, 1, @IdInvest, 50, 1
    FROM @Corretoras c
    WHERE NOT EXISTS (SELECT 1 FROM dbo.RegraCategorizacao r WHERE r.Texto = c.Texto);
END
GO
