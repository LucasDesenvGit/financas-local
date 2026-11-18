package cripto

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/erros"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/servidor"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/sessao"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tarefas"
)

// Registrar põe GET /api/cripto/cotacoes no mux (o CriptoController), com o cliente de verdade.
func Registrar(mux *http.ServeMux, db *sql.DB, log *slog.Logger, r relogio.Relogio) {
	RegistrarCom(mux, NovoServico(db, ClientePadrao(), r, log), log)
}

// RegistrarCom é o Registrar com o serviço pronto (os testes passam um com o Mercado Bitcoin do testeapoio.ServicoDeFora).
func RegistrarCom(mux *http.ServeMux, s *Servico, log *slog.Logger) {
	mux.HandleFunc("GET /api/cripto/cotacoes", func(w http.ResponseWriter, req *http.Request) {
		if _, ok := sessao.Usuario(req.Context()); !ok {
			w.WriteHeader(http.StatusUnauthorized) // o .NET já conferiu o login; chegar sem é requisição de fora
			return
		}
		cotacoes, err := s.Ultimas(req.Context())
		if err != nil {
			if req.Context().Err() == nil {
				log.Error("cotação de criptomoedas", "origem", erros.OrigemCripto, "erro", err)
				servidor.ResponderErroInterno(w)
			}
			return
		}
		respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, cotacoes)
	})
}

// ClientePadrao é o cliente de produção (HTTPS, 20 s).
func ClientePadrao() *Cliente {
	c, err := NovoCliente(URLPadrao, nil)
	if err != nil {
		panic(err) // a URL é uma constante: erro aqui é bug
	}
	return c
}

// Agenda do CriptoHostedService: 1 minuto depois de subir e, dali em diante, a cada 30 minutos. Cripto negocia 24 h
// por dia: não há horário de fechamento como na PTAX.
const (
	AtrasoDaTarefa    = time.Minute
	IntervaloDaTarefa = 30 * time.Minute
)

// Tarefa é o CriptoHostedService.
type Tarefa struct {
	servico *Servico
	log     *slog.Logger
	// recusa é a última recusa da fonte já registrada ("status 403"), para não registrar a mesma 48 vezes por dia
	// (#762); volta a "" quando a cotação atualiza. Só a goroutine da tarefa lê e escreve.
	recusa string
}

// NovaTarefa cria a tarefa.
func NovaTarefa(s *Servico, log *slog.Logger) *Tarefa { return &Tarefa{servico: s, log: log} }

// Rodar roda a tarefa até o ctx acabar (chame numa goroutine).
func (t *Tarefa) Rodar(ctx context.Context) {
	tarefas.Periodica(ctx, t.log, "a cotação de criptomoedas", AtrasoDaTarefa, IntervaloDaTarefa, t.Ciclo)
}

// Ciclo atualiza uma vez. Sem internet ou fonte fora do ar é aviso (tenta de novo no próximo ciclo; a tela mostra a
// última guardada). A fonte que recusa o pedido (#762) é erro, registrado uma vez enquanto a recusa for a mesma, com o
// aviso de quando voltar. O resto volta como erro, que o tarefas.Periodica registra.
func (t *Tarefa) Ciclo(ctx context.Context) error {
	resultado, err := t.servico.Atualizar(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return err
		}
		if errors.Is(err, ErrRecusa) {
			if err.Error() != t.recusa {
				t.recusa = err.Error()
				t.log.Error("O Mercado Bitcoin recusou a consulta da cotação de criptomoedas: não é falta de internet. "+
					"A cotação fica parada na última guardada; o site tenta de novo a cada 30 minutos e só registra outra vez se a resposta mudar.",
					"origem", erros.OrigemCripto, "erro", err)
			}
			return nil
		}
		if ehDeRede(err) {
			t.log.Warn("Não foi possível consultar a cotação de criptomoedas no Mercado Bitcoin.", "origem", erros.OrigemCripto, "erro", err)
			return nil
		}
		return err
	}
	if t.recusa != "" {
		t.recusa = ""
		t.log.Info("O Mercado Bitcoin voltou a aceitar a consulta: a cotação de criptomoedas voltou a atualizar.", "origem", erros.OrigemCripto)
	}
	t.log.Info("Cotação de criptomoedas atualizada.", "origem", erros.OrigemCripto, "novas", resultado.Novas)
	return nil
}
