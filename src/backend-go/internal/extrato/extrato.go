// Pacote extrato são as rotas de /api/transacoes, a tela de Transações (o TransacoesController do C#). O modelo da
// transação que as regras leem e gravam fica em internal/transacoes, que a categorização e as transferências usam:
// as rotas ficam aqui para esses pacotes não dependerem de quem responde HTTP.
//
// Parte 1 (#78): a lista, com os filtros, a ordem, a paginação e os totais, obter e o lançamento manual (o 04-contas
// lança uma compra por ele). Parte 2: as sugestões de categoria (o pacote sugestoes). Parte 3: alterar, ajustar,
// excluir, as alterações em lote e a transferência. Parte 4: o renomear (#123) e a categoria em lote com o Desfazer
// (#87, o pacote desfazer).
package extrato

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/desfazer"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/poucoinformativa"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/servidor"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipolancamento"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
)

const mensagemNaoEncontrado = "Registro não encontrado."

// parametros é o que o extrato usa dos Parâmetros (o ParametroService do C#): o mínimo de letras da descrição pouco
// informativa. A interface é declarada aqui, por quem usa: o serviço de verdade é o internal/parametros, e o teste
// passa um valor fixo.
type parametros interface {
	ObterInteiro(ctx context.Context, chave string) (int, error)
}

// Servico atende as rotas do extrato.
type Servico struct {
	db         *sql.DB
	relogio    relogio.Relogio
	parametros parametros
	log        *slog.Logger
	// O estado de antes das alterações em lote, para o Desfazer: na memória, por 10 minutos (o IMemoryCache do C#).
	memoriaCategorias *desfazer.Memoria[[]categoriaAntes]
	memoriaNomes      *desfazer.Memoria[desfazerNome]
}

// NovoServico monta o serviço com o banco, o relógio, os parâmetros e o log.
func NovoServico(db *sql.DB, r relogio.Relogio, p parametros, log *slog.Logger) *Servico {
	return &Servico{db: db, relogio: r, parametros: p, log: log,
		memoriaCategorias: desfazer.Nova[[]categoriaAntes](r), memoriaNomes: desfazer.Nova[desfazerNome](r)}
}

// Registrar põe as rotas no mux.
func Registrar(mux *http.ServeMux, s *Servico) {
	mux.HandleFunc("GET /api/transacoes", s.listar)
	mux.HandleFunc("GET /api/transacoes/{id}", s.obter)
	mux.HandleFunc("POST /api/transacoes", s.criar)
	mux.HandleFunc("GET /api/transacoes/sugestoes-categoria", s.sugestoesDeCategoria)
	mux.HandleFunc("GET /api/transacoes/{id}/sugestoes-categoria", s.sugestoesDaTransacao)
	mux.HandleFunc("PUT /api/transacoes/{id}", s.atualizar)
	mux.HandleFunc("PATCH /api/transacoes/{id}", s.ajustar)
	mux.HandleFunc("DELETE /api/transacoes/{id}", s.excluir)
	mux.HandleFunc("PUT /api/transacoes/emergencia-em-lote", s.emergenciaEmLote)
	mux.HandleFunc("PUT /api/transacoes/revisada-em-lote", s.revisadaEmLote)
	mux.HandleFunc("PUT /api/transacoes/categoria-em-lote", s.categoriaEmLote)
	mux.HandleFunc("POST /api/transacoes/transferencia", s.transferir)
	mux.HandleFunc("PATCH /api/transacoes/em-lote", s.categoriasEmLote)
	mux.HandleFunc("POST /api/transacoes/em-lote/desfazer/{chave}", s.desfazerCategorias)
	mux.HandleFunc("GET /api/transacoes/{id}/renomear/previa", s.previaRenomear)
	mux.HandleFunc("PUT /api/transacoes/{id}/descricao", s.renomear)
	mux.HandleFunc("PATCH /api/transacoes/descricao/em-lote", s.renomearEmLote)
	mux.HandleFunc("POST /api/transacoes/descricao/desfazer/{chave}", s.desfazerRenomear)
}

