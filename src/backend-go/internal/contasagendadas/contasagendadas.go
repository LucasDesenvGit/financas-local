// Package contasagendadas são as contas a pagar (#79): o ContasAgendadasController do C# (as 9 rotas de
// /api/contas-agendadas), os avisos das recorrentes (#158, ADR 0043) e as sugestões do extrato (#62, #205, #241). A
// conciliação com o extrato fica no internal/cobrancas, que a partida (#444) e a sincronização (#83) também chamam.
package contasagendadas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"unicode"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/categorizacao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cobrancas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relatorios"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/servidor"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
)

// origemManual é a OrigemConta "manual": só nela o "Pagar" lança a despesa (no Open Finance, ela chega sozinha).
const origemManual = 1

// statusEfetivada é o StatusTransacao.Efetivada.
const statusEfetivada = 1

// Servico são as contas a pagar.
type Servico struct {
	db         *sql.DB
	relogio    relogio.Relogio
	log        *slog.Logger
	relatorios *relatorios.Servico
}

// NovoServico monta o serviço.
func NovoServico(db *sql.DB, r relogio.Relogio, log *slog.Logger) *Servico {
	return &Servico{db: db, relogio: r, log: log, relatorios: relatorios.NovoServico(db, r)}
}

// Registrar põe as rotas de /api/contas-agendadas no mux.
func Registrar(mux *http.ServeMux, s *Servico) {
	mux.HandleFunc("GET /api/contas-agendadas", s.listar)
	mux.HandleFunc("GET /api/contas-agendadas/avisos", s.avisos)
	mux.HandleFunc("GET /api/contas-agendadas/sugestoes", s.sugestoes)
	mux.HandleFunc("POST /api/contas-agendadas", s.criar)
	mux.HandleFunc("PUT /api/contas-agendadas/{id}", s.atualizar)
	mux.HandleFunc("POST /api/contas-agendadas/{id}/ligar-extrato", s.ligarExtrato)
	mux.HandleFunc("DELETE /api/contas-agendadas/{id}/ligar-extrato", s.desligarExtrato)
	mux.HandleFunc("DELETE /api/contas-agendadas/{id}", s.excluir)
	mux.HandleFunc("POST /api/contas-agendadas/{id}/pagar", s.pagar)
}

func (s *Servico) hoje() tipos.Data { return relogio.Hoje(s.relogio) }

func (s *Servico) agora() tipos.DataHora { return tipos.NovaDataHora(s.relogio.Agora()) }

// idDaRota é o {id:int} do C#: o que não é número de 32 bits não casa com a rota, e a resposta é 404 sem corpo.
func idDaRota(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimPrefix(r.PathValue("id"), "+"), 10, 32)
	if err != nil {
		respostas.SemCorpo(w, http.StatusNotFound)
		return 0, false
	}
	return id, true
}

// erroInterno é o 500 do .NET, com o erro no log; requisição cancelada não responde nem registra.
func (s *Servico) erroInterno(w http.ResponseWriter, r *http.Request, etapa string, err error) {
	if errors.Is(err, context.Canceled) || r.Context().Err() != nil {
		return
	}
	s.log.ErrorContext(r.Context(), "erro nas contas a pagar", "origem", "API", "etapa", etapa, "erro", err)
	servidor.ResponderErroInterno(w)
}

func naoEncontrado(w http.ResponseWriter) {
	respostas.Mensagem(w, http.StatusNotFound, "Registro não encontrado.")
}

