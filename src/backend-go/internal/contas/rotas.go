package contas

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/bancos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/faturas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// listar: sem as arquivadas, a não ser com incluirArquivadas=true; pela ordem Arquivada, Tipo e Nome do SQLite (o Id
// desempata, a ordem da tabela).
func (s *Servico) listar(w http.ResponseWriter, r *http.Request) {
	incluir := false
	if valor, ok := respostas.ParametroDaQuery(r, "incluirArquivadas"); ok {
		if incluir, ok = respostas.BoolDaQuery(valor); !ok {
			respostas.Validacao(w, map[string][]string{"incluirArquivadas": {respostas.MensagemDaQuery(valor)}})
			return
		}
	}
	lista, err := lerContas(r.Context(), s.db, `WHERE ? OR NOT c."Arquivada" ORDER BY c."Arquivada", c."Tipo", c."Nome", c."Id"`, incluir)
	if err != nil {
		s.erroInterno(w, r, "listar as contas", err)
		return
	}
	ex, err := s.extras(r.Context())
	if err != nil {
		s.erroInterno(w, r, "saldos e faturas das contas", err)
		return
	}
	dtos := make([]ContaDto, len(lista))
	for i, c := range lista {
		dtos[i] = paraDto(c, ex, c.AtualizadoEm)
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, dtos)
}