func (s *Servico) erroInterno(w http.ResponseWriter, r *http.Request, etapa string, err error) {
	if errors.Is(err, context.Canceled) || r.Context().Err() != nil {
		return
	}
	s.log.ErrorContext(r.Context(), "erro nas transações", "origem", "API", "etapa", etapa, "erro", err)
	servidor.ResponderErroInterno(w)
}

// TransacaoDto, na ordem do record do C#.
type TransacaoDto struct {
	Id                int64               `json:"id"`
	ContaId           int64               `json:"contaId"`
	ContaNome         string              `json:"contaNome"`
	OpenFinance       bool                `json:"openFinance"`
	Data              tipos.Data          `json:"data"`
	Descricao         string              `json:"descricao"`
	DescricaoOriginal *string             `json:"descricaoOriginal"`
	Estabelecimento   *string             `json:"estabelecimento"`
	Valor             tipos.Decimal       `json:"valor"`
	Tipo              int64               `json:"tipo"`
	Status            int64               `json:"status"`
	CategoriaId       *int64              `json:"categoriaId"`
	CategoriaNome     *string             `json:"categoriaNome"`
	CategoriaCor      *string             `json:"categoriaCor"`
	ParcelaNumero     *int64              `json:"parcelaNumero"`
	ParcelaTotal      *int64              `json:"parcelaTotal"`
	IgnorarRelatorios bool                `json:"ignorarRelatorios"`
	Observacao        *string             `json:"observacao"`
	Emergencia        bool                `json:"emergencia"`
	TipoLancamento    tipolancamento.Tipo `json:"tipoLancamento"`
	RegraDescricaoId  *int64              `json:"regraDescricaoId"`
}

// PaginaDto é o PaginaTransacoesDto do C#.
type PaginaDto struct {
	Itens         []TransacaoDto `json:"itens"`
	Total         int            `json:"total"`
	Pagina        int64          `json:"pagina"`
	TamanhoPagina int64          `json:"tamanhoPagina"`
	TotalReceitas tipos.Decimal  `json:"totalReceitas"`
	TotalDespesas tipos.Decimal  `json:"totalDespesas"`
}

// de é o FROM das consultas: a transação com a conta, a categoria e a categoria pai (a natureza vem dela quando a
// categoria não tem).
const de = `FROM "Transacao" AS t INNER JOIN "Conta" AS co ON co."Id" = t."ContaId"
	LEFT JOIN "Categoria" AS ca ON ca."Id" = t."CategoriaId" LEFT JOIN "Categoria" AS pai ON pai."Id" = ca."CategoriaPaiId"`

const colunasDoDto = `t."Id", t."ContaId", co."Nome", t."PluggyTransactionId" IS NOT NULL, t."Data", t."Descricao",
	t."DescricaoOriginal", t."Estabelecimento", t."Valor", t."Tipo", t."Status", t."CategoriaId", ca."Nome", ca."Cor",
	t."ParcelaNumero", t."ParcelaTotal", t."IgnorarRelatorios", t."Observacao", t."Emergencia", t."MeioPagamento",
	t."MesmaTitularidade", t."TransferenciaGrupo" IS NOT NULL, co."Tipo", t."RegraDescricaoId"`

// lerDtos lê as transações da consulta com as colunas do DTO, e calcula o tipo de lançamento (ADR 0025) de cada uma.
func lerDtos(ctx context.Context, q consultas.DBTX, resto string, argumentos ...any) ([]TransacaoDto, error) {
	linhas, err := q.QueryContext(ctx, `SELECT `+colunasDoDto+` `+de+` `+resto, argumentos...)
	if err != nil {
		return nil, fmt.Errorf("ler as transações: %w", err)
	}
	defer linhas.Close()
	lista := []TransacaoDto{}
	for linhas.Next() {
		var d TransacaoDto
		var meio *string
		var mesmaTitularidade *bool
		var doSistema bool
		var tipoConta int64
		if err := linhas.Scan(&d.Id, &d.ContaId, &d.ContaNome, &d.OpenFinance, &d.Data, &d.Descricao, &d.DescricaoOriginal,
			&d.Estabelecimento, &d.Valor, &d.Tipo, &d.Status, &d.CategoriaId, &d.CategoriaNome, &d.CategoriaCor,
			&d.ParcelaNumero, &d.ParcelaTotal, &d.IgnorarRelatorios, &d.Observacao, &d.Emergencia, &meio,
			&mesmaTitularidade, &doSistema, &tipoConta, &d.RegraDescricaoId); err != nil {
			return nil, fmt.Errorf("ler transação: %w", err)
		}
		d.TipoLancamento = tipolancamento.Classificar(d.Valor, d.Descricao, d.DescricaoOriginal, meio, mesmaTitularidade,
			doSistema, tipoConta)
		lista = append(lista, d)
	}
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("ler as transações: %w", err)
	}
	return lista, nil
}

