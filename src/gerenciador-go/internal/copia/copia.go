// Package copia é o "Salvar uma cópia do banco" (CopiaDoBanco.cs): uma cópia consistente com o site no ar, pelo
// VACUUM INTO do SQLite.
package copia

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"

	_ "modernc.org/sqlite"
)

// ErrSemBanco: o banco não está onde a instalação diz.
var ErrSemBanco = errors.New("o banco não foi encontrado")

// Salvar copia o banco para o destino, trocando o arquivo que já estiver lá.
func Salvar(banco, destino string) error {
	if _, err := os.Stat(banco); err != nil {
		return fmt.Errorf("%w: %s", ErrSemBanco, banco)
	}
	if err := os.Remove(destino); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	// A cópia tem todos os dados financeiros: no Linux, nasce legível só pelo dono (como a pasta dados/; no Windows, o
	// modo não muda nada). O VACUUM INTO aceita um arquivo vazio como destino e mantém a permissão dele.
	f, err := os.OpenFile(destino, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	f.Close()
	parametros := url.Values{}
	parametros.Add("_pragma", "busy_timeout(30000)")
	db, err := sql.Open("sqlite", banco+"?"+parametros.Encode())
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.Exec("VACUUM INTO ?", destino); err != nil {
		return fmt.Errorf("copiar o banco: %w", err)
	}
	return nil
}
