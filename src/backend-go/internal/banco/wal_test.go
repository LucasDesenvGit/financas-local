package banco

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Variáveis do ajudante que roda num processo separado (o próprio executável de teste, chamado de novo).
const (
	varBancoCompartilhado = "FINANCAS_GRAVAR_JUNTO"       // caminho do banco em que o ajudante grava
	varQuemGrava          = "FINANCAS_GRAVAR_JUNTO_QUEM"  // prefixo das chaves gravadas pelo ajudante
	varQuantasVezes       = "FINANCAS_GRAVAR_JUNTO_VEZES" // quantas transações (padrão: 300)
	// Com FINANCAS_GRAVAR_JUNTO_SEGUNDOS, grava por esse tempo em vez de um número fixo de vezes, segurando a trava
	// de gravação 5 ms em cada transação e descansando 5 ms entre elas: quem grava junto (o .NET, no script de
	// ponta a ponta) encontra o banco ocupado de verdade e tem de esperar.
	varSegundos = "FINANCAS_GRAVAR_JUNTO_SEGUNDOS"
)

// ritmo de gravação do ajudante.
type ritmo struct {
	vezes   int           // quantas transações (se ate for zero)
	ate     time.Time     // grava até este instante
	segurar time.Duration // quanto tempo segura a trava em cada transação
	pausa   time.Duration // descanso entre transações
}

// gravarConfiguracoes grava n chaves na Configuracao, uma transação para cada, no formato do EF Core. Cada transação
// lê antes de gravar: com o BEGIN comum (deferred), duas dessas ao mesmo tempo podem travar uma na outra e falhar
// na hora com "database is locked"; com o BEGIN IMMEDIATE do Abrir, a segunda espera a primeira.
func gravarConfiguracoes(ctx context.Context, db *sql.DB, quem string, r ritmo) (int, error) {
	i := 0
	for ; r.ate.IsZero() && i < r.vezes || !r.ate.IsZero() && time.Now().Before(r.ate); i++ {
		err := EmTransacao(ctx, db, func(tx *sql.Tx) error {
			var total int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM "Configuracao"`).Scan(&total); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx,
				`INSERT INTO "Configuracao" ("Chave", "Valor", "Protegido", "AtualizadoEm") VALUES (?, ?, 0, ?)
				 ON CONFLICT ("Chave") DO UPDATE SET "Valor" = excluded."Valor", "AtualizadoEm" = excluded."AtualizadoEm"`,
				fmt.Sprintf("teste-wal-%s-%d", quem, i), strconv.Itoa(total), tipos.NovaDataHora(time.Now().UTC()))
			time.Sleep(r.segurar)
			return err
		})
		if err != nil {
			return i, fmt.Errorf("%s, transação %d: %w", quem, i, err)
		}
		time.Sleep(r.pausa)
	}
	return i, nil
}

// TestGravarJuntoComOutroProcesso é o ajudante: só roda com FINANCAS_GRAVAR_JUNTO apontando para um banco. O teste
// de WAL abaixo o chama num processo separado, e o scripts/ci/testar-go-atras-do-dotnet.sh o roda contra o banco do
// .NET enquanto o .NET também grava (#231).
func TestGravarJuntoComOutroProcesso(t *testing.T) {
	caminho := os.Getenv(varBancoCompartilhado)
	if caminho == "" {
		t.Skip("ajudante: só roda com " + varBancoCompartilhado + " (chamado pelo TestDoisProcessosGravandoEmWAL ou pelo script de ponta a ponta)")
	}
	r := ritmo{vezes: 300}
	if v, err := strconv.Atoi(os.Getenv(varQuantasVezes)); err == nil && v > 0 {
		r.vezes = v
	}
	if s, err := strconv.Atoi(os.Getenv(varSegundos)); err == nil && s > 0 {
		r = ritmo{ate: time.Now().Add(time.Duration(s) * time.Second), segurar: 5 * time.Millisecond, pausa: 5 * time.Millisecond}
	}
	quem := os.Getenv(varQuemGrava)
	if quem == "" {
		quem = "go"
	}
	ctx := context.Background()
	db, err := Abrir(ctx, caminho)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	inicio := time.Now()
	n, err := gravarConfiguracoes(ctx, db, quem, r)
	if err != nil {
		t.Fatal(err)
	}
	// O script de ponta a ponta lê esta linha para conferir que nenhuma gravação se perdeu.
	t.Logf("transacoes=%d em %s", n, time.Since(inicio).Round(time.Millisecond))
}

// Dois processos gravando no mesmo arquivo ao mesmo tempo, em WAL, com o busy_timeout e o BEGIN IMMEDIATE do
// Abrir (#231): nenhuma transação falha, e nenhuma se perde.
func TestDoisProcessosGravandoEmWAL(t *testing.T) {
	if os.Getenv(varBancoCompartilhado) != "" {
		t.Skip("dentro do ajudante")
	}
	ctx := context.Background()
	caminho := filepath.Join(t.TempDir(), "wal.db")
	db, err := Abrir(ctx, caminho)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Como o EsquemaBanco do .NET faz ao iniciar: o modo WAL fica gravado no arquivo.
	var modo string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode=WAL").Scan(&modo); err != nil || modo != "wal" {
		t.Fatalf("journal_mode=%q, erro %v", modo, err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE "Configuracao" ("Chave" TEXT NOT NULL PRIMARY KEY, "Valor" TEXT NULL,
		"Protegido" INTEGER NOT NULL DEFAULT 0, "AtualizadoEm" TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}

	const vezes = 300
	executavel, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	outro := exec.Command(executavel, "-test.run=^TestGravarJuntoComOutroProcesso$", "-test.count=1", "-test.v")
	outro.Env = append(os.Environ(), varBancoCompartilhado+"="+caminho, varQuemGrava+"=filho",
		varQuantasVezes+"="+strconv.Itoa(vezes))

	var grupo sync.WaitGroup
	var saidaDoOutro []byte
	var erroDoOutro error
	grupo.Go(func() { saidaDoOutro, erroDoOutro = outro.CombinedOutput() })
	_, erroDeste := gravarConfiguracoes(ctx, db, "pai", ritmo{vezes: vezes})
	grupo.Wait()

	if erroDeste != nil {
		t.Errorf("este processo: %v", erroDeste)
	}
	if erroDoOutro != nil {
		t.Errorf("o outro processo: %v\n%s", erroDoOutro, saidaDoOutro)
	}
	for _, quem := range []string{"pai", "filho"} {
		var n int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM "Configuracao" WHERE "Chave" LIKE ?`,
			"teste-wal-"+quem+"-%").Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != vezes {
			t.Errorf("%s gravou %d de %d", quem, n, vezes)
		}
	}
}