// onde junta as condições do WHERE, cada uma com os argumentos dela (só "?": texto do usuário nunca vai no SQL).
type onde struct {
	condicoes  []string
	argumentos []any
}

func (o *onde) e(condicao string, argumentos ...any) {
	o.condicoes = append(o.condicoes, "("+condicao+")")
	o.argumentos = append(o.argumentos, argumentos...)
}

// com devolve uma cópia com mais uma condição, sem mexer nesta (as fatias não são compartilhadas).
func (o onde) com(condicao string, argumentos ...any) onde {
	c := onde{condicoes: slices.Clone(o.condicoes), argumentos: slices.Clone(o.argumentos)}
	c.e(condicao, argumentos...)
	return c
}

func (o onde) sql() string {
	if len(o.condicoes) == 0 {
		return ""
	}
	return "WHERE " + strings.Join(o.condicoes, " AND ")
}

// lista devolve "?, ?, ?" e os argumentos, para um IN.
func lista[T any](valores []T) (string, []any) {
	args := make([]any, len(valores))
	for i, v := range valores {
		args[i] = v
	}
	return strings.TrimSuffix(strings.Repeat("?, ", len(valores)), ", "), args
}

// escaparLike é o do C#: a barra invertida, o % e o _ digitados valem como texto (ESCAPE '\').
var escaparLike = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// filtrar monta o WHERE dos filtros (o FiltrarAsync do C#). Os dois filtros que o SQL não sabe fazer (o tipo de
// lançamento e a descrição pouco informativa) são contas na memória sobre o que os outros já restringiram, e voltam
// para o SQL como uma lista de Ids.
func (s *Servico) filtrar(ctx context.Context, f filtro) (onde, error) {
	var o onde
	if f.Inicio != nil {
		o.e(`t."Data" >= ?`, *f.Inicio)
	}
	if f.Fim != nil {
		o.e(`t."Data" <= ?`, *f.Fim)
	}
	if f.ContaId != nil {
		o.e(`t."ContaId" = ?`, *f.ContaId)
	}
	if len(f.Ids) > 0 {
		marcas, args := lista(f.Ids)
		o.e(`t."Id" IN (`+marcas+`)`, args...)
	}
	// Categorias marcadas (cada uma com as subcategorias) e/ou "sem categoria": basta atender a uma delas.
	categorias := slices.Clone(f.CategoriaIds)
	if f.CategoriaId != nil {
		categorias = append(categorias, *f.CategoriaId)
	}
	slices.Sort(categorias)
	categorias = slices.Compact(categorias)
	if f.SemCategoria || len(categorias) > 0 {
		marcas, args := lista(categorias)
		o.e(`(? AND t."CategoriaId" IS NULL) OR (t."CategoriaId" IS NOT NULL AND (t."CategoriaId" IN (`+marcas+`)
			OR (ca."CategoriaPaiId" IS NOT NULL AND ca."CategoriaPaiId" IN (`+marcas+`))))`,
			append(append([]any{f.SemCategoria}, args...), args...)...)
	}
	// Natureza da categoria ou, se ela não tem, a da categoria pai; basta atender a uma das marcadas (OU).
	if f.SemNatureza || len(f.Naturezas) > 0 {
		marcas, args := lista(f.Naturezas)
		o.e(`(? AND COALESCE(ca."Natureza", pai."Natureza") IS NULL) OR COALESCE(ca."Natureza", pai."Natureza") IN (`+marcas+`)`,
			append([]any{f.SemNatureza}, args...)...)
	}
	if f.Tipo != nil {
		o.e(`t."Tipo" = ?`, *f.Tipo)
	}
	if f.SomenteEmergencias {
		o.e(`t."Emergencia"`)
	}
	if f.Busca != nil && strings.TrimSpace(*f.Busca) != "" {
		// #123: acha pelo nome mostrado e pelo nome que veio do banco. #158: o LIKE do SQLite ignora a caixa das letras
		// sem acento ("netflix" acha "NETFLIX.COM").
		padrao := "%" + escaparLike.Replace(strings.TrimSpace(*f.Busca)) + "%"
		o.e(`t."Descricao" LIKE ? ESCAPE '\' OR (t."DescricaoOriginal" IS NOT NULL AND t."DescricaoOriginal" LIKE ? ESCAPE '\')
			OR (t."Estabelecimento" IS NOT NULL AND t."Estabelecimento" LIKE ? ESCAPE '\')
			OR (t."Observacao" IS NOT NULL AND t."Observacao" LIKE ? ESCAPE '\')`, padrao, padrao, padrao, padrao)
	}
	if f.Lancamento != nil {
		ids, err := s.idsDoLancamento(ctx, o, tipolancamento.Grupos[*f.Lancamento])
		if err != nil {
			return onde{}, err
		}
		marcas, args := lista(ids)
		o.e(`t."Id" IN (`+marcas+`)`, args...)
	}
	if f.PoucoInformativa {
		ids, err := s.idsPoucoInformativos(ctx, o)
		if err != nil {
			return onde{}, err
		}
		marcas, args := lista(ids)
		o.e(`t."Id" IN (`+marcas+`)`, args...)
	}
	return o, nil
}

