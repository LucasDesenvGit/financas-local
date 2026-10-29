package relatorios

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Recorrência de uma conta a pagar (o enum Recorrencia do C#).
const (
	RecorrenciaUnica  = 0
	RecorrenciaMensal = 1
	RecorrenciaAnual  = 2
)

// FormaCartaoCredito é a FormaCobranca "cartão de crédito" (#158, ADR 0043): a cobrança cai na fatura, que já está no
// painel.
const FormaCartaoCredito = 3

// maiorNumeroDoDia é o DateOnly.MaxDayNumber do C#: o número do dia 31/12/9999.
const maiorNumeroDoDia = 3652058

// maximoDePagamentos é o limite do laço de PagamentosAte (uma conta mensal por 100 anos).
const maximoDePagamentos = 1200

// ErroDataForaDoIntervalo é a ArgumentOutOfRangeException que o DateOnly.AddDays do C# lança quando a data de chegada
// sai do ano 1 a 9999: lá vira o 500 do servidor.
var ErroDataForaDoIntervalo = errors.New("a data sai do intervalo de 1 a 9999")

// ContaAgendadaDto é uma conta a pagar com o próximo vencimento, na ordem do record ContaAgendadaDto do C#.
type ContaAgendadaDto struct {
	Id                  int64         `json:"id"`
	Descricao           string        `json:"descricao"`
	Valor               tipos.Decimal `json:"valor"`
	ProximoVencimento   tipos.Data    `json:"proximoVencimento"`
	Recorrencia         int64         `json:"recorrencia"`
	CategoriaId         *int64        `json:"categoriaId"`
	CategoriaNome       *string       `json:"categoriaNome"`
	ContaId             *int64        `json:"contaId"`
	ContaNome           *string       `json:"contaNome"`
	Ativa               bool          `json:"ativa"`
	UltimoPagamentoEm   *tipos.Data   `json:"ultimoPagamentoEm"`
	DiasParaVencer      int           `json:"diasParaVencer"`
	DataLimite          *tipos.Data   `json:"dataLimite"`
	PagamentosRestantes *int          `json:"pagamentosRestantes"`
	Encerrada           bool          `json:"encerrada"`
	FormaCobranca       int64         `json:"formaCobranca"`
	TextoCobranca       *string       `json:"textoCobranca"`
	Observacao          *string       `json:"observacao"`
}

// ContasAgendadas são as contas a pagar, pelo vencimento mais próximo (empate: a ordem do Id).
//
// Sem proximosDias, vêm todas, inclusive as inativas e as encerradas. Com ele, só as ativas que vencem até hoje mais
// esses dias (as atrasadas também entram), sem as cobradas no cartão de crédito: no painel, o que cai no cartão não é
// conta a pagar, entra na fatura (#158). Um número de dias que leva a data para fora do ano 1 a 9999 dá
// ErroDataForaDoIntervalo, como a exceção do C#.
//
// diasParaVencer é o vencimento menos hoje (negativo se atrasou). pagamentosRestantes só vem para conta ativa com
// data limite; encerrada é a conta inativa que passou da data limite (as de recorrência única nunca).
func (s *Servico) ContasAgendadas(ctx context.Context, proximosDias *int) ([]ContaAgendadaDto, error) {
	hoje := s.hoje()
	consulta := `SELECT c."Id", c."Descricao", c."Valor", c."ProximoVencimento", c."Recorrencia", c."CategoriaId",
		cat."Nome", c."ContaId", conta."Nome", c."Ativa", c."UltimoPagamentoEm", c."DataLimite", c."FormaCobranca",
		c."TextoCobranca", c."Observacao"
		FROM "ContaAgendada" c
		LEFT JOIN "Categoria" cat ON cat."Id" = c."CategoriaId"
		LEFT JOIN "Conta" conta ON conta."Id" = c."ContaId" `
	var argumentos []any
	if proximosDias != nil {
		// O DateOnly.AddDays do C# confere o número do dia de chegada (0 a MaxDayNumber), sem estourar com um int grande.
		if *proximosDias < -hoje.NumeroDoDia() || *proximosDias > maiorNumeroDoDia-hoje.NumeroDoDia() {
			return nil, fmt.Errorf("vencimentos dos próximos %d dias a partir de %s: %w", *proximosDias, hoje, ErroDataForaDoIntervalo)
		}
		limite := hoje.SomarDias(*proximosDias)
		consulta += `WHERE c."Ativa" = 1 AND c."ProximoVencimento" <= ? AND c."FormaCobranca" <> ? `
		argumentos = append(argumentos, limite, FormaCartaoCredito)
	}
	consulta += `ORDER BY c."ProximoVencimento", c."Id"`

	linhas, err := s.db.QueryContext(ctx, consulta, argumentos...)
	if err != nil {
		return nil, fmt.Errorf("ler as contas a pagar: %w", err)
	}
	defer linhas.Close()
	lista := []ContaAgendadaDto{}
	for linhas.Next() {
		var c ContaAgendadaDto
		var ultimo, limite sql.Null[tipos.Data]
		if err := linhas.Scan(&c.Id, &c.Descricao, &c.Valor, &c.ProximoVencimento, &c.Recorrencia, &c.CategoriaId,
			&c.CategoriaNome, &c.ContaId, &c.ContaNome, &c.Ativa, &ultimo, &limite, &c.FormaCobranca, &c.TextoCobranca,
			&c.Observacao); err != nil {
			return nil, fmt.Errorf("ler conta a pagar: %w", err)
		}
		if ultimo.Valid {
			c.UltimoPagamentoEm = &ultimo.V
		}
		if limite.Valid {
			c.DataLimite = &limite.V
		}
		if c.Ativa {
			if n, ok := PagamentosAte(c.ProximoVencimento, c.Recorrencia, c.DataLimite); ok {
				c.PagamentosRestantes = &n
			}
		}
		c.Encerrada = !c.Ativa && c.DataLimite != nil && c.Recorrencia != RecorrenciaUnica &&
			c.ProximoVencimento.Depois(*c.DataLimite)
		c.DiasParaVencer = c.ProximoVencimento.NumeroDoDia() - hoje.NumeroDoDia()
		lista = append(lista, c)
	}
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("ler as contas a pagar: %w", err)
	}
	return lista, nil
}

