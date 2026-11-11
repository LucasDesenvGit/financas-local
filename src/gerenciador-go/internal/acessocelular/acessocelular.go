// Package acessocelular liga e desliga o acesso pelo celular do site direto pelo gerenciador (#684, ADR 0061), sem
// login: lê o segredo que o site guarda na pasta das chaves e chama POST /api/celular/gerenciador/{ligar,desligar},
// que o site só aceita do próprio computador e com esse segredo. No Windows, a pasta das chaves só é lida por
// administradores: o gerenciador roda isto elevado (--celular), e o Resultado volta como código de saída.
package acessocelular

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/elevacao"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/instalacao"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/situacao"
)

// ArquivoChave e Cabecalho são os do site (celular.ArquivoChaveGerenciador e CabecalhoChaveGerenciador).
const (
	ArquivoChave = "gerenciador-celular.chave"
	Cabecalho    = "X-Gerenciador-Chave"
)

// inicioSemRede é o começo da celular.MensagemSemRede do site, que vem no 409 quando o computador não está numa rede
// de casa (o outro 409 é a porta ou o firewall).
const inicioSemRede = "Este computador não está numa rede de casa"

// Resultado de ligar ou desligar. O valor é o código de saída do gerenciador elevado (0 = Feito).
type Resultado int

const (
	Feito Resultado = iota
	// Falhou: erro sem motivo conhecido (o site deu 500, caiu no meio, ou a resposta não se lê).
	Falhou
	// ComAviso: ligou, mas o site avisa algo (no Windows, a rede como Pública, que deixa a porta fechada).
	ComAviso
	// SemRede: o computador não tem IP numa rede de casa.
	SemRede
	// PortaFechada: a porta não abriu (outro programa nela) ou o firewall não deixou.
	PortaFechada
	// SemPermissao: sem o segredo (não deu para ler o arquivo) ou o site não aceitou (versão antiga, sem a rota).
	SemPermissao
	// Cancelado: o aviso do UAC foi cancelado (só no Windows; não é código de saída).
	Cancelado
)

// DoCodigo é o Resultado de um código de saída do gerenciador elevado; código desconhecido é Falhou.
func DoCodigo(codigo uint32) Resultado {
	if r := Resultado(codigo); codigo <= uint32(SemPermissao) {
		return r
	}
	return Falhou
}

// Mudar liga (ligar) ou desliga o acesso no site da instalação.
func Mudar(ctx context.Context, cliente *http.Client, site, pastaChaves string, ligar bool) Resultado {
	dados, err := os.ReadFile(filepath.Join(pastaChaves, ArquivoChave))
	if err != nil {
		return SemPermissao
	}
	rota := "api/celular/gerenciador/desligar"
	if ligar {
		rota = "api/celular/gerenciador/ligar"
	}
	pedido, err := http.NewRequestWithContext(ctx, http.MethodPost, site+rota, nil)
	if err != nil {
		return Falhou
	}
	pedido.Header.Set(Cabecalho, strings.TrimSpace(string(dados)))
	resposta, err := cliente.Do(pedido)
	if err != nil {
		return Falhou
	}
	defer resposta.Body.Close()
	var corpo struct {
		Ligado   *bool    `json:"ligado"`
		Avisos   []string `json:"avisos"`
		Mensagem string   `json:"mensagem"`
	}
	conteudo, err := io.ReadAll(io.LimitReader(resposta.Body, 64<<10))
	if err != nil {
		return Falhou
	}
	leu := json.Unmarshal(conteudo, &corpo) == nil
	switch resposta.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return SemPermissao
	case http.StatusConflict:
		if leu && strings.HasPrefix(corpo.Mensagem, inicioSemRede) {
			return SemRede
		}
		return PortaFechada
	default:
		return Falhou
	}
	if !leu || corpo.Ligado == nil || *corpo.Ligado != ligar {
		return Falhou
	}
	if ligar && len(corpo.Avisos) > 0 {
		return ComAviso
	}
	return Feito
}

// Texto é o que a janela diz depois (vazio: o cartão já mostra o estado novo).
func Texto(r Resultado) string {
	switch r {
	case Feito:
		return ""
	case ComAviso:
		return "Ligado, mas falta um passo neste computador: veja o aviso em “Ver no site”."
	case SemRede:
		return "Este computador não está numa rede de casa (Wi-Fi ou cabo): não há endereço para o celular abrir."
	case PortaFechada:
		return "Não deu para abrir a porta: outro programa pode estar usando, ou o firewall não deixou. Os detalhes estão no site, em Configurações › Erros."
	case SemPermissao:
		return "Não deu para mudar daqui. Use “Ver no site”, que liga e desliga com o seu login."
	case Cancelado:
		return "Cancelado: nada mudou."
	default:
		return "Não deu certo. Use “Ver no site” para tentar por lá; os detalhes ficam em Configurações › Erros."
	}
}

// NaInstalacao é o Mudar no site deste computador, com o cliente da situação (que só fala com o loopback) e mais
// tempo: abrir a porta no firewall leva alguns segundos.
func NaInstalacao(inst instalacao.Instalacao, ligar bool) Resultado {
	cliente := situacao.ClientePara(inst.Site)
	cliente.Timeout = 30 * time.Second
	ctx, cancelar := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancelar()
	return Mudar(ctx, cliente, inst.Site, inst.PastaChaves, ligar)
}

// DoElevado traduz o que o gerenciador elevado (--celular) devolveu: o UAC cancelado, o código de saída, ou Falhou
// quando nem rodou (sem código).
func DoElevado(r elevacao.Resultado, codigo uint32) Resultado {
	switch {
	case r == elevacao.Feito:
		return Feito
	case r == elevacao.Cancelado:
		return Cancelado
	case codigo == 0:
		return Falhou
	default:
		return DoCodigo(codigo)
	}
}
