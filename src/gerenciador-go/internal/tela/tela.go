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
	// Develop é o "Atualizar para a última do develop" da instalação de desenvolvimento (ADR 0040).
	Develop Develop

	// Linux: a janela no Linux (ADR 0033), com o que muda lá: ícones em SVG (a fonte de ícones é do Windows), "Iniciar
	// com o sistema", o registro do serviço e o "sem login"; nada de .msi.
	Linux bool
	// Comando é o comando para atualizar pelos scripts no Linux, para copiar ("" sem a pasta do projeto).
	Comando string
	// SemLogin, no Linux: o site sobe sem a sessão aberta ("sim"), não sobe ("nao") ou não dá para saber ("").
	SemLogin string
	// ErroSemLogin troca o texto do "sem login" quando ligar não deu certo.
	ErroSemLogin string
}

// NaoDeuCerto é o detalhe quando ligar, desligar ou reiniciar falha: o motivo, quando o sistema diz (no Linux, o do
// systemctl), e onde ver mais.
func NaoDeuCerto(motivo, ondeVer string) string {
	if motivo == "" {
		return "Não deu certo. " + ondeVer
	}
	return "Não deu certo: " + motivo + " " + ondeVer
}

// TextoPelosScriptsNoLinux é o cartão "Versão" da instalação pelos scripts no Linux (MostrarComoAtualizar do C#): com
// a pasta do projeto, o comando vem logo abaixo, para copiar (Nielsen 6: reconhecer em vez de lembrar).
func TextoPelosScriptsNoLinux(comComando bool) string {
	if comComando {
		return "Instalado pelos scripts: a versão nova vem do código. Para atualizar o site e o gerenciador, rode no " +
			"terminal (o site fica fora do ar por alguns segundos e o banco ganha uma cópia antes):"
	}
	return "Instalado pelos scripts: para atualizar, rode git pull, scripts/geral/3-publicar-atualizar.sh e " +
		"scripts/linux/instalar-gerenciador.sh na pasta do projeto."
}

// ComandoParaAtualizar é o comando que atualiza o site e o gerenciador pelos scripts, na pasta do projeto.
func ComandoParaAtualizar(raiz string) string {
	return "cd " + aspasDoShell(raiz) + " && git pull && scripts/geral/3-publicar-atualizar.sh && scripts/linux/instalar-gerenciador.sh"
}

// aspasDoShell põe o texto entre aspas simples para o bash (a aspa simples dentro vira '\”).
func aspasDoShell(valor string) string { return "'" + strings.ReplaceAll(valor, "'", `'\''`) + "'" }

// TextoSemLogin explica se o site fica no ar sem a sessão aberta (o "linger" do systemd).
func (t Tela) TextoSemLogin() string {
	switch {
	case t.ErroSemLogin != "":
		return "Não deu certo: " + t.ErroSemLogin + " No terminal: loginctl enable-linger"
	case t.SemLogin == "sim":
		return "O site sobe junto com o computador, mesmo antes de você entrar na sua conta."
	case t.SemLogin == "nao":
		return "Hoje o site só fica no ar enquanto você está com a sessão aberta: ao sair da conta, ele desliga."
	default:
		return ""
	}
}

// RotuloMais é o nome do "Mais opções" fechado, que diz o que tem dentro.
func (t Tela) RotuloMais() string {
	if t.Linux {
		return "Mais opções: cópia do banco, erros, registro, manual e preferências"
	}
	return "Mais opções: cópia do banco, erros, manual e preferências"
}

// Develop: Disponivel quando a pasta do projeto é um clone na branch develop; Saida é o que os scripts escreveram.
type Develop struct {
	Disponivel bool
	Rodando    bool
	Saida      string
	// Notas: depois de trazer o develop, há o "O que mudou" desde a versão de antes (#432).
	Notas bool
}

// TextoDeDesenvolvimento é o cartão "Versão" da instalação de desenvolvimento (MostrarVersao do C#).
func TextoDeDesenvolvimento(raiz string) string {
	return "Instalação de desenvolvimento: a pasta " + raiz + " é um clone na branch develop. O botão abaixo traz o " +
		"develop do GitHub e reinstala (só para quem desenvolve; o site fica fora do ar alguns minutos e o banco ganha " +
		"uma cópia antes)."
}

// ConfirmarDevelop é a pergunta antes de atualizar pelo develop, com o texto do AtualizarPeloGitAsync do C#. No Linux,
// nada pede administrador.
func ConfirmarDevelop(linux bool) Aviso {
	administrador := " (pede permissão de administrador)"
	if linux {
		administrador = ""
	}
	return Aviso{Tipo: "pergunta", Titulo: "Atualizar esta instalação de desenvolvimento para a última versão do develop?",
		Mensagem: "O gerenciador traz o código do GitHub e reinstala o site" + administrador + ". O site fica " +
			"fora do ar por alguns minutos, enquanto o front-end e a API são compilados, e o banco ganha uma cópia em backups " +
			"antes de mudar. Com alterações locais ou em outra branch, nada é mudado: a tela diz o motivo."}
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
	Develop             bool   `json:"develop"`
	DevelopRodando      bool   `json:"developRodando"`
	Saida               string `json:"saida"`
	NotasDevelop        bool   `json:"notasDevelop"`
	SemLogin            string `json:"semLogin"`
	TextoSemLogin       string `json:"textoSemLogin"`
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
		Develop: t.Develop.Disponivel, DevelopRodando: t.Develop.Rodando, Saida: t.Develop.Saida,
		NotasDevelop: t.Develop.Notas, SemLogin: t.SemLogin, TextoSemLogin: t.TextoSemLogin(),
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
	// TextoPagina troca o "Ver a página da versão no GitHub" (o do develop abre o CHANGELOG).
	TextoPagina string `json:"textoPagina,omitempty"`
}

