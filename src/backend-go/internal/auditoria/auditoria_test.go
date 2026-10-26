package auditoria

import (
	"context"
	"database/sql"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/sessao"
)

// loginsFalsos é o ObterLoginDoUsuario sem o banco.
type loginsFalsos map[int64]string

func (l loginsFalsos) ObterLoginDoUsuario(_ context.Context, id int64) (string, error) {
	if id == 99 {
		return "", errors.New("banco fora")
	}
	login, ok := l[id]
	if !ok {
		return "", sql.ErrNoRows
	}
	return login, nil
}

// Regra: Login é o User.Identity.Name: o login de quem está na sessão; nil sem sessão ou com usuário que não existe
// mais; erro do banco volta como erro.
func TestLogin(t *testing.T) {
	q := loginsFalsos{1: "ana.teste"}
	if login, err := Login(context.Background(), q); login != nil || err != nil {
		t.Errorf("sem sessão: %v %v", login, err)
	}
	if login, err := Login(sessao.ComUsuario(context.Background(), 1), q); err != nil || login == nil || *login != "ana.teste" {
		t.Errorf("com sessão: %v %v", login, err)
	}
	if login, err := Login(sessao.ComUsuario(context.Background(), 2), q); login != nil || err != nil {
		t.Errorf("usuário que não existe mais: %v %v", login, err)
	}
	if _, err := Login(sessao.ComUsuario(context.Background(), 99), q); err == nil {
		t.Error("o erro do banco sumiu")
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
