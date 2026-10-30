package extrato

import (
	"database/sql"
	"fmt"
	"math"
	"net/http"
	"slices"
	"strconv"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/auditoria"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/categorizacao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/desfazer"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transferencias"
)

// maximoPorFiltro é o MaximoPorFiltro do C# (#87): "todas do filtro" de uma vez, no máximo.
const maximoPorFiltro = 5000

// categoriaAntes é o EstadoAnterior do C#: como a transação estava antes da alteração em lote, e a categoria que o lote
// deu (o Desfazer só volta a que continua com ela).
type categoriaAntes struct {
	id                int64
	categoria         *int64
	manual            bool
	tipo              int64
	ignorarRelatorios bool
	aplicada          *int64
}

// ResultadoCategoriasEmLoteDto é o do C#.
type ResultadoCategoriasEmLoteDto struct {
	Alteradas int     `json:"alteradas"`
	Desfazer  *string `json:"desfazer"`
}

// filtroDoCorpo são as propriedades do FiltroTransacoesDto lido do corpo (JSON), com os padrões e os atributos da
// classe: no corpo, os enums são número, e o texto vazio fica vazio (na query, vira null).
type filtroDoCorpo struct {
	inicio, fim                                              respostas.CampoDataOpcional
	conta, categoria                                         respostas.CampoInteiroOpcional
	categorias, naturezas, ids                               respostas.CampoLista
	semCategoria, semNatureza, emergencias, poucoInformativa respostas.CampoBool
	lancamento, busca, ordem, direcao                        respostas.CampoTexto
	tipo                                                     respostas.CampoEnum
	pagina, tamanho                                          respostas.CampoInteiro
}

func novoFiltroDoCorpo() *filtroDoCorpo {
	semLimite := math.MaxInt
	return &filtroDoCorpo{
		inicio: respostas.CampoDataOpcional{Propriedade: "Inicio"}, fim: respostas.CampoDataOpcional{Propriedade: "Fim"},
		conta:      respostas.CampoInteiroOpcional{Propriedade: "ContaId"},
		categoria:  respostas.CampoInteiroOpcional{Propriedade: "CategoriaId"},
		categorias: respostas.CampoLista{Propriedade: "CategoriaIds", Item: respostas.ItemInt, Opcional: true, Maximo: 200},
		naturezas: respostas.CampoLista{Propriedade: "Naturezas", Item: respostas.ItemEnum("Financas.Api.Entidades.NaturezaGasto"),
			Opcional: true, Maximo: 2},
		ids:              respostas.CampoLista{Propriedade: "Ids", Item: respostas.ItemLong, Opcional: true, Maximo: 200},
		semCategoria:     respostas.CampoBool{Propriedade: "SemCategoria"},
		semNatureza:      respostas.CampoBool{Propriedade: "SemNatureza"},
		emergencias:      respostas.CampoBool{Propriedade: "SomenteEmergencias"},
		poucoInformativa: respostas.CampoBool{Propriedade: "PoucoInformativa"},
		lancamento:       respostas.CampoTexto{Propriedade: "Lancamento", Maximo: semLimite, Opcional: true, Padrao: padraoLancamento},
		busca:            respostas.CampoTexto{Propriedade: "Busca", Maximo: 100, Opcional: true},
		ordem:            respostas.CampoTexto{Propriedade: "Ordem", Maximo: semLimite, Opcional: true, Padrao: padraoOrdem},
		direcao:          respostas.CampoTexto{Propriedade: "Direcao", Maximo: semLimite, Opcional: true, Padrao: padraoDirecao},
		tipo:             respostas.CampoEnum{Propriedade: "Tipo", Anulavel: true, Tipo: "Financas.Api.Entidades.TipoTransacao"},
		pagina:           respostas.CampoInteiro{Propriedade: "Pagina", Faixa: &respostas.Faixa{Minimo: 1, Maximo: math.MaxInt32}, Valor: 1},
		tamanho:          respostas.CampoInteiro{Propriedade: "TamanhoPagina", Faixa: &respostas.Faixa{Minimo: 1, Maximo: 500}, Valor: 50},
	}
}

