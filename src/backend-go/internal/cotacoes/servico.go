package cotacoes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cambio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/configuracao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/segredos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

const (
	// ChaveTokenBrapi é o token da brapi na Configuracao.
	ChaveTokenBrapi = "Cotacao.BrapiToken"
	// CodigoIfix é o índice dos FIIs.
	CodigoIfix = "IFIX"

	tipoFii, tipoAcao = 1, 2
	tamanhoDoNome     = 150
	casasDoPreco      = 4
	// pausa é a gentileza com a fonte gratuita, entre um ativo e outro.
	pausa = 250 * time.Millisecond
	// validadeDoIndice: o histórico do índice é consultado no máximo uma vez nesse tempo.
	validadeDoIndice = 12 * time.Hour
	// diasDeFolgaAntes e diasDeFolgaDepois: a margem dos fechamentos diários em volta do período pedido.
	diasDeFolgaAntes, diasDeFolgaDepois = 7, 2
)

// simbolosDoIfix: o Yahoo já publicou o IFIX com símbolos diferentes; tenta os conhecidos.
var simbolosDoIfix = []string{"IFIX.SA", "^IFIX"}

// fonte é o que o serviço usa das fontes. A interface é de quem usa: a de verdade é o Cliente.
type fonte interface {
	Yahoo(ctx context.Context, simbolo string, primeiraOperacao tipos.Data, anoDeHoje int) (*Obtida, error)
	Brapi(ctx context.Context, ticker, token string) (*Obtida, error)
	MensaisDoIndice(ctx context.Context, simbolo string) ([]Fechamento, error)
	FechamentosDiarios(ctx context.Context, ticker string, inicio, fim int64) ([]Fechamento, error)
}

// historico é o que as cotações usam do câmbio.
type historico interface {
	GarantirHistorico(ctx context.Context, moeda string, desde tipos.Data) error
}

// Servico é o CotacaoService. O cofre é nil até a troca dos segredos (#85, ADR 0049): sem ele, não há token, e a brapi fica
// de fora.
type Servico struct {
	db      *sql.DB
	config  *configuracao.Servico
	fonte   fonte
	cambio  historico
	cofre   configuracao.Cofre
	relogio relogio.Relogio
	log     *slog.Logger
	esperar func(ctx context.Context, d time.Duration) error // a pausa entre ativos; nos testes, nada
}

// NovoServico monta o serviço com as dependências.
func NovoServico(db *sql.DB, f fonte, c historico, cofre configuracao.Cofre, r relogio.Relogio, log *slog.Logger) *Servico {
	return &Servico{db: db, config: configuracao.Novo(db, r), fonte: f, cambio: c, cofre: cofre, relogio: r, log: log, esperar: esperarComContexto}
}

