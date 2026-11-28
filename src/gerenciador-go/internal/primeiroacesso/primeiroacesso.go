// Package primeiroacesso monta o endereço que o gerenciador abre no navegador enquanto o site ainda não tem usuário
// (#68, item 4, ADR 0075): o site só deixa criar o primeiro usuário com um token, que quem lê o segredo da pasta das
// chaves (o mesmo do acesso pelo celular, ADR 0061) consegue gerar. Sem poder ler o segredo (no Windows, a pasta é só
// de administradores), abre a página de sempre, que explica o que falta.
package primeiroacesso

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/instalacao"
)

// ArquivoChave é o segredo do site (celular.ArquivoChaveGerenciador).
const ArquivoChave = "gerenciador-celular.chave"

// Token é o do site (autenticacao.TokenDeConfiguracao): <instante em segundos>.<HMAC-SHA256 de "configurar|instante",
// em hexadecimal>, com o texto do arquivo como chave. Vale 10 minutos. O valor de referência do teste é o mesmo do
// teste do site: se a fórmula mudar lá, muda aqui.
func Token(segredo string, agora time.Time) string {
	instante := strconv.FormatInt(agora.Unix(), 10)
	h := hmac.New(sha256.New, []byte(segredo))
	h.Write([]byte("configurar|" + instante))
	return instante + "." + hex.EncodeToString(h.Sum(nil))
}

// Resultado do pedido do token ao gerenciador elevado (Windows).
type Resultado int

const (
	Feito Resultado = iota
	Cancelado
	Falhou
)

// PedirElevado devolve o token gerado por uma instância elevada do gerenciador (--primeiro-acesso), para quando a pasta
// das chaves só abre para administradores. Nil: não há elevação (Linux).
type PedirElevado func() (string, Resultado)

// Avisos de quando o link não pôde ser feito: o site abre a página de sempre, que explica o que falta.
const (
	AvisoCancelado = "Sem a permissão de administrador não dá para gerar o link do primeiro acesso: o site abriu sem ele e não deixa criar o usuário. Clique em “Abrir o site” de novo e aceite o aviso."
	AvisoFalhou    = "Não deu para gerar o link do primeiro acesso: o site abriu sem ele e não deixa criar o usuário. Tente de novo em “Abrir o site”."
)

// Endereco é o que "Abrir o site" abre, e o aviso a mostrar (vazio se não há). Só o início do site ("") muda: se o site
// responde que ainda não há usuário, é o link do primeiro acesso, com o token feito do segredo da pasta das chaves;
// se a pasta não abre para esta conta, pede o token à instância elevada (uma vez: com o usuário criado, nunca mais).
// Sem conseguir, é a página de sempre, com o aviso. Quem abre o navegador é quem chamou, sem elevação.
func Endereco(ctx context.Context, cliente *http.Client, inst instalacao.Instalacao, pagina string, agora time.Time, elevado PedirElevado) (endereco, aviso string) {
	padrao := inst.Pagina(pagina)
	if pagina != "" || !semUsuario(ctx, cliente, inst) {
		return padrao, ""
	}
	if segredo, err := LerSegredo(inst.PastaChaves); err == nil {
		return link(inst, Token(segredo, agora)), ""
	}
	if elevado == nil {
		return padrao, ""
	}
	token, r := elevado()
	switch {
	case r == Feito && formatoDoToken.MatchString(token):
		return link(inst, token), ""
	case r == Cancelado:
		return padrao, AvisoCancelado
	default:
		return padrao, AvisoFalhou
	}
}

// LerSegredo é o texto do arquivo do segredo, sem espaços nas pontas.
func LerSegredo(pastaChaves string) (string, error) {
	dados, err := os.ReadFile(filepath.Join(pastaChaves, ArquivoChave))
	return strings.TrimSpace(string(dados)), err
}

func link(inst instalacao.Instalacao, token string) string {
	return inst.Pagina("entrar?configurar=" + token)
}

// semUsuario pergunta ao site (GET /api/auth/estado, sem login) se ainda não há usuário. Qualquer dúvida é "não".
func semUsuario(ctx context.Context, cliente *http.Client, inst instalacao.Instalacao) bool {
	pedido, err := http.NewRequestWithContext(ctx, http.MethodGet, inst.Pagina("api/auth/estado"), nil)
	if err != nil {
		return false
	}
	resposta, err := cliente.Do(pedido)
	if err != nil {
		return false
	}
	defer resposta.Body.Close()
	var corpo struct {
		Configurado *bool `json:"configurado"`
	}
	dados, err := io.ReadAll(io.LimitReader(resposta.Body, 64<<10))
	if err != nil || resposta.StatusCode != http.StatusOK || json.Unmarshal(dados, &corpo) != nil {
		return false
	}
	return corpo.Configurado != nil && !*corpo.Configurado
}
