package autenticacao

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
)

// A configuração inicial (criar o primeiro usuário) só abre com um token de quem lê o segredo do gerenciador
// (dados/chaves/gerenciador-celular.chave, ADR 0061 e 0075): outro programa do computador que não lê essa pasta não
// chega a criar o usuário antes do dono. O token é <instante em segundos>.<HMAC-SHA256 em hexadecimal>, vale 10 minutos
// e não revela o segredo. Quem o gera é o gerenciador (que abre o site com ele no endereço) ou o subcomando
// link-de-configuracao do executável.
const (
	// CabecalhoTokenConfiguracao leva o token no POST /api/auth/configurar.
	CabecalhoTokenConfiguracao = "X-Configurar-Token"
	// ValidadeTokenConfiguracao é quanto o token vale depois de gerado.
	ValidadeTokenConfiguracao = 10 * time.Minute

	// toleranciaRelogio aceita um token de poucos segundos no futuro (relógios quase iguais, processos diferentes).
	toleranciaRelogio = time.Minute
)

// MensagemSemTokenConfiguracao é o 403 de quem tenta criar o usuário sem o token, ou com um vencido.
const MensagemSemTokenConfiguracao = "Para criar o usuário, abra o site pelo gerenciador (Abrir o site) ou pelo link que a instalação mostrou: o link vale por 10 minutos."

// TokenDeConfiguracao gera o token para o instante dado, a partir do segredo (o texto do arquivo, sem espaços nas
// pontas).
func TokenDeConfiguracao(segredo string, agora time.Time) string {
	instante := strconv.FormatInt(agora.Unix(), 10)
	return instante + "." + assinarConfiguracao(segredo, instante)
}

// TokenDeConfiguracaoValido diz se o token foi gerado com o segredo e ainda vale. Sem segredo carregado, nenhum token
// vale.
func TokenDeConfiguracaoValido(segredo, token string, agora time.Time) bool {
	if segredo == "" {
		return false
	}
	instante, assinatura, ok := strings.Cut(strings.TrimSpace(token), ".")
	if !ok {
		return false
	}
	segundos, err := strconv.ParseInt(instante, 10, 64)
	if err != nil {
		return false
	}
	idade := agora.Sub(time.Unix(segundos, 0))
	if idade > ValidadeTokenConfiguracao || idade < -toleranciaRelogio {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(assinatura), []byte(assinarConfiguracao(segredo, instante))) == 1
}

func assinarConfiguracao(segredo, instante string) string {
	h := hmac.New(sha256.New, []byte(segredo))
	h.Write([]byte("configurar|" + instante))
	return hex.EncodeToString(h.Sum(nil))
}
