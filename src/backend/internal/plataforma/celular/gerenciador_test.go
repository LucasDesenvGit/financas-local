package celular

import (
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
)

const chaveDosTestes = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// Regra (#684): o site cria o segredo do gerenciador uma vez (64 caracteres hexadecimais) e depois só lê; arquivo
// estragado é erro, com o que fazer.
func TestChaveDoGerenciador(t *testing.T) {
	pasta := filepath.Join(t.TempDir(), "chaves", "nova")
	chave, err := CarregarChaveDoGerenciador(pasta)
	if err != nil {
		t.Fatal(err)
	}
	if len(chave) != 64 || strings.Trim(chave, "0123456789abcdef") != "" {
		t.Errorf("chave %q", chave)
	}
	deNovo, err := CarregarChaveDoGerenciador(pasta)
	if err != nil || deNovo != chave {
		t.Errorf("leu %q, %v; esperado %q", deNovo, err, chave)
	}
	outra, err := CarregarChaveDoGerenciador(filepath.Join(t.TempDir(), "outra"))
	if err != nil || outra == chave {
		t.Errorf("outra pasta: %q %v", outra, err)
	}
	// Com quebra de linha no fim (alguém abriu e salvou no editor): vale.
	caminho := filepath.Join(pasta, ArquivoChaveGerenciador)
	if err := os.WriteFile(caminho, []byte(chaveDosTestes+"\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if lida, err := CarregarChaveDoGerenciador(pasta); err != nil || lida != chaveDosTestes {
		t.Errorf("com quebra de linha: %q %v", lida, err)
	}
	for _, conteudo := range []string{chaveDosTestes[:62], chaveDosTestes + "00", "zz" + chaveDosTestes[2:], ""} {
		if err := os.WriteFile(caminho, []byte(conteudo), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := CarregarChaveDoGerenciador(pasta); err == nil || !strings.Contains(err.Error(), "apague o arquivo") {
			t.Errorf("%q: %v", conteudo, err)
		}
	}
	// A pasta das chaves é um arquivo: não dá para criar.
	arquivo := filepath.Join(t.TempDir(), "arquivo")
	if err := os.WriteFile(arquivo, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := CarregarChaveDoGerenciador(filepath.Join(arquivo, "chaves")); err == nil {
		t.Error("pasta impossível, e sem erro")
	}
}

// pedirDoGerenciador é o POST do gerenciador: do endereço e com a chave dados.
func (a *ambiente) pedirDoGerenciador(rota, remoto, chave string) *httptest.ResponseRecorder {
	a.t.Helper()
	r := httptest.NewRequestWithContext(a.t.Context(), "POST", rota, nil)
	r.RemoteAddr = remoto
	if chave != "" {
		r.Header.Set(CabecalhoChaveGerenciador, chave)
	}
	w := httptest.NewRecorder()
	a.rotas().ServeHTTP(w, r)
	return w
}

// Regra (#684): ligar e desligar pelo gerenciador só do próprio computador e com o segredo; o resto é 404 sem corpo,
// como se a rota não existisse, e nada muda.
func TestGerenciadorSemOSegredo(t *testing.T) {
	a := novoAmbiente(t)
	a.s.o.ChaveGerenciador = chaveDosTestes
	for _, rota := range []string{RotaGerenciadorLigar, RotaGerenciadorDesligar} {
		for _, c := range []struct{ remoto, chave string }{
			{"192.168.0.20:50000", chaveDosTestes}, // da rede, com o segredo
			{"127.0.0.1:50000", ""},                // sem o segredo
			{"127.0.0.1:50000", chaveDosTestes[:63] + "0"},
			{"[::1]:50000", strings.ToUpper(chaveDosTestes)},
			{"127.0.0.1:50000", chaveDosTestes + "0"},
		} {
			if w := a.pedirDoGerenciador(rota, c.remoto, c.chave); w.Code != 404 || w.Body.Len() != 0 {
				t.Errorf("%s de %s com %q: %d %s", rota, c.remoto, c.chave, w.Code, w.Body)
			}
		}
	}
	// Site sem o segredo carregado: ninguém passa, nem com o cabeçalho vazio.
	a.s.o.ChaveGerenciador = ""
	if w := a.pedirDoGerenciador(RotaGerenciadorLigar, "127.0.0.1:50000", ""); w.Code != 404 {
		t.Errorf("sem segredo no site: %d", w.Code)
	}
	if len(a.pedidos) != 0 || a.ipGravado() != nil || len(a.eventos()) != 0 || a.fw.fechados != 0 {
		t.Errorf("mudou algo: escutou %v, gravou %v, auditoria %v, fechou %d", a.pedidos, a.ipGravado(), a.eventos(), a.fw.fechados)
	}
}

// Regra (#684): com o segredo, do 127.0.0.1 ou do ::1, liga no IP preferido (o primeiro da lista) e desliga, com a
// auditoria; ligar de novo não troca nada.
func TestGerenciadorLigaEDesliga(t *testing.T) {
	a := novoAmbiente(t)
	a.s.o.ChaveGerenciador = chaveDosTestes
	w := a.pedirDoGerenciador(RotaGerenciadorLigar, "127.0.0.1:50000", chaveDosTestes)
	if w.Code != 200 {
		t.Fatalf("ligar: %d %s", w.Code, w.Body)
	}
	if !strings.Contains(w.Body.String(), `"ligado":true`) || !strings.Contains(w.Body.String(), `"endereco":"https://192.168.0.10:5443"`) {
		t.Errorf("ligar: %s", w.Body)
	}
	if ip := a.ipGravado(); ip == nil || *ip != "192.168.0.10" {
		t.Errorf("gravou %v", ip)
	}
	if w := a.pedirDoGerenciador(RotaGerenciadorLigar, "[::1]:50000", chaveDosTestes); w.Code != 200 ||
		!strings.Contains(w.Body.String(), `"ligado":true`) {
		t.Errorf("ligar de novo: %d %s", w.Code, w.Body)
	}
	if !reflect.DeepEqual(a.pedidos, []string{"192.168.0.10:5443"}) {
		t.Errorf("ligar de novo reabriu a porta: %v", a.pedidos)
	}
	w = a.pedirDoGerenciador(RotaGerenciadorDesligar, "[::1]:50000", chaveDosTestes)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"ligado":false`) {
		t.Errorf("desligar: %d %s", w.Code, w.Body)
	}
	if a.ipGravado() != nil || a.fw.fechados != 1 {
		t.Errorf("desligado: gravado %v, firewall fechado %d vezes", a.ipGravado(), a.fw.fechados)
	}
	if e := a.eventos(); !reflect.DeepEqual(e, []string{"AcessoCelularLigado 127.0.0.1", "AcessoCelularDesligado ::1"}) {
		t.Errorf("auditoria %v", e)
	}
}

// Regra (#684): sem escolher, o IP gravado vale enquanto for deste computador; se o roteador deu outro, o preferido.
func TestLigarSemEscolher(t *testing.T) {
	a := novoAmbiente(t)
	if _, err := a.s.Ligar(t.Context(), a.pedido(), "10.0.0.20"); err != nil {
		t.Fatal(err)
	}
	// O site parou (a escolha fica gravada) e subiu sem conseguir escutar: ligar volta no IP gravado.
	a.s.Parar(context.Background())
	sit, err := a.s.LigarSemEscolher(t.Context(), a.pedido())
	if err != nil || !sit.Ligado || *sit.IP != "10.0.0.20" {
		t.Fatalf("gravado: %+v %v", sit, err)
	}
	// O IP gravado saiu deste computador: liga no preferido.
	a.s.Parar(context.Background())
	a.mu.Lock()
	a.enderecos = []Endereco{wifi}
	a.mu.Unlock()
	sit, err = a.s.LigarSemEscolher(t.Context(), a.pedido())
	if err != nil || !sit.Ligado || *sit.IP != "192.168.0.10" {
		t.Fatalf("o gravado saiu: %+v %v", sit, err)
	}
	if ip := a.ipGravado(); ip == nil || *ip != "192.168.0.10" {
		t.Errorf("gravou %v", ip)
	}
}

// Regra (#684): sem rede de casa, 409 com o motivo; sem conseguir listar a rede, o 500 de sempre.
func TestGerenciadorSemRede(t *testing.T) {
	a := novoAmbiente(t)
	a.s.o.ChaveGerenciador = chaveDosTestes
	a.enderecos = []Endereco{}
	w := a.pedirDoGerenciador(RotaGerenciadorLigar, "127.0.0.1:1", chaveDosTestes)
	if w.Code != 409 {
		t.Fatalf("sem rede: %d %s", w.Code, w.Body)
	}
	testeapoio.ConferirJSON(t, w.Body.Bytes(), `{"mensagem":"`+MensagemSemRede+`"}`)

	a.errLista = errors.New("sem acesso às interfaces")
	if w := a.pedirDoGerenciador(RotaGerenciadorLigar, "127.0.0.1:1", chaveDosTestes); w.Code != 500 {
		t.Errorf("sem lista: %d %s", w.Code, w.Body)
	}
	if len(a.pedidos) != 0 || a.ipGravado() != nil {
		t.Errorf("mudou algo: %v %v", a.pedidos, a.ipGravado())
	}
}
