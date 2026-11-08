package selic

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
)

// relogioQueAnda é o relógio dos testes da pausa: o teste o faz andar entre uma abertura e outra.
type relogioQueAnda struct{ agora time.Time }

func (r *relogioQueAnda) Agora() time.Time { return r.agora }

// servicoQueAnda é o serviço num banco vazio, com hoje em 30/10/2026, às 15h em Brasília, e o relógio que anda.
func servicoQueAnda(t *testing.T, f *fonteFalsa) (*Servico, *relogioQueAnda, *bytes.Buffer) {
	t.Helper()
	r := &relogioQueAnda{agora: testeapoio.AgoraEmBrasilia(2026, 10, 30, 15, 0).Agora()}
	var saida bytes.Buffer
	return NovoServico(testeapoio.NovoBancoVazio(t), f, r, slog.New(slog.NewTextHandler(&saida, nil))), r, &saida
}

// abrirInvestimentos faz o que o GET /api/carteira faz com a Selic: a série e a meta.
func abrirInvestimentos(t *testing.T, s *Servico) {
	t.Helper()
	if _, err := s.ObterSerie(t.Context(), outubro(1)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MetaAtual(t.Context()); err != nil {
		t.Fatal(err)
	}
}

// Regra (#534): depois de uma falha do Banco Central, as aberturas dos próximos 15 minutos não vão a ele, nem pela
// série nem pela meta, e não avisam de novo; aos 15 minutos, a primeira abertura tenta outra vez. Sem internet, só a
// primeira abertura espera a falha.
func TestPausaDepoisDaFalha(t *testing.T) {
	t.Parallel()
	semInternet := errors.New("sem internet")
	f := &fonteFalsa{errosEm: map[string]error{"2026-10-01": semInternet}, erroMeta: semInternet}
	s, r, saida := servicoQueAnda(t, f)

	passos := []struct {
		nome                   string
		andar                  time.Duration
		internetVoltou         bool
		pedidos, metas, avisos int
	}{
		{"a primeira abertura espera a falha da série, e a meta já fica na pausa", 0, false, 1, 0, 1},
		{"logo depois, nada vai ao Banco Central nem avisa", time.Second, false, 1, 0, 1},
		{"14min59s depois da falha, ainda na pausa", 14*time.Minute + 58*time.Second, false, 1, 0, 1},
		{"aos 15 minutos, tenta de novo e começa outra pausa", time.Second, false, 2, 0, 2},
		{"a internet voltou: passada a pausa, a série e a meta respondem", 15 * time.Minute, true, 3, 1, 2},
		{"com a resposta, a abertura seguinte não espera pausa nenhuma", time.Second, true, 4, 1, 2},
	}
	for _, p := range passos {
		r.agora = r.agora.Add(p.andar)
		if p.internetVoltou {
			f.errosEm, f.erroMeta, f.meta = nil, nil, ptr("15.00")
		}
		abrirInvestimentos(t, s)
		avisos := strings.Count(saida.String(), "level=WARN")
		if len(f.pedidos) != p.pedidos || f.metas != p.metas || avisos != p.avisos {
			t.Fatalf("%s: %d pedidos da série, %d da meta e %d avisos; esperado %d, %d e %d", p.nome, len(f.pedidos), f.metas,
				avisos, p.pedidos, p.metas, p.avisos)
		}
	}
}

// Regra (#534): a falha da meta também começa a pausa, e a série espera junto: é o mesmo Banco Central.
func TestPausaPelaMeta(t *testing.T) {
	t.Parallel()
	f := &fonteFalsa{erroMeta: errors.New("sem internet")}
	s, r, _ := servicoQueAnda(t, f)
	if _, err := s.MetaAtual(t.Context()); err != nil {
		t.Fatal(err)
	}
	r.agora = r.agora.Add(time.Minute)
	if _, err := s.ObterSerie(t.Context(), outubro(1)); err != nil {
		t.Fatal(err)
	}
	if f.metas != 1 || len(f.pedidos) != 0 {
		t.Errorf("%d pedidos da meta e %d da série; esperado 1 e 0", f.metas, len(f.pedidos))
	}
}

// Regra (#534): o que não é falha do Banco Central não começa a pausa: o pedido cancelado (quem desistiu foi o
// navegador) e o erro ao gravar no banco (o Banco Central respondeu).
func TestOQueNaoComecaAPausa(t *testing.T) {
	t.Parallel()
	t.Run("pedido cancelado durante a chamada", func(t *testing.T) {
		t.Parallel()
		// O ObterSerie para no banco com o contexto já cancelado, antes de chegar ao Banco Central: aqui o pedido
		// é cancelado no meio da chamada, como quando o navegador desiste esperando a resposta.
		s, _, _ := servicoQueAnda(t, &fonteFalsa{})
		ctx, cancelar := context.WithCancel(t.Context())
		err := s.perguntarAoBancoCentral(ctx, func() error {
			cancelar()
			return context.Canceled
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("erro %v", err)
		}
		chamou := false
		if err := s.perguntarAoBancoCentral(t.Context(), func() error { chamou = true; return nil }); err != nil || !chamou {
			t.Errorf("depois do cancelado: erro %v, chamou %v", err, chamou)
		}
	})
	t.Run("erro ao gravar", func(t *testing.T) {
		t.Parallel()
		f := &fonteFalsa{taxas: map[string][]Taxa{"2026-10-01": {umaTaxa(outubro(1), "0.05")}}, meta: ptr("15.00")}
		s, db, _, _ := servicoDeTeste(t, f)
		if _, err := db.Exec(`CREATE TRIGGER nao_grava BEFORE INSERT ON TaxaSelicDiaria
			BEGIN SELECT RAISE(ABORT, 'não grava'); END`); err != nil {
			t.Fatal(err)
		}
		abrirInvestimentos(t, s)
		abrirInvestimentos(t, s)
		if len(f.pedidos) != 2 || f.metas != 1 {
			t.Errorf("%d pedidos da série e %d da meta; esperado 2 e 1 (a meta fica guardada por 12 horas)", len(f.pedidos), f.metas)
		}
	})
}

func ptr(s string) *string { return &s }
