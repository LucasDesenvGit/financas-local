package servidor

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// #86: os testes das bordas que o teste de mutação mostrou sem cobertura. Até a #86, o teste que lia o Seguranca.cs
// por caminho relativo falhava dentro da cópia do módulo que o gremlins usa, e todo mutante do pacote contava como
// morto: estas lacunas ficavam escondidas.

func TestEhAPI(t *testing.T) {
	casos := []struct {
		caminho string
		ehAPI   bool
	}{
		{"/api", true},
		{"/API", true},
		{"/api/contas", true},
		{"/Api/Contas", true},
		{"/apiX", false},
		{"/abc", false}, // 4 caracteres que não são "/api": não pode olhar caminho[:5]
		{"/ap", false},
		{"/", false},
		{"", false},
		{"/index.html", false},
	}
	for _, c := range casos {
		if got := ehAPI(c.caminho); got != c.ehAPI {
			t.Errorf("ehAPI(%q) = %v, esperado %v", c.caminho, got, c.ehAPI)
		}
	}
}

func TestRespostaComStatus(t *testing.T) {
	t.Run("sem WriteHeader, o Write é 200", func(t *testing.T) {
		r := envolver(httptest.NewRecorder())
		if _, err := r.Write([]byte("x")); err != nil {
			t.Fatal(err)
		}
		if r.statusFinal() != http.StatusOK {
			t.Errorf("status = %d", r.statusFinal())
		}
	})
	t.Run("o WriteHeader vale, mesmo com Write depois", func(t *testing.T) {
		r := envolver(httptest.NewRecorder())
		r.WriteHeader(http.StatusNotFound)
		if _, err := r.Write([]byte("x")); err != nil {
			t.Fatal(err)
		}
		if r.statusFinal() != http.StatusNotFound {
			t.Errorf("status = %d", r.statusFinal())
		}
	})
	t.Run("vale o primeiro WriteHeader", func(t *testing.T) {
		r := envolver(httptest.NewRecorder())
		r.WriteHeader(http.StatusCreated)
		r.WriteHeader(http.StatusInternalServerError)
		if r.statusFinal() != http.StatusCreated {
			t.Errorf("status = %d", r.statusFinal())
		}
	})
	t.Run("nada escrito é 200", func(t *testing.T) {
		if s := envolver(httptest.NewRecorder()).statusFinal(); s != http.StatusOK {
			t.Errorf("status = %d", s)
		}
	})
	t.Run("envolver de novo devolve o mesmo", func(t *testing.T) {
		r := envolver(httptest.NewRecorder())
		if envolver(r) != r {
			t.Error("envolveu duas vezes")
		}
	})
}

func TestLimparCaminho(t *testing.T) {
	casos := map[string]string{
		"/":             "/",
		"//":            "/",
		"/media/":       "/media/",
		"/media":        "/media",
		"/a/../b/":      "/b/",
		"/a/./b":        "/a/b",
		"/api//contas/": "/api/contas/",
		"":              "/",
	}
	for entrada, esperado := range casos {
		if got := limparCaminho(entrada); got != esperado {
			t.Errorf("limparCaminho(%q) = %q, esperado %q", entrada, got, esperado)
		}
	}
}

// Uma pasta do site com barra no fim serve o index.html dela (UseDefaultFiles); sem a barra, a pasta não é servida
// como arquivo.
func TestPastaDoSite(t *testing.T) {
	h := handlerDeTeste(t)
	comBarra := pedir(t, h, "GET", "/pasta/", "127.0.0.1:5443")
	if comBarra.Code != http.StatusOK || !strings.Contains(comBarra.Body.String(), "<title>pasta</title>") {
		t.Errorf("/pasta/: %d %q", comBarra.Code, comBarra.Body.String())
	}
	semBarra := pedir(t, h, "GET", "/pasta", "127.0.0.1:5443")
	if strings.Contains(semBarra.Body.String(), "<title>pasta</title>") {
		t.Errorf("/pasta sem barra serviu o index da pasta: %d %q", semBarra.Code, semBarra.Body.String())
	}
	if semBarra.Code == http.StatusOK && !strings.Contains(semBarra.Body.String(), "<title>Finanças</title>") {
		t.Errorf("/pasta sem barra: 200 sem ser o index.html do site: %q", semBarra.Body.String())
	}
}

func TestSegredoDoModoAtras(t *testing.T) {
	rotas := http.NewServeMux()
	if _, err := NovoHandlerAtras(strings.Repeat("s", TamanhoMinimoDoSegredo), rotas, logDescartado); err != nil {
		t.Errorf("segredo com o tamanho mínimo exato foi recusado: %v", err)
	}
	if _, err := NovoHandlerAtras(strings.Repeat("s", TamanhoMinimoDoSegredo-1), rotas, logDescartado); err == nil {
		t.Error("segredo menor que o mínimo foi aceito")
	}
}

// O certificado temporário vale um dia, a partir de uma hora antes de agora (para um relógio um pouco atrasado).
func TestCertificadoTemporarioValeUmDia(t *testing.T) {
	c, err := CertificadoTemporario()
	if err != nil {
		t.Fatal(err)
	}
	if validade := c.Leaf.NotAfter.Sub(c.Leaf.NotBefore); validade != 25*time.Hour {
		t.Errorf("de %v a %v (%v); esperado 1 h antes de agora até 24 h depois", c.Leaf.NotBefore, c.Leaf.NotAfter, validade)
	}
}

// Os prazos do Kestrel (servidor.go e atras_do_dotnet.go usam os mesmos).
func TestPrazosDoServidor(t *testing.T) {
	if prazoDosCabecalhos != 30*time.Second || prazoParado != 130*time.Second {
		t.Errorf("prazos: cabeçalhos %v, parado %v; esperado 30 s e 130 s", prazoDosCabecalhos, prazoParado)
	}
}
