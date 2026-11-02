// Pacote sincronizacao traz os dados do Open Finance para o banco (#83): o SincronizacaoService do .NET. Conexão, contas,
// faturas, transações, investimentos, empréstimos e a carteira de renda variável.
//
// Cada etapa grava numa transação do banco, como os SaveChanges do .NET: uma falha no meio não deixa a etapa pela metade,
// e o que veio antes dela fica. A chamada à Pluggy nunca acontece com uma transação aberta.
package sincronizacao

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"
	"unicode"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/categorizacao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cobrancas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/configuracao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/pluggy"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/regrasdescricao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/segredos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transferencias"
)

// As chaves da Configuracao (ConfiguracaoService.cs).
const (
	chaveClientId          = "Pluggy.ClientId"
	chaveClientSecret      = "Pluggy.ClientSecret"
	chaveDiasRetroativos   = "Sincronizacao.DiasRetroativos"
	chaveDiasPrimeiraCarga = "Sincronizacao.DiasPrimeiraCarga"
	semCredenciais         = "Configure o Client ID e o Client Secret da Pluggy."
)

// Origens do log de sincronização.
const (
	OrigemManual     = "Manual"
	OrigemAutomatica = "Automatica"
)

// classificadorDeTrocos é o que a sincronização usa da carteira: o troco sem dúvida vai para a categoria da compra (ADR 0044).
type classificadorDeTrocos interface {
	ClassificarTrocosSemDuvida(ctx context.Context) (int, error)
}

// Servico sincroniza. A trava é o SemaphoreSlim do .NET: uma rodada por vez, sem fila.
type Servico struct {
	db         *sql.DB
	config     *configuracao.Servico
	cofre      configuracao.Cofre
	pluggy     *pluggy.Cliente
	trocos     classificadorDeTrocos
	relogio    relogio.Relogio
	log        *slog.Logger
	emExecucao atomic.Bool
}

// NovoServico monta o serviço.
func NovoServico(db *sql.DB, cofre configuracao.Cofre, cliente *pluggy.Cliente, trocos classificadorDeTrocos, r relogio.Relogio, log *slog.Logger) *Servico {
	return &Servico{db: db, config: configuracao.Novo(db, r), cofre: cofre, pluggy: cliente, trocos: trocos, relogio: r, log: log}
}

// EmExecucao diz se há uma rodada em andamento.
func (s *Servico) EmExecucao() bool { return s.emExecucao.Load() }

// Resultado é o ResultadoSincronizacaoDto.
type Resultado struct {
	Sucesso               bool    `json:"sucesso"`
	TransacoesNovas       int     `json:"transacoesNovas"`
	TransacoesAtualizadas int     `json:"transacoesAtualizadas"`
	Mensagem              *string `json:"mensagem"`
}

func falhou(mensagem string) Resultado { return Resultado{Mensagem: &mensagem} }

func (s *Servico) agora() tipos.DataHora { return tipos.NovaDataHora(s.relogio.Agora()) }

func (s *Servico) avisar(msg string, args ...any) {
	s.log.Warn(msg, append([]any{"origem", "Open Finance"}, args...)...)
}

// credenciais lê o Client ID e o Client Secret. Sem um dos dois, ok é false. O segredo gravado pelo .NET não se lê até a
// troca dos segredos (#85): vale como não configurado, com o aviso no log.
func (s *Servico) credenciais(ctx context.Context) (id, secret string, ok bool, err error) {
	clientId, err := s.config.Obter(ctx, chaveClientId)
	if err != nil {
		return "", "", false, err
	}
	segredo, err := s.config.ObterSegredo(ctx, s.cofre, chaveClientSecret)
	if errors.Is(err, segredos.ErrInformarDeNovo) {
		s.avisar("O Client Secret da Pluggy precisa ser informado de novo.")
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}
	if clientId == nil || segredo == nil || strings.TrimFunc(*clientId, unicode.IsSpace) == "" || strings.TrimFunc(*segredo, unicode.IsSpace) == "" {
		return "", "", false, nil
	}
	return *clientId, *segredo, true, nil
}

