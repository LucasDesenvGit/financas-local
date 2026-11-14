package modulos

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
)

// moduloDeTeste é um módulo de exemplo: uma rota e uma tarefa que conta os ciclos.
type moduloDeTeste struct {
	info  Info
	rodou chan struct{} // recebe um sinal a cada ciclo da tarefa
	rotas []string      // padrões que ele tenta registrar
}

func (m *moduloDeTeste) Info() Info { return m.info }

func (m *moduloDeTeste) Registrar(r *Rotas) {
	padroes := m.rotas
	if padroes == nil {
		padroes = []string{"GET /api/" + m.info.ID + "/ola"}
	}
	for _, p := range padroes {
		r.HandleFunc(p, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`"ola"`)) })
	}
}

func (m *moduloDeTeste) Tarefas() []Tarefa {
	return []Tarefa{{Nome: "o teste", Atraso: time.Millisecond, Intervalo: time.Millisecond,
		Ciclo: func(context.Context) error {
			select {
			case m.rodou <- struct{}{}:
			default: // ninguém esperando: segue
			}
			return nil
		}}}
}

func exemplo() *moduloDeTeste {
	return &moduloDeTeste{info: Info{ID: "exemplo", Nome: "Exemplo", Descricao: "Um módulo de teste."}, rodou: make(chan struct{}, 1)}
}

func novoCatalogo(t *testing.T, mods ...Modulo) (*Catalogo, *http.ServeMux) {
	t.Helper()
	db := testeapoio.NovoBanco(t)
	c := NovoCatalogo(db, testeapoio.RelogioDosTestes(), slog.New(slog.DiscardHandler))
	for _, m := range mods {
		c.Adicionar(m)
	}
	mux := http.NewServeMux()
	c.Montar(mux)
	return c, mux
}

func TestAdicionarRecusaModuloMalFeito(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome   string
		info   Info
		panico string
	}{
		{"id com maiúscula", Info{ID: "Trabalho", Nome: "T", Descricao: "d"}, "id inválido"},
		{"id vazio", Info{ID: "", Nome: "T", Descricao: "d"}, "id inválido"},
		{"id com barra", Info{ID: "a/b", Nome: "T", Descricao: "d"}, "id inválido"},
		{"sem nome", Info{ID: "a", Descricao: "d"}, "sem nome"},
		{"sem descrição", Info{ID: "a", Nome: "A"}, "sem nome ou sem descrição"},
		{"internet ligado por padrão", Info{ID: "a", Nome: "A", Descricao: "d", Destinos: []string{"x.com"}, LigadoPorPadrao: true},
			"nascer desligado"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			cat := NovoCatalogo(nil, testeapoio.RelogioDosTestes(), slog.New(slog.DiscardHandler))
			conferirPanico(t, c.panico, func() { cat.Adicionar(&moduloDeTeste{info: c.info}) })
		})
	}
	t.Run("repetido", func(t *testing.T) {
		t.Parallel()
		cat := NovoCatalogo(nil, testeapoio.RelogioDosTestes(), slog.New(slog.DiscardHandler))
		cat.Adicionar(exemplo())
		conferirPanico(t, "duas vezes", func() { cat.Adicionar(exemplo()) })
	})
}

func TestRotaForaDoPrefixoDoModulo(t *testing.T) {
	t.Parallel()
	casos := []struct {
		padrao string
		aceita bool
	}{
		{"GET /api/exemplo/ola", true},
		{"/api/exemplo/ola", true},
		{"GET /api/exemplo", true},
		{"POST /api/exemplo/{id}/x", true},
		{"GET /api/exemplos", false}, // outro prefixo que começa igual
		{"GET /api/transacoes", false},
		{"GET /exemplo", false},
	}
	for _, c := range casos {
		t.Run(c.padrao, func(t *testing.T) {
			t.Parallel()
			m := exemplo()
			m.rotas = []string{c.padrao}
			db := testeapoio.NovoBanco(t)
			cat := NovoCatalogo(db, testeapoio.RelogioDosTestes(), slog.New(slog.DiscardHandler))
			cat.Adicionar(m)
			montar := func() { cat.Montar(http.NewServeMux()) }
			if c.aceita {
				montar()
				return
			}
			conferirPanico(t, "fora de /api/exemplo/", montar)
		})
	}
}

