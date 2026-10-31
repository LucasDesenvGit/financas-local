package carteira

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/auditoria"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cambio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/categorizacao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
)

const observacaoDaCompraQueFaltou = "Compra que faltou no Open Finance (Conferir investimentos)"

// EntradaDaCorrecao é o CorrigirInvestimentosDto: as correções escolhidas. A taxa da B3 é recalculada aqui, não vem da tela.
type EntradaDaCorrecao struct {
	AtivosParaFii, Trocos, Rendimentos, Taxas []int64
	Compras                                   []ItemDoExtrato
}

// ResultadoDaCorrecao é o ResultadoCorrecaoInvestimentosDto.
type ResultadoDaCorrecao struct {
	Tipos       int     `json:"tipos"`
	Compras     int     `json:"compras"`
	Trocos      int     `json:"trocos"`
	Rendimentos int     `json:"rendimentos"`
	Taxas       int     `json:"taxas"`
	Desfazer    *string `json:"desfazer"`
}

// estadoAnterior é o estado de antes da correção, guardado só na memória para o "Desfazer".
type estadoAnterior struct {
	tipos            []tipoAnterior
	operacoesCriadas []int64
	transacoes       []transacaoAnterior
	taxas            []taxaAnterior
}

type tipoAnterior struct{ ativo, tipo int64 }

type transacaoAnterior struct {
	id, aplicada    int64
	categoria       *int64
	manual, ignorar bool
	tipo            int64
}

type taxaAnterior struct {
	operacao        int64
	antes, aplicada tipos.Decimal
}

func semRepetir(ids []int64) []int64 {
	var unicos []int64
	for _, id := range ids {
		if !slices.Contains(unicos, id) {
			unicos = append(unicos, id)
		}
	}
	return unicos
}

// listaDeIds devolve o "(?, ?, ?)" e os argumentos.
func listaDeIds(ids []int64) (string, []any) {
	argumentos := make([]any, len(ids))
	for i, id := range ids {
		argumentos[i] = id
	}
	return "(" + strings.TrimSuffix(strings.Repeat("?, ", len(ids)), ", ") + ")", argumentos
}

// correcao é o que uma correção vai gravar, junto.
type correcao struct {
	anterior estadoAnterior
	compras  []Operacao
	mudadas  []*transacoes.Transacao
	trocos   int
	rendas   int
}

func (s *Servico) corrigirTipos(ctx context.Context, ids []int64, c *correcao) (string, error) {
	if len(ids) == 0 {
		return "", nil
	}
	em, argumentos := listaDeIds(ids)
	linhas, err := s.db.QueryContext(ctx, `SELECT "Id", "Ticker", "Tipo", "Moeda" FROM "Ativo" WHERE "Id" IN `+em, argumentos...)
	if err != nil {
		return "", fmt.Errorf("ler os ativos: %w", err)
	}
	defer linhas.Close()
	achados, mensagem := 0, ""
	for linhas.Next() {
		var id, tipo int64
		var ticker, moeda string
		if err := linhas.Scan(&id, &ticker, &tipo, &moeda); err != nil {
			return "", fmt.Errorf("ler ativo: %w", err)
		}
		achados++
		if tipo == tipoFii {
			continue
		}
		if moeda != cambio.MoedaBase && mensagem == "" {
			mensagem = ticker + " é do exterior: não pode ser FII."
		}
		c.anterior.tipos = append(c.anterior.tipos, tipoAnterior{id, tipo})
	}
	if err := linhas.Err(); err != nil {
		return "", fmt.Errorf("ler os ativos: %w", err)
	}
	if achados != len(ids) {
		return "Algum ativo não existe mais. Confira de novo.", nil
	}
	return mensagem, nil
}

func (s *Servico) corrigirTaxas(ctx context.Context, ids []int64, c *correcao) (string, error) {
	if len(ids) == 0 {
		return "", nil
	}
	ativos, err := s.ativosPeloId(ctx)
	if err != nil {
		return "", err
	}
	a, err := s.analisarExtrato(ctx, ativos)
	if err != nil {
		return "", err
	}
	sugeridas := map[int64]taxaDaOperacao{}
	for _, t := range a.taxas {
		for _, o := range t.operacoes {
			sugeridas[o.operacao.Id] = o
		}
	}
	for _, id := range ids {
		o, ok := sugeridas[id]
		if !ok {
			return "Alguma taxa da B3 mudou desde a conferência. Confira de novo.", nil
		}
		c.anterior.taxas = append(c.anterior.taxas, taxaAnterior{id, o.operacao.Taxas, o.taxa})
	}
	return "", nil
}

