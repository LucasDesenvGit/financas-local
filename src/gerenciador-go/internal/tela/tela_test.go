package tela

import (
	"path/filepath"
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
		procura               string
		esperado              string
	}{
		{"pelo .msi, site fora do ar", "2.42.21", true, false, "",
			"Versão instalada: 2.42.21. Versão nova: clique em “Procurar versão nova”."},
		{"de desenvolvimento", "2.42.21-dev.3", true, false, "",
			"Versão de desenvolvimento instalada: 2.42.21-dev.3. Versão nova: clique em “Procurar versão nova”."},
		{"fora do ar, depois de procurar", "2.42.21", true, false, "Você está na versão mais nova.",
			"Versão instalada: 2.42.21. Você está na versão mais nova."},
		{"site no ar: a versão vai na situação", "2.42.21", true, true, "", ""},
		{"site no ar, depois de procurar: só o resultado", "2.42.21", true, true, "Tem versão nova: 2.43.0.",
			"Tem versão nova: 2.43.0."},
		{"pelos scripts", "2.42.21", false, true, "x", "Instalado pelos scripts: para atualizar o site, rode de novo o " +
			"instalar-windows.cmd (ou o scripts/geral/3-publicar-atualizar.sh) na pasta do projeto. Ou passe para o " +
			"instalador: o .msi traz o banco e as chaves, e daí em diante a versão nova chega por aqui."},
	}
	for _, c := range casos {
		if obtido := TextoDaVersao(c.instalada, c.peloMsi, c.siteInformou, c.procura); obtido != c.esperado {
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
	if strings.Contains(html, ">Ligar<") || strings.Contains(html, `aria-describedby="dica-`) ||
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
	// Abrir, Desligar, Reiniciar; Atualizar, Procurar, O que mudou, Cancelar download; Salvar uma cópia, Ver os erros,
	// Abrir o manual; Abrir a pasta e Atualizar (do aviso); Ver tudo e Ver a página da versão (das notas).
	if len(icones) != 14 {
		t.Errorf("%d ícones", len(icones))
	}
	for _, i := range icones {
		if !strings.Contains(i, `aria-hidden="true"`) {
			t.Errorf("ícone lido pelo leitor de tela: %s", i)
		}
	}
}

// Regra: ícone sem glifo é erro ao montar a página, e não um botão sem ícone, nos dois sistemas.
func TestIconeSemGlifo(t *testing.T) {
	t.Parallel()
	for _, linux := range []bool{false, true} {
		if _, err := (Tela{Linux: linux}).Icone("nenhum"); err == nil || err.Error() != "ícone sem glifo: nenhum" {
			t.Errorf("linux %v: erro %v", linux, err)
		}
	}
}

// Regra: todo ícone tem o glifo do Windows e o desenho do Linux, e nada mais (um nome a mais de um lado só seria um
// botão sem ícone no outro sistema).
func TestIconesNosDoisSistemas(t *testing.T) {
	t.Parallel()
	if len(glifos) != len(desenhos) {
		t.Errorf("%d glifos e %d desenhos", len(glifos), len(desenhos))
	}
	for nome := range glifos {
		if _, ok := desenhos[nome]; !ok {
			t.Errorf("%s sem desenho no Linux", nome)
		}
	}
}

// Regra: no Windows, o glifo é o caractere da fonte de ícones; no Linux, o SVG do lucide na cor do texto, também
// fora do leitor de tela.
func TestGlifoPorSistema(t *testing.T) {
	t.Parallel()
	if g, _ := (Tela{}).Icone("ligar"); g != `<span class="ico" aria-hidden="true">`+"\uE7E8"+`</span>` {
		t.Errorf("windows: %s", g)
	}
	g, _ := (Tela{Linux: true}).Icone("ligar")
	if g != `<span class="ico" aria-hidden="true"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" `+
		`stroke-linecap="round" stroke-linejoin="round"><path d="M12 2v10"/><path d="M18.4 6.6a9 9 0 1 1-12.77.04"/></svg></span>` {
		t.Errorf("linux: %s", g)
	}
}

// Regra: a atualização da janela aberta (GET /estado) segue as mesmas regras da página inteira.
func TestAtualizacao(t *testing.T) {
	t.Parallel()
	desligado := Tela{Situacao: Situacao{Estado: Desligado, Titulo: "O site está desligado.", Detalhe: "Clique"}, Versao: "v"}.Atualizacao()
	if desligado != (Atualizacao{Ponto: "negativo", Titulo: "O site está desligado.", Detalhe: "Clique", AbrirPrimario: false,
		LigarAcao: "ligar", LigarTexto: "Ligar", LigarTecla: "l", LigarHabilitado: true, LigarPrimario: true, Versao: "v",
		ProcurarHabilitado: true, AtualizarTexto: "Atualizar para "}) {
		t.Errorf("desligado: %+v", desligado)
	}
	noAr := Tela{Situacao: Situacao{Estado: NoAr}}.Atualizacao()
	if noAr != (Atualizacao{Ponto: "positivo", AbrirHabilitado: true, AbrirPrimario: true, LigarAcao: "desligar",
		LigarTexto: "Desligar", LigarTecla: "d", LigarHabilitado: true, ReiniciarHabilitado: true, PaginasDoSite: true,
		ProcurarHabilitado: true, AtualizarTexto: "Atualizar para "}) {
		t.Errorf("no ar: %+v", noAr)
	}
}

// Regra: detalhe e versão vazios ficam escondidos (os elementos existem, para a atualização preencher).
func TestVaziosEscondidos(t *testing.T) {
	t.Parallel()
	html := escrever(t, Tela{Situacao: Verificando})
	if !strings.Contains(html, `id="detalhe" hidden>`) || !strings.Contains(html, `id="versao" role="status" aria-live="polite" hidden>`) {
		t.Error("vazios à mostra")
	}
	cheio := escrever(t, Tela{Situacao: Situacao{Detalhe: "d"}, Versao: "v"})
	if !strings.Contains(cheio, `id="detalhe">d<`) || !strings.Contains(cheio, `id="versao" role="status" aria-live="polite">v<`) {
		t.Error("cheios escondidos")
	}
}

// Regra: o aviso da cópia salva traz o arquivo e a pasta à parte (com "Abrir a pasta"); cancelado e erro são alertas,
// com os textos do C#.
func TestAvisoDaCopia(t *testing.T) {
	t.Parallel()
	destino := filepath.Join(t.TempDir(), "FinancasLocal-2026-10-28.db")
	casos := []struct {
		feito, cancelado bool
		quer             Aviso
	}{
		{true, false, Aviso{Tipo: "sucesso", Titulo: "Cópia do banco salva",
			Mensagem: "Ela tem todos os seus dados financeiros: guarde em lugar seguro.",
			Arquivo:  "FinancasLocal-2026-10-28.db", Pasta: filepath.Dir(destino)}},
		{false, true, Aviso{Tipo: "alerta", Titulo: "Cópia do banco", Mensagem: "Cancelado: nenhuma cópia foi feita."}},
		{false, false, Aviso{Tipo: "alerta", Titulo: "Cópia do banco",
			Mensagem: "Não deu para copiar o banco. Confira se a pasta escolhida aceita gravação."}},
	}
	for _, c := range casos {
		if obtido := AvisoDaCopia(c.feito, c.cancelado, destino); obtido != c.quer {
			t.Errorf("%v %v: %+v", c.feito, c.cancelado, obtido)
		}
	}
	// Na raiz do disco, a pasta fica com a barra (C:\ e não C:).
	raiz := filepath.VolumeName(destino) + string(filepath.Separator)
	if obtido := AvisoDaCopia(true, false, raiz+"copia.db"); obtido.Pasta != raiz {
		t.Errorf("raiz: %q", obtido.Pasta)
	}
}

// Regra: o diálogo do aviso está na página, com título, ícone do projeto e os dois botões.
func TestDialogoDoAviso(t *testing.T) {
	t.Parallel()
	html := escrever(t, Tela{})
	for _, parte := range []string{`<dialog id="aviso" class="aviso" aria-labelledby="aviso-titulo"`, `src="/icone.png"`,
		`id="aviso-abrir-pasta"`, `Abrir a pasta</button>`, `id="aviso-ok" class="botao primario"`} {
		if !strings.Contains(html, parte) {
			t.Errorf("sem %s", parte)
		}
	}
}

// Regra: procurar fica bloqueado procurando ou baixando; a versão nova e o download chegam na atualização da janela.
func TestProcuraNaAtualizacao(t *testing.T) {
	t.Parallel()
	for _, p := range []Procura{{Procurando: true}, {Baixando: true}} {
		if (Tela{Procura: p}).ProcurarHabilitado() {
			t.Errorf("%+v: procurar liberado", p)
		}
	}
	a := Tela{Procura: Procura{Nova: "2.45.0", Baixando: true, Progresso: 42}}.Atualizacao()
	if a.Nova != "2.45.0" || a.AtualizarTexto != "Atualizar para 2.45.0" || !a.Baixando || a.Progresso != 42 || a.ProcurarHabilitado {
		t.Errorf("%+v", a)
	}
}

// Regra: as frases depois de procurar, as do C#.
func TestResultadoDaProcura(t *testing.T) {
	t.Parallel()
	casos := map[[2]bool]string{
		{true, true}:   "Tem versão de desenvolvimento nova: 2.45.0-dev.3.",
		{true, false}:  "Tem versão nova: 2.45.0-dev.3.",
		{false, true}:  "Você está na última versão de desenvolvimento.",
		{false, false}: "Você está na versão mais nova.",
	}
	for c, quer := range casos {
		if obtido := ResultadoDaProcura(c[0], c[1], "2.45.0-dev.3"); obtido != quer {
			t.Errorf("%v: %q", c, obtido)
		}
	}
}

// Regra: o balão só quando o site estava no ar e desligou ou parou de responder, com a janela fechada (o
// ShowBalloonTip do FormPrincipal.cs), com o título da situação e o ícone de alerta.
func TestAvisoDoSiteQueCaiu(t *testing.T) {
	t.Parallel()
	noAr := Situacao{Estado: NoAr, Titulo: "O site está no ar."}
	desligado := Situacao{Estado: Desligado, Titulo: "O site está desligado."}
	semResposta := Situacao{Estado: SemResposta, Titulo: "O site não responde."}
	mudando := Situacao{Estado: Mudando, Titulo: "O site está ligando…"}
	naoInstalado := Situacao{Estado: NaoInstalado, Titulo: "O site não está instalado."}
	casos := []struct {
		nome             string
		anterior, nova   Situacao
		janelaAberta, ok bool
	}{
		{"desligou", noAr, desligado, false, true},
		{"parou de responder", noAr, semResposta, false, true},
		{"com a janela aberta", noAr, desligado, true, false},
		{"continua no ar", noAr, noAr, false, false},
		{"ligando", noAr, mudando, false, false},
		{"desinstalado", noAr, naoInstalado, false, false},
		{"já estava desligado", desligado, desligado, false, false},
		{"sem resposta para desligado", semResposta, desligado, false, false},
		{"da primeira consulta", Verificando, desligado, false, false},
	}
	for _, c := range casos {
		aviso, ok := AvisoDoSiteQueCaiu(c.anterior, c.nova, c.janelaAberta)
		quer := AvisoNaBandeja{}
		if c.ok {
			quer = AvisoNaBandeja{Texto: c.nova.Titulo, Alerta: true}
		}
		if ok != c.ok || aviso != quer {
			t.Errorf("%s: %+v, %v", c.nome, aviso, ok)
		}
	}
}

// Regra: o "ligando…" e o "desligando…" não trocam a situação firme; as outras trocam.
func TestFirme(t *testing.T) {
	t.Parallel()
	noAr := Situacao{Estado: NoAr, Titulo: "O site está no ar."}
	desligando := Situacao{Estado: Mudando, Titulo: "O site está desligando…"}
	for _, nova := range []Situacao{
		{Estado: NaoInstalado, Titulo: "O site não está instalado."},
		{Estado: Desligado, Titulo: "O site está desligado."},
		{Estado: SemResposta, Titulo: "O site não responde."},
		{Estado: NoAr, Titulo: "O site está no ar de novo."},
	} {
		if obtido := Firme(noAr, nova); obtido != nova {
			t.Errorf("%v: %+v", nova.Estado, obtido)
		}
	}
	if obtido := Firme(noAr, desligando); obtido != noAr {
		t.Errorf("desligando: %+v", obtido)
	}
	// Parado pelo services.msc: no ar, desligando, desligado; o aviso sai, comparando com o "no ar".
	if _, ok := AvisoDoSiteQueCaiu(Firme(noAr, desligando), Situacao{Estado: Desligado}, false); !ok {
		t.Error("services.msc: sem aviso")
	}
}

// Regra: o balão da versão nova só com a janela fechada e versão achada, com o ícone de informação.
func TestAvisoDaVersaoNova(t *testing.T) {
	t.Parallel()
	if aviso, ok := AvisoDaVersaoNova("2.50.0", false); !ok || aviso != (AvisoNaBandeja{Texto: "Versão nova: 2.50.0."}) {
		t.Errorf("janela fechada: %+v, %v", aviso, ok)
	}
	for _, c := range []struct {
		versao       string
		janelaAberta bool
	}{{"2.50.0", true}, {"", false}, {"", true}} {
		if aviso, ok := AvisoDaVersaoNova(c.versao, c.janelaAberta); ok || aviso != (AvisoNaBandeja{}) {
			t.Errorf("%+v: %+v, %v", c, aviso, ok)
		}
	}
}

// Regra: a pergunta antes de atualizar, com o texto do C#.
func TestConfirmarAtualizacao(t *testing.T) {
	t.Parallel()
	a := ConfirmarAtualizacao("2.45.0")
	if a.Tipo != "pergunta" || a.Titulo != "Atualizar o Finanças Local para a versão 2.45.0?" ||
		!strings.HasPrefix(a.Mensagem, "O instalador abre em seguida") || !strings.Contains(a.Mensagem, `dados\backups`) {
		t.Errorf("%+v", a)
	}
}

// Regra: pelo .msi, sem versão nova, "Atualizar" e "O que mudou" ficam escondidos e o download também; com versão
// nova baixando, aparecem, com "Atualizar" bloqueado e a barra no ponto do download.
func TestCartaoDaVersao(t *testing.T) {
	t.Parallel()
	sem := escrever(t, Tela{PeloMsi: true})
	for _, parte := range []string{`id="atualizar" class="botao primario" type="button" accesskey="t" hidden>`,
		`id="notas" class="botao discreto" type="button" accesskey="q" hidden>`, `id="baixando" hidden>`} {
		if !strings.Contains(sem, parte) {
			t.Errorf("sem versão nova, falta %s", parte)
		}
	}
	com := escrever(t, Tela{PeloMsi: true, Procura: Procura{Nova: "2.45.0", Baixando: true, Progresso: 30}})
	for _, parte := range []string{`accesskey="t" disabled>`, `Atualizar para 2.45.0</span>`,
		`id="notas" class="botao discreto" type="button" accesskey="q">`, `id="baixando">`, `value="30"`,
		`data-acao="procurar" accesskey="p" disabled>`} {
		if !strings.Contains(com, parte) {
			t.Errorf("baixando, falta %s", parte)
		}
	}
	if scripts := escrever(t, Tela{}); strings.Contains(scripts, `id="atualizar"`) || strings.Contains(scripts, `id="baixando"`) ||
		strings.Contains(scripts, "Receber versões de desenvolvimento") {
		t.Error("pelos scripts, com a versão nova")
	}
}

// Regra: "Receber versões de desenvolvimento" é um interruptor com o ⓘ que abre a explicação (Disclosure).
func TestInterruptorDeDesenvolvimento(t *testing.T) {
	t.Parallel()
	ligado := escrever(t, Tela{PeloMsi: true, ReceberDesenvolvimento: true})
	for _, parte := range []string{`data-preferencia="desenvolvimento" checked>Receber versões de desenvolvimento`,
		`class="info" type="button" aria-expanded="false" aria-controls="info-desenvolvimento"`, `id="info-desenvolvimento" hidden>`} {
		if !strings.Contains(ligado, parte) {
			t.Errorf("falta %s", parte)
		}
	}
	if strings.Contains(escrever(t, Tela{PeloMsi: true}), `data-preferencia="desenvolvimento" checked`) {
		t.Error("ligado sem pedir")
	}
}

// Regra: pelos scripts, num clone na develop, aparece "Atualizar para a última do develop" no lugar de "Baixar o
// instalador .msi"; rodando, o botão fica bloqueado e a caixa mostra a saída dos scripts.
func TestCartaoDoDevelop(t *testing.T) {
	t.Parallel()
	fora := escrever(t, Tela{})
	if !strings.Contains(fora, `id="atualizar-develop" class="botao primario" type="button" accesskey="v" hidden>`) ||
		strings.Contains(fora, `data-acao="baixar-msi" accesskey="b" aria-description="Abre a página da versão mais recente no GitHub" hidden`) ||
		!strings.Contains(fora, `id="saida" class="saida" readonly aria-label="Saída da atualização" hidden>`) {
		t.Error("fora do clone: botões errados")
	}
	rodando := escrever(t, Tela{Develop: Develop{Disponivel: true, Rodando: true, Saida: "==> Buscando <o develop>"}})
	for _, parte := range []string{`accesskey="v" disabled>`, `aria-description="Abre a página da versão mais recente no GitHub" hidden>`,
		`aria-label="Saída da atualização">==&gt; Buscando &lt;o develop&gt;</textarea>`} {
		if !strings.Contains(rodando, parte) {
			t.Errorf("rodando, falta %s", parte)
		}
	}
	if strings.Contains(escrever(t, Tela{PeloMsi: true}), `id="atualizar-develop"`) {
		t.Error("pelo .msi, com o develop")
	}
	a := Tela{Develop: Develop{Disponivel: true, Rodando: true, Saida: "x"}}.Atualizacao()
	if !a.Develop || !a.DevelopRodando || a.Saida != "x" {
		t.Errorf("%+v", a)
	}
}

// Regra: o texto do cartão e a pergunta antes, com os textos do C#.
func TestTextosDoDevelop(t *testing.T) {
	t.Parallel()
	if obtido := TextoDeDesenvolvimento(`C:\projeto`); !strings.HasPrefix(obtido, `Instalação de desenvolvimento: a pasta C:\projeto é um clone na branch develop.`) {
		t.Error(obtido)
	}
	a := ConfirmarDevelop(false)
	if a.Tipo != "pergunta" || a.Titulo != "Atualizar esta instalação de desenvolvimento para a última versão do develop?" ||
		!strings.HasPrefix(a.Mensagem, "O gerenciador traz o código do GitHub e reinstala o site (pede permissão de administrador). O site fica") ||
		!strings.HasSuffix(a.Mensagem, "Com alterações locais ou em outra branch, nada é mudado: a tela diz o motivo.") {
		t.Errorf("%+v", a)
	}
	// No Linux, nada pede administrador.
	if l := ConfirmarDevelop(true); l.Titulo != a.Titulo || !strings.HasPrefix(l.Mensagem, "O gerenciador traz o código do GitHub e reinstala o site. O site fica") {
		t.Errorf("linux: %+v", l)
	}
}

// Regra: o detalhe quando ligar, desligar ou reiniciar falha: sem motivo (Windows, pelo UAC), o texto de sempre; com
// o motivo do systemctl (Linux), ele vem antes de onde ver mais.
func TestNaoDeuCerto(t *testing.T) {
	t.Parallel()
	casos := map[[2]string]string{
		{"", "Veja os eventos do Windows (Visualizador de Eventos › Aplicativo, origem FinancasLocal)."}:            "Não deu certo. Veja os eventos do Windows (Visualizador de Eventos › Aplicativo, origem FinancasLocal).",
		{"Job for financaslocal.service failed.", "Em “Mais opções”, “Ver o registro do serviço” mostra o motivo."}: "Não deu certo: Job for financaslocal.service failed. Em “Mais opções”, “Ver o registro do serviço” mostra o motivo.",
	}
	for entrada, esperado := range casos {
		if obtido := NaoDeuCerto(entrada[0], entrada[1]); obtido != esperado {
			t.Errorf("%q", obtido)
		}
	}
}

// Regra: no Linux, pelos scripts, o cartão "Versão" diz como atualizar; com a pasta do projeto, o comando vem pronto,
// com a pasta entre aspas simples para o bash (inclusive com aspa e espaço no nome).
func TestComoAtualizarNoLinux(t *testing.T) {
	t.Parallel()
	if !strings.HasSuffix(TextoPelosScriptsNoLinux(true), "rode no terminal (o site fica fora do ar por alguns segundos e o banco ganha uma cópia antes):") ||
		TextoPelosScriptsNoLinux(false) != "Instalado pelos scripts: para atualizar, rode git pull, scripts/geral/3-publicar-atualizar.sh e "+
			"scripts/linux/instalar-gerenciador.sh na pasta do projeto." {
		t.Error("textos")
	}
	esperado := `cd '/home/ana/D'\''Ávila finanças' && git pull && scripts/geral/3-publicar-atualizar.sh && scripts/linux/instalar-gerenciador.sh`
	if obtido := ComandoParaAtualizar("/home/ana/D'Ávila finanças"); obtido != esperado {
		t.Errorf("%s", obtido)
	}
}

// Regra: o "sem login" (linger) diz o que acontece ao sair da conta; sem dar para saber, o bloco some; o erro de
// ligar troca o texto e diz o comando.
func TestTextoSemLogin(t *testing.T) {
	t.Parallel()
	casos := []struct {
		semLogin, erro, esperado string
	}{
		{"sim", "", "O site sobe junto com o computador, mesmo antes de você entrar na sua conta."},
		{"nao", "", "Hoje o site só fica no ar enquanto você está com a sessão aberta: ao sair da conta, ele desliga."},
		{"", "", ""},
		{"nao", "Access denied.", "Não deu certo: Access denied. No terminal: loginctl enable-linger"},
	}
	for _, c := range casos {
		tl := Tela{Linux: true, SemLogin: c.semLogin, ErroSemLogin: c.erro}
		if tl.TextoSemLogin() != c.esperado || tl.Atualizacao().TextoSemLogin != c.esperado || tl.Atualizacao().SemLogin != c.semLogin {
			t.Errorf("%+v: %q", c, tl.TextoSemLogin())
		}
	}
}

// Regra: a página no Linux: ícones em SVG (nenhum caractere da fonte do Windows), "Iniciar com o sistema", "Ver o
// registro do serviço", o comando para copiar e o "sem login", e nada de .msi; no Windows, nada disso.
func TestPaginaNoLinux(t *testing.T) {
	t.Parallel()
	html := escrever(t, Tela{Linux: true, MaisOpcoes: true, Comando: "cd '/x' && git pull", SemLogin: "nao",
		Situacao: Situacao{Estado: NoAr}})
	if regexp.MustCompile("[\uE000-\uF8FF]").MatchString(html) {
		t.Error("sobrou um glifo da fonte do Windows")
	}
	for _, parte := range []string{
		"Iniciar com o sistema (fica na área de notificação)</label>",
		`<button id="registro" class="botao" type="button" accesskey="g"><span class="ico" aria-hidden="true"><svg`,
		"Ver o registro do serviço</button>",
		`<input id="texto-comando" class="saida" type="text" readonly value="cd &#39;/x&#39; &amp;&amp; git pull" aria-label="Comando para atualizar">`,
		`data-acao="copiar-comando" accesskey="o">`,
		`<button id="botao-sem-login" class="botao" type="button" data-acao="sem-login" accesskey="s">Manter o site no ar sem login</button>`,
		`data-rotulo="Mais opções: cópia do banco, erros, registro, manual e preferências"`,
	} {
		if !strings.Contains(html, parte) {
			t.Errorf("falta %s", parte)
		}
	}
	if strings.Contains(html, "Baixar o instalador") || strings.Contains(html, "Iniciar com o Windows") {
		t.Error("no Linux, coisa do Windows")
	}
	// Sem a pasta do projeto, sem o comando; site que sobe sem login, sem o botão; sem dar para saber, sem o bloco.
	sim := escrever(t, Tela{Linux: true, SemLogin: "sim"})
	if strings.Contains(sim, `id="comando"`) || !strings.Contains(sim, `data-acao="sem-login" accesskey="s" hidden>`) ||
		!strings.Contains(escrever(t, Tela{Linux: true}), `id="sem-login" hidden>`) {
		t.Error("sem comando ou com o linger: elementos errados")
	}
	// Instalação de desenvolvimento: o botão do develop no lugar do comando.
	if !strings.Contains(escrever(t, Tela{Linux: true, Comando: "x", Develop: Develop{Disponivel: true}}), `id="comando" hidden>`) {
		t.Error("develop com o comando à mostra")
	}
	windows := escrever(t, Tela{MaisOpcoes: true, Comando: "x", SemLogin: "nao"})
	if strings.Contains(windows, `id="registro"`) || strings.Contains(windows, `id="sem-login"`) || strings.Contains(windows, "<svg") {
		t.Error("no Windows, coisa do Linux")
	}
}
