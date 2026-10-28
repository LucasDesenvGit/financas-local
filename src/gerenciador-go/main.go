// O gerenciador do site em Go (ADR 0052): o ícone na bandeja e a janela própria com o Wails v3, igual à do
// gerenciador em C# (src/gerenciador/Financas.Gerenciador). A página é servida por dentro do programa, sem porta
// aberta.
//
// A versão nova (procurar, baixar e instalar) entra na #420; o Linux, na #417.
package main

import (
	"errors"
	"flag"
	"log"
	"os"

	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/copia"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/instalacao"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/servico"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/tela"
)

// versao vem do Directory.Build.props na compilação (-ldflags "-X main.versao=...").
var versao = "0.0.0-dev.0"

// Situações de exemplo do -mostrar, com os textos do Situacao.cs, para comparar a janela com a do C# sem mexer no
// serviço.
var exemplos = map[string]tela.Situacao{
	"nao-instalado": {Estado: tela.NaoInstalado, Titulo: "O Finanças Local não está instalado neste computador.",
		Detalhe: "Instale pelo arquivo FinancasLocal.msi."},
	"desligado": {Estado: tela.Desligado, Titulo: "O site está desligado.", Detalhe: "Clique em “Ligar” para usar."},
	"ligando":   {Estado: tela.Mudando, Titulo: "O site está ligando…", Detalhe: "Aguarde alguns segundos."},
	"no-ar":     {Estado: tela.NoAr, Titulo: "O site está no ar.", Detalhe: "Versão " + versao + " · https://127.0.0.1:5443/", VersaoSite: versao},
	"sem-resposta": {Estado: tela.SemResposta, Titulo: "O site está ligado, mas não responde.",
		Detalhe: "Acabou de ligar? Espere alguns segundos. Se continuar assim, clique em “Reiniciar”."},
}

func main() {
	// Como o Program.cs: --servico ligar|desligar|reiniciar e --backup "destino" são as ações que o próprio
	// gerenciador roda como administrador (UAC), sem janela.
	if len(os.Args) == 3 && os.Args[1] == "--servico" && servico.AcaoValida(os.Args[2]) {
		if err := servico.Executar(os.Args[2]); err != nil {
			log.Print(err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "--backup" {
		os.Exit(backup(os.Args[2]))
	}

	// --bandeja: começando com o Windows, só o ícone na área de notificação.
	bandeja := flag.Bool("bandeja", false, "começa sem janela, só na área de notificação (início com o Windows)")
	mostrar := flag.String("mostrar", "", "mostra uma situação de exemplo, sem consultar o serviço (desligado, no-ar, ligando, sem-resposta, nao-instalado)")
	maisOpcoes := flag.Bool("mais-opcoes", false, "abre com \"Mais opções\" aberto")
	flag.Parse()

	g := novoGerenciador(instalacao.Atual())
	g.maisOpcoes = *maisOpcoes
	if *mostrar != "" {
		s, ok := exemplos[*mostrar]
		if !ok {
			log.Fatalf("-mostrar=%s: não conheço essa situação", *mostrar)
		}
		g.situacao, g.exemplo = s, true
	}
	g.iniciar(!*bandeja)
}

// backup roda elevado: 0 = feito, 2 = sem o banco, 1 = outro erro (os códigos do AcoesElevadas.Backup).
func backup(destino string) int {
	err := copia.Salvar(instalacao.Atual().CaminhoBanco, destino)
	switch {
	case err == nil:
		return 0
	case errors.Is(err, copia.ErrSemBanco):
		return 2
	default:
		log.Print(err)
		return 1
	}
}
