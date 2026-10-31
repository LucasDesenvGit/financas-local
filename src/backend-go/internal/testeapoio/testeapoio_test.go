package testeapoio

import (
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/sessao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Exemplo de teste de integração (banco, fábrica e consulta), o critério de aceite da #369: tem de rodar bem abaixo
// de 1 s. A soma é em Go, com decimal (dinheiro é TEXT no banco: nada de SUM no SQL).
func TestExemploDeIntegracao(t *testing.T) {
	t.Parallel()
	inicio := time.Now() // mede o próprio tempo, que é o assunto aqui (exceção da ADR 0051)
	db := NovoBancoVazio(t)
	fab := NovaFabrica(t, db, HojeEmBrasilia(2026, 10, 22))

	mercado := fab.Categoria(func(c *Categoria) { c.Nome = "Mercado" })
	conta := fab.Conta()
	fab.Transacao(conta, func(tr *Transacao) { tr.Valor = Valor("-12.50"); tr.CategoriaId = &mercado.Id })
	fab.Transacao(conta, func(tr *Transacao) { tr.Valor = Valor("-0.125"); tr.CategoriaId = &mercado.Id })
	fab.Transacao(conta, func(tr *Transacao) { tr.Valor = Valor("3000.00"); tr.Tipo = 2 }) // receita, sem categoria

	linhas, err := db.QueryContext(t.Context(), `SELECT Valor FROM Transacao WHERE CategoriaId = ? ORDER BY Id`, mercado.Id)
	if err != nil {
		t.Fatal(err)
	}
	defer linhas.Close()
	total := tipos.Zero.Decimal()
	for linhas.Next() {
		var valor tipos.Decimal
		if err := linhas.Scan(&valor); err != nil {
			t.Fatal(err)
		}
		total = total.Add(valor.Decimal())
	}
	if err := linhas.Err(); err != nil {
		t.Fatal(err)
	}
	if obtido := tipos.DeDecimal(total).String(); obtido != "-12.625" {
		t.Errorf("total do Mercado = %s, esperado -12.625", obtido)
	}
	decorrido := time.Since(inicio)
	t.Logf("banco temporário + fábrica + consulta: %v", decorrido)
	// Na CI, o tempo depende da máquina e dos outros testes paralelos: com -race (Linux), o SQLite em Go puro fica
	// dezenas de vezes mais lento; no Windows (#459, sem -race), o disco do runner chegou a 1,4 s. Lá, a trava de tempo
	// é a da #371. Na máquina de quem desenvolve, sem -race, o exemplo leva uns 5 ms.
	if !comRace && os.Getenv("GITHUB_ACTIONS") == "" && decorrido > time.Second {
		t.Errorf("o exemplo levou %v, esperado menos de 1 s", decorrido)
	}
}

func TestNovoBancoComEsquemaEDadosIniciais(t *testing.T) {
	t.Parallel()
	db := NovoBanco(t)
	var categorias int
	if err := db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM Categoria`).Scan(&categorias); err != nil {
		t.Fatal(err)
	}
	if categorias == 0 {
		t.Error("NovoBanco deveria vir com as categorias padrão do 02-dados-iniciais.sql")
	}
	// A coluna nova do 2-criar-banco.sh também tem de existir (o esquema é o real, não só o 01-estrutura.sql).
	if _, err := db.ExecContext(t.Context(), `SELECT DescricaoManual, RegraDescricaoId FROM Transacao`); err != nil {
		t.Errorf("as colunas novas do 2-criar-banco.sh não estão no banco: %v", err)
	}
	var chaves int
	if err := db.QueryRowContext(t.Context(), `PRAGMA foreign_keys`).Scan(&chaves); err != nil || chaves != 1 {
		t.Errorf("PRAGMA foreign_keys = %d (%v), esperado 1", chaves, err)
	}
}

func TestNovoBancoVazioComecaDoId1(t *testing.T) {
	t.Parallel()
	db := NovoBancoVazio(t)
	for _, tabela := range tabelasDosDadosIniciais {
		var n int
		if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM "+tabela).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Errorf("%s tem %d linhas, esperado 0", tabela, n)
		}
	}
	fab := NovaFabrica(t, db, RelogioDosTestes())
	categoria := fab.Categoria()
	if regra := fab.RegraCategorizacao(categoria); categoria.Id != 1 || regra.Id != 1 {
		t.Errorf("Ids = categoria %d, regra %d; esperado 1 e 1", categoria.Id, regra.Id)
	}
}

// Cada teste paralelo tem o próprio arquivo: gravar num não aparece no outro.
func TestBancosParalelosNaoSeMisturam(t *testing.T) {
	t.Parallel()
	for i := range 4 {
		t.Run("banco "+strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()
			db := NovoBancoVazio(t)
			fab := NovaFabrica(t, db, RelogioDosTestes())
			for range i + 1 {
				fab.Categoria()
			}
			var n int
			if err := db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM Categoria`).Scan(&n); err != nil {
				t.Fatal(err)
			}
			if n != i+1 {
				t.Errorf("Categoria tem %d linhas, esperado %d", n, i+1)
			}
		})
	}
}

