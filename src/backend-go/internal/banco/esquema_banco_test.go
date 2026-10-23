package banco

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/esquema"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/esquema/embutido"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
)

// O gabarito do .NET (testdata/esquema-dotnet.json) é escrito pela sonda testdata/sonda-esquema, que compila o mesmo
// Data/EsquemaBanco.cs da API e roda os cenários abaixo. Os scripts com que ele foi feito ficam em
// testdata/entradas-esquema/*.gz: o Go aplica exatamente esses, e o teste não fica velho quando o banco/ muda.

type gabaritoEsquema struct {
	Pragmas        map[string]string `json:"pragmas"`
	Entradas       map[string]string `json:"entradas"`
	Assinaturas    map[string]int32  `json:"assinaturas"`
	TextoDeLeitura string            `json:"textoDeLeitura"`
	Leitura        []string          `json:"leitura"`
	Cenarios       struct {
		Novo struct {
			Banco  despejo  `json:"banco"`
			Copias []string `json:"copias"`
		} `json:"novo"`
		SemMudanca struct {
			ArquivoIgual bool     `json:"arquivoIgual"`
			Copias       []string `json:"copias"`
		} `json:"semMudanca"`
		Vazio struct {
			Banco  resumo   `json:"banco"`
			Copias []string `json:"copias"`
		} `json:"vazio"`
		Antigo struct {
			Antes                resumo   `json:"antes"`
			Depois               resumo   `json:"depois"`
			Copia                resumo   `json:"copia"`
			NomeDaCopiaNoFormato bool     `json:"nomeDaCopiaNoFormato"`
			Copias               []string `json:"copias"`
		} `json:"antigo"`
		Rodizio struct {
			Arquivos  []string `json:"arquivos"`
			Pastas    []string `json:"pastas"`
			Restantes []string `json:"restantes"`
		} `json:"rodizio"`
		PastaSQLMaiuscula struct {
			Copias []string `json:"copias"`
		} `json:"pastaSqlMaiuscula"`
		PastaSemSQL struct {
			Copias []string `json:"copias"`
		} `json:"pastaSemSql"`
	} `json:"cenarios"`
}

// despejo é a estrutura e os dados de um banco, montados pelas mesmas consultas da sonda (Despejar, no Program.cs).
type despejo struct {
	UserVersion int64             `json:"userVersion"`
	Tabelas     []tabelaDespejada `json:"tabelas"`
	Outros      []string          `json:"outros"`
}

type tabelaDespejada struct {
	Nome               string   `json:"nome"`
	SQLHash            string   `json:"sqlHash"`
	Colunas            []string `json:"colunas"` // cid|nome|tipo|notnull|padrão|pk
	Indices            []string `json:"indices"` // nome|unique|origem|parcial|colunas|hash do SQL
	ChavesEstrangeiras []string `json:"chavesEstrangeiras"`
	Linhas             int      `json:"linhas"`
	DadosHash          string   `json:"dadosHash"`
}

// resumo é o despejo com um hash por tabela (Resumir, no Program.cs).
type resumo struct {
	UserVersion int64             `json:"userVersion"`
	Tabelas     map[string]string `json:"tabelas"`
	Outros      []string          `json:"outros"`
}

// instanteDosTestes é 23/10/2026 02:30 em UTC, 22/10/2026 23:30 em Brasília: o dia muda com o fuso.
var instanteDosTestes = relogio.Fixo{Instante: time.Date(2026, 10, 23, 2, 30, 0, 0, time.UTC)}

func semLog() *slog.Logger { return slog.New(slog.DiscardHandler) }

