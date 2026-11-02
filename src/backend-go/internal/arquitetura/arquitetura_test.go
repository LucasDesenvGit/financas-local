//go:build arquitetura

// Package arquitetura confere quem pode importar quem no back-end Go (#476), no estilo do ArchUnit do Java: regras que
// hoje dependiam da revisão e se perdem fácil com três pessoas e agentes escrevendo código ao mesmo tempo.
//
// Só local, por decisão de 29/10/2026: a build tag tira este pacote do "go test ./..." de todo dia e da CI. Roda com
//
//	go test -tags arquitetura ./internal/arquitetura/
//
// O grafo vem do "go list -json ./..." (só a biblioteca padrão, ADR 0051). Cada regra tem o motivo e, quando há, as
// exceções explicadas. Mudou a arquitetura de propósito? Mude a tabela aqui no mesmo PR, com o motivo. As regras de
// arquitetura (regra sem HTTP, mapa de contextos, módulos) são as da ADR 0056.
package arquitetura

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

const modulo = "github.com/LucasDesenvGit/financas-local/src/backend-go/"

// pacote é o que as regras olham de cada pacote, com o módulo cortado ("internal/carteira").
type pacote struct {
	Caminho string
	Imports []string // diretos, fora dos _test.go
	Deps    []string // todos os que ele usa, diretos e indiretos, fora dos _test.go
}

/* ------------------------------------------------------------ as tabelas */

// Regra 1: o driver do SQLite.
const driverDoBanco = "modernc.org/sqlite"

var podemUsarODriver = map[string]string{
	"internal/banco":         "abre o banco (WAL, busy_timeout) para todo mundo",
	"internal/banco/esquema": "aplica o banco/01-estrutura.sql",
	"testes/contrato":        "abre o banco dos casos para conferir o que foi gravado",
}

// Regra 2: o apoio aos testes.
const apoioAosTestes = "internal/testeapoio"

// Regra 3: pacote de regra não conhece HTTP, nem por outro pacote.
var proibidosNaRegra = []string{"net/http", "internal/respostas", "internal/servidor"}

var regraSemHTTP = []string{
	"internal/banco", "internal/banco/consultas", "internal/bancos", "internal/categorizacao", "internal/cobrancas",
	"internal/configuracao",
	"internal/desfazer", "internal/emprestimos", "internal/faturas", "internal/parcelas", "internal/poucoinformativa",
	"internal/regrasdescricao", "internal/relatorios", "internal/relogio", "internal/segredos", "internal/sugestoes",
	"internal/texto", "internal/tipolancamento", "internal/tipos", "internal/transacoes",
}

// Regra de pacote que ainda chega ao HTTP por outro pacote. Quando deixar de chegar, o teste avisa para tirar daqui.
var regraComHTTPPorEnquanto = map[string]string{
	"internal/transferencias": "usa o reservas.EhMovimentoDeReserva, uma regra pura que mora num assunto com rotas",
	"internal/tarefas":        "registra a falha das tarefas no sistema de erros (erros.OrigemCripto, erros.OrigemSistema)",
}

// Regra 4: quem importa quem. Todo pacote de internal/ está num destes grupos.
var (
	// base: o que qualquer pacote pode importar (banco, tipos, relógio, HTTP comum, texto, sessão).
	base = []string{
		"internal/banco", "internal/banco/consultas", "internal/banco/esquema", "internal/banco/esquema/embutido",
		"internal/banco/esquema/gerar", "internal/config", "internal/relogio", "internal/respostas",
		"internal/servidor", "internal/sessao", "internal/testeapoio", "internal/texto", "internal/tipos",
	}
	// transversais: serviços que qualquer assunto usa (auditoria, parâmetros do banco, erros do sistema, tarefas em
	// segundo plano, segredos, versão).
	transversais = []string{
		"internal/auditoria", "internal/configuracao", "internal/erros", "internal/segredos", "internal/tarefas",
		"internal/versao",
	}
	// montadores: juntam tudo (rotas e programa) e podem importar qualquer assunto.
	montadores = []string{"internal/api", "cmd/financas", "testes/contrato", "internal/arquitetura"}
	// modulos (#190, #191, ADR 0056): visualizações que se ligam e desligam. Um módulo usa o núcleo (assuntos, base e
	// transversais), mas o núcleo não importa módulo, e um módulo não importa outro. O primeiro módulo entra aqui.
	modulos = []string{}
)