func (s *Servico) listar(w http.ResponseWriter, r *http.Request) {
	lista, err := s.relatorios.ContasAgendadas(r.Context(), nil)
	if err != nil {
		s.erroInterno(w, r, "listar", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, lista)
}

func (s *Servico) avisos(w http.ResponseWriter, r *http.Request) {
	lista, err := s.Avisos(r.Context(), s.hoje())
	if err != nil {
		s.erroInterno(w, r, "avisos", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, lista)
}

func (s *Servico) sugestoes(w http.ResponseWriter, r *http.Request) {
	lista, err := s.Sugerir(r.Context(), s.hoje())
	if err != nil {
		s.erroInterno(w, r, "sugestões", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, lista)
}

// salvarDto é o SalvarContaAgendadaDto.
type salvarDto struct {
	Descricao         string
	Valor             tipos.Decimal
	ProximoVencimento tipos.Data
	Recorrencia       int64
	CategoriaId       *int64
	ContaId           *int64
	Ativa             bool
	DataLimite        *tipos.Data
	FormaCobranca     int64
	TextoCobranca     *string
	Observacao        *string
	ChaveExtrato      *string
}

// lerSalvar lê o corpo como o model binding do record: Ativa ausente vale true e a forma de cobrança, "à mão" (os
// padrões do construtor), e o texto da cobrança e a observação têm a mensagem própria do tamanho.
func lerSalvar(w http.ResponseWriter, r *http.Request) (salvarDto, bool) {
	descricao := respostas.CampoTexto{Propriedade: "Descricao", Maximo: 150}
	valor := respostas.CampoDecimal{Propriedade: "Valor", Faixa: &respostas.Faixa{Minimo: 0.01, Maximo: 999999999}}
	vencimento := respostas.CampoData{Propriedade: "ProximoVencimento"}
	recorrencia := respostas.CampoEnum{Propriedade: "Recorrencia"}
	categoria := respostas.CampoInteiroOpcional{Propriedade: "CategoriaId"}
	conta := respostas.CampoInteiroOpcional{Propriedade: "ContaId"}
	ativa := respostas.CampoBool{Propriedade: "Ativa", Valor: true}
	limite := respostas.CampoDataOpcional{Propriedade: "DataLimite"}
	forma := respostas.CampoEnum{Propriedade: "FormaCobranca"}
	textoCobranca := respostas.CampoTexto{Propriedade: "TextoCobranca", Maximo: 100, Opcional: true,
		MensagemDoTamanho: "O texto da cobrança pode ter até 100 caracteres."}
	observacao := respostas.CampoTexto{Propriedade: "Observacao", Maximo: 300, Opcional: true,
		MensagemDoTamanho: "A observação pode ter até 300 caracteres."}
	chave := respostas.CampoTexto{Propriedade: "ChaveExtrato", Maximo: 150, Opcional: true}
	if !respostas.LerObjeto(w, r, "dto", "Financas.Api.Dtos.SalvarContaAgendadaDto", &descricao, &valor, &vencimento,
		&recorrencia, &categoria, &conta, &ativa, &limite, &forma, &textoCobranca, &observacao, &chave) {
		return salvarDto{}, false
	}
	return salvarDto{Descricao: *descricao.Valor, Valor: valor.Valor, ProximoVencimento: vencimento.Valor,
		Recorrencia: recorrencia.Ou(0), CategoriaId: categoria.Valor, ContaId: conta.Valor, Ativa: ativa.Valor,
		DataLimite: limite.Valor, FormaCobranca: forma.Ou(cobrancas.FormaAMao), TextoCobranca: textoCobranca.Valor,
		Observacao: observacao.Valor, ChaveExtrato: chave.Valor}, true
}

// validar é o ValidarAsync: a categoria e a conta existem, e a conta ativa e recorrente tem a data limite depois do
// próximo vencimento (a encerrada pode ser editada sem mexer nas datas). Devolve a mensagem do 400, ou "".
func (s *Servico) validar(ctx context.Context, d salvarDto) (string, error) {
	if d.CategoriaId != nil {
		if ok, err := s.existe(ctx, `SELECT 1 FROM "Categoria" WHERE "Id" = ?`, *d.CategoriaId); err != nil || !ok {
			return "Categoria inválida.", err
		}
	}
	if d.ContaId != nil {
		if ok, err := s.existe(ctx, `SELECT 1 FROM "Conta" WHERE "Id" = ?`, *d.ContaId); err != nil || !ok {
			return "Conta inválida.", err
		}
	}
	if d.Ativa && d.Recorrencia != relatorios.RecorrenciaUnica && d.DataLimite != nil && d.DataLimite.Antes(d.ProximoVencimento) {
		return fmt.Sprintf("A data limite (até %s) não pode ser antes do próximo vencimento (%s).",
			diaMesAno(*d.DataLimite), diaMesAno(d.ProximoVencimento)), nil
	}
	return "", nil
}

func (s *Servico) existe(ctx context.Context, consulta string, id int64) (bool, error) {
	var um int
	err := s.db.QueryRowContext(ctx, consulta, id).Scan(&um)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func diaMesAno(d tipos.Data) string { return fmt.Sprintf("%02d/%02d/%04d", d.Dia(), d.Mes(), d.Ano()) }

func diaMes(d tipos.Data) string { return fmt.Sprintf("%02d/%02d", d.Dia(), d.Mes()) }

// conta é a linha de ContaAgendada que as rotas gravam.
type conta struct {
	Descricao         string
	Valor             tipos.Decimal
	ProximoVencimento tipos.Data
	Recorrencia       int64
	CategoriaId       *int64
	ContaId           *int64
	Ativa             bool
	DataLimite        *tipos.Data
	FormaCobranca     int64
	TextoCobranca     *string
	Observacao        *string
	ChaveExtrato      *string
}

// aplicar é o Aplicar: descrição e observação aparadas (vazia vira null), sem data limite na conta única, forma de
// cobrança fora do enum vira "à mão" (#158), o texto da cobrança sem acento e em maiúsculas (vazio vira null) e a
// chave do extrato só quando vem (#205: na edição ela vem vazia, e a ligação que já existe fica).
func aplicar(c *conta, d salvarDto) {
	c.Descricao = strings.TrimFunc(d.Descricao, unicode.IsSpace)
	c.Valor = d.Valor
	c.ProximoVencimento = d.ProximoVencimento
	c.Recorrencia = d.Recorrencia
	c.CategoriaId = d.CategoriaId
	c.ContaId = d.ContaId
	c.Ativa = d.Ativa
	c.DataLimite = d.DataLimite
	if d.Recorrencia == relatorios.RecorrenciaUnica {
		c.DataLimite = nil
	}
	c.FormaCobranca = cobrancas.FormaAMao
	if d.FormaCobranca >= 1 && d.FormaCobranca <= 3 {
		c.FormaCobranca = d.FormaCobranca
	}
	c.TextoCobranca = nil
	if d.TextoCobranca != nil {
		if t := texto.ParaRegras(*d.TextoCobranca); t != "" {
			c.TextoCobranca = &t
		}
	}
	c.Observacao = aparado(d.Observacao)
	if chave := aparado(d.ChaveExtrato); chave != nil {
		c.ChaveExtrato = chave
	}
}

// aparado é o string.IsNullOrWhiteSpace(x) ? null : x.Trim().
func aparado(x *string) *string {
	if x == nil {
		return nil
	}
	t := strings.TrimFunc(*x, unicode.IsSpace)
	if t == "" {
		return nil
	}
	return &t
}

// criar é o POST: grava e concilia logo (a cobrança pode já estar no extrato), numa transação do banco só.
func (s *Servico) criar(w http.ResponseWriter, r *http.Request) {
	d, ok := lerSalvar(w, r)
	if !ok {
		return
	}
	s.salvar(w, r, 0, d)
}

// atualizar é o PUT: a conta a pagar que não existe é 404, antes da validação (como no C#).
func (s *Servico) atualizar(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	d, ok := lerSalvar(w, r)
	if !ok {
		return
	}
	s.salvar(w, r, id, d)
}

func (s *Servico) salvar(w http.ResponseWriter, r *http.Request, id int64, d salvarDto) {
	ctx := r.Context()
	var c conta
	if id != 0 {
		achou, err := s.ler(ctx, id, &c)
		if err != nil {
			s.erroInterno(w, r, "ler a conta a pagar", err)
			return
		}
		if !achou {
			naoEncontrado(w)
			return
		}
	}
	mensagem, err := s.validar(ctx, d)
	if err != nil {
		s.erroInterno(w, r, "validar a conta a pagar", err)
		return
	}
	if mensagem != "" {
		respostas.Mensagem(w, http.StatusBadRequest, mensagem)
		return
	}
	aplicar(&c, d)
	err = banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		if id == 0 {
			_, err := tx.ExecContext(ctx, `INSERT INTO "ContaAgendada" ("Descricao", "Valor", "ProximoVencimento",
				"Recorrencia", "CategoriaId", "ContaId", "Ativa", "UltimoPagamentoEm", "DataLimite", "FormaCobranca",
				"TextoCobranca", "Observacao", "ChaveExtrato", "CriadoEm") VALUES (?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, ?, ?, ?, ?)`,
				c.Descricao, c.Valor, c.ProximoVencimento, c.Recorrencia, c.CategoriaId, c.ContaId, c.Ativa, c.DataLimite,
				c.FormaCobranca, c.TextoCobranca, c.Observacao, c.ChaveExtrato, s.agora())
			if err != nil {
				return fmt.Errorf("criar a conta a pagar: %w", err)
			}
		} else {
			_, err := tx.ExecContext(ctx, `UPDATE "ContaAgendada" SET "Descricao" = ?, "Valor" = ?, "ProximoVencimento" = ?,
				"Recorrencia" = ?, "CategoriaId" = ?, "ContaId" = ?, "Ativa" = ?, "DataLimite" = ?, "FormaCobranca" = ?,
				"TextoCobranca" = ?, "Observacao" = ?, "ChaveExtrato" = ? WHERE "Id" = ?`,
				c.Descricao, c.Valor, c.ProximoVencimento, c.Recorrencia, c.CategoriaId, c.ContaId, c.Ativa, c.DataLimite,
				c.FormaCobranca, c.TextoCobranca, c.Observacao, c.ChaveExtrato, id)
			if err != nil {
				return fmt.Errorf("atualizar a conta a pagar %d: %w", id, err)
			}
		}
		_, err := cobrancas.Conciliar(ctx, tx, s.hoje())
		return err
	})
	if err != nil {
		s.erroInterno(w, r, "salvar a conta a pagar", err)
		return
	}
	respostas.SemCorpo(w, http.StatusNoContent)
}

// ler lê a conta a pagar id em c; false se ela não existe.
func (s *Servico) ler(ctx context.Context, id int64, c *conta) (bool, error) {
	err := s.db.QueryRowContext(ctx, `SELECT "Descricao", "Valor", "ProximoVencimento", "Recorrencia", "CategoriaId",
		"ContaId", "Ativa", "DataLimite", "FormaCobranca", "TextoCobranca", "Observacao", "ChaveExtrato"
		FROM "ContaAgendada" WHERE "Id" = ?`, id).Scan(&c.Descricao, &c.Valor, &c.ProximoVencimento, &c.Recorrencia,
		&c.CategoriaId, &c.ContaId, &c.Ativa, &c.DataLimite, &c.FormaCobranca, &c.TextoCobranca, &c.Observacao, &c.ChaveExtrato)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("ler a conta a pagar %d: %w", id, err)
	}
	return true, nil
}

// ligarExtrato é o "é a mesma" (#205): a sugestão do extrato é esta conta a pagar. Grava a chave aparada.
func (s *Servico) ligarExtrato(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	chave := respostas.CampoTexto{Propriedade: "ChaveExtrato", Maximo: 150}
	if !respostas.LerObjeto(w, r, "dto", "Financas.Api.Dtos.LigarExtratoContaAgendadaDto", &chave) {
		return
	}
	// O "Informe a sugestão do extrato." do C# não chega a valer: o [Required] já recusa a chave só com espaços.
	s.mudarChave(w, r, id, aparado(chave.Valor))
}

// desligarExtrato é o "Desfazer" do "é a mesma": a sugestão pode voltar.
func (s *Servico) desligarExtrato(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	s.mudarChave(w, r, id, nil)
}

func (s *Servico) mudarChave(w http.ResponseWriter, r *http.Request, id int64, chave *string) {
	res, err := s.db.ExecContext(r.Context(), `UPDATE "ContaAgendada" SET "ChaveExtrato" = ? WHERE "Id" = ?`, chave, id)
	if err != nil {
		s.erroInterno(w, r, "ligar ao extrato", err)
		return
	}
	s.responderAlteradas(w, r, res)
}

func (s *Servico) excluir(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	res, err := s.db.ExecContext(r.Context(), `DELETE FROM "ContaAgendada" WHERE "Id" = ?`, id)
	if err != nil {
		s.erroInterno(w, r, "excluir", err)
		return
	}
	s.responderAlteradas(w, r, res)
}

// responderAlteradas: nenhuma linha alterada é 404 (o ExecuteUpdate e o ExecuteDelete do C#).
func (s *Servico) responderAlteradas(w http.ResponseWriter, r *http.Request, res sql.Result) {
	n, err := res.RowsAffected()
	switch {
	case err != nil:
		s.erroInterno(w, r, "contar as linhas", err)
	case n == 0:
		naoEncontrado(w)
	default:
		respostas.SemCorpo(w, http.StatusNoContent)
	}
}

// pagar marca o vencimento atual como pago e avança para o próximo (ou desativa a única; #69: passou da data limite,
// a conta é encerrada). Lança a despesa, se pedido, só quando a conta vinculada é manual.
func (s *Servico) pagar(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	lancar := respostas.CampoBool{Propriedade: "LancarTransacao"}
	data := respostas.CampoDataOpcional{Propriedade: "Data"}
	if !respostas.LerObjeto(w, r, "dto", "Financas.Api.Dtos.PagarContaAgendadaDto", &lancar, &data) {
		return
	}
	ctx := r.Context()
	var c conta
	achou, err := s.ler(ctx, id, &c)
	if err != nil {
		s.erroInterno(w, r, "ler a conta a pagar", err)
		return
	}
	if !achou {
		naoEncontrado(w)
		return
	}
	dataPagamento := s.hoje()
	if data.Valor != nil {
		dataPagamento = *data.Valor
	}
	if lancar.Valor {
		manual := false
		if c.ContaId != nil {
			var origem int64
			if err := s.db.QueryRowContext(ctx, `SELECT "Origem" FROM "Conta" WHERE "Id" = ?`, *c.ContaId).Scan(&origem); err != nil &&
				!errors.Is(err, sql.ErrNoRows) {
				s.erroInterno(w, r, "ler a conta", err)
				return
			}
			manual = origem == origemManual
		}
		if !manual {
			respostas.Mensagem(w, http.StatusBadRequest, "Para lançar a despesa, vincule uma conta manual. Em contas do Open "+
				"Finance o pagamento chega pela sincronização.")
			return
		}
	}
	proximo, ativa := c.ProximoVencimento, c.Ativa
	if p, ok := relatorios.ProximoVencimentoApos(c.ProximoVencimento, c.Recorrencia); ok {
		proximo = p
		if c.DataLimite != nil && p.Depois(*c.DataLimite) {
			ativa = false
		}
	} else {
		ativa = false
	}
	err = banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		if lancar.Valor {
			if err := s.lancarDespesa(ctx, tx, c, dataPagamento); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, `UPDATE "ContaAgendada" SET "UltimoPagamentoEm" = ?, "ProximoVencimento" = ?, "Ativa" = ?
			WHERE "Id" = ?`, dataPagamento, proximo, ativa, id)
		return err
	})
	if err != nil {
		s.erroInterno(w, r, "pagar", err)
		return
	}
	respostas.SemCorpo(w, http.StatusNoContent)
}

