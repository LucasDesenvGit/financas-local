package celular

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/respostas"
)

// O gerenciador liga e desliga o acesso direto (#684), sem login: ele não tem a senha do site. Quem prova que pode é
// um segredo próprio, que o site cria em dados/chaves. No Windows, essa pasta só é lida pelo SYSTEM e pelos
// Administradores, então o gerenciador pede o UAC para ler; no Linux, é do usuário do serviço (0600). Quem tem o
// segredo só consegue ligar e desligar o acesso: ele não abre sessão nem lê dado nenhum.
const (
	// ArquivoChaveGerenciador é o segredo, em hexadecimal, na pasta das chaves.
	ArquivoChaveGerenciador = "gerenciador-celular.chave"
	// CabecalhoChaveGerenciador leva o segredo. Um cabeçalho próprio também barra o pedido de um site de fora: o
	// navegador só o manda depois do preflight do CORS, que o site não aceita.
	CabecalhoChaveGerenciador = "X-Gerenciador-Chave"
	// Rotas do gerenciador: sem login e sem CSRF (a autenticação deixa passar), só do próprio computador e com o
	// segredo. Sem isso, 404, como se não existissem.
	RotaGerenciadorLigar    = "/api/celular/gerenciador/ligar"
	RotaGerenciadorDesligar = "/api/celular/gerenciador/desligar"
)

// tamanhoChaveGerenciador: 32 bytes aleatórios, 64 caracteres no arquivo.
const tamanhoChaveGerenciador = 32

// ErrSemRede: ligar sem escolher, e este computador não tem IP numa rede de casa.
var ErrSemRede = errors.New("este computador não tem endereço numa rede de casa")

// MensagemSemRede é a do ErrSemRede.
const MensagemSemRede = "Este computador não está numa rede de casa (Wi-Fi ou cabo): não há endereço para o celular abrir."

// CarregarChaveDoGerenciador lê o segredo da pasta das chaves, ou o cria (O_EXCL: se outro processo criou agora, fica
// o dele). Arquivo com outro tamanho ou que não é hexadecimal é erro: apagar faz o site criar outro.
func CarregarChaveDoGerenciador(pasta string) (string, error) {
	caminho := filepath.Join(pasta, ArquivoChaveGerenciador)
	if dados, err := os.ReadFile(caminho); err == nil {
		chave := strings.TrimSpace(string(dados))
		if b, err := hex.DecodeString(chave); err != nil || len(b) != tamanhoChaveGerenciador {
			return "", fmt.Errorf("a chave do gerenciador %s não é válida: apague o arquivo para o site criar outra", caminho)
		}
		return chave, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("ler a chave do gerenciador %s: %w", caminho, err)
	}

	if err := os.MkdirAll(pasta, 0o700); err != nil {
		return "", fmt.Errorf("criar a pasta das chaves %s: %w", pasta, err)
	}
	b := make([]byte, tamanhoChaveGerenciador)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	chave := hex.EncodeToString(b)
	arquivo, err := os.OpenFile(caminho, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return CarregarChaveDoGerenciador(pasta)
	}
	if err != nil {
		return "", fmt.Errorf("criar a chave do gerenciador %s: %w", caminho, err)
	}
	if _, err := arquivo.WriteString(chave); err != nil {
		arquivo.Close()
		return "", fmt.Errorf("gravar a chave do gerenciador %s: %w", caminho, err)
	}
	if err := arquivo.Close(); err != nil {
		return "", fmt.Errorf("gravar a chave do gerenciador %s: %w", caminho, err)
	}
	return chave, nil
}

// doGerenciador: o pedido veio do próprio computador e traz o segredo (comparado em tempo constante). Sem segredo
// carregado, ninguém passa.
func (s *Servico) doGerenciador(r *http.Request) bool {
	if s.o.ChaveGerenciador == "" || !doProprioComputador(r) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(r.Header.Get(CabecalhoChaveGerenciador)), []byte(s.o.ChaveGerenciador)) == 1
}

func (s *Servico) ligarPeloGerenciadorHTTP(w http.ResponseWriter, r *http.Request) {
	if !s.doGerenciador(r) {
		respostas.SemCorpo(w, http.StatusNotFound)
		return
	}
	sit, err := s.LigarSemEscolher(r.Context(), r)
	s.responder(w, r, sit, err, "ligar pelo gerenciador")
}

func (s *Servico) desligarPeloGerenciadorHTTP(w http.ResponseWriter, r *http.Request) {
	if !s.doGerenciador(r) {
		respostas.SemCorpo(w, http.StatusNotFound)
		return
	}
	sit, err := s.Desligar(r.Context(), r)
	s.responder(w, r, sit, err, "desligar pelo gerenciador")
}

// LigarSemEscolher é o ligar do gerenciador, que não mostra a lista de IPs: já ligado, fica como está; senão, o IP
// gravado, se ainda for deste computador, ou o preferido (o da rota padrão, primeiro da lista).
func (s *Servico) LigarSemEscolher(ctx context.Context, r *http.Request) (Situacao, error) {
	s.mu.Lock()
	if s.escuta != nil {
		sit := s.situacao(ctx)
		s.mu.Unlock()
		return sit, nil
	}
	s.mu.Unlock()

	gravado, err := s.conf.Obter(ctx, ChaveIP)
	if err != nil {
		return Situacao{}, err
	}
	if gravado != nil {
		if e, ok := s.doComputador(*gravado); ok {
			return s.Ligar(ctx, r, e.IP)
		}
	}
	enderecos, err := s.o.Enderecos()
	if err != nil {
		return Situacao{}, fmt.Errorf("listar os IPs deste computador: %w", err)
	}
	if len(enderecos) == 0 {
		return Situacao{}, ErrSemRede
	}
	return s.Ligar(ctx, r, enderecos[0].IP)
}
