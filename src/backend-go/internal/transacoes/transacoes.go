// Pacote transacoes tem a Transacao como as regras do .NET a leem e a mudam (categoria, tipo, nome e "fora dos
// relatórios"): a categorização (internal/categorizacao), as regras de descrição (internal/regrasdescricao) e as
// transferências (internal/transferencias). As rotas de /api/transacoes vêm na #78 e crescem a partir daqui.
//
// Gravar faz o que o SaveChanges do EF Core faz com uma entidade alterada: um UPDATE só das colunas que mudaram.
package transacoes

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Tipos de transação (o enum TipoTransacao do C#).
const (
	Despesa       = 1
	Receita       = 2
	Transferencia = 3
)

// Transacao são as colunas de Transacao que as regras usam. Valor negativo é saída.
type Transacao struct {
	Id                 int64
	ContaId            int64
	Data               tipos.Data
	Descricao          string
	DescricaoOriginal  *string
	Valor              tipos.Decimal
	Tipo               int64
	CategoriaId        *int64
	CategoriaManual    bool
	PluggyCategoriaId  *string
	IgnorarRelatorios  bool
	MeioPagamento      *string
	MesmaTitularidade  *bool
	RegraDescricaoId   *int64
	DescricaoManual    bool
	TransferenciaGrupo *tipos.Guid
	ParcelaNumero      *int64
	ParcelaTotal       *int64
	AtualizadoEm       tipos.DataHora

	antes *Transacao // como estava no banco, para Gravar mandar só o que mudou
}

const colunas = `"Id", "ContaId", "Data", "Descricao", "DescricaoOriginal", "Valor", "Tipo", "CategoriaId",
	"CategoriaManual", "PluggyCategoriaId", "IgnorarRelatorios", "MeioPagamento", "MesmaTitularidade",
	"RegraDescricaoId", "DescricaoManual", "TransferenciaGrupo", "ParcelaNumero", "ParcelaTotal", "AtualizadoEm"`

// Listar lê as transações de uma consulta. onde é um trecho de SQL escrito no código (nunca texto de quem usa), com os
// valores em argumentos "?": "WHERE "CategoriaId" = ?".
func Listar(ctx context.Context, q consultas.DBTX, onde string, argumentos ...any) ([]*Transacao, error) {
	linhas, err := q.QueryContext(ctx, `SELECT `+colunas+` FROM "Transacao" `+onde, argumentos...)
	if err != nil {
		return nil, fmt.Errorf("ler transações: %w", err)
	}
	defer linhas.Close()
	lista := []*Transacao{}
	for linhas.Next() {
		var t Transacao
		var guid sql.Null[tipos.Guid]
		if err := linhas.Scan(&t.Id, &t.ContaId, &t.Data, &t.Descricao, &t.DescricaoOriginal, &t.Valor, &t.Tipo,
			&t.CategoriaId, &t.CategoriaManual, &t.PluggyCategoriaId, &t.IgnorarRelatorios, &t.MeioPagamento,
			&t.MesmaTitularidade, &t.RegraDescricaoId, &t.DescricaoManual, &guid, &t.ParcelaNumero, &t.ParcelaTotal,
			&t.AtualizadoEm); err != nil {
			return nil, fmt.Errorf("ler transação: %w", err)
		}
		if guid.Valid {
			t.TransferenciaGrupo = &guid.V
		}
		copia := t
		t.antes = &copia
		lista = append(lista, &t)
	}
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("ler transações: %w", err)
	}
	return lista, nil
}

// Mudou diz se alguma coluna gravável mudou desde a leitura.
func (t *Transacao) Mudou() bool { return len(t.mudancas()) > 0 }

// mudancas são as colunas que as regras mudam e que estão diferentes do que veio do banco.
func (t *Transacao) mudancas() []mudanca {
	a := t.antes
	if a == nil {
		return nil
	}
	var lista []mudanca
	if t.Descricao != a.Descricao {
		lista = append(lista, mudanca{"Descricao", t.Descricao})
	}
	if t.Tipo != a.Tipo {
		lista = append(lista, mudanca{"Tipo", t.Tipo})
	}
	if !iguais(t.CategoriaId, a.CategoriaId) {
		lista = append(lista, mudanca{"CategoriaId", t.CategoriaId})
	}
	if t.CategoriaManual != a.CategoriaManual {
		lista = append(lista, mudanca{"CategoriaManual", t.CategoriaManual})
	}
	if t.IgnorarRelatorios != a.IgnorarRelatorios {
		lista = append(lista, mudanca{"IgnorarRelatorios", t.IgnorarRelatorios})
	}
	if !iguais(t.RegraDescricaoId, a.RegraDescricaoId) {
		lista = append(lista, mudanca{"RegraDescricaoId", t.RegraDescricaoId})
	}
	if t.AtualizadoEm.String() != a.AtualizadoEm.String() {
		lista = append(lista, mudanca{"AtualizadoEm", t.AtualizadoEm})
	}
	return lista
}

type mudanca struct {
	coluna string
	valor  any
}

func iguais(a, b *int64) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

// Gravar manda ao banco só as colunas que mudaram (o SaveChanges do EF Core) e deixa a transação como gravada.
func Gravar(ctx context.Context, q consultas.DBTX, t *Transacao) error {
	lista := t.mudancas()
	if len(lista) == 0 {
		return nil
	}
	partes := make([]string, len(lista))
	argumentos := make([]any, 0, len(lista)+1)
	for i, m := range lista {
		partes[i] = `"` + m.coluna + `" = ?`
		argumentos = append(argumentos, m.valor)
	}
	argumentos = append(argumentos, t.Id)
	if _, err := q.ExecContext(ctx, `UPDATE "Transacao" SET `+strings.Join(partes, ", ")+` WHERE "Id" = ?`, argumentos...); err != nil {
		return fmt.Errorf("gravar a transação %d: %w", t.Id, err)
	}
	copia := *t
	copia.antes = nil
	t.antes = &copia
	return nil
}

// GravarTodas grava as que mudaram, na ordem da lista.
func GravarTodas(ctx context.Context, q consultas.DBTX, lista []*Transacao) error {
	for _, t := range lista {
		if err := Gravar(ctx, q, t); err != nil {
			return err
		}
	}
	return nil
}
