// Package situacao responde "o site está funcionando?", como o VerificadorSituacao do Situacao.cs: pergunta ao
// serviço do sistema e, com ele rodando, se o site responde (GET /api/auth/estado, que não exige login).
package situacao

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/instalacao"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/servico"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/tela"
)

// Consulta junta o que é preciso para ver a situação; os campos são trocados nos testes.
type Consulta struct {
	Instalacao   instalacao.Instalacao
	Servico      func() (servico.Estado, error)
	ComoInstalar string
	Cliente      *http.Client
	// PortaOcupada diz se algum programa já atende na porta do site.
	PortaOcupada func(porta string) bool
}

// Nova é a consulta deste computador.
func Nova(i instalacao.Instalacao) Consulta {
	return Consulta{Instalacao: i, Servico: servico.Consultar, ComoInstalar: servico.ComoInstalar,
		Cliente: ClientePara(i.Site), PortaOcupada: portaOcupada}
}

// ClientePara pergunta só se o site responde, neste computador, usando validação TLS padrão.
func ClientePara(site string) *http.Client {
	transporte := &http.Transport{Proxy: nil}
	_, _ = url.Parse(site)
	return &http.Client{Transport: transporte, Timeout: 4 * time.Second}
}

// Consultar é a situação agora.
func (c Consulta) Consultar(ctx context.Context) tela.Situacao {
	estado, err := c.Servico()
	if err != nil {
		return tela.Situacao{Estado: tela.SemResposta, Titulo: "Não deu para ver a situação do site.", Detalhe: err.Error()}
	}
	switch estado {
	case servico.NaoInstalado:
		return tela.Situacao{Estado: tela.NaoInstalado, Titulo: "O Finanças Local não está instalado neste computador.",
			Detalhe: c.ComoInstalar}
	case servico.Parado:
		detalhe := "Clique em “Ligar” para usar."
		if porta := c.porta(); c.PortaOcupada(porta) {
			detalhe = "A porta " + porta + " está em uso por outro programa: o site não liga enquanto ela estiver ocupada."
		}
		return tela.Situacao{Estado: tela.Desligado, Titulo: "O site está desligado.", Detalhe: detalhe}
	case servico.Ligando:
		return tela.Situacao{Estado: tela.Mudando, Titulo: "O site está ligando…", Detalhe: "Aguarde alguns segundos."}
	case servico.Desligando:
		return tela.Situacao{Estado: tela.Mudando, Titulo: "O site está desligando…", Detalhe: "Aguarde alguns segundos."}
	}

	if versao, ok := c.responde(ctx); ok {
		detalhe := c.Instalacao.Site
		if versao != "" {
			detalhe = "Versão " + versao + " · " + c.Instalacao.Site
		}
		return tela.Situacao{Estado: tela.NoAr, Titulo: "O site está no ar.", Detalhe: detalhe, VersaoSite: versao}
	}
	return tela.Situacao{Estado: tela.SemResposta, Titulo: "O site está ligado, mas não responde.",
		Detalhe: "Acabou de ligar? Espere alguns segundos. Se continuar assim, clique em “Reiniciar”."}
}

// responde diz se o site respondeu, e a versão que ele informa (versões antigas não informam).
func (c Consulta) responde(ctx context.Context) (versao string, ok bool) {
	pedido, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Instalacao.Pagina("api/auth/estado"), nil)
	if err != nil {
		return "", false
	}
	resposta, err := c.Cliente.Do(pedido)
	if err != nil {
		return "", false
	}
	defer resposta.Body.Close()
	if resposta.StatusCode < 200 || resposta.StatusCode > 299 {
		return "", false
	}
	var corpo struct {
		Versao any `json:"versao"`
	}
	if dados, err := io.ReadAll(io.LimitReader(resposta.Body, 64<<10)); err == nil && json.Unmarshal(dados, &corpo) == nil {
		versao, _ = corpo.Versao.(string)
	}
	return versao, true
}

// porta do endereço do site (443 quando ele não diz).
func (c Consulta) porta() string {
	u, err := url.Parse(c.Instalacao.Site)
	if err != nil || u.Port() == "" {
		return "443"
	}
	return u.Port()
}

// portaOcupada tenta conectar na porta pelo loopback: se alguém atende, ela está em uso. No loopback a resposta é
// imediata (aceita ou recusada).
func portaOcupada(porta string) bool {
	conexao, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", porta))
	if err != nil {
		return false
	}
	conexao.Close()
	return true
}