// TestarCredenciais tenta autenticar. Devolve se deu certo e a mensagem; erro só no que não é resposta da Pluggy.
func (s *Servico) TestarCredenciais(ctx context.Context) (bool, string, error) {
	id, secret, ok, err := s.credenciais(ctx)
	if err != nil {
		return false, "", err
	}
	if !ok {
		return false, "Client ID e Client Secret ainda não configurados.", nil
	}
	_, err = s.pluggy.CriarSessao(ctx, id, secret)
	var erroPluggy *pluggy.Erro
	if errors.As(err, &erroPluggy) {
		return false, erroPluggy.Mensagem, nil
	}
	if err != nil {
		return false, "", err
	}
	return true, "Conexão com a Pluggy realizada com sucesso.", nil
}

// ConsultarItem busca o item, para validar antes de cadastrar a conexão.
func (s *Servico) ConsultarItem(ctx context.Context, itemId string) (*pluggy.Item, error) {
	id, secret, ok, err := s.credenciais(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, &pluggy.Erro{Mensagem: "Configure o Client ID e o Client Secret da Pluggy antes de adicionar conexões."}
	}
	sessao, err := s.pluggy.CriarSessao(ctx, id, secret)
	if err != nil {
		return nil, err
	}
	return s.pluggy.ObterItem(ctx, sessao, itemId)
}

// Sincronizar roda uma conexão (conexaoId) ou todas as ativas. Conexão que falha vira mensagem no resultado, e a próxima
// roda assim mesmo; erro só no cancelamento e no que impede a rodada de começar.
func (s *Servico) Sincronizar(ctx context.Context, conexaoId *int64, origem string) (Resultado, error) {
	if !s.emExecucao.CompareAndSwap(false, true) {
		return falhou("Já existe uma sincronização em andamento."), nil
	}
	defer s.emExecucao.Store(false)

	id, secret, ok, err := s.credenciais(ctx)
	if err != nil {
		return Resultado{}, err
	}
	if !ok {
		return falhou(semCredenciais), nil
	}
	conexoes, err := s.conexoesAtivas(ctx, conexaoId)
	if err != nil {
		return Resultado{}, err
	}
	if len(conexoes) == 0 {
		return falhou("Nenhuma conexão ativa para sincronizar."), nil
	}
	sessao, err := s.pluggy.CriarSessao(ctx, id, secret)
	var erroPluggy *pluggy.Erro
	if errors.As(err, &erroPluggy) {
		if err := s.registrarFalhaDeSessao(ctx, origem, erroPluggy.Mensagem); err != nil {
			return Resultado{}, err
		}
		return falhou(erroPluggy.Mensagem), nil
	}
	if err != nil {
		return Resultado{}, err
	}
	// A lista de categorias não impede a sincronização das transações.
	if err := s.atualizarCategorias(ctx, sessao); err != nil {
		if ctx.Err() != nil {
			return Resultado{}, ctx.Err()
		}
		s.avisar("Não foi possível atualizar a lista de categorias da Pluggy.", "erro", err)
	}
	regras, err := categorizacao.Carregar(ctx, s.db)
	if err != nil {
		return Resultado{}, err
	}
	descricoes, err := regrasdescricao.Carregar(ctx, s.db)
	if err != nil {
		return Resultado{}, err
	}
	r := &rodada{Servico: s, sessao: sessao, regras: regras, descricoes: descricoes, origem: origem}
	total := Resultado{Sucesso: true}
	var erros []string
	for _, conexao := range conexoes {
		parcial, err := r.sincronizarConexao(ctx, conexao)
		if err != nil {
			return Resultado{}, err
		}
		total.TransacoesNovas += parcial.TransacoesNovas
		total.TransacoesAtualizadas += parcial.TransacoesAtualizadas
		if !parcial.Sucesso && parcial.Mensagem != nil {
			erros = append(erros, *parcial.Mensagem)
		}
	}
	if err := s.config.DefinirData(ctx, configuracao.UltimaSincronizacaoGeral, s.relogio.Agora()); err != nil {
		return Resultado{}, err
	}
	mensagem := "Sincronização concluída."
	if len(erros) > 0 {
		total.Sucesso, mensagem = false, strings.Join(erros, " | ")
	}
	total.Mensagem = &mensagem
	return total, nil
}

