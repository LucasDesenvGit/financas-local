package comando

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// O próprio executável do teste faz o papel do programa do sistema (o padrão dos testes do os/exec): com
// COMANDO_FALSO, ele escreve o que o caso pede e sai, sem rodar os testes.
func TestMain(m *testing.M) {
	switch os.Getenv("COMANDO_FALSO") {
	case "":
		os.Exit(m.Run())
	case "ok":
		fmt.Print("LoadState=loaded\n", os.Getenv("SYSTEMD_PAGER"), "|", os.Getenv("SYSTEMD_COLORS"), "|", strings.Join(os.Args[1:], " "))
	case "erro":
		fmt.Print("saída comum")
		fmt.Fprint(os.Stderr, "  Unit financaslocal.service not loaded.\n")
		os.Exit(5)
	case "erro-sem-stderr":
		fmt.Print(" só no stdout \n")
		os.Exit(1)
	case "mudo":
		os.Exit(3)
	case "demora":
		// Espera ser encerrado pelo limite de tempo (o processo é morto, então o sono nunca termina).
		time.Sleep(time.Minute)
	}
	os.Exit(0)
}

func falso(t *testing.T, caso string) string {
	t.Helper()
	t.Setenv("COMANDO_FALSO", caso)
	return os.Args[0]
}

// Regra: com sucesso, devolve o stdout, com os argumentos passados separados e o systemd sem paginador nem cores.
func TestRodarDevolveASaida(t *testing.T) {
	saida, err := Rodar(context.Background(), 10*time.Second, falso(t, "ok"), "--user", "show", "a b")
	if err != nil || saida != "LoadState=loaded\n|0|--user show a b" {
		t.Errorf("%q, %v", saida, err)
	}
}

// Regra: com erro, a mensagem é a do programa (stderr; sem ele, o stdout), sem espaços sobrando; sem nenhuma das duas,
// o nome do programa e o código.
func TestRodarComErro(t *testing.T) {
	casos := map[string]string{
		"erro":            "Unit financaslocal.service not loaded.",
		"erro-sem-stderr": "só no stdout",
	}
	for caso, esperado := range casos {
		if saida, err := Rodar(context.Background(), 10*time.Second, falso(t, caso)); saida != "" || err == nil || err.Error() != esperado {
			t.Errorf("%s: %q, %v", caso, saida, err)
		}
	}
	if _, err := Rodar(context.Background(), 10*time.Second, falso(t, "mudo")); err == nil || !strings.HasSuffix(err.Error(), "exit status 3") {
		t.Errorf("mudo: %v", err)
	}
}

// Regra: programa que não existe e programa que passa do limite viram frases para a janela.
func TestRodarSemProgramaEComDemora(t *testing.T) {
	if _, err := Rodar(context.Background(), 10*time.Second, "financaslocal-programa-que-nao-existe"); err == nil ||
		err.Error() != "o programa financaslocal-programa-que-nao-existe não foi encontrado neste computador" {
		t.Errorf("sem programa: %v", err)
	}
	if _, err := Rodar(context.Background(), 200*time.Millisecond, falso(t, "demora")); err == nil || !strings.Contains(err.Error(), " não respondeu em ") {
		t.Errorf("demora: %v", err)
	}
}
