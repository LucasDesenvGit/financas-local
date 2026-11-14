// Pacote api junta as rotas de todos os assuntos já em Go num ServeMux só. É o MapControllers do Go.
//
// Cada assunto tem o próprio pacote (internal/<area>/<assunto>) e exporta Registrar(mux, <o que ele usa>), que põe as
// rotas dele no mux com o padrão do Go 1.22 ("GET /api/assunto/{id}"). Aqui entra uma linha por assunto, e só isso:
// assim, assuntos feitos em paralelo (#75, #77, #81) não mexem nas mesmas linhas. No merge, conflito neste arquivo é
// sempre "fique com as duas linhas" (e o gofmt põe os imports em ordem).
//
// Desde a #86 o Go atende todas as rotas, sem o .NET na frente. Quem põe a autenticação na frente deste mux é o
// cmd/financas (autenticacao.Proteger). O desenho das peças está em docs/arquitetura/componentes.md (#464).
package api

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/ajustes/categorias"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/ajustes/parametros"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/analises/painel"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/movimentacao/conferenciabanco"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/movimentacao/contas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/movimentacao/contasagendadas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/movimentacao/extrato"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/openfinance/openfinance"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/openfinance/pluggy"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/openfinance/sincronizacao"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/patrimonio/cambio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/patrimonio/carteira"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/patrimonio/cotacoes"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/patrimonio/cripto"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/patrimonio/investimentos"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/patrimonio/reservas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/patrimonio/selic"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/planejamento/metas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/planejamento/orcamentos"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/config"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/modulos"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/saude"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/segredos"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/versao"
)

// Dependencias de todos os assuntos, passadas pelo construtor (sem container de injeção). Cada assunto recebe no
// Registrar só o que usa.
type Dependencias struct {
	DB      *sql.DB
	Log     *slog.Logger
	Relogio relogio.Relogio // relogio.Sistema no site; relogio.APartirDe no contrato (-inicio-do-relogio)
	Config  config.Config   // os appsettings (caminhos, padrões dos parâmetros)
}

// NovasRotas devolve o ServeMux com as rotas de todos os assuntos.
func NovasRotas(d Dependencias) *http.ServeMux { return Montar(d).Rotas }

// Montado é a API pronta: as rotas e o que uma tarefa em segundo plano divide com elas.
type Montado struct {
	Rotas         *http.ServeMux
	Sincronizacao *sincronizacao.Servico // a mesma instância nas rotas e na tarefa, por causa da trava de uma rodada por vez
	Modulos       *modulos.Catalogo      // os módulos (ADR 0064): o cmd/financas roda as tarefas deles
}

// Montar monta os assuntos. Caminho sem rota recebe 404 sem corpo, como o MapFallback("/api/{**caminho}") do .NET.
func Montar(d Dependencias) Montado {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "0")
		w.WriteHeader(http.StatusNotFound)
	})

	// Uma linha por assunto, na ordem do épico #70.
	saude.Registrar(mux, d.DB, d.Log, d.Relogio) // #231: o exemplo que passa pelo caminho todo (front-end → .NET → Go → banco)
	par := parametros.Novo(parametros.Opcoes{
		DB: d.DB, Relogio: d.Relogio, Log: d.Log, Versao: versao.Atual,
		CaminhoBanco: d.Config.BancoCaminho, PastaChaves: d.Config.PastaChaves, Padroes: parametros.PadroesDoAppsettings(d.Config),
	})
	parametros.Registrar(mux, par)                                            // #76
	cambio.Registrar(mux, d.DB, d.Log, d.Relogio)                             // #81: câmbio pela PTAX (ADR 0007)
	cripto.Registrar(mux, d.DB, d.Log, d.Relogio)                             // #81: criptomoedas pelo Mercado Bitcoin (ADR 0048)
	categorias.Registrar(mux, categorias.NovoServico(d.DB, d.Relogio, d.Log)) // #77: categorias, regras e regras de descrição
	contas.Registrar(mux, contas.NovoServico(d.DB, d.Relogio, d.Log))         // #77: contas e faturas do cartão
	extrato.Registrar(mux, extrato.NovoServico(d.DB, d.Relogio, par, d.Log))  // #78: transações (as 18 rotas)
	contasAPagar := contasagendadas.NovoServico(d.DB, d.Relogio, d.Log)
	contasagendadas.Registrar(mux, contasAPagar)                                   // #79: contas a pagar
	orcamentos.Registrar(mux, orcamentos.NovoServico(d.DB, par, d.Relogio, d.Log)) // #79: orçamentos e a sugestão (#39)
	cambioDaCarteira := cambio.NovoServico(d.DB, cambio.ClientePadrao(), d.Relogio, d.Log)
	reservasDaCarteira := reservas.NovoServico(d.DB, cambioDaCarteira, d.Relogio, d.Log)
	reservas.Registrar(mux, reservasDaCarteira)                                              // #82: as reservas (ADRs 0010, 0037 e 0038)
	metas.Registrar(mux, metas.NovoServico(d.DB, reservasDaCarteira, par, d.Relogio, d.Log)) // #79: metas e o plano do caixa
	// O token da brapi só vale depois da troca dos segredos (#85, ADR 0049): o cofre abre a chave no primeiro uso.
	cofre := segredos.NovoTardio(d.Config.PastaChaves)
	cotacoesDaCarteira := cotacoes.NovoServico(d.DB, cotacoes.ClientePadrao(), cambioDaCarteira, cofre, d.Relogio, d.Log)
	daCarteira := carteira.NovoServico(d.DB, cambioDaCarteira, reservasDaCarteira,
		selic.NovoServico(d.DB, selic.ClientePadrao(), d.Relogio, d.Log), cotacoesDaCarteira, cofre, d.Relogio, d.Log)
	carteira.Registrar(mux, daCarteira)                                                                                                     // #82: a carteira
	conferenciabanco.Registrar(mux, conferenciabanco.NovoServico(d.DB, reservasDaCarteira, daCarteira, cambioDaCarteira, d.Relogio, d.Log)) // #445: conferir com o banco
	painel.Registrar(mux, painel.NovoServico(d.DB, daCarteira, reservasDaCarteira, cambioDaCarteira, par, contasAPagar, d.Relogio, d.Log))  // #80: empréstimos e relatórios
	investimentos.Registrar(mux, investimentos.NovoServico(d.DB, cambioDaCarteira, d.Relogio, d.Log))                                       // #80: os do Open Finance
	clientePluggy, err := pluggy.NovoCliente(d.Config.PluggyURLBase, nil, d.Log)
	if err != nil {
		panic(err) // como o Program.cs, que recusa subir com a URL da Pluggy sem HTTPS
	}
	sincronia := sincronizacao.NovoServico(d.DB, cofre, clientePluggy, daCarteira, d.Relogio, d.Log)
	openfinance.Registrar(mux, openfinance.NovoServico(d.DB, cofre, par, sincronia, cotacoesDaCarteira, d.Relogio, d.Log)) // #83: as rotas do Open Finance

	// Os módulos (#190, ADR 0064): um catalogo.Adicionar por módulo, na ordem em que aparecem em Configurações ›
	// Módulos. O catálogo guarda as rotas de cada um (desligado, 409) e põe as dele (GET /api/modulos).
	catalogo := modulos.NovoCatalogo(d.DB, d.Relogio, d.Log)
	catalogo.Montar(mux)
	return Montado{Rotas: mux, Sincronizacao: sincronia, Modulos: catalogo}
}