func (s *Servico) conexoesAtivas(ctx context.Context, conexaoId *int64) ([]int64, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT "Id" FROM "ConexaoBancaria" WHERE "Ativa" AND (? IS NULL OR "Id" = ?) ORDER BY "Id"`, conexaoId, conexaoId)
	if err != nil {
		return nil, fmt.Errorf("ler as conexões ativas: %w", err)
	}
	defer linhas.Close()
	var ids []int64
	for linhas.Next() {
		var id int64
		if err := linhas.Scan(&id); err != nil {
			return nil, fmt.Errorf("ler conexão ativa: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, linhas.Err()
}

func (s *Servico) registrarFalhaDeSessao(ctx context.Context, origem, mensagem string) error {
	agora := s.agora()
	_, err := s.db.ExecContext(ctx, `INSERT INTO "SincronizacaoLog" ("Origem", "IniciadoEm", "FinalizadoEm", "Sucesso", "TransacoesNovas",
		"TransacoesAtualizadas", "Mensagem") VALUES (?, ?, ?, 0, 0, 0, ?)`, origem, agora, agora, limitado(mensagem, 2000))
	return err
}

// rodada é o que uma sincronização carrega de uma conexão para a outra.
type rodada struct {
	*Servico
	sessao     pluggy.Sessao
	regras     *categorizacao.Regras
	descricoes *regrasdescricao.Regras
	origem     string
}

// sincronizarConexao roda uma conexão. A falha fica no log e na conexão, e vira o resultado; erro só no cancelamento.
func (r *rodada) sincronizarConexao(ctx context.Context, conexao int64) (Resultado, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO "SincronizacaoLog" ("ConexaoBancariaId", "Origem", "IniciadoEm", "Sucesso", "TransacoesNovas",
		"TransacoesAtualizadas") VALUES (?, ?, ?, 0, 0, 0)`, conexao, r.origem, r.agora())
	if err != nil {
		return Resultado{}, fmt.Errorf("gravar o log da sincronização: %w", err)
	}
	logId, err := res.LastInsertId()
	if err != nil {
		return Resultado{}, fmt.Errorf("ler o id do log: %w", err)
	}

	novas, atualizadas := 0, 0
	err = r.executar(ctx, conexao, logId, &novas, &atualizadas)
	if err == nil {
		return Resultado{Sucesso: true, TransacoesNovas: novas, TransacoesAtualizadas: atualizadas}, nil
	}
	if ctx.Err() != nil {
		return Resultado{}, ctx.Err()
	}
	r.log.Error("erro ao sincronizar a conexão", "origem", "Open Finance", "conexao", conexao, "erro", err)
	mensagem := "Erro interno: " + err.Error()
	var erroPluggy *pluggy.Erro
	if errors.As(err, &erroPluggy) {
		mensagem = erroPluggy.Mensagem
	}
	mensagem = limitado(mensagem, 2000)
	if _, err := r.db.ExecContext(ctx, `UPDATE "ConexaoBancaria" SET "UltimoErro" = ? WHERE "Id" = ?`, mensagem, conexao); err != nil {
		return Resultado{}, fmt.Errorf("gravar o erro da conexão: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, `UPDATE "SincronizacaoLog" SET "Sucesso" = 0, "FinalizadoEm" = ?, "TransacoesNovas" = ?,
		"TransacoesAtualizadas" = ?, "Mensagem" = ? WHERE "Id" = ?`, r.agora(), novas, atualizadas, mensagem, logId); err != nil {
		return Resultado{}, fmt.Errorf("gravar o log da falha: %w", err)
	}
	return Resultado{TransacoesNovas: novas, TransacoesAtualizadas: atualizadas, Mensagem: &mensagem}, nil
}

// executar é o corpo do try do SincronizarConexaoAsync: novas e atualizadas ficam certas mesmo com o erro no meio.
func (r *rodada) executar(ctx context.Context, conexao, logId int64, novas, atualizadas *int) error {
	var itemId string
	var corAtual *string
	if err := r.db.QueryRowContext(ctx, `SELECT "PluggyItemId", "CorPrimaria" FROM "ConexaoBancaria" WHERE "Id" = ?`, conexao).Scan(&itemId, &corAtual); err != nil {
		return fmt.Errorf("ler a conexão: %w", err)
	}
	item, err := r.pluggy.ObterItem(ctx, r.sessao, itemId)
	if err != nil {
		return err
	}
	cor, err := r.gravarItem(ctx, conexao, item, corAtual)
	if err != nil {
		return err
	}
	diasRetroativos, err := r.config.ObterInteiro(ctx, chaveDiasRetroativos, 30)
	if err != nil {
		return err
	}
	diasPrimeiraCarga, err := r.config.ObterInteiro(ctx, chaveDiasPrimeiraCarga, 365)
	if err != nil {
		return err
	}
	hoje := relogio.Hoje(r.relogio)
	contas, err := r.pluggy.ListarContas(ctx, r.sessao, itemId)
	if err != nil {
		return err
	}
	for _, pc := range contas {
		conta, err := r.gravarConta(ctx, conexao, pc, cor)
		if err != nil {
			return err
		}
		mapaFaturas := map[string]int64{}
		if conta.ehCartao {
			if mapaFaturas, err = r.sincronizarFaturas(ctx, conta); err != nil {
				return err
			}
		}
		primeiraCarga := conta.nova
		if !primeiraCarga {
			var existe int
			err := r.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM "Transacao" WHERE "ContaId" = ?)`, conta.id).Scan(&existe)
			if err != nil {
				return fmt.Errorf("conferir as transações da conta: %w", err)
			}
			primeiraCarga = existe == 0
		}
		dias := diasRetroativos
		if primeiraCarga {
			dias = diasPrimeiraCarga
		}
		n, a, err := r.sincronizarTransacoes(ctx, conta, hoje.SomarDias(-dias), mapaFaturas)
		if err != nil {
			return err
		}
		*novas += n
		*atualizadas += a
	}
	if err := r.sincronizarInvestimentos(ctx, conexao, itemId); err != nil {
		return err
	}
	if err := r.sincronizarEmprestimos(ctx, conexao, itemId); err != nil {
		return err
	}
	if err := r.concluir(ctx, conexao, logId, item, len(contas), *novas, *atualizadas); err != nil {
		return err
	}
	return r.rotinasDoFim(ctx)
}