func (s *Servico) obter(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	c, err := lerConta(r.Context(), s.db, id)
	if err != nil {
		s.erroInterno(w, r, "ler a conta", err)
		return
	}
	if c == nil {
		respostas.Mensagem(w, http.StatusNotFound, mensagemNaoEncontrado)
		return
	}
	ex, err := s.extras(r.Context())
	if err != nil {
		s.erroInterno(w, r, "saldos e faturas das contas", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, paraDto(*c, ex, c.AtualizadoEm))
}

// faturasDoMes (#323): a fatura de cada cartão que fecha no mês escolhido, o mesmo cálculo do Painel.
func (s *Servico) faturasDoMes(w http.ResponseWriter, r *http.Request) {
	var ano, mes int32
	erros := map[string][]string{}
	for _, p := range []struct {
		nome  string
		valor *int32
	}{{"ano", &ano}, {"mes", &mes}} {
		valor, ok := respostas.ParametroDaQuery(r, p.nome)
		if !ok {
			continue
		}
		if n, ok := respostas.InteiroDaQuery(valor); ok {
			*p.valor = n
		} else {
			erros[p.nome] = []string{respostas.MensagemDaQuery(valor)}
		}
	}
	if len(erros) > 0 {
		respostas.Validacao(w, erros)
		return
	}
	if ano < 2000 || ano > 2100 || mes < 1 || mes > 12 {
		respostas.Mensagem(w, http.StatusBadRequest, "Período inválido.")
		return
	}
	lista, err := faturas.DoMesDosCartoes(r.Context(), s.db, relogio.PrimeiroDiaMes(int(ano), time.Month(mes)),
		relogio.UltimoDiaMes(int(ano), time.Month(mes)), relogio.Hoje(s.relogio))
	if err != nil {
		s.erroInterno(w, r, "faturas do mês", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, lista)
}

type bancoDto struct {
	Codigo string `json:"codigo"`
	Nome   string `json:"nome"`
}

// bancos: o catálogo que a tela sabe identificar (ADR 0024), para o campo "Banco".
func (s *Servico) bancos(w http.ResponseWriter, _ *http.Request) {
	lista := make([]bancoDto, len(bancos.Catalogo))
	for i, b := range bancos.Catalogo {
		lista[i] = bancoDto{b.Codigo, b.Nome}
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, lista)
}

// salvarContaDto é o SalvarContaDto do C#, já lido e validado.
type salvarContaDto struct {
	Nome             string
	Tipo             int64
	Numero           *string
	SaldoInicial     tipos.Decimal
	LimiteCredito    *tipos.Decimal
	DiaFechamento    *int64
	DiaVencimento    *int64
	Cor              string
	IncluirNoSaldo   bool
	Arquivada        bool
	Banco            *string
	ContaVinculadaId *int64
}

// lerSalvarConta é o model binding do SalvarContaDto: [Required, StringLength(120)] Nome, [StringLength(60)] Numero,
// [Range(1, 31)] nos dois dias, [Required, RegularExpression("^#[0-9a-fA-F]{6}$")] Cor e [StringLength(30)] Banco.
func lerSalvarConta(w http.ResponseWriter, r *http.Request) (salvarContaDto, bool) {
	nome := respostas.CampoTexto{Propriedade: "Nome", Maximo: 120}
	tipo := respostas.CampoEnum{Propriedade: "Tipo"}
	numero := respostas.CampoTexto{Propriedade: "Numero", Maximo: 60, Opcional: true}
	saldoInicial := respostas.CampoDecimal{Propriedade: "SaldoInicial"}
	limite := respostas.CampoDecimalOpcional{Propriedade: "LimiteCredito"}
	diaFechamento := respostas.CampoByteOpcional{Propriedade: "DiaFechamento", Faixa: &respostas.Faixa{Minimo: 1, Maximo: 31}}
	diaVencimento := respostas.CampoByteOpcional{Propriedade: "DiaVencimento", Faixa: &respostas.Faixa{Minimo: 1, Maximo: 31}}
	cor := respostas.CampoTexto{Propriedade: "Cor", Maximo: math.MaxInt32, Padrao: respostas.NovoPadrao(`^#[0-9a-fA-F]{6}$`)}
	incluir := respostas.CampoBool{Propriedade: "IncluirNoSaldo"}
	arquivada := respostas.CampoBool{Propriedade: "Arquivada"}
	banco := respostas.CampoTexto{Propriedade: "Banco", Maximo: 30, Opcional: true}
	vinculada := respostas.CampoInteiroOpcional{Propriedade: "ContaVinculadaId"}
	if !respostas.LerObjeto(w, r, "dto", "Financas.Api.Dtos.SalvarContaDto", &nome, &tipo, &numero, &saldoInicial,
		&limite, &diaFechamento, &diaVencimento, &cor, &incluir, &arquivada, &banco, &vinculada) {
		return salvarContaDto{}, false
	}
	return salvarContaDto{Nome: *nome.Valor, Tipo: tipo.Ou(0), Numero: numero.Valor, SaldoInicial: saldoInicial.Valor,
		LimiteCredito: limite.Valor, DiaFechamento: diaFechamento.Valor, DiaVencimento: diaVencimento.Valor,
		Cor: *cor.Valor, IncluirNoSaldo: incluir.Valor, Arquivada: arquivada.Valor, Banco: banco.Valor,
		ContaVinculadaId: vinculada.Valor}, true
}

// validarBanco: o banco é um código do catálogo ou nenhum (detectar); a conta do cartão, só para cartão, uma conta que
// existe, não é cartão e não é a própria. Devolve a mensagem do 400, ou "".
func (s *Servico) validarBanco(ctx context.Context, d salvarContaDto, atual *conta) (string, error) {
	if d.Banco != nil && !bancos.Existe(d.Banco) {
		return "Banco inválido.", nil
	}
	if d.ContaVinculadaId == nil {
		return "", nil
	}
	tipo := d.Tipo
	if atual != nil && atual.Origem == OpenFinance {
		tipo = atual.Tipo
	}
	if tipo != CartaoCredito {
		return "Só cartão de crédito tem conta vinculada.", nil
	}
	if atual != nil && *d.ContaVinculadaId == atual.Id {
		return "O cartão não pode ser a própria conta dele.", nil
	}
	var tipoDaOutra int64
	err := s.db.QueryRowContext(ctx, `SELECT "Tipo" FROM "Conta" WHERE "Id" = ?`, *d.ContaVinculadaId).Scan(&tipoDaOutra)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return "Conta do cartão inválida.", nil
	case err != nil:
		return "", err
	case tipoDaOutra == CartaoCredito:
		return "A conta do cartão precisa ser uma conta, não outro cartão.", nil
	}
	return "", nil
}

// aplicar é o Aplicar do controller: nome, cor, incluir no saldo, arquivada, banco e conta do cartão valem para
// qualquer conta (ADR 0024); o resto só para a manual, porque o da conta do Open Finance vem da instituição.
func aplicar(c *conta, d salvarContaDto, manual bool, agora tipos.DataHora) {
	c.Nome = strings.TrimSpace(d.Nome)
	c.Cor = d.Cor
	c.IncluirNoSaldo = d.IncluirNoSaldo
	c.Arquivada = d.Arquivada
	c.Banco = d.Banco
	c.AtualizadoEm = agora
	tipoAgora := c.Tipo
	if manual {
		tipoAgora = d.Tipo
	}
	c.ContaVinculadaId = nil
	if tipoAgora == CartaoCredito {
		c.ContaVinculadaId = d.ContaVinculadaId
	}
	if !manual {
		return
	}
	c.Tipo = d.Tipo
	c.Numero = nil
	if d.Numero != nil {
		numero := strings.TrimSpace(*d.Numero)
		c.Numero = &numero
	}
	c.SaldoInicial = d.SaldoInicial
	c.LimiteCredito, c.DiaFechamento, c.DiaVencimento = nil, nil, nil
	if d.Tipo == CartaoCredito {
		c.LimiteCredito, c.DiaFechamento, c.DiaVencimento = d.LimiteCredito, d.DiaFechamento, d.DiaVencimento
	}
}

func (s *Servico) criar(w http.ResponseWriter, r *http.Request) {
	d, ok := lerSalvarConta(w, r)
	if !ok {
		return
	}
	if d.Tipo == Investimento && d.SaldoInicial.Decimal().IsNegative() {
		respostas.Mensagem(w, http.StatusBadRequest, "Saldo inicial inválido.")
		return
	}
	erro, err := s.validarBanco(r.Context(), d, nil)
	if err != nil {
		s.erroInterno(w, r, "validar o banco da conta", err)
		return
	}
	if erro != "" {
		respostas.Mensagem(w, http.StatusBadRequest, erro)
		return
	}
	agora := s.agora()
	c := conta{Origem: Manual, Moeda: "BRL"}
	aplicar(&c, d, true, agora)
	res, err := s.db.ExecContext(r.Context(), `INSERT INTO "Conta" ("ConexaoBancariaId", "PluggyAccountId", "Nome", "Tipo",
		"Origem", "Numero", "Moeda", "SaldoInicial", "Saldo", "LimiteCredito", "LimiteDisponivel", "LimiteAjustadoNoApp",
		"LimiteDiferencaAvaliada", "DiaFechamento", "DiaVencimento", "Cor", "IncluirNoSaldo", "Arquivada", "Banco",
		"ContaVinculadaId", "SaldoReservado", "ReservasInformadas", "CriadoEm", "AtualizadoEm")
		VALUES (NULL, NULL, ?, ?, ?, ?, ?, ?, ?, ?, NULL, NULL, NULL, ?, ?, ?, ?, ?, ?, ?, NULL, NULL, ?, ?)`,
		c.Nome, c.Tipo, c.Origem, c.Numero, c.Moeda, c.SaldoInicial, c.Saldo, c.LimiteCredito, c.DiaFechamento,
		c.DiaVencimento, c.Cor, c.IncluirNoSaldo, c.Arquivada, c.Banco, c.ContaVinculadaId, agora, c.AtualizadoEm)
	if err == nil {
		c.Id, err = res.LastInsertId()
	}
	if err != nil {
		s.erroInterno(w, r, "criar a conta", err)
		return
	}
	ex, err := s.extras(r.Context())
	if err != nil {
		s.erroInterno(w, r, "saldos e faturas das contas", err)
		return
	}
	// O C# manda a URL absoluta (CreatedAtAction); atrás do .NET, o Go não sabe o host, e ninguém lê esse cabeçalho.
	w.Header().Set("Location", "/api/contas/"+strconv.FormatInt(c.Id, 10))
	respostas.JSON(w, http.StatusCreated, respostas.ConteudoJSON, paraDto(c, ex, tipos.DataHoraUtc{DataHora: c.AtualizadoEm}))
}

func (s *Servico) atualizar(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	d, ok := lerSalvarConta(w, r)
	if !ok {
		return
	}
	c, err := lerConta(r.Context(), s.db, id)
	if err != nil {
		s.erroInterno(w, r, "ler a conta", err)
		return
	}
	if c == nil {
		respostas.Mensagem(w, http.StatusNotFound, mensagemNaoEncontrado)
		return
	}
	erro, err := s.validarBanco(r.Context(), d, c)
	if err != nil {
		s.erroInterno(w, r, "validar o banco da conta", err)
		return
	}
	if erro != "" {
		respostas.Mensagem(w, http.StatusBadRequest, erro)
		return
	}
	antes := *c
	aplicar(c, d, c.Origem == Manual, s.agora())
	if err := gravar(r.Context(), s.db, antes, *c); err != nil {
		s.erroInterno(w, r, "editar a conta", err)
		return
	}
	ex, err := s.extras(r.Context())
	if err != nil {
		s.erroInterno(w, r, "saldos e faturas das contas", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, paraDto(*c, ex, tipos.DataHoraUtc{DataHora: c.AtualizadoEm}))
}

// gravar manda só as colunas que mudaram, como o SaveChanges do EF Core: decimal pelo valor (0 e 0.0 são iguais),
// texto byte a byte. Os nomes das colunas são fixos, daqui; os valores vão por parâmetro.
func gravar(ctx context.Context, db *sql.DB, antes, depois conta) error {
	var colunas []string
	var valores []any
	mudou := func(coluna string, diferente bool, valor any) {
		if diferente {
			colunas = append(colunas, `"`+coluna+`" = ?`)
			valores = append(valores, valor)
		}
	}
	mudou("Nome", antes.Nome != depois.Nome, depois.Nome)
	mudou("Tipo", antes.Tipo != depois.Tipo, depois.Tipo)
	mudou("Numero", !textoIgual(antes.Numero, depois.Numero), depois.Numero)
	mudou("SaldoInicial", antes.SaldoInicial.Decimal().Cmp(depois.SaldoInicial.Decimal()) != 0, depois.SaldoInicial)
	mudou("LimiteCredito", !decimalIgual(antes.LimiteCredito, depois.LimiteCredito), depois.LimiteCredito)
	mudou("LimiteAjustadoNoApp", !boolIgual(antes.LimiteAjustadoNoApp, depois.LimiteAjustadoNoApp), depois.LimiteAjustadoNoApp)
	mudou("LimiteDiferencaAvaliada", !decimalIgual(antes.LimiteDiferencaAvaliada, depois.LimiteDiferencaAvaliada),
		depois.LimiteDiferencaAvaliada)
	mudou("DiaFechamento", !inteiroIgual(antes.DiaFechamento, depois.DiaFechamento), depois.DiaFechamento)
	mudou("DiaVencimento", !inteiroIgual(antes.DiaVencimento, depois.DiaVencimento), depois.DiaVencimento)
	mudou("Cor", antes.Cor != depois.Cor, depois.Cor)
	mudou("IncluirNoSaldo", antes.IncluirNoSaldo != depois.IncluirNoSaldo, depois.IncluirNoSaldo)
	mudou("Arquivada", antes.Arquivada != depois.Arquivada, depois.Arquivada)
	mudou("Banco", !textoIgual(antes.Banco, depois.Banco), depois.Banco)
	mudou("ContaVinculadaId", !inteiroIgual(antes.ContaVinculadaId, depois.ContaVinculadaId), depois.ContaVinculadaId)
	mudou("AtualizadoEm", antes.AtualizadoEm != depois.AtualizadoEm, depois.AtualizadoEm)
	if len(colunas) == 0 {
		return nil
	}
	_, err := db.ExecContext(ctx, `UPDATE "Conta" SET `+strings.Join(colunas, ", ")+` WHERE "Id" = ?`,
		append(valores, depois.Id)...)
	return err
}

func textoIgual(a, b *string) bool  { return (a == nil) == (b == nil) && (a == nil || *a == *b) }
func inteiroIgual(a, b *int64) bool { return (a == nil) == (b == nil) && (a == nil || *a == *b) }
func boolIgual(a, b *bool) bool     { return (a == nil) == (b == nil) && (a == nil || *a == *b) }
func decimalIgual(a, b *tipos.Decimal) bool {
	return (a == nil) == (b == nil) && (a == nil || a.Decimal().Cmp(b.Decimal()) == 0)
}

// excluir: só a conta manual, com as transações dela (FK em cascata). Antes, as transações deixam de apontar para as
// faturas da conta (a FK delas não tem cascata), e os cartões que pertenciam a ela ficam sem conta vinculada (não há
// FK, ADR 0024). Tudo numa transação do banco.
func (s *Servico) excluir(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	var origem int64
	err := s.db.QueryRowContext(r.Context(), `SELECT "Origem" FROM "Conta" WHERE "Id" = ?`, id).Scan(&origem)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		respostas.Mensagem(w, http.StatusNotFound, mensagemNaoEncontrado)
		return
	case err != nil:
		s.erroInterno(w, r, "ler a conta", err)
		return
	case origem == OpenFinance:
		respostas.Mensagem(w, http.StatusBadRequest,
			"Contas do Open Finance são gerenciadas pela conexão. Use 'Arquivar' ou remova a conexão.")
		return
	}
	err = banco.EmTransacao(r.Context(), s.db, func(tx *sql.Tx) error {
		if err := DesvincularFaturas(r.Context(), tx, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(r.Context(), `UPDATE "Conta" SET "ContaVinculadaId" = NULL WHERE "ContaVinculadaId" = ?`, id); err != nil {
			return err
		}
		_, err := tx.ExecContext(r.Context(), `DELETE FROM "Conta" WHERE "Id" = ?`, id)
		return err
	})
	if err != nil {
		s.erroInterno(w, r, "excluir a conta", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DesvincularFaturas tira das transações das contas o vínculo com as faturas, antes de excluir as contas (a FK da
// transação para a fatura não tem cascata). É o DesvincularFaturasAsync do C#, que a conexão também usa.
func DesvincularFaturas(ctx context.Context, tx *sql.Tx, contaIds ...int64) error {
	if len(contaIds) == 0 {
		return nil
	}
	ids := make([]any, len(contaIds))
	for i, id := range contaIds {
		ids[i] = id
	}
	_, err := tx.ExecContext(ctx, `UPDATE "Transacao" SET "FaturaCartaoId" = NULL WHERE "ContaId" IN (`+
		strings.TrimSuffix(strings.Repeat("?, ", len(ids)), ", ")+`) AND "FaturaCartaoId" IS NOT NULL`, ids...)
	return err
}

// faturaDto é o FaturaDto do C#.
type faturaDto struct {
	Id                   int64          `json:"id"`
	DataVencimento       tipos.Data     `json:"dataVencimento"`
	DataFechamento       *tipos.Data    `json:"dataFechamento"`
	ValorTotal           tipos.Decimal  `json:"valorTotal"`
	ValorMinimo          *tipos.Decimal `json:"valorMinimo"`
	ValorPago            *tipos.Decimal `json:"valorPago"`
	QuantidadeTransacoes int64          `json:"quantidadeTransacoes"`
	Situacao             string         `json:"situacao"`
}

// faturasDaConta: as 24 faturas importadas mais novas do cartão, com a quantidade de transações ligadas a cada uma, o
// pago (o informado ou o lançado no cartão, #251) e a situação.
func (s *Servico) faturasDaConta(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	var existe bool
	if err := s.db.QueryRowContext(r.Context(), `SELECT EXISTS (SELECT 1 FROM "Conta" WHERE "Id" = ?)`, id).Scan(&existe); err != nil {
		s.erroInterno(w, r, "ler a conta", err)
		return
	}
	if !existe {
		respostas.Mensagem(w, http.StatusNotFound, mensagemNaoEncontrado)
		return
	}
	lista, err := s.lerFaturasDaConta(r.Context(), id)
	if err != nil {
		s.erroInterno(w, r, "faturas da conta", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, lista)
}

func (s *Servico) lerFaturasDaConta(ctx context.Context, id int64) ([]faturaDto, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT f."Id", f."DataVencimento", f."DataFechamento", f."ValorTotal",
		f."ValorMinimo", f."ValorPago", (SELECT COUNT(*) FROM "Transacao" AS t WHERE t."FaturaCartaoId" = f."Id")
		FROM "FaturaCartao" AS f WHERE f."ContaId" = ? ORDER BY f."DataVencimento" DESC, f."Id" LIMIT 24`, id)
	if err != nil {
		return nil, err
	}
	lista := []faturaDto{}
	for linhas.Next() {
		var f faturaDto
		if err := linhas.Scan(&f.Id, &f.DataVencimento, &f.DataFechamento, &f.ValorTotal, &f.ValorMinimo, &f.ValorPago,
			&f.QuantidadeTransacoes); err != nil {
			linhas.Close()
			return nil, err
		}
		lista = append(lista, f)
	}
	linhas.Close()
	if err := linhas.Err(); err != nil {
		return nil, err
	}
	hoje := relogio.Hoje(s.relogio)
	desde := hoje
	vencimentos := make([]tipos.Data, len(lista))
	for i, f := range lista {
		vencimentos[i] = f.DataVencimento
		if i == 0 || f.DataVencimento.Antes(desde) {
			desde = f.DataVencimento
		}
	}
	pagamentos, err := faturas.LerPagamentos(ctx, s.db, []any{id}, desde.SomarMeses(-2))
	if err != nil {
		return nil, err
	}
	for i, f := range lista {
		pago := faturas.Pago(f.ValorPago, faturas.PagoNasTransacoes(f.DataVencimento, vencimentos, pagamentos[id]))
		lista[i].ValorPago = pago
		lista[i].Situacao = faturas.Situacao(tipos.DeDecimal(f.ValorTotal.Decimal().Abs()), pago, f.DataFechamento,
			f.DataVencimento, hoje)
	}
	return lista, nil
}

// limiteAjustado (#171): a resposta a "você reduziu o limite no app?" (null desfaz). Guarda a diferença de agora entre
// o limite usado (aprovado − disponível) e a dívida, para o "não" voltar a perguntar só quando ela mudar.
func (s *Servico) limiteAjustado(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	ajustado := respostas.CampoBoolOpcional{Propriedade: "Ajustado"}
	if !respostas.LerObjeto(w, r, "dto", "Financas.Api.Dtos.ResponderLimiteAjustadoDto", &ajustado) {
		return
	}
	c, err := lerConta(r.Context(), s.db, id)
	if err != nil {
		s.erroInterno(w, r, "ler a conta", err)
		return
	}
	if c == nil {
		respostas.Mensagem(w, http.StatusNotFound, mensagemNaoEncontrado)
		return
	}
	if c.Tipo != CartaoCredito {
		respostas.Mensagem(w, http.StatusBadRequest, "Só cartão de crédito tem limite.")
		return
	}
	antes := *c
	c.LimiteAjustadoNoApp = ajustado.Valor
	c.LimiteDiferencaAvaliada = nil
	if ajustado.Valor != nil && c.LimiteCredito != nil && c.LimiteDisponivel != nil {
		divida := tipos.Maior(tipos.Zero, tipos.DeDecimal(c.Saldo.Decimal().Neg()))
		diferenca := tipos.Subtrair(tipos.Subtrair(*c.LimiteCredito, *c.LimiteDisponivel), divida)
		c.LimiteDiferencaAvaliada = &diferenca
	}
	c.AtualizadoEm = s.agora()
	if err := gravar(r.Context(), s.db, antes, *c); err != nil {
		s.erroInterno(w, r, "responder o limite ajustado", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
