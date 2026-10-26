package configuracao

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/segredos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
)

// Regra: LerInteiro é o int.TryParse(NumberStyles.Integer, invariante).
func TestLerInteiro(t *testing.T) {
	casos := []struct {
		texto string
		n     int
		ok    bool
	}{
		{"12", 12, true}, {"-7", -7, true}, {"+7", 7, true}, {" \t12\r\n", 12, true}, {"012", 12, true}, {"12\x00\x00", 12, true},
		{"2147483647", 2147483647, true}, {"-2147483648", -2147483648, true},
		{"9", 9, true}, {"1990", 1990, true}, // o 9 e o 0 nas pontas da faixa de dígitos (#400: nenhum caso tinha o 9)
		{"2147483648", 0, false}, {"", 0, false}, {" ", 0, false}, {"+", 0, false}, {"+-1", 0, false}, {"--1", 0, false},
		{"1.5", 0, false}, {"1,5", 0, false}, {"1e2", 0, false}, {"1 2", 0, false}, {" 12", 0, false}, {"١٢", 0, false},
	}
	for _, c := range casos {
		n, ok := LerInteiro(c.texto)
		if n != c.n || ok != c.ok {
			t.Errorf("LerInteiro(%q) = %d, %v; esperado %d, %v", c.texto, n, ok, c.n, c.ok)
		}
	}
}

// Regra: LerBool é o bool.TryParse.
func TestLerBool(t *testing.T) {
	casos := []struct {
		texto string
		b, ok bool
	}{
		{"true", true, true}, {"True", true, true}, {"FALSE", false, true}, {"  true\x00 ", true, true}, {" false", false, true},
		{"1", false, false}, {"0", false, false}, {"sim", false, false}, {"", false, false}, {"tru e", false, false},
	}
	for _, c := range casos {
		b, ok := LerBool(c.texto)
		if b != c.b || ok != c.ok {
			t.Errorf("LerBool(%q) = %v, %v; esperado %v, %v", c.texto, b, ok, c.b, c.ok)
		}
	}
}

// Regra: LerInstante guarda o Kind do texto (RoundtripKind), e o JSON sai como o System.Text.Json escreve.
func TestLerInstante(t *testing.T) {
	brasilia := time.FixedZone("", -3*3600)
	anterior := time.Local
	time.Local = brasilia
	t.Cleanup(func() { time.Local = anterior })
	casos := []struct{ texto, json string }{
		{"2026-10-14T09:00:00.0000000Z", `"2026-10-14T09:00:00Z"`},
		{"2026-10-14T09:00:00.1234567Z", `"2026-10-14T09:00:00.1234567Z"`},
		{"2026-10-14T09:00:00.120Z", `"2026-10-14T09:00:00.12Z"`},
		{"2026-10-14 09:00:00.5", `"2026-10-14T09:00:00.5"`},
		{"2026-10-14T09:00", `"2026-10-14T09:00:00"`},
		{"2026-10-14", `"2026-10-14T00:00:00"`},
		{"2026-10-14T12:00:00+00:00", `"2026-10-14T09:00:00-03:00"`}, // com deslocamento: a hora local
		{"2026-10-14T09:00:00-03:00", `"2026-10-14T09:00:00-03:00"`},
	}
	for _, c := range casos {
		i, ok := LerInstante(c.texto)
		if !ok {
			t.Errorf("LerInstante(%q) não leu", c.texto)
			continue
		}
		if b, _ := json.Marshal(i); string(b) != c.json {
			t.Errorf("LerInstante(%q) = %s, esperado %s", c.texto, b, c.json)
		}
	}
	for _, texto := range []string{"", "ontem", "2026-13-01", "2026-10-14T25:00:00Z"} {
		if _, ok := LerInstante(texto); ok {
			t.Errorf("LerInstante(%q) leu um texto inválido", texto)
		}
	}
}

// Regra: segredo não é lido pelo Obter (o valor está cifrado pelo .NET); Existe conta só com valor.
func TestObterSegredoEExiste(t *testing.T) {
	db := testeapoio.NovoBanco(t)
	ctx := t.Context()
	if _, err := db.ExecContext(ctx, `INSERT INTO "Configuracao" VALUES ('Pluggy.ClientSecret', 'CfDJ8...', 1, '2026-10-01 00:00:00'),
		('Sem.Valor', NULL, 0, '2026-10-01 00:00:00')`); err != nil {
		t.Fatal(err)
	}
	s := Novo(db, testeapoio.RelogioDosTestes())
	if _, err := s.Obter(ctx, "Pluggy.ClientSecret"); !errors.Is(err, ErrProtegido) {
		t.Errorf("segredo: erro %v, esperado ErrProtegido", err)
	}
	for chave, esperado := range map[string]bool{"Pluggy.ClientSecret": true, "Sem.Valor": false, "Nao.Existe": false, "Sincronizacao.IntervaloHoras": true} {
		if existe, err := s.Existe(ctx, chave); err != nil || existe != esperado {
			t.Errorf("Existe(%s) = %v, %v; esperado %v", chave, existe, err, esperado)
		}
	}
	if v, err := s.Obter(ctx, "Sem.Valor"); v != nil || err != nil {
		t.Errorf("sem valor: %v, %v", v, err)
	}
	if n, err := s.ObterInteiro(ctx, "Nao.Existe", 42); n != 42 || err != nil {
		t.Errorf("padrão do inteiro: %d, %v", n, err)
	}
	if b, err := s.ObterBool(ctx, "Sincronizacao.Habilitada", false); !b || err != nil {
		t.Errorf("bool gravado: %v, %v", b, err)
	}
}

