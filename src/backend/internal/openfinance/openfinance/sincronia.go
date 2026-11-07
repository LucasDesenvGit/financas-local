package openfinance

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"net/http"
	"strings"
	"unicode"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/openfinance/pluggy"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/openfinance/sincronizacao"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/auditoria"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/respostas"
)

// padraoDoItem é o [RegularExpression] do AdicionarConexaoDto.
var padraoDoItem = func() *respostas.Padrao {
	p := respostas.NovoPadrao("^[0-9a-fA-F-]{20,64}$")
	p.MensagemPropria = "Item ID inválido."
	return p
}()

// testar autentica na Pluggy com as credenciais salvas.
func (s *Servico) testar(w http.ResponseWriter, r *http.Request) {
	ok, mensagem, err := s.sincronia.TestarCredenciais(r.Context())
	if err != nil {
		s.erroInterno(w, r, "testar as credenciais", err)
		return
	}
	if !ok {
		respostas.Mensagem(w, http.StatusBadRequest, mensagem)
		return
	}
	respostas.Mensagem(w, http.StatusOK, mensagem)
}

// adicionarConexao cadastra um item do Meu Pluggy (valida na Pluggy antes) e já faz a primeira sincronização.
func (s *Servico) adicionarConexao(w http.ResponseWriter, r *http.Request) {
	campo := respostas.CampoTexto{Propriedade: "ItemId", Maximo: math.MaxInt32, Padrao: padraoDoItem}
	if !respostas.LerObjeto(w, r, "dto", "Financas.Api.Dtos.AdicionarConexaoDto", &campo) {
		return
	}
	ctx := r.Context()
	itemId := strings.ToLower(strings.TrimFunc(*campo.Valor, unicode.IsSpace))
	var existe int64
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM "ConexaoBancaria" WHERE "PluggyItemId" = ?`, itemId).Scan(&existe)
	switch {
	case err == nil:
		respostas.Mensagem(w, http.StatusConflict, "Esta conexão já está cadastrada.")
		return
	case !errors.Is(err, sql.ErrNoRows):
		s.erroInterno(w, r, "ler as conexões", err)
		return
	}
	item, err := s.sincronia.ConsultarItem(ctx, itemId)
	var erroPluggy *pluggy.Erro
	if errors.As(err, &erroPluggy) {
		respostas.Mensagem(w, http.StatusBadRequest, erroPluggy.Mensagem)
		return
	}
	if err != nil {
		s.erroInterno(w, r, "consultar o item", err)
		return
	}
	login, err := auditoria.Login(ctx, consultas.New(s.db))
	if err != nil {
		s.erroInterno(w, r, "ler o login", err)
		return
	}
	id, err := s.gravarConexao(ctx, r, item, itemId, login)
	if err != nil {
		s.erroInterno(w, r, "gravar a conexão", err)
		return
	}
	s.responderSincronizacao(w, r, &id)
}

func (s *Servico) gravarConexao(ctx context.Context, r *http.Request, item *pluggy.Item, itemId string, login *string) (int64, error) {
	chave := itemId
	if n := len(item.Id); n > 0 && respostas.TamanhoUtf16(item.Id) <= 64 {
		chave = strings.ToLower(item.Id)
	}
	var nome *string
	if item.Connector != nil && item.Connector.Name != nil {
		cortado := respostas.CortarUtf16(*item.Connector.Name, 150)
		nome = &cortado
	}
	var id int64
	err := banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `INSERT INTO "ConexaoBancaria" ("PluggyItemId", "NomeInstituicao", "Status", "Ativa", "CriadoEm") VALUES (?, ?, ?, 1, ?)`,
			chave, nome, item.Status, s.agora())
		if err != nil {
			return err
		}
		if id, err = res.LastInsertId(); err != nil {
			return err
		}
		return auditoria.Registrar(ctx, consultas.New(tx), r, s.agora(), "ConexaoAdicionada", login)
	})
	return id, err
}

// sincronizar roda uma conexão (conexaoId) ou todas as ativas.
func (s *Servico) sincronizar(w http.ResponseWriter, r *http.Request) {
	var conexaoId *int64
	if valor, tem := respostas.ParametroDaQuery(r, "conexaoId"); tem && valor != "" {
		id, ok := respostas.InteiroDaQuery(valor)
		if !ok {
			respostas.Validacao(w, map[string][]string{"conexaoId": {respostas.MensagemDaQuery(valor)}})
			return
		}
		v := int64(id)
		conexaoId = &v
	}
	s.responderSincronizacao(w, r, conexaoId)
}

func (s *Servico) responderSincronizacao(w http.ResponseWriter, r *http.Request, conexaoId *int64) {
	resultado, err := s.sincronia.Sincronizar(r.Context(), conexaoId, sincronizacao.OrigemManual)
	if err != nil {
		s.erroInterno(w, r, "sincronizar", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, resultado)
}

// importarCarteira registra como compras os FIIs, ações e ETFs do Open Finance e já busca as cotações. Falha nas cotações
// não falha a rota: elas podem ser atualizadas depois.
func (s *Servico) importarCarteira(w http.ResponseWriter, r *http.Request) {
	resultado, err := s.sincronia.ImportarCarteira(r.Context())
	var erroPluggy *pluggy.Erro
	if errors.As(err, &erroPluggy) {
		respostas.Mensagem(w, http.StatusBadRequest, erroPluggy.Mensagem)
		return
	}
	if err != nil {
		s.erroInterno(w, r, "importar a carteira", err)
		return
	}
	if resultado.AtivosCriados > 0 || resultado.OperacoesImportadas > 0 {
		if _, err := s.cotacoes.Atualizar(r.Context(), nil); err != nil {
			if r.Context().Err() != nil {
				return
			}
			s.log.Warn("não deu para atualizar as cotações depois de importar a carteira", "origem", "Investimentos", "erro", err)
		}
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, resultado)
}
