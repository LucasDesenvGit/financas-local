-- Leitura do Usuario. So leitura: quem grava e o .NET (login, senha, sessao) ate a #75 (ADR 0049).
-- SEM ACENTO neste arquivo e nomes entre aspas duplas: veja o README.md do pacote banco.

-- name: ObterLoginDoUsuario :one
SELECT "Login"
FROM "Usuario"
WHERE "Id" = ?;