func esperarComContexto(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// avisar registra o aviso com a origem e o texto do C#: a tela de erros soma pelo texto, e um aviso por ativo fica uma
// linha por ativo.
func (s *Servico) avisar(msg string, args ...any) {
	s.log.Warn(msg, append([]any{"origem", "Investimentos"}, args...)...)
}

// Resultado é o ResultadoCotacoes.
type Resultado struct {
	Atualizados    int      `json:"atualizados"`
	ProventosNovos int      `json:"proventosNovos"`
	Falhas         []string `json:"falhas"`
}

// ativo é o que a atualização precisa de um Ativo.
type ativo struct {
	id                int64
	ticker, moeda     string
	nome              *string
	tipo              int64
	manual            bool
	primeiraOperacao  *tipos.Data
	precoAtual        *tipos.Decimal
	precoAnterior     *tipos.Decimal
	precoAtualizadoEm *tipos.DataHora
	fonteCotacao      *string
}

func (s *Servico) lerAtivos(ctx context.Context, somente []int64) ([]ativo, error) {
	consulta := `SELECT a."Id", a."Ticker", a."Moeda", a."Nome", a."Tipo", a."PrecoManual", a."PrecoAtual", a."PrecoAnterior",
		a."PrecoAtualizadoEm", a."FonteCotacao", (SELECT MIN(o."Data") FROM "OperacaoAtivo" AS o WHERE o."AtivoId" = a."Id")
		FROM "Ativo" AS a`
	var argumentos []any
	if somente != nil {
		consulta += ` WHERE a."Id" IN (` + strings.TrimSuffix(strings.Repeat("?, ", len(somente)), ", ") + `)`
		for _, id := range somente {
			argumentos = append(argumentos, id)
		}
	}
	linhas, err := s.db.QueryContext(ctx, consulta+` ORDER BY a."Id"`, argumentos...)
	if err != nil {
		return nil, fmt.Errorf("ler os ativos: %w", err)
	}
	defer linhas.Close()
	var ativos []ativo
	for linhas.Next() {
		var a ativo
		var primeira sql.Null[tipos.Data]
		if err := linhas.Scan(&a.id, &a.ticker, &a.moeda, &a.nome, &a.tipo, &a.manual, &a.precoAtual, &a.precoAnterior,
			&a.precoAtualizadoEm, &a.fonteCotacao, &primeira); err != nil {
			return nil, fmt.Errorf("ler ativo: %w", err)
		}
		if primeira.Valid {
			a.primeiraOperacao = &primeira.V
		}
		ativos = append(ativos, a)
	}
	return ativos, linhas.Err()
}

// token é o token da brapi. O gravado pelo .NET não se lê até a troca dos segredos: sem ele, só não há brapi.
func (s *Servico) token(ctx context.Context) (string, error) {
	if s.cofre == nil {
		return "", nil
	}
	t, err := s.config.ObterSegredo(ctx, s.cofre, ChaveTokenBrapi)
	if errors.Is(err, segredos.ErrInformarDeNovo) {
		s.avisar("O token da brapi precisa ser informado de novo.")
		return "", nil
	}
	if err != nil || t == nil || strings.TrimFunc(*t, unicode.IsSpace) == "" {
		return "", err
	}
	return *t, nil
}

// consultar pede a cotação às fontes: o Yahoo; se ele vier na moeda errada, nada (comparação exata de propósito: o "GBp",
// em pence, não pode passar como GBP); na B3, sem preço, a brapi.
func (s *Servico) consultar(ctx context.Context, a *ativo, token string, hoje tipos.Data) (*Obtida, error) {
	exterior := a.moeda != cambio.MoedaBase
	primeira := hoje
	if a.primeiraOperacao != nil {
		primeira = *a.primeiraOperacao
	}
	simbolo := a.ticker
	if !exterior {
		simbolo += ".SA"
	}
	cotacao, err := s.fonte.Yahoo(ctx, simbolo, primeira, hoje.Ano())
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		s.avisar(fmt.Sprintf("Falha ao consultar %s no Yahoo Finance.", a.ticker), "erro", err)
		cotacao = nil
	}
	if cotacao != nil && cotacao.Moeda != nil && *cotacao.Moeda != a.moeda {
		s.avisar(fmt.Sprintf("Cotação de %s veio em %s, mas o ativo está em %s; ignorada.", a.ticker, *cotacao.Moeda, a.moeda))
		cotacao = nil
	}
	if exterior {
		if err := s.cambio.GarantirHistorico(ctx, a.moeda, primeira); err != nil {
			return nil, err
		}
	}
	if (cotacao == nil || cotacao.Preco == nil) && !exterior && token != "" {
		pela, err := s.fonte.Brapi(ctx, a.ticker, token)
		switch {
		case ctx.Err() != nil:
			return nil, ctx.Err()
		case err != nil:
			s.avisar(fmt.Sprintf("Falha ao consultar %s na brapi.", a.ticker), "erro", err)
		case pela != nil:
			cotacao = pela
		}
	}
	return cotacao, nil
}

// aplicar aplica ao ativo o nome e o preço da cotação. Diz se o preço foi atualizado e se conta como falha. O FII que veio
// do Open Finance como "STOCK" e sem emissor (ex.: GARE11): o nome oficial corrige o tipo.
func (s *Servico) aplicar(a *ativo, c *Obtida) (atualizou, falhou bool) {
	semNome := a.nome == nil || strings.TrimFunc(*a.nome, unicode.IsSpace) == ""
	if semNome && c.Nome != nil && strings.TrimFunc(*c.Nome, unicode.IsSpace) != "" {
		nome := respostas.CortarUtf16(*c.Nome, tamanhoDoNome)
		a.nome = &nome
		if a.tipo == tipoAcao && a.moeda == cambio.MoedaBase && PareceFii(&nome) {
			a.tipo = tipoFii
		}
	}
	if c.Preco == nil {
		return false, true
	}
	if a.manual || !c.Preco.Decimal().IsPositive() {
		return false, false
	}
	preco := tipos.Arredondar(*c.Preco, casasDoPreco)
	a.precoAtual, a.precoAnterior = &preco, nil
	if c.PrecoAnterior != nil && c.PrecoAnterior.Decimal().IsPositive() {
		anterior := tipos.Arredondar(*c.PrecoAnterior, casasDoPreco)
		a.precoAnterior = &anterior
	}
	agora := tipos.NovaDataHora(s.relogio.Agora())
	a.precoAtualizadoEm, a.fonteCotacao = &agora, &c.Fonte
	return true, false
}