// dependencias é a tabela de quem pode importar quem entre os assuntos: a semente do mapa de contextos (#462, #474).
// Assunto sem linha não importa nenhum outro. Dependência nova = acrescentar aqui, com o motivo, no mesmo PR.
var dependencias = []struct {
	De     string
	Para   []string
	Motivo string
}{
	{"internal/autenticacao", nil, "só os transversais (auditoria, erros)"},
	{"internal/bancos", nil, "catálogo de bancos"},
	{"internal/cambio", nil, "PTAX; só os transversais"},
	{"internal/carteira", []string{"internal/bancos", "internal/cambio", "internal/categorizacao", "internal/contas",
		"internal/cotacoes", "internal/desfazer", "internal/reservas", "internal/selic", "internal/transacoes"},
		"a carteira junta posições, cotações, câmbio, reservas e as transações de investimento (#82, #299)"},
	{"internal/categorias", []string{"internal/categorizacao", "internal/regrasdescricao", "internal/transacoes",
		"internal/transferencias"}, "as rotas de categorias e regras reaplicam as regras nas transações (#77)"},
	{"internal/categorizacao", []string{"internal/parcelas", "internal/tipolancamento", "internal/transacoes"},
		"a regra de categoria olha o tipo do lançamento e grava na transação; o acerto das compras parceladas estornadas casa o estorno com as parcelas (#444)"},
	{"internal/cobrancas", []string{"internal/relatorios", "internal/sugestoes"},
		"a conciliação avança o vencimento como a lista e usa a chave do extrato das sugestões (#79, #158, #205)"},
	{"internal/contasagendadas", []string{"internal/categorizacao", "internal/cobrancas", "internal/relatorios",
		"internal/sugestoes", "internal/transacoes"},
		"as contas a pagar listam pela base dos relatórios, conciliam e lançam a despesa do pagamento (#79)"},
	{"internal/conferenciabanco", []string{"internal/bancos", "internal/cambio", "internal/carteira", "internal/contas",
		"internal/faturas", "internal/reservas"}, "a conferência do banco compara os saldos de todos esses (#445)"},
	{"internal/contas", []string{"internal/bancos", "internal/faturas"}, "a conta mostra o banco e a fatura do cartão"},
	{"internal/cotacoes", []string{"internal/cambio"}, "cotação em dólar convertida pela PTAX"},
	{"internal/cripto", nil, "Mercado Bitcoin; só os transversais"},
	{"internal/desfazer", nil, "o desfazer das ações em massa"},
	{"internal/emprestimos", nil, "totais de empréstimos (#447)"},
	{"internal/extrato", []string{"internal/categorizacao", "internal/desfazer", "internal/poucoinformativa",
		"internal/regrasdescricao", "internal/sugestoes", "internal/tipolancamento", "internal/transacoes",
		"internal/transferencias"}, "a tela de transações junta as regras, as sugestões e o desfazer (#78)"},
	{"internal/faturas", []string{"internal/bancos", "internal/parcelas"}, "a fatura soma as parcelas do cartão"},
	{"internal/investimentos", []string{"internal/cambio", "internal/carteira"},
		"o saldo em reais pelo câmbio e o nome de exibição da carteira (#170)"},
	{"internal/metas", []string{"internal/contas", "internal/faturas", "internal/parametros", "internal/parcelas",
		"internal/relatorios", "internal/reservas", "internal/transacoes"},
		"o progresso soma o saldo da conta ou do cofrinho; o plano usa as faturas, as parcelas e o histórico"},
	{"internal/orcamentos", []string{"internal/emprestimos", "internal/parametros", "internal/relatorios", "internal/transacoes"},
		"a lista do mês e a sugestão (#39) usam a base dos relatórios, os totais de empréstimos, os parâmetros e o tipo da transação (#79)"},
	{"internal/partida", []string{"internal/carteira", "internal/categorizacao", "internal/cobrancas", "internal/transferencias"},
		"as seis rotinas que acertam dados ao subir chamam a regra de cada assunto, na ordem do .NET: nenhuma regra mora aqui (#444)"},
	{"internal/parametros", nil, "só os transversais (auditoria, configuração)"},
	{"internal/painel", []string{"internal/cambio", "internal/carteira", "internal/contas", "internal/emprestimos",
		"internal/extrato", "internal/relatorios", "internal/transacoes"},
		"as rotas da entrada painel (#80): empréstimos e relatórios, com o patrimônio pelas contas, a carteira e o câmbio"},
	{"internal/parcelas", nil, "parcelas de compras no cartão"},
	{"internal/poucoinformativa", nil, "descrição que não diz nada"},
	{"internal/regrasdescricao", []string{"internal/categorizacao", "internal/transacoes"},
		"a regra de descrição renomeia a transação e recategoriza"},
	{"internal/relatorios", []string{"internal/parcelas", "internal/transacoes"},
		"a base dos relatórios lê as transações (#447); a projeção soma as parcelas futuras (#80, #165)"},
	{"internal/reservas", []string{"internal/cambio"}, "reserva em dólar convertida pela PTAX"},
	{"internal/saude", nil, "GET de saúde da API"},
	{"internal/selic", nil, "Selic do BCB"},
	{"internal/sugestoes", nil, "sugestões de categoria pelo histórico"},
	{"internal/tipolancamento", nil, "tipo de cada lançamento (ADR 0025)"},
	{"internal/transacoes", nil, "o modelo da transação"},
	{"internal/transferencias", []string{"internal/categorizacao", "internal/reservas", "internal/tipolancamento",
		"internal/transacoes"}, "transferência entre contas e movimento de reserva (ADR 0031)"},
}

