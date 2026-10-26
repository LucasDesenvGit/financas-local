package erros

// Testes que o teste de mutação (#373) mostrou que faltavam (#401): em cada um, a mutação que ele pega.

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
)

// Regra: na query, valor só com espaço é nulo (data, texto e nível), o tamanho 1 vale, um "&" vazio não corta o
// resto, o parâmetro com escape inválido é ignorado, e a lista de números de nível também se junta.
// Pega: os "!ok ||" das datas e do texto virando "&&", o "ok &&" do nível virando "||", o "< 1" virando "<= 1", o
// "continue" do par vazio virando "break", o "||" dos erros de escape virando "&&" e o "total |= n" dos números.
func TestLerFiltroBordas(t *testing.T) {
	casos := []struct {
		consulta string
		erro     string // "" é aceito
	}{
		{"inicio=%20&fim=%20", ""},
		{"situacao=%20&origem=%20", ""},
		{"nivel=%20", ""},
		{"tamanhoPagina=1", ""},
		{"&pagina=0", "Pagina"},
		{"&&tamanhoPagina=0", "TamanhoPagina"},
		{"pagina=%zz", ""},
		{"pagina=%zz&tamanhoPagina=0", "TamanhoPagina"}, // o estragado é pulado, e o que vem depois é lido
		{"nivel=3,4", "Nivel"},
	}
	for _, c := range casos {
		_, erros := lerFiltro(c.consulta)
		obtido := ""
		for chave := range erros {
			obtido = chave
		}
		if obtido != c.erro || len(erros) > 1 {
			t.Errorf("%s: erros %v, esperado só %q", c.consulta, erros, c.erro)
		}
	}
}

// Regra: a lista filtra pela origem e pula as páginas anteriores.
// Pega: a condição da origem invertida e o "*" do Skip virando "/".
func TestListarPorOrigemEPagina(t *testing.T) {
	_, mux := rotasComErros(t)
	ids := func(caminho string) []int64 {
		t.Helper()
		r := testeapoio.Chamar(t, mux, "GET", caminho, "")
		r.ConferirStatus(t, http.StatusOK)
		var p struct {
			Itens []struct {
				Id int64 `json:"id"`
			} `json:"itens"`
		}
		if err := json.Unmarshal(r.Corpo, &p); err != nil {
			t.Fatal(err)
		}
		var lista []int64
		for _, i := range p.Itens {
			lista = append(lista, i.Id)
		}
		return lista
	}
	if got := ids("/api/erros?situacao=todos&origem=C%C3%A2mbio"); len(got) != 1 || got[0] != 2 {
		t.Errorf("origem Câmbio: %v, esperado [2]", got)
	}
	// Do mais recente para o mais antigo: o 2 (03/10) e o 1 (02/10); a página 2 de 1 item é o 1.
	if got := ids("/api/erros?situacao=todos&tamanhoPagina=1&pagina=2"); len(got) != 1 || got[0] != 1 {
		t.Errorf("página 2: %v, esperado [1]", got)
	}
	// Com 2 por página, a página 2 pula os 2: (2-1)*2, e não (2-1)/2.
	if got := ids("/api/erros?situacao=todos&tamanhoPagina=2&pagina=2"); len(got) != 0 {
		t.Errorf("página 2 de 2 itens: %v, esperado vazia", got)
	}
}

// Regra: o lote tem no máximo 100 registros; o resto fica para a próxima volta.
// Pega: "< maximoPorLote" virando "<=".
func TestLoteDeNoMaximo100(t *testing.T) {
	fila := NovaFila()
	for range 150 {
		fila.Registrar(Registro{Mensagem: "x"})
	}
	g := NovoGravador(nil, fila, testeapoio.RelogioDosTestes(), parametrosFixos(90), slog.New(slog.DiscardHandler))
	if lote := g.lerLote(nil); len(lote) != maximoPorLote || len(fila.canal) != 50 {
		t.Errorf("lote %d, na fila %d; esperado 100 e 50", len(lote), len(fila.canal))
	}
}

// Regra: dois erros diferentes no mesmo lote viram duas linhas, e as duas são gravadas.
// Pega: o "&&" da procura no lote virando "||" (agrupava erros diferentes) e o "if err != nil" do salvar invertido
// (só a primeira linha era gravada).
func TestLoteComDoisErrosDiferentes(t *testing.T) {
	db := testeapoio.NovoBanco(t)
	t0 := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	gravar(t, db,
		Registro{Em: t0, Nivel: Erro, Origem: "API", Categoria: "c", Mensagem: "primeiro"},
		Registro{Em: t0, Nivel: Erro, Origem: "API", Categoria: "c", Mensagem: "segundo"})
	if l := linhasDaTabela(t, db); len(l) != 2 {
		t.Errorf("%d linhas, esperado 2:\n%s", len(l), strings.Join(l, "\n"))
	}
}

// Regra: gravar e limpar sem erro não escrevem nada no log.
// Pega: os "err != nil &&" do gravarLote e do limparSeVenceu virando "||" ou invertidos (logavam "não foi possível"
// quando dava certo).
func TestGravarELimparSemErroNaoLogam(t *testing.T) {
	db := testeapoio.NovoBanco(t)
	var log []string
	g := NovoGravador(db, NovaFila(), testeapoio.RelogioDosTestes(), parametrosFixos(90), slog.New(gravadorDeTexto{saida: &log}))
	g.gravarLote(t.Context(), []Registro{{Em: testeapoio.InstanteDosTestes, Nivel: Erro, Origem: "API", Categoria: "c", Mensagem: "x"}})
	g.limparSeVenceu(t.Context())
	if len(log) != 0 {
		t.Errorf("logou sem erro: %v", log)
	}
}

// Regra: atributo que não é erro vai para a mensagem (e não há detalhe); o erro simples não ganha a linha da cadeia;
// o do errors.Join tem a cadeia com o erro de dentro.
// Pega: o "ok &&" do atributo error virando "||", os "> 0" e "> 1" do detalhe e da cadeia virando ">=", e o
// "len(lista) > 0" do Unwrap do Join invertido.
func TestDetalheDoLog(t *testing.T) {
	log, _, lidos := capturar(t)
	type ponto struct{ X int }
	log.Warn("sem erro", "dados", ponto{1})
	log.Error("simples", "erro", errors.New("falhou"))
	log.Error("juntos", "erro", errors.Join(errors.New("a"), errors.New("b")))
	r := lidos()
	if r[0].Mensagem != `sem erro (dados={"X":1})` || r[0].Detalhe != nil {
		t.Errorf("sem erro: %q, detalhe %v", r[0].Mensagem, r[0].Detalhe)
	}
	if r[1].Detalhe == nil || *r[1].Detalhe != "*errors.errorString: falhou" {
		t.Errorf("simples: %v", r[1].Detalhe)
	}
	if r[2].Detalhe == nil || *r[2].Detalhe != "*errors.joinError: a\nb\n   cadeia: *errors.joinError → *errors.errorString" {
		t.Errorf("juntos: %q", *r[2].Detalhe)
	}
}
