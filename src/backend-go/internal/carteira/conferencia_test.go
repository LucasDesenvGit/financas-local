package carteira

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cotacoes"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

type passoDaConferencia struct {
	Nome       string            `json:"nome"`
	Resultado  json.RawMessage   `json:"resultado"`
	Ativos     []json.RawMessage `json:"ativos"`
	Operacoes  []json.RawMessage `json:"operacoes"`
	Transacoes []json.RawMessage `json:"transacoes"`
	Auditoria  []json.RawMessage `json:"auditoria"`
	Trocos     []json.RawMessage `json:"trocos"`
}

// cotacoesDaConferencia são as cotações com as respostas de testdata/conferencia-respostas.json, o mesmo arquivo da sonda.
func cotacoesDaConferencia(t *testing.T, db *sql.DB) *cotacoes.Servico {
	t.Helper()
	dados, err := os.ReadFile("testdata/conferencia-respostas.json")
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
	srv := testeapoio.NovoServicoDeFora(t)
	srv.Em("GET", "*", testeapoio.RespostaDeFora{Status: 404, Corpo: "{}"})
	for caminho, lista := range rotas {
		srv.Em("GET", caminho, testeapoio.RespostaDeFora{Status: lista[0].Status, Corpo: lista[0].Corpo})
	}
	cliente, err := cotacoes.NovoCliente(srv.URL(), srv.URL(), srv.Cliente())
	if err != nil {
		t.Fatal(err)
	}
	return cotacoes.NovoServico(db, cliente, nil, nil, testeapoio.HojeEmBrasilia(2026, 10, 30), slog.New(slog.DiscardHandler))
}

func valor(t *testing.T, texto string) tipos.Decimal {
	t.Helper()
	return testeapoio.Valor(texto)
}

