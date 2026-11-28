package contrato

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// bancoDeTeste cria um banco com Conta e Outra (AUTOINCREMENT) e devolve o caminho.
func bancoDeTeste(t *testing.T) string {
	t.Helper()
	caminho := filepath.Join(t.TempDir(), "t.db")
	db, err := sql.Open("sqlite", caminho)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, c := range []string{
		`CREATE TABLE Conta (Id INTEGER PRIMARY KEY AUTOINCREMENT, Nome TEXT)`,
		`CREATE TABLE Outra (Id INTEGER PRIMARY KEY AUTOINCREMENT, Nome TEXT)`,
		`INSERT INTO Conta (Nome) VALUES ('base')`,
	} {
		if _, err := db.Exec(c); err != nil {
			t.Fatal(err)
		}
	}
	return caminho
}

func TestFotografarSoAsTabelasDadas(t *testing.T) {
	caminho := bancoDeTeste(t)
	db, err := abrirSoLeitura(caminho)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	foto, err := fotografar(t.Context(), db, []string{"Conta"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := foto["Conta"]; !ok || len(foto) != 1 {
		t.Errorf("a foto tem de ter só Conta, veio %v", foto)
	}
	if _, err := fotografar(t.Context(), db, []string{"NaoExiste"}); err == nil {
		t.Error("tabela que não existe tem de dar erro")
	}
}

func TestJanelaDeIds(t *testing.T) {
	anterior := TabelasComJanela
	TabelasComJanela = []string{"Conta", "Outra"}
	defer func() { TabelasComJanela = anterior }()
	caminho := bancoDeTeste(t)

	if err := janelaDeIds(t.Context(), caminho, 4); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", caminho)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, tabela := range []string{"Conta", "Outra"} {
		var id int64
		if err := db.QueryRow(`INSERT INTO ` + tabela + ` (Nome) VALUES ('novo') RETURNING Id`).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if want := int64(5*TamanhoDaJanela + 1); id != want {
			t.Errorf("%s: primeiro id do arquivo 04 = %d, esperava %d", tabela, id, want)
		}
	}
	if err := janelaDeIds(t.Context(), caminho, 5); err != nil {
		t.Fatal(err)
	}
	// Janela que já passou (um arquivo anterior criou linhas demais) é erro, não id deslocado em silêncio.
	if err := janelaDeIds(t.Context(), caminho, 4); err == nil {
		t.Error("janela já passada tem de dar erro")
	}
}
