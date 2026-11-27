/* =====================================================================
   FinancasLocal - Tabelas oficiais (INSS e IRRF por vigência)
   SQLite. Idempotente. Diferente do 02-dados-iniciais.sql (só em banco novo),
   o site aplica este a cada partida, em banco novo ou existente (ADR 0065):
   é assim que a vigência de um ano novo chega a quem já tem banco.
   ===================================================================== */

/* #210 (ADR 0065): as tabelas do INSS e do IRRF por vigência, com a fonte. Entram uma vez (WHERE NOT EXISTS): uma
   vigência que a pessoa corrigiu não volta ao valor daqui. Ano novo: uma vigência nova, com a fonte. */
INSERT INTO InssVigencia (Inicio, Fonte)
SELECT '2025-01-01', 'Portaria Interministerial MPS/MF nº 6, de 10/01/2025'
WHERE NOT EXISTS (SELECT 1 FROM InssVigencia WHERE Inicio = '2025-01-01');
INSERT INTO InssFaixa (Inicio, Ordem, Ate, Aliquota)
SELECT v.Inicio, v.Ordem, v.Ate, v.Aliquota
FROM (SELECT '2025-01-01' AS Inicio, 1 AS Ordem, '1518.00' AS Ate, '0.075' AS Aliquota
      UNION ALL SELECT '2025-01-01', 2, '2793.88', '0.09'
      UNION ALL SELECT '2025-01-01', 3, '4190.83', '0.12'
      UNION ALL SELECT '2025-01-01', 4, '8157.41', '0.14') v
WHERE NOT EXISTS (SELECT 1 FROM InssFaixa f WHERE f.Inicio = v.Inicio);

INSERT INTO InssVigencia (Inicio, Fonte)
SELECT '2026-01-01', 'Portaria Interministerial MPS/MF nº 13, de 09/01/2026'
WHERE NOT EXISTS (SELECT 1 FROM InssVigencia WHERE Inicio = '2026-01-01');
INSERT INTO InssFaixa (Inicio, Ordem, Ate, Aliquota)
SELECT v.Inicio, v.Ordem, v.Ate, v.Aliquota
FROM (SELECT '2026-01-01' AS Inicio, 1 AS Ordem, '1621.00' AS Ate, '0.075' AS Aliquota
      UNION ALL SELECT '2026-01-01', 2, '2902.84', '0.09'
      UNION ALL SELECT '2026-01-01', 3, '4354.27', '0.12'
      UNION ALL SELECT '2026-01-01', 4, '8475.55', '0.14') v
WHERE NOT EXISTS (SELECT 1 FROM InssFaixa f WHERE f.Inicio = v.Inicio);

INSERT INTO IrrfVigencia (Inicio, DeducaoDependente, DescontoSimplificado, Fonte)
SELECT '2025-05-01', '189.59', '607.20', 'Lei nº 15.191/2025 (tabela mensal desde maio de 2025)'
WHERE NOT EXISTS (SELECT 1 FROM IrrfVigencia WHERE Inicio = '2025-05-01');
INSERT INTO IrrfVigencia (Inicio, DeducaoDependente, DescontoSimplificado, ReducaoIsencaoAte, ReducaoMaxima, ReducaoAte,
                          ReducaoConstante, ReducaoFator, Fonte)
SELECT '2026-01-01', '189.59', '607.20', '5000.00', '312.89', '7350.00', '978.62', '0.133145',
       'Lei nº 15.191/2025 (tabela mensal) e Lei nº 15.270/2025 (redução até R$ 7.350,00)'
WHERE NOT EXISTS (SELECT 1 FROM IrrfVigencia WHERE Inicio = '2026-01-01');
INSERT INTO IrrfFaixa (Inicio, Ordem, Ate, Aliquota, Deducao)
SELECT v.Inicio, f.Ordem, f.Ate, f.Aliquota, f.Deducao
FROM (SELECT '2025-05-01' AS Inicio UNION ALL SELECT '2026-01-01') v
JOIN (SELECT 1 AS Ordem, '2428.80' AS Ate, '0' AS Aliquota, '0' AS Deducao
      UNION ALL SELECT 2, '2826.65', '0.075', '182.16'
      UNION ALL SELECT 3, '3751.05', '0.15', '394.16'
      UNION ALL SELECT 4, '4664.68', '0.225', '675.49'
      UNION ALL SELECT 5, NULL, '0.275', '908.73') f
WHERE NOT EXISTS (SELECT 1 FROM IrrfFaixa x WHERE x.Inicio = v.Inicio);