func TestLigadoPeloPadraoEPelaEscolha(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome    string
		padrao  bool
		gravado string // "" = sem escolha
		ligado  bool
	}{
		{"sem escolha, nasce desligado", false, "", false},
		{"sem escolha, nasce ligado", true, "", true},
		{"escolheu ligar", false, "true", true},
		{"escolheu desligar", true, "false", false},
		{"valor estranho vale o padrão", true, "talvez", true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			m := exemplo()
			m.info.LigadoPorPadrao = c.padrao
			cat, _ := novoCatalogo(t, m)
			if c.gravado != "" {
				testeapoio.NovaFabrica(t, cat.db, cat.relogio).Configuracao(Chave("exemplo"), c.gravado)
			}
			ligado, err := cat.Ligado(t.Context(), "exemplo")
			if err != nil || ligado != c.ligado {
				t.Fatalf("Ligado = %v, %v; quer %v", ligado, err, c.ligado)
			}
		})
	}
	t.Run("desconhecido", func(t *testing.T) {
		t.Parallel()
		cat, _ := novoCatalogo(t)
		if _, err := cat.Ligado(t.Context(), "nenhum"); !errors.Is(err, ErrDesconhecido) {
			t.Fatalf("erro = %v; quer ErrDesconhecido", err)
		}
	})
}

func TestRotasDoCatalogo(t *testing.T) {
	t.Parallel()
	noticias := &moduloDeTeste{info: Info{ID: "noticias", Nome: "Notícias", Descricao: "Notícias do dia.",
		Destinos: []string{"g1.globo.com", "www.tabnews.com.br"}}}
	trabalho := &moduloDeTeste{info: Info{ID: "trabalho", Nome: "Trabalho", Descricao: "Salário e holerite."}}
	cat, mux := novoCatalogo(t, trabalho, noticias)

	testeapoio.Chamar(t, mux, "GET", "/api/modulos", "").ConferirJSON(t, `[
		{"id":"trabalho","nome":"Trabalho","descricao":"Salário e holerite.","acessaInternet":false,"destinos":[],"ligado":false},
		{"id":"noticias","nome":"Notícias","descricao":"Notícias do dia.","acessaInternet":true,
		 "destinos":["g1.globo.com","www.tabnews.com.br"],"ligado":false}]`)

	// Desligado: a rota do módulo responde 409, sem rodar o handler.
	r := testeapoio.Chamar(t, mux, "GET", "/api/trabalho/ola", "")
	r.ConferirStatus(t, http.StatusConflict)
	r.ConferirJSON(t, `{"mensagem":"O módulo Trabalho está desligado. Ligue em Configurações › Módulos."}`)

	// Ligar: responde a situação nova, a rota passa a responder e a auditoria registra.
	testeapoio.Chamar(t, mux, "PUT", "/api/modulos/trabalho", `{"ligado":true}`).ConferirJSON(t,
		`{"id":"trabalho","nome":"Trabalho","descricao":"Salário e holerite.","acessaInternet":false,"destinos":[],"ligado":true}`)
	testeapoio.Chamar(t, mux, "GET", "/api/trabalho/ola", "").ConferirJSON(t, `"ola"`)
	// O outro módulo continua desligado.
	testeapoio.Chamar(t, mux, "GET", "/api/noticias/ola", "").ConferirStatus(t, http.StatusConflict)

	testeapoio.Chamar(t, mux, "PUT", "/api/modulos/trabalho", `{"ligado":false}`).ConferirStatus(t, http.StatusOK)
	testeapoio.Chamar(t, mux, "GET", "/api/trabalho/ola", "").ConferirStatus(t, http.StatusConflict)

	var eventos []string
	linhas, err := cat.db.QueryContext(t.Context(), `SELECT Evento FROM AuditoriaAcesso ORDER BY Id`)
	if err != nil {
		t.Fatal(err)
	}
	defer linhas.Close()
	for linhas.Next() {
		var e string
		if err := linhas.Scan(&e); err != nil {
			t.Fatal(err)
		}
		eventos = append(eventos, e)
	}
	if got := strings.Join(eventos, ", "); got != "ModuloLigado (trabalho), ModuloDesligado (trabalho)" {
		t.Fatalf("auditoria = %q", got)
	}
}

