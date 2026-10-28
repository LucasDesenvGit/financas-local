// Package tela monta a janela do gerenciador em HTML, igual à do FormPrincipal.cs (ADR 0052): os mesmos cartões,
// textos, ícones e bloqueios. Não depende do Wails, para dar para testar sem janela.
package tela

import (
	_ "embed"
	"errors"
	"html/template"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Estado é o EstadoSite do Situacao.cs, na mesma ordem.
type Estado int

const (
	NaoInstalado Estado = iota
	Desligado
	Mudando
	NoAr
	SemResposta
)

// Situacao responde "o site está funcionando?" em palavras do dia a dia (Nielsen 2), como o Situacao.cs.
type Situacao struct {
	Estado  Estado
	Titulo  string
	Detalhe string
}

// Verificando é a situação antes da primeira consulta (a do ItemSituacao do menu da bandeja em C#).
var Verificando = Situacao{Estado: Mudando, Titulo: "Verificando…"}

// Curto é a situação em duas ou três palavras, para o menu da bandeja (#289): "O site está no ar." vira "No ar";
// "O site está ligando…" vira "Ligando…".
func (s Situacao) Curto() string {
	if s.Estado == NaoInstalado {
		return "Não instalado"
	}
	t := strings.TrimRight(strings.TrimPrefix(s.Titulo, "O site está "), ".")
	if t == "" {
		return s.Titulo
	}
	r, n := utf8.DecodeRuneInString(t)
	return string(unicode.ToUpper(r)) + t[n:]
}

// DicaSiteDesligado aparece nos botões das páginas do site (erros, manual) quando ele não está no ar.
const DicaSiteDesligado = "É preciso ligar o site para ver isto. Use o botão “Ligar” acima."

// Tela é tudo o que a janela mostra.
type Tela struct {
	Situacao Situacao
	// Ocupado: uma ação (ligar, desligar, reiniciar) está em andamento.
	Ocupado bool
	// Versao é o texto do cartão "Versão" (TextoDaVersao).
	Versao string
	// PeloMsi: instalado pelo .msi; só então aparecem "Procurar versão nova" e a preferência de procurar ao abrir. Pelos
	// scripts, aparece "Baixar o instalador .msi".
	PeloMsi           bool
	MaisOpcoes        bool
	IniciarComSistema bool
	ProcurarAoAbrir   bool
	PastaDados        string
	Endereco          string
	Escuro            bool
}

// Ponto é a classe da cor do ponto da situação (Tema.DaSituacao); a cor nunca é a única pista (WCAG 1.4.1).
func (t Tela) Ponto() string {
	switch t.Situacao.Estado {
	case NoAr:
		return "positivo"
	case Desligado:
		return "negativo"
	case SemResposta, Mudando:
		return "alerta"
	default:
		return "neutro"
	}
}

func (t Tela) instalado() bool { return t.Situacao.Estado != NaoInstalado }
func (t Tela) desligado() bool { return t.Situacao.Estado == Desligado }
func (t Tela) responde() bool {
	return t.Situacao.Estado == NoAr || t.Situacao.Estado == SemResposta
}

// O botão em destaque é a próxima ação provável: abrir o site no ar, ligar o desligado.

func (t Tela) AbrirHabilitado() bool { return t.responde() }
func (t Tela) AbrirPrimario() bool   { return !t.desligado() }
func (t Tela) LigarPrimario() bool   { return t.desligado() }
func (t Tela) LigarHabilitado() bool {
	return t.instalado() && t.Situacao.Estado != Mudando && !t.Ocupado
}
func (t Tela) ReiniciarHabilitado() bool { return t.instalado() && t.responde() && !t.Ocupado }

// LigarAcao é "ligar" com o site desligado e "desligar" nos outros casos, como o _ligar do C#.
func (t Tela) LigarAcao() string {
	if t.desligado() {
		return "ligar"
	}
	return "desligar"
}

// PaginasDoSiteHabilitadas: erros e manual são páginas do site, e só abrem com ele no ar.
func (t Tela) PaginasDoSiteHabilitadas() bool { return t.Situacao.Estado == NoAr }

// TextoDaVersao é o cartão "Versão" do MostrarVersao do C#, sem a procura (#420). Com o site no ar, a versão já vai
// no detalhe da situação, e o texto fica vazio.
func TextoDaVersao(instalada string, peloMsi, siteInformouVersao bool) string {
	if !peloMsi {
		return "Instalado pelos scripts: para atualizar o site, rode de novo o instalar-windows.cmd " +
			"(ou o scripts/geral/3-publicar-atualizar.sh) na pasta do projeto. Ou passe para o instalador: o .msi " +
			"traz o banco e as chaves, e daí em diante a versão nova chega por aqui."
	}
	if siteInformouVersao {
		return ""
	}
	de := ""
	if strings.Contains(instalada, "-dev.") {
		de = "de desenvolvimento "
	}
	return "Versão " + de + "instalada: " + instalada + ". Versão nova: clique em “Procurar versão nova”."
}

// Os glifos dos botões do C# (IconeBotao, Controles.cs), da fonte de ícones do Windows (Segoe Fluent Icons).
var glifos = map[string]string{
	"abrir": "", "ligar": "", "reiniciar": "", "baixar": "", "procurar": "",
	"informacao": "", "erros": "", "manual": "", "salvar": "",
}

//go:embed estilo.css
var Estilo []byte

//go:embed pagina.html
var textoDaPagina string

var pagina = template.Must(template.New("pagina").Funcs(template.FuncMap{
	"icone": icone,
	"dica":  func() string { return DicaSiteDesligado },
}).Parse(textoDaPagina))

// icone é o glifo do botão, fora do nome lido pelo leitor de tela (aria-hidden).
func icone(nome string) (template.HTML, error) {
	g, ok := glifos[nome]
	if !ok {
		return "", errors.New("ícone sem glifo: " + nome)
	}
	return template.HTML(`<span class="ico" aria-hidden="true">` + g + `</span>`), nil
}

// Escrever escreve a página da janela.
func (t Tela) Escrever(w io.Writer) error { return pagina.Execute(w, t) }
