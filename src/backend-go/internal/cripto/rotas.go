package cripto

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/erros"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/servidor"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/sessao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tarefas"
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
}

// NovaTarefa cria a tarefa.
func NovaTarefa(s *Servico, log *slog.Logger) *Tarefa { return &Tarefa{servico: s, log: log} }

// Rodar roda a tarefa até o ctx acabar (chame numa goroutine).
func (t *Tarefa) Rodar(ctx context.Context) {
	tarefas.Periodica(ctx, t.log, "a cotação de criptomoedas", AtrasoDaTarefa, IntervaloDaTarefa, t.Ciclo)
}

// Ciclo atualiza uma vez. Sem internet ou fonte fora do ar é aviso (tenta de novo no próximo ciclo; a tela mostra a
// última guardada); o resto volta como erro, que o tarefas.Periodica registra.
func (t *Tarefa) Ciclo(ctx context.Context) error {
	resultado, err := t.servico.Atualizar(ctx)
	if err != nil {
		if ehDeRede(err) && ctx.Err() == nil {
			t.log.Warn("Não foi possível consultar a cotação de criptomoedas no Mercado Bitcoin.", "origem", erros.OrigemCripto, "erro", err)
			return nil
		}
		return err
	}
	t.log.Info("Cotação de criptomoedas atualizada.", "origem", erros.OrigemCripto, "novas", resultado.Novas)
	return nil
}
