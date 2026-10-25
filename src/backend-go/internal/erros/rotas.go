package erros

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/auditoria"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/configuracao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/servidor"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

const mensagemNaoEncontrado = "Registro não encontrado."

// Servico é o ErrosController.
type Servico struct {
	db      *sql.DB
	q       *consultas.Queries
	fila    *Fila
	relogio relogio.Relogio
	log     *slog.Logger
}

func NovoServico(db *sql.DB, fila *Fila, r relogio.Relogio, log *slog.Logger) *Servico {
	return &Servico{db: db, q: consultas.New(db), fila: fila, relogio: r, log: log}
}

// Registrar põe as 5 rotas de api/erros no mux. Login e CSRF são de quem está na frente (a autenticação do Go
// sozinho): aqui só chega quem passou.
func Registrar(mux *http.ServeMux, s *Servico) {
	mux.HandleFunc("GET /api/erros", s.listar)
	mux.HandleFunc("GET /api/erros/{id}", s.detalhe)
	mux.HandleFunc("PUT /api/erros/{id}/situacao", s.situacao)
	mux.HandleFunc("POST /api/erros/resolver", s.resolverTodos)
	mux.HandleFunc("POST /api/erros/front-end", s.registrarDoFrontEnd)
}

// ErroSistemaDto, na ordem do record do C#. Datas em UTC, com "Z" (o Utc do controller).
type ErroSistemaDto struct {
	Id          int64                  `json:"id"`
	PrimeiraEm  configuracao.Instante  `json:"primeiraEm"`
	UltimaEm    configuracao.Instante  `json:"ultimaEm"`
	Ocorrencias int                    `json:"ocorrencias"`
	Nivel       Nivel                  `json:"nivel"`
	Origem      string                 `json:"origem"`
	Mensagem    string                 `json:"mensagem"`
	Rota        *string                `json:"rota"`
	Usuario     *string                `json:"usuario"`
	Resolvido   bool                   `json:"resolvido"`
	ResolvidoEm *configuracao.Instante `json:"resolvidoEm"`
}

// PaginaErrosDto: Abertos é o total em aberto (sem filtro), para o contador da aba.
type PaginaErrosDto struct {
	Itens         []ErroSistemaDto `json:"itens"`
	Total         int              `json:"total"`
	Pagina        int              `json:"pagina"`
	TamanhoPagina int              `json:"tamanhoPagina"`
	Abertos       int              `json:"abertos"`
	Origens       []string         `json:"origens"`
}

type ErroSistemaDetalheDto struct {
	Id        int64   `json:"id"`
	Categoria string  `json:"categoria"`
	Detalhe   *string `json:"detalhe"`
}

func (s *Servico) listar(w http.ResponseWriter, r *http.Request) {
	f, erros := lerFiltro(r.URL.RawQuery)
	if erros != nil {
		respostas.Validacao(w, erros)
		return
	}
	pagina, err := s.Listar(r.Context(), f)
	if err != nil {
		s.erroInterno(w, r, "listar", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, pagina)
}

// Listar é o Listar do controller: a página do filtro (mais recente primeiro, empate pelo Id), o total do filtro, os
// em aberto da tabela toda e as origens que existem.
func (s *Servico) Listar(ctx context.Context, f filtro) (PaginaErrosDto, error) {
	onde, args := f.condicoes()
	p := PaginaErrosDto{Itens: []ErroSistemaDto{}, Origens: []string{}, Pagina: f.pagina, TamanhoPagina: f.tamanhoPagina}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM "ErroSistema" WHERE `+onde, args...).Scan(&p.Total); err != nil {
		return p, err
	}
	// O Skip((Pagina - 1) * TamanhoPagina) do C# é conta de int: passa do limite e dá a volta, como aqui.
	pular := int32(int64(f.pagina-1) * int64(f.tamanhoPagina))
	linhas, err := s.db.QueryContext(ctx, `SELECT "Id", "PrimeiraEm", "UltimaEm", "Ocorrencias", "Nivel", "Origem", "Mensagem",
		"Rota", "Usuario", "Resolvido", "ResolvidoEm"
		FROM "ErroSistema" WHERE `+onde+`
		ORDER BY "UltimaEm" DESC, "Id" DESC
		LIMIT ? OFFSET ?`, append(args, f.tamanhoPagina, pular)...)
	if err != nil {
		return p, err
	}
	defer linhas.Close()
	for linhas.Next() {
		var e ErroSistemaDto
		var primeira, ultima tipos.DataHora
		var resolvidoEm *tipos.DataHora
		if err := linhas.Scan(&e.Id, &primeira, &ultima, &e.Ocorrencias, &e.Nivel, &e.Origem, &e.Mensagem, &e.Rota, &e.Usuario,
			&e.Resolvido, &resolvidoEm); err != nil {
			return p, err
		}
		e.PrimeiraEm, e.UltimaEm = configuracao.InstanteUTC(primeira.Tempo()), configuracao.InstanteUTC(ultima.Tempo())
		if resolvidoEm != nil {
			i := configuracao.InstanteUTC(resolvidoEm.Tempo())
			e.ResolvidoEm = &i
		}
		p.Itens = append(p.Itens, e)
	}
	if err := linhas.Err(); err != nil {
		return p, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM "ErroSistema" WHERE "Resolvido" = 0`).Scan(&p.Abertos); err != nil {
		return p, err
	}
	origens, err := s.db.QueryContext(ctx, `SELECT DISTINCT "Origem" FROM "ErroSistema" ORDER BY "Origem"`)
	if err != nil {
		return p, err
	}
	defer origens.Close()
	for origens.Next() {
		var o string
		if err := origens.Scan(&o); err != nil {
			return p, err
		}
		p.Origens = append(p.Origens, o)
	}
	return p, origens.Err()
}