// ConfirmarAtualizacao é a pergunta antes de baixar, com o texto do AtualizarAsync do C#.
func ConfirmarAtualizacao(versao string) Aviso {
	return Aviso{Tipo: "pergunta", Titulo: "Atualizar o Finanças Local para a versão " + versao + "?",
		Mensagem: "O instalador abre em seguida e o site fica fora do ar por alguns segundos. Antes de mudar o banco, " +
			"a versão nova guarda uma cópia dele em dados\\backups. Seus dados continuam onde estão."}
}

// Os glifos dos botões do C# (IconeBotao, Controles.cs), da fonte de ícones do Windows (Segoe Fluent Icons).
var glifos = map[string]string{
	"abrir": "\uE8A7", "ligar": "\uE7E8", "reiniciar": "\uE72C", "baixar": "\uE896", "procurar": "\uE721",
	"informacao": "\uE946", "cancelar": "\uE711", "pasta": "\uE838", "erros": "\uE7BA", "manual": "\uE897", "salvar": "\uE74E",
	"sucesso": "\uE930", "alerta": "\uE7BA", "copiar": "\uE8C8", "registro": "\uE8A5",
}

// No Linux, que não tem a fonte de ícones do Windows, os mesmos ícones vêm do lucide (os do site, @lucide/angular
// 1.61.0, licença ISC): external-link, power, rotate-cw, download, search, info, x, folder-open, triangle-alert,
// circle-question-mark, save, circle-check, copy e scroll-text.
var desenhos = map[string]string{
	"abrir":      `<path d="M15 3h6v6"/><path d="M10 14 21 3"/><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/>`,
	"ligar":      `<path d="M12 2v10"/><path d="M18.4 6.6a9 9 0 1 1-12.77.04"/>`,
	"reiniciar":  `<path d="M21 12a9 9 0 1 1-9-9c2.52 0 4.93 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/>`,
	"baixar":     `<path d="M12 15V3"/><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><path d="m7 10 5 5 5-5"/>`,
	"procurar":   `<path d="m21 21-4.34-4.34"/><circle cx="11" cy="11" r="8"/>`,
	"informacao": `<circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/>`,
	"cancelar":   `<path d="M18 6 6 18"/><path d="m6 6 12 12"/>`,
	"pasta":      `<path d="m6 14 1.5-2.9A2 2 0 0 1 9.24 10H20a2 2 0 0 1 1.94 2.5l-1.54 6a2 2 0 0 1-1.95 1.5H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h3.9a2 2 0 0 1 1.69.9l.81 1.2a2 2 0 0 0 1.67.9H18a2 2 0 0 1 2 2v2"/>`,
	"erros":      `<path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3"/><path d="M12 9v4"/><path d="M12 17h.01"/>`,
	"manual":     `<circle cx="12" cy="12" r="10"/><path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3"/><path d="M12 17h.01"/>`,
	"salvar":     `<path d="M15.2 3a2 2 0 0 1 1.4.6l3.8 3.8a2 2 0 0 1 .6 1.4V19a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2z"/><path d="M17 21v-7a1 1 0 0 0-1-1H8a1 1 0 0 0-1 1v7"/><path d="M7 3v4a1 1 0 0 0 1 1h7"/>`,
	"sucesso":    `<circle cx="12" cy="12" r="10"/><path d="m16 9-5.5 5.5L8 12"/>`,
	"alerta":     `<path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3"/><path d="M12 9v4"/><path d="M12 17h.01"/>`,
	"copiar":     `<rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/>`,
	"registro":   `<path d="M15 12h-5"/><path d="M15 8h-5"/><path d="M19 17V5a2 2 0 0 0-2-2H4"/><path d="M8 21h12a2 2 0 0 0 2-2v-1a1 1 0 0 0-1-1H11a1 1 0 0 0-1 1v1a2 2 0 1 1-4 0V5a2 2 0 1 0-4 0v2a1 1 0 0 0 1 1h3"/>`,
}

//go:embed estilo.css
var Estilo []byte

//go:embed pagina.html
var textoDaPagina string

var pagina = template.Must(template.New("pagina").Funcs(template.FuncMap{
	"dica": func() string { return DicaSiteDesligado },
}).Parse(textoDaPagina))

// Glifo é o desenho do ícone: o caractere da fonte de ícones do Windows ou, no Linux, o SVG do lucide, na cor do
// texto.
func (t Tela) Glifo(nome string) (template.HTML, error) {
	if t.Linux {
		d, ok := desenhos[nome]
		if !ok {
			return "", errors.New("ícone sem glifo: " + nome)
		}
		return template.HTML(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" ` +
			`stroke-linecap="round" stroke-linejoin="round">` + d + `</svg>`), nil
	}
	g, ok := glifos[nome]
	if !ok {
		return "", errors.New("ícone sem glifo: " + nome)
	}
	return template.HTML(g), nil
}

// Icone é o glifo do botão, fora do nome lido pelo leitor de tela (aria-hidden).
func (t Tela) Icone(nome string) (template.HTML, error) {
	g, err := t.Glifo(nome)
	if err != nil {
		return "", err
	}
	return `<span class="ico" aria-hidden="true">` + g + `</span>`, nil
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
