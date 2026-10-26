package configuracao

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
)

// #81: o Instante contra o gabarito do .NET, os "instantes" do tipos/testdata/csharp.json (gravados pela sonda
// tipos/testdata/sonda-csharp): o texto lido como o ObterDataAsync (DateTimeStyles.RoundtripKind) e escrito pelo
// System.Text.Json e pelo ToString("O").
func TestInstanteIgualAoCSharp(t *testing.T) {
	t.Parallel()
	dados, err := os.ReadFile("../tipos/testdata/csharp.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Instantes []struct {
			Texto, JSON, Redondo, Tipo string
		} `json:"instantes"`
		AgoraEmJSON string `json:"agoraEmJson"`
	}
	if err := json.Unmarshal(dados, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Instantes) == 0 {
		t.Fatal("gabarito sem instantes: rode a sonda de novo")
	}
	for _, c := range g.Instantes {
		t.Run(c.Texto, func(t *testing.T) {
			i, ok := LerInstante(c.Texto)
			if !ok {
				t.Fatal("não leu")
			}
			if j, _ := json.Marshal(i); string(j) != c.JSON {
				t.Errorf("JSON %s, o C# escreve %s", j, c.JSON)
			}
			if i.TextoRedondo() != c.Redondo {
				t.Errorf(`ToString("O") %s, o C# escreve %s`, i.TextoRedondo(), c.Redondo)
			}
			if (c.Tipo == "Unspecified") != (i.tipo == semFuso) || (c.Tipo == "Utc") != (i.tipo == utc) {
				t.Errorf("DateTimeKind %s, mas o tipo lido é %d", c.Tipo, i.tipo)
			}
		})
	}
	t.Run("DateTime.SpecifyKind(..., Utc) no JSON", func(t *testing.T) {
		if j, _ := json.Marshal(InstanteUTC(time.Date(2026, 10, 14, 21, 0, 0, 0, time.UTC))); string(j) != g.AgoraEmJSON {
			t.Errorf("JSON %s, o C# escreve %s", j, g.AgoraEmJSON)
		}
	})
}

func TestDefinirData(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	r := testeapoio.RelogioDosTestes() // 23/10/2026 02:30 UTC
	if err := Novo(db, r).DefinirData(t.Context(), "Cambio.UltimaAtualizacao", time.Date(2026, 10, 15, 1, 2, 3, 123456789, time.UTC)); err != nil {
		t.Fatal(err)
	}
	var valor, protegido, em string
	if err := db.QueryRow(`SELECT Valor, Protegido, AtualizadoEm FROM Configuracao WHERE Chave = 'Cambio.UltimaAtualizacao'`).Scan(&valor, &protegido, &em); err != nil {
		t.Fatal(err)
	}
	// DateTime.UtcNow.ToString("O") (7 casas, a precisão do C#); o AtualizadoEm é o do relógio, no formato do EF Core.
	if valor != "2026-10-15T01:02:03.1234567Z" || protegido != "0" || em != "2026-10-23 02:30:00" {
		t.Errorf("gravado %q, %q, %q", valor, protegido, em)
	}
	lido, err := Novo(db, r).ObterData(t.Context(), "Cambio.UltimaAtualizacao")
	if err != nil || lido == nil || !lido.Tempo().Equal(time.Date(2026, 10, 15, 1, 2, 3, 123456700, time.UTC)) {
		t.Errorf("lido de volta %v, %v", lido, err)
	}
}