func lerGabaritoEsquema(t *testing.T) (gabaritoEsquema, esquema.Scripts) {
	t.Helper()
	dados, err := os.ReadFile("testdata/esquema-dotnet.json")
	if err != nil {
		t.Fatal(err)
	}
	var g gabaritoEsquema
	if err := json.Unmarshal(dados, &g); err != nil {
		t.Fatal(err)
	}
	entrada := func(nome string) string {
		arquivo, err := os.Open(filepath.Join("testdata", "entradas-esquema", nome+".gz"))
		if err != nil {
			t.Fatal(err)
		}
		defer arquivo.Close()
		leitor, err := gzip.NewReader(arquivo)
		if err != nil {
			t.Fatal(err)
		}
		conteudo, err := io.ReadAll(leitor)
		if err != nil {
			t.Fatal(err)
		}
		if got := hashTexto(conteudo); got != g.Entradas[nome] {
			t.Fatalf("entradas-esquema/%s.gz não é o arquivo do gabarito (sha256 %s, esperado %s): rode a sonda de novo", nome, got, g.Entradas[nome])
		}
		return esquema.Texto(conteudo)
	}
	return g, esquema.Scripts{
		Estrutura:     entrada("01-estrutura.sql"),
		DadosIniciais: entrada("02-dados-iniciais.sql"),
		CriarBanco:    entrada("2-criar-banco.sh"),
	}
}

func hashTexto[T string | []byte](conteudo T) string {
	soma := sha256.Sum256([]byte(conteudo))
	return hex.EncodeToString(soma[:])
}

// ------------------------------------------------------------------------------------------- assinatura e leitura

func TestAssinaturaIgualAoDotnet(t *testing.T) {
	g, scripts := lerGabaritoEsquema(t)
	colunas := esquema.LerColunasNovas(scripts.CriarBanco)
	casos := map[string]int32{
		"scripts":        esquema.Assinatura(scripts.Estrutura, colunas),
		"scriptsComCrlf": esquema.Assinatura(strings.ReplaceAll(scripts.Estrutura, "\n", "\r\n"), colunas),
		"vazio":          esquema.Assinatura("", nil),
		"acentos": esquema.Assinatura("CREATE TABLE Ação (Descrição TEXT);\r\n", []esquema.ColunaNova{
			{Tabela: "A", Coluna: "B", Definicao: "TEXT NULL"},
			{Tabela: "C", Coluna: "D", Definicao: "TEXT NOT NULL DEFAULT 'é'"},
		}),
	}
	if len(casos) != len(g.Assinaturas) {
		t.Fatalf("o gabarito tem %d assinaturas e o teste conhece %d: a sonda mudou?", len(g.Assinaturas), len(casos))
	}
	for nome, got := range casos {
		if want, ok := g.Assinaturas[nome]; !ok || got != want {
			t.Errorf("assinatura %q = %d, o .NET calcula %d", nome, got, want)
		}
	}
}

func TestLeituraDasLinhasIgualAoDotnet(t *testing.T) {
	g, _ := lerGabaritoEsquema(t)
	var got []string
	for _, c := range esquema.LerColunasNovas(g.TextoDeLeitura) {
		got = append(got, c.Tabela+"|"+c.Coluna+"|"+c.Definicao)
	}
	if !slices.Equal(got, g.Leitura) {
		t.Errorf("linhas lidas pelo Go:\n%q\npelo .NET:\n%q", got, g.Leitura)
	}
}

func TestTextoTiraOBOM(t *testing.T) {
	if got := esquema.Texto([]byte("\xEF\xBB\xBFCREATE")); got != "CREATE" {
		t.Errorf("Texto = %q, o StreamReader do .NET tira o BOM", got)
	}
}

// ------------------------------------------------------------------------------------------- cópias embutidas

func TestCopiasEmDia(t *testing.T) {
	embutidos := map[string][]byte{
		"01-estrutura.sql":      embutido.Estrutura,
		"02-dados-iniciais.sql": embutido.DadosIniciais,
		"2-criar-banco.sh":      embutido.CriarBanco,
	}
	if len(embutidos) != len(esquema.Copias) {
		t.Fatalf("o embutido tem %d arquivos e o esquema.Copias %d", len(embutidos), len(esquema.Copias))
	}
	for copia, origem := range esquema.Copias {
		original, err := os.ReadFile(filepath.Join(raiz, origem))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(embutidos[copia], original) {
			t.Errorf("a cópia embutida de %s está velha: rode `go generate ./...` em src/backend-go", origem)
		}
		// O StreamReader do .NET troca bytes inválidos por U+FFFD, e a assinatura mudaria entre os dois lados.
		if !utf8.Valid(original) {
			t.Errorf("%s não é UTF-8 válido", origem)
		}
	}
}