// lancarDespesa grava a despesa do pagamento na conta manual: o valor negativo, efetivada, a categoria da conta a
// pagar (manual, se houver) e o tipo pelo tipo da categoria (o AplicarTipo).
func (s *Servico) lancarDespesa(ctx context.Context, tx *sql.Tx, c conta, data tipos.Data) error {
	rs, err := categorizacao.Carregar(ctx, tx)
	if err != nil {
		return err
	}
	agora := s.agora()
	t := transacoes.Transacao{ContaId: *c.ContaId, Data: data, Descricao: c.Descricao,
		Valor: tipos.DeDecimal(c.Valor.Decimal().Neg()), Status: statusEfetivada, CategoriaId: c.CategoriaId,
		CategoriaManual: c.CategoriaId != nil}
	rs.AplicarTipo(&t)
	_, err = tx.ExecContext(ctx, `INSERT INTO "Transacao" ("ContaId", "PluggyTransactionId", "Data", "Descricao",
		"DescricaoOriginal", "Estabelecimento", "Valor", "Tipo", "Status", "CategoriaId", "CategoriaManual",
		"PluggyCategoriaId", "FaturaCartaoId", "ParcelaNumero", "ParcelaTotal", "IgnorarRelatorios", "Emergencia",
		"DescricaoRevisada", "MeioPagamento", "MesmaTitularidade", "RegraDescricaoId", "DescricaoManual",
		"TransferenciaGrupo", "Observacao", "CriadoEm", "AtualizadoEm")
		VALUES (?, NULL, ?, ?, NULL, NULL, ?, ?, ?, ?, ?, NULL, NULL, NULL, NULL, ?, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, ?, ?)`,
		t.ContaId, t.Data, t.Descricao, t.Valor, t.Tipo, t.Status, t.CategoriaId, t.CategoriaManual, t.IgnorarRelatorios,
		agora, agora)
	if err != nil {
		return fmt.Errorf("lançar a despesa do pagamento: %w", err)
	}
	return nil
}