// A fábrica grava no formato do EF Core: é o que o .NET lê do mesmo arquivo (ADR 0049).
func TestFabricaGravaNoFormatoDoEfCore(t *testing.T) {
	t.Parallel()
	db := NovoBancoVazio(t)
	fab := NovaFabrica(t, db, RelogioDosTestes())
	grupo, err := tipos.LerGuid("2f00defb-672e-45ef-87d0-702995ae8d2e")
	if err != nil {
		t.Fatal(err)
	}
	conta := fab.Conta(func(c *Conta) {
		c.Tipo = 3
		c.LimiteCredito = new(Valor("5000"))
		c.LimiteAjustadoNoApp = new(false)
	})
	tr := fab.Transacao(conta, func(tr *Transacao) { tr.Valor = Valor("-1.50"); tr.TransferenciaGrupo = &grupo })

	var saldo, limite, ajustado, data, valor, guid, criadoEm, ignorar string
	if err := db.QueryRowContext(t.Context(), `SELECT c.Saldo, c.LimiteCredito, c.LimiteAjustadoNoApp, t.Data,
		t.Valor, t.TransferenciaGrupo, t.CriadoEm, t.IgnorarRelatorios
		FROM Transacao t JOIN Conta c ON c.Id = t.ContaId WHERE t.Id = ?`, tr.Id).
		Scan(&saldo, &limite, &ajustado, &data, &valor, &guid, &criadoEm, &ignorar); err != nil {
		t.Fatal(err)
	}
	casos := []struct{ coluna, obtido, esperado string }{
		{"Conta.Saldo (decimal zero)", saldo, "0.0"},
		{"Conta.LimiteCredito (decimal sem casas)", limite, "5000.0"},
		{"Conta.LimiteAjustadoNoApp (bool? false)", ajustado, "0"},
		{"Transacao.Data (hoje em Brasília, não em UTC)", data, "2026-10-22"},
		{"Transacao.Valor (sem zero à direita)", valor, "-1.5"},
		{"Transacao.TransferenciaGrupo (GUID em maiúsculas)", guid, "2F00DEFB-672E-45EF-87D0-702995AE8D2E"},
		{"Transacao.CriadoEm (UTC, sem fração zero)", criadoEm, "2026-10-23 02:30:00"},
		{"Transacao.IgnorarRelatorios (bool)", ignorar, "0"},
	}
	for _, c := range casos {
		if c.obtido != c.esperado {
			t.Errorf("%s = %q, esperado %q", c.coluna, c.obtido, c.esperado)
		}
	}
}

