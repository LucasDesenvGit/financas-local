package servico

import (
	"context"
	"errors"
	"fmt"
	"os/user"
	"strconv"
	"strings"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/comando"
)

// ComoInstalar é o que dizer quando o site não está instalado.
const ComoInstalar = "Instale pelos scripts: rode scripts/geral/instalar.sh na pasta do projeto."

// Consultar lê o estado da unidade do systemd --user.
func Consultar() (Estado, error) {
	p, err := Mostrar("LoadState", "ActiveState")
	if err != nil {
		return NaoInstalado, err
	}
	return EstadoDaUnidade(p), nil
}

// Mostrar lê propriedades da unidade (systemctl --user show).
func Mostrar(nomes ...string) (map[string]string, error) {
	argumentos := []string{"--user", "show", Unidade}
	for _, nome := range nomes {
		argumentos = append(argumentos, "-p", nome)
	}
	saida, err := comando.Rodar(context.Background(), 10*time.Second, "systemctl", argumentos...)
	if err != nil {
		return nil, fmt.Errorf("Não deu para falar com o systemd do usuário (systemctl --user). %w", err)
	}
	return Propriedades(saida), nil
}

// Executar liga, desliga ou reinicia pelo systemctl --user, que espera o serviço mudar. No Linux, nada disso pede
// administrador: roda no próprio gerenciador.
func Executar(acao string) error {
	verbo, ok := verbos[acao]
	if !ok {
		return errors.New("ação desconhecida: " + acao)
	}
	_, err := comando.Rodar(context.Background(), 30*time.Second, "systemctl", "--user", verbo, Unidade)
	return err
}

// SobeSemLogin diz se o site fica no ar sem a sessão aberta: o serviço do usuário só roda com ela, a não ser com o
// "linger" do systemd-logind. sabe é false quando não dá para saber (sem logind).
func SobeSemLogin() (sobe, sabe bool) {
	u, err := user.Current()
	if err != nil {
		return false, false
	}
	saida, err := comando.Rodar(context.Background(), 10*time.Second, "loginctl", "show-user", u.Username, "-p", "Linger", "--value")
	if err != nil {
		return false, false
	}
	return strings.TrimSpace(saida) == "yes", true
}

// ManterNoArSemLogin liga o "linger" do próprio usuário. Na maioria das distribuições a sessão ativa pode fazer isso
// sem senha; se a política pedir, o próprio sistema mostra a janela de autenticação.
func ManterNoArSemLogin() error {
	_, err := comando.Rodar(context.Background(), 2*time.Minute, "loginctl", "enable-linger")
	return err
}

// Registro é o fim do registro do serviço (journald): o que a API escreveu, inclusive ao não ligar.
func Registro(linhas int) (string, error) {
	return comando.Rodar(context.Background(), 10*time.Second, "journalctl", "--user", "-u", Unidade,
		"-n", strconv.Itoa(linhas), "--no-pager", "-o", "short-iso")
}
