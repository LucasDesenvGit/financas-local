// Pacote modulos é o núcleo dos módulos (#190, #191, ADR 0064): visualizações que vão além do básico de finanças e
// que cada pessoa liga ou desliga em Configurações › Módulos.
//
// Um módulo é código compilado junto com o site (micro-kernel: Richards e Ford, Fundamentals of Software
// Architecture). Ele implementa o contrato Modulo, mora em internal/modulos/<id> e é montado no internal/api. O núcleo
// guarda as rotas dele (desligado, 409 sem rodar nada) e roda as tarefas dele só com ele ligado. A escolha fica na
// tabela Configuracao (chave Modulos.<id>.Ligado), como os parâmetros (ADR 0015): vale para qualquer navegador e para
// as tarefas de fundo.
package modulos

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/auditoria"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/configuracao"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// Modulo é o contrato que cada módulo implementa.
type Modulo interface {
	// Info diz o que o módulo é: o catálogo mostra isso em Configurações › Módulos.
	Info() Info
	// Registrar põe as rotas do módulo, todas em /api/<id>/, pelo Rotas (que as guarda).
	Registrar(rotas *Rotas)
	// Tarefas são as tarefas em segundo plano do módulo (pode ser nenhuma). O núcleo as roda só com o módulo ligado.
	Tarefas() []Tarefa
}

// Info é o que o catálogo sabe de um módulo.
type Info struct {
	ID        string // só letras minúsculas: vira o prefixo /api/<id>/ e a chave Modulos.<id>.Ligado
	Nome      string // "Trabalho"
	Descricao string // uma frase: o que ele mostra
	// Destinos são os domínios que o módulo acessa na internet ("g1.globo.com"). Vazio: não acessa a internet.
	Destinos []string
	// LigadoPorPadrao vale até a pessoa escolher. Módulo que acessa a internet nasce desligado (a montagem falha se não).
	LigadoPorPadrao bool
}

// AcessaInternet diz se o módulo tem algum destino na internet.
func (i Info) AcessaInternet() bool { return len(i.Destinos) > 0 }

// Tarefa é uma tarefa em segundo plano de um módulo, rodada pelo tarefas.Periodica.
type Tarefa struct {
	Nome      string        // como aparece no log ("as notícias")
	Atraso    time.Duration // espera antes do primeiro ciclo
	Intervalo time.Duration // entre um ciclo e o próximo
	Ciclo     func(context.Context) error
}

// Situacao é um módulo como a API devolve: o que ele é e se está ligado.
type Situacao struct {
	ID             string   `json:"id"`
	Nome           string   `json:"nome"`
	Descricao      string   `json:"descricao"`
	AcessaInternet bool     `json:"acessaInternet"`
	Destinos       []string `json:"destinos"`
	Ligado         bool     `json:"ligado"`
}

// Eventos da auditoria ("Acessos recentes"), com o id entre parênteses.
const (
	EventoLigado    = "ModuloLigado"
	EventoDesligado = "ModuloDesligado"
)

// MensagemDesligado é o corpo do 409 de uma rota de módulo desligado.
func MensagemDesligado(nome string) string {
	return "O módulo " + nome + " está desligado. Ligue em Configurações › Módulos."
}

// ErrDesconhecido é o id que não está no catálogo.
var ErrDesconhecido = fmt.Errorf("módulo desconhecido")

var idValido = regexp.MustCompile(`^[a-z]+$`)

// Catalogo é a lista dos módulos compilados no site, na ordem em que entraram.
type Catalogo struct {
	db      *sql.DB
	conf    *configuracao.Servico
	q       *consultas.Queries
	relogio relogio.Relogio
	log     *slog.Logger
	modulos []Modulo
	porID   map[string]Modulo
}

// NovoCatalogo devolve o catálogo vazio: cada módulo entra com Adicionar, no internal/api.
func NovoCatalogo(db *sql.DB, r relogio.Relogio, log *slog.Logger) *Catalogo {
	return &Catalogo{db: db, conf: configuracao.Novo(db, r), q: consultas.New(db), relogio: r, log: log, porID: map[string]Modulo{}}
}