func TestDefinirComCorpoErrado(t *testing.T) {
	t.Parallel()
	_, mux := novoCatalogo(t, exemplo())
	casos := []struct {
		nome, caminho, corpo string
		status               int
	}{
		{"módulo que não existe", "/api/modulos/nenhum", `{"ligado":true}`, http.StatusNotFound},
		{"sem o ligado", "/api/modulos/exemplo", `{}`, http.StatusBadRequest},
		{"ligado nulo", "/api/modulos/exemplo", `{"ligado":null}`, http.StatusBadRequest},
		{"ligado em texto", "/api/modulos/exemplo", `{"ligado":"true"}`, http.StatusBadRequest},
		{"sem corpo nem tipo", "/api/modulos/exemplo", ``, http.StatusUnsupportedMediaType},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			testeapoio.Chamar(t, mux, "PUT", c.caminho, c.corpo).ConferirStatus(t, c.status)
		})
	}
}

func TestTarefaSoRodaComOModuloLigado(t *testing.T) {
	t.Parallel()
	m := exemplo()
	cat, mux := novoCatalogo(t, m)
	rodou := make(chan struct{}, 1)
	ciclo := cat.cicloSeLigado("exemplo", func(context.Context) error { rodou <- struct{}{}; return nil })

	// Desligado: o ciclo volta sem rodar a tarefa.
	if err := ciclo(t.Context()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-rodou:
		t.Fatal("a tarefa rodou com o módulo desligado")
	default:
	}
	// Ligado: roda.
	testeapoio.Chamar(t, mux, "PUT", "/api/modulos/exemplo", `{"ligado":true}`).ConferirStatus(t, http.StatusOK)
	if err := ciclo(t.Context()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-rodou:
	default:
		t.Fatal("a tarefa não rodou com o módulo ligado")
	}
}

func TestIniciarTarefasRodaPeloPeriodica(t *testing.T) {
	t.Parallel()
	m := exemplo()
	m.info.LigadoPorPadrao = true
	cat, _ := novoCatalogo(t, m)
	ctx, parar := context.WithCancel(t.Context())
	var emAndamento sync.WaitGroup
	cat.IniciarTarefas(ctx, &emAndamento)
	select {
	case <-m.rodou:
	case <-time.After(5 * time.Second):
		t.Fatal("a tarefa do módulo ligado não rodou")
	}
	parar()
	emAndamento.Wait() // para quando o ctx acaba
}

func TestCicloSeLigadoDevolveOErroDaLeitura(t *testing.T) {
	t.Parallel()
	cat := NovoCatalogo(nil, testeapoio.RelogioDosTestes(), slog.New(slog.DiscardHandler))
	rodou := false
	err := cat.cicloSeLigado("nenhum", func(context.Context) error { rodou = true; return nil })(t.Context())
	if !errors.Is(err, ErrDesconhecido) || rodou {
		t.Fatalf("erro = %v, rodou = %v; quer ErrDesconhecido sem rodar", err, rodou)
	}
}

func conferirPanico(t *testing.T, trecho string, f func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("não deu pânico; queria %q", trecho)
		}
		if msg, _ := r.(string); !strings.Contains(msg, trecho) {
			t.Fatalf("pânico %q; queria %q", r, trecho)
		}
	}()
	f()
}
