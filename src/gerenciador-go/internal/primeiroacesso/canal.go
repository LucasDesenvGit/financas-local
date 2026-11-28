package primeiroacesso

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/instalacao"
)

// No Windows, quem lê o segredo é uma instância elevada (--primeiro-acesso <canal>), e ela só devolve o token, por um
// pipe nomeado que a instância sem elevação criou: nome aleatório, ACL só do usuário, só aceita um cliente e só da
// máquina (canal_windows.go). O elevado não abre navegador nem grava arquivo: escreve uma linha, o token, e sai.

// PrefixoDoCanal é o começo do nome do pipe; o elevado só escreve em nome com esse começo e 32 caracteres hexadecimais.
const PrefixoDoCanal = `\\.\pipe\financas-primeiro-acesso-`

var (
	formatoDoCanal = regexp.MustCompile(`^` + regexp.QuoteMeta(PrefixoDoCanal) + `[0-9a-f]{32}$`)
	formatoDoToken = regexp.MustCompile(`^[0-9]{1,12}\.[0-9a-f]{64}$`)
)

// NovoNomeDoCanal é um nome de pipe que ninguém adivinha.
func NovoNomeDoCanal() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return PrefixoDoCanal + hex.EncodeToString(b), nil
}

// CanalValido: o nome é um que NovoNomeDoCanal faria.
func CanalValido(nome string) bool { return formatoDoCanal.MatchString(nome) }

// EscreverToken é o que o elevado devolve: o token numa linha.
func EscreverToken(w io.Writer, token string) error {
	_, err := io.WriteString(w, token+"\n")
	return err
}

// LerToken lê uma linha de até 256 bytes e só a aceita no formato do token: qualquer outra coisa é erro.
func LerToken(r io.Reader) (string, error) {
	linha, err := bufio.NewReaderSize(io.LimitReader(r, 256), 256).ReadString('\n')
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(linha)
	if !formatoDoToken.MatchString(token) {
		return "", errors.New("o token recebido não tem o formato esperado")
	}
	return token, nil
}

// GerarEEnviar é a parte da instância elevada que não depende do Windows: lê o segredo da pasta das chaves, gera o token
// e o escreve em destino.
func GerarEEnviar(inst instalacao.Instalacao, agora time.Time, destino io.Writer) error {
	segredo, err := LerSegredo(inst.PastaChaves)
	if err != nil {
		return err
	}
	return EscreverToken(destino, Token(segredo, agora))
}

// Elevado é a instância elevada (--primeiro-acesso <canal>): lê o segredo, gera o token e o escreve no pipe. Só escreve
// em nome de pipe do formato certo. O resultado é o código de saída.
func Elevado(canal string, executar func(io.Writer) error) int {
	if !CanalValido(canal) {
		return 2
	}
	pipe, err := os.OpenFile(canal, os.O_WRONLY, 0)
	if err != nil {
		return 1
	}
	defer pipe.Close()
	if err := executar(pipe); err != nil {
		return 1
	}
	return 0
}
