-- Leitura da Configuracao (parametros e configuracoes). So leitura: quem grava e o .NET ate a #76 (ADR 0049).
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