// lerDebitos são a data e o valor dos lançamentos.
func (s *Servico) lerDebitos(ctx context.Context, ids []int64) (map[int64]movimento, bool, error) {
	em, argumentos := listaDeIds(ids)
	linhas, err := s.db.QueryContext(ctx, `SELECT "Id", "Data", "Valor" FROM "Transacao" WHERE "Id" IN `+em, argumentos...)
	if err != nil {
		return nil, false, fmt.Errorf("ler os débitos: %w", err)
	}
	defer linhas.Close()
	debitos := map[int64]movimento{}
	for linhas.Next() {
		var m movimento
		if err := linhas.Scan(&m.id, &m.data, &m.valor); err != nil {
			return nil, false, fmt.Errorf("ler débito: %w", err)
		}
		debitos[m.id] = m
	}
	if err := linhas.Err(); err != nil {
		return nil, false, fmt.Errorf("ler os débitos: %w", err)
	}
	var ligados int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM "OperacaoAtivo" WHERE "TransacaoId" IN `+em, argumentos...).Scan(&ligados); err != nil {
		return nil, false, fmt.Errorf("ler as operações ligadas: %w", err)
	}
	return debitos, ligados > 0, nil
}

func (s *Servico) corrigirCompras(ctx context.Context, itens []ItemDoExtrato, c *correcao) (string, error) {
	if len(itens) == 0 {
		return "", nil
	}
	ids := make([]int64, len(itens))
	for i, item := range itens {
		ids[i] = item.TransacaoId
	}
	if len(semRepetir(ids)) != len(ids) {
		return "O mesmo débito foi escolhido para duas compras.", nil
	}
	debitos, jaLigados, err := s.lerDebitos(ctx, ids)
	if err != nil {
		return "", err
	}
	if jaLigados {
		return "Algum débito já tem uma operação na carteira. Confira de novo.", nil
	}
	for _, item := range itens {
		debito, ok := debitos[item.TransacaoId]
		if !ok || !debito.valor.Decimal().IsNegative() {
			return "Escolha um débito de compra do extrato.", nil
		}
		ticker := NormalizarTicker(&item.Ticker)
		var ativo int64
		if ticker != nil {
			if ativo, _, ok, err = s.ativoDoTicker(ctx, *ticker); err != nil {
				return "", err
			}
		}
		if ticker == nil || !ok {
			return item.Ticker + " não está na carteira.", nil
		}
		if item.Tipo != operacaoCompra || !positivo(item.Quantidade) || !positivo(item.PrecoUnitario) {
			return "Informe cotas e preço da compra de " + *ticker + ".", nil
		}
		transacao, observacao := item.TransacaoId, observacaoDaCompraQueFaltou
		c.compras = append(c.compras, Operacao{AtivoId: ativo, Tipo: operacaoCompra, Data: debito.data, Quantidade: tipos.Arredondar(item.Quantidade, 6),
			PrecoUnitario: tipos.Arredondar(item.PrecoUnitario, 6), Taxas: tipos.Arredondar(tipos.Maior(tipos.Zero, item.Taxas), 2), Cambio: um,
			Observacao: &observacao, Origem: origemExtrato, TransacaoId: &transacao})
	}
	return "", nil
}

func (s *Servico) corrigirCreditos(ctx context.Context, trocos, rendimentos []int64, c *correcao) (string, error) {
	ids := append(slices.Clone(trocos), rendimentos...)
	if len(ids) == 0 {
		return "", nil
	}
	em, argumentos := listaDeIds(ids)
	lista, err := transacoes.Listar(ctx, s.db, `WHERE "Id" IN `+em, argumentos...)
	if err != nil {
		return "", err
	}
	if len(lista) != len(ids) || slices.ContainsFunc(lista, func(t *transacoes.Transacao) bool { return !positivo(t.Valor) }) {
		return "Algum crédito não existe mais. Confira de novo.", nil
	}
	rs, err := categorizacao.Carregar(ctx, s.db)
	if err != nil {
		return "", err
	}
	categorias, err := s.lerCategorias(ctx)
	if err != nil {
		return "", err
	}
	var categoriaTroco *CategoriaDoTrocoDto
	if len(trocos) > 0 {
		if categoriaTroco, err = s.categoriaDoTroco(ctx); err != nil {
			return "", err
		}
		if categoriaTroco == nil {
			return "Crie uma categoria do tipo transferência (ex.: Investimentos) para o troco das compras.", nil
		}
	}
	agora := tipos.NovaDataHora(s.relogio.Agora())
	for _, id := range ids {
		t := lista[slices.IndexFunc(lista, func(x *transacoes.Transacao) bool { return x.Id == id })]
		ehTroco := slices.Contains(trocos, id)
		var nova *int64
		if ehTroco {
			nova = &categoriaTroco.Id
		} else {
			descricao := t.Descricao
			if t.DescricaoOriginal != nil {
				descricao = *t.DescricaoOriginal
			}
			nova = categoriaDeRendimento(rs, descricao, t.PluggyCategoriaId, categorias)
		}
		if nova == nil || (t.CategoriaId != nil && *t.CategoriaId == *nova && t.CategoriaManual == ehTroco) {
			continue // as regras não dizem que é rendimento, ou já está certo
		}
		c.anterior.transacoes = append(c.anterior.transacoes, transacaoAnterior{t.Id, *nova, t.CategoriaId, t.CategoriaManual, t.IgnorarRelatorios, t.Tipo})
		t.CategoriaId, t.CategoriaManual, t.AtualizadoEm = nova, ehTroco, agora // o provento volta para a categoria automática
		rs.AplicarTipo(t)
		c.mudadas = append(c.mudadas, t)
		if ehTroco {
			c.trocos++
		} else {
			c.rendas++
		}
	}
	return "", nil
}

// gravarCorrecao grava tudo de uma vez, com a auditoria.
func (s *Servico) gravarCorrecao(ctx context.Context, r *http.Request, c *correcao, login *string) error {
	agora := tipos.NovaDataHora(s.relogio.Agora())
	evento := fmt.Sprintf("ConferenciaInvestimentos (%d tipo, %d compra, %d troco, %d rendimento, %d taxa)", len(c.anterior.tipos), len(c.compras), c.trocos, c.rendas, len(c.anterior.taxas))
	return banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		for _, t := range c.anterior.tipos {
			if _, err := tx.ExecContext(ctx, `UPDATE "Ativo" SET "Tipo" = ? WHERE "Id" = ?`, tipoFii, t.ativo); err != nil {
				return fmt.Errorf("gravar o tipo: %w", err)
			}
		}
		for _, t := range c.anterior.taxas {
			if _, err := tx.ExecContext(ctx, `UPDATE "OperacaoAtivo" SET "Taxas" = ? WHERE "Id" = ?`, t.aplicada, t.operacao); err != nil {
				return fmt.Errorf("gravar a taxa: %w", err)
			}
		}
		for _, o := range c.compras {
			res, err := tx.ExecContext(ctx, `INSERT INTO "OperacaoAtivo" ("AtivoId", "Tipo", "Data", "Quantidade", "PrecoUnitario", "Taxas", "Cambio", "Observacao",
				"Origem", "TransacaoId", "CriadoEm") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, o.AtivoId, o.Tipo, o.Data, o.Quantidade, o.PrecoUnitario, o.Taxas, o.Cambio,
				o.Observacao, o.Origem, o.TransacaoId, agora)
			if err != nil {
				return fmt.Errorf("gravar a compra: %w", err)
			}
			id, err := res.LastInsertId()
			if err != nil {
				return fmt.Errorf("ler o id da compra: %w", err)
			}
			c.anterior.operacoesCriadas = append(c.anterior.operacoesCriadas, id)
		}
		if err := transacoes.GravarTodas(ctx, tx, c.mudadas); err != nil {
			return err
		}
		return auditoria.Registrar(ctx, consultas.New(tx), r, agora, evento, login)
	})
}