// idDaRota é a restrição {id:long} da rota: o que não é um long não casa com a rota, e cai no 404 sem corpo do
// MapFallback.
func idDaRota(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimPrefix(r.PathValue("id"), "+"), 10, 64)
	if err != nil {
		respostas.SemCorpo(w, http.StatusNotFound)
		return 0, false
	}
	return id, true
}

// detalhe devolve a categoria e a pilha: só quando a pessoa abre o detalhe (podem ter alguns KB cada).
func (s *Servico) detalhe(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	d := ErroSistemaDetalheDto{Id: id}
	err := s.db.QueryRowContext(r.Context(), `SELECT "Categoria", "Detalhe" FROM "ErroSistema" WHERE "Id" = ?`, id).Scan(&d.Categoria, &d.Detalhe)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		respostas.Mensagem(w, http.StatusNotFound, mensagemNaoEncontrado)
	case err != nil:
		s.erroInterno(w, r, "detalhe", err)
	default:
		respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, d)
	}
}

// situacao marca um erro como resolvido (com a hora) ou o reabre.
func (s *Servico) situacao(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	resolvido := &respostas.CampoBool{Propriedade: "Resolvido"}
	if !respostas.LerObjeto(w, r, "dto", "Financas.Api.Dtos.SituacaoErroDto", resolvido) {
		return
	}
	var resolvidoEm *tipos.DataHora
	if resolvido.Valor {
		agora := tipos.NovaDataHora(s.relogio.Agora())
		resolvidoEm = &agora
	}
	resultado, err := s.db.ExecContext(r.Context(), `UPDATE "ErroSistema" SET "Resolvido" = ?, "ResolvidoEm" = ? WHERE "Id" = ?`,
		resolvido.Valor, resolvidoEm, id)
	var alterados int64
	if err == nil {
		alterados, err = resultado.RowsAffected()
	}
	switch {
	case err != nil:
		s.erroInterno(w, r, "situação", err)
	case alterados == 0:
		respostas.Mensagem(w, http.StatusNotFound, mensagemNaoEncontrado)
	default:
		respostas.SemCorpo(w, http.StatusNoContent)
	}
}

// resolverTodos marca como resolvidos todos os em aberto que passam no filtro (todas as páginas).
func (s *Servico) resolverTodos(w http.ResponseWriter, r *http.Request) {
	f, erros := lerFiltro(r.URL.RawQuery)
	if erros != nil {
		respostas.Validacao(w, erros)
		return
	}
	onde, args := f.condicoes()
	args = append([]any{tipos.NovaDataHora(s.relogio.Agora())}, args...)
	resultado, err := s.db.ExecContext(r.Context(), `UPDATE "ErroSistema" SET "Resolvido" = 1, "ResolvidoEm" = ?
		WHERE `+onde+` AND "Resolvido" = 0`, args...)
	var quantidade int64
	if err == nil {
		quantidade, err = resultado.RowsAffected()
	}
	if err != nil {
		s.erroInterno(w, r, "resolver", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, map[string]int64{"quantidade": quantidade})
}

// registrarDoFrontEnd é o erro não tratado no navegador (ErrorHandler do Angular). Sempre 204: descartar (fila
// cheia, mais de 30 na hora) não é erro.
func (s *Servico) registrarDoFrontEnd(w http.ResponseWriter, r *http.Request) {
	mensagem := &respostas.CampoTexto{Propriedade: "Mensagem", Maximo: 1000}
	detalhe := &respostas.CampoTexto{Propriedade: "Detalhe", Maximo: 8000, Opcional: true}
	rota := &respostas.CampoTexto{Propriedade: "Rota", Maximo: 300, Opcional: true}
	if !respostas.LerTextos(w, r, "dto", "Financas.Api.Dtos.ErroFrontEndDto", mensagem, detalhe, rota) {
		return
	}
	login, err := auditoria.Login(r.Context(), s.q)
	if err != nil {
		s.erroInterno(w, r, "front-end", err)
		return
	}
	registro := Registro{Em: s.relogio.Agora(), Nivel: Erro, Origem: OrigemFrontEnd, Categoria: OrigemFrontEnd,
		Mensagem: *mensagem.Valor, Detalhe: detalhe.Valor, Usuario: login}
	if rota.Valor != nil {
		tela := "tela " + *rota.Valor
		registro.Rota = &tela
	}
	s.fila.RegistrarDoFrontEnd(registro)
	respostas.SemCorpo(w, http.StatusNoContent)
}

// erroInterno é o UseExceptionHandler: o erro vai para o log (e para a tabela, com a rota), e quem pediu recebe só o
// 500 de sempre.
func (s *Servico) erroInterno(w http.ResponseWriter, r *http.Request, etapa string, err error) {
	s.log.ErrorContext(r.Context(), "erro nos erros do sistema", "origem", OrigemAPI, "etapa", etapa, "erro", err)
	servidor.ResponderErroInterno(w)
}
