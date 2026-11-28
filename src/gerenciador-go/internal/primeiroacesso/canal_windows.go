package primeiroacesso

import (
	"errors"
	"io"
	"os"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/elevacao"
)

// Pedir é o PedirElevado do Windows: a pasta das chaves só abre para administradores.
var Pedir PedirElevado = pedirPeloElevado

// esperaDoToken é quanto se espera o token depois de o elevado terminar.
const esperaDoToken = 5 * time.Second

// leitorDoPipe lê o handle do servidor do pipe; o pipe quebrado (o cliente fechou) é o fim.
type leitorDoPipe windows.Handle

func (l leitorDoPipe) Read(p []byte) (int, error) {
	var n uint32
	err := windows.ReadFile(windows.Handle(l), p, &n, nil)
	if errors.Is(err, windows.ERROR_BROKEN_PIPE) || (err == nil && n == 0) {
		return int(n), io.EOF
	}
	return int(n), err
}

// criarServidor cria o pipe só para esta vez: entrada apenas, uma instância (FIRST_PIPE_INSTANCE: ninguém ocupou o
// nome antes), recusa quem vem da rede e só o usuário atual (e ninguém mais) pode abrir. O elevado do UAC é o mesmo
// usuário, com o mesmo SID.
func criarServidor(nome string) (windows.Handle, error) {
	usuario, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return 0, err
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;;GA;;;" + usuario.User.Sid.String() + ")")
	if err != nil {
		return 0, err
	}
	atributos := windows.SecurityAttributes{SecurityDescriptor: sd}
	atributos.Length = uint32(unsafe.Sizeof(atributos))
	ponteiro, err := windows.UTF16PtrFromString(nome)
	if err != nil {
		return 0, err
	}
	return windows.CreateNamedPipe(ponteiro, windows.PIPE_ACCESS_INBOUND|windows.FILE_FLAG_FIRST_PIPE_INSTANCE,
		windows.PIPE_TYPE_BYTE|windows.PIPE_READMODE_BYTE|windows.PIPE_WAIT|windows.PIPE_REJECT_REMOTE_CLIENTS,
		1, 0, 512, 0, &atributos)
}

type leitura struct {
	token string
	err   error
}

// pedirPeloElevado cria o pipe, abre a instância elevada (UAC) com o nome dele e lê o token que ela escreve.
func pedirPeloElevado() (string, Resultado) {
	nome, err := NovoNomeDoCanal()
	if err != nil {
		return "", Falhou
	}
	servidor, err := criarServidor(nome)
	if err != nil {
		return "", Falhou
	}
	defer windows.CloseHandle(servidor)
	chegou := make(chan leitura, 1)
	go func() {
		if err := windows.ConnectNamedPipe(servidor, nil); err != nil && !errors.Is(err, windows.ERROR_PIPE_CONNECTED) {
			chegou <- leitura{err: err}
			return
		}
		token, err := LerToken(leitorDoPipe(servidor))
		chegou <- leitura{token, err}
	}()

	resultado := elevacao.Executar("--primeiro-acesso", nome)
	// O elevado terminou. Se ele nunca conectou (UAC cancelado, erro antes), a leitura espera para sempre: conecta-se
	// aqui só para soltá-la (com o cliente já conectado, a abertura falha, e tudo bem).
	if f, err := os.OpenFile(nome, os.O_WRONLY, 0); err == nil {
		f.Close()
	}
	switch resultado {
	case elevacao.Cancelado:
		<-chegou
		return "", Cancelado
	case elevacao.Falhou:
		<-chegou
		return "", Falhou
	}
	select {
	case l := <-chegou:
		if l.err != nil {
			return "", Falhou
		}
		return l.token, Feito
	case <-time.After(esperaDoToken):
		return "", Falhou
	}
}