// idsDoLancamento (#111): o tipo de lançamento é calculado (o dado do banco ou a descrição), então a conta é na
// memória, sobre o que os filtros anteriores deixaram.
func (s *Servico) idsDoLancamento(ctx context.Context, o onde, grupo []tipolancamento.Tipo) ([]int64, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT t."Id", t."Valor", t."Descricao", t."DescricaoOriginal", t."MeioPagamento",
		t."MesmaTitularidade", t."TransferenciaGrupo" IS NOT NULL, co."Tipo" `+de+` `+o.sql(), o.argumentos...)
	if err != nil {
		return nil, fmt.Errorf("ler as transações para o tipo de lançamento: %w", err)
	}
	defer linhas.Close()
	ids := []int64{}
	for linhas.Next() {
		var id, tipoConta int64
		var valor tipos.Decimal
		var descricao string
		var original, meio *string
		var mesma *bool
		var doSistema bool
		if err := linhas.Scan(&id, &valor, &descricao, &original, &meio, &mesma, &doSistema, &tipoConta); err != nil {
			return nil, fmt.Errorf("ler transação para o tipo de lançamento: %w", err)
		}
		if slices.Contains(grupo, tipolancamento.Classificar(valor, descricao, original, meio, mesma, doSistema, tipoConta)) {
			ids = append(ids, id)
		}
	}
	return ids, linhas.Err()
}

// idsPoucoInformativos (#9): as que ainda pedem atenção e têm menos letras úteis que o mínimo dos parâmetros. A conta
// é na memória, sobre o que os outros filtros já restringiram.
func (s *Servico) idsPoucoInformativos(ctx context.Context, o onde) ([]int64, error) {
	minimo, err := s.parametros.ObterInteiro(ctx, poucoinformativa.ChaveMinimoLetras)
	if err != nil {
		return nil, err
	}
	c := o.com(poucoinformativa.Candidatas)
	linhas, err := s.db.QueryContext(ctx, `SELECT t."Id", t."Descricao" `+de+` `+c.sql(), c.argumentos...)
	if err != nil {
		return nil, fmt.Errorf("ler as transações pouco informativas: %w", err)
	}
	defer linhas.Close()
	ids := []int64{}
	for linhas.Next() {
		var id int64
		var descricao string
		if err := linhas.Scan(&id, &descricao); err != nil {
			return nil, fmt.Errorf("ler transação pouco informativa: %w", err)
		}
		if poucoinformativa.EhPoucoInformativa(descricao, minimo) {
			ids = append(ids, id)
		}
	}
	return ids, linhas.Err()
}

// listar é o Listar do C#: a página pedida, o total e as somas do filtro inteiro.
func (s *Servico) listar(w http.ResponseWriter, r *http.Request) {
	f, erros := lerFiltro(r)
	if len(erros) > 0 {
		respostas.Validacao(w, erros)
		return
	}
	pagina, err := s.pagina(r.Context(), f)
	if err != nil {
		s.erroInterno(w, r, "listar as transações", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, pagina)
}

func (s *Servico) pagina(ctx context.Context, f filtro) (PaginaDto, error) {
	o, err := s.filtrar(ctx, f)
	if err != nil {
		return PaginaDto{}, err
	}
	p := PaginaDto{Pagina: f.Pagina, TamanhoPagina: f.TamanhoPagina}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) `+de+` `+o.sql(), o.argumentos...).Scan(&p.Total); err != nil {
		return PaginaDto{}, fmt.Errorf("contar as transações: %w", err)
	}
	if p.TotalReceitas, p.TotalDespesas, err = s.totais(ctx, o); err != nil {
		return PaginaDto{}, err
	}
	if p.Itens, err = s.itens(ctx, o, f); err != nil {
		return PaginaDto{}, err
	}
	return p, nil
}

