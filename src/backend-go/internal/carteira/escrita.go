package carteira

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cambio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cotacoes"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// origemManual é o OrigemOperacao.Manual do C#.
const origemManual = 1

// fonteManual é a FonteCotacao do preço informado à mão.
const fonteManual = "Manual"

// dataMinimaDaOperacao: antes disso, é erro de digitação.
var dataMinimaDaOperacao = tipos.NovaData(1995, time.January, 1)

// entradaDaOperacao é o SalvarOperacaoDto. Preço e taxas na moeda do ativo. Moeda só vale no primeiro lançamento do ativo
// (sem ela, BRL, a B3). Cambio são os reais por 1 unidade da moeda; ignorado em BRL e, sem ele, vale a PTAX da data.
type entradaDaOperacao struct {
	Ticker                    string
	TipoAtivo                 *int64
	Tipo                      int64
	Data                      tipos.Data
	Quantidade, PrecoUnitario tipos.Decimal
	Taxas                     tipos.Decimal
	Observacao, Moeda         *string
	Cambio                    *tipos.Decimal
}

// lerEntradaDaOperacao é o model binding do SalvarOperacaoDto.
func lerEntradaDaOperacao(w http.ResponseWriter, r *http.Request) (entradaDaOperacao, bool) {
	ticker := respostas.CampoTexto{Propriedade: "Ticker", Minimo: 1, Maximo: 12}
	tipoAtivo := respostas.CampoEnum{Propriedade: "TipoAtivo", Anulavel: true}
	tipo := respostas.CampoEnum{Propriedade: "Tipo"}
	data := respostas.CampoData{Propriedade: "Data"}
	quantidade := respostas.CampoDecimal{Propriedade: "Quantidade", Faixa: &respostas.Faixa{Minimo: 0.000001, Maximo: 1000000000}}
	preco := respostas.CampoDecimal{Propriedade: "PrecoUnitario", Faixa: &respostas.Faixa{Minimo: 0, Maximo: 100000000}}
	taxas := respostas.CampoDecimal{Propriedade: "Taxas", Faixa: &respostas.Faixa{Minimo: 0, Maximo: 10000000}}
	observacao := respostas.CampoTexto{Propriedade: "Observacao", Maximo: 300, Opcional: true}
	moeda := respostas.CampoTexto{Propriedade: "Moeda", Maximo: 3, Opcional: true}
	cotacao := respostas.CampoDecimalOpcional{Propriedade: "Cambio", Faixa: &respostas.Faixa{Minimo: 0.000001, Maximo: 100000}}
	if !respostas.LerObjeto(w, r, "dto", "Financas.Api.Dtos.SalvarOperacaoDto", &ticker, &tipoAtivo, &tipo, &data, &quantidade, &preco, &taxas,
		&observacao, &moeda, &cotacao) {
		return entradaDaOperacao{}, false
	}
	return entradaDaOperacao{Ticker: *ticker.Valor, TipoAtivo: tipoAtivo.Valor, Tipo: tipo.Ou(0), Data: data.Valor, Quantidade: quantidade.Valor,
		PrecoUnitario: preco.Valor, Taxas: taxas.Valor, Observacao: observacao.Valor, Moeda: moeda.Valor, Cambio: cotacao.Valor}, true
}

// tipoDeAtivoExiste é o Enum.IsDefined do TipoAtivo.
func tipoDeAtivoExiste(t int64) bool { return t >= tipoFii && t <= tipoOutro }

// validarOperacao é o Validar do controller: devolve a mensagem de erro, ou "".
func validarOperacao(e entradaDaOperacao, hoje tipos.Data) string {
	switch {
	case e.Tipo != operacaoCompra && e.Tipo != operacaoVenda:
		return "Tipo de operação inválido."
	case e.TipoAtivo != nil && !tipoDeAtivoExiste(*e.TipoAtivo):
		return "Tipo de ativo inválido."
	case e.Data.Depois(hoje):
		return "A data não pode estar no futuro."
	case e.Data.Antes(dataMinimaDaOperacao):
		return "Data inválida."
	}
	return ""
}

