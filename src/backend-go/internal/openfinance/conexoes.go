package openfinance

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/auditoria"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/configuracao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/contas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// conexaoDto é o ConexaoDto.
type conexaoDto struct {
	Id                           int64           `json:"id"`
	PluggyItemId                 string          `json:"pluggyItemId"`
	NomeInstituicao              *string         `json:"nomeInstituicao"`
	CorPrimaria                  *string         `json:"corPrimaria"`
	Status                       *string         `json:"status"`
	StatusExecucao               *string         `json:"statusExecucao"`
	UltimaAtualizacaoInstituicao *tipos.DataHora `json:"ultimaAtualizacaoInstituicao"`
	ConsentimentoExpiraEm        *tipos.DataHora `json:"consentimentoExpiraEm"`
	UltimaSincronizacaoEm        *tipos.DataHora `json:"ultimaSincronizacaoEm"`
	UltimoErro                   *string         `json:"ultimoErro"`
	Ativa                        bool            `json:"ativa"`
	QuantidadeContas             int64           `json:"quantidadeContas"`
	NomeExibicao                 *string         `json:"nomeExibicao"`
}

func (s *Servico) lerConexoes(ctx context.Context) ([]conexaoDto, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT c."Id", c."PluggyItemId", c."NomeInstituicao", c."CorPrimaria", c."Status",
		c."StatusExecucao", c."UltimaAtualizacaoInstituicao", c."ConsentimentoExpiraEm", c."UltimaSincronizacaoEm", c."UltimoErro",
		c."Ativa", (SELECT COUNT(*) FROM "Conta" ct WHERE ct."ConexaoBancariaId" = c."Id")
		FROM "ConexaoBancaria" c ORDER BY c."Id"`)
	if err != nil {
		return nil, fmt.Errorf("ler as conexões: %w", err)
	}
	defer linhas.Close()
	lista := []conexaoDto{}
	for linhas.Next() {
		var c conexaoDto
		if err := linhas.Scan(&c.Id, &c.PluggyItemId, &c.NomeInstituicao, &c.CorPrimaria, &c.Status, &c.StatusExecucao,
			&c.UltimaAtualizacaoInstituicao, &c.ConsentimentoExpiraEm, &c.UltimaSincronizacaoEm, &c.UltimoErro, &c.Ativa,
			&c.QuantidadeContas); err != nil {
			return nil, fmt.Errorf("ler conexão: %w", err)
		}
		lista = append(lista, c)
	}
	return lista, linhas.Err()
}

// listarConexoes devolve as conexões pelo nome de exibição, na ordem pt-BR: o banco deduzido das contas (#170), e não
// "MeuPluggy", que é o nome do conector.
func (s *Servico) listarConexoes(w http.ResponseWriter, r *http.Request) {
	lista, err := s.lerConexoes(r.Context())
	if err != nil {
		s.erroInterno(w, r, "listar as conexões", err)
		return
	}
	nomes, err := contas.NomesDasConexoes(r.Context(), s.db)
	if err != nil {
		s.erroInterno(w, r, "ler os nomes das conexões", err)
		return
	}
	for i := range lista {
		if nome, ok := nomes[lista[i].Id]; ok {
			lista[i].NomeExibicao = &nome
		}
	}
	slices.SortStableFunc(lista, func(a, b conexaoDto) int { return compararOpcional(a.NomeExibicao, b.NomeExibicao) })
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, lista)
}

// compararOpcional é o OrderBy do LINQ em string?: nulo primeiro, e o texto na ordem pt-BR.
func compararOpcional(a, b *string) int {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil:
		return -1
	case b == nil:
		return 1
	}
	return texto.CompararNaCultura(*a, *b)
}

// alterarAtiva liga ou desliga a conexão. Sem o parâmetro "ativa", vale false, como o bool do [FromQuery] no .NET.
func (s *Servico) alterarAtiva(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	ativa := false
	if valor, tem := respostas.ParametroDaQuery(r, "ativa"); tem {
		if ativa, ok = respostas.BoolDaQuery(valor); !ok {
			respostas.Validacao(w, map[string][]string{"ativa": {respostas.MensagemDaQuery(valor)}})
			return
		}
	}
	resultado, err := s.db.ExecContext(r.Context(), `UPDATE "ConexaoBancaria" SET "Ativa" = ? WHERE "Id" = ?`, ativa, id)
	if err != nil {
		s.erroInterno(w, r, "alterar a conexão", err)
		return
	}
	alteradas, err := resultado.RowsAffected()
	if err != nil {
		s.erroInterno(w, r, "alterar a conexão", err)
		return
	}
	if alteradas == 0 {
		respostas.Mensagem(w, http.StatusNotFound, mensagemNaoEncontrado)
		return
	}
	respostas.SemCorpo(w, http.StatusNoContent)
}

// removerConexao apaga a conexão e tudo o que veio dela: o banco apaga em cascata o que tem chave estrangeira, e o
// histórico dos investimentos (sem chave, de propósito) e o log (ON DELETE SET NULL) saem aqui (#575).
func (s *Servico) removerConexao(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	if s.emExecucao.Load() {
		respostas.Mensagem(w, http.StatusConflict, "Aguarde a sincronização em andamento terminar.")
		return
	}
	ctx := r.Context()
	var existe int64
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM "ConexaoBancaria" WHERE "Id" = ?`, id).Scan(&existe)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		respostas.Mensagem(w, http.StatusNotFound, mensagemNaoEncontrado)
		return
	case err != nil:
		s.erroInterno(w, r, "ler a conexão", err)
		return
	}
	login, err := auditoria.Login(ctx, consultas.New(s.db))
	if err != nil {
		s.erroInterno(w, r, "ler o login", err)
		return
	}
	err = banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		ids, err := idsDasContas(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := contas.DesvincularFaturas(ctx, tx, ids...); err != nil {
			return err
		}
		for _, sqlApagar := range []string{
			`DELETE FROM "HistoricoInvestimento" WHERE "ConexaoBancariaId" = ?`,
			`DELETE FROM "SincronizacaoLog" WHERE "ConexaoBancariaId" = ?`,
			`DELETE FROM "ConexaoBancaria" WHERE "Id" = ?`,
		} {
			if _, err := tx.ExecContext(ctx, sqlApagar, id); err != nil {
				return err
			}
		}
		return auditoria.Registrar(ctx, consultas.New(tx), r, s.agora(), "ConexaoRemovida", login)
	})
	if err != nil {
		s.erroInterno(w, r, "remover a conexão", err)
		return
	}
	respostas.SemCorpo(w, http.StatusNoContent)
}