// totais: receitas e despesas fora das transferências e das ignoradas, pelo tipo e não pelo sinal (estorno no cartão
// é despesa positiva e abate as despesas, ADR 0036). A soma do C# é feita no SQLite pelo EF Core e volta na escala em
// que ele grava ("0.0" sem nenhuma linha); a despesa sai com o sinal trocado.
func (s *Servico) totais(ctx context.Context, o onde) (receitas, despesas tipos.Decimal, err error) {
	c := o.com(`NOT t."IgnorarRelatorios" AND t."Tipo" <> 3`)
	linhas, err := s.db.QueryContext(ctx, `SELECT t."Tipo", t."Valor" `+de+` `+c.sql(), c.argumentos...)
	if err != nil {
		return receitas, despesas, fmt.Errorf("somar as transações: %w", err)
	}
	defer linhas.Close()
	for linhas.Next() {
		var tipo int64
		var valor tipos.Decimal
		if err := linhas.Scan(&tipo, &valor); err != nil {
			return receitas, despesas, fmt.Errorf("somar transação: %w", err)
		}
		switch tipo {
		case 2:
			receitas = tipos.Somar(receitas, valor)
		case 1:
			despesas = tipos.Somar(despesas, valor)
		}
	}
	if err := linhas.Err(); err != nil {
		return receitas, despesas, fmt.Errorf("somar as transações: %w", err)
	}
	return receitas.NaEscalaDoBanco(), tipos.DeDecimal(despesas.NaEscalaDoBanco().Decimal().Neg()), nil
}

// itens é a página, na ordem pedida (#244). A ordem pelo valor (sem o sinal) é feita aqui, porque dinheiro é texto no
// banco; as outras, no SQL. Desempate sempre pela data mais recente e pelo Id, para a paginação não repetir nem pular.
func (s *Servico) itens(ctx context.Context, o onde, f filtro) ([]TransacaoDto, error) {
	crescente := f.Direcao != nil && *f.Direcao == "asc"
	desc := " DESC"
	if crescente {
		desc = ""
	}
	pular := (f.Pagina - 1) * f.TamanhoPagina
	ordem := ""
	switch {
	case f.Ordem != nil && *f.Ordem == "valor":
		return s.itensPeloValor(ctx, o, crescente, pular, f.TamanhoPagina)
	case f.Ordem != nil && *f.Ordem == "categoria":
		// Sem categoria no fim nas duas direções; o nome sem diferença de maiúsculas (o NOCASE do SQLite, só ASCII).
		ordem = `t."CategoriaId" IS NULL, ca."Nome" COLLATE NOCASE` + desc + `, t."Data" DESC, t."Id" DESC`
	case f.Ordem != nil && *f.Ordem == "conta":
		ordem = `co."Nome" COLLATE NOCASE` + desc + `, t."Data" DESC, t."Id" DESC`
	default:
		ordem = `t."Data"` + desc + `, t."Id"` + desc
	}
	return lerDtos(ctx, s.db, o.sql()+` ORDER BY `+ordem+` LIMIT ? OFFSET ?`,
		append(slices.Clone(o.argumentos), f.TamanhoPagina, pular)...)
}