/* ------------------------------------------------------------ as regras */

// violacoes devolve uma mensagem por violação, cada uma com a regra, o motivo e o que fazer.
func violacoes(pacotes []pacote) []string {
	var v []string
	existe := map[string]bool{}
	for _, p := range pacotes {
		existe[p.Caminho] = true
	}

	for _, p := range pacotes {
		// Regra 1: o driver do SQLite.
		for _, imp := range p.Imports {
			if (imp == driverDoBanco || strings.HasPrefix(imp, driverDoBanco+"/")) && podemUsarODriver[p.Caminho] == "" {
				v = append(v, fmt.Sprintf("regra 1 (só o internal/banco abre o SQLite): %s importa %s. O que fazer: "+
					"use o *sql.DB que o internal/banco abre (WAL, busy_timeout, formato do EF Core).", p.Caminho, imp))
			}
		}
		// Regra 2: o apoio aos testes só em _test.go.
		if p.Caminho != apoioAosTestes && slices.Contains(p.Deps, apoioAosTestes) {
			v = append(v, fmt.Sprintf("regra 2 (o %s é só para _test.go): %s chega a ele. O que fazer: importe o "+
				"apoio só nos arquivos _test.go; o programa não pode levar banco de mentira nem fábrica de dados.",
				apoioAosTestes, p.Caminho))
		}
	}

	// Regra 3: regra sem HTTP.
	porCaminho := map[string]pacote{}
	for _, p := range pacotes {
		porCaminho[p.Caminho] = p
	}
	for _, caminho := range regraSemHTTP {
		p, ok := porCaminho[caminho]
		if !ok {
			v = append(v, fmt.Sprintf("regra 3: %s está na lista regraSemHTTP, mas não existe mais. O que fazer: tire "+
				"da lista.", caminho))
			continue
		}
		var chega []string
		for _, proibido := range proibidosNaRegra {
			if slices.Contains(p.Deps, proibido) {
				chega = append(chega, proibido)
			}
		}
		if len(chega) > 0 {
			v = append(v, fmt.Sprintf("regra 3 (a regra de negócio não conhece HTTP): %s chega a %s%s. O que "+
				"fazer: tire o import (a função de texto ou de cálculo vai para o pacote de regra, e o HTTP fica nas "+
				"rotas do assunto), ou registre em regraComHTTPPorEnquanto com o motivo.",
				caminho, strings.Join(chega, ", "), caminhoAte(porCaminho, caminho, chega[0])))
		}
	}
	for caminho := range regraComHTTPPorEnquanto {
		p, ok := porCaminho[caminho]
		if !ok || !slices.ContainsFunc(proibidosNaRegra, func(x string) bool { return slices.Contains(p.Deps, x) }) {
			v = append(v, fmt.Sprintf("regra 3: %s não chega mais ao HTTP. O que fazer: tire de "+
				"regraComHTTPPorEnquanto e ponha em regraSemHTTP, para não voltar.", caminho))
		}
	}

	// Regra 4: quem importa quem.
	grupo := map[string]string{}
	for _, c := range base {
		grupo[c] = "base"
	}
	for _, c := range transversais {
		grupo[c] = "transversal"
	}
	for _, c := range montadores {
		grupo[c] = "montador"
	}
	for _, c := range modulos {
		grupo[c] = "modulo"
	}
	permitido := map[string]map[string]bool{}
	for _, d := range dependencias {
		grupo[d.De] = "assunto"
		permitido[d.De] = map[string]bool{}
		for _, para := range d.Para {
			permitido[d.De][para] = true
		}
	}
	for _, p := range pacotes {
		if grupo[p.Caminho] == "" {
			v = append(v, fmt.Sprintf("regra 4: %s não está em nenhum grupo. O que fazer: assunto novo ganha uma "+
				"linha em dependencias (com quem ele importa e o motivo); pacote de base ou transversal entra na lista "+
				"do grupo.", p.Caminho))
			continue
		}
		if grupo[p.Caminho] == "montador" {
			continue
		}
		for _, imp := range p.Imports {
			// Regra 5: os módulos (ADR 0056).
			if grupo[imp] == "modulo" && grupo[p.Caminho] == "modulo" {
				v = append(v, fmt.Sprintf("regra 5 (um módulo não depende de outro): %s importa %s. O que fazer: o que "+
					"os dois usam vai para o núcleo (um assunto ou a base); cada módulo se liga e desliga sozinho.",
					p.Caminho, imp))
				continue
			}
			if grupo[imp] == "modulo" {
				v = append(v, fmt.Sprintf("regra 5 (o núcleo não depende de módulo): %s importa o módulo %s. O que "+
					"fazer: o módulo usa o núcleo, e não o contrário; o que o núcleo precisa vai para um assunto.",
					p.Caminho, imp))
				continue
			}
			if grupo[p.Caminho] == "modulo" {
				continue // o módulo pode usar qualquer assunto do núcleo
			}
			if grupo[imp] != "assunto" || permitido[p.Caminho][imp] {
				continue
			}
			if grupo[p.Caminho] == "assunto" {
				v = append(v, fmt.Sprintf("regra 4 (mapa de contextos): %s importa %s, que não está na linha dele em "+
					"dependencias. O que fazer: se a dependência é de propósito, acrescente %s na linha de %s, com o "+
					"motivo, no mesmo PR; se não, use o que o outro assunto expõe pela API ou mova a regra comum para "+
					"um pacote de regra.", p.Caminho, imp, imp, p.Caminho))
			} else {
				v = append(v, fmt.Sprintf("regra 4: %s (%s) importa o assunto %s. O que fazer: base e transversais "+
					"não dependem de assunto; mova o que for comum para a base.", p.Caminho, grupo[p.Caminho], imp))
			}
		}
	}
	for _, d := range dependencias {
		if !existe[d.De] {
			v = append(v, fmt.Sprintf("regra 4: %s está em dependencias, mas não existe mais. O que fazer: tire a linha.",
				d.De))
		}
	}
	sort.Strings(v)
	return v
}

