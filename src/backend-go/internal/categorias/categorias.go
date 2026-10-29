// Pacote categorias tem as rotas de três controllers do .NET (#77, épico #70): CategoriasController (api/categorias),
// RegrasController (api/regras, com o "reaplicar") e RegrasDescricaoController (api/regras-descricao, ADR 0030). A
// regra de negócio fica nos pacotes que a sincronização (#83) também usa: categorizacao, regrasdescricao e
// transferencias. Aqui só o HTTP: ler a entrada como o model binding do ASP.NET, chamar a regra e responder o mesmo JSON.
//
// Login e CSRF são de quem está na frente (o .NET, ou a autenticação do Go sozinho): aqui só chega quem passou.
package categorias

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/categorizacao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/servidor"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
)

const mensagemNaoEncontrado = "Registro não encontrado."

// semLimite é o tamanho de um texto sem [StringLength].
const semLimite = math.MaxInt32

// Servico atende os três controllers.
type Servico struct {
	db      *sql.DB
	relogio relogio.Relogio
	log     *slog.Logger
}

func NovoServico(db *sql.DB, r relogio.Relogio, log *slog.Logger) *Servico {
	return &Servico{db: db, relogio: r, log: log}
}

// Registrar põe as 14 rotas no mux.
func Registrar(mux *http.ServeMux, s *Servico) {
	mux.HandleFunc("GET /api/categorias", s.listarCategorias)
	mux.HandleFunc("POST /api/categorias", s.criarCategoria)
	mux.HandleFunc("PUT /api/categorias/{id}", s.atualizarCategoria)
	mux.HandleFunc("DELETE /api/categorias/{id}", s.excluirCategoria)

	mux.HandleFunc("GET /api/regras", s.listarRegras)
	mux.HandleFunc("POST /api/regras", s.criarRegra)
	mux.HandleFunc("PUT /api/regras/{id}", s.atualizarRegra)
	mux.HandleFunc("DELETE /api/regras/{id}", s.excluirRegra)
	mux.HandleFunc("POST /api/regras/reaplicar", s.reaplicar)

	mux.HandleFunc("GET /api/regras-descricao", s.listarRegrasDescricao)
	mux.HandleFunc("GET /api/regras-descricao/previa", s.previa)
	mux.HandleFunc("POST /api/regras-descricao", s.criarRegraDescricao)
	mux.HandleFunc("PUT /api/regras-descricao/{id}", s.atualizarRegraDescricao)
	mux.HandleFunc("DELETE /api/regras-descricao/{id}", s.excluirRegraDescricao)
}

func (s *Servico) agora() tipos.DataHora { return tipos.NovaDataHora(s.relogio.Agora()) }

// idDaRota é o {id:int} do ASP.NET: o que não é um int de 32 bits não casa com a rota e cai no 404 sem corpo do
// fallback de /api.
func idDaRota(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimPrefix(r.PathValue("id"), "+"), 10, 32)
	if err != nil {
		respostas.SemCorpo(w, http.StatusNotFound)
		return 0, false
	}
	return id, true
}

// erroInterno registra o erro e responde o 500 de sempre. Pedido cancelado (o navegador desistiu) não é erro.
func (s *Servico) erroInterno(w http.ResponseWriter, r *http.Request, etapa string, err error) {
	if errors.Is(err, context.Canceled) || r.Context().Err() != nil {
		return
	}
	s.log.ErrorContext(r.Context(), "erro nas categorias e regras", "origem", "API", "etapa", etapa, "erro", err)
	servidor.ResponderErroInterno(w)
}

/* ----------------------------------------------------------------------------------------------- Categorias */

// CategoriaDto, na ordem do record do C#.
type CategoriaDto struct {
	Id             int64   `json:"id"`
	Nome           string  `json:"nome"`
	Tipo           int64   `json:"tipo"`
	Cor            string  `json:"cor"`
	Icone          *string `json:"icone"`
	CategoriaPaiId *int64  `json:"categoriaPaiId"`
	Ativa          bool    `json:"ativa"`
	Natureza       *int64  `json:"natureza"`
}

