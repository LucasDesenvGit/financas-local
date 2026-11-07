// Pacote openfinance são as rotas do Open Finance (#83): o OpenFinanceController do .NET. Credenciais, conexões, estado
// da sincronização e o mapa das categorias da Pluggy. Quem fala com a Pluggy é o pacote pluggy.
package openfinance

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/openfinance/pluggy"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/openfinance/sincronizacao"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/patrimonio/cotacoes"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/configuracao"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/servidor"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

const mensagemNaoEncontrado = "Registro não encontrado."

// As chaves da Configuracao (ConfiguracaoService.cs).
const (
	chaveClientId          = "Pluggy.ClientId"
	chaveClientSecret      = "Pluggy.ClientSecret"
	chaveIntervaloHoras    = "Sincronizacao.IntervaloHoras"
	chaveDiasRetroativos   = "Sincronizacao.DiasRetroativos"
	chaveHabilitada        = "Sincronizacao.Habilitada"
	chaveDiasConsentimento = "Painel.DiasAvisoConsentimento"
)

// fonteDeParametros é o que as rotas usam do internal/ajustes/parametros: o valor limitado à faixa do catálogo.
type fonteDeParametros interface {
	ObterInteiro(ctx context.Context, chave string) (int, error)
}

// sincronizador é o que as rotas usam da sincronização: uma interface para o teste das rotas não falar com a Pluggy.
type sincronizador interface {
	EmExecucao() bool
	TestarCredenciais(ctx context.Context) (bool, string, error)
	ConsultarItem(ctx context.Context, itemId string) (*pluggy.Item, error)
	Sincronizar(ctx context.Context, conexaoId *int64, origem string) (sincronizacao.Resultado, error)
	ImportarCarteira(ctx context.Context) (sincronizacao.ResultadoCarteira, error)
}

// atualizadorDeCotacoes busca as cotações depois de importar a carteira.
type atualizadorDeCotacoes interface {
	Atualizar(ctx context.Context, somente []int64) (cotacoes.Resultado, error)
}

// Servico atende as rotas.
type Servico struct {
	db         *sql.DB
	config     *configuracao.Servico
	cofre      configuracao.Cofre
	parametros fonteDeParametros
	sincronia  sincronizador
	cotacoes   atualizadorDeCotacoes
	relogio    relogio.Relogio
	log        *slog.Logger
}

// NovoServico monta o serviço. O cofre guarda o Client Secret no esquema do Go (#85, ADR 0049).
func NovoServico(db *sql.DB, cofre configuracao.Cofre, parametros fonteDeParametros, sincronia sincronizador, cotacoes atualizadorDeCotacoes, r relogio.Relogio, log *slog.Logger) *Servico {
	return &Servico{db: db, config: configuracao.Novo(db, r), cofre: cofre, parametros: parametros, sincronia: sincronia, cotacoes: cotacoes, relogio: r, log: log}
}

// Registrar põe as rotas no mux.
func Registrar(mux *http.ServeMux, s *Servico) {
	mux.HandleFunc("GET /api/open-finance/configuracao", s.obterConfiguracao)
	mux.HandleFunc("PUT /api/open-finance/configuracao", s.salvarConfiguracao)
	mux.HandleFunc("DELETE /api/open-finance/configuracao/secret", s.removerSecret)
	mux.HandleFunc("PUT /api/open-finance/meu-nome", s.salvarMeuNome)
	mux.HandleFunc("GET /api/open-finance/conexoes", s.listarConexoes)
	mux.HandleFunc("PUT /api/open-finance/conexoes/{id}/ativa", s.alterarAtiva)
	mux.HandleFunc("DELETE /api/open-finance/conexoes/{id}", s.removerConexao)
	mux.HandleFunc("GET /api/open-finance/sincronizacao/estado", s.estado)
	mux.HandleFunc("GET /api/open-finance/status", s.status)
	mux.HandleFunc("GET /api/open-finance/logs", s.logs)
	mux.HandleFunc("GET /api/open-finance/categorias", s.categorias)
	mux.HandleFunc("PUT /api/open-finance/categorias/{pluggyCategoriaId}", s.mapearCategoria)
	mux.HandleFunc("POST /api/open-finance/testar", s.testar)
	mux.HandleFunc("POST /api/open-finance/conexoes", s.adicionarConexao)
	mux.HandleFunc("POST /api/open-finance/sincronizar", s.sincronizar)
	mux.HandleFunc("POST /api/carteira/importar-open-finance", s.importarCarteira)
}

func (s *Servico) agora() tipos.DataHora { return tipos.NovaDataHora(s.relogio.Agora()) }

// idDaRota é o {id:int} do .NET: o que não é int não casa com a rota e dá 404 sem corpo.
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
	s.log.ErrorContext(r.Context(), "erro no Open Finance", "origem", "API", "etapa", etapa, "erro", err)
	servidor.ResponderErroInterno(w)
}