func emBranco(s *string) bool { return s == nil || strings.TrimFunc(*s, unicode.IsSpace) == "" }

// aparadoOuNulo é o texto aparado, ou nil se estiver em branco.
func aparadoOuNulo(s *string) *string {
	if emBranco(s) {
		return nil
	}
	t := strings.TrimFunc(*s, unicode.IsSpace)
	return &t
}

// sugerirTipo é o SugerirTipo, pelo padrão da B3: final 11 costuma ser FII, ETF ou unit; 31 a 35 e 39, BDR; 3 a 8, ações.
func sugerirTipo(ticker string) int64 {
	switch ticker[4:] { // o formato da B3 só deixa dígitos depois das 4 primeiras posições
	case "11":
		return tipoFii
	case "31", "32", "33", "34", "35", "39":
		return tipoBdr
	case "3", "4", "5", "6", "7", "8":
		return tipoAcao
	}
	return tipoOutro
}

// cambioDaOperacao é o CambioDaOperacaoAsync: 1 em reais; o informado; ou, sem ele, a PTAX da data: de venda na compra (a
// corretora vende a moeda a você) e de compra na venda. nil se não houver como obter.
func (s *Servico) cambioDaOperacao(ctx context.Context, moeda string, e entradaDaOperacao) (*tipos.Decimal, error) {
	if moeda == cambio.MoedaBase {
		return opcional(um), nil
	}
	if e.Cambio != nil && positivo(*e.Cambio) {
		return e.Cambio, nil
	}
	ptax, err := s.cambio.ObterCotacao(ctx, moeda, &e.Data)
	if err != nil || ptax == nil {
		return nil, err
	}
	if e.Tipo == operacaoCompra {
		return &ptax.Venda, nil
	}
	return &ptax.Compra, nil
}

