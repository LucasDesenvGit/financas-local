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
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
)

const modulo = "github.com/LucasDesenvGit/financas-local/src/backend/"

// pacote é o que as regras olham de cada pacote, com o módulo cortado ("internal/patrimonio/carteira").
type pacote struct {
	Caminho string
	Imports []string // diretos, fora dos _test.go
	Deps    []string // todos os que ele usa, diretos e indiretos, fora dos _test.go
}

/* ------------------------------------------------------------ as tabelas */

// Regra 1: o driver do SQLite.
const driverDoBanco = "modernc.org/sqlite"

var podemUsarODriver = map[string]string{
	"internal/plataforma/banco":         "abre o banco (WAL, busy_timeout) para todo mundo",
	"internal/plataforma/banco/esquema": "aplica o banco/01-estrutura.sql",
	"testes/contrato":                   "abre o banco dos casos para conferir o que foi gravado",
}

// Regra 2: o apoio aos testes.
const apoioAosTestes = "internal/plataforma/testeapoio"

// Regra 3: pacote de regra não conhece HTTP, nem por outro pacote.
var proibidosNaRegra = []string{"net/http", "internal/plataforma/respostas", "internal/plataforma/servidor"}

var regraSemHTTP = []string{
	"internal/plataforma/banco", "internal/plataforma/banco/consultas", "internal/movimentacao/bancos", "internal/ajustes/categorizacao", "internal/movimentacao/cobrancas",
	"internal/plataforma/configuracao",
	"internal/plataforma/desfazer", "internal/patrimonio/emprestimos", "internal/movimentacao/faturas", "internal/movimentacao/parcelas", "internal/movimentacao/poucoinformativa",
	"internal/ajustes/regrasdescricao", "internal/analises/relatorios", "internal/plataforma/relogio", "internal/plataforma/segredos", "internal/movimentacao/sugestoes",
	"internal/plataforma/texto", "internal/movimentacao/tipolancamento", "internal/plataforma/tipos", "internal/movimentacao/transacoes",
	"internal/modulos/trabalho/encargos",
}

// Regra de pacote que ainda chega ao HTTP por outro pacote. Quando deixar de chegar, o teste avisa para tirar daqui.
var regraComHTTPPorEnquanto = map[string]string{
	"internal/movimentacao/transferencias": "usa o reservas.EhMovimentoDeReserva, uma regra pura que mora num assunto com rotas",
	"internal/plataforma/tarefas":          "registra a falha das tarefas no sistema de erros (erros.OrigemCripto, erros.OrigemSistema)",
}

// Regra 4: quem importa quem. Todo pacote de internal/ está num destes grupos.
var (
	// base: o que qualquer pacote pode importar (banco, tipos, relógio, HTTP comum, texto, sessão).
	base = []string{
		"internal/plataforma/banco", "internal/plataforma/banco/consultas", "internal/plataforma/banco/esquema", "internal/plataforma/banco/esquema/embutido",
		"internal/plataforma/banco/esquema/gerar", "internal/plataforma/config", "internal/plataforma/relogio", "internal/plataforma/respostas", "internal/plataforma/servicosistema",
		"internal/plataforma/servidor", "internal/plataforma/sessao", "internal/plataforma/testeapoio", "internal/plataforma/texto", "internal/plataforma/tipos",
	}
	// transversais: serviços que qualquer assunto usa (auditoria, parâmetros do banco, erros do sistema, tarefas em
	// segundo plano, segredos, versão) e o núcleo dos módulos (o contrato que cada módulo implementa, ADR 0064).
	transversais = []string{
		"internal/plataforma/auditoria", "internal/plataforma/configuracao", "internal/plataforma/erros", "internal/plataforma/segredos", "internal/plataforma/tarefas",
		"internal/plataforma/modulos", "internal/plataforma/versao",
	}
	// montadores: juntam tudo (rotas e programa) e podem importar qualquer assunto.
	montadores = []string{"internal/api", "cmd/financas", "testes/contrato", "testes/contrato/pluggyfalsa", "internal/arquitetura"}
)

// raizDosModulos (#190, #191, ADRs 0056 e 0064): visualizações que se ligam e desligam. Todo pacote daqui é de módulo,
// sem lista: internal/modulos/<id> é o módulo e internal/modulos/<id>/<assunto>, os assuntos dele. Um módulo usa o
// núcleo (assuntos, base e transversais), mas o núcleo não importa módulo, e um módulo não importa outro (os pacotes
// do mesmo <id> se importam à vontade).
const raizDosModulos = "internal/modulos/"

