// Pacote auditoria é o AuditoriaService do .NET: grava em AuditoriaAcesso quem fez o quê, de onde e com que navegador.
package auditoria

import (
	"context"
	"net"
	"net/http"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Registrar é o AuditoriaService.Registrar: o evento, o login (até 100 caracteres), o IP e o navegador (até 400), na
// hora UTC. Recebe as consultas de quem chama para gravar na mesma transação (o SaveChanges do .NET).
//
// Atrás do .NET (ADR 0049) o IP é o da conexão do .NET com o Go, 127.0.0.1: o repasse não manda o do navegador. Como o
// site só aceita conexão do próprio computador, é o mesmo valor que o .NET grava.
func Registrar(ctx context.Context, q *consultas.Queries, r *http.Request, agora tipos.DataHora, evento string, login *string) error {
	if login != nil {
		cortado := respostas.CortarUtf16(*login, 100)
		login = &cortado
	}
	ip := IP(r)
	agente := respostas.CortarUtf16(r.UserAgent(), 400)
	return q.InserirAuditoria(ctx, consultas.InserirAuditoriaParams{
		DataHora: agora, Evento: evento, Login: login, Ip: &ip, UserAgent: &agente,
	})
}

// IP é o Connection.RemoteIpAddress do ASP.NET: o endereço de quem abriu a conexão, sem a porta.
func IP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
