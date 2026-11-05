-- Configuracao (parametros e configuracoes). Desde a #76 o Go grava os parametros (PUT /api/parametros, atras do .NET).
-- Valor com Protegido = 1 e um segredo cifrado pelo Data Protection do .NET: o Go nao decifra nem escreve em log.
-- SEM ACENTO neste arquivo e nomes entre aspas duplas: veja o README.md do pacote banco.

-- name: ObterConfiguracao :one
SELECT "Chave", "Valor", "Protegido", "AtualizadoEm"
FROM "Configuracao"
WHERE "Chave" = ?;

-- name: ListarConfiguracoes :many
SELECT "Chave", "Valor", "Protegido", "AtualizadoEm"
FROM "Configuracao"
ORDER BY "Chave";

-- name: DefinirConfiguracao :exec
-- O DefinirAsync do ConfiguracaoService: cria ou atualiza, sempre com Protegido e AtualizadoEm novos.
INSERT INTO "Configuracao" ("Chave", "Valor", "Protegido", "AtualizadoEm")
VALUES (?, ?, ?, ?)
ON CONFLICT ("Chave") DO UPDATE SET
    "Valor" = excluded."Valor",
    "Protegido" = excluded."Protegido",
    "AtualizadoEm" = excluded."AtualizadoEm";

-- name: ExisteConfiguracao :one
SELECT EXISTS (SELECT 1 FROM "Configuracao" WHERE "Chave" = ? AND "Valor" IS NOT NULL) AS "Existe";

-- name: ContarContas :one
SELECT COUNT(*) FROM "Conta";

-- name: ContarTransacoes :one
SELECT COUNT(*) FROM "Transacao";

-- name: ContarConexoesAtivas :one
SELECT COUNT(*) FROM "ConexaoBancaria" WHERE "Ativa" = 1;