// listarCategorias: por tipo e nome, no ORDER BY do SQLite (byte a byte).
func (s *Servico) listarCategorias(w http.ResponseWriter, r *http.Request) {
	linhas, err := s.db.QueryContext(r.Context(), `SELECT "Id", "Nome", "Tipo", "Cor", "Icone", "CategoriaPaiId", "Ativa",
		"Natureza" FROM "Categoria" ORDER BY "Tipo", "Nome"`)
	if err != nil {
		s.erroInterno(w, r, "listar categorias", err)
		return
	}
	defer linhas.Close()
	lista := []CategoriaDto{}
	for linhas.Next() {
		var c CategoriaDto
		if err := linhas.Scan(&c.Id, &c.Nome, &c.Tipo, &c.Cor, &c.Icone, &c.CategoriaPaiId, &c.Ativa, &c.Natureza); err != nil {
			s.erroInterno(w, r, "ler categoria", err)
			return
		}
		lista = append(lista, c)
	}
	if err := linhas.Err(); err != nil {
		s.erroInterno(w, r, "listar categorias", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, lista)
}

// salvarCategoria é o SalvarCategoriaDto, já com os padrões do construtor do record (Ativa = true).
type salvarCategoria struct {
	nome           respostas.CampoTexto
	tipo           respostas.CampoEnum
	cor            respostas.CampoTexto
	icone          respostas.CampoTexto
	categoriaPaiId respostas.CampoInteiroOpcional
	ativa          respostas.CampoBool
	natureza       respostas.CampoEnum
}

var padraoCor = respostas.NovoPadrao(`^#[0-9a-fA-F]{6}$`)

// lerCategoria é o model binding e a validação dos atributos: [Required, StringLength(80)] Nome, [Required,
// RegularExpression] Cor, [StringLength(40)] Icone. Devolve false se já respondeu o 400.
func lerCategoria(w http.ResponseWriter, r *http.Request) (*salvarCategoria, bool) {
	d := &salvarCategoria{
		nome:           respostas.CampoTexto{Propriedade: "Nome", Maximo: 80},
		tipo:           respostas.CampoEnum{Propriedade: "Tipo"},
		cor:            respostas.CampoTexto{Propriedade: "Cor", Maximo: semLimite, Padrao: padraoCor},
		icone:          respostas.CampoTexto{Propriedade: "Icone", Maximo: 40, Opcional: true},
		categoriaPaiId: respostas.CampoInteiroOpcional{Propriedade: "CategoriaPaiId"},
		ativa:          respostas.CampoBool{Propriedade: "Ativa", Valor: true},
		natureza:       respostas.CampoEnum{Propriedade: "Natureza", Anulavel: true},
	}
	ok := respostas.LerObjeto(w, r, "dto", "Financas.Api.Dtos.SalvarCategoriaDto",
		&d.nome, &d.tipo, &d.cor, &d.icone, &d.categoriaPaiId, &d.ativa, &d.natureza)
	return d, ok
}

// categoria é o que o Aplicar do controller grava: nome e ícone aparados (ícone só de espaços vira null) e a natureza
// só nas categorias de despesa.
func (d *salvarCategoria) categoria(id int64) CategoriaDto {
	c := CategoriaDto{Id: id, Nome: strings.TrimSpace(*d.nome.Valor), Tipo: d.tipo.Ou(0), Cor: *d.cor.Valor,
		CategoriaPaiId: d.categoriaPaiId.Valor, Ativa: d.ativa.Valor}
	if d.icone.Valor != nil && strings.TrimSpace(*d.icone.Valor) != "" {
		icone := strings.TrimSpace(*d.icone.Valor)
		c.Icone = &icone
	}
	if c.Tipo == categorizacao.CategoriaDespesa {
		c.Natureza = d.natureza.Valor
	}
	return c
}

// validarCategoria é o ValidarAsync: as regras que o front-end mostra como mensagem (nome repetido, pai).
func validarCategoria(ctx context.Context, q *sql.Tx, id *int64, c CategoriaDto) (string, error) {
	var existe bool
	consulta, argumentos := `SELECT EXISTS (SELECT 1 FROM "Categoria" WHERE "Nome" = ?)`, []any{c.Nome}
	if id != nil {
		consulta, argumentos = `SELECT EXISTS (SELECT 1 FROM "Categoria" WHERE "Nome" = ? AND "Id" <> ?)`, []any{c.Nome, *id}
	}
	if err := q.QueryRowContext(ctx, consulta, argumentos...).Scan(&existe); err != nil {
		return "", err
	}
	if existe {
		return "Já existe uma categoria com esse nome.", nil
	}
	if c.CategoriaPaiId == nil {
		return "", nil
	}
	paiId := *c.CategoriaPaiId
	if id != nil && paiId == *id {
		return "Uma categoria não pode ser pai dela mesma.", nil
	}
	var avo *int64
	var tipoPai int64
	err := q.QueryRowContext(ctx, `SELECT "CategoriaPaiId", "Tipo" FROM "Categoria" WHERE "Id" = ?`, paiId).Scan(&avo, &tipoPai)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return "Categoria pai inválida.", nil
	case err != nil:
		return "", err
	case avo != nil:
		return "Só é permitido um nível de subcategoria.", nil
	case tipoPai != c.Tipo:
		return "A subcategoria deve ter o mesmo tipo da categoria pai.", nil
	}
	if id != nil {
		var temFilhas bool
		if err := q.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM "Categoria" WHERE "CategoriaPaiId" = ?)`, *id).Scan(&temFilhas); err != nil {
			return "", err
		}
		if temFilhas {
			return "Esta categoria possui subcategorias e não pode virar subcategoria.", nil
		}
	}
	return "", nil
}

func (s *Servico) criarCategoria(w http.ResponseWriter, r *http.Request) {
	d, ok := lerCategoria(w, r)
	if !ok {
		return
	}
	c := d.categoria(0)
	var problema string
	err := banco.EmTransacao(r.Context(), s.db, func(tx *sql.Tx) error {
		var err error
		if problema, err = validarCategoria(r.Context(), tx, nil, c); err != nil || problema != "" {
			return err
		}
		res, err := tx.ExecContext(r.Context(), `INSERT INTO "Categoria" ("Nome", "Tipo", "Cor", "Icone", "CategoriaPaiId",
			"Ativa", "Natureza") VALUES (?, ?, ?, ?, ?, ?, ?)`, c.Nome, c.Tipo, c.Cor, c.Icone, c.CategoriaPaiId, c.Ativa, c.Natureza)
		if err != nil {
			return err
		}
		c.Id, err = res.LastInsertId()
		return err
	})
	switch {
	case err != nil:
		s.erroInterno(w, r, "criar categoria", err)
	case problema != "":
		respostas.Mensagem(w, http.StatusBadRequest, problema)
	default:
		respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, c)
	}
}

func (s *Servico) atualizarCategoria(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	d, ok := lerCategoria(w, r)
	if !ok {
		return
	}
	c := d.categoria(id)
	var naoExiste bool
	var problema string
	err := banco.EmTransacao(r.Context(), s.db, func(tx *sql.Tx) error {
		var tipoAntes int64
		err := tx.QueryRowContext(r.Context(), `SELECT "Tipo" FROM "Categoria" WHERE "Id" = ?`, id).Scan(&tipoAntes)
		if errors.Is(err, sql.ErrNoRows) {
			naoExiste = true
			return nil
		}
		if err != nil {
			return err
		}
		if problema, err = validarCategoria(r.Context(), tx, &id, c); err != nil || problema != "" {
			return err
		}
		if _, err := tx.ExecContext(r.Context(), `UPDATE "Categoria" SET "Nome" = ?, "Tipo" = ?, "Cor" = ?, "Icone" = ?,
			"CategoriaPaiId" = ?, "Ativa" = ?, "Natureza" = ? WHERE "Id" = ?`,
			c.Nome, c.Tipo, c.Cor, c.Icone, c.CategoriaPaiId, c.Ativa, c.Natureza, id); err != nil {
			return err
		}
		if tipoAntes != c.Tipo {
			return refazerTipoDasTransacoes(r.Context(), tx, id, c.Tipo)
		}
		return nil
	})
	switch {
	case err != nil:
		s.erroInterno(w, r, "editar categoria", err)
	case naoExiste:
		respostas.Mensagem(w, http.StatusNotFound, mensagemNaoEncontrado)
	case problema != "":
		respostas.Mensagem(w, http.StatusBadRequest, problema)
	default:
		respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, c)
	}
}

// refazerTipoDasTransacoes acerta as transações da categoria que trocou de tipo: para transferência, todas viram
// transferência fora dos relatórios; para despesa ou receita, as que não são transferência lançada pelo site ficam com
// o tipo pelo sinal (no cartão, entrada é estorno: despesa, ADR 0036) e voltam aos relatórios. AtualizadoEm não muda
// (o ExecuteUpdate do C# não mexe nele).
func refazerTipoDasTransacoes(ctx context.Context, tx *sql.Tx, categoriaId, tipo int64) error {
	if tipo == categorizacao.CategoriaTransferencia {
		_, err := tx.ExecContext(ctx, `UPDATE "Transacao" SET "Tipo" = ?, "IgnorarRelatorios" = 1 WHERE "CategoriaId" = ?`,
			transacoes.Transferencia, categoriaId)
		return err
	}
	cartoes, err := categorizacao.Cartoes(ctx, tx)
	if err != nil {
		return err
	}
	linhas, err := tx.QueryContext(ctx, `SELECT "Id", "ContaId", "Valor" FROM "Transacao"
		WHERE "CategoriaId" = ? AND "TransferenciaGrupo" IS NULL`, categoriaId)
	if err != nil {
		return err
	}
	porTipo := map[int64][]int64{}
	for linhas.Next() {
		var id, contaId int64
		var valor tipos.Decimal
		if err := linhas.Scan(&id, &contaId, &valor); err != nil {
			linhas.Close()
			return err
		}
		novo := categorizacao.TipoPeloSinal(valor, cartoes[contaId])
		porTipo[novo] = append(porTipo[novo], id)
	}
	linhas.Close()
	for _, novo := range []int64{transacoes.Despesa, transacoes.Receita} {
		for ids := porTipo[novo]; len(ids) > 0; {
			lote := ids[:min(len(ids), 500)]
			ids = ids[len(lote):]
			argumentos := []any{novo}
			marcas := make([]string, len(lote))
			for i, id := range lote {
				marcas[i] = "?"
				argumentos = append(argumentos, id)
			}
			if _, err := tx.ExecContext(ctx, `UPDATE "Transacao" SET "Tipo" = ?, "IgnorarRelatorios" = 0 WHERE "Id" IN (`+
				strings.Join(marcas, ", ")+`)`, argumentos...); err != nil {
				return err
			}
		}
	}
	return nil
}

// excluirCategoria: as transações ficam sem categoria (e deixam de ser de categoria manual); orçamentos e regras dela
// saem pela chave estrangeira em cascata.
func (s *Servico) excluirCategoria(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	var naoExiste, temFilhas bool
	err := banco.EmTransacao(r.Context(), s.db, func(tx *sql.Tx) error {
		var existe bool
		if err := tx.QueryRowContext(r.Context(), `SELECT EXISTS (SELECT 1 FROM "Categoria" WHERE "Id" = ?)`, id).Scan(&existe); err != nil {
			return err
		}
		if !existe {
			naoExiste = true
			return nil
		}
		if err := tx.QueryRowContext(r.Context(), `SELECT EXISTS (SELECT 1 FROM "Categoria" WHERE "CategoriaPaiId" = ?)`, id).Scan(&temFilhas); err != nil || temFilhas {
			return err
		}
		if _, err := tx.ExecContext(r.Context(), `UPDATE "Transacao" SET "CategoriaId" = NULL, "CategoriaManual" = 0
			WHERE "CategoriaId" = ?`, id); err != nil {
			return err
		}
		_, err := tx.ExecContext(r.Context(), `DELETE FROM "Categoria" WHERE "Id" = ?`, id)
		return err
	})
	switch {
	case err != nil:
		s.erroInterno(w, r, "excluir categoria", err)
	case naoExiste:
		respostas.Mensagem(w, http.StatusNotFound, mensagemNaoEncontrado)
	case temFilhas:
		respostas.Mensagem(w, http.StatusBadRequest, "Exclua ou mova as subcategorias antes.")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
