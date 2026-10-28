// Package tela monta a janela do gerenciador em HTML, igual à do FormPrincipal.cs (ADR 0052): os mesmos cartões,
// textos, ícones e bloqueios. Não depende do Wails, para dar para testar sem janela.
package tela

import (
	_ "embed"
	"errors"
	"html/template"
	"io"
	"path/filepath"
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
	// VersaoSite é a que o site no ar informa ("" quando ele não responde ou não informa).
	VersaoSite string
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
	// ReceberDesenvolvimento: "Procurar versão nova" olha o pré-lançamento "desenvolvimento" (ADR 0040).
	ReceberDesenvolvimento bool
	Procura                Procura
}

// Procura é o estado da versão nova no cartão "Versão" (o ProcurarAsync e o AtualizarAsync do C#).
type Procura struct {
	Procurando bool
	// Nova é a versão nova achada ("" sem versão nova): aparecem "Atualizar para …" e "O que mudou".
	Nova      string
	Baixando  bool
	Progresso int // de 0 a 100
}

// ProcurarHabilitado: nem procurando nem baixando.
func (t Tela) ProcurarHabilitado() bool { return !t.Procura.Procurando && !t.Procura.Baixando }

// AtualizarTexto é o "Atualizar para X" do botão em destaque.
func (t Tela) AtualizarTexto() string { return "Atualizar para " + t.Procura.Nova }

// ErroDaProcura: sem internet, a procura falha, e o site continua funcionando.
const ErroDaProcura = "Não deu para procurar versão nova agora (sem internet?). O site continua funcionando."

