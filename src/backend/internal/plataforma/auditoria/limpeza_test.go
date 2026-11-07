package auditoria

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// prazoFixo é a Parametrização dos testes: o prazo em dias, ou o erro de quem não conseguiu ler.
type prazoFixo struct {
	dias int
	erro error
}

func (p prazoFixo) ObterInteiro(_ context.Context, chave string) (int, error) {
	if chave != ChaveDiasRetencao {
		return 0, errors.New("chave inesperada: " + chave)
	}
	return p.dias, p.erro
}

// acessar grava um acesso fictício na hora dada (UTC).
func acessar(t *testing.T, db *sql.DB, quando time.Time, evento string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO "AuditoriaAcesso" ("DataHora", "Evento", "Login", "Ip", "UserAgent")
		VALUES (?, ?, 'ana.teste', '127.0.0.1', 'Navegador de teste')`, tipos.NovaDataHora(quando), evento); err != nil {
		t.Fatal(err)
	}
}

func eventosGuardados(t *testing.T, db *sql.DB) string {
	t.Helper()
	linhas, err := db.Query(`SELECT "Evento" FROM "AuditoriaAcesso" ORDER BY "Id"`)
	if err != nil {
		t.Fatal(err)
	}
	defer linhas.Close()
	var eventos []string
	for linhas.Next() {
		var e string
		if err := linhas.Scan(&e); err != nil {
			t.Fatal(err)
		}
		eventos = append(eventos, e)
	}
	return strings.Join(eventos, ",")
}

// Regra (#530): apaga os acessos de antes do prazo (contado da hora de agora); o que está exatamente no limite fica.
func TestLimpar(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome     string
		dias     int
		ficam    string
		apagados int64
	}{
		{"180 dias", 180, "limite,179 dias,agora", 2},
		{"30 dias", 30, "agora", 4},
		{"730 dias", 730, "181 dias,1 s antes do limite,limite,179 dias,agora", 0},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			db := testeapoio.NovoBanco(t)
			agora := testeapoio.InstanteDosTestes
			dia := 24 * time.Hour
			acessar(t, db, agora.Add(-181*dia), "181 dias")
			acessar(t, db, agora.Add(-180*dia-time.Second), "1 s antes do limite")
			acessar(t, db, agora.Add(-180*dia), "limite")
			acessar(t, db, agora.Add(-179*dia), "179 dias")
			acessar(t, db, agora, "agora")
			apagados, err := NovaLimpeza(db, prazoFixo{dias: c.dias}, testeapoio.RelogioDosTestes(), slog.New(slog.DiscardHandler)).
				Limpar(t.Context())
			if err != nil || apagados != c.apagados || eventosGuardados(t, db) != c.ficam {
				t.Errorf("erro %v, %d apagados, ficaram %q", err, apagados, eventosGuardados(t, db))
			}
		})
	}
}

// Regra (#530): sem o prazo, não apaga nada e devolve o erro.
func TestLimparSemOPrazo(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBanco(t)
	acessar(t, db, testeapoio.InstanteDosTestes.Add(-1000*24*time.Hour), "antigo")
	semPrazo := errors.New("sem a Parametrização")
	_, err := NovaLimpeza(db, prazoFixo{erro: semPrazo}, testeapoio.RelogioDosTestes(), slog.New(slog.DiscardHandler)).
		Limpar(t.Context())
	if !errors.Is(err, semPrazo) || eventosGuardados(t, db) != "antigo" {
		t.Errorf("erro %v, ficaram %q", err, eventosGuardados(t, db))
	}
}

// Regra (#530): a limpeza roda ao subir e de novo a cada 24 horas, até parar; a que falha vira aviso e não para as
// próximas.
func TestRodarLimpaAoSubirEACadaDia(t *testing.T) {
	db := testeapoio.NovoBanco(t)
	velho := testeapoio.InstanteDosTestes.Add(-200 * 24 * time.Hour)
	acessar(t, db, velho, "antes de subir")
	synctest.Test(t, func(t *testing.T) {
		var saida bytes.Buffer
		l := NovaLimpeza(db, prazoFixo{dias: 180}, testeapoio.RelogioDosTestes(), slog.New(slog.NewTextHandler(&saida, nil)))
		ctx, parar := context.WithCancel(t.Context())
		parou := make(chan struct{})
		go func() { l.Rodar(ctx); close(parou) }()
		synctest.Wait()
		if g := eventosGuardados(t, db); g != "" {
			t.Fatalf("ao subir, ficou %q", g)
		}
		acessar(t, db, velho, "depois de subir")
		time.Sleep(24*time.Hour - time.Second) // o relógio do synctest: não espera de verdade
		synctest.Wait()
		if g := eventosGuardados(t, db); g != "depois de subir" {
			t.Fatalf("antes de 24 h, ficou %q", g)
		}
		time.Sleep(time.Second)
		synctest.Wait()
		if g := eventosGuardados(t, db); g != "" {
			t.Fatalf("depois de 24 h, ficou %q", g)
		}
		if saida.Len() != 0 {
			t.Errorf("log sem falha: %q", saida.String())
		}
		parar()
		<-parou
	})
}

// Regra (#530): a falha vira aviso no log, com a origem; parar não é falha.
func TestRodarAvisaAFalha(t *testing.T) {
	db := testeapoio.NovoBanco(t)
	synctest.Test(t, func(t *testing.T) {
		var saida bytes.Buffer
		l := NovaLimpeza(db, prazoFixo{erro: errors.New("sem a Parametrização")}, testeapoio.RelogioDosTestes(),
			slog.New(slog.NewTextHandler(&saida, nil)))
		ctx, parar := context.WithCancel(t.Context())
		parou := make(chan struct{})
		go func() { l.Rodar(ctx); close(parou) }()
		synctest.Wait()
		parar()
		<-parou
		if s := saida.String(); strings.Count(s, "level=WARN") != 1 || !strings.Contains(s, "origem=Sistema") ||
			!strings.Contains(s, "sem a Parametrização") {
			t.Errorf("log %q", s)
		}
	})
}

// Regra (#530): o banco que recusa apagar devolve o erro, sem apagar nada.
func TestLimparComOBancoRecusando(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBanco(t)
	acessar(t, db, testeapoio.InstanteDosTestes.Add(-1000*24*time.Hour), "antigo")
	if _, err := db.Exec(`CREATE TRIGGER nao_apaga BEFORE DELETE ON "AuditoriaAcesso" BEGIN SELECT RAISE(ABORT, 'não apaga'); END`); err != nil {
		t.Fatal(err)
	}
	_, err := NovaLimpeza(db, prazoFixo{dias: 180}, testeapoio.RelogioDosTestes(), slog.New(slog.DiscardHandler)).Limpar(t.Context())
	if err == nil || !strings.Contains(err.Error(), "apagar os acessos antigos") || eventosGuardados(t, db) != "antigo" {
		t.Errorf("erro %v, ficaram %q", err, eventosGuardados(t, db))
	}
}
