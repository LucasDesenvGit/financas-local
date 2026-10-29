package relatorios

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
)

var errDeTeste = errors.New("falha de teste")

// dbQueFalha é o banco de verdade, mas a consulta que contém "quando" (a enésima vez que aparece) devolve erro. Serve
// para conferir que cada consulta que falha vira um erro, e não uma lista pela metade.
type dbQueFalha struct {
	consultas.DBTX
	quando string
	vez    int // a enésima consulta que contém "quando" falha (1 = a primeira)
	vistas *int
}

func falhaEm(db consultas.DBTX, quando string, vez int) dbQueFalha {
	return dbQueFalha{DBTX: db, quando: quando, vez: vez, vistas: new(0)}
}

func (d dbQueFalha) QueryContext(ctx context.Context, consulta string, argumentos ...any) (*sql.Rows, error) {
	if strings.Contains(consulta, d.quando) {
		*d.vistas++
		if *d.vistas == d.vez {
			return nil, errDeTeste
		}
	}
	return d.DBTX.QueryContext(ctx, consulta, argumentos...)
}

// Cada consulta que falha vira um erro que carrega a causa (errors.Is), em vez de uma lista pela metade.
func TestConsultaQueFalhaViraErro(t *testing.T) {
	t.Parallel()
	agora := testeapoio.HojeEmBrasilia(2026, 10, 22)
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, agora)
	categoria := fab.Categoria()
	fab.Orcamento(categoria)
	fab.ContaAgendada()
	// Uma transação antiga: a janela da renda tem meses e as consultas seguintes rodam.
	fab.Transacao(fab.Conta(), func(tr *testeapoio.Transacao) { tr.Data = dia(2025, 1, 1) })

	casos := []struct {
		nome   string
		quando string
		vez    int
		chama  func(s *Servico) (any, error)
	}{
		{"Relevantes", `FROM "Transacao"`, 1, func(s *Servico) (any, error) { return s.Relevantes(t.Context(), dia(2026, 10, 1), dia(2026, 10, 31)) }},
		{"TotaisPorCategoria, as transações", `FROM "Transacao"`, 1, func(s *Servico) (any, error) {
			return s.TotaisPorCategoria(t.Context(), dia(2026, 10, 1), dia(2026, 10, 31), 1, true, false)
		}},
		{"TotaisPorCategoria, as categorias", `FROM "Categoria"`, 1, func(s *Servico) (any, error) {
			return s.TotaisPorCategoria(t.Context(), dia(2026, 10, 1), dia(2026, 10, 31), 1, true, false)
		}},
		{"OrcamentosDoMes, os orçamentos", `FROM "Orcamento"`, 1, func(s *Servico) (any, error) { return s.OrcamentosDoMes(t.Context(), 2026, 10) }},
		{"OrcamentosDoMes, as transações", `FROM "Transacao"`, 1, func(s *Servico) (any, error) { return s.OrcamentosDoMes(t.Context(), 2026, 10) }},
		{"OrcamentosDoMes, as categorias", `FROM "Categoria"`, 1, func(s *Servico) (any, error) { return s.OrcamentosDoMes(t.Context(), 2026, 10) }},
		{"ContasAgendadas", `FROM "ContaAgendada"`, 1, func(s *Servico) (any, error) { return s.ContasAgendadas(t.Context(), nil) }},
		{"RendaEParcelas, a janela", `FROM "Transacao"`, 1, func(s *Servico) (any, error) { _, _, _, err := s.RendaEParcelas(t.Context()); return nil, err }},
		{"RendaEParcelas, o último mês", `FROM "Transacao"`, 2, func(s *Servico) (any, error) { _, _, _, err := s.RendaEParcelas(t.Context()); return nil, err }},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			s := NovoServico(falhaEm(db, c.quando, c.vez), agora)
			if _, err := c.chama(s); !errors.Is(err, errDeTeste) {
				t.Fatalf("erro = %v, esperado o erro do banco embrulhado", err)
			}
		})
	}
}

// A primeira transação é lida com QueryRow: com o contexto cancelado, o erro chega a quem chamou.
func TestMesesCompletosComContextoCancelado(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	s := NovoServico(db, testeapoio.HojeEmBrasilia(2026, 10, 22))
	ctx, cancelar := context.WithCancel(t.Context())
	cancelar()
	if _, _, _, err := s.MesesCompletos(ctx, 6); !errors.Is(err, context.Canceled) {
		t.Errorf("MesesCompletos: %v, esperado context.Canceled", err)
	}
	if _, _, err := s.JanelaHistorico(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("JanelaHistorico: %v, esperado context.Canceled", err)
	}
	if _, _, _, err := s.RendaEParcelas(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("RendaEParcelas: %v, esperado context.Canceled", err)
	}
}

// Dado que o .NET não gravaria (dinheiro ou data que não é do formato) vira erro na leitura, e não um zero calado.
func TestDadoForaDoFormatoViraErro(t *testing.T) {
	t.Parallel()
	agora := testeapoio.HojeEmBrasilia(2026, 10, 22)
	casos := []struct {
		nome    string
		estraga string // o UPDATE que deixa a única linha da tabela fora do formato
		chama   func(ctx context.Context, s *Servico) error
	}{
		{"valor da transação", `UPDATE Transacao SET Valor = 'dez reais'`, func(ctx context.Context, s *Servico) error {
			_, err := s.Relevantes(ctx, dia(2026, 10, 1), dia(2026, 10, 31))
			return err
		}},
		{"valor do orçamento", `UPDATE Orcamento SET Valor = '1x'`, func(ctx context.Context, s *Servico) error {
			_, err := s.OrcamentosDoMes(ctx, 2026, 10)
			return err
		}},
		{"vencimento da conta a pagar", `UPDATE ContaAgendada SET ProximoVencimento = 'amanhã'`, func(ctx context.Context, s *Servico) error {
			_, err := s.ContasAgendadas(ctx, nil)
			return err
		}},
		{"valor da conta a pagar", `UPDATE ContaAgendada SET Valor = '1x'`, func(ctx context.Context, s *Servico) error {
			_, err := s.ContasAgendadas(ctx, nil)
			return err
		}},
		{"data da primeira transação", `UPDATE Transacao SET Data = 'ontem'`, func(ctx context.Context, s *Servico) error {
			_, _, _, err := s.MesesCompletos(ctx, 6)
			return err
		}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			db := testeapoio.NovoBancoVazio(t)
			fab := testeapoio.NovaFabrica(t, db, agora)
			fab.Transacao(fab.Conta(), func(tr *testeapoio.Transacao) { tr.Data = dia(2026, 10, 10) })
			fab.Orcamento(fab.Categoria())
			fab.ContaAgendada()
			if _, err := db.ExecContext(t.Context(), c.estraga); err != nil {
				t.Fatal(err)
			}
			s := NovoServico(db, agora)
			if err := c.chama(t.Context(), s); err == nil {
				t.Error("o dado fora do formato devia dar erro")
			}
		})
	}
}