// Corrigir é o CorrigirAsync: aplica as correções escolhidas numa gravação só: tipo FII, compras que faltaram (ligadas ao
// débito do extrato), troco e venda na categoria da compra (como escolha manual, para as regras não trocarem de volta) e
// provento de volta para a categoria automática. Devolve a mensagem para quem pediu, se a correção não valer.
func (s *Servico) Corrigir(ctx context.Context, r *http.Request, e EntradaDaCorrecao, login *string) (*ResultadoDaCorrecao, string, error) {
	var c correcao
	if m, err := s.corrigirTipos(ctx, semRepetir(e.AtivosParaFii), &c); m != "" || err != nil {
		return nil, m, err
	}
	if m, err := s.corrigirTaxas(ctx, semRepetir(e.Taxas), &c); m != "" || err != nil {
		return nil, m, err
	}
	if m, err := s.corrigirCompras(ctx, e.Compras, &c); m != "" || err != nil {
		return nil, m, err
	}
	trocos, rendimentos := semRepetir(e.Trocos), semRepetir(e.Rendimentos)
	if slices.ContainsFunc(trocos, func(id int64) bool { return slices.Contains(rendimentos, id) }) {
		return nil, "Um crédito não pode ser troco e rendimento ao mesmo tempo.", nil
	}
	if m, err := s.corrigirCreditos(ctx, trocos, rendimentos, &c); m != "" || err != nil {
		return nil, m, err
	}
	if len(c.anterior.tipos) == 0 && len(c.compras) == 0 && len(c.mudadas) == 0 && len(c.anterior.taxas) == 0 {
		return &ResultadoDaCorrecao{}, "", nil
	}
	if err := s.gravarCorrecao(ctx, r, &c, login); err != nil {
		return nil, "", err
	}
	chave := s.desfazer.Guardar(c.anterior)
	return &ResultadoDaCorrecao{len(c.anterior.tipos), len(c.compras), c.trocos, c.rendas, len(c.anterior.taxas), &chave}, "", nil
}

