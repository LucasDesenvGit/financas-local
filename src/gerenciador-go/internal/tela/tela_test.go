package tela

import (
	"regexp"
	"strings"
	"testing"
)

// Regra: os botões da situação seguem o Exibir do FormPrincipal.cs, estado por estado.
func TestBotoesPorEstado(t *testing.T) {
	t.Parallel()
	casos := []struct {
		estado                   Estado
		ponto                    string
		abrir, abrirPrimario     bool
		ligar, ligarPrimario     bool
		acao                     string
		reiniciar, paginasDoSite bool
	}{
		{NaoInstalado, "neutro", false, true, false, false, "desligar", false, false},
		{Desligado, "negativo", false, false, true, true, "ligar", false, false},
		{Mudando, "alerta", false, true, false, false, "desligar", false, false},
		{NoAr, "positivo", true, true, true, false, "desligar", true, true},
		{SemResposta, "alerta", true, true, true, false, "desligar", true, false},
	}
	for _, c := range casos {
		tl := Tela{Situacao: Situacao{Estado: c.estado}}
		obtido := []any{tl.Ponto(), tl.AbrirHabilitado(), tl.AbrirPrimario(), tl.LigarHabilitado(), tl.LigarPrimario(),
			tl.LigarAcao(), tl.ReiniciarHabilitado(), tl.PaginasDoSiteHabilitadas()}
		esperado := []any{c.ponto, c.abrir, c.abrirPrimario, c.ligar, c.ligarPrimario, c.acao, c.reiniciar, c.paginasDoSite}
		for i := range obtido {
			if obtido[i] != esperado[i] {
				t.Errorf("estado %d: %v, esperado %v", c.estado, obtido, esperado)
				break
			}
		}
	}
}

// Regra: com uma ação em andamento, ligar e reiniciar ficam bloqueados; abrir o site, não.
func TestOcupadoBloqueiaLigarEReiniciar(t *testing.T) {
	t.Parallel()
	tl := Tela{Situacao: Situacao{Estado: NoAr}, Ocupado: true}
	if tl.LigarHabilitado() || tl.ReiniciarHabilitado() || !tl.AbrirHabilitado() {
		t.Errorf("ligar %v, reiniciar %v, abrir %v", tl.LigarHabilitado(), tl.ReiniciarHabilitado(), tl.AbrirHabilitado())
	}
}

// Regra: a situação curta do menu da bandeja (#289).
func TestCurto(t *testing.T) {
	t.Parallel()
	casos := map[Situacao]string{
		{Estado: NoAr, Titulo: "O site está no ar."}:                                            "No ar",
		{Estado: Mudando, Titulo: "O site está ligando…"}:                                       "Ligando…",
		{Estado: SemResposta, Titulo: "O site está ligado, mas não responde."}:                  "Ligado, mas não responde",
		{Estado: NaoInstalado, Titulo: "O Finanças Local não está instalado neste computador."}: "Não instalado",
		{Estado: SemResposta, Titulo: "Não deu para ver a situação do site."}:                   "Não deu para ver a situação do site",
		{Estado: Mudando, Titulo: "O site está "}:                                               "O site está ",
		Verificando: "Verificando…",
	}
	for s, esperado := range casos {
		if obtido := s.Curto(); obtido != esperado {
			t.Errorf("%q: %q, esperado %q", s.Titulo, obtido, esperado)
		}
	}
}

// Regra: o texto do cartão "Versão" (MostrarVersao do C#).
func TestTextoDaVersao(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome, instalada       string
		peloMsi, siteInformou bool
		esperado              string
	}{
		{"pelo .msi, site fora do ar", "2.42.21", true, false,
			"Versão instalada: 2.42.21. Versão nova: clique em “Procurar versão nova”."},
		{"de desenvolvimento", "2.42.21-dev.3", true, false,
			"Versão de desenvolvimento instalada: 2.42.21-dev.3. Versão nova: clique em “Procurar versão nova”."},
		{"site no ar: a versão vai na situação", "2.42.21", true, true, ""},
		{"pelos scripts", "2.42.21", false, true, "Instalado pelos scripts: para atualizar o site, rode de novo o " +
			"instalar-windows.cmd (ou o scripts/geral/3-publicar-atualizar.sh) na pasta do projeto. Ou passe para o " +
			"instalador: o .msi traz o banco e as chaves, e daí em diante a versão nova chega por aqui."},
	}
	for _, c := range casos {
		if obtido := TextoDaVersao(c.instalada, c.peloMsi, c.siteInformou); obtido != c.esperado {
			t.Errorf("%s: %q", c.nome, obtido)
		}
	}
}

