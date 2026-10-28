package situacao

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/instalacao"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/servico"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/tela"
)

func consulta(estado servico.Estado, err error, site string, ocupada bool) Consulta {
	return Consulta{
		Instalacao:   instalacao.Instalacao{Site: site},
		Servico:      func() (servico.Estado, error) { return estado, err },
		ComoInstalar: "Instale pelo arquivo FinancasLocal.msi.",
		Cliente:      ClientePara(site),
		PortaOcupada: func(string) bool { return ocupada },
	}
}

// siteFalso responde ao /api/auth/estado com o status e o corpo dados, no loopback, com certificado do teste.
func siteFalso(t *testing.T, status int, corpo string) string {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/auth/estado" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(corpo))
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/"
}

// Regra: os textos de cada estado do serviço, os do Situacao.cs.
func TestEstadosDoServico(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome    string
		estado  servico.Estado
		ocupada bool
		quer    tela.Situacao
	}{
		{"não instalado", servico.NaoInstalado, false, tela.Situacao{Estado: tela.NaoInstalado,
			Titulo: "O Finanças Local não está instalado neste computador.", Detalhe: "Instale pelo arquivo FinancasLocal.msi."}},
		{"parado", servico.Parado, false, tela.Situacao{Estado: tela.Desligado, Titulo: "O site está desligado.",
			Detalhe: "Clique em “Ligar” para usar."}},
		{"parado com a porta ocupada", servico.Parado, true, tela.Situacao{Estado: tela.Desligado, Titulo: "O site está desligado.",
			Detalhe: "A porta 5443 está em uso por outro programa: o site não liga enquanto ela estiver ocupada."}},
		{"ligando", servico.Ligando, false, tela.Situacao{Estado: tela.Mudando, Titulo: "O site está ligando…", Detalhe: "Aguarde alguns segundos."}},
		{"desligando", servico.Desligando, false, tela.Situacao{Estado: tela.Mudando, Titulo: "O site está desligando…", Detalhe: "Aguarde alguns segundos."}},
	}
	for _, c := range casos {
		if obtida := consulta(c.estado, nil, "https://127.0.0.1:5443/", c.ocupada).Consultar(t.Context()); obtida != c.quer {
			t.Errorf("%s: %+v", c.nome, obtida)
		}
	}
}

// Regra: erro ao perguntar ao serviço vira "Não deu para ver a situação do site.", com o erro no detalhe.
func TestErroDoServico(t *testing.T) {
	t.Parallel()
	obtida := consulta(0, errors.New("acesso negado"), "https://127.0.0.1:5443/", false).Consultar(t.Context())
	if obtida != (tela.Situacao{Estado: tela.SemResposta, Titulo: "Não deu para ver a situação do site.", Detalhe: "acesso negado"}) {
		t.Errorf("%+v", obtida)
	}
}

// Regra: rodando e respondendo, "no ar", com a versão que o site informa; sem versão (site antigo, versão que não é
// texto ou JSON que não lê), só o endereço.
func TestNoAr(t *testing.T) {
	t.Parallel()
	casos := map[string]struct{ versao, detalhe string }{
		`{"versao":"2.43.1","logado":false}`: {"2.43.1", "Versão 2.43.1 · "},
		`{"logado":false}`:                   {"", ""},
		`{"versao":243}`:                     {"", ""},
		`não é JSON`:                         {"", ""},
	}
	for corpo, quer := range casos {
		site := siteFalso(t, http.StatusOK, corpo)
		obtida := consulta(servico.Rodando, nil, site, false).Consultar(t.Context())
		esperada := tela.Situacao{Estado: tela.NoAr, Titulo: "O site está no ar.", Detalhe: quer.detalhe + site, VersaoSite: quer.versao}
		if obtida != esperada {
			t.Errorf("%s: %+v", corpo, obtida)
		}
	}
}

// Regra: rodando sem responder (erro, status fora de 200 a 299 ou ninguém na porta), "ligado, mas não responde".
func TestSemResposta(t *testing.T) {
	t.Parallel()
	ouvinte, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	fechado := "https://" + ouvinte.Addr().String() + "/"
	ouvinte.Close()
	for _, site := range []string{siteFalso(t, http.StatusInternalServerError, `{"versao":"1"}`),
		siteFalso(t, 300, `{"versao":"1"}`), fechado} {
		obtida := consulta(servico.Rodando, nil, site, false).Consultar(t.Context())
		if obtida.Estado != tela.SemResposta || obtida.Titulo != "O site está ligado, mas não responde." ||
			obtida.Detalhe != "Acabou de ligar? Espere alguns segundos. Se continuar assim, clique em “Reiniciar”." {
			t.Errorf("%s: %+v", site, obtida)
		}
	}
}

// Regra: o 299 ainda é sucesso.
func TestStatus299(t *testing.T) {
	t.Parallel()
	site := siteFalso(t, 299, `{}`)
	if obtida := consulta(servico.Rodando, nil, site, false).Consultar(t.Context()); obtida.Estado != tela.NoAr {
		t.Errorf("%+v", obtida)
	}
}

// Regra: o certificado só deixa de ser conferido no loopback; nunca há proxy.
func TestClientePara(t *testing.T) {
	t.Parallel()
	for site, semConferir := range map[string]bool{
		"https://127.0.0.1:5443/": true, "https://localhost:5443/": true, "https://[::1]:5443/": true,
		"https://financas.exemplo:5443/": false, "https://192.168.0.10/": false, "%": false,
	} {
		transporte := ClientePara(site).Transport.(*http.Transport)
		obtido := transporte.TLSClientConfig != nil && transporte.TLSClientConfig.InsecureSkipVerify
		if obtido != semConferir || transporte.Proxy != nil {
			t.Errorf("%s: sem conferir %v", site, obtido)
		}
	}
	if ClientePara("https://127.0.0.1/").Timeout.Seconds() != 4 {
		t.Error("tempo limite")
	}
}

// Regra: a porta é a do endereço; sem porta, a 443.
func TestPorta(t *testing.T) {
	t.Parallel()
	for site, porta := range map[string]string{"https://127.0.0.1:5443/": "5443", "https://127.0.0.1/": "443", "%": "443"} {
		if obtida := (Consulta{Instalacao: instalacao.Instalacao{Site: site}}).porta(); obtida != porta {
			t.Errorf("%s: %s", site, obtida)
		}
	}
}

// Regra: a porta com alguém atendendo está ocupada; a fechada, não.
func TestPortaOcupada(t *testing.T) {
	t.Parallel()
	ouvinte, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, porta, _ := net.SplitHostPort(ouvinte.Addr().String())
	if !portaOcupada(porta) {
		t.Error("ocupada não percebida")
	}
	ouvinte.Close()
	if portaOcupada(porta) {
		t.Error("livre como ocupada")
	}
}
