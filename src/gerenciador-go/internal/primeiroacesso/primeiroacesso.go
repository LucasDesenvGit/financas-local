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

// Endereco é o que "Abrir o site" abre. Só o início do site ("") muda: se o site responde que ainda não há usuário e o
// segredo pode ser lido, é o link do primeiro acesso; senão, a página pedida, como sempre.
func Endereco(ctx context.Context, cliente *http.Client, inst instalacao.Instalacao, pagina string, agora time.Time) string {
	padrao := inst.Pagina(pagina)
	if pagina != "" || !semUsuario(ctx, cliente, inst) {
		return padrao
	}
	dados, err := os.ReadFile(filepath.Join(inst.PastaChaves, ArquivoChave))
	if err != nil {
		return padrao
	}
	return inst.Pagina("entrar?configurar=" + Token(strings.TrimSpace(string(dados)), agora))
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
