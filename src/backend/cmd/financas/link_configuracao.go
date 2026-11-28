package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/autenticacao"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/celular"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
)

// linkDeConfiguracao é o subcomando link-de-configuracao (#68, item 4, ADR 0075): imprime o endereço que abre o
// primeiro acesso (criar o usuário). Quem o roda precisa ler a pasta das chaves, que é o que prova que é o dono; os
// scripts de instalação mostram o resultado no fim. O link vale 10 minutos: rode de novo para outro.
func linkDeConfiguracao(argumentos []string, saida io.Writer, r relogio.Relogio) error {
	opcoes := flag.NewFlagSet("link-de-configuracao", flag.ContinueOnError)
	chaves := opcoes.String("chaves", "", "pasta das chaves (dados/chaves)")
	url := opcoes.String("url", "https://localhost:5443", "endereço do site")
	if err := opcoes.Parse(argumentos); err != nil {
		return err
	}
	if *chaves == "" {
		return errors.New("link-de-configuracao: informe -chaves")
	}
	segredo, err := celular.LerChaveDoGerenciador(*chaves)
	if err != nil {
		return fmt.Errorf("%w (o site cria o arquivo ao subir pela primeira vez; esta conta precisa ler a pasta)", err)
	}
	_, err = fmt.Fprintf(saida, "%s/entrar?configurar=%s\n", strings.TrimRight(*url, "/"), autenticacao.TokenDeConfiguracao(segredo, r.Agora()))
	return err
}