func (s *Servico) operacoesDoAtivo(ctx context.Context, id int64) ([]Operacao, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT "Id", "AtivoId", "Tipo", "Data", "Quantidade", "PrecoUnitario", "Taxas", "Cambio",
		"Observacao", "Origem" FROM "OperacaoAtivo" WHERE "AtivoId" = ?`, id)
	if err != nil {
		return nil, fmt.Errorf("ler as operações do ativo: %w", err)
	}
	defer linhas.Close()
	var lista []Operacao
	for linhas.Next() {
		var o Operacao
		if err := linhas.Scan(&o.Id, &o.AtivoId, &o.Tipo, &o.Data, &o.Quantidade, &o.PrecoUnitario, &o.Taxas, &o.Cambio, &o.Observacao, &o.Origem); err != nil {
			return nil, fmt.Errorf("ler operação: %w", err)
		}
		lista = append(lista, o)
	}
	return lista, linhas.Err()
}

// vendaPossivel é o VendaPossivelAsync: nenhuma venda, em ordem cronológica, passa da quantidade em carteira. O C# desempata
// pelo Id, mas operações do mesmo dia e tipo só somam ou só subtraem: a ordem entre elas não muda o resultado.
func vendaPossivel(operacoes []Operacao) bool {
	ordenadas := slices.Clone(operacoes)
	slices.SortStableFunc(ordenadas, func(a, b Operacao) int {
		return cmp.Or(cmp.Compare(a.Data.NumeroDoDia(), b.Data.NumeroDoDia()), cmp.Compare(a.Tipo, b.Tipo))
	})
	quantidade := tipos.Zero
	for _, o := range ordenadas {
		if o.Tipo == operacaoCompra {
			quantidade = tipos.Somar(quantidade, o.Quantidade)
		} else {
			quantidade = tipos.Subtrair(quantidade, o.Quantidade)
		}
		if quantidade.Decimal().LessThan(quaseZero.Decimal().Neg()) {
			return false
		}
	}
	return true
}

func erroNaEntrada(w http.ResponseWriter, mensagem string) {
	respostas.Mensagem(w, http.StatusBadRequest, mensagem)
}

func mensagemDoTickerInvalido(exterior bool) string {
	if exterior {
		return "Código inválido. Use o código como no Yahoo Finance, por exemplo AAPL, VOO ou SAP.DE (ativos da B3 ficam em reais)."
	}
	return "Código inválido. Use o código da B3, por exemplo MXRF11 ou PETR4."
}

func mensagemSemCambio(moeda string) string {
	return "Informe o câmbio (R$ por 1 " + moeda + "): não há cotação do Banco Central para essa data."
}

// criacao é o que a criação de uma operação grava.
type criacao struct {
	ativoId       int64 // 0 num ativo novo
	ticker, moeda string
	tipoDoAtivo   int64
	cambio        tipos.Decimal
}

// novoAtivoPermitido confere o primeiro lançamento de um ativo no exterior: a moeda tem de ter cotação do Banco Central
// (com a lista de moedas vazia, não dá para saber, e passa), e FII e BDR são da B3.
func (s *Servico) novoAtivoPermitido(ctx context.Context, moeda string, e entradaDaOperacao) (string, error) {
	moedas, err := s.cambio.ListarMoedas(ctx)
	if err != nil {
		return "", err
	}
	if len(moedas) > 0 && !slices.ContainsFunc(moedas, func(m cambio.Moeda) bool { return m.Codigo == moeda }) {
		return "Não há cotação de câmbio do Banco Central para " + moeda + ".", nil
	}
	if e.TipoAtivo != nil && (*e.TipoAtivo == tipoFii || *e.TipoAtivo == tipoBdr) {
		return "FIIs e BDRs são da B3: lance em reais.", nil
	}
	return "", nil
}

func (s *Servico) ativoDoTicker(ctx context.Context, ticker string) (id int64, moeda string, achou bool, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT "Id", "Moeda" FROM "Ativo" WHERE "Ticker" = ?`, ticker).Scan(&id, &moeda)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", false, nil
	}
	if err != nil {
		return 0, "", false, fmt.Errorf("ler o ativo: %w", err)
	}
	return id, moeda, true, nil
}

// prepararCriacao é a parte do CriarOperacao antes de gravar: devolve a mensagem de erro para quem pediu, ou o que gravar.
func (s *Servico) prepararCriacao(ctx context.Context, e entradaDaOperacao) (*criacao, string, error) {
	moeda := cambio.MoedaBase
	if !emBranco(e.Moeda) {
		m, ok := cambio.NormalizarMoeda(*e.Moeda)
		if !ok {
			return nil, "Moeda inválida. Use o código de 3 letras (ex.: USD).", nil
		}
		moeda = m
	}
	exterior := moeda != cambio.MoedaBase
	ticker := NormalizarTickerDaMoeda(&e.Ticker, moeda)
	if ticker == nil {
		return nil, mensagemDoTickerInvalido(exterior), nil
	}
	if m := validarOperacao(e, relogio.Hoje(s.relogio)); m != "" {
		return nil, m, nil
	}
	id, moedaDoAtivo, existe, err := s.ativoDoTicker(ctx, *ticker)
	if err != nil {
		return nil, "", err
	}
	c := criacao{ativoId: id, ticker: *ticker, moeda: moedaDoAtivo}
	if !existe {
		c.moeda = moeda
		if exterior {
			if m, err := s.novoAtivoPermitido(ctx, moeda, e); err != nil || m != "" {
				return nil, m, err
			}
		}
		switch {
		case e.TipoAtivo != nil:
			c.tipoDoAtivo = *e.TipoAtivo
		case exterior:
			c.tipoDoAtivo = tipoAcao
		default:
			c.tipoDoAtivo = sugerirTipo(*ticker)
		}
	} else if moedaDoAtivo != moeda {
		return nil, fmt.Sprintf("%s já está lançado em %s. A moeda de um ativo não muda: use %s.", *ticker, moedaDoAtivo, moedaDoAtivo), nil
	}
	cotacao, err := s.cambioDaOperacao(ctx, c.moeda, e)
	if err != nil {
		return nil, "", err
	}
	if cotacao == nil {
		return nil, mensagemSemCambio(c.moeda), nil
	}
	c.cambio = *cotacao
	if e.Tipo == operacaoVenda {
		// Só a venda confere: num ativo novo, sem operações, ela já não cabe.
		operacoes, err := s.operacoesDoAtivo(ctx, c.ativoId)
		if err != nil {
			return nil, "", err
		}
		if !vendaPossivel(append(operacoes, operacaoDaEntrada(e, c.cambio))) {
			return nil, "A quantidade vendida é maior do que a quantidade em carteira nessa data.", nil
		}
	}
	return &c, "", nil
}