func desfazerTipos(ctx context.Context, tx *sql.Tx, lista []tipoAnterior) (int, error) {
	desfeitas := 0
	for _, t := range lista {
		res, err := tx.ExecContext(ctx, `UPDATE "Ativo" SET "Tipo" = ? WHERE "Id" = ? AND "Tipo" = ?`, t.tipo, t.ativo, tipoFii)
		if err != nil {
			return 0, fmt.Errorf("voltar o tipo: %w", err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return 0, err
		}
		desfeitas += int(n)
	}
	return desfeitas, nil
}

func desfazerTaxas(ctx context.Context, tx *sql.Tx, lista []taxaAnterior) (int, error) {
	desfeitas := 0
	for _, t := range lista {
		var atual tipos.Decimal
		err := tx.QueryRowContext(ctx, `SELECT "Taxas" FROM "OperacaoAtivo" WHERE "Id" = ?`, t.operacao).Scan(&atual)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return 0, fmt.Errorf("ler a taxa: %w", err)
		}
		if !atual.Decimal().Equal(t.aplicada.Decimal()) {
			continue
		}
		if _, err := tx.ExecContext(ctx, `UPDATE "OperacaoAtivo" SET "Taxas" = ? WHERE "Id" = ?`, t.antes, t.operacao); err != nil {
			return 0, fmt.Errorf("voltar a taxa: %w", err)
		}
		desfeitas++
	}
	return desfeitas, nil
}

func categoriasExistentes(ctx context.Context, tx *sql.Tx) (map[int64]bool, error) {
	linhas, err := tx.QueryContext(ctx, `SELECT "Id" FROM "Categoria"`)
	if err != nil {
		return nil, fmt.Errorf("ler as categorias: %w", err)
	}
	defer linhas.Close()
	ids := map[int64]bool{}
	for linhas.Next() {
		var id int64
		if err := linhas.Scan(&id); err != nil {
			return nil, fmt.Errorf("ler categoria: %w", err)
		}
		ids[id] = true
	}
	return ids, linhas.Err()
}