func TestFabricaNulosEAjustes(t *testing.T) {
	t.Parallel()
	db := NovoBancoVazio(t)
	fab := NovaFabrica(t, db, RelogioDosTestes())
	pai := fab.Categoria(func(c *Categoria) { c.Tipo = 2 })
	filha := fab.Subcategoria(pai, func(c *Categoria) { c.Ativa = false })
	if filha.Tipo != 2 || filha.CategoriaPaiId == nil || *filha.CategoriaPaiId != pai.Id {
		t.Errorf("Subcategoria = %+v, esperado filha de %d com o tipo 2 do pai", filha, pai.Id)
	}
	fab.RegraCategorizacao(filha, func(r *RegraCategorizacao) { r.Texto = "PADARIA"; r.TipoCorrespondencia = 3 })

	var paiId, ativa, natureza, texto, tipo any
	if err := db.QueryRowContext(t.Context(), `SELECT c.CategoriaPaiId, c.Ativa, c.Natureza, r.Texto, r.TipoCorrespondencia
		FROM RegraCategorizacao r JOIN Categoria c ON c.Id = r.CategoriaId`).Scan(&paiId, &ativa, &natureza, &texto, &tipo); err != nil {
		t.Fatal(err)
	}
	if paiId != pai.Id || ativa != int64(0) || natureza != nil || texto != "PADARIA" || tipo != int64(3) {
		t.Errorf("gravado: pai %v, ativa %v, natureza %v, texto %v, tipo %v; esperado %d, 0, NULL, PADARIA, 3",
			paiId, ativa, natureza, texto, tipo, pai.Id)
	}
}

// A chave estrangeira está ligada: a fábrica não esconde erro de dado inconsistente.
func TestFabricaFalhaComChaveEstrangeira(t *testing.T) {
	t.Parallel()
	db := NovoBancoVazio(t)
	falso := &tbFalso{TB: t}
	func() {
		defer func() { _ = recover() }() // o Fatalf do falso para a fábrica com um pânico
		NovaFabrica(falso, db, RelogioDosTestes()).Transacao(Conta{Id: 999})
	}()
	if !strings.Contains(falso.mensagens, "FOREIGN KEY") {
		t.Errorf("Transacao numa conta que não existe: mensagem = %q, esperado o erro de FOREIGN KEY", falso.mensagens)
	}
}

// tbFalso é um testing.TB que guarda as mensagens de Errorf e Fatalf em vez de falhar o teste de verdade. O Fatalf
// entra em pânico para parar quem chamou, como o FailNow para o teste.
type tbFalso struct {
	testing.TB
	mensagens string
}

func (f *tbFalso) Helper() {}
func (f *tbFalso) Errorf(formato string, argumentos ...any) {
	f.mensagens += fmt.Sprintf(formato, argumentos...) + "\n"
}
func (f *tbFalso) Fatalf(formato string, argumentos ...any) {
	f.Errorf(formato, argumentos...)
	panic("Fatalf")
}

func TestRelogio(t *testing.T) {
	casos := []struct {
		nome     string
		relogio  relogio.Relogio
		hoje     string
		instante string // em UTC
	}{
		{"o relógio dos testes já é o dia seguinte em UTC", RelogioDosTestes(), "2026-10-22", "2026-10-23T02:30:00Z"},
		{"hoje em Brasília é ao meio-dia", HojeEmBrasilia(2026, 10, 15), "2026-10-15", "2026-10-15T15:00:00Z"},
		{"23h30 em Brasília", AgoraEmBrasilia(2026, 12, 31, 23, 30), "2026-12-31", "2027-01-01T02:30:00Z"},
		{"meia-noite em Brasília", AgoraEmBrasilia(2027, 1, 1, 0, 0), "2027-01-01", "2027-01-01T03:00:00Z"},
		{"horário de verão antigo (até 2019)", HojeEmBrasilia(2018, 12, 1), "2018-12-01", "2018-12-01T14:00:00Z"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if hoje := relogio.Hoje(c.relogio).String(); hoje != c.hoje {
				t.Errorf("Hoje = %s, esperado %s", hoje, c.hoje)
			}
			if instante := c.relogio.Agora().Format(time.RFC3339); instante != c.instante {
				t.Errorf("Agora = %s, esperado %s", instante, c.instante)
			}
		})
	}
	if d := Dia(2024, 2, 29).String(); d != "2024-02-29" {
		t.Errorf("Dia(2024, 2, 29) = %s, esperado 2024-02-29", d)
	}
}