func operacaoDaEntrada(e entradaDaOperacao, cotacao tipos.Decimal) Operacao {
	return Operacao{Tipo: e.Tipo, Data: e.Data, Quantidade: e.Quantidade, PrecoUnitario: e.PrecoUnitario, Taxas: e.Taxas, Cambio: cotacao,
		Observacao: aparadoOuNulo(e.Observacao)}
}

// gravarCriacao grava o ativo novo, se for o caso, e a operação, de uma vez.
func (s *Servico) gravarCriacao(ctx context.Context, c *criacao, o Operacao) (int64, error) {
	agora := tipos.NovaDataHora(s.relogio.Agora())
	var idDaOperacao int64
	err := banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		if c.ativoId == 0 {
			res, err := tx.ExecContext(ctx, `INSERT INTO "Ativo" ("Ticker", "Tipo", "Moeda", "PrecoManual", "CriadoEm") VALUES (?, ?, ?, 0, ?)`,
				c.ticker, c.tipoDoAtivo, c.moeda, agora)
			if err != nil {
				return fmt.Errorf("gravar o ativo: %w", err)
			}
			if c.ativoId, err = res.LastInsertId(); err != nil {
				return fmt.Errorf("ler o id do ativo: %w", err)
			}
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO "OperacaoAtivo" ("AtivoId", "Tipo", "Data", "Quantidade", "PrecoUnitario", "Taxas", "Cambio",
			"Observacao", "Origem", "CriadoEm") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, c.ativoId, o.Tipo, o.Data, o.Quantidade, o.PrecoUnitario,
			o.Taxas, o.Cambio, o.Observacao, origemManual, agora)
		if err != nil {
			return fmt.Errorf("gravar a operação: %w", err)
		}
		idDaOperacao, err = res.LastInsertId()
		return err
	})
	return idDaOperacao, err
}

// atualizarSemFalhar pede as cotações dos ativos: a falha não impede o que acabou de ser gravado.
func (s *Servico) atualizarSemFalhar(ctx context.Context, ids ...int64) error {
	if _, err := s.cotacoes.Atualizar(ctx, ids); err != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	return nil
}

// respostaDaCriacao é o new { op.Id, ativo.Ticker, AtivoId = ativo.Id } do controller.
type respostaDaCriacao struct {
	Id      int64  `json:"id"`
	Ticker  string `json:"ticker"`
	AtivoId int64  `json:"ativoId"`
}

