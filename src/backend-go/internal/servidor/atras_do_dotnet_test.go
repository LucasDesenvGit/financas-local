package servidor

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/sessao"
)

const segredoDeTeste = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// rotasQueContam respondem 200 com o usuário do contexto e anotam os cabeçalhos internos que chegaram.
func rotasQueContam(t *testing.T, vistos *[]string) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for nome := range r.Header {
			if strings.HasPrefix(nome, "X-Financas-") {
				*vistos = append(*vistos, nome)
			}
		}
		id, ok := sessao.Usuario(r.Context())
		if !ok {
			t.Error("a rota recebeu requisição sem usuário no contexto")
		}
		_, _ = io.WriteString(w, strconv.FormatInt(id, 10))
	})
}

func TestSegredoEUsuarioObrigatorios(t *testing.T) {
	var vistos []string
	h, err := NovoHandlerAtras(segredoDeTeste, rotasQueContam(t, &vistos), logDescartado)
	if err != nil {
		t.Fatal(err)
	}
	casos := []struct {
		nome, caminho, segredo, usuario string
		status                          int
	}{
		{"sem segredo", "/api/go/saude", "", "1", http.StatusUnauthorized},
		{"segredo errado do mesmo tamanho", "/api/go/saude", strings.Repeat("x", len(segredoDeTeste)), "1", http.StatusUnauthorized},
		{"segredo pela metade", "/api/go/saude", segredoDeTeste[:32], "1", http.StatusUnauthorized},
		{"segredo com um caractere a mais", "/api/go/saude", segredoDeTeste + "0", "1", http.StatusUnauthorized},
		{"sem usuário", "/api/go/saude", segredoDeTeste, "", http.StatusUnauthorized},
		{"usuário zero", "/api/go/saude", segredoDeTeste, "0", http.StatusUnauthorized},
		{"usuário negativo", "/api/go/saude", segredoDeTeste, "-3", http.StatusUnauthorized},
		{"usuário que não é número", "/api/go/saude", segredoDeTeste, "admin", http.StatusUnauthorized},
		{"tudo certo", "/api/go/saude", segredoDeTeste, "7", http.StatusOK},
		{"fora de /api, mesmo com segredo", "/index.html", segredoDeTeste, "7", http.StatusNotFound},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, c.caminho, nil)
			if c.segredo != "" {
				r.Header.Set(CabecalhoSegredo, c.segredo)
			}
			if c.usuario != "" {
				r.Header.Set(CabecalhoUsuario, c.usuario)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != c.status {
				t.Fatalf("status %d, esperado %d", w.Code, c.status)
			}
			if c.status == http.StatusUnauthorized && w.Body.Len() != 0 {
				t.Errorf("401 com corpo %q: o .NET responde 401 sem corpo", w.Body.String())
			}
			if c.status == http.StatusOK && w.Body.String() != c.usuario {
				t.Errorf("a rota viu o usuário %q, esperado %q", w.Body.String(), c.usuario)
			}
		})
	}
	if len(vistos) != 0 {
		t.Errorf("os cabeçalhos internos chegaram à rota (e poderiam ir para um log): %v", vistos)
	}
}

func TestSegredoCurtoRecusado(t *testing.T) {
	if _, err := NovoHandlerAtras("curto", http.NotFoundHandler(), logDescartado); err == nil {
		t.Fatal("aceitou um segredo curto")
	}
}

// O Go atrás do .NET escuta só em 127.0.0.1, numa porta escolhida pelo sistema, avisa a porta e para quando o
// contexto acaba.
func TestRodarAtrasDoDotnet(t *testing.T) {
	var vistos []string
	ctx, cancelar := context.WithCancel(context.Background())
	portas := make(chan int, 1)
	terminou := make(chan error, 1)
	go func() {
		terminou <- RodarAtrasDoDotnet(ctx, OpcoesAtras{
			Segredo:     segredoDeTeste,
			Rotas:       rotasQueContam(t, &vistos),
			Log:         logDescartado,
			AvisarPorta: func(porta int) error { portas <- porta; return nil },
		})
	}()

	var porta int
	select {
	case porta = <-portas:
	case err := <-terminou:
		t.Fatalf("parou antes de avisar a porta: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("não avisou a porta em 10 s")
	}
	if porta <= 0 {
		t.Fatalf("porta %d", porta)
	}

	// Escuta em 127.0.0.1 e não em todas as interfaces: o endereço "0.0.0.0" teria aceitado a conexão em outro IP.
	endereco := net.JoinHostPort("127.0.0.1", strconv.Itoa(porta))
	pedido, _ := http.NewRequest(http.MethodGet, "http://"+endereco+"/api/qualquer", nil)
	pedido.Header.Set(CabecalhoSegredo, segredoDeTeste)
	pedido.Header.Set(CabecalhoUsuario, "42")
	resposta, err := http.DefaultClient.Do(pedido)
	if err != nil {
		t.Fatal(err)
	}
	corpo, _ := io.ReadAll(resposta.Body)
	resposta.Body.Close()
	if resposta.StatusCode != http.StatusOK || string(corpo) != "42" {
		t.Fatalf("status %d, corpo %q", resposta.StatusCode, corpo)
	}
	if ips, _ := net.InterfaceAddrs(); len(ips) > 0 {
		for _, ip := range ips {
			if rede, ok := ip.(*net.IPNet); ok && !rede.IP.IsLoopback() && rede.IP.To4() != nil {
				conexao, err := net.DialTimeout("tcp", net.JoinHostPort(rede.IP.String(), strconv.Itoa(porta)), time.Second)
				if err == nil {
					conexao.Close()
					t.Errorf("o Go aceitou conexão em %s: devia escutar só em 127.0.0.1", rede.IP)
				}
			}
		}
	}

	cancelar()
	select {
	case err := <-terminou:
		if err != nil {
			t.Fatalf("parou com erro: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("não parou em 15 s depois do contexto acabar")
	}
}
