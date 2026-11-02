package categorizacao

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
)

// lerTabela devolve as linhas da consulta como o gabarito as grava: uma lista de objetos coluna → valor.
func lerTabela(t *testing.T, db *sql.DB, consulta string) []map[string]any {
	t.Helper()
	linhas, err := db.QueryContext(t.Context(), consulta)
	if err != nil {
		t.Fatal(err)
	}
	defer linhas.Close()
	colunas, _ := linhas.Columns()
	resultado := []map[string]any{}
	for linhas.Next() {
		valores := make([]any, len(colunas))
		ponteiros := make([]any, len(colunas))
		for i := range valores {
			ponteiros[i] = &valores[i]
		}
		if err := linhas.Scan(ponteiros...); err != nil {
			t.Fatal(err)
		}
		linha := map[string]any{}
		for i, c := range colunas {
			linha[c] = valores[i]
		}
		resultado = append(resultado, linha)
	}
	if err := linhas.Err(); err != nil {
		t.Fatal(err)
	}
	return resultado
}

// normalizar passa pelo JSON, como o gabarito: os números viram float64, e o resto fica como está.
func normalizar(t *testing.T, v any) any {
	t.Helper()
	dados, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var r any
	if err := json.Unmarshal(dados, &r); err != nil {
		t.Fatal(err)
	}
	return r
}

// Regra: as três rotinas de partida do CategorizacaoService (regras padrão novas, entradas nos cartões, compras
// parceladas estornadas) fazem em Go o que o C# faz, na ordem do Program.cs: o mesmo resultado e o mesmo banco depois de
// cada uma, e a segunda partida não muda nada. O gabarito é o testdata/acertos.json, gravado pela sonda
// (testdata/sonda-csharp) com o CategorizacaoService de verdade sobre o testdata/acertos-cenario.sql.
func TestAcertosIguaisAoCSharp(t *testing.T) {
	dados, err := os.ReadFile("testdata/acertos.json")
	if err != nil {
		t.Fatal(err)
	}
	var gabarito struct {
		Passos []struct {
			Nome       string
			Mudaram    int
			Transacoes []map[string]any
			Regras     []map[string]any
			Marcas     []map[string]any
		}
	}
	if err := json.Unmarshal(dados, &gabarito); err != nil {
		t.Fatal(err)
	}
	if len(gabarito.Passos) != 6 {
		t.Fatalf("o gabarito tem %d passos, esperado 6: a sonda mudou?", len(gabarito.Passos))
	}

	db := testeapoio.NovoBancoVazio(t)
	rel := testeapoio.AgoraEmBrasilia(2026, 10, 30, 12, 0)
	hojeDoCenario := relogio.Hoje(rel)
	cenario, err := os.ReadFile("testdata/acertos-cenario.sql")
	if err != nil {
		t.Fatal(err)
	}
	cenarioSQL := regexp.MustCompile(`@D(-?\d+)`).ReplaceAllStringFunc(string(cenario), func(m string) string {
		n, _ := strconv.Atoi(m[2:])
		return hojeDoCenario.SomarDias(n).String()
	})
	if _, err := db.ExecContext(t.Context(), cenarioSQL); err != nil {
		t.Fatal(err)
	}

	agora := agoraDe(rel)
	rotinas := map[string]func(tx *sql.Tx) (int, error){
		"regras padrão novas": func(tx *sql.Tx) (int, error) { return CriarRegrasPadrao(t.Context(), tx, rel) },
		"entradas nos cartões": func(tx *sql.Tx) (int, error) {
			return ReclassificarEntradasNosCartoes(t.Context(), tx, agora)
		},
		"compras parceladas estornadas": func(tx *sql.Tx) (int, error) {
			return IgnorarComprasParceladasEstornadas(t.Context(), tx, hojeDoCenario, agora)
		},
	}
	for _, passo := range gabarito.Passos {
		nome := passo.Nome
		if len(nome) > len("segunda partida: ") && nome[:len("segunda partida: ")] == "segunda partida: " {
			nome = nome[len("segunda partida: "):]
		}
		rotina, existe := rotinas[nome]
		if !existe {
			t.Fatalf("passo %q do gabarito sem rotina", passo.Nome)
		}
		mudaram := emTx(t, db, rotina)
		if mudaram != passo.Mudaram {
			t.Errorf("%s: mudaram %d, o C# deu %d", passo.Nome, mudaram, passo.Mudaram)
		}
		for _, tabela := range []struct {
			nome     string
			consulta string
			esperado []map[string]any
		}{
			{"Transacao", `SELECT Id, CategoriaId, CategoriaManual, Tipo, IgnorarRelatorios FROM Transacao ORDER BY Id`, passo.Transacoes},
			{"RegraCategorizacao", `SELECT Id, Texto, TipoCorrespondencia, CategoriaId, Prioridade, Ativa FROM RegraCategorizacao ORDER BY Id`, passo.Regras},
			{"Configuracao", `SELECT Chave, Valor IS NOT NULL AS TemValor, Protegido FROM Configuracao WHERE Chave LIKE 'Regras.%' ORDER BY Chave`, passo.Marcas},
		} {
			obtido := normalizar(t, lerTabela(t, db, tabela.consulta))
			esperado := normalizar(t, tabela.esperado)
			if !reflect.DeepEqual(obtido, esperado) {
				t.Errorf("%s, tabela %s: diferente do C#\n%s", passo.Nome, tabela.nome, diferenca(obtido, esperado))
			}
		}
	}
}

// diferenca aponta a primeira linha em que as duas listas diferem.
func diferenca(obtido, esperado any) string {
	a, _ := obtido.([]any)
	b, _ := esperado.([]any)
	for i := 0; i < max(len(a), len(b)); i++ {
		var x, y any
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if !reflect.DeepEqual(x, y) {
			return fmt.Sprintf("linha %d: Go %v, C# %v (Go %d linhas, C# %d)", i, x, y, len(a), len(b))
		}
	}
	return ""
}
