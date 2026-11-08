// O gerenciador do site em Go (ADR 0052): o ícone na bandeja e a janela própria com o Wails v3, igual à do
// gerenciador em C# (src/gerenciador/Financas.Gerenciador). A página é servida por dentro do programa, sem porta
// aberta.
//
// No Linux (#417), o serviço é do systemd do próprio usuário: ligar, desligar e copiar o banco rodam no próprio
// gerenciador, sem pedir senha (sistema_linux.go).
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"

	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/copia"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/desenvolvimento"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/instalacao"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/restauracao"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/servico"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/situacao"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/tela"
)

// versao vem do Directory.Build.props na compilação (-ldflags "-X main.versao=...").
var versao = "0.0.0-dev.0"

// Situações de exemplo do -mostrar, com os textos do Situacao.cs, para comparar a janela com a do C# sem mexer no
// serviço.
var exemplos = map[string]tela.Situacao{
	"nao-instalado": {Estado: tela.NaoInstalado, Titulo: "O Finanças Local não está instalado neste computador.",
		Detalhe: servico.ComoInstalar},
	"desligado": {Estado: tela.Desligado, Titulo: "O site está desligado.", Detalhe: "Clique em “Ligar” para usar."},
	"ligando":   {Estado: tela.Mudando, Titulo: "O site está ligando…", Detalhe: "Aguarde alguns segundos."},
	"no-ar":     {Estado: tela.NoAr, Titulo: "O site está no ar.", Detalhe: "Versão " + versao + " · https://127.0.0.1:5443/", VersaoSite: versao},
	"sem-resposta": {Estado: tela.SemResposta, Titulo: "O site está ligado, mas não responde.",
		Detalhe: "Acabou de ligar? Espere alguns segundos. Se continuar assim, clique em “Reiniciar”."},
}

func main() {
	// Como o Program.cs: --servico ligar|desligar|reiniciar, --backup "destino", --restaurar "cópia" (#533) e
	// --instalar-pelos-scripts (só na
	// instalação de desenvolvimento, ADR 0040) são as ações que o próprio gerenciador roda como administrador (UAC),
	// sem janela.
	if len(os.Args) == 3 && os.Args[1] == "--servico" && servico.AcaoValida(os.Args[2]) {
		if err := servico.Executar(os.Args[2]); err != nil {
			log.Print(err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) == 2 && os.Args[1] == "--instalar-pelos-scripts" {
		os.Exit(desenvolvimento.ReinstalarElevado())
	}
	if len(os.Args) == 3 && os.Args[1] == "--backup" {
		os.Exit(backup(os.Args[2]))
	}
	if len(os.Args) == 3 && os.Args[1] == "--restaurar" {
		os.Exit(restaurar(os.Args[2]))
	}
	// --situacao: como o do Program.cs do Linux, escreve a situação e sai (0 = no ar), sem janela; a CI usa (#277).
	if len(os.Args) == 2 && os.Args[1] == "--situacao" {
		i := instalacao.Atual()
		os.Exit(situacao.Resumo(os.Stdout, situacao.Nova(i).Consultar(context.Background()), i, versao))
	}

	// --bandeja: começando com o Windows, só o ícone na área de notificação.
	bandeja := flag.Bool("bandeja", false, "começa sem janela, só na área de notificação (início com o Windows)")
	mostrar := flag.String("mostrar", "", "mostra uma situação de exemplo, sem consultar o serviço (desligado, no-ar, ligando, sem-resposta, nao-instalado)")
	maisOpcoes := flag.Bool("mais-opcoes", false, "abre com \"Mais opções\" aberto")
	flag.Parse()

	permitirPrimeiroPlano()
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

// restaurar roda elevado (#533): 0 = feito, 1 = não deu (o motivo vai para o log; a janela diz o que fazer).
func restaurar(copiaEscolhida string) int {
	if _, err := restauracao.Restaurar(instalacao.Atual().CaminhoBanco, copiaEscolhida, servicoDoSite{}); err != nil {
		log.Print(err)
		return 1
	}
	return 0
}

// servicoDoSite liga e desliga o serviço do site para a restauração (no Windows, quem chama já está elevado).
type servicoDoSite struct{}

func (servicoDoSite) Desligar() error { return servico.Executar("desligar") }
func (servicoDoSite) Ligar() error    { return servico.Executar("ligar") }
