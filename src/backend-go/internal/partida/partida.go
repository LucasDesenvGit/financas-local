// Pacote partida são as rotinas que acertam dados toda vez que a API sobe (Program.cs do .NET, logo depois do
// EsquemaBanco.Aplicar): seis, sempre nesta ordem, que importa (#444).
//
//  1. regras padrão novas, uma vez por banco, com marca em Configuracao (ADR 0038, #172);
//  2. entrada no cartão é estorno ou pagamento de fatura, nunca receita (ADR 0036, #166);
//  3. compra parcelada devolvida inteira sai dos relatórios (ADR 0036, #257): precisa da 2, que é quem transforma em
//     estorno (despesa positiva) a entrada que o banco mandou como receita;
//  4. transferência só fica fora dos relatórios entre contas suas (ADR 0031);
//  5. cobrança automática que chegou dá baixa na conta a pagar (#158);
//  6. troco "sem dúvida" das compras de renda variável vai para a categoria da compra (ADR 0044, #173).
//
// A regra de cada uma mora no pacote do assunto dela (categorizacao, transferencias, cobrancas, carteira), numa função
// só: a sincronização (#83) chama as mesmas depois de cada rodada. Aqui só ficam a ordem, a transação de cada rotina e o
// que fazer quando uma falha.
//
// Quem chama é o cmd/financas, só quando o Go está na frente (sozinho, com -rotinas-de-partida): atrás do .NET, quem
// roda as rotinas é o .NET, e rodar nos dois seria acertar duas vezes (ADR 0049, #297).
package partida

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/carteira"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/categorizacao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cobrancas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transferencias"
)

// Opcoes são as dependências de Executar.
type Opcoes struct {
	DB      *sql.DB
	Relogio relogio.Relogio
	Log     *slog.Logger
}

// Resultado é o que uma rotina fez: quantas linhas acertou e, se parou, o erro.
type Resultado struct {
	Rotina  string
	Mudaram int
	Erro    error
}

// Nomes das rotinas, na ordem de execução. Aparecem no log e em Resultado.Rotina.
const (
	RegrasPadrao                = "regras padrão novas"
	EntradasNosCartoes          = "entradas nos cartões"
	ComprasParceladasEstornadas = "compras parceladas estornadas"
	Transferencias              = "transferências"
	ContasAPagar                = "contas a pagar cobradas sozinhas"
	TrocosSemDuvida             = "trocos sem dúvida"
)

// rotina é um passo da partida. Cada uma grava o que acertou (a própria transação) antes de a próxima começar.
type rotina struct {
	nome     string
	executar func(ctx context.Context) (int, error)
}

// Executar roda as seis rotinas, na ordem do .NET, e devolve o que cada uma fez (só as que rodaram). Erro numa rotina não
// derruba a subida: é registrado como aviso (que vai para Configurações › Erros) e a partida segue para a próxima (#444);
// a que falhou desfez o que fez e roda de novo na próxima partida. Diferente do try/catch único do Program.cs, que para
// na primeira: cada rotina tem a transação dela, então seguir não deixa nada pela metade, e uma rotina quebrada não
// segura as outras a cada subida. Só o contexto cancelado (o sistema parando) interrompe a partida.
func Executar(ctx context.Context, o Opcoes) []Resultado {
	return executar(ctx, o.Log, rotinas(o))
}

// rotinas são as seis, na ordem. As cinco primeiras gravam numa transação cada uma (o SaveChanges do .NET entre uma e
// outra); a última tem a transação dela, como o ClassificarTrocosSemDuvidaAsync, que grava sozinho.
func rotinas(o Opcoes) []rotina {
	emTransacao := func(fn func(ctx context.Context, tx *sql.Tx) (int, error)) func(context.Context) (int, error) {
		return func(ctx context.Context) (int, error) {
			var mudaram int
			err := banco.EmTransacao(ctx, o.DB, func(tx *sql.Tx) (err error) {
				mudaram, err = fn(ctx, tx)
				return err
			})
			if err != nil {
				return 0, err
			}
			return mudaram, nil
		}
	}
	agora := func() tipos.DataHora { return tipos.NovaDataHora(o.Relogio.Agora()) }
	trocos := carteira.NovoServicoDeAcertos(o.DB, o.Relogio, o.Log)
	return []rotina{
		{RegrasPadrao, emTransacao(func(ctx context.Context, tx *sql.Tx) (int, error) {
			return categorizacao.CriarRegrasPadrao(ctx, tx, o.Relogio)
		})},
		{EntradasNosCartoes, emTransacao(func(ctx context.Context, tx *sql.Tx) (int, error) {
			return categorizacao.ReclassificarEntradasNosCartoes(ctx, tx, agora())
		})},
		{ComprasParceladasEstornadas, emTransacao(func(ctx context.Context, tx *sql.Tx) (int, error) {
			return categorizacao.IgnorarComprasParceladasEstornadas(ctx, tx, relogio.Hoje(o.Relogio), agora())
		})},
		{Transferencias, emTransacao(func(ctx context.Context, tx *sql.Tx) (int, error) {
			return transferencias.Reclassificar(ctx, tx, agora())
		})},
		{ContasAPagar, emTransacao(func(ctx context.Context, tx *sql.Tx) (int, error) {
			return cobrancas.Conciliar(ctx, tx, relogio.Hoje(o.Relogio))
		})},
		{TrocosSemDuvida, trocos.ClassificarTrocosSemDuvida},
	}
}

// executar roda as rotinas na ordem, todas, mesmo quando uma falha; para só com o contexto cancelado, em que as seguintes
// falhariam do mesmo jeito. Pânico em rotina vira erro (o .NET pega toda exceção): a API sobe do mesmo jeito.
func executar(ctx context.Context, log *slog.Logger, lista []rotina) []Resultado {
	resultados := make([]Resultado, 0, len(lista))
	for _, r := range lista {
		n, err := rodar(ctx, r)
		resultados = append(resultados, Resultado{Rotina: r.nome, Mudaram: n, Erro: err})
		if err != nil {
			log.Warn("Não foi possível acertar os dados ao iniciar: a rotina falhou e roda de novo na próxima partida.",
				"rotina", r.nome, "erro", err)
			if ctx.Err() != nil {
				break
			}
			continue
		}
		if n > 0 {
			log.Info("rotina de partida acertou dados", "rotina", r.nome, "mudaram", n)
		}
	}
	return resultados
}

func rodar(ctx context.Context, r rotina) (n int, err error) {
	defer func() {
		if p := recover(); p != nil {
			n, err = 0, fmt.Errorf("pânico na rotina %q: %v", r.nome, p)
		}
	}()
	return r.executar(ctx)
}
