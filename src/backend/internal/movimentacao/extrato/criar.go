package extrato

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/ajustes/categorizacao"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/movimentacao/transacoes"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/movimentacao/transferencias"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/patrimonio/reservas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// salvarTransacaoDto é o SalvarTransacaoDto do C#, já lido e validado.
type salvarTransacaoDto struct {
	ContaId           int64
	Data              tipos.Data
	Descricao         string
	Valor             tipos.Decimal
	Tipo              int64
	CategoriaId       *int64
	Observacao        *string
	IgnorarRelatorios bool
	Status            int64
	Emergencia        bool
	ReservaNome       *string // #758: o cofrinho de onde saiu o gasto, só em conta do Open Finance
}

// tamanhoMaximoReserva é o tamanho do nome do cofrinho, o mesmo da meta e da conta a pagar.
const tamanhoMaximoReserva = 100

// lerSalvarTransacao é o model binding do SalvarTransacaoDto: [Required, StringLength(300)] Descricao,
// [Range(0.01, 999999999)] Valor e [StringLength(1000)] Observacao; Status vale Efetivada (1) e Emergencia false se
// não vierem (os padrões do record). ReservaNome (#758) é opcional, com até 100 caracteres.
func lerSalvarTransacao(w http.ResponseWriter, r *http.Request) (salvarTransacaoDto, bool) {
	conta := respostas.CampoInteiro{Propriedade: "ContaId"}
	data := respostas.CampoData{Propriedade: "Data", Obrigatoria: true} // #503
	descricao := respostas.CampoTexto{Propriedade: "Descricao", Maximo: 300}
	valor := respostas.CampoDecimal{Propriedade: "Valor", Faixa: &respostas.Faixa{Minimo: 0.01, Maximo: 999999999}}
	tipo := respostas.CampoEnum{Propriedade: "Tipo"}
	categoria := respostas.CampoInteiroOpcional{Propriedade: "CategoriaId"}
	observacao := respostas.CampoTexto{Propriedade: "Observacao", Maximo: 1000, Opcional: true}
	ignorar := respostas.CampoBool{Propriedade: "IgnorarRelatorios"}
	status := respostas.CampoEnum{Propriedade: "Status"}
	emergencia := respostas.CampoBool{Propriedade: "Emergencia"}
	reserva := respostas.CampoTexto{Propriedade: "ReservaNome", Maximo: tamanhoMaximoReserva, Opcional: true}
	if !respostas.LerObjeto(w, r, "dto", "Financas.Api.Dtos.SalvarTransacaoDto", &conta, &data, &descricao, &valor, &tipo,
		&categoria, &observacao, &ignorar, &status, &emergencia, &reserva) {
		return salvarTransacaoDto{}, false
	}
	return salvarTransacaoDto{ContaId: conta.Valor, Data: data.Valor, Descricao: *descricao.Valor, Valor: valor.Valor,
		Tipo: tipo.Ou(0), CategoriaId: categoria.Valor, Observacao: observacao.Valor, IgnorarRelatorios: ignorar.Valor,
		Status: status.Ou(1), Emergencia: emergencia.Valor, ReservaNome: aparadoOuNulo(reserva.Valor)}, true
}

// origemManual é a OrigemConta das contas manuais.
const origemManual = 1

// validarManual é o ValidarManualAsync do C#: nunca transferência (tem a opção própria), e a categoria, se veio, tem
// de existir. Em conta manual, sem cofrinho. Em conta do Open Finance, só o gasto pago direto de um cofrinho (#758,
// ADR 0063): despesa, com o cofrinho, que não pode ser o Méli Dólar (em dólar). Devolve a mensagem do 400, ou "".
func (s *Servico) validarManual(ctx context.Context, d salvarTransacaoDto) (string, error) {
	if d.Tipo == transacoes.Transferencia {
		return "Para transferências use a opção 'Transferência'.", nil
	}
	var origem int64
	err := s.db.QueryRowContext(ctx, `SELECT "Origem" FROM "Conta" WHERE "Id" = ?`, d.ContaId).Scan(&origem)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return "Conta inválida.", nil
	case err != nil:
		return "", err
	case origem == origemManual && d.ReservaNome != nil:
		return "O cofrinho só vale em conta do Open Finance: em conta manual, o gasto sai do saldo da conta.", nil
	case origem != origemManual && d.ReservaNome == nil:
		return "Não é possível lançar manualmente em contas do Open Finance. Os lançamentos chegam pela sincronização.", nil
	case origem != origemManual && d.Tipo == transacoes.Receita:
		return "Do cofrinho só se lança despesa.", nil
	case origem != origemManual && reservas.EhNomeMeliDolar(*d.ReservaNome):
		return reservas.ErrSaidaMeliDolar.Error(), nil
	}
	if d.CategoriaId != nil {
		existe, err := s.categoriaExiste(ctx, *d.CategoriaId)
		if err != nil {
			return "", err
		}
		if !existe {
			return "Categoria inválida.", nil
		}
	}
	return "", nil
}

