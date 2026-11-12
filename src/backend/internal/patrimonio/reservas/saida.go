package reservas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/movimentacao/cobrancas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/patrimonio/cambio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// Saída de um cofrinho (#691, ADR 0063): o gasto pago direto da reserva, sem passar pela conta, que não chega pelo
// Open Finance. Vira um lançamento do app na conta, marcado com a reserva (Transacao.ReservaNome): entra nos
// relatórios como despesa e baixa a estimativa da reserva.

// origemOpenFinance é a OrigemConta do Open Finance: só nela há reservas.
const origemOpenFinance = 2

// Os erros da saída que voltam com a mensagem para quem usa.
var (
	ErrSoOpenFinance     = errors.New("O cofrinho tem de ser de uma conta do Open Finance.")
	ErrSaidaMeliDolar    = errors.New("O Méli Dólar não tem saída pelo site: ela é em dólar.")
	ErrCategoriaInvalida = errors.New("Categoria inválida.")
	ErrDiferencaSoHoje   = errors.New("Para registrar a diferença como gasto, confira com a data de hoje.")
	ErrSemDiferenca      = errors.New("O saldo conferido não está abaixo da estimativa: não há diferença para registrar.")
	ErrReservaInvalida   = errors.New("Esta reserva não tem estimativa para comparar (o valor vem do banco ou é em dólar).")
)

// Saida é o gasto pago pelo cofrinho Nome da conta ContaId. Valor é positivo: é gravado negativo.
type Saida struct {
	ContaId     int64
	Nome        string
	Data        tipos.Data
	Valor       tipos.Decimal
	Descricao   string
	CategoriaId *int64
}

// RegistrarSaida grava a saída do cofrinho e devolve o id do lançamento. A conta tem de existir e ser do Open
// Finance, o cofrinho não pode ser o Méli Dólar, e a categoria, se vier, tem de existir.
func (s *Servico) RegistrarSaida(ctx context.Context, q consultas.DBTX, d Saida) (int64, error) {
	var origem int64
	err := q.QueryRowContext(ctx, `SELECT "Origem" FROM "Conta" WHERE "Id" = ?`, d.ContaId).Scan(&origem)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return 0, ErrContaInvalida
	case err != nil:
		return 0, fmt.Errorf("ler a conta da saída: %w", err)
	case origem != origemOpenFinance:
		return 0, ErrSoOpenFinance
	}
	nome := strings.TrimFunc(d.Nome, unicode.IsSpace)
	if ehNomeMeliDolar(nome) {
		return 0, ErrSaidaMeliDolar
	}
	if d.CategoriaId != nil {
		var existe bool
		if err := q.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM "Categoria" WHERE "Id" = ?)`, *d.CategoriaId).
			Scan(&existe); err != nil {
			return 0, fmt.Errorf("conferir a categoria da saída: %w", err)
		}
		if !existe {
			return 0, ErrCategoriaInvalida
		}
	}
	return cobrancas.LancarDespesa(ctx, q, cobrancas.Despesa{ContaId: d.ContaId, Data: d.Data,
		Descricao: strings.TrimFunc(d.Descricao, unicode.IsSpace), Valor: d.Valor, CategoriaId: d.CategoriaId,
		ReservaNome: &nome}, tipos.NovaDataHora(s.relogio.Agora()))
}

// ConferirComDiferenca é a conferência que registra a diferença como gasto (#691): com o saldo conferido abaixo da
// estimativa de hoje, a diferença vira uma saída do cofrinho, na data de hoje, e a conferência é gravada; as duas
// coisas numa transação do banco só. Depois disso, o saldo da reserva é o conferido (o lançamento do próprio dia já
// está na conferência). Só com a data de hoje (ErrDiferencaSoHoje), numa reserva estimada em reais
// (ErrReservaInvalida) e com o conferido abaixo da estimativa (ErrSemDiferenca).
func (s *Servico) ConferirComDiferenca(ctx context.Context, conta int64, nome string, saldo tipos.Decimal, data tipos.Data,
	categoria *int64) error {
	if !data.Igual(relogio.Hoje(s.relogio)) {
		return ErrDiferencaSoHoje
	}
	nome = strings.TrimFunc(nome, unicode.IsSpace)
	lista, err := s.Listar(ctx)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(lista, func(r ReservaDto) bool { return r.ContaId == conta && mesmoNome(r.Nome, nome) })
	if i < 0 {
		return ErrReservaInvalida
	}
	estimada := lista[i]
	if estimada.Moeda != cambio.MoedaBase || estimada.InformadoPeloBanco || estimada.RestanteDoBanco {
		return ErrReservaInvalida
	}
	diferenca := tipos.Subtrair(estimada.Saldo, tipos.Arredondar(saldo, 2))
	if !diferenca.Decimal().IsPositive() {
		return ErrSemDiferenca
	}
	return banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := s.RegistrarSaida(ctx, tx, Saida{ContaId: conta, Nome: estimada.Nome, Data: data, Valor: diferenca,
			Descricao: "Gasto pelo cofrinho " + estimada.Nome + " (diferença da conferência)", CategoriaId: categoria}); err != nil {
			return err
		}
		return s.gravarConferencia(ctx, tx, conta, estimada.Nome, saldo, data, nil)
	})
}