// Regra: Definir cria ou troca, tira a proteção e sempre regrava AtualizadoEm em UTC.
func TestDefinir(t *testing.T) {
	db := testeapoio.NovoBanco(t)
	ctx := t.Context()
	agora := time.Date(2026, 10, 15, 9, 30, 0, 0, time.FixedZone("", -3*3600))
	s := Novo(db, relogio.Fixo{Instante: agora})
	valor := "abc"
	for _, chave := range []string{"Nova.Chave", "Sincronizacao.IntervaloHoras"} {
		if err := s.Definir(ctx, chave, &valor); err != nil {
			t.Fatal(err)
		}
		var gravado, atualizado string
		var protegido int
		if err := db.QueryRowContext(ctx, `SELECT "Valor", "Protegido", "AtualizadoEm" FROM "Configuracao" WHERE "Chave" = ?`, chave).
			Scan(&gravado, &protegido, &atualizado); err != nil {
			t.Fatal(err)
		}
		if gravado != "abc" || protegido != 0 || atualizado != "2026-10-15 12:30:00" {
			t.Errorf("%s: %q, %d, %q", chave, gravado, protegido, atualizado)
		}
	}
}

// Regra: o segredo é gravado cifrado (Protegido = 1) e lido de volta; o valor antigo, do Data Protection, volta como
// "informe de novo" e a linha não muda.
func TestSegredoNovoEAntigo(t *testing.T) {
	db := testeapoio.NovoBanco(t)
	ctx := t.Context()
	cofre, err := segredos.Abrir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := Novo(db, testeapoio.RelogioDosTestes())
	segredo := "meu-client-secret"
	if err := s.DefinirSegredo(ctx, cofre, "Pluggy.ClientSecret", &segredo); err != nil {
		t.Fatal(err)
	}
	var gravado string
	var protegido int
	if err := db.QueryRowContext(ctx, `SELECT "Valor", "Protegido" FROM "Configuracao" WHERE "Chave" = 'Pluggy.ClientSecret'`).Scan(&gravado, &protegido); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(gravado, "v2:") || strings.Contains(gravado, segredo) || protegido != 1 {
		t.Errorf("gravado %q, Protegido %d", gravado, protegido)
	}
	if lido, err := s.ObterSegredo(ctx, cofre, "Pluggy.ClientSecret"); err != nil || lido == nil || *lido != segredo {
		t.Errorf("lido %v, %v", lido, err)
	}

	const antigo = `INSERT INTO "Configuracao" VALUES ('Brapi.Token', 'CfDJ8NZn1tJ0oQ1Gm1gBQ', 1, '2026-10-01 00:00:00')`
	if _, err := db.ExecContext(ctx, antigo); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ObterSegredo(ctx, cofre, "Brapi.Token"); !errors.Is(err, segredos.ErrInformarDeNovo) {
		t.Errorf("valor antigo: %v, esperado ErrInformarDeNovo", err)
	}
	var linha string
	if err := db.QueryRowContext(ctx, `SELECT "Valor" || '|' || "Protegido" || '|' || "AtualizadoEm" FROM "Configuracao" WHERE "Chave" = 'Brapi.Token'`).Scan(&linha); err != nil {
		t.Fatal(err)
	}
	if linha != "CfDJ8NZn1tJ0oQ1Gm1gBQ|1|2026-10-01 00:00:00" {
		t.Errorf("a linha antiga mudou: %q", linha)
	}
}

// Regra: ObterInteiro e ObterBool leem o valor gravado; sem valor, ou com valor que não se lê, o padrão (#400: o
// teste de mutação mostrou que só a chave que falta era conferida).
func TestObterInteiroEBool(t *testing.T) {
	db := testeapoio.NovoBanco(t)
	ctx := t.Context()
	if _, err := db.ExecContext(ctx, `INSERT INTO "Configuracao" VALUES ('Teste.Inteiro', ' 42 ', 0, '2026-10-01 00:00:00'),
		('Teste.InteiroRuim', '4,2', 0, '2026-10-01 00:00:00'), ('Teste.Bool', 'False', 0, '2026-10-01 00:00:00'),
		('Teste.BoolRuim', 'talvez', 0, '2026-10-01 00:00:00')`); err != nil {
		t.Fatal(err)
	}
	s := Novo(db, testeapoio.RelogioDosTestes())
	inteiros := map[string]int{"Teste.Inteiro": 42, "Teste.InteiroRuim": 7, "Nao.Existe": 7}
	for chave, esperado := range inteiros {
		if n, err := s.ObterInteiro(ctx, chave, 7); n != esperado || err != nil {
			t.Errorf("ObterInteiro(%s) = %d, %v; esperado %d", chave, n, err, esperado)
		}
	}
	booleanos := map[string]bool{"Teste.Bool": false, "Teste.BoolRuim": true, "Nao.Existe": true}
	for chave, esperado := range booleanos {
		if b, err := s.ObterBool(ctx, chave, true); b != esperado || err != nil {
			t.Errorf("ObterBool(%s) = %v, %v; esperado %v", chave, b, err, esperado)
		}
	}
}