// voltarTransacoes devolve cada transação ao que era, se ela ainda está na categoria aplicada. A categoria que não existe
// mais fica vazia.
func (s *Servico) voltarTransacoes(ctx context.Context, tx *sql.Tx, lista []transacaoAnterior, manual func(transacaoAnterior) bool) (int, error) {
	if len(lista) == 0 {
		return 0, nil
	}
	ids := make([]int64, len(lista))
	for i, a := range lista {
		ids[i] = a.id
	}
	em, argumentos := listaDeIds(ids)
	atuais, err := transacoes.Listar(ctx, tx, `WHERE "Id" IN `+em, argumentos...)
	if err != nil {
		return 0, err
	}
	categorias, err := categoriasExistentes(ctx, tx)
	if err != nil {
		return 0, err
	}
	agora := tipos.NovaDataHora(s.relogio.Agora())
	var mudadas []*transacoes.Transacao
	for _, a := range lista {
		i := slices.IndexFunc(atuais, func(t *transacoes.Transacao) bool { return t.Id == a.id })
		if i < 0 || atuais[i].CategoriaId == nil || *atuais[i].CategoriaId != a.aplicada {
			continue
		}
		t := atuais[i]
		t.CategoriaId = a.categoria
		if a.categoria != nil && !categorias[*a.categoria] {
			t.CategoriaId = nil
		}
		t.CategoriaManual, t.Tipo, t.IgnorarRelatorios, t.AtualizadoEm = manual(a), a.tipo, a.ignorar, agora
		mudadas = append(mudadas, t)
	}
	return len(mudadas), transacoes.GravarTodas(ctx, tx, mudadas)
}

// Desfazer é o DesfazerAsync: volta o tipo, apaga as compras lançadas, volta categoria, "manual", tipo e "ignorar nos
// relatórios" dos créditos e a taxa das operações. O que mudou de novo depois disso fica como está. false se a chave não
// vale mais.
func (s *Servico) Desfazer(ctx context.Context, r *http.Request, chave string, login *string) (int, bool, error) {
	anterior, ok := s.desfazer.Tirar(chave)
	if !ok {
		return 0, false, nil
	}
	desfeitas := 0
	err := banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		n, err := desfazerTipos(ctx, tx, anterior.tipos)
		if err != nil {
			return err
		}
		desfeitas = n
		em, argumentos := listaDeIds(anterior.operacoesCriadas)
		res, err := tx.ExecContext(ctx, `DELETE FROM "OperacaoAtivo" WHERE "Id" IN `+em, argumentos...)
		if err != nil {
			return fmt.Errorf("apagar as compras: %w", err)
		}
		apagadas, err := res.RowsAffected()
		if err != nil {
			return err
		}
		desfeitas += int(apagadas)
		if n, err = desfazerTaxas(ctx, tx, anterior.taxas); err != nil {
			return err
		}
		desfeitas += n
		n, err = s.voltarTransacoes(ctx, tx, anterior.transacoes, func(a transacaoAnterior) bool { return a.manual })
		if err != nil {
			return err
		}
		desfeitas += n
		return auditoria.Registrar(ctx, consultas.New(tx), r, tipos.NovaDataHora(s.relogio.Agora()), "ConferenciaInvestimentosDesfeita ("+strconv.Itoa(desfeitas)+")", login)
	})
	return desfeitas, true, err
}

func (s *Servico) guardarTrocos(ctx context.Context, tx *sql.Tx, lista []trocoAutomatico) error {
	if len(lista) == 0 {
		return s.config.ComTx(tx).Definir(ctx, ChaveTrocosAutomaticos, nil)
	}
	dados, err := json.Marshal(lista)
	if err != nil {
		return fmt.Errorf("guardar os trocos: %w", err)
	}
	valor := string(dados)
	return s.config.ComTx(tx).Definir(ctx, ChaveTrocosAutomaticos, &valor)
}