// ResultadoDaProcura é a frase depois de procurar, como no C#.
func ResultadoDaProcura(nova, desenvolvimento bool, versao string) string {
	switch {
	case nova && desenvolvimento:
		return "Tem versão de desenvolvimento nova: " + versao + "."
	case nova:
		return "Tem versão nova: " + versao + "."
	case desenvolvimento:
		return "Você está na última versão de desenvolvimento."
	default:
		return "Você está na versão mais nova."
	}
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

// LigarTexto e LigarTecla: "Ligar" (Alt+L) ou "Desligar" (Alt+D), como o "&Ligar"/"&Desligar" do C#.
func (t Tela) LigarTexto() string {
	if t.desligado() {
		return "Ligar"
	}
	return "Desligar"
}

func (t Tela) LigarTecla() string { return strings.ToLower(t.LigarTexto()[:1]) }

// PaginasDoSiteHabilitadas: erros e manual são páginas do site, e só abrem com ele no ar.
func (t Tela) PaginasDoSiteHabilitadas() bool { return t.Situacao.Estado == NoAr }

// Atualizacao é o que muda na janela aberta a cada consulta da situação (GET /estado): a página aplica nos
// elementos que já existem, sem recarregar, para não perder o foco nem o "Mais opções".
type Atualizacao struct {
	Ponto               string `json:"ponto"`
	Titulo              string `json:"titulo"`
	Detalhe             string `json:"detalhe"`
	AbrirHabilitado     bool   `json:"abrirHabilitado"`
	AbrirPrimario       bool   `json:"abrirPrimario"`
	LigarAcao           string `json:"ligarAcao"`
	LigarTexto          string `json:"ligarTexto"`
	LigarTecla          string `json:"ligarTecla"`
	LigarHabilitado     bool   `json:"ligarHabilitado"`
	LigarPrimario       bool   `json:"ligarPrimario"`
	ReiniciarHabilitado bool   `json:"reiniciarHabilitado"`
	PaginasDoSite       bool   `json:"paginasDoSite"`
	Versao              string `json:"versao"`
	ProcurarHabilitado  bool   `json:"procurarHabilitado"`
	Nova                string `json:"nova"`
	AtualizarTexto      string `json:"atualizarTexto"`
	Baixando            bool   `json:"baixando"`
	Progresso           int    `json:"progresso"`
}

// Atualizacao da tela, com as mesmas regras da página inteira.
func (t Tela) Atualizacao() Atualizacao {
	return Atualizacao{
		Ponto: t.Ponto(), Titulo: t.Situacao.Titulo, Detalhe: t.Situacao.Detalhe,
		AbrirHabilitado: t.AbrirHabilitado(), AbrirPrimario: t.AbrirPrimario(),
		LigarAcao: t.LigarAcao(), LigarTexto: t.LigarTexto(), LigarTecla: t.LigarTecla(),
		LigarHabilitado: t.LigarHabilitado(), LigarPrimario: t.LigarPrimario(),
		ReiniciarHabilitado: t.ReiniciarHabilitado(), PaginasDoSite: t.PaginasDoSiteHabilitadas(), Versao: t.Versao,
		ProcurarHabilitado: t.ProcurarHabilitado(), Nova: t.Procura.Nova, AtualizarTexto: t.AtualizarTexto(),
		Baixando: t.Procura.Baixando, Progresso: t.Procura.Progresso,
	}
}

// TextoDaVersao é o cartão "Versão" do MostrarVersao do C#. Com o site no ar, a versão já vai no detalhe da situação:
// fica só o resultado da procura. Fora do ar, a versão instalada (o .msi instala site e gerenciador juntos) e o
// resultado da procura, ou o convite para procurar.
func TextoDaVersao(instalada string, peloMsi, siteInformouVersao bool, resultadoDaProcura string) string {
	if !peloMsi {
		return "Instalado pelos scripts: para atualizar o site, rode de novo o instalar-windows.cmd " +
			"(ou o scripts/geral/3-publicar-atualizar.sh) na pasta do projeto. Ou passe para o instalador: o .msi " +
			"traz o banco e as chaves, e daí em diante a versão nova chega por aqui."
	}
	var partes []string
	if !siteInformouVersao {
		de := ""
		if strings.Contains(instalada, "-dev.") {
			de = "de desenvolvimento "
		}
		partes = append(partes, "Versão "+de+"instalada: "+instalada+".")
		if resultadoDaProcura == "" {
			resultadoDaProcura = "Versão nova: clique em “Procurar versão nova”."
		}
	}
	if resultadoDaProcura != "" {
		partes = append(partes, resultadoDaProcura)
	}
	return strings.Join(partes, " ")
}

// Notas é o "O que mudou" da versão nova, já em HTML seguro (atualizacao.HTMLDasNotas).
type Notas struct {
	Titulo    string        `json:"titulo"`
	Novidades template.HTML `json:"novidades"`
	Resto     template.HTML `json:"resto"`
	Pagina    string        `json:"pagina"`
}

// ConfirmarAtualizacao é a pergunta antes de baixar, com o texto do AtualizarAsync do C#.
func ConfirmarAtualizacao(versao string) Aviso {
	return Aviso{Tipo: "pergunta", Titulo: "Atualizar o Finanças Local para a versão " + versao + "?",
		Mensagem: "O instalador abre em seguida e o site fica fora do ar por alguns segundos. Antes de mudar o banco, " +
			"a versão nova guarda uma cópia dele em dados\\backups. Seus dados continuam onde estão."}
}

// Os glifos dos botões do C# (IconeBotao, Controles.cs), da fonte de ícones do Windows (Segoe Fluent Icons).
var glifos = map[string]string{
	"abrir": "", "ligar": "", "reiniciar": "", "baixar": "", "procurar": "",
	"informacao": "", "cancelar": "", "pasta": "", "erros": "", "manual": "", "salvar": "",
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

// Aviso é o diálogo que a janela mostra no desenho dela (e não a caixa do Windows): o resultado da cópia do banco.
type Aviso struct {
	// Tipo: "sucesso" ou "alerta" (o ícone e a cor).
	Tipo     string `json:"tipo"`
	Titulo   string `json:"titulo"`
	Mensagem string `json:"mensagem"`
	// Arquivo e Pasta só na cópia salva, em linhas separadas, para o caminho comprido não quebrar no meio da frase; com
	// eles aparece "Abrir a pasta".
	Arquivo string `json:"arquivo,omitempty"`
	Pasta   string `json:"pasta,omitempty"`
}

// AvisoDaCopia é o resultado de "Salvar uma cópia do banco…", com os textos do SalvarCopiaAsync do C#.
func AvisoDaCopia(feito, cancelado bool, destino string) Aviso {
	switch {
	case feito:
		return Aviso{Tipo: "sucesso", Titulo: "Cópia do banco salva",
			Mensagem: "Ela tem todos os seus dados financeiros: guarde em lugar seguro.",
			Arquivo:  filepath.Base(destino), Pasta: filepath.Dir(destino)}
	case cancelado:
		return Aviso{Tipo: "alerta", Titulo: "Cópia do banco", Mensagem: "Cancelado: nenhuma cópia foi feita."}
	default:
		return Aviso{Tipo: "alerta", Titulo: "Cópia do banco",
			Mensagem: "Não deu para copiar o banco. Confira se a pasta escolhida aceita gravação."}
	}
}

// Escrever escreve a página da janela.
func (t Tela) Escrever(w io.Writer) error { return pagina.Execute(w, t) }
