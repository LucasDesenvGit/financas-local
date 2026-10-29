// Pacote transacoes tem a Transacao como as regras do .NET a leem e a mudam (categoria, tipo, nome e "fora dos
// relatórios"): a categorização (internal/categorizacao), as regras de descrição (internal/regrasdescricao) e as
// transferências (internal/transferencias). As rotas de /api/transacoes (internal/extrato, #78) mudam também a conta,
// a data, o valor, o status, a observação e a emergência.
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

// Transacao são as colunas de Transacao que as regras e as telas mudam. Valor negativo é saída.
type Transacao struct {
	Id                  int64
	ContaId             int64
	PluggyTransactionId *string // só para ler: veio do Open Finance
	Data                tipos.Data
	Descricao           string
	DescricaoOriginal   *string
	Valor               tipos.Decimal
	Tipo                int64
	Status              int64
	CategoriaId         *int64
	CategoriaManual     bool
	PluggyCategoriaId   *string
	IgnorarRelatorios   bool
	Emergencia          bool
	MeioPagamento       *string
	MesmaTitularidade   *bool
	RegraDescricaoId    *int64
	DescricaoManual     bool
	TransferenciaGrupo  *tipos.Guid
	ParcelaNumero       *int64
	ParcelaTotal        *int64
	Observacao          *string
	AtualizadoEm        tipos.DataHora

	antes *Transacao // como estava no banco, para Gravar mandar só o que mudou
}

const colunas = `"Id", "ContaId", "PluggyTransactionId", "Data", "Descricao", "DescricaoOriginal", "Valor", "Tipo",
	"Status", "CategoriaId", "CategoriaManual", "PluggyCategoriaId", "IgnorarRelatorios", "Emergencia", "MeioPagamento",
	"MesmaTitularidade", "RegraDescricaoId", "DescricaoManual", "TransferenciaGrupo", "ParcelaNumero", "ParcelaTotal",
	"Observacao", "AtualizadoEm"`

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
		if err := linhas.Scan(&t.Id, &t.ContaId, &t.PluggyTransactionId, &t.Data, &t.Descricao, &t.DescricaoOriginal,
			&t.Valor, &t.Tipo, &t.Status, &t.CategoriaId, &t.CategoriaManual, &t.PluggyCategoriaId, &t.IgnorarRelatorios,
			&t.Emergencia, &t.MeioPagamento, &t.MesmaTitularidade, &t.RegraDescricaoId, &t.DescricaoManual, &guid,
			&t.ParcelaNumero, &t.ParcelaTotal, &t.Observacao, &t.AtualizadoEm); err != nil {
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

// mudancas são as colunas que as regras e as telas mudam e que estão diferentes do que veio do banco. O valor compara
// como decimal, como o EF Core: 60 e 60.0 são o mesmo, e o texto no banco fica como está.
func (t *Transacao) mudancas() []mudanca {
	a := t.antes
	if a == nil {
		return nil
	}
	var lista []mudanca
	if t.ContaId != a.ContaId {
		lista = append(lista, mudanca{"ContaId", t.ContaId})
	}
	if !t.Data.Igual(a.Data) {
		lista = append(lista, mudanca{"Data", t.Data})
	}
	if t.Descricao != a.Descricao {
		lista = append(lista, mudanca{"Descricao", t.Descricao})
	}
	if !textosIguais(t.DescricaoOriginal, a.DescricaoOriginal) {
		lista = append(lista, mudanca{"DescricaoOriginal", t.DescricaoOriginal})
	}
	if t.Valor.Decimal().Cmp(a.Valor.Decimal()) != 0 {
		lista = append(lista, mudanca{"Valor", t.Valor})
	}
	if t.Tipo != a.Tipo {
		lista = append(lista, mudanca{"Tipo", t.Tipo})
	}
	if t.Status != a.Status {
		lista = append(lista, mudanca{"Status", t.Status})
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
	if t.Emergencia != a.Emergencia {
		lista = append(lista, mudanca{"Emergencia", t.Emergencia})
	}
	if !iguais(t.RegraDescricaoId, a.RegraDescricaoId) {
		lista = append(lista, mudanca{"RegraDescricaoId", t.RegraDescricaoId})
	}
	if t.DescricaoManual != a.DescricaoManual {
		lista = append(lista, mudanca{"DescricaoManual", t.DescricaoManual})
	}
	if !textosIguais(t.Observacao, a.Observacao) {
		lista = append(lista, mudanca{"Observacao", t.Observacao})
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

func textosIguais(a, b *string) bool {
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