func (f *filtroDoCorpo) campos() []respostas.Campo {
	return []respostas.Campo{&f.inicio, &f.fim, &f.conta, &f.categoria, &f.categorias, &f.semCategoria, &f.naturezas,
		&f.semNatureza, &f.emergencias, &f.poucoInformativa, &f.lancamento, &f.tipo, &f.busca, &f.ids, &f.pagina,
		&f.tamanho, &f.ordem, &f.direcao}
}

func (f *filtroDoCorpo) filtro() filtro {
	return filtro{Inicio: f.inicio.Valor, Fim: f.fim.Valor, ContaId: f.conta.Valor, CategoriaId: f.categoria.Valor,
		CategoriaIds: f.categorias.Valor, SemCategoria: f.semCategoria.Valor, Naturezas: f.naturezas.Valor,
		SemNatureza: f.semNatureza.Valor, SomenteEmergencias: f.emergencias.Valor, PoucoInformativa: f.poucoInformativa.Valor,
		Lancamento: f.lancamento.Valor, Tipo: f.tipo.Valor, Busca: f.busca.Valor, Ids: f.ids.Valor, Pagina: f.pagina.Valor,
		TamanhoPagina: f.tamanho.Valor, Ordem: f.ordem.Valor, Direcao: f.direcao.Valor}
}

type escolha struct {
	id         int64
	categoria  *int64
	automatica bool
}

// lerCategoriasEmLote é o model binding do AlterarCategoriasEmLoteDto (uma classe): Itens (até 1000, cada um um
// AlteracaoCategoriaDto), Filtro, CategoriaId e Automatica. Um item null volta como nil.
func lerCategoriasEmLote(w http.ResponseWriter, r *http.Request) (itens []*escolha, f *filtro, geral escolha, ok bool) {
	itensCampo := respostas.CampoListaDeObjetos{Propriedade: "Itens", TipoItem: "Financas.Api.Dtos.AlteracaoCategoriaDto",
		Maximo: 1000, Novo: func() []respostas.Campo {
			return []respostas.Campo{&respostas.CampoLong{Propriedade: "Id"}, &respostas.CampoInteiroOpcional{Propriedade: "CategoriaId"},
				&respostas.CampoBool{Propriedade: "Automatica"}}
		}}
	filtroCampos := novoFiltroDoCorpo()
	filtroCampo := respostas.CampoObjeto{Propriedade: "Filtro", Tipo: "Financas.Api.Dtos.FiltroTransacoesDto",
		Campos: filtroCampos.campos()}
	categoria := respostas.CampoInteiroOpcional{Propriedade: "CategoriaId"}
	automatica := respostas.CampoBool{Propriedade: "Automatica"}
	if !respostas.LerClasse(w, r, "dto", "Financas.Api.Dtos.AlterarCategoriasEmLoteDto", &itensCampo, &filtroCampo,
		&categoria, &automatica) {
		return nil, nil, escolha{}, false
	}
	for _, campos := range itensCampo.Itens {
		if campos == nil {
			itens = append(itens, nil)
			continue
		}
		itens = append(itens, &escolha{campos[0].(*respostas.CampoLong).Valor, campos[1].(*respostas.CampoInteiroOpcional).Valor,
			campos[2].(*respostas.CampoBool).Valor})
	}
	if filtroCampo.Veio {
		x := filtroCampos.filtro()
		f = &x
	}
	return itens, f, escolha{categoria: categoria.Valor, automatica: automatica.Valor}, true
}