// criarOperacao é o POST /api/carteira/operacoes. Primeiro lançamento do ativo: busca a cotação e os proventos na hora, e
// a falha não impede o cadastro (segue com o preço médio).
func (s *Servico) criarOperacao(w http.ResponseWriter, r *http.Request) {
	e, ok := lerEntradaDaOperacao(w, r)
	if !ok {
		return
	}
	c, mensagem, err := s.prepararCriacao(r.Context(), e)
	switch {
	case err != nil:
		s.erroInterno(w, r, "preparar a operação", err)
		return
	case mensagem != "":
		erroNaEntrada(w, mensagem)
		return
	}
	novo := c.ativoId == 0
	id, err := s.gravarCriacao(r.Context(), c, operacaoDaEntrada(e, c.cambio))
	if err != nil {
		s.erroInterno(w, r, "gravar a operação", err)
		return
	}
	if novo {
		if err := s.atualizarSemFalhar(r.Context(), c.ativoId); err != nil {
			s.erroInterno(w, r, "buscar a cotação", err)
			return
		}
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, respostaDaCriacao{Id: id, Ticker: c.ticker, AtivoId: c.ativoId})
}

// idDaRota é o {id:int}: o que não é um int não casa com a rota, e a resposta é o 404 sem corpo.
func idDaRota(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimPrefix(r.PathValue("id"), "+"), 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return 0, false
	}
	return id, true
}

// operacaoGuardada é a operação com o que interessa do ativo dela.
type operacaoGuardada struct {
	ativoId       int64
	ticker, moeda string
}

func (s *Servico) lerOperacaoGuardada(ctx context.Context, id int64) (*operacaoGuardada, error) {
	var o operacaoGuardada
	err := s.db.QueryRowContext(ctx, `SELECT a."Id", a."Ticker", a."Moeda" FROM "OperacaoAtivo" AS o
		INNER JOIN "Ativo" AS a ON a."Id" = o."AtivoId" WHERE o."Id" = ?`, id).Scan(&o.ativoId, &o.ticker, &o.moeda)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ler a operação: %w", err)
	}
	return &o, nil
}

func registroNaoEncontrado(w http.ResponseWriter) {
	respostas.Mensagem(w, http.StatusNotFound, "Registro não encontrado.")
}

