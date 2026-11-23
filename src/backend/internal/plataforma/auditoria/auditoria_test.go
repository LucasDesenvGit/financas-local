package auditoria

import (
	"context"
	"database/sql"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/sessao"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// usuarioDeTeste grava um usuário no banco e devolve o Id dele.
func usuarioDeTeste(t *testing.T, db *sql.DB, login string) int64 {
	t.Helper()
	r, err := db.Exec(`INSERT INTO "Usuario" ("Login", "SenhaHash", "CarimboSeguranca", "TentativasFalhas", "CriadoEm")
		VALUES (?, 'hash', ?, 0, ?)`, login, tipos.NovoGuid(), tipos.NovaDataHora(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	id, err := r.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// Regra: Login é o User.Identity.Name: o login de quem está na sessão; nil sem sessão ou com usuário que não existe
// mais; erro do banco volta como erro.
func TestLogin(t *testing.T) {
	db := testeapoio.NovoBanco(t)
	id := usuarioDeTeste(t, db, "ana.teste")
	if login, err := Login(context.Background(), db); login != nil || err != nil {
		t.Errorf("sem sessão: %v %v", login, err)
	}
	if login, err := Login(sessao.ComUsuario(context.Background(), id), db); err != nil || login == nil || *login != "ana.teste" {
		t.Errorf("com sessão: %v %v", login, err)
	}
	if login, err := Login(sessao.ComUsuario(context.Background(), id+1), db); login != nil || err != nil {
		t.Errorf("usuário que não existe mais: %v %v", login, err)
	}
	fechado := testeapoio.NovoBanco(t)
	fechado.Close()
	if _, err := Login(sessao.ComUsuario(context.Background(), 1), fechado); err == nil {
		t.Error("o erro do banco sumiu")
	}
}

// Regra: Registrar grava o evento com o login cortado em 100 caracteres, o IP sem a porta e o navegador cortado em
// 400; RegistrarSemPedido grava só a hora e o evento (login, IP e navegador ficam NULL).
func TestRegistrar(t *testing.T) {
	ctx := context.Background()
	db := testeapoio.NovoBanco(t)
	agora := tipos.NovaDataHora(time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC))
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:5443"
	r.Header.Set("User-Agent", strings.Repeat("n", 500))
	login := strings.Repeat("l", 150)
	if err := Registrar(ctx, db, r, agora, "Entrou", &login); err != nil {
		t.Fatal(err)
	}
	if err := RegistrarSemPedido(ctx, db, agora, "Rotina"); err != nil {
		t.Fatal(err)
	}
	linhas, err := db.Query(`SELECT "Evento", length("Login"), "Ip", length("UserAgent") FROM "AuditoriaAcesso" ORDER BY "Id"`)
	if err != nil {
		t.Fatal(err)
	}
	defer linhas.Close()
	var obtido []string
	for linhas.Next() {
		var evento string
		var tamLogin, tamAgente sql.NullInt64
		var ip sql.NullString
		if err := linhas.Scan(&evento, &tamLogin, &ip, &tamAgente); err != nil {
			t.Fatal(err)
		}
		obtido = append(obtido, fmt.Sprintf("%s %d %s %d", evento, tamLogin.Int64, ip.String, tamAgente.Int64))
	}
	if esperado := "Entrou 100 127.0.0.1 400|Rotina 0  0"; strings.Join(obtido, "|") != esperado {
		t.Errorf("gravou %q, esperado %q", strings.Join(obtido, "|"), esperado)
	}
}

// Regra: IP é o endereço sem a porta; sem porta, fica como veio.
func TestIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	for remoto, esperado := range map[string]string{"127.0.0.1:5443": "127.0.0.1", "[::1]:80": "::1", "sem-porta": "sem-porta"} {
		r.RemoteAddr = remoto
		if ip := IP(r); ip != esperado {
			t.Errorf("IP(%q) = %q, esperado %q", remoto, ip, esperado)
		}
	}
}