// categoriasEmLote é o PATCH /api/transacoes/em-lote (#87, o modo edição de Transações): troca só a categoria, por
// transação (Itens) ou de todas as transações do filtro da busca, com as regras do PATCH de uma: a escolhida fica
// manual, "automática" reaplica as regras, e o tipo acompanha. O estado de antes fica na memória, para o Desfazer.
func (s *Servico) categoriasEmLote(w http.ResponseWriter, r *http.Request) {
	itens, f, geral, ok := lerCategoriasEmLote(w, r)
	if !ok {
		return
	}
	if slices.Contains(itens, nil) {
		// O C# recebe o item null e quebra ao ler o Id dele (NullReferenceException): o 500.
		s.erroInterno(w, r, "ler os itens", fmt.Errorf("item nulo na lista"))
		return
	}
	porItens := len(itens) > 0
	if porItens == (f != nil) {
		respostas.Mensagem(w, http.StatusBadRequest, "Informe as transações ou o filtro (um dos dois).")
		return
	}
	escolhas := []escolha{geral}
	if porItens {
		escolhas = escolhas[:0]
		for _, i := range itens {
			escolhas = append(escolhas, *i)
		}
	}
	var categorias []int64
	for _, e := range escolhas {
		if e.automatica {
			continue
		}
		if e.categoria == nil {
			respostas.Mensagem(w, http.StatusBadRequest, "Escolha uma categoria ou a categorização automática.")
			return
		}
		if !slices.Contains(categorias, *e.categoria) {
			categorias = append(categorias, *e.categoria)
		}
	}
	if len(categorias) > 0 {
		em, argumentos := emLista(categorias)
		var n int
		if err := s.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM "Categoria" WHERE "Id" IN `+em, argumentos...).Scan(&n); err != nil {
			s.erroInterno(w, r, "conferir as categorias", err)
			return
		}
		if n != len(categorias) {
			respostas.Mensagem(w, http.StatusBadRequest, "Categoria inválida.")
			return
		}
	}

	var lista []*transacoes.Transacao
	escolhaDe := map[int64]escolha{}
	if porItens {
		var ids []int64
		for _, i := range itens {
			if !slices.Contains(ids, i.id) {
				ids = append(ids, i.id)
			}
		}
		if len(ids) != len(itens) {
			respostas.Mensagem(w, http.StatusBadRequest, "Transação repetida na lista.")
			return
		}
		em, argumentos := emLista(ids)
		var err error
		if lista, err = transacoes.Listar(r.Context(), s.db, `WHERE "Id" IN `+em, argumentos...); err != nil {
			s.erroInterno(w, r, "ler as transações", err)
			return
		}
		if len(lista) != len(ids) {
			respostas.Mensagem(w, http.StatusNotFound, "Alguma transação não existe mais. Atualize a lista.")
			return
		}
		for _, i := range itens {
			escolhaDe[i.id] = *i
		}
	} else {
		var err error
		var total int
		if lista, total, err = s.doFiltro(r, *f); err != nil {
			s.erroInterno(w, r, "ler as transações do filtro", err)
			return
		}
		if total > maximoPorFiltro {
			respostas.Mensagem(w, http.StatusBadRequest, "O filtro tem "+strconv.Itoa(total)+" transações; o limite de uma vez é "+
				strconv.Itoa(maximoPorFiltro)+". Restrinja o período.")
			return
		}
		for _, t := range lista {
			escolhaDe[t.Id] = geral
		}
	}

	var anteriores []categoriaAntes
	err := banco.EmTransacao(r.Context(), s.db, func(tx *sql.Tx) error {
		rs, err := categorizacao.Carregar(r.Context(), tx)
		if err != nil {
			return err
		}
		agora := tipos.NovaDataHora(s.relogio.Agora())
		for _, t := range lista {
			antes := categoriaAntes{id: t.Id, categoria: t.CategoriaId, manual: t.CategoriaManual, tipo: t.Tipo,
				ignorarRelatorios: t.IgnorarRelatorios}
			if e := escolhaDe[t.Id]; e.automatica {
				t.CategoriaManual = false
				t.CategoriaId = rs.Resolver(t)
			} else {
				t.CategoriaManual, t.CategoriaId = true, e.categoria
			}
			rs.AplicarTipo(t)
			if mesmoId(t.CategoriaId, antes.categoria) && t.CategoriaManual == antes.manual {
				continue
			}
			t.AtualizadoEm = agora
			antes.aplicada = t.CategoriaId
			anteriores = append(anteriores, antes)
		}
		if len(anteriores) == 0 {
			return nil
		}
		q := consultas.New(tx)
		login, err := auditoria.Login(r.Context(), q)
		if err != nil {
			return err
		}
		if err := auditoria.Registrar(r.Context(), q, r, agora, "CategoriaEmLote ("+strconv.Itoa(len(anteriores))+")", login); err != nil {
			return err
		}
		// Como no C#, o SaveChanges grava também o tipo que mudou nas que ficaram com a mesma categoria.
		if err := transacoes.GravarTodas(r.Context(), tx, lista); err != nil {
			return err
		}
		_, err = transferencias.Reclassificar(r.Context(), tx, agora)
		return err
	})
	if err != nil {
		s.erroInterno(w, r, "alterar as categorias em lote", err)
		return
	}
	resultado := ResultadoCategoriasEmLoteDto{Alteradas: len(anteriores)}
	if len(anteriores) > 0 {
		chave := s.memoriaCategorias.Guardar(anteriores)
		resultado.Desfazer = &chave
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, resultado)
}

// doFiltro lê as transações do filtro (o FiltrarAsync, sem página nem ordem) e quantas são.
func (s *Servico) doFiltro(r *http.Request, f filtro) ([]*transacoes.Transacao, int, error) {
	o, err := s.filtrar(r.Context(), f)
	if err != nil {
		return nil, 0, err
	}
	linhas, err := s.db.QueryContext(r.Context(), `SELECT t."Id" `+de+` `+o.sql(), o.argumentos...)
	if err != nil {
		return nil, 0, fmt.Errorf("ler os ids do filtro: %w", err)
	}
	var ids []int64
	for linhas.Next() {
		var id int64
		if err := linhas.Scan(&id); err != nil {
			linhas.Close()
			return nil, 0, err
		}
		ids = append(ids, id)
	}
	linhas.Close()
	if err := linhas.Err(); err != nil {
		return nil, 0, err
	}
	if len(ids) == 0 || len(ids) > maximoPorFiltro {
		return nil, len(ids), nil
	}
	em, argumentos := emLista(ids)
	lista, err := transacoes.Listar(r.Context(), s.db, `WHERE "Id" IN `+em, argumentos...)
	return lista, len(ids), err
}

// desfazerCategorias é o POST /api/transacoes/em-lote/desfazer/{chave}: volta a categoria, o "à mão", o tipo e o
// "fora dos relatórios" de antes. A que mudou de categoria de novo depois fica como está (não passa por cima de outra
// edição), e a categoria que não existe mais volta como nenhuma.
func (s *Servico) desfazerCategorias(w http.ResponseWriter, r *http.Request) {
	anteriores, ok := s.memoriaCategorias.Tirar(r.PathValue("chave"))
	if !ok {
		respostas.Mensagem(w, http.StatusNotFound, desfazer.MensagemVencida)
		return
	}
	var restauradas int
	err := banco.EmTransacao(r.Context(), s.db, func(tx *sql.Tx) error {
		porId := map[int64]categoriaAntes{}
		ids := make([]int64, len(anteriores))
		for i, a := range anteriores {
			porId[a.id], ids[i] = a, a.id
		}
		em, argumentos := emLista(ids)
		lista, err := transacoes.Listar(r.Context(), tx, `WHERE "Id" IN `+em, argumentos...)
		if err != nil {
			return err
		}
		existentes := map[int64]bool{}
		linhas, err := tx.QueryContext(r.Context(), `SELECT "Id" FROM "Categoria"`)
		if err != nil {
			return err
		}
		for linhas.Next() {
			var id int64
			if err := linhas.Scan(&id); err != nil {
				linhas.Close()
				return err
			}
			existentes[id] = true
		}
		linhas.Close()
		if err := linhas.Err(); err != nil {
			return err
		}
		agora := tipos.NovaDataHora(s.relogio.Agora())
		for _, t := range lista {
			a := porId[t.Id]
			if !mesmoId(t.CategoriaId, a.aplicada) {
				continue
			}
			t.CategoriaId = a.categoria
			if a.categoria != nil && !existentes[*a.categoria] {
				t.CategoriaId = nil
			}
			t.CategoriaManual, t.Tipo, t.IgnorarRelatorios, t.AtualizadoEm = a.manual, a.tipo, a.ignorarRelatorios, agora
			restauradas++
		}
		q := consultas.New(tx)
		login, err := auditoria.Login(r.Context(), q)
		if err != nil {
			return err
		}
		if err := auditoria.Registrar(r.Context(), q, r, agora, "CategoriaEmLoteDesfeita ("+strconv.Itoa(restauradas)+")", login); err != nil {
			return err
		}
		if err := transacoes.GravarTodas(r.Context(), tx, lista); err != nil {
			return err
		}
		_, err = transferencias.Reclassificar(r.Context(), tx, agora)
		return err
	})
	if err != nil {
		s.erroInterno(w, r, "desfazer as categorias em lote", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, ResultadoAplicacaoDto{int64(restauradas)})
}
