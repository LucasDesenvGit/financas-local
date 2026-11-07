package restauracao

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// servicoFalso anota o que a restauração pediu ao serviço, e falha quando o teste manda.
type servicoFalso struct {
	pedidos                 []string
	erroDesligar, erroLigar error
}

func (s *servicoFalso) Desligar() error {
	s.pedidos = append(s.pedidos, "desligar")
	return s.erroDesligar
}
func (s *servicoFalso) Ligar() error { s.pedidos = append(s.pedidos, "ligar"); return s.erroLigar }

// criarBanco grava um banco com as tabelas que a conferência procura e uma marca para saber qual é qual.
func criarBanco(t *testing.T, caminho, marca string, tabelas ...string) {
	t.Helper()
	if len(tabelas) == 0 {
		tabelas = []string{"Transacao", "Conta", "Usuario"}
	}
	db, err := sql.Open("sqlite", caminho)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, nome := range tabelas {
		if _, err := db.Exec(`CREATE TABLE "` + nome + `" ("Id" INTEGER PRIMARY KEY)`); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`CREATE TABLE "Marca" ("Texto" TEXT); INSERT INTO "Marca" VALUES (?)`, marca); err != nil {
		t.Fatal(err)
	}
}

func marca(t *testing.T, caminho string) string {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(caminho)+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var texto string
	if err := db.QueryRow(`SELECT "Texto" FROM "Marca"`).Scan(&texto); err != nil {
		t.Fatalf("ler a marca de %s: %v", caminho, err)
	}
	return texto
}

func existe(caminho string) bool {
	_, err := os.Stat(caminho)
	return err == nil
}