func escrever(t *testing.T, tl Tela) string {
	t.Helper()
	var b strings.Builder
	if err := tl.Escrever(&b); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// botao devolve a tag de abertura do botão com o nome dado (o texto depois do ícone).
func botao(t *testing.T, html, nome string) string {
	t.Helper()
	m := regexp.MustCompile(`<button[^>]*>(?:<span class="ico"[^>]*>[^<]*</span>)?(?:<span id="[^"]*">)?` + regexp.QuoteMeta(nome) + `(?:</span>)?</button>`).
		FindString(html)
	if m == "" {
		t.Fatalf("sem o botão %q", nome)
	}
	return m[:strings.Index(m, ">")+1]
}

// Regra: com o site desligado, a página mostra os mesmos bloqueios e destaques do C#, e a dica dos botões das
// páginas do site (WCAG 1.4.13: a dica é texto do próprio botão, por aria-describedby).
func TestPaginaComOSiteDesligado(t *testing.T) {
	t.Parallel()
	html := escrever(t, Tela{Situacao: Situacao{Estado: Desligado, Titulo: "O site está desligado.", Detalhe: "Clique em “Ligar” para usar."}, PeloMsi: true})
	for nome, deve := range map[string][]string{
		"Abrir o site":         {"disabled"},
		"Ligar":                {`class="botao primario"`, `data-acao="ligar"`, `accesskey="l"`},
		"Reiniciar":            {"disabled"},
		"Ver os erros do site": {"disabled", `aria-describedby="dica-erros"`},
		"Abrir o manual":       {"disabled", `aria-describedby="dica-manual"`},
	} {
		tag := botao(t, html, nome)
		for _, d := range deve {
			if !strings.Contains(tag, d) {
				t.Errorf("%s sem %s: %s", nome, d, tag)
			}
		}
	}
	if strings.Count(html, DicaSiteDesligado) != 2 {
		t.Error("a dica deve aparecer nos dois botões das páginas do site")
	}
	if !strings.Contains(html, `class="ponto negativo"`) || !strings.Contains(html, "O site está desligado.") {
		t.Error("situação desligada não aparece")
	}
}

// Regra: no ar, tudo liberado, "Abrir o site" em destaque, "Desligar" no lugar de "Ligar", e sem dica.
func TestPaginaComOSiteNoAr(t *testing.T) {
	t.Parallel()
	html := escrever(t, Tela{Situacao: Situacao{Estado: NoAr, Titulo: "O site está no ar.", Detalhe: "Versão 2.42.21 · https://127.0.0.1:5443/"}})
	for _, nome := range []string{"Abrir o site", "Desligar", "Reiniciar", "Ver os erros do site", "Abrir o manual"} {
		if tag := botao(t, html, nome); strings.Contains(tag, "disabled") {
			t.Errorf("%s bloqueado: %s", nome, tag)
		}
	}
	if tag := botao(t, html, "Abrir o site"); !strings.Contains(tag, "primario") {
		t.Errorf("Abrir o site sem destaque: %s", tag)
	}
	if strings.Contains(html, ">Ligar<") || strings.Contains(html, `aria-describedby="`) ||
		strings.Count(html, `role="tooltip" hidden>`+DicaSiteDesligado) != 2 {
		t.Error("dica à mostra ou Ligar com o site no ar")
	}
}

// Regra: pelo .msi, "Procurar versão nova" e a preferência de procurar ao abrir; pelos scripts, "Baixar o instalador
// .msi" e nada de procurar.
func TestPaginaPeloMsiEPelosScripts(t *testing.T) {
	t.Parallel()
	msi := escrever(t, Tela{PeloMsi: true, ProcurarAoAbrir: true})
	if !strings.Contains(msi, "Procurar versão nova</button>") || !strings.Contains(msi, "Procurar versão nova ao abrir") ||
		strings.Contains(msi, "Baixar o instalador") {
		t.Error("pelo .msi: botões errados")
	}
	scripts := escrever(t, Tela{})
	if strings.Contains(scripts, "Procurar versão nova") || !strings.Contains(scripts, "Baixar o instalador .msi</button>") {
		t.Error("pelos scripts: botões errados")
	}
}

// Regra: "Mais opções" é um Disclosure (WAI-ARIA): fechado, o painel fica escondido e o botão diz o que tem dentro.
func TestMaisOpcoes(t *testing.T) {
	t.Parallel()
	fechado := escrever(t, Tela{})
	if !strings.Contains(fechado, `aria-expanded="false"`) || !strings.Contains(fechado, `id="painel-mais" hidden`) ||
		!strings.Contains(fechado, `aria-label="Mais opções: cópia do banco, erros, manual e preferências"`) {
		t.Error("fechado: atributos errados")
	}
	aberto := escrever(t, Tela{MaisOpcoes: true})
	if !strings.Contains(aberto, `aria-expanded="true"`) || strings.Contains(aberto, `id="painel-mais" hidden`) ||
		!strings.Contains(aberto, "Menos opções ▴") {
		t.Error("aberto: atributos errados")
	}
}

// Regra: o tema e as preferências aparecem como estão; o rodapé mostra a pasta de dados e o endereço, escapados.
func TestTemaPreferenciasERodape(t *testing.T) {
	t.Parallel()
	html := escrever(t, Tela{Escuro: true, IniciarComSistema: true, PastaDados: `C:\Dados <teste>`, Endereco: "https://127.0.0.1:5443/"})
	if !strings.Contains(html, `data-tema="escuro"`) {
		t.Error("sem o tema escuro")
	}
	if !strings.Contains(html, `data-preferencia="iniciar" accesskey="i" checked`) {
		t.Error("iniciar com o Windows desmarcado")
	}
	if !strings.Contains(html, `Dados em C:\Dados &lt;teste&gt;. Endereço: https://127.0.0.1:5443/`) {
		t.Error("rodapé errado ou sem escapar")
	}
	if strings.Contains(escrever(t, Tela{}), `data-tema="escuro"`) {
		t.Error("tema escuro sem pedir")
	}
}

// Regra: todo botão tem o nome de hoje, sem o "&" das teclas de atalho do WinForms, e o ícone fica fora do nome
// lido pelo leitor de tela (aria-hidden).
func TestNomesDosBotoes(t *testing.T) {
	t.Parallel()
	html := escrever(t, Tela{Situacao: Situacao{Estado: NoAr}, PeloMsi: true})
	if strings.Contains(html, "&amp;") {
		t.Error("sobrou um & de atalho")
	}
	icones := regexp.MustCompile(`<span class="ico"[^>]*>`).FindAllString(html, -1)
	// Abrir, Desligar, Reiniciar, Procurar, Salvar uma cópia, Ver os erros e Abrir o manual.
	if len(icones) != 7 {
		t.Errorf("%d ícones", len(icones))
	}
	for _, i := range icones {
		if !strings.Contains(i, `aria-hidden="true"`) {
			t.Errorf("ícone lido pelo leitor de tela: %s", i)
		}
	}
}

// Regra: ícone sem glifo é erro ao montar a página, e não um botão sem ícone.
func TestIconeSemGlifo(t *testing.T) {
	t.Parallel()
	if _, err := icone("nenhum"); err == nil || err.Error() != "ícone sem glifo: nenhum" {
		t.Errorf("erro %v", err)
	}
}

// Regra: a atualização da janela aberta (GET /estado) segue as mesmas regras da página inteira.
func TestAtualizacao(t *testing.T) {
	t.Parallel()
	desligado := Tela{Situacao: Situacao{Estado: Desligado, Titulo: "O site está desligado.", Detalhe: "Clique"}, Versao: "v"}.Atualizacao()
	if desligado != (Atualizacao{Ponto: "negativo", Titulo: "O site está desligado.", Detalhe: "Clique", AbrirPrimario: false,
		LigarAcao: "ligar", LigarTexto: "Ligar", LigarTecla: "l", LigarHabilitado: true, LigarPrimario: true, Versao: "v"}) {
		t.Errorf("desligado: %+v", desligado)
	}
	noAr := Tela{Situacao: Situacao{Estado: NoAr}}.Atualizacao()
	if noAr != (Atualizacao{Ponto: "positivo", AbrirHabilitado: true, AbrirPrimario: true, LigarAcao: "desligar",
		LigarTexto: "Desligar", LigarTecla: "d", LigarHabilitado: true, ReiniciarHabilitado: true, PaginasDoSite: true}) {
		t.Errorf("no ar: %+v", noAr)
	}
}

// Regra: detalhe e versão vazios ficam escondidos (os elementos existem, para a atualização preencher).
func TestVaziosEscondidos(t *testing.T) {
	t.Parallel()
	html := escrever(t, Tela{Situacao: Verificando})
	if !strings.Contains(html, `id="detalhe" hidden>`) || !strings.Contains(html, `id="versao" hidden>`) {
		t.Error("vazios à mostra")
	}
	cheio := escrever(t, Tela{Situacao: Situacao{Detalhe: "d"}, Versao: "v"})
	if !strings.Contains(cheio, `id="detalhe">d<`) || !strings.Contains(cheio, `id="versao">v<`) {
		t.Error("cheios escondidos")
	}
}