// Regra: a conferência é a do ConferenciaInvestimentosService do C#, passo a passo, sobre o mesmo cenário
// (testdata/conferencia.json): o que ela aponta, a correção, o desfazer, os trocos automáticos e cada erro de corrigir, com
// o banco depois de cada passo.
func TestConferenciaIgualAoCSharp(t *testing.T) {
	dados, err := os.ReadFile("testdata/conferencia.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Passos []passoDaConferencia `json:"passos"`
	}
	if err := json.Unmarshal(dados, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Passos) < 40 {
		t.Fatalf("%d passos: a sonda mudou?", len(g.Passos))
	}
	db := testeapoio.NovoBancoVazio(t)
	cenario, err := os.ReadFile("testdata/conferencia-cenario.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(cenario)); err != nil {
		t.Fatal(err)
	}
	r := testeapoio.HojeEmBrasilia(2026, 10, 30)
	s := NovoServico(db, nil, nil, nil, cotacoesDaConferencia(t, db), cofreFalso{}, r, slog.New(slog.DiscardHandler))
	pedido := httptest.NewRequest("POST", "/", nil)
	login := "teste"
	var chave *string

	corrigir := func(ctx context.Context, e EntradaDaCorrecao) (any, error) {
		resultado, mensagem, err := s.Corrigir(ctx, pedido, e, &login)
		if err != nil {
			return nil, err
		}
		saida := map[string]any{"erro": nil, "tipos": nil, "temChave": false}
		if mensagem != "" {
			saida["erro"] = mensagem
		}
		if resultado != nil {
			saida["tipos"], saida["temChave"] = resultado.Tipos, resultado.Desfazer != nil
		}
		return saida, nil
	}
	item := func(id int64, ticker string, tipo int64, q, p string) ItemDoExtrato {
		return ItemDoExtrato{TransacaoId: id, Ticker: ticker, Tipo: tipo, Quantidade: valor(t, q), PrecoUnitario: valor(t, p), Taxas: tipos.Zero}
	}
	falt := func(id int64) EntradaDaCorrecao {
		return EntradaDaCorrecao{Compras: []ItemDoExtrato{item(id, "FALT11", 1, "1", "1")}}
	}
	acoes := map[string]func(ctx context.Context) (any, error){
		"conferir": func(ctx context.Context) (any, error) { return s.Conferir(ctx) },
		"créditos como receita": func(ctx context.Context) (any, error) {
			quantidade, total, err := s.CreditosComoReceita(ctx)
			return map[string]any{"quantidade": quantidade, "total": total}, err
		},
		"proventos da corretora": func(ctx context.Context) (any, error) {
			lista, err := s.ProventosDaCorretora(ctx)
			saida := []map[string]any{}
			for _, p := range lista {
				saida = append(saida, map[string]any{"id": p.Id, "data": p.Data, "valor": p.Valor})
			}
			return saida, err
		},
		"corrigir": func(ctx context.Context) (any, error) {
			compra := ItemDoExtrato{TransacaoId: 131, Ticker: "falt11", Tipo: 1, Quantidade: valor(t, "20.1234567"), PrecoUnitario: valor(t, "50"), Taxas: valor(t, "0.125")}
			resultado, mensagem, err := s.Corrigir(ctx, pedido, EntradaDaCorrecao{AtivosParaFii: []int64{3, 4, 3}, Compras: []ItemDoExtrato{compra},
				Trocos: []int64{102, 103, 141}, Rendimentos: []int64{151, 213}, Taxas: []int64{1}}, &login)
			if err != nil || mensagem != "" {
				t.Fatalf("corrigir: %v %q", err, mensagem)
			}
			chave = resultado.Desfazer
			return map[string]any{"erro": nil, "tipos": resultado.Tipos, "compras": resultado.Compras, "trocos": resultado.Trocos, "rendimentos": resultado.Rendimentos,
				"taxas": resultado.Taxas, "temChave": chave != nil}, nil
		},
		"desfazer": func(ctx context.Context) (any, error) {
			n, ok, err := s.Desfazer(ctx, pedido, *chave, &login)
			if !ok {
				return nil, err
			}
			return n, err
		},
		"classificar os trocos": func(ctx context.Context) (any, error) { return s.ClassificarTrocosSemDuvida(ctx) },
		"resumo dos trocos":     func(ctx context.Context) (any, error) { return s.ResumoTrocosAutomaticos(ctx) },
		"desfazer os trocos": func(ctx context.Context) (any, error) {
			return s.DesfazerTrocosAutomaticos(ctx, pedido, &login)
		},
		"dispensar": func(ctx context.Context) (any, error) { return nil, s.DispensarTrocosAutomaticos(ctx) },
		"erro: ativo que não existe": func(ctx context.Context) (any, error) {
			return corrigir(ctx, EntradaDaCorrecao{AtivosParaFii: []int64{999}})
		},
		"erro: exterior para FII": func(ctx context.Context) (any, error) {
			return corrigir(ctx, EntradaDaCorrecao{AtivosParaFii: []int64{9}})
		},
		"erro: taxa que mudou": func(ctx context.Context) (any, error) { return corrigir(ctx, EntradaDaCorrecao{Taxas: []int64{999}}) },
		"erro: mesmo débito duas vezes": func(ctx context.Context) (any, error) {
			return corrigir(ctx, EntradaDaCorrecao{Compras: []ItemDoExtrato{item(131, "FALT11", 1, "1", "1"), item(131, "FALT11", 1, "1", "1")}})
		},
		"erro: débito já ligado":           func(ctx context.Context) (any, error) { return corrigir(ctx, falt(121)) },
		"erro: débito que não existe":      func(ctx context.Context) (any, error) { return corrigir(ctx, falt(999)) },
		"erro: crédito no lugar do débito": func(ctx context.Context) (any, error) { return corrigir(ctx, falt(102)) },
		"erro: ticker fora da carteira": func(ctx context.Context) (any, error) {
			return corrigir(ctx, EntradaDaCorrecao{Compras: []ItemDoExtrato{item(133, "ZZZZ3", 1, "1", "1")}})
		},
		"erro: ticker inválido": func(ctx context.Context) (any, error) {
			return corrigir(ctx, EntradaDaCorrecao{Compras: []ItemDoExtrato{item(133, "X", 1, "1", "1")}})
		},
		"erro: venda no lugar da compra": func(ctx context.Context) (any, error) {
			return corrigir(ctx, EntradaDaCorrecao{Compras: []ItemDoExtrato{item(133, "FALT11", 2, "1", "1")}})
		},
		"erro: sem cotas": func(ctx context.Context) (any, error) {
			return corrigir(ctx, EntradaDaCorrecao{Compras: []ItemDoExtrato{item(133, "FALT11", 1, "0", "1")}})
		},
		"erro: sem preço": func(ctx context.Context) (any, error) {
			return corrigir(ctx, EntradaDaCorrecao{Compras: []ItemDoExtrato{item(133, "FALT11", 1, "1", "0")}})
		},
		"erro: troco e rendimento": func(ctx context.Context) (any, error) {
			return corrigir(ctx, EntradaDaCorrecao{Trocos: []int64{102}, Rendimentos: []int64{102}})
		},
		"erro: crédito que não existe":     func(ctx context.Context) (any, error) { return corrigir(ctx, EntradaDaCorrecao{Trocos: []int64{999}}) },
		"erro: débito no lugar do crédito": func(ctx context.Context) (any, error) { return corrigir(ctx, EntradaDaCorrecao{Trocos: []int64{101}}) },
		"erro: nada a corrigir":            func(ctx context.Context) (any, error) { return corrigir(ctx, EntradaDaCorrecao{}) },
		"erro: provento que as regras não mudam": func(ctx context.Context) (any, error) {
			return corrigir(ctx, EntradaDaCorrecao{Rendimentos: []int64{181}})
		},
	}
	guardarChave := func(ctx context.Context, e EntradaDaCorrecao) (any, error) {
		resultado, mensagem, err := s.Corrigir(ctx, pedido, e, &login)
		if err != nil {
			return nil, err
		}
		saida := map[string]any{"erro": nil, "tipos": nil, "trocos": nil, "taxas": nil, "temChave": false}
		if mensagem != "" {
			saida["erro"] = mensagem
		}
		if resultado != nil {
			chave = resultado.Desfazer
			saida["tipos"], saida["trocos"], saida["taxas"], saida["temChave"] = resultado.Tipos, resultado.Trocos, resultado.Taxas, chave != nil
		}
		return saida, nil
	}
	acoes["corrigir as taxas"] = func(ctx context.Context) (any, error) {
		return guardarChave(ctx, EntradaDaCorrecao{Taxas: []int64{1, 14, 15}})
	}
	acoes["corrigir os créditos"] = func(ctx context.Context) (any, error) {
		return guardarChave(ctx, EntradaDaCorrecao{Trocos: []int64{122, 151, 132, 113}})
	}
	acoes["troco sem categoria"] = func(ctx context.Context) (any, error) {
		return guardarChave(ctx, EntradaDaCorrecao{Trocos: []int64{132}})
	}
	acoes["rendimento sem categoria do troco"] = func(ctx context.Context) (any, error) {
		return guardarChave(ctx, EntradaDaCorrecao{Rendimentos: []int64{213}})
	}
	acoes["erro: ativo que já é FII"] = func(ctx context.Context) (any, error) {
		return corrigir(ctx, EntradaDaCorrecao{AtivosParaFii: []int64{6}})
	}
	acoes["desfazer as taxas"] = acoes["desfazer"]
	acoes["desfazer os créditos"] = acoes["desfazer"]
	acoes["desfazer os trocos sem nada guardado"] = acoes["desfazer os trocos"]
	acoes["resumo com os trocos mudados"] = acoes["resumo dos trocos"]
	acoes["resumo com o texto inválido"] = acoes["resumo dos trocos"]
	acoes["conferir com outra categoria"] = acoes["conferir"]
	acoes["conferir sem categoria"] = acoes["conferir"]
	acoes["conferir sem operações"] = acoes["conferir"]
	acoes["conferir depois de corrigir"] = acoes["conferir"]
	acoes["desfazer de novo"] = acoes["desfazer"]
	acoes["resumo depois de desfazer"] = acoes["resumo dos trocos"]
	acoes["classificar de novo"] = acoes["classificar os trocos"]

	for _, p := range g.Passos {
		if sql, ok := strings.CutPrefix(p.Nome, "mudar: "); ok {
			if _, err := db.Exec(sql); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if !t.Run(p.Nome, func(t *testing.T) {
			acao, ok := acoes[p.Nome]
			if !ok {
				t.Fatalf("passo sem ação: %q", p.Nome)
			}
			resultado, err := acao(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			obtido, err := json.Marshal(resultado)
			if err != nil {
				t.Fatal(err)
			}
			testeapoio.ConferirJSON(t, obtido, string(p.Resultado))
			conferirTabela(t, db, `SELECT Id, Tipo FROM Ativo ORDER BY Id`, p.Ativos)
			conferirTabela(t, db, `SELECT Id, AtivoId, Tipo, Data, Quantidade, PrecoUnitario, Taxas, Origem, TransacaoId, Observacao FROM OperacaoAtivo ORDER BY Id`, p.Operacoes)
			conferirTabela(t, db, `SELECT Id, CategoriaId, CategoriaManual, Tipo, IgnorarRelatorios FROM Transacao ORDER BY Id`, p.Transacoes)
			conferirTabela(t, db, `SELECT Evento, Login FROM AuditoriaAcesso ORDER BY Id`, p.Auditoria)
			conferirTabela(t, db, `SELECT Valor FROM Configuracao WHERE Chave = 'Investimentos.TrocosAutomaticos'`, p.Trocos)
		}) {
			t.FailNow() // cada passo parte do banco do anterior
		}
	}
}