// caminhoAte mostra por onde o pacote chega ao proibido (" (por internal/x → internal/respostas)"), para a mensagem
// apontar o import que precisa sair. Vazio quando o import é direto.
func caminhoAte(porCaminho map[string]pacote, de, ate string) string {
	var passos []string
	for atual := de; !slices.Contains(porCaminho[atual].Imports, ate); {
		proximo := ""
		for _, imp := range porCaminho[atual].Imports {
			if p, ok := porCaminho[imp]; ok && slices.Contains(p.Deps, ate) {
				proximo = imp
				break
			}
		}
		if proximo == "" || len(passos) > len(porCaminho) {
			break
		}
		passos = append(passos, proximo)
		atual = proximo
	}
	if len(passos) == 0 {
		return ""
	}
	return " (por " + strings.Join(passos, " → ") + ")"
}

/* ------------------------------------------------------------ o grafo */

// lerPacotes roda o "go list -json ./..." na raiz do módulo e corta o nome do módulo dos caminhos.
func lerPacotes(t *testing.T) []pacote {
	t.Helper()
	cmd := exec.Command("go", "list", "-e", "-json", "./...")
	cmd.Dir = filepath.Join("..", "..")
	var saida, erro bytes.Buffer
	cmd.Stdout, cmd.Stderr = &saida, &erro
	if err := cmd.Run(); err != nil {
		t.Fatalf("go list: %v\n%s", err, erro.String())
	}
	cortar := func(lista []string) []string {
		var r []string
		for _, c := range lista {
			r = append(r, strings.TrimPrefix(c, modulo))
		}
		return r
	}
	var pacotes []pacote
	dec := json.NewDecoder(&saida)
	for {
		var p struct {
			ImportPath string
			GoFiles    []string
			Imports    []string
			Deps       []string
		}
		if err := dec.Decode(&p); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		if len(p.GoFiles) == 0 { // só _test.go (este pacote)
			continue
		}
		pacotes = append(pacotes, pacote{strings.TrimPrefix(p.ImportPath, modulo), cortar(p.Imports), cortar(p.Deps)})
	}
	if len(pacotes) < 30 {
		t.Fatalf("o go list achou só %d pacotes: rodou fora do módulo?", len(pacotes))
	}
	return pacotes
}

