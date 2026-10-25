package erros

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

const (
	maximoPorLote      = 100
	janelaAgrupamento  = 24 * time.Hour
	esperaMaxima       = time.Hour // acorda pelo menos de hora em hora, para a limpeza diária rodar mesmo sem erros
	prazoNoDesligar    = 5 * time.Second
	chaveDiasRetencao  = "Diagnostico.DiasRetencaoErros"
	intervaloDeLimpeza = 24 * time.Hour
)

// Parametros é o que o gravador usa do pacote parametros (declarado aqui, por quem usa).
type Parametros interface {
	ObterInteiro(ctx context.Context, chave string) (int, error)
}

// Gravador é o GravadorErrosHostedService: grava no banco os registros da fila, já limpos, e apaga os antigos uma vez
// por dia. A mesma ocorrência (assinatura) ainda em aberto e vista nas últimas 24 horas só soma +1 em Ocorrencias;
// depois de resolvida, se acontecer de novo, vira uma linha nova.
type Gravador struct {
	db            *sql.DB
	fila          *Fila
	relogio       relogio.Relogio
	parametros    Parametros
	log           *slog.Logger // o log sem a captura para a tabela (o próximo handler do Handler)
	ultimaLimpeza time.Time
}

func NovoGravador(db *sql.DB, fila *Fila, r relogio.Relogio, p Parametros, log *slog.Logger) *Gravador {
	return &Gravador{db: db, fila: fila, relogio: r, parametros: p, log: log}
}

// Rodar grava até o contexto acabar. Ao parar, grava mais um lote do que estiver na fila (até 5 s), sem esperar por
// mais, e só então volta: quem chama espera o Rodar terminar antes de fechar o banco.
func (g *Gravador) Rodar(ctx context.Context) {
	for {
		espera := time.NewTimer(esperaMaxima)
		select {
		case <-ctx.Done():
			espera.Stop()
			prazo, cancelar := context.WithTimeout(context.Background(), prazoNoDesligar)
			g.gravarLote(prazo, g.lerLote(nil))
			cancelar()
			return
		case primeiro := <-g.fila.canal:
			espera.Stop()
			g.gravarLote(ctx, g.lerLote([]Registro{primeiro}))
		case <-espera.C:
		}
		g.limparSeVenceu(ctx)
	}
}

// lerLote completa o lote com o que já está na fila, até 100, sem esperar.
func (g *Gravador) lerLote(lote []Registro) []Registro {
	for len(lote) < maximoPorLote {
		select {
		case r := <-g.fila.canal:
			lote = append(lote, r)
		default:
			return lote
		}
	}
	return lote
}

