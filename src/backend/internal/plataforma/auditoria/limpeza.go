package auditoria

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

const (
	// ChaveDiasRetencao é o parâmetro do prazo da auditoria de acesso (#530), na Parametrização.
	ChaveDiasRetencao = "Seguranca.DiasRetencaoAuditoria"
	// intervaloDeLimpeza: a limpeza roda ao subir e depois uma vez por dia, como a dos erros do sistema.
	intervaloDeLimpeza = 24 * time.Hour
)

// Parametros é o que a limpeza usa da Parametrização: só o prazo. A interface é de quem usa.
type Parametros interface {
	ObterInteiro(ctx context.Context, chave string) (int, error)
}

// Limpeza apaga os acessos mais antigos que o prazo da Parametrização (#530). A tabela só crescia: cada entrada, saída
// e troca de senha fica nela, com o IP e o navegador, e a tela de Segurança lista sem apagar.
type Limpeza struct {
	db         *sql.DB
	parametros Parametros
	relogio    relogio.Relogio
	log        *slog.Logger
}

// NovaLimpeza monta a limpeza com o banco, a Parametrização, o relógio e o log.
func NovaLimpeza(db *sql.DB, p Parametros, r relogio.Relogio, log *slog.Logger) *Limpeza {
	return &Limpeza{db: db, parametros: p, relogio: r, log: log}
}

// Rodar limpa ao subir e depois a cada 24 horas, até o contexto acabar. Erro vira aviso no log (e em Configurações ›
// Erros) e a próxima volta tenta de novo.
func (l *Limpeza) Rodar(ctx context.Context) {
	for {
		if _, err := l.Limpar(ctx); err != nil && ctx.Err() == nil {
			l.log.Warn("Não foi possível apagar os acessos antigos da auditoria.", "origem", "Sistema", "erro", err)
		}
		espera := time.NewTimer(intervaloDeLimpeza)
		select {
		case <-ctx.Done():
			espera.Stop()
			return
		case <-espera.C:
		}
	}
}

// Limpar apaga os acessos com a hora (UTC) de antes do prazo e devolve quantos apagou. A hora é texto no formato do EF
// Core, sempre com o mesmo tamanho, então comparar o texto é comparar a hora (como a limpeza dos erros).
func (l *Limpeza) Limpar(ctx context.Context) (int64, error) {
	dias, err := l.parametros.ObterInteiro(ctx, ChaveDiasRetencao)
	if err != nil {
		return 0, err
	}
	limite := l.relogio.Agora().Add(-time.Duration(dias) * 24 * time.Hour)
	r, err := l.db.ExecContext(ctx, `DELETE FROM "AuditoriaAcesso" WHERE "DataHora" < ?`, tipos.NovaDataHora(limite))
	if err != nil {
		return 0, fmt.Errorf("apagar os acessos antigos: %w", err)
	}
	return r.RowsAffected()
}