func (s *Servico) itensPeloValor(ctx context.Context, o onde, crescente bool, pular, quantos int64) ([]TransacaoDto, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT t."Id", t."Data", t."Valor" `+de+` `+o.sql(), o.argumentos...)
	if err != nil {
		return nil, fmt.Errorf("ler as transações para ordenar pelo valor: %w", err)
	}
	type chave struct {
		id    int64
		data  tipos.Data
		valor tipos.Decimal
	}
	var todas []chave
	for linhas.Next() {
		var c chave
		if err := linhas.Scan(&c.id, &c.data, &c.valor); err != nil {
			linhas.Close()
			return nil, fmt.Errorf("ler transação para ordenar pelo valor: %w", err)
		}
		todas = append(todas, c)
	}
	linhas.Close()
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("ler as transações para ordenar pelo valor: %w", err)
	}
	slices.SortFunc(todas, func(a, b chave) int {
		c := compararComoOEFDecimal(a.valor, b.valor)
		if !crescente {
			c = -c
		}
		if c != 0 {
			return c
		}
		if c := b.data.NumeroDoDia() - a.data.NumeroDoDia(); c != 0 {
			return c
		}
		return cmp.Compare(b.id, a.id)
	})
	if pular >= int64(len(todas)) {
		return []TransacaoDto{}, nil
	}
	todas = todas[pular:min(int64(len(todas)), pular+quantos)]
	ids := make([]int64, len(todas))
	posicao := make(map[int64]int, len(todas))
	for i, c := range todas {
		ids[i], posicao[c.id] = c.id, i
	}
	marcas, args := lista(ids)
	dtos, err := lerDtos(ctx, s.db, `WHERE t."Id" IN (`+marcas+`)`, args...)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(dtos, func(a, b TransacaoDto) int { return posicao[a.Id] - posicao[b.Id] })
	return dtos, nil
}

// compararComoOEFDecimal é a ordem que o C# dá ao valor sem o sinal, com o defeito dela (#499). O EF Core ordena o
// decimal no SQLite com a collation EF_DECIMAL, que lê o texto com a cultura do processo: em pt-BR o ponto é separador
// de milhar, e "12.75" vale 1275, depois de "18.9" (189) e de "100.0" (1000). Conferido com o próprio EF Core (sonda
// da #78) e no contrato 07-transacoes ("ordem por valor crescente"). O texto é o que a consulta do C# compara: o valor
// gravado, para o positivo, e o ef_negate dele (o decimal no formato do banco), para o negativo.
func compararComoOEFDecimal(a, b tipos.Decimal) int {
	chave := func(v tipos.Decimal) *big.Int {
		texto := v.String()
		if v.Decimal().IsNegative() {
			texto = tipos.DeDecimal(v.Decimal().Neg()).TextoNoBanco()
		}
		n, _ := new(big.Int).SetString(strings.ReplaceAll(texto, ".", ""), 10)
		return n
	}
	return chave(a).Cmp(chave(b))
}

// obter é o Obter do C#: {id:long}; o que não é número de 64 bits não casa com a rota (404 sem corpo).
func (s *Servico) obter(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimPrefix(r.PathValue("id"), "+"), 10, 64)
	if err != nil {
		respostas.SemCorpo(w, http.StatusNotFound)
		return
	}
	lista, err := lerDtos(r.Context(), s.db, `WHERE t."Id" = ?`, id)
	if err != nil {
		s.erroInterno(w, r, "ler a transação", err)
		return
	}
	if len(lista) == 0 {
		respostas.Mensagem(w, http.StatusNotFound, mensagemNaoEncontrado)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, lista[0])
}

// MaioresDespesas são as maiores despesas do período (o GET /api/relatorios/maiores-despesas do C#, #80), sem as
// ignoradas nos relatórios, no máximo quantas. O C# ordena pelo valor gravado no SQLite, com a collation EF_DECIMAL,
// que lê o texto em pt-BR (o ponto é separador de milhar): "-1850.0" vale −18500, "-299.9" vale −2999 e "-27.45" vale
// −2745, que vem antes de "-223.5" (−2235). Aqui a ordem é em Go (dinheiro é texto no banco), com a mesma leitura
// (contrato 14-relatorios). O empate fica na ordem em que o SQLite percorre o período: o índice IX_Transacao_Data, da
// data mais nova para a mais velha.
func MaioresDespesas(ctx context.Context, q consultas.DBTX, inicio, fim tipos.Data, quantas int) ([]TransacaoDto, error) {
	linhas, err := q.QueryContext(ctx, `SELECT "Id", "Valor" FROM "Transacao" WHERE "Data" >= ? AND "Data" <= ?
		AND "Tipo" = ? AND "IgnorarRelatorios" = 0 ORDER BY "Data" DESC, "Id"`, inicio, fim, transacoes.Despesa)
	if err != nil {
		return nil, fmt.Errorf("ler as despesas do período: %w", err)
	}
	type chave struct {
		id    int64
		valor tipos.Decimal
	}
	var todas []chave
	for linhas.Next() {
		var c chave
		if err := linhas.Scan(&c.id, &c.valor); err != nil {
			linhas.Close()
			return nil, fmt.Errorf("ler despesa do período: %w", err)
		}
		todas = append(todas, c)
	}
	linhas.Close()
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("ler as despesas do período: %w", err)
	}
	slices.SortStableFunc(todas, func(a, b chave) int { return compararComSinalComoOEFDecimal(a.valor, b.valor) })
	todas = todas[:min(len(todas), quantas)]
	if len(todas) == 0 {
		return []TransacaoDto{}, nil
	}
	ids := make([]int64, len(todas))
	posicao := make(map[int64]int, len(todas))
	for i, c := range todas {
		ids[i], posicao[c.id] = c.id, i
	}
	marcas, args := lista(ids)
	dtos, err := lerDtos(ctx, q, `WHERE t."Id" IN (`+marcas+`)`, args...)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(dtos, func(a, b TransacaoDto) int { return posicao[a.Id] - posicao[b.Id] })
	return dtos, nil
}

// compararComSinalComoOEFDecimal é a ordem que a collation EF_DECIMAL dá ao valor gravado, com o sinal: o texto lido
// em pt-BR, com o ponto de separador de milhar ("12.75" vale 1275; "-1850.0", −18500).
func compararComSinalComoOEFDecimal(a, b tipos.Decimal) int {
	chave := func(v tipos.Decimal) *big.Int {
		n, _ := new(big.Int).SetString(strings.ReplaceAll(v.String(), ".", ""), 10)
		return n
	}
	return chave(a).Cmp(chave(b))
}

// TransacoesDoPainel são as transações do painel (#334): as últimas até hoje (as parcelas que o banco manda com data
// futura escondiam o que de fato aconteceu) e, à parte, as próximas depois de hoje, das mais próximas, com quantas são.
func TransacoesDoPainel(ctx context.Context, q consultas.DBTX, hoje tipos.Data, quantas int) (ultimas, futuras []TransacaoDto,
	quantidadeFuturas int, err error) {
	if ultimas, err = lerDtos(ctx, q, `WHERE t."Data" <= ? ORDER BY t."Data" DESC, t."Id" DESC LIMIT ?`, hoje, quantas); err != nil {
		return nil, nil, 0, err
	}
	if futuras, err = lerDtos(ctx, q, `WHERE t."Data" > ? ORDER BY t."Data", t."Id" LIMIT ?`, hoje, quantas); err != nil {
		return nil, nil, 0, err
	}
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM "Transacao" WHERE "Data" > ?`, hoje).Scan(&quantidadeFuturas); err != nil {
		return nil, nil, 0, fmt.Errorf("contar as transações futuras: %w", err)
	}
	return ultimas, futuras, quantidadeFuturas, nil
}