// gravar grava de uma vez o ativo, os proventos novos e os fechamentos mensais; devolve quantos proventos entraram.
func (s *Servico) gravar(ctx context.Context, a *ativo, c *Obtida) (int, error) {
	novos := 0
	err := banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE "Ativo" SET "Nome" = ?, "Tipo" = ?, "PrecoAtual" = ?, "PrecoAnterior" = ?,
			"PrecoAtualizadoEm" = ?, "FonteCotacao" = ? WHERE "Id" = ?`, a.nome, a.tipo, a.precoAtual, a.precoAnterior, a.precoAtualizadoEm,
			a.fonteCotacao, a.id); err != nil {
			return fmt.Errorf("gravar o ativo: %w", err)
		}
		var err error
		if novos, err = gravarProventos(ctx, tx, a.id, c.Proventos); err != nil {
			return err
		}
		return gravarMensais(ctx, tx, `"PrecoMensalAtivo"`, `"AtivoId"`, a.id, c.Mensais)
	})
	return novos, err
}

func gravarProventos(ctx context.Context, tx *sql.Tx, id int64, proventos []Provento) (int, error) {
	if len(proventos) == 0 {
		return 0, nil
	}
	linhas, err := tx.QueryContext(ctx, `SELECT "DataCom" FROM "ProventoAtivo" WHERE "AtivoId" = ?`, id)
	if err != nil {
		return 0, fmt.Errorf("ler os proventos: %w", err)
	}
	defer linhas.Close()
	existentes := map[string]bool{}
	for linhas.Next() {
		var d tipos.Data
		if err := linhas.Scan(&d); err != nil {
			return 0, fmt.Errorf("ler provento: %w", err)
		}
		existentes[d.String()] = true
	}
	if err := linhas.Err(); err != nil {
		return 0, fmt.Errorf("ler os proventos: %w", err)
	}
	novos := 0
	for _, p := range proventos {
		if !p.Valor.Decimal().IsPositive() || existentes[p.DataCom.String()] {
			continue
		}
		existentes[p.DataCom.String()] = true
		if _, err := tx.ExecContext(ctx, `INSERT INTO "ProventoAtivo" ("AtivoId", "DataCom", "ValorPorCota") VALUES (?, ?, ?)`, id, p.DataCom, p.Valor); err != nil {
			return 0, fmt.Errorf("gravar o provento: %w", err)
		}
		novos++
	}
	return novos, nil
}

// gravarMensais grava os fechamentos da tabela, pela chave dada (o ativo, ou o código do índice): o do mês que já existe é
// corrigido.
func gravarMensais(ctx context.Context, tx *sql.Tx, tabela, coluna string, chave any, mensais []Fechamento) error {
	for _, f := range mensais {
		if !f.Valor.Decimal().IsPositive() {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO `+tabela+` (`+coluna+`, "Mes", "Fechamento") VALUES (?, ?, ?)
			ON CONFLICT DO UPDATE SET "Fechamento" = excluded."Fechamento"`, chave, f.Data, tipos.Arredondar(f.Valor, casasDoPreco)); err != nil {
			return fmt.Errorf("gravar o fechamento: %w", err)
		}
	}
	return nil
}

// atualizarIndice é o AtualizarIndiceAsync: o histórico mensal de um índice, no máximo uma vez a cada 12 horas.
func (s *Servico) atualizarIndice(ctx context.Context, codigo string) error {
	chave := "Indice." + codigo + ".ConsultadoEm"
	consultado, err := s.config.ObterData(ctx, chave)
	if err != nil {
		return err
	}
	agora := s.relogio.Agora()
	if consultado != nil && agora.Sub(consultado.Tempo()) < validadeDoIndice {
		return nil
	}
	var mensais []Fechamento
	for _, simbolo := range simbolosDoIfix {
		if mensais, err = s.fonte.MensaisDoIndice(ctx, simbolo); err != nil {
			return err
		}
		if len(mensais) > 0 {
			break
		}
	}
	return banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		if err := s.config.ComTx(tx).DefinirData(ctx, chave, agora); err != nil {
			return err
		}
		return gravarMensais(ctx, tx, `"IndiceMensal"`, `"Codigo"`, codigo, mensais)
	})
}

