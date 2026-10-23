// Pacote banco abre o SQLite do Finanças Local do jeito que o back-end .NET abre, dá a função de transação e cria e
// atualiza o esquema como o EsquemaBanco do .NET (AplicarEsquema, em esquema_banco.go).
//
// O Go e o .NET convivem no mesmo arquivo durante a migração (ADR 0049), então tudo o que se grava aqui tem de
// estar no formato do EF Core: use os tipos do pacote tipos (Decimal, Data, DataHora, Guid). Armadilhas no
// README.md desta pasta.
package banco

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"

	_ "modernc.org/sqlite" // registra o driver "sqlite" (Go puro, sem cgo)
)

// EsperaSeOcupado é quanto uma conexão espera quando outra está gravando, em milissegundos. É o mesmo tempo que o
// Microsoft.Data.Sqlite espera por padrão (30 s) antes de desistir de um banco ocupado.
const EsperaSeOcupado = 30_000

// Abrir abre o banco no caminho dado, com os PRAGMAs que o EF Core usa (foreign_keys ligado, conferido pela sonda
// em internal/tipos/testdata) e a espera de banco ocupado.
//
// Não muda o journal_mode: ele é do arquivo, não da conexão, e passar para WAL é decisão da #231.
func Abrir(ctx context.Context, caminho string) (*sql.DB, error) {
	parametros := url.Values{}
	parametros.Add("_pragma", "foreign_keys(1)")
	parametros.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", EsperaSeOcupado))
	// BEGIN IMMEDIATE: a transação já pede a trava de gravação ao começar, e a espera acima vale para ela. Com o
	// BEGIN comum, duas transações que leem e depois gravam podem travar uma na outra e falhar na hora.
	parametros.Set("_txlock", "immediate")

	db, err := sql.Open("sqlite", caminho+"?"+parametros.Encode())
	if err != nil {
		return nil, fmt.Errorf("abrir o banco %s: %w", caminho, err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("abrir o banco %s: %w", caminho, err)
	}
	return db, nil
}

// EmTransacao roda fn numa transação: confirma se fn devolver nil e desfaz se devolver erro ou entrar em pânico
// (o pânico continua depois do desfazer).
func EmTransacao(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) (err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("começar transação: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()
	if err := fn(tx); err != nil {
		if errDesfazer := tx.Rollback(); errDesfazer != nil {
			return errors.Join(err, fmt.Errorf("desfazer transação: %w", errDesfazer))
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("confirmar transação: %w", err)
	}
	return nil
}