func idsDasContas(ctx context.Context, tx *sql.Tx, conexao int64) ([]int64, error) {
	linhas, err := tx.QueryContext(ctx, `SELECT "Id" FROM "Conta" WHERE "ConexaoBancariaId" = ?`, conexao)
	if err != nil {
		return nil, fmt.Errorf("ler as contas da conexão: %w", err)
	}
	defer linhas.Close()
	var ids []int64
	for linhas.Next() {
		var id int64
		if err := linhas.Scan(&id); err != nil {
			return nil, fmt.Errorf("ler conta da conexão: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, linhas.Err()
}

func (s *Servico) estado(w http.ResponseWriter, _ *http.Request) {
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, struct {
		EmExecucao bool `json:"emExecucao"`
	}{s.emExecucao.Load()})
}

// statusDto é o StatusSincronizacaoDto.
type statusDto struct {
	UltimaSincronizacao    *configuracao.Instante `json:"ultimaSincronizacao"`
	EmExecucao             bool                   `json:"emExecucao"`
	ConexoesAtivas         int                    `json:"conexoesAtivas"`
	ConexoesComErro        int                    `json:"conexoesComErro"`
	ConsentimentosVencendo int                    `json:"consentimentosVencendo"`
}

// status é o resumo leve da barra superior: lê só a configuração e as conexões ativas.
func (s *Servico) status(w http.ResponseWriter, r *http.Request) {
	d, err := s.lerStatus(r.Context())
	if err != nil {
		s.erroInterno(w, r, "ler o status", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, d)
}

func (s *Servico) lerStatus(ctx context.Context) (statusDto, error) {
	d := statusDto{EmExecucao: s.emExecucao.Load()}
	ultima, err := s.config.ObterData(ctx, configuracao.UltimaSincronizacaoGeral)
	if err != nil {
		return d, err
	}
	if ultima != nil {
		emUtc := ultima.ParaUtc()
		d.UltimaSincronizacao = &emUtc
	}
	dias, err := s.parametros.ObterInteiro(ctx, chaveDiasConsentimento)
	if err != nil {
		return d, err
	}
	limite := s.relogio.Agora().AddDate(0, 0, dias)
	linhas, err := s.db.QueryContext(ctx, `SELECT "UltimoErro", "Status", "ConsentimentoExpiraEm" FROM "ConexaoBancaria" WHERE "Ativa"`)
	if err != nil {
		return d, fmt.Errorf("ler as conexões ativas: %w", err)
	}
	defer linhas.Close()
	for linhas.Next() {
		var ultimoErro, situacao *string
		var expira *tipos.DataHora
		if err := linhas.Scan(&ultimoErro, &situacao, &expira); err != nil {
			return d, fmt.Errorf("ler conexão ativa: %w", err)
		}
		d.ConexoesAtivas++
		// A mesma regra dos alertas do Painel: erro na última sincronização ou instituição pedindo ação no Meu Pluggy.
		if (ultimoErro != nil && *ultimoErro != "") || (situacao != nil && (*situacao == "LOGIN_ERROR" || *situacao == "WAITING_USER_ACTION")) {
			d.ConexoesComErro++
		}
		if expira != nil && !expira.Tempo().After(limite) {
			d.ConsentimentosVencendo++
		}
	}
	return d, linhas.Err()
}

// logDto é o SincronizacaoLogDto.
type logDto struct {
	Id                    int64           `json:"id"`
	ConexaoBancariaId     *int64          `json:"conexaoBancariaId"`
	NomeInstituicao       *string         `json:"nomeInstituicao"`
	Origem                string          `json:"origem"`
	IniciadoEm            tipos.DataHora  `json:"iniciadoEm"`
	FinalizadoEm          *tipos.DataHora `json:"finalizadoEm"`
	Sucesso               bool            `json:"sucesso"`
	TransacoesNovas       int64           `json:"transacoesNovas"`
	TransacoesAtualizadas int64           `json:"transacoesAtualizadas"`
	Mensagem              *string         `json:"mensagem"`
}

// logs devolve as 50 sincronizações mais novas, com o mesmo nome da lista de conexões (#170).
func (s *Servico) logs(w http.ResponseWriter, r *http.Request) {
	lista, err := s.lerLogs(r.Context())
	if err != nil {
		s.erroInterno(w, r, "ler os logs", err)
		return
	}
	nomes, err := contas.NomesDasConexoes(r.Context(), s.db)
	if err != nil {
		s.erroInterno(w, r, "ler os nomes das conexões", err)
		return
	}
	for i, l := range lista {
		if l.ConexaoBancariaId == nil {
			continue
		}
		if nome, ok := nomes[*l.ConexaoBancariaId]; ok {
			lista[i].NomeInstituicao = &nome
		}
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, lista)
}

func (s *Servico) lerLogs(ctx context.Context) ([]logDto, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT l."Id", l."ConexaoBancariaId", c."NomeInstituicao", l."Origem", l."IniciadoEm",
		l."FinalizadoEm", l."Sucesso", l."TransacoesNovas", l."TransacoesAtualizadas", l."Mensagem"
		FROM "SincronizacaoLog" l LEFT JOIN "ConexaoBancaria" c ON c."Id" = l."ConexaoBancariaId"
		ORDER BY l."Id" DESC LIMIT 50`)
	if err != nil {
		return nil, fmt.Errorf("ler os logs: %w", err)
	}
	defer linhas.Close()
	lista := []logDto{}
	for linhas.Next() {
		var l logDto
		if err := linhas.Scan(&l.Id, &l.ConexaoBancariaId, &l.NomeInstituicao, &l.Origem, &l.IniciadoEm, &l.FinalizadoEm,
			&l.Sucesso, &l.TransacoesNovas, &l.TransacoesAtualizadas, &l.Mensagem); err != nil {
			return nil, fmt.Errorf("ler log: %w", err)
		}
		lista = append(lista, l)
	}
	return lista, linhas.Err()
}