// moduloDe é o módulo do pacote ("internal/modulos/trabalho/holerite" → "internal/modulos/trabalho"), ou "".
func moduloDe(caminho string) string {
	resto, ok := strings.CutPrefix(caminho, raizDosModulos)
	if !ok || resto == "" {
		return ""
	}
	id, _, _ := strings.Cut(resto, "/")
	return raizDosModulos + id
}

// dependencias é a tabela de quem pode importar quem entre os assuntos: a semente do mapa de contextos (#462, #474).
// Assunto sem linha não importa nenhum outro. Dependência nova = acrescentar aqui, com o motivo, no mesmo PR.
var dependencias = []struct {
	De     string
	Para   []string
	Motivo string
}{
	{"internal/plataforma/autenticacao", nil, "só os transversais (auditoria, erros)"},
	{"internal/plataforma/celular", nil, "acesso pelo celular (#328): só os transversais (auditoria, configuração, erros) e a base (o servidor monta o da rede)"},
	{"internal/movimentacao/bancos", nil, "catálogo de bancos"},
	{"internal/patrimonio/cambio", nil, "PTAX; só os transversais"},
	{"internal/patrimonio/carteira", []string{"internal/movimentacao/bancos", "internal/patrimonio/cambio", "internal/ajustes/categorizacao", "internal/movimentacao/contas",
		"internal/patrimonio/cotacoes", "internal/plataforma/desfazer", "internal/patrimonio/reservas", "internal/patrimonio/selic", "internal/movimentacao/transacoes"},
		"a carteira junta posições, cotações, câmbio, reservas e as transações de investimento (#82, #299)"},
	{"internal/ajustes/categorias", []string{"internal/ajustes/categorizacao", "internal/ajustes/regrasdescricao", "internal/movimentacao/transacoes",
		"internal/movimentacao/transferencias"}, "as rotas de categorias e regras reaplicam as regras nas transações (#77)"},
	{"internal/ajustes/categorizacao", []string{"internal/movimentacao/parcelas", "internal/movimentacao/tipolancamento", "internal/movimentacao/transacoes"},
		"a regra de categoria olha o tipo do lançamento e grava na transação; o acerto das compras parceladas estornadas casa o estorno com as parcelas (#444)"},
	{"internal/movimentacao/cobrancas", []string{"internal/analises/relatorios", "internal/movimentacao/sugestoes"},
		"a conciliação avança o vencimento como a lista e usa a chave do extrato das sugestões (#79, #158, #205)"},
	{"internal/movimentacao/contasagendadas", []string{"internal/ajustes/categorizacao", "internal/movimentacao/cobrancas", "internal/analises/relatorios",
		"internal/movimentacao/sugestoes", "internal/movimentacao/transacoes"},
		"as contas a pagar listam pela base dos relatórios, conciliam e lançam a despesa do pagamento (#79)"},
	{"internal/movimentacao/conferenciabanco", []string{"internal/movimentacao/bancos", "internal/patrimonio/cambio", "internal/patrimonio/carteira", "internal/movimentacao/contas",
		"internal/movimentacao/faturas", "internal/patrimonio/reservas"}, "a conferência do banco compara os saldos de todos esses (#445)"},
	{"internal/movimentacao/contas", []string{"internal/movimentacao/bancos", "internal/movimentacao/faturas"}, "a conta mostra o banco e a fatura do cartão"},
	{"internal/patrimonio/cotacoes", []string{"internal/patrimonio/cambio"}, "cotação em dólar convertida pela PTAX"},
	{"internal/patrimonio/cripto", nil, "Mercado Bitcoin; só os transversais"},
	{"internal/plataforma/desfazer", nil, "o desfazer das ações em massa"},
	{"internal/patrimonio/emprestimos", nil, "totais de empréstimos (#447)"},
	{"internal/movimentacao/extrato", []string{"internal/ajustes/categorizacao", "internal/plataforma/desfazer", "internal/movimentacao/poucoinformativa",
		"internal/ajustes/regrasdescricao", "internal/movimentacao/sugestoes", "internal/movimentacao/tipolancamento", "internal/movimentacao/transacoes",
		"internal/movimentacao/transferencias"}, "a tela de transações junta as regras, as sugestões e o desfazer (#78)"},
	{"internal/movimentacao/faturas", []string{"internal/movimentacao/bancos", "internal/movimentacao/parcelas"}, "a fatura soma as parcelas do cartão"},
	{"internal/patrimonio/investimentos", []string{"internal/patrimonio/cambio", "internal/patrimonio/carteira"},
		"o saldo em reais pelo câmbio e o nome de exibição da carteira (#170)"},
	{"internal/planejamento/metas", []string{"internal/movimentacao/contas", "internal/movimentacao/faturas", "internal/ajustes/parametros", "internal/movimentacao/parcelas",
		"internal/analises/relatorios", "internal/patrimonio/reservas", "internal/movimentacao/transacoes"},
		"o progresso soma o saldo da conta ou do cofrinho; o plano usa as faturas, as parcelas e o histórico"},
	{"internal/planejamento/orcamentos", []string{"internal/patrimonio/emprestimos", "internal/ajustes/parametros", "internal/analises/relatorios", "internal/movimentacao/transacoes"},
		"a lista do mês e a sugestão (#39) usam a base dos relatórios, os totais de empréstimos, os parâmetros e o tipo da transação (#79)"},
	{"internal/partida", []string{"internal/patrimonio/carteira", "internal/ajustes/categorizacao", "internal/movimentacao/cobrancas", "internal/movimentacao/transferencias"},
		"as seis rotinas que acertam dados ao subir chamam a regra de cada assunto, na ordem do .NET: nenhuma regra mora aqui (#444)"},
	{"internal/ajustes/parametros", nil, "só os transversais (auditoria, configuração)"},
	{"internal/analises/painel", []string{"internal/patrimonio/cambio", "internal/patrimonio/carteira", "internal/movimentacao/contas", "internal/movimentacao/contasagendadas",
		"internal/patrimonio/emprestimos", "internal/movimentacao/extrato", "internal/movimentacao/faturas", "internal/movimentacao/poucoinformativa", "internal/analises/relatorios",
		"internal/patrimonio/reservas", "internal/movimentacao/transacoes"},
		"as rotas da entrada painel (#80): o painel junta o resumo e os avisos de cada assunto (contas, carteira, contas a pagar, faturas, reservas)"},
	{"internal/openfinance/openfinance", []string{"internal/ajustes/categorizacao", "internal/movimentacao/contas", "internal/patrimonio/cotacoes", "internal/openfinance/pluggy", "internal/openfinance/sincronizacao", "internal/movimentacao/transacoes",
		"internal/movimentacao/transferencias"}, "as conexões usam o nome e as faturas das contas; mapear uma categoria reaplica as regras; as rotas chamam a sincronização (#83)"},
	{"internal/movimentacao/parcelas", nil, "parcelas de compras no cartão"},
	{"internal/openfinance/pluggy", nil, "cliente HTTP da Pluggy (ADR 0006); não grava no banco"},
	{"internal/movimentacao/poucoinformativa", nil, "descrição que não diz nada"},
	{"internal/ajustes/regrasdescricao", []string{"internal/ajustes/categorizacao", "internal/movimentacao/transacoes"},
		"a regra de descrição renomeia a transação e recategoriza"},
	{"internal/analises/relatorios", []string{"internal/movimentacao/parcelas", "internal/movimentacao/transacoes"},
		"a base dos relatórios lê as transações (#447); a projeção soma as parcelas futuras (#80, #165)"},
	{"internal/patrimonio/reservas", []string{"internal/patrimonio/cambio"}, "reserva em dólar convertida pela PTAX"},
	{"internal/plataforma/saude", nil, "GET de saúde da API"},
	{"internal/patrimonio/selic", nil, "Selic do BCB"},
	{"internal/openfinance/sincronizacao", []string{"internal/patrimonio/carteira", "internal/ajustes/categorizacao", "internal/movimentacao/cobrancas", "internal/movimentacao/contas", "internal/patrimonio/cotacoes",
		"internal/patrimonio/emprestimos", "internal/patrimonio/investimentos", "internal/openfinance/pluggy", "internal/ajustes/regrasdescricao", "internal/movimentacao/transacoes", "internal/movimentacao/transferencias"},
		"a sincronização grava em cada assunto e chama, no fim, as mesmas rotinas da partida (#83)"},
	{"internal/movimentacao/sugestoes", nil, "sugestões de categoria pelo histórico"},
	{"internal/movimentacao/tipolancamento", nil, "tipo de cada lançamento (ADR 0025)"},
	{"internal/movimentacao/transacoes", nil, "o modelo da transação"},
	{"internal/movimentacao/transferencias", []string{"internal/ajustes/categorizacao", "internal/patrimonio/reservas", "internal/movimentacao/tipolancamento",
		"internal/movimentacao/transacoes"}, "transferência entre contas e movimento de reserva (ADR 0031)"},
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
				v = append(v, fmt.Sprintf("regra 1 (só o internal/plataforma/banco abre o SQLite): %s importa %s. O que fazer: "+
					"use o *sql.DB que o internal/plataforma/banco abre (WAL, busy_timeout, formato do EF Core).", p.Caminho, imp))
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
	permitido := map[string]map[string]bool{}
	for _, d := range dependencias {
		grupo[d.De] = "assunto"
		permitido[d.De] = map[string]bool{}
		for _, para := range d.Para {
			permitido[d.De][para] = true
		}
	}
	for _, p := range pacotes {
		for _, c := range append([]string{p.Caminho}, p.Imports...) {
			if moduloDe(c) != "" {
				grupo[c] = "modulo"
			}
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
				if moduloDe(imp) == moduloDe(p.Caminho) {
					continue // o módulo e os assuntos dele
				}
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

// caminhoAte mostra por onde o pacote chega ao proibido (" (por internal/x → internal/plataforma/respostas)"), para a mensagem
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

// paginaDosComponentes é o diagrama de Componentes (C4 nível 3, #464), a partir desta pasta.
var paginaDosComponentes = filepath.Join("..", "..", "..", "..", "docs", "arquitetura", "componentes.md")

// pacoteCitado acha os pacotes citados entre crases na página ("`internal/plataforma/banco`"). A crase de fechar logo depois do
// nome separa o internal/plataforma/banco do internal/plataforma/banco/consultas e deixa de fora os arquivos ("`internal/x/y.go`").
var pacoteCitado = regexp.MustCompile("`((?:cmd|internal|testes)/[a-z0-9/]+)`")

// pacotesForaDaPagina devolve uma mensagem por pacote que existe e não está na página, e por pacote citado que não
// existe mais.
func pacotesForaDaPagina(pacotes []string, pagina string) []string {
	citados := map[string]bool{}
	for _, m := range pacoteCitado.FindAllStringSubmatch(pagina, -1) {
		citados[m[1]] = true
	}
	existe := map[string]bool{}
	var v []string
	for _, p := range pacotes {
		existe[p] = true
		if !citados[p] {
			v = append(v, fmt.Sprintf("diagrama de Componentes: %s não está em docs/arquitetura/componentes.md. O que "+
				"fazer: ponha o pacote na linha do componente dele na tabela \"Cada pacote num componente\" (e no "+
				"desenho, se for um componente novo), no mesmo PR.", p))
		}
	}
	for c := range citados {
		if !existe[c] {
			v = append(v, fmt.Sprintf("diagrama de Componentes: docs/arquitetura/componentes.md cita %s, que não "+
				"existe mais. O que fazer: tire da página.", c))
		}
	}
	sort.Strings(v)
	return v
}

// Regra: todo pacote aparece na página do diagrama de Componentes (#464), e a página não cita pacote que saiu.
func TestDiagramaDeComponentes(t *testing.T) {
	pagina, err := os.ReadFile(paginaDosComponentes)
	if err != nil {
		t.Fatal(err)
	}
	// Este pacote só tem _test.go, então o lerPacotes o deixa de fora; a página o cita, como os outros.
	caminhos := []string{"internal/arquitetura"}
	for _, p := range lerPacotes(t) {
		caminhos = append(caminhos, p.Caminho)
	}
	for _, v := range pacotesForaDaPagina(caminhos, string(pagina)) {
		t.Error(v)
	}
}

// Regra: a conferência da página acusa o pacote que falta e o que saiu, sem confundir um pacote com o de dentro dele.
func TestPacotesForaDaPagina(t *testing.T) {
	casos := []struct {
		nome     string
		pacotes  []string
		pagina   string
		mensagem []string
	}{
		{"todos citados", []string{"internal/plataforma/banco", "internal/plataforma/banco/consultas"},
			"| Base | `internal/plataforma/banco`, `internal/plataforma/banco/consultas` |", nil},
		{"falta um", []string{"internal/plataforma/banco", "internal/novo"}, "`internal/plataforma/banco`",
			[]string{"diagrama de Componentes: internal/novo não está"}},
		{"o de dentro não conta pelo de fora", []string{"internal/plataforma/banco"}, "`internal/plataforma/banco/consultas`", []string{
			"diagrama de Componentes: docs/arquitetura/componentes.md cita internal/plataforma/banco/consultas",
			"diagrama de Componentes: internal/plataforma/banco não está",
		}},
		{"arquivo não é pacote", []string{"internal/x"}, "`internal/x` e `internal/x/y_test.go`", nil},
		{"saiu do código", []string{"cmd/financas"}, "`cmd/financas` e `internal/velho`",
			[]string{"diagrama de Componentes: docs/arquitetura/componentes.md cita internal/velho"}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			v := pacotesForaDaPagina(c.pacotes, c.pagina)
			if len(v) != len(c.mensagem) {
				t.Fatalf("violações: %q\nquer %d", v, len(c.mensagem))
			}
			for i, m := range c.mensagem {
				if !strings.HasPrefix(v[i], m) || !strings.Contains(v[i], "O que fazer:") {
					t.Errorf("violação %d: %q\nquer começando com %q e com o que fazer", i, v[i], m)
				}
			}
		})
	}
}

// Regra: cada regra falha com um import proibido de mentira, e a mensagem diz a regra e o que fazer.
func TestCadaRegraFalha(t *testing.T) {
	modulos := []string{"internal/modulos/trabalho", "internal/modulos/trabalho/holerite", "internal/modulos/noticias"}
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
	if v := violacoes(mudar("internal/modulos/trabalho", func(p *pacote) { p.Imports = []string{"internal/movimentacao/transacoes"} })); len(v) > 0 {
		t.Errorf("módulo usando um assunto do núcleo: %v", v)
	}
	// E o módulo usa os assuntos dele.
	if v := violacoes(mudar("internal/modulos/trabalho", func(p *pacote) { p.Imports = []string{"internal/modulos/trabalho/holerite"} })); len(v) > 0 {
		t.Errorf("módulo usando um assunto dele: %v", v)
	}
	casos := []struct {
		nome     string
		grafo    []pacote
		mensagem string
	}{
		{"driver fora do banco", mudar("internal/patrimonio/carteira", func(p *pacote) { p.Imports = []string{driverDoBanco} }),
			"regra 1 (só o internal/plataforma/banco abre o SQLite): internal/patrimonio/carteira importa modernc.org/sqlite"},
		{"apoio no programa", mudar("internal/movimentacao/contas", func(p *pacote) { p.Deps = []string{apoioAosTestes} }),
			"regra 2 (o internal/plataforma/testeapoio é só para _test.go): internal/movimentacao/contas chega a ele"},
		{"regra com HTTP", mudar("internal/movimentacao/transacoes", func(p *pacote) {
			p.Imports, p.Deps = []string{"internal/plataforma/respostas"}, []string{"internal/plataforma/respostas", "net/http"}
		}), "regra 3 (a regra de negócio não conhece HTTP): internal/movimentacao/transacoes chega a net/http, internal/plataforma/respostas"},
		{"exceção que sarou", mudar("internal/plataforma/tarefas", func(p *pacote) { p.Deps = nil }),
			"regra 3: internal/plataforma/tarefas não chega mais ao HTTP"},
		{"pacote sem grupo", mudar("internal/novo", func(*pacote) {}),
			"regra 4: internal/novo não está em nenhum grupo"},
		{"assunto fora do mapa", mudar("internal/patrimonio/selic", func(p *pacote) { p.Imports = []string{"internal/patrimonio/carteira"} }),
			"regra 4 (mapa de contextos): internal/patrimonio/selic importa internal/patrimonio/carteira"},
		{"módulo importa módulo", mudar("internal/modulos/noticias", func(p *pacote) { p.Imports = []string{"internal/modulos/trabalho/holerite"} }),
			"regra 5 (um módulo não depende de outro): internal/modulos/noticias importa internal/modulos/trabalho/holerite"},
		{"núcleo importa módulo", mudar("internal/movimentacao/contas", func(p *pacote) { p.Imports = []string{"internal/modulos/trabalho"} }),
			"regra 5 (o núcleo não depende de módulo): internal/movimentacao/contas importa o módulo internal/modulos/trabalho"},
		{"núcleo dos módulos importa módulo", mudar("internal/plataforma/modulos", func(p *pacote) { p.Imports = []string{"internal/modulos/noticias"} }),
			"regra 5 (o núcleo não depende de módulo): internal/plataforma/modulos importa o módulo internal/modulos/noticias"},
		{"base que depende de assunto", mudar("internal/plataforma/tipos", func(p *pacote) { p.Imports = []string{"internal/movimentacao/contas"} }),
			"regra 4: internal/plataforma/tipos (base) importa o assunto internal/movimentacao/contas"},
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