// ProximoVencimentoApos é o próximo vencimento de uma conta recorrente, do mesmo jeito que o pagamento avança: mês a
// mês ou ano a ano a partir do vencimento anterior (e não do dia original). A conta mensal de 31/01 vence 28/02, e
// dali em diante no dia 28 (28/03, 28/04...): o dia 31 não volta. Recorrência única (ou desconhecida) não tem
// próximo: ok é false.
func ProximoVencimentoApos(vencimento tipos.Data, recorrencia int64) (proximo tipos.Data, ok bool) {
	switch recorrencia {
	case RecorrenciaMensal:
		return vencimento.SomarMeses(1), true
	case RecorrenciaAnual:
		return vencimento.SomarAnos(1), true
	default:
		return tipos.Data{}, false
	}
}

// PagamentosAte é quantos pagamentos faltam até a data limite, contando o próximo (#69). Sem data limite, ou numa
// conta de recorrência única, ok é false. Conta no máximo 1200.
func PagamentosAte(proximo tipos.Data, recorrencia int64, limite *tipos.Data) (pagamentos int, ok bool) {
	if limite == nil || recorrencia == RecorrenciaUnica {
		return 0, false
	}
	n := 0
	for v, temProximo := proximo, true; temProximo && !v.Depois(*limite) && n < maximoDePagamentos; v, temProximo = ProximoVencimentoApos(v, recorrencia) {
		n++
	}
	return n, true
}