// ClassificarTrocosSemDuvida é o ClassificarTrocosSemDuvidaAsync (#173 b, ADR 0044), que a sincronização (#83) e a partida
// (#444) chamam: o troco "sem dúvida" que ainda conta como receita vai para a categoria da compra, como escolha manual.
// Crédito com categoria escolhida à mão e valor de venda ficam de fora. O que mudou fica guardado para o Painel.
func (s *Servico) ClassificarTrocosSemDuvida(ctx context.Context) (int, error) {
	ativos, err := s.ativosPeloId(ctx)
	if err != nil {
		return 0, err
	}
	a, err := s.analisarExtrato(ctx, ativos)
	if err != nil {
		return 0, err
	}
	var ids []int64
	for _, c := range a.deOperacao {
		if c.compra != nil && a.trocosSemDuvida[c.credito.id] && c.credito.tipo != tipoTransacaoTransferencia && !c.credito.categoriaManual {
			ids = append(ids, c.credito.id)
		}
	}
	if len(ids) == 0 {
		return 0, nil
	}
	categoria, err := s.categoriaDoTroco(ctx)
	if err != nil || categoria == nil {
		return 0, err
	}
	guardados, err := s.trocosGuardados(ctx)
	if err != nil {
		return 0, err
	}
	rs, err := categorizacao.Carregar(ctx, s.db)
	if err != nil {
		return 0, err
	}
	em, argumentos := listaDeIds(ids)
	lista, err := transacoes.Listar(ctx, s.db, `WHERE "Id" IN `+em+` ORDER BY "Id"`, argumentos...)
	if err != nil {
		return 0, err
	}
	agora := tipos.NovaDataHora(s.relogio.Agora())
	for _, t := range lista {
		guardados = append(guardados, trocoAutomatico{t.Id, t.CategoriaId, t.CategoriaManual, t.Tipo, t.IgnorarRelatorios, categoria.Id})
		t.CategoriaId, t.CategoriaManual, t.AtualizadoEm = &categoria.Id, true, agora // como na conferência: as regras não trocam de volta
		rs.AplicarTipo(t)
	}
	guardados = guardados[max(0, len(guardados)-maximoTrocosGuardados):]
	err = banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		if err := transacoes.GravarTodas(ctx, tx, lista); err != nil {
			return err
		}
		if err := s.guardarTrocos(ctx, tx, guardados); err != nil {
			return err
		}
		return auditoria.RegistrarSemPedido(ctx, consultas.New(tx), agora, "TrocosAutomaticos ("+strconv.Itoa(len(ids))+")")
	})
	if err != nil {
		return 0, err
	}
	return len(ids), nil
}

// TrocosAutomaticosDto é o do C#.
type TrocosAutomaticosDto struct {
	Quantidade int           `json:"quantidade"`
	Total      tipos.Decimal `json:"total"`
	Ids        []int64       `json:"ids"`
	Desde      tipos.Data    `json:"desde"`
	Ate        tipos.Data    `json:"ate"`
}

// ResumoTrocosAutomaticos é o ResumoTrocosAutomaticosAsync: os trocos classificados que ainda estão na categoria aplicada,
// para o aviso do Painel (#80). nil se não houver.
func (s *Servico) ResumoTrocosAutomaticos(ctx context.Context) (*TrocosAutomaticosDto, error) {
	guardados, err := s.trocosGuardados(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, len(guardados))
	aplicada := map[int64]int64{}
	for i, g := range guardados {
		ids[i] = g.Id
		aplicada[g.Id] = g.Aplicada
	}
	em, argumentos := listaDeIds(ids)
	lista, err := transacoes.Listar(ctx, s.db, `WHERE "Id" IN `+em+` ORDER BY "Data"`, argumentos...)
	if err != nil {
		return nil, err
	}
	r := TrocosAutomaticosDto{Total: tipos.Zero, Ids: []int64{}}
	for _, t := range lista {
		if t.CategoriaId != nil && *t.CategoriaId == aplicada[t.Id] {
			if r.Quantidade == 0 {
				r.Desde = t.Data
			}
			r.Quantidade++
			r.Total = tipos.Somar(r.Total, t.Valor)
			r.Ids = append(r.Ids, t.Id)
			r.Ate = t.Data
		}
	}
	if r.Quantidade == 0 {
		return nil, nil
	}
	return &r, nil
}

// DesfazerTrocosAutomaticos é o DesfazerTrocosAutomaticosAsync: volta os trocos para a categoria de antes, que fica como
// escolha manual, para a próxima sincronização não classificar de novo o que você disse que não é troco.
func (s *Servico) DesfazerTrocosAutomaticos(ctx context.Context, r *http.Request, login *string) (int, error) {
	guardados, err := s.trocosGuardados(ctx)
	if err != nil {
		return 0, err
	}
	lista := make([]transacaoAnterior, len(guardados))
	for i, g := range guardados {
		lista[i] = transacaoAnterior{g.Id, g.Aplicada, g.CategoriaId, true, g.IgnorarRelatorios, g.Tipo}
	}
	desfeitos := 0
	err = banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		var erro error
		if desfeitos, erro = s.voltarTransacoes(ctx, tx, lista, func(transacaoAnterior) bool { return true }); erro != nil {
			return erro
		}
		if err := s.guardarTrocos(ctx, tx, nil); err != nil {
			return err
		}
		return auditoria.Registrar(ctx, consultas.New(tx), r, tipos.NovaDataHora(s.relogio.Agora()), "TrocosAutomaticosDesfeitos ("+strconv.Itoa(desfeitos)+")", login)
	})
	return desfeitos, err
}