// Adicionar põe o módulo no catálogo. Erro de quem escreveu o módulo (id inválido ou repetido, sem nome, acessa a
// internet e nasce ligado) é pânico: aparece na primeira vez que o site sobe, e nos testes.
func (c *Catalogo) Adicionar(m Modulo) {
	i := m.Info()
	switch {
	case !idValido.MatchString(i.ID):
		panic(fmt.Sprintf("módulo com id inválido %q: só letras minúsculas", i.ID))
	case c.porID[i.ID] != nil:
		panic(fmt.Sprintf("módulo %q adicionado duas vezes", i.ID))
	case i.Nome == "" || i.Descricao == "":
		panic(fmt.Sprintf("módulo %q sem nome ou sem descrição", i.ID))
	case i.AcessaInternet() && i.LigadoPorPadrao:
		panic(fmt.Sprintf("módulo %q acessa a internet e nasce ligado: tem de nascer desligado (ADR 0064)", i.ID))
	}
	c.modulos = append(c.modulos, m)
	c.porID[i.ID] = m
}

// Chave é a chave da Configuracao com a escolha do módulo.
func Chave(id string) string { return "Modulos." + id + ".Ligado" }

// Ligado diz se o módulo está ligado: a escolha gravada ou, sem escolha, o padrão do módulo.
func (c *Catalogo) Ligado(ctx context.Context, id string) (bool, error) {
	m := c.porID[id]
	if m == nil {
		return false, fmt.Errorf("%w: %s", ErrDesconhecido, id)
	}
	return c.conf.ObterBool(ctx, Chave(id), m.Info().LigadoPorPadrao)
}

// Listar devolve todos os módulos do catálogo, com a situação de cada um.
func (c *Catalogo) Listar(ctx context.Context) ([]Situacao, error) {
	lista := make([]Situacao, 0, len(c.modulos))
	for _, m := range c.modulos {
		i := m.Info()
		ligado, err := c.Ligado(ctx, i.ID)
		if err != nil {
			return nil, err
		}
		destinos := i.Destinos
		if destinos == nil {
			destinos = []string{}
		}
		lista = append(lista, Situacao{ID: i.ID, Nome: i.Nome, Descricao: i.Descricao, AcessaInternet: i.AcessaInternet(),
			Destinos: destinos, Ligado: ligado})
	}
	return lista, nil
}

// Definir liga ou desliga o módulo e registra na auditoria, numa transação. r é o pedido de quem mudou (IP, navegador).
func (c *Catalogo) Definir(ctx context.Context, r *http.Request, id string, ligado bool) (Situacao, error) {
	if c.porID[id] == nil {
		return Situacao{}, fmt.Errorf("%w: %s", ErrDesconhecido, id)
	}
	evento := EventoDesligado
	if ligado {
		evento = EventoLigado
	}
	valor := strconv.FormatBool(ligado)
	err := banco.EmTransacao(ctx, c.db, func(tx *sql.Tx) error {
		q := c.q.WithTx(tx)
		login, err := auditoria.Login(ctx, q)
		if err != nil {
			return err
		}
		if err := auditoria.Registrar(ctx, q, r, tipos.NovaDataHora(c.relogio.Agora()), evento+" ("+id+")", login); err != nil {
			return err
		}
		return c.conf.ComTx(tx).Definir(ctx, Chave(id), &valor)
	})
	if err != nil {
		return Situacao{}, fmt.Errorf("gravar o módulo %s: %w", id, err)
	}
	lista, err := c.Listar(ctx)
	if err != nil {
		return Situacao{}, err
	}
	for _, s := range lista {
		if s.ID == id {
			return s, nil
		}
	}
	return Situacao{}, fmt.Errorf("%w: %s", ErrDesconhecido, id)
}