func TestValor(t *testing.T) {
	if v := Valor("10.50"); v.String() != "10.50" || v.TextoNoBanco() != "10.5" {
		t.Errorf("Valor(10.50) = %s (no banco %s), esperado 10.50 (no banco 10.5)", v, v.TextoNoBanco())
	}
	defer func() {
		if recover() == nil {
			t.Error("Valor(\"dez\") deveria entrar em pânico")
		}
	}()
	Valor("dez")
}

func TestChamarComUsuario(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/eco", func(w http.ResponseWriter, r *http.Request) {
		id, ok := sessao.Usuario(r.Context())
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var entrada struct {
			Valor tipos.Decimal `json:"valor"`
		}
		if err := json.NewDecoder(r.Body).Decode(&entrada); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"usuario": id, "valor": entrada.Valor, "tipo": r.Header.Get("Content-Type"), "itens": []int{},
		})
	})

	resposta := Chamar(t, mux, "POST", "/api/eco", `{"valor":10.50}`)
	resposta.ConferirStatus(t, http.StatusOK)
	resposta.ConferirJSON(t, `{"usuario":1,"valor":10.50,"tipo":"application/json","itens":[]}`)

	outro := ChamarComo(t, mux, 7, "POST", "/api/eco", `{"valor":1}`)
	outro.ConferirJSON(t, `{"usuario":7,"valor":1,"tipo":"application/json","itens":[]}`)

	ChamarComo(t, mux, 0, "POST", "/api/eco", `{}`).ConferirStatus(t, http.StatusUnauthorized)
}

// O comparador é o do contrato: estas diferenças têm de falhar o teste.
func TestConferirJSONPegaAsDiferencas(t *testing.T) {
	t.Parallel()
	casos := []struct{ nome, esperado, obtido, mensagem string }{
		{"escala do decimal", `{"valor":10.50}`, `{"valor":10.5}`, "/valor: esperado número 10.50, obtido número 10.5"},
		{"lista vazia e null", `{"itens":[]}`, `{"itens":null}`, "/itens: esperado lista com 0 itens, obtido null"},
		{"null e ausente", `{"a":null}`, `{}`, "/a: esperado null, obtido ausente"},
		{"JSON inválido", `{}`, `{`, "JSON obtido inválido"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			falso := &tbFalso{TB: t}
			ConferirJSON(falso, []byte(c.obtido), c.esperado)
			if !strings.Contains(falso.mensagens, c.mensagem) {
				t.Errorf("ConferirJSON(%s, %s): erro = %q, esperado conter %q", c.obtido, c.esperado, falso.mensagens, c.mensagem)
			}
		})
	}
	ConferirJSON(t, []byte(`{"b":[1.0,null],"a":"x"}`), `{"a":"x","b":[1.0,null]}`) // ordem das chaves não importa
}

// O apoio importa testing e não pode entrar no executável: só arquivos _test.go o importam. O go build já ignora
// os _test.go; este teste pega quem importar o pacote de um arquivo comum.
func TestSoImportadoPorTestes(t *testing.T) {
	t.Parallel()
	const caminho = "github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	raiz := filepath.Join("..", "..") // src/backend-go
	err := filepath.WalkDir(raiz, func(arquivo string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "testdata" || d.Name() == "testeapoio") {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(arquivo, ".go") || strings.HasSuffix(arquivo, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), arquivo, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			if strings.Trim(imp.Path.Value, `"`) == caminho {
				t.Errorf("%s importa o testeapoio: só arquivos _test.go podem importar", arquivo)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