/* ------------------------------------------------------------ os testes */

// Regra: o código de hoje segue as quatro regras, com as exceções registradas.
func TestArquitetura(t *testing.T) {
	for _, v := range violacoes(lerPacotes(t)) {
		t.Error(v)
	}
}

// Regra: cada regra falha com um import proibido de mentira, e a mensagem diz a regra e o que fazer.
func TestCadaRegraFalha(t *testing.T) {
	antes := modulos
	modulos = []string{"internal/trabalho", "internal/noticias"}
	defer func() { modulos = antes }()
	limpo := func() []pacote {
		var r []pacote
		// Um grafo mínimo que passa: os pacotes das listas, sem import nenhum, e as exceções chegando ao HTTP.
		vistos := map[string]bool{}
		for _, grupo := range [][]string{base, transversais, regraSemHTTP, modulos} {
			for _, c := range grupo {
				if !vistos[c] {
					vistos[c] = true
					r = append(r, pacote{Caminho: c})
				}
			}
		}
		for _, d := range dependencias {
			if !vistos[d.De] {
				vistos[d.De] = true
				r = append(r, pacote{Caminho: d.De})
			}
		}
		for i := range r {
			if regraComHTTPPorEnquanto[r[i].Caminho] != "" {
				r[i].Deps = []string{"net/http"}
			}
		}
		return r
	}
	if v := violacoes(limpo()); len(v) > 0 {
		t.Fatalf("o grafo limpo já tem violações: %v", v)
	}
	mudar := func(caminho string, f func(*pacote)) []pacote {
		r := limpo()
		for i := range r {
			if r[i].Caminho == caminho {
				f(&r[i])
				return r
			}
		}
		return append(r, func() pacote { p := pacote{Caminho: caminho}; f(&p); return p }())
	}
	// O módulo pode usar o núcleo: um assunto, sem linha na tabela.
	if v := violacoes(mudar("internal/trabalho", func(p *pacote) { p.Imports = []string{"internal/transacoes"} })); len(v) > 0 {
		t.Errorf("módulo usando um assunto do núcleo: %v", v)
	}
	casos := []struct {
		nome     string
		grafo    []pacote
		mensagem string
	}{
		{"driver fora do banco", mudar("internal/carteira", func(p *pacote) { p.Imports = []string{driverDoBanco} }),
			"regra 1 (só o internal/banco abre o SQLite): internal/carteira importa modernc.org/sqlite"},
		{"apoio no programa", mudar("internal/contas", func(p *pacote) { p.Deps = []string{apoioAosTestes} }),
			"regra 2 (o internal/testeapoio é só para _test.go): internal/contas chega a ele"},
		{"regra com HTTP", mudar("internal/transacoes", func(p *pacote) {
			p.Imports, p.Deps = []string{"internal/respostas"}, []string{"internal/respostas", "net/http"}
		}), "regra 3 (a regra de negócio não conhece HTTP): internal/transacoes chega a net/http, internal/respostas"},
		{"exceção que sarou", mudar("internal/tarefas", func(p *pacote) { p.Deps = nil }),
			"regra 3: internal/tarefas não chega mais ao HTTP"},
		{"pacote sem grupo", mudar("internal/novo", func(*pacote) {}),
			"regra 4: internal/novo não está em nenhum grupo"},
		{"assunto fora do mapa", mudar("internal/selic", func(p *pacote) { p.Imports = []string{"internal/carteira"} }),
			"regra 4 (mapa de contextos): internal/selic importa internal/carteira"},
		{"módulo importa módulo", mudar("internal/noticias", func(p *pacote) { p.Imports = []string{"internal/trabalho"} }),
			"regra 5 (um módulo não depende de outro): internal/noticias importa internal/trabalho"},
		{"núcleo importa módulo", mudar("internal/contas", func(p *pacote) { p.Imports = []string{"internal/trabalho"} }),
			"regra 5 (o núcleo não depende de módulo): internal/contas importa o módulo internal/trabalho"},
		{"base que depende de assunto", mudar("internal/tipos", func(p *pacote) { p.Imports = []string{"internal/contas"} }),
			"regra 4: internal/tipos (base) importa o assunto internal/contas"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			v := violacoes(c.grafo)
			if len(v) != 1 || !strings.HasPrefix(v[0], c.mensagem) || !strings.Contains(v[0], "O que fazer:") {
				t.Errorf("violações: %q\nquer uma, começando com %q e com o que fazer", v, c.mensagem)
			}
		})
	}
}
