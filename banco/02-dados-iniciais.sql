/* =====================================================================
   FinancasLocal - Dados iniciais (categorias padrão e configurações)
   SQLite. Idempotente: só insere/atualiza o que ainda não existe.
   ===================================================================== */

/* Categorias padrão. INSERT OR IGNORE usa a UNIQUE(Nome) da tabela Categoria
   para não duplicar em execuções seguintes. */
INSERT OR IGNORE INTO Categoria (Nome, Tipo, Cor, Icone) VALUES
    ('Alimentação',            1, '#f8c09a', 'restaurante'),
    ('Mercado',                1, '#f2d98b', 'carrinho'),
    ('Moradia',                1, '#d4a5e8', 'casa'),
    ('Transporte',             1, '#92c8f0', 'carro'),
    ('Saúde',                  1, '#f4a89a', 'saude'),
    ('Educação',               1, '#8fd8d8', 'livro'),
    ('Lazer',                  1, '#f3b3d3', 'lazer'),
    ('Compras',                1, '#c9b3f5', 'sacola'),
    ('Serviços e assinaturas', 1, '#a7a3f5', 'assinatura'),
    ('Viagens',                1, '#9fd2f6', 'aviao'),
    ('Impostos e taxas',       1, '#c4bcb4', 'imposto'),
    ('Tarifas bancárias',      1, '#b8b0a8', 'banco'),
    ('Seguros',                1, '#a9b4c6', 'escudo'),
    ('Doações',                1, '#f5a3b0', 'coracao'),
    ('Pets',                   1, '#e3c48f', 'pet'),
    ('Cuidados pessoais',      1, '#e7b3ea', 'pessoal'),
    ('Juros e empréstimos',    1, '#f19c9c', 'juros'),
    ('Outros gastos',          1, '#b4bccb', 'outros'),
    ('Salário',                2, '#8fd5b4', 'salario'),
    ('Rendimentos',            2, '#a8dcb9', 'grafico'),
    ('Outras receitas',        2, '#b5e3c4', 'entrada'),
    ('Transferências',         3, '#9ccfc6', 'transferencia'),
    ('Pagamento de fatura',    3, '#a9c9d6', 'cartao'),
    ('Investimentos',          3, '#8fd8c8', 'investimento');

/* Configurações padrão. INSERT OR IGNORE usa a PRIMARY KEY(Chave).
   AtualizadoEm não tem mais DEFAULT no banco (a API já o preenche sozinha
   via EF Core), então este script precisa de um valor literal.
   Datas em UTC no mesmo formato texto que o EF Core grava ("yyyy-MM-dd HH:mm:ss.fff",
   sem "T" nem "Z"): formatos misturados quebram a ordenação e o "Z" faz a leitura
   converter para a hora local. */
INSERT OR IGNORE INTO Configuracao (Chave, Valor, Protegido, AtualizadoEm) VALUES
    ('Sincronizacao.IntervaloHoras',    '6',    0, strftime('%Y-%m-%d %H:%M:%f', 'now')),
    ('Sincronizacao.DiasRetroativos',   '30',   0, strftime('%Y-%m-%d %H:%M:%f', 'now')),
    ('Sincronizacao.DiasPrimeiraCarga', '365',  0, strftime('%Y-%m-%d %H:%M:%f', 'now')),
    ('Sincronizacao.Habilitada',        'true', 0, strftime('%Y-%m-%d %H:%M:%f', 'now'));

/* Paleta pastel (v2): atualiza as cores padrão antigas, sem mexer nas que você personalizou.
   Só troca a cor de uma categoria se ela ainda estiver com a cor "antiga" exata. */