// aplicar é o Aplicar do controller: a descrição aparada, o valor com o sinal pelo tipo (receita positiva, o resto
// negativo), o status, a observação aparada (em branco é nula), a categoria escolhida (manual) ou a das regras, e o tipo
// pela categoria e pelo sinal (ADR 0036). O tipo do corpo só decide o sinal. Ignorar nos relatórios vale fora das
// transferências; emergência, só para saída.
func aplicar(t *transacoes.Transacao, d salvarTransacaoDto, rs *categorizacao.Regras, agora tipos.DataHora) {
	t.Data = d.Data
	t.Descricao = strings.TrimSpace(d.Descricao)
	t.DescricaoOriginal = nil
	abs := tipos.DeDecimal(d.Valor.Decimal().Abs())
	t.Valor = abs
	if d.Tipo != transacoes.Receita {
		t.Valor = tipos.DeDecimal(abs.Decimal().Neg())
	}
	t.Status = d.Status
	t.Observacao = aparadoOuNulo(d.Observacao)
	t.CategoriaManual = d.CategoriaId != nil
	t.CategoriaId = d.CategoriaId
	if t.CategoriaId == nil {
		t.CategoriaId = rs.ResolverTexto(t.Descricao, nil)
	}
	rs.AplicarTipo(t)
	if t.Tipo != transacoes.Transferencia {
		t.IgnorarRelatorios = d.IgnorarRelatorios
	}
	t.Emergencia = d.Emergencia && t.Valor.Decimal().IsNegative()
	t.ReservaNome = d.ReservaNome
	t.AtualizadoEm = agora
}

// aparadoOuNulo é o "string.IsNullOrWhiteSpace(x) ? null : x.Trim()" do C#.
func aparadoOuNulo(s *string) *string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	aparado := strings.TrimSpace(*s)
	return &aparado
}

// criar é o lançamento manual (em conta manual, ou o gasto pago por um cofrinho numa do Open Finance, #758). Depois de gravar, as transferências entre contas suas são
// conferidas de novo (o AcertarTransferenciasAsync, ADR 0031): um lançamento novo pode formar par com outro. As duas
// coisas vão numa transação do banco só (no C#, dois SaveChanges).
func (s *Servico) criar(w http.ResponseWriter, r *http.Request) {
	d, ok := lerSalvarTransacao(w, r)
	if !ok {
		return
	}
	mensagem, err := s.validarManual(r.Context(), d)
	if err != nil {
		s.erroInterno(w, r, "validar o lançamento", err)
		return
	}
	if mensagem != "" {
		respostas.Mensagem(w, http.StatusBadRequest, mensagem)
		return
	}
	var id int64
	err = banco.EmTransacao(r.Context(), s.db, func(tx *sql.Tx) error {
		rs, err := categorizacao.Carregar(r.Context(), tx)
		if err != nil {
			return err
		}
		agora := tipos.NovaDataHora(s.relogio.Agora())
		t := transacoes.Transacao{ContaId: d.ContaId}
		aplicar(&t, d, rs, agora)
		res, err := tx.ExecContext(r.Context(), `INSERT INTO "Transacao" ("ContaId", "PluggyTransactionId", "Data",
			"Descricao", "DescricaoOriginal", "Estabelecimento", "Valor", "Tipo", "Status", "CategoriaId", "CategoriaManual",
			"PluggyCategoriaId", "FaturaCartaoId", "ParcelaNumero", "ParcelaTotal", "IgnorarRelatorios", "Emergencia",
			"DescricaoRevisada", "MeioPagamento", "MesmaTitularidade", "RegraDescricaoId", "DescricaoManual",
			"TransferenciaGrupo", "Observacao", "ReservaNome", "CriadoEm", "AtualizadoEm")
			VALUES (?, NULL, ?, ?, NULL, NULL, ?, ?, ?, ?, ?, NULL, NULL, NULL, NULL, ?, ?, 0, NULL, NULL, NULL, 0, NULL, ?, ?, ?, ?)`,
			t.ContaId, t.Data, t.Descricao, t.Valor, t.Tipo, t.Status, t.CategoriaId, t.CategoriaManual, t.IgnorarRelatorios,
			t.Emergencia, t.Observacao, t.ReservaNome, agora, t.AtualizadoEm)
		if err != nil {
			return err
		}
		if id, err = res.LastInsertId(); err != nil {
			return err
		}
		_, err = transferencias.Reclassificar(r.Context(), tx, agora)
		return err
	})
	if err != nil {
		s.erroInterno(w, r, "criar o lançamento", err)
		return
	}
	lista, err := lerDtos(r.Context(), s.db, `WHERE t."Id" = ?`, id)
	if err != nil || len(lista) == 0 {
		s.erroInterno(w, r, "ler o lançamento criado", errors.Join(err, errors.New("lançamento criado não encontrado")))
		return
	}
	// O C# manda a URL absoluta (CreatedAtAction); atrás do .NET, o Go não sabe o host, e ninguém lê esse cabeçalho.
	w.Header().Set("Location", "/api/transacoes/"+strconv.FormatInt(id, 10))
	respostas.JSON(w, http.StatusCreated, respostas.ConteudoJSON, lista[0])
}
