-- Usuario e AuditoriaAcesso. Ate a inversao (#85) quem grava no site e o .NET; o Go grava quando roda sozinho, com o
-- login dele (#75). As colunas e o formato sao os do EF Core (GUID em maiusculas, data e hora "AAAA-MM-DD hh:mm:ss.f").
-- SEM ACENTO neste arquivo e nomes entre aspas duplas: veja o README.md do pacote banco.

-- name: ObterLoginDoUsuario :one
SELECT "Login"
FROM "Usuario"
WHERE "Id" = ?;

-- name: ExisteUsuario :one
SELECT EXISTS (SELECT 1 FROM "Usuario") AS "Existe";

-- name: ObterUsuarioPorLogin :one
SELECT "Id", "Login", "SenhaHash", "CarimboSeguranca", "TentativasFalhas", "BloqueadoAte", "UltimoLoginEm", "CriadoEm"
FROM "Usuario"
WHERE "Login" = ?
LIMIT 1;

-- name: ObterUsuarioPorId :one
SELECT "Id", "Login", "SenhaHash", "CarimboSeguranca", "TentativasFalhas", "BloqueadoAte", "UltimoLoginEm", "CriadoEm"
FROM "Usuario"
WHERE "Id" = ?
LIMIT 1;

-- name: SessaoValida :one
SELECT EXISTS (SELECT 1 FROM "Usuario" WHERE "Id" = ? AND "CarimboSeguranca" = ?) AS "Valida";

-- name: InserirUsuario :one
INSERT INTO "Usuario" ("Login", "SenhaHash", "CarimboSeguranca", "TentativasFalhas", "BloqueadoAte", "UltimoLoginEm", "CriadoEm")
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING "Id";

-- name: AtualizarUsuario :exec
UPDATE "Usuario"
SET "SenhaHash" = ?, "CarimboSeguranca" = ?, "TentativasFalhas" = ?, "BloqueadoAte" = ?, "UltimoLoginEm" = ?
WHERE "Id" = ?;

-- name: InserirAuditoria :exec
INSERT INTO "AuditoriaAcesso" ("DataHora", "Evento", "Login", "Ip", "UserAgent")
VALUES (?, ?, ?, ?, ?);

-- name: ListarAuditoria :many
SELECT "Id", "DataHora", "Evento", "Login", "Ip", "UserAgent"
FROM "AuditoriaAcesso"
ORDER BY "Id" DESC
LIMIT 100;