// Atualizar é o AtualizarAsync: as cotações e os proventos de todos os ativos (somente nil) ou dos pedidos, um por vez,
// e o IFIX no fim. O erro de uma fonte vira aviso e fica em Falhas; o do banco, ou o pedido cancelado, para tudo.
func (s *Servico) Atualizar(ctx context.Context, somente []int64) (Resultado, error) {
	r := Resultado{Falhas: []string{}}
	ativos, err := s.lerAtivos(ctx, somente)
	if err != nil {
		return r, err
	}
	token, err := s.token(ctx)
	if err != nil {
		return r, err
	}
	hoje := relogio.Hoje(s.relogio)
	for i := range ativos {
		a := &ativos[i]
		if err := ctx.Err(); err != nil {
			return r, err
		}
		cotacao, err := s.consultar(ctx, a, token, hoje)
		if err != nil {
			return r, err
		}
		if cotacao == nil {
			r.Falhas = append(r.Falhas, a.ticker)
			continue
		}
		atualizou, falhou := s.aplicar(a, cotacao)
		if atualizou {
			r.Atualizados++
		}
		if falhou {
			r.Falhas = append(r.Falhas, a.ticker)
		}
		novos, err := s.gravar(ctx, a, cotacao)
		if err != nil {
			return r, err
		}
		r.ProventosNovos += novos
		if err := s.esperar(ctx, pausa); err != nil {
			return r, err
		}
	}
	if len(ativos) > 0 {
		if err := s.atualizarIndice(ctx, CodigoIfix); err != nil {
			if ctx.Err() != nil {
				return r, ctx.Err()
			}
			s.avisar("Não foi possível atualizar o IFIX.", "erro", err)
		}
	}
	return r, nil
}

// FechamentosDiarios é o FechamentosDiariosAsync: os fechamentos do ativo entre as datas, com uma folga em volta, para
// descobrir o preço da cota no dia de uma compra. Se a fonte não responder, vazio.
func (s *Servico) FechamentosDiarios(ctx context.Context, ticker string, de, ate tipos.Data) ([]Fechamento, error) {
	segundos := func(d tipos.Data) int64 { return time.Date(d.Ano(), d.Mes(), d.Dia(), 0, 0, 0, 0, time.UTC).Unix() }
	serie, err := s.fonte.FechamentosDiarios(ctx, ticker, segundos(de.SomarDias(-diasDeFolgaAntes)), segundos(ate.SomarDias(diasDeFolgaDepois)))
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		s.avisar(fmt.Sprintf("Falha ao consultar o histórico de %s.", ticker), "erro", err)
		return nil, nil
	}
	return serie, nil
}

// palavrasDo texto são os pedaços entre o que não é letra, dígito, marca ou conector: o \b do .NET é de Unicode, e o do
// RE2, só de ASCII.
func palavrasDo(t string) []string {
	return strings.FieldsFunc(t, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.Is(unicode.Mn, r) && !unicode.Is(unicode.Nd, r) && !unicode.Is(unicode.Pc, r)
	})
}

// PareceFii é o ClassificacaoAtivo.PareceFii: "FII"/"Fiagro" no texto, ou fundo + imobiliário ("FDO INV IMOB"). Fundo de
// índice (ETF) com "imobiliário" no nome não conta.
func PareceFii(nomeOuEmissor *string) bool {
	if nomeOuEmissor == nil {
		return false
	}
	normalizado := texto.ParaRegras(*nomeOuEmissor)
	sigla, fundo := false, false
	for _, p := range palavrasDo(normalizado) {
		switch p {
		case "INDICE", "ETF":
			return false
		case "FII", "FIAGRO":
			sigla = true
		case "FUNDO", "FUNDOS", "FDO", "FUND":
			fundo = true
		}
	}
	return sigla || (fundo && strings.Contains(normalizado, "IMOB"))
}