// ------------------------------------------------------------------------------------------- cenários do .NET

func TestEsquemaIgualAoDotnet(t *testing.T) {
	g, scripts := lerGabaritoEsquema(t)
	ctx := context.Background()
	pasta := t.TempDir()
	aplicar := func(t *testing.T, caminho string) {
		t.Helper()
		if err := aplicarEsquema(ctx, caminho, scripts, instanteDosTestes, semLog()); err != nil {
			t.Fatal(err)
		}
	}
	novo := filepath.Join(pasta, "novo", "dados", "sql", "FinancasLocal.db")

	t.Run("novo", func(t *testing.T) {
		aplicar(t, novo)
		compararDespejos(t, despejar(t, novo), g.Cenarios.Novo.Banco)
		compararCopias(t, filepath.Join(pasta, "novo"), g.Cenarios.Novo.Copias)
	})

	t.Run("semMudanca", func(t *testing.T) {
		antes := impressao(t, novo)
		time.Sleep(20 * time.Millisecond) // para uma gravação mudar a data do arquivo
		aplicar(t, novo)
		if igual := impressao(t, novo) == antes; igual != g.Cenarios.SemMudanca.ArquivoIgual {
			t.Errorf("arquivo igual depois de aplicar de novo = %v, no .NET %v", igual, g.Cenarios.SemMudanca.ArquivoIgual)
		}
		compararCopias(t, filepath.Join(pasta, "novo"), g.Cenarios.SemMudanca.Copias)
	})

	t.Run("vazio", func(t *testing.T) {
		vazio := filepath.Join(pasta, "vazio", "dados", "sql", "FinancasLocal.db")
		copiarBanco(t, os.DevNull, vazio)
		aplicar(t, vazio)
		compararResumos(t, "vazio", resumir(despejar(t, vazio)), g.Cenarios.Vazio.Banco)
		compararCopias(t, filepath.Join(pasta, "vazio"), g.Cenarios.Vazio.Copias)
	})

	t.Run("antigo", func(t *testing.T) {
		antigo := filepath.Join(pasta, "antigo", "dados", "sql", "FinancasLocal.db")
		copiarBanco(t, novo, antigo)
		executar(t, antigo, "ALTER TABLE Meta DROP COLUMN Icone; ALTER TABLE RegraDescricao DROP COLUMN ValorMaximo; PRAGMA user_version = 7;")
		compararResumos(t, "antes", resumir(despejar(t, antigo)), g.Cenarios.Antigo.Antes)
		aplicar(t, antigo)
		compararResumos(t, "depois", resumir(despejar(t, antigo)), g.Cenarios.Antigo.Depois)
		copias, err := filepath.Glob(filepath.Join(pasta, "antigo", "dados", "backups", PrefixoBackup+"*.db"))
		if err != nil || len(copias) != 1 {
			t.Fatalf("esperava uma cópia, achei %v (%v)", copias, err)
		}
		compararResumos(t, "cópia", resumir(despejar(t, copias[0])), g.Cenarios.Antigo.Copia)
		formato := regexp.MustCompile(`^FinancasLocal-antes-atualizacao-\d{8}_\d{6}\.db$`).MatchString(filepath.Base(copias[0]))
		if formato != g.Cenarios.Antigo.NomeDaCopiaNoFormato {
			t.Errorf("nome da cópia %s no formato = %v, no .NET %v", filepath.Base(copias[0]), formato, g.Cenarios.Antigo.NomeDaCopiaNoFormato)
		}
		compararCopias(t, filepath.Join(pasta, "antigo"), g.Cenarios.Antigo.Copias)
	})

	t.Run("rodizio", func(t *testing.T) {
		r := g.Cenarios.Rodizio
		banco := filepath.Join(pasta, "rodizio", "dados", "sql", "FinancasLocal.db")
		copiarBanco(t, novo, banco)
		executar(t, banco, "PRAGMA user_version = 7;")
		copias := filepath.Join(pasta, "rodizio", "dados", "backups")
		if err := os.MkdirAll(copias, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, nome := range r.Arquivos {
			if err := os.WriteFile(filepath.Join(copias, nome), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		for _, nome := range r.Pastas {
			if err := os.Mkdir(filepath.Join(copias, nome), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		aplicar(t, banco)
		entradas, err := os.ReadDir(copias)
		if err != nil {
			t.Fatal(err)
		}
		restantes := []string{}
		for _, e := range entradas {
			nome := e.Name()
			if !slices.Contains(r.Arquivos, nome) && !slices.Contains(r.Pastas, nome) {
				nome = "<nova>"
			}
			restantes = append(restantes, nome)
		}
		slices.Sort(restantes)
		if !slices.Equal(restantes, r.Restantes) {
			t.Errorf("ficaram:\n%q\nno .NET:\n%q", restantes, r.Restantes)
		}
	})

	for nome, c := range map[string]struct {
		partes []string
		copias []string
	}{
		"pastaSqlMaiuscula": {[]string{"dados", "SQL", "FinancasLocal.db"}, g.Cenarios.PastaSQLMaiuscula.Copias},
		"pastaSemSql":       {[]string{"outra", "FinancasLocal.db"}, g.Cenarios.PastaSemSQL.Copias},
	} {
		t.Run(nome, func(t *testing.T) {
			banco := filepath.Join(append([]string{pasta, nome}, c.partes...)...)
			copiarBanco(t, novo, banco)
			executar(t, banco, "PRAGMA user_version = 7;")
			aplicar(t, banco)
			compararCopias(t, filepath.Join(pasta, nome), c.copias)
		})
	}

	t.Run("pragmas", func(t *testing.T) {
		// O EsquemaBanco abre com foreign_keys ligado (o SQLite do .NET já vem assim). Banco ocupado: o .NET deixa o
		// busy_timeout em 0 e tenta de novo no próprio código até o DefaultTimeout (30 s); o Go usa o busy_timeout
		// de 30 s do Abrir, o mesmo tempo.
		db, err := Abrir(ctx, novo)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		var chaves string
		if err := db.QueryRow("PRAGMA foreign_keys").Scan(&chaves); err != nil {
			t.Fatal(err)
		}
		if chaves != g.Pragmas["foreign_keys"] {
			t.Errorf("foreign_keys = %s, no .NET %s", chaves, g.Pragmas["foreign_keys"])
		}
	})
}

// despejar lê a estrutura e os dados com as mesmas consultas da sonda.
func despejar(t *testing.T, caminho string) despejo {
	t.Helper()
	db, err := Abrir(context.Background(), caminho)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	type par struct{ a, b string }
	pares := func(consulta string, args ...any) []par {
		linhas, err := db.Query(consulta, args...)
		if err != nil {
			t.Fatal(err)
		}
		defer linhas.Close()
		var lista []par
		for linhas.Next() {
			var p par
			if err := linhas.Scan(&p.a, &p.b); err != nil {
				t.Fatal(err)
			}
			lista = append(lista, p)
		}
		return lista
	}
	textos := func(consulta string, args ...any) []string {
		linhas, err := db.Query(consulta, args...)
		if err != nil {
			t.Fatal(err)
		}
		defer linhas.Close()
		lista := []string{}
		for linhas.Next() {
			var s sql.NullString
			if err := linhas.Scan(&s); err != nil {
				t.Fatal(err)
			}
			lista = append(lista, s.String)
		}
		return lista
	}

	d := despejo{Tabelas: []tabelaDespejada{}}
	for _, tabela := range pares("SELECT name, sql FROM sqlite_master WHERE type = 'table' ORDER BY name") {
		nome := tabela.a
		indices := []string{}
		for _, i := range pares(`SELECT name, "unique" || '|' || origin || '|' || partial FROM pragma_index_list(?) ORDER BY name`, nome) {
			colunas := textos("SELECT ifnull(name, '<expr>') FROM pragma_index_info(?) ORDER BY seqno", i.a)
			var sqlIndice sql.NullString
			if err := db.QueryRow("SELECT sql FROM sqlite_master WHERE type = 'index' AND name = ?", i.a).Scan(&sqlIndice); err != nil {
				t.Fatal(err)
			}
			hashIndice := "<sem sql>"
			if sqlIndice.Valid {
				hashIndice = hashTexto(sqlIndice.String)
			}
			indices = append(indices, fmt.Sprintf("%s|%s|%s|%s", i.a, i.b, strings.Join(colunas, ","), hashIndice))
		}
		nomesColunas := textos("SELECT name FROM pragma_table_info(?) ORDER BY cid", nome)
		dados := textos(fmt.Sprintf(`SELECT %s FROM "%s" ORDER BY rowid`, expressaoDaLinha(nomesColunas), nome))
		d.Tabelas = append(d.Tabelas, tabelaDespejada{
			Nome:    nome,
			SQLHash: hashTexto(tabela.b),
			Colunas: textos(`SELECT cid || '|' || name || '|' || type || '|' || "notnull" || '|' || ifnull(dflt_value, '<null>') || '|' || pk `+
				"FROM pragma_table_info(?) ORDER BY cid", nome),
			Indices: indices,
			ChavesEstrangeiras: textos(`SELECT id || '|' || seq || '|' || "table" || '|' || "from" || '|' || ifnull("to", '<null>') || '|' || `+
				`on_update || '|' || on_delete || '|' || "match" FROM pragma_foreign_key_list(?) ORDER BY id, seq`, nome),
			Linhas:    len(dados),
			DadosHash: hashTexto(strings.Join(dados, "\n")),
		})
	}
	if err := db.QueryRow("PRAGMA user_version").Scan(&d.UserVersion); err != nil {
		t.Fatal(err)
	}
	d.Outros = textos("SELECT type || '|' || name || '|' || tbl_name || '|' || ifnull(sql, '<null>') FROM sqlite_master " +
		"WHERE type NOT IN ('table', 'index') ORDER BY type, name")
	return d
}

// expressaoDaLinha é a da sonda: a linha como texto (quote() do SQLite), com a data e hora de agora dos dados
// iniciais trocada por <data-hora>.
func expressaoDaLinha(colunas []string) string {
	partes := make([]string, len(colunas))
	for i, c := range colunas {
		partes[i] = fmt.Sprintf(`CASE WHEN typeof("%[1]s") = 'text' AND "%[1]s" GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:*' `+
			`THEN '''<data-hora>''' ELSE quote("%[1]s") END`, c)
	}
	return strings.Join(partes, " || '|' || ")
}

func resumir(d despejo) resumo {
	r := resumo{UserVersion: d.UserVersion, Tabelas: map[string]string{}, Outros: d.Outros}
	for _, t := range d.Tabelas {
		linhas := []string{t.Nome, "sql:" + t.SQLHash}
		for _, c := range t.Colunas {
			linhas = append(linhas, "coluna:"+c)
		}
		for _, i := range t.Indices {
			linhas = append(linhas, "indice:"+i)
		}
		for _, f := range t.ChavesEstrangeiras {
			linhas = append(linhas, "fk:"+f)
		}
		linhas = append(linhas, fmt.Sprintf("linhas:%d", t.Linhas), "dados:"+t.DadosHash)
		r.Tabelas[t.Nome] = hashTexto(strings.Join(linhas, "\n"))
	}
	return r
}

// compararDespejos diz tabela por tabela e coluna por coluna o que difere.
func compararDespejos(t *testing.T, got, want despejo) {
	t.Helper()
	if got.UserVersion != want.UserVersion {
		t.Errorf("user_version = %d, no .NET %d", got.UserVersion, want.UserVersion)
	}
	if !slices.Equal(got.Outros, want.Outros) {
		t.Errorf("gatilhos e visões: %q, no .NET %q", got.Outros, want.Outros)
	}
	nomes := func(d despejo) []string {
		var n []string
		for _, t := range d.Tabelas {
			n = append(n, t.Nome)
		}
		return n
	}
	if !slices.Equal(nomes(got), nomes(want)) {
		t.Fatalf("tabelas:\n%q\nno .NET:\n%q", nomes(got), nomes(want))
	}
	for i, g := range got.Tabelas {
		w := want.Tabelas[i]
		for _, campo := range []struct {
			nome      string
			got, want []string
		}{
			{"colunas", g.Colunas, w.Colunas},
			{"índices", g.Indices, w.Indices},
			{"chaves estrangeiras", g.ChavesEstrangeiras, w.ChavesEstrangeiras},
		} {
			if !slices.Equal(campo.got, campo.want) {
				t.Errorf("%s, %s:\n%q\nno .NET:\n%q", g.Nome, campo.nome, campo.got, campo.want)
			}
		}
		if g.SQLHash != w.SQLHash {
			t.Errorf("%s: o CREATE TABLE guardado é outro", g.Nome)
		}
		if g.Linhas != w.Linhas || g.DadosHash != w.DadosHash {
			t.Errorf("%s: dados diferentes (%d linhas, no .NET %d)", g.Nome, g.Linhas, w.Linhas)
		}
	}
}

func compararResumos(t *testing.T, qual string, got, want resumo) {
	t.Helper()
	if got.UserVersion != want.UserVersion {
		t.Errorf("%s: user_version = %d, no .NET %d", qual, got.UserVersion, want.UserVersion)
	}
	if !slices.Equal(got.Outros, want.Outros) {
		t.Errorf("%s: gatilhos e visões %q, no .NET %q", qual, got.Outros, want.Outros)
	}
	for nome, hash := range want.Tabelas {
		if got.Tabelas[nome] != hash {
			t.Errorf("%s: a tabela %s difere do .NET", qual, nome)
		}
	}
	for nome := range got.Tabelas {
		if _, ok := want.Tabelas[nome]; !ok {
			t.Errorf("%s: a tabela %s não existe no .NET", qual, nome)
		}
	}
}

// compararCopias confere em que pastas do cenário apareceram cópias "antes da atualização" (PastasComCopia, na sonda).
func compararCopias(t *testing.T, pastaCenario string, want []string) {
	t.Helper()
	contagem := map[string]int{}
	err := filepath.WalkDir(pastaCenario, func(caminho string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !e.IsDir() && strings.HasPrefix(e.Name(), PrefixoBackup) && strings.HasSuffix(e.Name(), ".db") {
			relativa, err := filepath.Rel(pastaCenario, filepath.Dir(caminho))
			if err != nil {
				return err
			}
			contagem[filepath.ToSlash(relativa)]++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for pasta, n := range contagem {
		got = append(got, fmt.Sprintf("%s: %d", pasta, n))
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("cópias em %q, no .NET %q", got, want)
	}
}

func impressao(t *testing.T, caminho string) string {
	t.Helper()
	conteudo, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(caminho)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%s@%d", hashTexto(conteudo), info.ModTime().UnixNano())
}

func copiarBanco(t *testing.T, origem, destino string) {
	t.Helper()
	conteudo, err := os.ReadFile(origem)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(destino), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destino, conteudo, 0o644); err != nil {
		t.Fatal(err)
	}
}

func executar(t *testing.T, caminho, comandos string) {
	t.Helper()
	db, err := Abrir(context.Background(), caminho)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(comandos); err != nil {
		t.Fatal(err)
	}
}

// ------------------------------------------------------------------------------------------- com os scripts de agora

func TestAplicarEsquemaComOsScriptsEmbutidos(t *testing.T) {
	ctx := context.Background()
	caminho := filepath.Join(t.TempDir(), "dados", "sql", "FinancasLocal.db")
	if err := AplicarEsquema(ctx, caminho, instanteDosTestes, semLog()); err != nil {
		t.Fatal(err)
	}
	embutidos := esquema.Scripts{Estrutura: string(embutido.Estrutura), CriarBanco: string(embutido.CriarBanco)}
	assinatura := esquema.Assinatura(embutidos.Estrutura, esquema.LerColunasNovas(embutidos.CriarBanco))
	if d := despejar(t, caminho); d.UserVersion != int64(assinatura) {
		t.Errorf("user_version = %d, esperado %d", d.UserVersion, assinatura)
	}

	// Banco antigo: falta uma coluna nova e uma categoria foi apagada. A coluna volta, a categoria não (dados
	// iniciais só no banco novo), e fica uma cópia de antes.
	executar(t, caminho, "ALTER TABLE Meta DROP COLUMN Icone; DELETE FROM Categoria WHERE Id = (SELECT max(Id) FROM Categoria WHERE CategoriaPaiId IS NOT NULL); PRAGMA user_version = 7;")
	categorias := contar(t, caminho, "SELECT COUNT(*) FROM Categoria")
	if err := AplicarEsquema(ctx, caminho, instanteDosTestes, semLog()); err != nil {
		t.Fatal(err)
	}
	if n := contar(t, caminho, "SELECT COUNT(*) FROM pragma_table_info('Meta') WHERE name = 'Icone'"); n != 1 {
		t.Error("a coluna Meta.Icone não voltou")
	}
	if n := contar(t, caminho, "SELECT COUNT(*) FROM Categoria"); n != categorias {
		t.Errorf("%d categorias depois de atualizar, eram %d: os dados iniciais rodaram de novo", n, categorias)
	}
	copia := filepath.Join(filepath.Dir(caminho), "..", "backups", PrefixoBackup+"20261022_233000.db")
	if _, err := os.Stat(copia); err != nil {
		t.Errorf("a cópia deveria ter o nome na hora de Brasília (22/10 23:30): %v", err)
	}
}

func contar(t *testing.T, caminho, consulta string) int {
	t.Helper()
	db, err := Abrir(context.Background(), caminho)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow(consulta).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestAplicarEsquemaSemColunasNovas(t *testing.T) {
	caminho := filepath.Join(t.TempDir(), "FinancasLocal.db")
	err := aplicarEsquema(context.Background(), caminho, esquema.Scripts{Estrutura: "CREATE TABLE A (Id INTEGER);"}, instanteDosTestes, semLog())
	if err == nil || !strings.Contains(err.Error(), "nenhuma coluna nova") {
		t.Fatalf("sem linhas adicionar_coluna_se_ausente deveria falhar como o .NET; erro: %v", err)
	}
	if _, err := os.Stat(caminho); !errors.Is(err, fs.ErrNotExist) {
		t.Error("o banco não deveria ser criado")
	}
}

func TestPastaDasCopias(t *testing.T) {
	for caminho, want := range map[string]string{
		filepath.Join("x", "dados", "sql", "a.db"): filepath.Join("x", "dados", "backups"),
		filepath.Join("x", "dados", "Sql", "a.db"): filepath.Join("x", "dados", "backups"),
		filepath.Join("x", "outra", "a.db"):        filepath.Join("x", "outra", "backups"),
		filepath.Join("x", "sqlite", "a.db"):       filepath.Join("x", "sqlite", "backups"),
	} {
		if got := pastaDasCopias(caminho); got != want {
			t.Errorf("pastaDasCopias(%s) = %s, esperado %s", caminho, got, want)
		}
	}
}

func TestApagarCopiasAntigasSemPermissao(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("precisa de permissão de pasta do Unix e de um usuário que não seja o root")
	}
	pasta := t.TempDir()
	for dia := 1; dia <= BackupsGuardados+1; dia++ {
		if err := os.WriteFile(filepath.Join(pasta, fmt.Sprintf("%s202001%02d_000000.db", PrefixoBackup, dia)), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(pasta, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(pasta, 0o755) })
	// No .NET, o UnauthorizedAccessException do FileInfo.Delete não é pego (só o IOException) e a API não sobe.
	if err := apagarCopiasAntigas(pasta); !errors.Is(err, fs.ErrPermission) {
		t.Errorf("sem permissão para apagar, esperava erro de permissão; veio %v", err)
	}
}
