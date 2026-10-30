package cotacoes

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// cofreFalso guarda o segredo às claras: o teste é da cotação, e não da cifra.
type cofreFalso struct{}

func (cofreFalso) Cifrar(_, segredo string) (string, error)   { return segredo, nil }
func (cofreFalso) Decifrar(_, gravado string) (string, error) { return gravado, nil }

type semHistorico struct{}

func (semHistorico) GarantirHistorico(context.Context, string, tipos.Data) error { return nil }

// respostasDoTeste carrega testdata/respostas.json num serviço falso: o mesmo arquivo que a sonda usou.
func respostasDoTeste(t *testing.T) *testeapoio.ServicoDeFora {
	t.Helper()
	dados, err := os.ReadFile("testdata/respostas.json")
	if err != nil {
		t.Fatal(err)
	}
	var rotas map[string][]struct {
		Status int    `json:"status"`
		Corpo  string `json:"corpo"`
	}
	if err := json.Unmarshal(dados, &rotas); err != nil {
		t.Fatal(err)
	}
	srv := testeapoio.NovoServicoDeFora(t, "Authorization")
	for caminho, lista := range rotas {
		var respostas []testeapoio.RespostaDeFora
		for _, r := range lista {
			respostas = append(respostas, testeapoio.RespostaDeFora{Status: r.Status, Corpo: r.Corpo})
		}
		srv.Em("GET", caminho, respostas...)
	}
	return srv
}

func linhasDe(t *testing.T, db *sql.DB, consulta string) []map[string]any {
	t.Helper()
	linhas, err := db.Query(consulta)
	if err != nil {
		t.Fatal(err)
	}
	defer linhas.Close()
	colunas, err := linhas.Columns()
	if err != nil {
		t.Fatal(err)
	}
	lista := []map[string]any{}
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
		lista = append(lista, linha)
	}
	return lista
}

func conferirIgual(t *testing.T, nome string, obtido any, esperado json.RawMessage) {
	t.Helper()
	dados, err := json.Marshal(obtido)
	if err != nil {
		t.Fatal(err)
	}
	if len(esperado) > 0 && string(esperado) != "null" {
		var a, b any
		if err := json.Unmarshal(dados, &a); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(esperado, &b); err != nil {
			t.Fatal(err)
		}
		if fmt.Sprint(a) != fmt.Sprint(b) {
			t.Errorf("%s:\n%s\no C# dá:\n%s", nome, dados, esperado)
		}
	}
}

// Regra: a atualização é a do CotacaoService do C#, com as mesmas respostas (testdata/respostas.json) sobre o mesmo cenário:
// os pedidos, na ordem e com o endereço exato, e o banco como ficou: o nome, o tipo corrigido pelo nome oficial, o preço com 4
// casas, os proventos, os fechamentos mensais (o último do mês, o existente corrigido), o IFIX, e o que é falha.
func TestAtualizarIgualAoCSharp(t *testing.T) {
	dados, err := os.ReadFile("testdata/atualizar.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Resultado []Resultado     `json:"resultado"`
		Ativos    json.RawMessage `json:"ativos"`
		Proventos json.RawMessage `json:"proventos"`
		Mensais   json.RawMessage `json:"mensais"`
		Indices   json.RawMessage `json:"indices"`
		Pedidos   []string        `json:"pedidos"`
	}
	if err := json.Unmarshal(dados, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Pedidos) < 25 {
		t.Fatal("gabarito pequeno: a sonda mudou?")
	}
	db := testeapoio.NovoBancoVazio(t)
	cenario, err := os.ReadFile("testdata/cenario.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(cenario)); err != nil {
		t.Fatal(err)
	}
	r := testeapoio.AgoraEmBrasilia(2026, 10, 30, 15, 0)
	if _, err := db.Exec(`INSERT INTO Configuracao (Chave, Valor, Protegido, AtualizadoEm) VALUES (?, 'token-de-teste', 1, '2026-01-01 00:00:00')`, ChaveTokenBrapi); err != nil {
		t.Fatal(err)
	}
	srv := respostasDoTeste(t)
	cliente, err := NovoCliente(srv.URL(), srv.URL(), srv.Cliente())
	if err != nil {
		t.Fatal(err)
	}
	s := NovoServico(db, cliente, semHistorico{}, cofreFalso{}, r, slog.New(slog.DiscardHandler))
	s.esperar = func(context.Context, time.Duration) error { return nil }
	obtido, err := s.Atualizar(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(obtido.Falhas, ",") != strings.Join(g.Resultado[0].Falhas, ",") || obtido.Atualizados != g.Resultado[0].Atualizados || obtido.ProventosNovos != g.Resultado[0].ProventosNovos {
		t.Errorf("resultado %+v, o C# dá %+v", obtido, g.Resultado[0])
	}
	if pedidos := strings.Join(srv.URIs(), "\n"); pedidos != strings.Join(g.Pedidos, "\n") {
		t.Errorf("pedidos:\n%s\no C# pede:\n%s", pedidos, strings.Join(g.Pedidos, "\n"))
	}
	conferirIgual(t, "ativos", linhasDe(t, db, `SELECT Id, Ticker, Nome, Tipo, PrecoAtual, PrecoAnterior, PrecoAtualizadoEm IS NOT NULL AS Atualizado, FonteCotacao FROM Ativo ORDER BY Id`), g.Ativos)
	conferirIgual(t, "proventos", linhasDe(t, db, `SELECT AtivoId, DataCom, ValorPorCota FROM ProventoAtivo ORDER BY AtivoId, DataCom`), g.Proventos)
	conferirIgual(t, "mensais", linhasDe(t, db, `SELECT AtivoId, Mes, Fechamento FROM PrecoMensalAtivo ORDER BY AtivoId, Mes`), g.Mensais)
	conferirIgual(t, "índices", linhasDe(t, db, `SELECT Codigo, Mes, Fechamento FROM IndiceMensal ORDER BY Codigo, Mes`), g.Indices)
}

// Regra: "parece FII" é o do C#: a sigla como palavra (com o \b de Unicode do .NET), fundo + imobiliário, e nunca fundo de
// índice.
func TestPareceFiiIgualAoCSharp(t *testing.T) {
	t.Parallel()
	dados, err := os.ReadFile("testdata/parecefii.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Nomes []struct {
			Nome *string `json:"nome"`
			Fii  bool    `json:"fii"`
		} `json:"nomes"`
	}
	if err := json.Unmarshal(dados, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Nomes) < 20 {
		t.Fatal("gabarito pequeno: a sonda mudou?")
	}
	for _, c := range g.Nomes {
		if PareceFii(c.Nome) != c.Fii {
			t.Errorf("PareceFii(%v) = %v", c.Nome, !c.Fii)
		}
	}
}
