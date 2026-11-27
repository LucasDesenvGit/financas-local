// Pacote auditoria: grava em AuditoriaAcesso quem fez o quê, de onde e com que navegador.
package auditoria

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/http"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/sessao"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// Registrar é o AuditoriaService.Registrar: o evento, o login (até 100 caracteres), o IP e o navegador (até 400), na
// hora UTC. Recebe o DBTX de quem chama (a transação dele) para gravar junto.
//
// O IP é o da conexão, sem a porta.
func Registrar(ctx context.Context, q banco.DBTX, r *http.Request, agora tipos.DataHora, evento string, login *string) error {
	if login != nil {
		cortado := respostas.CortarUtf16(*login, 100)
		login = &cortado
	}
	ip := IP(r)
	agente := respostas.CortarUtf16(r.UserAgent(), 400)
	return inserir(ctx, q, agora, evento, login, &ip, &agente)
}

// RegistrarSemPedido é o Registrar fora de uma requisição (a sincronização, as rotinas de partida): sem login, IP nem
// navegador.
func RegistrarSemPedido(ctx context.Context, q banco.DBTX, agora tipos.DataHora, evento string) error {
	return inserir(ctx, q, agora, evento, nil, nil, nil)
}

func inserir(ctx context.Context, q banco.DBTX, agora tipos.DataHora, evento string, login, ip, agente *string) error {
	_, err := q.ExecContext(ctx, `INSERT INTO "AuditoriaAcesso" ("DataHora", "Evento", "Login", "Ip", "UserAgent")
VALUES (?, ?, ?, ?, ?)`, agora, evento, login, ip, agente)
	return err
}

// IP: o endereço de quem abriu a conexão, sem a porta.
func IP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// Login: o login de quem está na sessão, ou nil (sem sessão, ou usuário que não existe
// mais).
func Login(ctx context.Context, q banco.DBTX) (*string, error) {
	id, ok := sessao.Usuario(ctx)
	if !ok {
		return nil, nil
	}
	var login string
	err := q.QueryRowContext(ctx, `SELECT "Login" FROM "Usuario" WHERE "Id" = ?`, id).Scan(&login)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &login, nil
}