// gravarItem atualiza a conexão com o que a Pluggy informa e devolve a cor em uso.
func (r *rodada) gravarItem(ctx context.Context, conexao int64, item *pluggy.Item, corAtual *string) (*string, error) {
	var nome, cor *string
	if item.Connector != nil {
		nome, cor = limitar(item.Connector.Name, 150), normalizarCor(item.Connector.PrimaryColor)
	}
	if cor == nil {
		cor = corAtual
	}
	_, err := r.db.ExecContext(ctx, `UPDATE "ConexaoBancaria" SET "NomeInstituicao" = COALESCE(?, "NomeInstituicao"), "CorPrimaria" = ?, "Status" = ?,
		"StatusExecucao" = ?, "UltimaAtualizacaoInstituicao" = ?, "ConsentimentoExpiraEm" = ? WHERE "Id" = ?`,
		nome, cor, limitar(item.Status, 40), limitar(item.ExecutionStatus, 60), dataHoraDe(item.LastUpdatedAt), dataHoraDe(item.ConsentExpiresAt), conexao)
	if err != nil {
		return nil, fmt.Errorf("gravar a conexão: %w", err)
	}
	return cor, nil
}

// concluir grava o fim da conexão e o log de sucesso.
func (r *rodada) concluir(ctx context.Context, conexao, logId int64, item *pluggy.Item, contas, novas, atualizadas int) error {
	var ultimoErro *string
	if item.Error != nil && item.Error.Message != nil && *item.Error.Message != "" {
		texto := limitado("Instituição: "+*item.Error.Message, 2000)
		ultimoErro = &texto
	}
	return banco.EmTransacao(ctx, r.db, func(tx *sql.Tx) error {
		agora := r.agora()
		if _, err := tx.ExecContext(ctx, `UPDATE "ConexaoBancaria" SET "UltimaSincronizacaoEm" = ?, "UltimoErro" = ? WHERE "Id" = ?`, agora, ultimoErro, conexao); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE "SincronizacaoLog" SET "Sucesso" = 1, "FinalizadoEm" = ?, "TransacoesNovas" = ?, "TransacoesAtualizadas" = ?,
			"Mensagem" = ? WHERE "Id" = ?`, agora, novas, atualizadas, strconv.Itoa(contas)+" conta(s) processada(s).", logId)
		return err
	})
}

// rotinasDoFim são as mesmas da partida, na ordem do .NET. O erro nos trocos não desfaz a sincronização: o troco continua
// na conferência.
func (r *rodada) rotinasDoFim(ctx context.Context) error {
	hoje := relogio.Hoje(r.relogio)
	rotinas := []func(*sql.Tx) (int, error){
		func(tx *sql.Tx) (int, error) { return transferencias.Reclassificar(ctx, tx, r.agora()) },
		func(tx *sql.Tx) (int, error) {
			return categorizacao.IgnorarComprasParceladasEstornadas(ctx, tx, hoje, r.agora())
		},
		func(tx *sql.Tx) (int, error) { return cobrancas.Conciliar(ctx, tx, hoje) },
	}
	for _, rotina := range rotinas {
		err := banco.EmTransacao(ctx, r.db, func(tx *sql.Tx) error {
			_, err := rotina(tx)
			return err
		})
		if err != nil {
			return err
		}
	}
	if _, err := r.trocos.ClassificarTrocosSemDuvida(ctx); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		r.avisar("Não deu para classificar os trocos da corretora depois da sincronização.", "erro", err)
	}
	return nil
}

// limitar é o Limitar do C#: corta em n caracteres UTF-16; nil continua nil.
func limitar(valor *string, n int) *string {
	if valor == nil {
		return nil
	}
	cortado := respostas.CortarUtf16(*valor, n)
	return &cortado
}

func limitado(valor string, n int) string { return respostas.CortarUtf16(valor, n) }

func dataHoraDe(i *pluggy.Instante) *tipos.DataHora {
	if i == nil {
		return nil
	}
	d := tipos.NovaDataHora(i.UTC())
	return &d
}

func dataDe(i *pluggy.Instante) *tipos.Data {
	if i == nil {
		return nil
	}
	d := i.Data()
	return &d
}

// arredondar é o Arredondar do C#: (decimal)double e 2 casas, para longe do zero. Sem valor, zero.
func arredondar(n *pluggy.Numero) (tipos.Decimal, error) {
	if n == nil {
		return tipos.Zero, nil
	}
	d, err := n.Decimal()
	if err != nil {
		return tipos.Decimal{}, err
	}
	return tipos.ArredondarLongeDoZero(d, 2), nil
}

func arredondarNulo(n *pluggy.Numero) (*tipos.Decimal, error) {
	if n == nil {
		return nil, nil
	}
	d, err := arredondar(n)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func normalizarMoeda(moeda *string) string {
	if moeda != nil && len([]rune(*moeda)) == 3 {
		return strings.ToUpper(*moeda)
	}
	return "BRL"
}

func normalizarCor(cor *string) *string {
	if cor == nil {
		return nil
	}
	c := strings.TrimLeft(strings.TrimSpace(*cor), "#")
	if len(c) != 6 || strings.IndexFunc(c, func(r rune) bool { return !unicode.Is(unicode.ASCII_Hex_Digit, r) }) >= 0 {
		return nil
	}
	resultado := "#" + strings.ToLower(c)
	return &resultado
}
