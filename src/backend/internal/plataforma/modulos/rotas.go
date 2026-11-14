package modulos

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tarefas"
)

// Rotas é o mux que o módulo recebe no Registrar: cada rota passa antes pela checagem de ligado.
type Rotas struct {
	mux    *http.ServeMux
	id     string
	nome   string
	ligado func(context.Context) (bool, error)
	log    *slog.Logger
}

// HandleFunc registra a rota do módulo, guardada. O caminho tem de começar em /api/<id> (o padrão do Go 1.22, com
// ou sem o método: "GET /api/trabalho/emprego"); fora disso é pânico na montagem, para o módulo não esconder rota fora
// do prefixo dele, sem a checagem.
func (r *Rotas) HandleFunc(padrao string, h http.HandlerFunc) {
	caminho := padrao
	if _, depois, temMetodo := strings.Cut(padrao, " "); temMetodo {
		caminho = depois
	}
	prefixo := "/api/" + r.id
	if caminho != prefixo && !strings.HasPrefix(caminho, prefixo+"/") {
		panic(fmt.Sprintf("rota %q do módulo %s fora de %s/", padrao, r.id, prefixo))
	}
	r.mux.HandleFunc(padrao, func(w http.ResponseWriter, req *http.Request) {
		ligado, err := r.ligado(req.Context())
		if err != nil {
			r.log.ErrorContext(req.Context(), "módulo: ler se está ligado", "modulo", r.id, "erro", err)
			respostas.SemCorpo(w, http.StatusInternalServerError)
			return
		}
		if !ligado {
			respostas.Mensagem(w, http.StatusConflict, MensagemDesligado(r.nome))
			return
		}
		h(w, req)
	})
}

// Montar põe no mux as rotas do catálogo (GET /api/modulos e PUT /api/modulos/{id}) e as de cada módulo, guardadas.
func (c *Catalogo) Montar(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/modulos", c.listarHTTP)
	mux.HandleFunc("PUT /api/modulos/{id}", c.definirHTTP)
	for _, m := range c.modulos {
		i := m.Info()
		m.Registrar(&Rotas{mux: mux, id: i.ID, nome: i.Nome, log: c.log,
			ligado: func(ctx context.Context) (bool, error) { return c.Ligado(ctx, i.ID) }})
	}
}

func (c *Catalogo) listarHTTP(w http.ResponseWriter, r *http.Request) {
	lista, err := c.Listar(r.Context())
	if err != nil {
		c.log.ErrorContext(r.Context(), "módulos: listar", "erro", err)
		respostas.SemCorpo(w, http.StatusInternalServerError)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, lista)
}

func (c *Catalogo) definirHTTP(w http.ResponseWriter, r *http.Request) {
	ligado := &respostas.CampoBoolOpcional{Propriedade: "Ligado"}
	if !respostas.LerObjeto(w, r, "dto", "DefinirModuloDto", ligado) {
		return
	}
	if ligado.Valor == nil {
		respostas.Validacao(w, map[string][]string{"Ligado": {respostas.Obrigatorio("Ligado")}})
		return
	}
	s, err := c.Definir(r.Context(), r, r.PathValue("id"), *ligado.Valor)
	switch {
	case errors.Is(err, ErrDesconhecido):
		respostas.SemCorpo(w, http.StatusNotFound)
	case err != nil:
		c.log.ErrorContext(r.Context(), "módulos: ligar ou desligar", "modulo", r.PathValue("id"), "erro", err)
		respostas.SemCorpo(w, http.StatusInternalServerError)
	default:
		respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, s)
	}
}

// IniciarTarefas põe para rodar as tarefas de todos os módulos, cada uma pelo tarefas.Periodica. Cada ciclo pergunta
// antes se o módulo está ligado: desligado, não roda (nem sai nada para a internet). Quem para é o ctx; quem espera é o
// emAndamento de quem chamou.
func (c *Catalogo) IniciarTarefas(ctx context.Context, emAndamento *sync.WaitGroup) {
	for _, m := range c.modulos {
		id := m.Info().ID
		for _, t := range m.Tarefas() {
			ciclo := c.cicloSeLigado(id, t.Ciclo)
			emAndamento.Go(func() { tarefas.Periodica(ctx, c.log, t.Nome, t.Atraso, t.Intervalo, ciclo) })
		}
	}
}

// cicloSeLigado é o ciclo da tarefa com a checagem de ligado na frente.
func (c *Catalogo) cicloSeLigado(id string, ciclo func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		ligado, err := c.Ligado(ctx, id)
		if err != nil || !ligado {
			return err
		}
		return ciclo(ctx)
	}
}
