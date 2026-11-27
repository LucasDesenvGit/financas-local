package autenticacao

import (
	"context"
	"database/sql"
	"errors"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// Usuario é a linha da tabela Usuario, no formato do banco (GUID em maiúsculas, data e hora "AAAA-MM-DD hh:mm:ss.f").
type Usuario struct {
	Id               int64
	Login            string
	SenhaHash        string
	CarimboSeguranca tipos.Guid
	TentativasFalhas int
	BloqueadoAte     *tipos.DataHora
	UltimoLoginEm    *tipos.DataHora
	CriadoEm         tipos.DataHora
}

const colunasUsuario = `"Id", "Login", "SenhaHash", "CarimboSeguranca", "TentativasFalhas", "BloqueadoAte", "UltimoLoginEm", "CriadoEm"`

// lerUsuario lê a linha do usuário; sql.ErrNoRows se não há.
func lerUsuario(linha *sql.Row) (Usuario, error) {
	var u Usuario
	err := linha.Scan(&u.Id, &u.Login, &u.SenhaHash, &u.CarimboSeguranca, &u.TentativasFalhas, &u.BloqueadoAte, &u.UltimoLoginEm, &u.CriadoEm)
	return u, err
}

func existeUsuario(ctx context.Context, q banco.DBTX) (existe bool, err error) {
	err = q.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM "Usuario")`).Scan(&existe)
	return existe, err
}

func usuarioPorLogin(ctx context.Context, q banco.DBTX, login string) (Usuario, error) {
	return lerUsuario(q.QueryRowContext(ctx, `SELECT `+colunasUsuario+` FROM "Usuario" WHERE "Login" = ? LIMIT 1`, login))
}

func usuarioPorId(ctx context.Context, q banco.DBTX, id int64) (Usuario, error) {
	return lerUsuario(q.QueryRowContext(ctx, `SELECT `+colunasUsuario+` FROM "Usuario" WHERE "Id" = ? LIMIT 1`, id))
}

// sessaoValida diz se o usuário existe e o carimbo de segurança da sessão é o dele.
func sessaoValida(ctx context.Context, q banco.DBTX, id int64, carimbo tipos.Guid) (valida bool, err error) {
	err = q.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM "Usuario" WHERE "Id" = ? AND "CarimboSeguranca" = ?)`, id, carimbo).Scan(&valida)
	return valida, err
}

// inserirUsuario grava o usuário e devolve o Id novo.
func inserirUsuario(ctx context.Context, q banco.DBTX, u Usuario) (id int64, err error) {
	err = q.QueryRowContext(ctx, `INSERT INTO "Usuario" ("Login", "SenhaHash", "CarimboSeguranca", "TentativasFalhas", "BloqueadoAte", "UltimoLoginEm", "CriadoEm")
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING "Id"`, u.Login, u.SenhaHash, u.CarimboSeguranca, u.TentativasFalhas, u.BloqueadoAte, u.UltimoLoginEm, u.CriadoEm).Scan(&id)
	return id, err
}

func atualizarUsuario(ctx context.Context, q banco.DBTX, u Usuario) error {
	_, err := q.ExecContext(ctx, `UPDATE "Usuario"
SET "SenhaHash" = ?, "CarimboSeguranca" = ?, "TentativasFalhas" = ?, "BloqueadoAte" = ?, "UltimoLoginEm" = ?
WHERE "Id" = ?`, u.SenhaHash, u.CarimboSeguranca, u.TentativasFalhas, u.BloqueadoAte, u.UltimoLoginEm, u.Id)
	return err
}

// valorDaConfiguracao é o Valor gravado na chave: nil se a linha não existe ou está sem valor.
func valorDaConfiguracao(ctx context.Context, q banco.DBTX, chave string) (*string, error) {
	var valor *string
	err := q.QueryRowContext(ctx, `SELECT "Valor" FROM "Configuracao" WHERE "Chave" = ?`, chave).Scan(&valor)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return valor, err
}

// ultimosAcessos são os 100 registros mais novos da AuditoriaAcesso.
func ultimosAcessos(ctx context.Context, q banco.DBTX) ([]auditoriaDto, error) {
	linhas, err := q.QueryContext(ctx, `SELECT "Id", "DataHora", "Evento", "Login", "Ip", "UserAgent"
FROM "AuditoriaAcesso"
ORDER BY "Id" DESC
LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer linhas.Close()
	registros := make([]auditoriaDto, 0, 100) // lista vazia sai [], nunca null
	for linhas.Next() {
		var x auditoriaDto
		if err := linhas.Scan(&x.Id, &x.DataHora, &x.Evento, &x.Login, &x.Ip, &x.UserAgent); err != nil {
			return nil, err
		}
		registros = append(registros, x)
	}
	return registros, linhas.Err()
}