// DispensarTrocosAutomaticos é o "Ok" no aviso do Painel: os trocos continuam como estão, e o aviso some.
func (s *Servico) DispensarTrocosAutomaticos(ctx context.Context) error {
	return banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error { return s.guardarTrocos(ctx, tx, nil) })
}

// Rotas.

func (s *Servico) conferir(w http.ResponseWriter, r *http.Request) {
	d, err := s.Conferir(r.Context())
	if err != nil {
		s.erroInterno(w, r, "conferir", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, d)
}

func (s *Servico) loginDoPedido(r *http.Request) (*string, error) {
	return auditoria.Login(r.Context(), consultas.New(s.db))
}

// corrigir é o POST /api/carteira/conferencia/corrigir.
func (s *Servico) corrigir(w http.ResponseWriter, r *http.Request) {
	fii := respostas.CampoLista{Propriedade: "AtivosParaFii", Item: respostas.ItemInt, Opcional: true, Maximo: 200}
	compras := respostas.CampoListaDeObjetos{Propriedade: "Compras", TipoItem: "Financas.Api.Dtos.ImportarExtratoItemDto", Maximo: 200, Novo: camposDoItemDoExtrato}
	trocos := respostas.CampoLista{Propriedade: "Trocos", Item: respostas.ItemLong, Opcional: true, Maximo: 2000}
	rendimentos := respostas.CampoLista{Propriedade: "Rendimentos", Item: respostas.ItemLong, Opcional: true, Maximo: 2000}
	taxas := respostas.CampoLista{Propriedade: "Taxas", Item: respostas.ItemInt, Opcional: true, Maximo: 2000}
	if !respostas.LerObjeto(w, r, "dto", "Financas.Api.Dtos.CorrigirInvestimentosDto", &fii, &compras, &trocos, &rendimentos, &taxas) {
		return
	}
	itens, ok := s.itensDaLista(w, r, "corrigir", compras.Itens)
	if !ok {
		return
	}
	login, err := s.loginDoPedido(r)
	if err != nil {
		s.erroInterno(w, r, "corrigir", err)
		return
	}
	resultado, mensagem, err := s.Corrigir(r.Context(), r, EntradaDaCorrecao{AtivosParaFii: fii.Valor, Compras: itens, Trocos: trocos.Valor, Rendimentos: rendimentos.Valor, Taxas: taxas.Valor}, login)
	switch {
	case err != nil:
		s.erroInterno(w, r, "corrigir", err)
	case mensagem != "":
		erroNaEntrada(w, mensagem)
	default:
		respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, resultado)
	}
}

// desfazerConferencia é o POST /api/carteira/conferencia/desfazer/{chave}.
func (s *Servico) desfazerConferencia(w http.ResponseWriter, r *http.Request) {
	login, err := s.loginDoPedido(r)
	if err != nil {
		s.erroInterno(w, r, "desfazer", err)
		return
	}
	desfeitas, ok, err := s.Desfazer(r.Context(), r, r.PathValue("chave"), login)
	switch {
	case err != nil:
		s.erroInterno(w, r, "desfazer", err)
	case !ok:
		respostas.Mensagem(w, http.StatusNotFound, "Não dá mais para desfazer: o prazo passou ou já foi desfeito.")
	default:
		respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, resultadoDaAplicacao{desfeitas})
	}
}

// desfazerTrocos é o POST /api/carteira/conferencia/trocos-automaticos/desfazer.
func (s *Servico) desfazerTrocos(w http.ResponseWriter, r *http.Request) {
	login, err := s.loginDoPedido(r)
	if err != nil {
		s.erroInterno(w, r, "desfazer os trocos", err)
		return
	}
	desfeitos, err := s.DesfazerTrocosAutomaticos(r.Context(), r, login)
	if err != nil {
		s.erroInterno(w, r, "desfazer os trocos", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, resultadoDaAplicacao{desfeitos})
}

// dispensarTrocos é o POST /api/carteira/conferencia/trocos-automaticos/dispensar.
func (s *Servico) dispensarTrocos(w http.ResponseWriter, r *http.Request) {
	if err := s.DispensarTrocosAutomaticos(r.Context()); err != nil {
		s.erroInterno(w, r, "dispensar os trocos", err)
		return
	}
	respostas.SemCorpo(w, http.StatusNoContent)
}