// atualizarOperacao é o PUT /api/carteira/operacoes/{id}: o ativo de uma operação não muda.
func (s *Servico) atualizarOperacao(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	e, ok := lerEntradaDaOperacao(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	guardada, err := s.lerOperacaoGuardada(ctx, id)
	if err != nil {
		s.erroInterno(w, r, "ler a operação", err)
		return
	}
	if guardada == nil {
		registroNaoEncontrado(w)
		return
	}
	if t := NormalizarTickerDaMoeda(&e.Ticker, guardada.moeda); t == nil || *t != guardada.ticker {
		erroNaEntrada(w, "Não é possível trocar o ativo de uma operação. Exclua e lance novamente.")
		return
	}
	if m := validarOperacao(e, relogio.Hoje(s.relogio)); m != "" {
		erroNaEntrada(w, m)
		return
	}
	cotacao, err := s.cambioDaOperacao(ctx, guardada.moeda, e)
	if err != nil {
		s.erroInterno(w, r, "câmbio da operação", err)
		return
	}
	if cotacao == nil {
		erroNaEntrada(w, mensagemSemCambio(guardada.moeda))
		return
	}
	nova := operacaoDaEntrada(e, *cotacao)
	operacoes, err := s.operacoesDoAtivo(ctx, guardada.ativoId)
	if err != nil {
		s.erroInterno(w, r, "ler as operações", err)
		return
	}
	for i := range operacoes {
		if operacoes[i].Id == id {
			nova.Id, nova.AtivoId, nova.Origem = id, operacoes[i].AtivoId, operacoes[i].Origem
			operacoes[i] = nova
		}
	}
	if !vendaPossivel(operacoes) {
		erroNaEntrada(w, "Com essa alteração, alguma venda ficaria maior do que a quantidade em carteira.")
		return
	}
	_, err = s.db.ExecContext(ctx, `UPDATE "OperacaoAtivo" SET "Tipo" = ?, "Data" = ?, "Quantidade" = ?, "PrecoUnitario" = ?, "Taxas" = ?,
		"Cambio" = ?, "Observacao" = ? WHERE "Id" = ?`, nova.Tipo, nova.Data, nova.Quantidade, nova.PrecoUnitario, nova.Taxas, nova.Cambio, nova.Observacao, id)
	if err != nil {
		s.erroInterno(w, r, "gravar a operação", err)
		return
	}
	respostas.SemCorpo(w, http.StatusNoContent)
}

// excluirOperacao é o DELETE /api/carteira/operacoes/{id}.
func (s *Servico) excluirOperacao(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	guardada, err := s.lerOperacaoGuardada(ctx, id)
	if err != nil {
		s.erroInterno(w, r, "ler a operação", err)
		return
	}
	if guardada == nil {
		registroNaoEncontrado(w)
		return
	}
	operacoes, err := s.operacoesDoAtivo(ctx, guardada.ativoId)
	if err != nil {
		s.erroInterno(w, r, "ler as operações", err)
		return
	}
	operacoes = slices.DeleteFunc(operacoes, func(o Operacao) bool { return o.Id == id })
	if !vendaPossivel(operacoes) {
		erroNaEntrada(w, "Sem essa compra, uma venda posterior ficaria maior do que a quantidade em carteira.")
		return
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM "OperacaoAtivo" WHERE "Id" = ?`, id); err != nil {
		s.erroInterno(w, r, "apagar a operação", err)
		return
	}
	respostas.SemCorpo(w, http.StatusNoContent)
}

// atualizarAtivo é o PUT /api/carteira/ativos/{id}. Ao tirar o preço manual, busca a cotação de novo, e a falha mantém o
// último preço.
func (s *Servico) atualizarAtivo(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	nome := respostas.CampoTexto{Propriedade: "Nome", Maximo: 150, Opcional: true}
	tipo := respostas.CampoEnum{Propriedade: "Tipo"}
	segmentoLido := respostas.CampoTexto{Propriedade: "Segmento", Maximo: 40, Opcional: true}
	manual := respostas.CampoBool{Propriedade: "PrecoManual"}
	preco := respostas.CampoDecimalOpcional{Propriedade: "PrecoAtual", Faixa: &respostas.Faixa{Minimo: 0, Maximo: 100000000}}
	if !respostas.LerObjeto(w, r, "dto", "Financas.Api.Dtos.AtualizarAtivoDto", &nome, &tipo, &segmentoLido, &manual, &preco) {
		return
	}
	ctx := r.Context()
	var moeda string
	var precoGuardado *tipos.Decimal
	var fonte *string
	err := s.db.QueryRowContext(ctx, `SELECT "Moeda", "PrecoAtual", "FonteCotacao" FROM "Ativo" WHERE "Id" = ?`, id).Scan(&moeda, &precoGuardado, &fonte)
	if errors.Is(err, sql.ErrNoRows) {
		registroNaoEncontrado(w)
		return
	}
	if err != nil {
		s.erroInterno(w, r, "ler o ativo", err)
		return
	}
	novoTipo := tipo.Ou(0)
	segmento := aparadoOuNulo(segmentoLido.Valor)
	switch {
	case !tipoDeAtivoExiste(novoTipo):
		erroNaEntrada(w, "Tipo de ativo inválido.")
		return
	case moeda != cambio.MoedaBase && (novoTipo == tipoFii || novoTipo == tipoBdr):
		erroNaEntrada(w, "FIIs e BDRs são da B3: um ativo no exterior não pode ter esse tipo.")
		return
	case manual.Valor && (preco.Valor == nil || !positivo(*preco.Valor)):
		erroNaEntrada(w, "Informe o preço atual para usar preço manual.")
		return
	case !SegmentoValido(segmento):
		erroNaEntrada(w, "Segmento inválido.")
		return
	}
	if manual.Valor {
		_, err = s.db.ExecContext(ctx, `UPDATE "Ativo" SET "Nome" = ?, "Tipo" = ?, "Segmento" = ?, "PrecoManual" = 1, "PrecoAnterior" = ?, "PrecoAtual" = ?,
			"PrecoAtualizadoEm" = ?, "FonteCotacao" = ? WHERE "Id" = ?`, aparadoOuNulo(nome.Valor), novoTipo, segmento, precoGuardado, preco.Valor,
			tipos.NovaDataHora(s.relogio.Agora()), fonteManual, id)
	} else {
		_, err = s.db.ExecContext(ctx, `UPDATE "Ativo" SET "Nome" = ?, "Tipo" = ?, "Segmento" = ?, "PrecoManual" = 0 WHERE "Id" = ?`,
			aparadoOuNulo(nome.Valor), novoTipo, segmento, id)
	}
	if err != nil {
		s.erroInterno(w, r, "gravar o ativo", err)
		return
	}
	if !manual.Valor && fonte != nil && *fonte == fonteManual {
		if err := s.atualizarSemFalhar(ctx, id); err != nil {
			s.erroInterno(w, r, "buscar a cotação", err)
			return
		}
	}
	respostas.SemCorpo(w, http.StatusNoContent)
}

// excluirAtivo é o DELETE /api/carteira/ativos/{id}: as operações e os proventos saem junto, pelas chaves estrangeiras.
func (s *Servico) excluirAtivo(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	res, err := s.db.ExecContext(r.Context(), `DELETE FROM "Ativo" WHERE "Id" = ?`, id)
	if err != nil {
		s.erroInterno(w, r, "apagar o ativo", err)
		return
	}
	n, err := res.RowsAffected()
	switch {
	case err != nil:
		s.erroInterno(w, r, "apagar o ativo", err)
	case n == 0:
		registroNaoEncontrado(w)
	default:
		respostas.SemCorpo(w, http.StatusNoContent)
	}
}

// configuracaoDaCotacao é o ConfiguracaoCotacaoDto.
type configuracaoDaCotacao struct {
	TokenBrapiConfigurado bool `json:"tokenBrapiConfigurado"`
}

// obterConfiguracao é o GET /api/carteira/configuracao: só diz se há token, nunca o valor.
func (s *Servico) obterConfiguracao(w http.ResponseWriter, r *http.Request) {
	existe, err := s.config.Existe(r.Context(), cotacoes.ChaveTokenBrapi)
	if err != nil {
		s.erroInterno(w, r, "ler a configuração", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, configuracaoDaCotacao{existe})
}

// tokenValido: sem espaço nem caractere de controle, que iriam parar no cabeçalho da consulta.
func tokenValido(token string) bool {
	return !strings.ContainsFunc(token, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) })
}

// salvarConfiguracao é o PUT /api/carteira/configuracao: grava ou tira o token da brapi; em branco, nada muda.
func (s *Servico) salvarConfiguracao(w http.ResponseWriter, r *http.Request) {
	token := respostas.CampoTexto{Propriedade: "TokenBrapi", Maximo: 200, Opcional: true}
	remover := respostas.CampoBool{Propriedade: "Remover"}
	if !respostas.LerObjeto(w, r, "dto", "Financas.Api.Dtos.SalvarConfiguracaoCotacaoDto", &token, &remover) {
		return
	}
	var err error
	switch aparado := aparadoOuNulo(token.Valor); {
	case remover.Valor:
		err = s.config.Definir(r.Context(), cotacoes.ChaveTokenBrapi, nil)
	case aparado == nil:
	case !tokenValido(*aparado):
		erroNaEntrada(w, "Token inválido.")
		return
	default:
		err = s.config.DefinirSegredo(r.Context(), s.cofre, cotacoes.ChaveTokenBrapi, aparado)
	}
	if err != nil {
		s.erroInterno(w, r, "gravar a configuração", err)
		return
	}
	respostas.SemCorpo(w, http.StatusNoContent)
}
