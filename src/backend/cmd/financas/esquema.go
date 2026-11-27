package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
)

// aplicarEsquema é o subcomando que os scripts de instalação chamam (#723): cria ou atualiza o banco, como o
// -aplicar-esquema do site, e sai; diferente do site, aplica o 02-dados-iniciais.sql também em banco que já existe
// (a vigência nova do INSS e do IRRF, ADR 0065). Assim a lista de colunas novas fica só no executável (esquema.ColunasNovas), e o
// bash e o PowerShell não leem mais nada do esquema. Rodar de novo não duplica nada.
func aplicarEsquema(ctx context.Context, argumentos []string, r relogio.Relogio) error {
	opcoes := flag.NewFlagSet("aplicar-esquema", flag.ContinueOnError)
	caminho := opcoes.String("banco", "", "caminho do banco (dados/sql/FinancasLocal.db)")
	if err := opcoes.Parse(argumentos); err != nil {
		return err
	}
	if *caminho == "" {
		return errors.New("aplicar-esquema: informe -banco")
	}
	return banco.AplicarEsquemaEDados(ctx, *caminho, r, slog.New(slog.NewTextHandler(os.Stderr, nil)))
}