WITH Cores (Nome, CorAntiga, CorNova) AS (
    VALUES
        ('Alimentação', '#ea580c', '#f8c09a'),
        ('Mercado', '#d97706', '#f2d98b'),
        ('Moradia', '#7c3aed', '#d4a5e8'),
        ('Transporte', '#2563eb', '#92c8f0'),
        ('Saúde', '#dc2626', '#f4a89a'),
        ('Educação', '#0891b2', '#8fd8d8'),
        ('Lazer', '#db2777', '#f3b3d3'),
        ('Compras', '#9333ea', '#c9b3f5'),
        ('Serviços e assinaturas', '#4f46e5', '#a7a3f5'),
        ('Viagens', '#0284c7', '#9fd2f6'),
        ('Impostos e taxas', '#57534e', '#c4bcb4'),
        ('Tarifas bancárias', '#78716c', '#b8b0a8'),
        ('Seguros', '#475569', '#a9b4c6'),
        ('Doações', '#e11d48', '#f5a3b0'),
        ('Pets', '#a16207', '#e3c48f'),
        ('Cuidados pessoais', '#c026d3', '#e7b3ea'),
        ('Juros e empréstimos', '#b91c1c', '#f19c9c'),
        ('Outros gastos', '#64748b', '#b4bccb'),
        ('Salário', '#16a34a', '#8fd5b4'),
        ('Rendimentos', '#15803d', '#a8dcb9'),
        ('Outras receitas', '#22c55e', '#b5e3c4'),
        ('Transferências', '#0f766e', '#9ccfc6'),
        ('Pagamento de fatura', '#115e59', '#a9c9d6'),
        ('Investimentos', '#0d9488', '#8fd8c8')
)
UPDATE Categoria
SET Cor = (
    SELECT Cores.CorNova FROM Cores
    WHERE Cores.Nome = Categoria.Nome AND Cores.CorAntiga = Categoria.Cor
)
WHERE EXISTS (
    SELECT 1 FROM Cores
    WHERE Cores.Nome = Categoria.Nome AND Cores.CorAntiga = Categoria.Cor
);

/* Natureza padrão das categorias de despesa — só preenche onde ainda não foi definida. */
UPDATE Categoria SET Natureza = 1
WHERE Natureza IS NULL AND Tipo = 1 AND Nome IN ('Mercado', 'Moradia', 'Transporte', 'Saúde', 'Educação',
    'Impostos e taxas', 'Tarifas bancárias', 'Seguros', 'Juros e empréstimos');
UPDATE Categoria SET Natureza = 2
WHERE Natureza IS NULL AND Tipo = 1 AND Nome IN ('Alimentação', 'Lazer', 'Compras', 'Serviços e assinaturas',
    'Viagens', 'Doações', 'Pets', 'Cuidados pessoais', 'Outros gastos');

/* Regras: transferências para corretoras viram "Investimentos" (não contam como despesa).
   Rode "Reaplicar regras" em Categorias e regras para corrigir transações antigas.
   RegraCategorizacao não tem chave natural única na estrutura, então a idempotência aqui
   é feita com WHERE NOT EXISTS (INSERT OR IGNORE não serviria sem uma UNIQUE/PK em Texto). */
WITH Corretoras (Texto) AS (
    VALUES
        ('XP INVEST'), ('CLEAR CORRETORA'), ('RICO INVEST'), ('NU INVEST'), ('NUINVEST'),
        ('INTER DTVM'), ('BTG PACTUAL CTVM'), ('GENIAL INVEST'), ('TORO INVEST'), ('ORAMA'),
        ('MODAL DTVM'), ('AGORA INVEST'), ('AVENUE SECURITIES'), ('WARREN'), ('B3 S.A')
)
INSERT INTO RegraCategorizacao (Texto, TipoCorrespondencia, CategoriaId, Prioridade, Ativa, CriadoEm)
SELECT Corretoras.Texto, 1, Categoria.Id, 50, 1, strftime('%Y-%m-%d %H:%M:%f', 'now')
FROM Corretoras
JOIN Categoria ON Categoria.Nome = 'Investimentos'
WHERE NOT EXISTS (SELECT 1 FROM RegraCategorizacao r WHERE r.Texto = Corretoras.Texto);
