package copia

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func bancoDeTeste(t *testing.T) string {
	t.Helper()
	caminho := filepath.Join(t.TempDir(), "FinancasLocal.db")
	db, err := sql.Open("sqlite", caminho)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE Conta (Id INTEGER PRIMARY KEY, Nome TEXT); INSERT INTO Conta (Nome) VALUES ('Corrente');`); err != nil {
		t.Fatal(err)
	}
	return caminho
}

func nomeDaConta(t *testing.T, caminho string) string {
	t.Helper()
	db, err := sql.Open("sqlite", caminho)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var nome string
	if err := db.QueryRow(`SELECT Nome FROM Conta`).Scan(&nome); err != nil {
		t.Fatal(err)
	}
	return nome
}

// Regra: a cópia tem os dados, e troca o arquivo que já estava no destino.
func TestSalvar(t *testing.T) {
	t.Parallel()
	banco := bancoDeTeste(t)
	destino := filepath.Join(t.TempDir(), "copia com espaço.db")
	if err := os.WriteFile(destino, []byte("antigo"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Salvar(banco, destino); err != nil {
		t.Fatal(err)
	}
	if nome := nomeDaConta(t, destino); nome != "Corrente" {
		t.Errorf("conta %q", nome)
	}
}

// Regra: sem o banco, ErrSemBanco, e nada é criado no destino.
func TestSemBanco(t *testing.T) {
	t.Parallel()
	destino := filepath.Join(t.TempDir(), "copia.db")
	err := Salvar(filepath.Join(t.TempDir(), "nao-existe.db"), destino)
	if !errors.Is(err, ErrSemBanco) {
		t.Errorf("erro %v", err)
	}
	if _, err := os.Stat(destino); !errors.Is(err, os.ErrNotExist) {
		t.Error("criou o destino")
	}
}

// Regra: destino que não dá para gravar é erro.
func TestDestinoInvalido(t *testing.T) {
	t.Parallel()
	if err := Salvar(bancoDeTeste(t), filepath.Join(t.TempDir(), "nao-existe", "copia.db")); err == nil {
		t.Error("sem erro")
	}
}