func ler(t *testing.T, caminho string) string {
	t.Helper()
	b, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// instalacao monta dados/sql/FinancasLocal.db (com um -wal e um -shm de mentira, do banco de agora) e uma cópia.
func instalacao(t *testing.T) (banco, copia string) {
	t.Helper()
	raiz := t.TempDir()
	banco = filepath.Join(raiz, "dados", "sql", "FinancasLocal.db")
	if err := os.MkdirAll(filepath.Dir(banco), 0o700); err != nil {
		t.Fatal(err)
	}
	criarBanco(t, banco, "agora")
	for _, sufixo := range []string{"-wal", "-shm"} {
		if err := os.WriteFile(banco+sufixo, []byte("do banco de agora"+sufixo), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	copia = filepath.Join(raiz, "pendrive", "FinancasLocal-copia.db")
	if err := os.MkdirAll(filepath.Dir(copia), 0o700); err != nil {
		t.Fatal(err)
	}
	criarBanco(t, copia, "cópia")
	return banco, copia
}

// Regra (#533): restaurar desliga o site, guarda o banco de agora (com o -wal e o -shm) em dados/antes-de-restaurar,
// põe a cópia no lugar sem o -wal e o -shm do banco antigo e liga o site.
func TestRestaurar(t *testing.T) {
	banco, copia := instalacao(t)
	s := &servicoFalso{}
	antes, err := Restaurar(banco, copia, s)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(s.pedidos, ",") != "desligar,ligar" {
		t.Errorf("pedidos ao serviço: %v", s.pedidos)
	}
	if marca(t, banco) != "cópia" {
		t.Error("o banco não é a cópia")
	}
	if existe(banco+"-wal") || existe(banco+"-shm") || existe(banco+".restaurando") {
		t.Error("sobrou o -wal, o -shm ou o arquivo temporário")
	}
	if quer := filepath.Join(filepath.Dir(filepath.Dir(banco)), "antes-de-restaurar"); antes != quer {
		t.Errorf("antes em %s, esperado %s", antes, quer)
	}
	guardado := filepath.Join(antes, "FinancasLocal.db")
	// Os arquivos antes do banco: abrir o banco com o SQLite mexe no -wal e no -shm (de mentira) ao lado dele.
	if ler(t, guardado+"-wal") != "do banco de agora-wal" || ler(t, guardado+"-shm") != "do banco de agora-shm" || marca(t, guardado) != "agora" {
		t.Error("o banco de antes não foi guardado inteiro")
	}
	if marca(t, copia) != "cópia" {
		t.Error("a cópia escolhida mudou")
	}
}

// Regra (#533): restaurar de novo troca o que estava em antes-de-restaurar (como os comandos do guia), e o banco sem
// -wal nem -shm também é guardado.
func TestRestaurarDuasVezes(t *testing.T) {
	banco, copia := instalacao(t)
	if _, err := Restaurar(banco, copia, &servicoFalso{}); err != nil {
		t.Fatal(err)
	}
	outra := filepath.Join(filepath.Dir(copia), "outra.db")
	criarBanco(t, outra, "outra")
	antes, err := Restaurar(banco, outra, &servicoFalso{})
	if err != nil {
		t.Fatal(err)
	}
	guardado := filepath.Join(antes, "FinancasLocal.db")
	if marca(t, banco) != "outra" || marca(t, guardado) != "cópia" || existe(guardado+"-wal") || existe(guardado+"-shm") {
		t.Errorf("banco %q, guardado %q, -wal guardado %v", marca(t, banco), marca(t, guardado), existe(guardado+"-wal"))
	}
}

// Regra (#533): o que não é uma cópia do banco do site é recusado antes de desligar o site, e nada muda.
func TestRecusaAntesDeDesligar(t *testing.T) {
	banco, copia := instalacao(t)
	texto := filepath.Join(filepath.Dir(copia), "anotacoes.db")
	if err := os.WriteFile(texto, []byte("não é um banco"), 0o600); err != nil {
		t.Fatal(err)
	}
	outroSqlite := filepath.Join(filepath.Dir(copia), "outro-programa.db")
	criarBanco(t, outroSqlite, "outro", "Clientes")
	casos := []struct {
		nome, arquivo string
		quer          error
	}{
		{"arquivo de texto", texto, ErrNaoEhCopia},
		{"banco de outro programa", outroSqlite, ErrNaoEhCopia},
		{"arquivo que não existe", filepath.Join(filepath.Dir(copia), "sumiu.db"), ErrNaoEhCopia},
		{"o próprio banco do site", banco, ErrMesmoArquivo},
	}
	for _, c := range casos {
		s := &servicoFalso{}
		if _, err := Restaurar(banco, c.arquivo, s); !errors.Is(err, c.quer) {
			t.Errorf("%s: erro %v, esperado %v", c.nome, err, c.quer)
		}
		if len(s.pedidos) != 0 || ler(t, banco+"-wal") != "do banco de agora-wal" || marca(t, banco) != "agora" {
			t.Errorf("%s: pedidos %v, banco %q", c.nome, s.pedidos, marca(t, banco))
		}
	}
	if err := Conferir(copia); err != nil {
		t.Errorf("a cópia certa foi recusada: %v", err)
	}
	if err := Conferir(texto); !errors.Is(err, ErrNaoEhCopia) {
		t.Errorf("Conferir(texto): %v", err)
	}
}

// Regra (#533): sem o banco do site, não desliga nada.
func TestSemBanco(t *testing.T) {
	_, copia := instalacao(t)
	s := &servicoFalso{}
	if _, err := Restaurar(filepath.Join(t.TempDir(), "dados", "sql", "FinancasLocal.db"), copia, s); !errors.Is(err, ErrSemBanco) || len(s.pedidos) != 0 {
		t.Errorf("erro %v, pedidos %v", err, s.pedidos)
	}
}

// Regra (#533): o site que não desliga não tem o banco trocado nem é ligado de novo.
func TestSiteQueNaoDesliga(t *testing.T) {
	banco, copia := instalacao(t)
	s := &servicoFalso{erroDesligar: errors.New("acesso negado")}
	if _, err := Restaurar(banco, copia, s); err == nil || !strings.Contains(err.Error(), "desligar o site") {
		t.Errorf("erro %v", err)
	}
	if strings.Join(s.pedidos, ",") != "desligar" || ler(t, banco+"-wal") != "do banco de agora-wal" || marca(t, banco) != "agora" {
		t.Errorf("pedidos %v, banco %q", s.pedidos, marca(t, banco))
	}
}

// Regra (#533): se a troca falha depois de mexer no banco, o de antes volta inteiro (com o -wal e o -shm), e o site é
// ligado de novo; se falha antes de mexer (sem onde guardar o de antes), o banco fica como estava, e o site também
// volta.
func TestFalhaNoMeio(t *testing.T) {
	t.Run("depois de mexer no banco", func(t *testing.T) {
		banco, copia := instalacao(t)
		renomear = func(string, string) error { return errors.New("disco cheio") }
		t.Cleanup(func() { renomear = os.Rename })
		s := &servicoFalso{}
		// O banco de antes voltou sem erro: o erro é só o da troca.
		if _, err := Restaurar(banco, copia, s); err == nil || !strings.Contains(err.Error(), "disco cheio") ||
			strings.Contains(err.Error(), "voltar o banco de antes") {
			t.Errorf("erro %v", err)
		}
		if strings.Join(s.pedidos, ",") != "desligar,ligar" || ler(t, banco+"-wal") != "do banco de agora-wal" ||
			ler(t, banco+"-shm") != "do banco de agora-shm" || existe(banco+".restaurando") || marca(t, banco) != "agora" {
			t.Errorf("pedidos %v, banco %q", s.pedidos, marca(t, banco))
		}
	})
	t.Run("o -wal que não dá para guardar", func(t *testing.T) {
		banco, copia := instalacao(t)
		// O -wal é uma pasta: abrir dá certo, ler não. Só o -wal e o -shm que não existem podem faltar no guardado.
		if err := os.Remove(banco + "-wal"); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(banco+"-wal", 0o700); err != nil {
			t.Fatal(err)
		}
		s := &servicoFalso{}
		if _, err := Restaurar(banco, copia, s); err == nil || !strings.Contains(err.Error(), "guardar o banco de antes") {
			t.Errorf("erro %v", err)
		}
		if strings.Join(s.pedidos, ",") != "desligar,ligar" || ler(t, banco+"-shm") != "do banco de agora-shm" {
			t.Errorf("pedidos %v", s.pedidos)
		}
		if info, err := os.Stat(banco + "-wal"); err != nil || !info.IsDir() {
			t.Errorf("o -wal mudou: %v", err)
		}
	})
	t.Run("antes de mexer no banco", func(t *testing.T) {
		banco, copia := instalacao(t)
		// antes-de-restaurar ocupado por um arquivo: não dá para guardar o banco de agora.
		if err := os.WriteFile(filepath.Join(filepath.Dir(filepath.Dir(banco)), PastaAntes), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		s := &servicoFalso{erroLigar: errors.New("não ligou")}
		_, err := Restaurar(banco, copia, s)
		if err == nil || !strings.Contains(err.Error(), "criar") || !strings.Contains(err.Error(), "ligar o site: não ligou") {
			t.Errorf("erro %v", err)
		}
		if strings.Join(s.pedidos, ",") != "desligar,ligar" || ler(t, banco+"-wal") != "do banco de agora-wal" || marca(t, banco) != "agora" {
			t.Errorf("pedidos %v, banco %q", s.pedidos, marca(t, banco))
		}
	})
}

// Regra (#533): sem o banco guardado, voltar não apaga o banco do site.
func TestVoltarSemOGuardado(t *testing.T) {
	banco, _ := instalacao(t)
	if err := voltar(banco, filepath.Join(t.TempDir(), "vazia")); err == nil {
		t.Error("voltou sem o banco guardado")
	}
	if ler(t, banco+"-wal") != "do banco de agora-wal" || marca(t, banco) != "agora" {
		t.Error("voltar mexeu no banco")
	}
}