// gravarLote grava o lote numa transação. Erro ao gravar vai só para o log, e o lote é descartado (não volta para a
// fila).
func (g *Gravador) gravarLote(ctx context.Context, lote []Registro) {
	if len(lote) == 0 {
		return
	}
	err := banco.EmTransacao(ctx, g.db, func(tx *sql.Tx) error {
		var linhas []*linha
		for _, r := range lote {
			var err error
			if linhas, err = agrupar(ctx, tx, linhas, r); err != nil {
				return err
			}
		}
		for _, l := range linhas {
			if err := l.salvar(ctx, tx); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		g.log.Error(fmt.Sprintf("Não foi possível gravar %d erro(s) na tabela de erros.", len(lote)), "erro", err)
	}
}

// linha é uma linha de ErroSistema que o lote criou ou mudou (o ChangeTracker do EF).
type linha struct {
	id                     int64 // 0: nova
	primeiraEm, ultimaEm   time.Time
	ocorrencias            int
	nivel                  Nivel
	origem, categoria      string
	mensagem               string
	detalhe, rota, usuario *string
	assinatura             string
	alterada               bool
}

// agrupar é o GravarAsync: procura a mesma ocorrência em aberto vista nas últimas 24 h, primeiro no que o lote já
// tem (ainda não salvo) e depois no banco; achou, soma; senão, cria.
func agrupar(ctx context.Context, tx *sql.Tx, linhas []*linha, r Registro) ([]*linha, error) {
	mensagem := *LimparECortar(&r.Mensagem, 1000)
	detalhe := LimparECortar(r.Detalhe, 8000)
	assinatura := Assinatura(r.Categoria, mensagem, detalhe)
	desde := r.Em.Add(-janelaAgrupamento)

	var existente *linha
	for _, l := range linhas {
		if l.assinatura == assinatura && !l.ultimaEm.Before(desde) {
			existente = l
			break
		}
	}
	if existente == nil {
		l, err := buscarEmAberto(ctx, tx, assinatura, desde)
		if err != nil {
			return nil, err
		}
		if l != nil {
			existente = l
			linhas = append(linhas, l)
		}
	}
	if existente != nil {
		existente.ocorrencias++
		existente.ultimaEm = r.Em
		existente.nivel = max(existente.nivel, r.Nivel)
		if rota := LimparECortar(r.Rota, 300); rota != nil {
			existente.rota = rota
		}
		if r.Usuario != nil {
			existente.usuario = LimparECortar(r.Usuario, 100)
		}
		existente.alterada = true
		return linhas, nil
	}
	return append(linhas, &linha{
		primeiraEm: r.Em, ultimaEm: r.Em, ocorrencias: 1, nivel: r.Nivel,
		origem: respostas.CortarUtf16(r.Origem, 30), categoria: respostas.CortarUtf16(r.Categoria, 200),
		mensagem: mensagem, detalhe: detalhe, rota: LimparECortar(r.Rota, 300), usuario: LimparECortar(r.Usuario, 100),
		assinatura: assinatura,
	}), nil
}

// buscarEmAberto é a ocorrência mais recente em aberto da assinatura, vista desde o instante dado.
func buscarEmAberto(ctx context.Context, tx *sql.Tx, assinatura string, desde time.Time) (*linha, error) {
	l := &linha{assinatura: assinatura}
	var primeira, ultima tipos.DataHora
	err := tx.QueryRowContext(ctx, `SELECT "Id", "PrimeiraEm", "UltimaEm", "Ocorrencias", "Nivel", "Origem", "Categoria",
		"Mensagem", "Detalhe", "Rota", "Usuario"
		FROM "ErroSistema"
		WHERE "Assinatura" = ? AND "Resolvido" = 0 AND "UltimaEm" >= ?
		ORDER BY "UltimaEm" DESC
		LIMIT 1`, assinatura, tipos.NovaDataHora(desde)).
		Scan(&l.id, &primeira, &ultima, &l.ocorrencias, &l.nivel, &l.origem, &l.categoria, &l.mensagem, &l.detalhe, &l.rota, &l.usuario)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("procurar a ocorrência em aberto: %w", err)
	}
	l.primeiraEm, l.ultimaEm = primeira.Tempo(), ultima.Tempo()
	return l, nil
}

func (l *linha) salvar(ctx context.Context, tx *sql.Tx) error {
	if l.id == 0 {
		_, err := tx.ExecContext(ctx, `INSERT INTO "ErroSistema" ("PrimeiraEm", "UltimaEm", "Ocorrencias", "Nivel", "Origem",
			"Categoria", "Mensagem", "Detalhe", "Rota", "Usuario", "Assinatura", "Resolvido", "ResolvidoEm")
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, NULL)`,
			tipos.NovaDataHora(l.primeiraEm), tipos.NovaDataHora(l.ultimaEm), l.ocorrencias, int(l.nivel), l.origem,
			l.categoria, l.mensagem, l.detalhe, l.rota, l.usuario, l.assinatura)
		if err != nil {
			return fmt.Errorf("gravar o erro: %w", err)
		}
		return nil
	}
	if !l.alterada {
		return nil
	}
	_, err := tx.ExecContext(ctx, `UPDATE "ErroSistema" SET "UltimaEm" = ?, "Ocorrencias" = ?, "Nivel" = ?, "Rota" = ?, "Usuario" = ?
		WHERE "Id" = ?`, tipos.NovaDataHora(l.ultimaEm), l.ocorrencias, int(l.nivel), l.rota, l.usuario, l.id)
	if err != nil {
		return fmt.Errorf("somar a ocorrência %d: %w", l.id, err)
	}
	return nil
}

// Assinatura é categoria + mensagem + tipo do erro (primeira linha do detalhe, até o ":"): os 16 primeiros bytes do
// SHA-256, em hexadecimal maiúsculo, como o Convert.ToHexString(bytes, 0, 16) do C#.
func Assinatura(categoria, mensagem string, detalhe *string) string {
	tipo := ""
	if detalhe != nil {
		primeira, _, _ := strings.Cut(*detalhe, "\n")
		antes, _, _ := strings.Cut(primeira, ":")
		tipo = strings.TrimSpace(antes)
	}
	soma := sha256.Sum256([]byte(categoria + "\n" + mensagem + "\n" + tipo))
	return strings.ToUpper(hex.EncodeToString(soma[:16]))
}

// limparSeVenceu apaga, uma vez por dia, os erros cuja última ocorrência passou do prazo da Parametrização
// (Diagnostico.DiasRetencaoErros). Roda já na primeira volta, como o .NET.
func (g *Gravador) limparSeVenceu(ctx context.Context) {
	agora := g.relogio.Agora()
	if !g.ultimaLimpeza.IsZero() && agora.Sub(g.ultimaLimpeza) < intervaloDeLimpeza {
		return
	}
	g.ultimaLimpeza = agora
	dias, err := g.parametros.ObterInteiro(ctx, chaveDiasRetencao)
	if err == nil {
		limite := agora.Add(-time.Duration(dias) * 24 * time.Hour)
		_, err = g.db.ExecContext(ctx, `DELETE FROM "ErroSistema" WHERE "UltimaEm" < ?`, tipos.NovaDataHora(limite))
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		g.log.Error("Não foi possível apagar os erros antigos.", "erro", err)
	}
}
