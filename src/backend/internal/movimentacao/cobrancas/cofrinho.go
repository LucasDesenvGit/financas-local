package cobrancas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/ajustes/categorizacao"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/analises/relatorios"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/movimentacao/transacoes"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// "Pago com" um cofrinho (#691, ADR 0063): a conta a pagar cujo dinheiro sai direto de uma reserva da conta, que o
// Open Finance não manda. O app grava a despesa (Transacao.ReservaNome) e a baixa (PagamentoCofrinho).

// FormaDebitoAutomatico é a FormaCobranca "débito automático": com um cofrinho, a baixa é no vencimento.
const FormaDebitoAutomatico = 2

// statusEfetivada é o StatusTransacao.Efetivada.
const statusEfetivada = 1

// diasDaDuplicata: a baixa automática de até tantos dias atrás ainda é conferida com o extrato.
const diasDaDuplicata = 60

// maximoDeBaixas é quantos vencimentos atrasados uma conta a pagar recebe numa conciliação só (app desligado por meses).
const maximoDeBaixas = 12

// ErrPagamentoMudou é a baixa pelo cofrinho que não dá mais para desfazer: a conta a pagar já mudou depois dela.
var ErrPagamentoMudou = errors.New("A conta a pagar mudou depois desse pagamento: desfaça pela tela de Transações.")

// ErrSemPagamento é a conta a pagar sem pagamento pelo cofrinho para desfazer ou ajustar.
var ErrSemPagamento = errors.New("Esta conta a pagar não tem pagamento pelo cofrinho para mudar.")

// ErrContaEncerrada é o "Pagar" pelo cofrinho de uma conta a pagar encerrada: a baixa reativaria a conta, e o
// "Desfazer" não saberia como ela estava.
var ErrContaEncerrada = errors.New("A conta a pagar está encerrada. Reative-a para pagar pelo cofrinho.")

// Despesa é o que o app lança numa conta: a despesa de uma conta a pagar, pela conta ou pelo cofrinho.
type Despesa struct {
	ContaId     int64
	Data        tipos.Data
	Descricao   string
	Valor       tipos.Decimal // positivo: é gravado negativo
	CategoriaId *int64
	ReservaNome *string // nil = sai do saldo da conta
}

// LancarDespesa grava a despesa efetivada, com a categoria (manual, se houver) e o tipo pelo tipo da categoria (o
// AplicarTipo), e devolve o id dela.
func LancarDespesa(ctx context.Context, q banco.DBTX, d Despesa, agora tipos.DataHora) (int64, error) {
	rs, err := categorizacao.Carregar(ctx, q)
	if err != nil {
		return 0, err
	}
	t := transacoes.Transacao{ContaId: d.ContaId, Data: d.Data, Descricao: d.Descricao,
		Valor: tipos.DeDecimal(d.Valor.Decimal().Abs().Neg()), Status: statusEfetivada, CategoriaId: d.CategoriaId,
		CategoriaManual: d.CategoriaId != nil}
	rs.AplicarTipo(&t)
	res, err := q.ExecContext(ctx, `INSERT INTO "Transacao" ("ContaId", "PluggyTransactionId", "Data", "Descricao",
		"DescricaoOriginal", "Estabelecimento", "Valor", "Tipo", "Status", "CategoriaId", "CategoriaManual",
		"PluggyCategoriaId", "FaturaCartaoId", "ParcelaNumero", "ParcelaTotal", "IgnorarRelatorios", "Emergencia",
		"DescricaoRevisada", "MeioPagamento", "MesmaTitularidade", "RegraDescricaoId", "DescricaoManual",
		"TransferenciaGrupo", "Observacao", "ReservaNome", "CriadoEm", "AtualizadoEm")
		VALUES (?, NULL, ?, ?, NULL, NULL, ?, ?, ?, ?, ?, NULL, NULL, NULL, NULL, ?, 0, 0, NULL, NULL, NULL, 0, NULL, NULL, ?, ?, ?)`,
		t.ContaId, t.Data, t.Descricao, t.Valor, t.Tipo, t.Status, t.CategoriaId, t.CategoriaManual, t.IgnorarRelatorios,
		d.ReservaNome, agora, agora)
	if err != nil {
		return 0, fmt.Errorf("lançar a despesa do pagamento: %w", err)
	}
	return res.LastInsertId()
}

// ContaDoCofrinho é a conta a pagar como a baixa pelo cofrinho a lê.
type ContaDoCofrinho struct {
	Id                int64
	Descricao         string
	Valor             tipos.Decimal
	ProximoVencimento tipos.Data
	Recorrencia       int64
	CategoriaId       *int64
	ContaId           int64
	Ativa             bool
	DataLimite        *tipos.Data
	UltimoPagamentoEm *tipos.Data
	ReservaNome       string
	ReservaDesde      *tipos.Data // a baixa automática só pega os vencimentos a partir deste dia (nil: sem limite)
}

// Avancar é o vencimento depois do atual, como no "Pagar": a única (ou a que passa da data limite, #69) é encerrada.
func Avancar(vencimento tipos.Data, recorrencia int64, dataLimite *tipos.Data) (proximo tipos.Data, ativa bool) {
	p, ok := relatorios.ProximoVencimentoApos(vencimento, recorrencia)
	if !ok {
		return vencimento, false
	}
	return p, dataLimite == nil || !p.Depois(*dataLimite)
}

// ProximoDoPagamento é o vencimento e o estado da conta a pagar depois do "Pagar": na ativa, o Avancar; na encerrada,
// a data avança (se houver próxima) e ela continua encerrada.
func ProximoDoPagamento(vencimento tipos.Data, recorrencia int64, dataLimite *tipos.Data, ativa bool) (tipos.Data, bool) {
	if ativa {
		return Avancar(vencimento, recorrencia, dataLimite)
	}
	if p, ok := relatorios.ProximoVencimentoApos(vencimento, recorrencia); ok {
		return p, false
	}
	return vencimento, false
}

// PagarPeloCofrinho dá baixa no vencimento atual da conta a pagar com a despesa saindo do cofrinho: grava o lançamento
// do app na data, a linha de PagamentoCofrinho (para o "Desfazer") e avança o vencimento. Muda c para o novo estado.
// Só na conta a pagar ativa (ErrContaEncerrada): assim o "Desfazer" sabe que ela estava ativa antes.
func PagarPeloCofrinho(ctx context.Context, q banco.DBTX, c *ContaDoCofrinho, data tipos.Data, automatico bool,
	agora tipos.DataHora) error {
	if !c.Ativa {
		return ErrContaEncerrada
	}
	reserva := c.ReservaNome
	id, err := LancarDespesa(ctx, q, Despesa{ContaId: c.ContaId, Data: data, Descricao: c.Descricao, Valor: c.Valor,
		CategoriaId: c.CategoriaId, ReservaNome: &reserva}, agora)
	if err != nil {
		return err
	}
	vencimento := c.ProximoVencimento
	proximo, ativa := Avancar(vencimento, c.Recorrencia, c.DataLimite)
	if _, err := q.ExecContext(ctx, `INSERT INTO "PagamentoCofrinho" ("ContaAgendadaId", "TransacaoId", "Vencimento",
		"ProximoVencimento", "UltimoPagamentoAnterior", "Automatico", "CriadoEm") VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.Id, id, vencimento, proximo, c.UltimoPagamentoEm, automatico, agora); err != nil {
		return fmt.Errorf("gravar o pagamento pelo cofrinho da conta a pagar %d: %w", c.Id, err)
	}
	if _, err := q.ExecContext(ctx, `UPDATE "ContaAgendada" SET "UltimoPagamentoEm" = ?, "ProximoVencimento" = ?, "Ativa" = ?
		WHERE "Id" = ?`, data, proximo, ativa, c.Id); err != nil {
		return fmt.Errorf("avançar a conta a pagar %d: %w", c.Id, err)
	}
	c.UltimoPagamentoEm, c.ProximoVencimento, c.Ativa = &data, proximo, ativa
	return nil
}

// baixarPeloCofrinho dá a baixa sozinha nas contas em débito automático pagas com um cofrinho cujo vencimento chegou
// (#691): a cobrança não vem pelo extrato. O vencimento desfeito antes (a linha de PagamentoCofrinho sem lançamento)
// não recebe baixa de novo: a conta fica vencida, com "Pagar". O vencimento de antes do dia em que o "Pago com" passou
// a valer (ReservaDesde) também não: quem cadastra a conta com o vencimento meses atrás não ganha despesas nos meses
// já fechados. Conta arquivada fica de fora, como na aba Reservas. Devolve quantas baixas deu.
func baixarPeloCofrinho(ctx context.Context, q banco.DBTX, hoje tipos.Data, agora tipos.DataHora) (int, error) {
	linhas, err := q.QueryContext(ctx, `SELECT a."Id", a."Descricao", a."Valor", a."ProximoVencimento", a."Recorrencia",
		a."CategoriaId", a."ContaId", a."Ativa", a."DataLimite", a."UltimoPagamentoEm", a."ReservaNome", a."ReservaDesde"
		FROM "ContaAgendada" AS a INNER JOIN "Conta" AS c ON c."Id" = a."ContaId"
		WHERE a."Ativa" AND a."FormaCobranca" = ? AND a."ReservaNome" IS NOT NULL AND c."Origem" = ? AND NOT c."Arquivada"
		AND a."ProximoVencimento" <= ? ORDER BY a."Id"`, FormaDebitoAutomatico, tipos.OrigemOpenFinance, hoje)
	if err != nil {
		return 0, fmt.Errorf("ler as contas a pagar pelo cofrinho: %w", err)
	}
	var contas []*ContaDoCofrinho
	for linhas.Next() {
		c := &ContaDoCofrinho{}
		if err := linhas.Scan(&c.Id, &c.Descricao, &c.Valor, &c.ProximoVencimento, &c.Recorrencia, &c.CategoriaId,
			&c.ContaId, &c.Ativa, &c.DataLimite, &c.UltimoPagamentoEm, &c.ReservaNome, &c.ReservaDesde); err != nil {
			linhas.Close()
			return 0, fmt.Errorf("ler conta a pagar pelo cofrinho: %w", err)
		}
		contas = append(contas, c)
	}
	linhas.Close()
	if err := linhas.Err(); err != nil {
		return 0, fmt.Errorf("ler as contas a pagar pelo cofrinho: %w", err)
	}
	baixas := 0
	for _, c := range contas {
		for i := 0; i < maximoDeBaixas && c.Ativa && !c.ProximoVencimento.Depois(hoje) &&
			(c.ReservaDesde == nil || !c.ProximoVencimento.Antes(*c.ReservaDesde)); i++ {
			var desfeito bool
			if err := q.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM "PagamentoCofrinho" WHERE "ContaAgendadaId" = ?
				AND "Vencimento" = ?)`, c.Id, c.ProximoVencimento).Scan(&desfeito); err != nil {
				return baixas, fmt.Errorf("conferir os pagamentos da conta a pagar %d: %w", c.Id, err)
			}
			if desfeito {
				break
			}
			if err := PagarPeloCofrinho(ctx, q, c, c.ProximoVencimento, true, agora); err != nil {
				return baixas, err
			}
			baixas++
		}
	}
	return baixas, nil
}

// baixaPeloCofrinho é uma baixa recente pelo cofrinho, com o lançamento ainda lá, de conta a pagar que concilia.
type baixaPeloCofrinho struct {
	transacao     int64
	conta         int64
	contaId       *int64
	textoCobranca string
	vencimento    tipos.Data
	data          tipos.Data
	criadoEm      tipos.DataHora
	ultimo        *tipos.Data
}

// desfazerDuplicadas apaga a baixa pelo cofrinho cuja cobrança chegou mesmo assim pelo extrato (o cofrinho estava sem
// saldo e o banco tirou da conta, ou o "Foi paga com um cofrinho?" respondido antes de a cobrança atrasada chegar): o
// texto da cobrança, até 10 dias antes ou depois do vencimento e importada depois da baixa. Vale para a automática e
// para a do "Pagar" (a conta é cobrada sozinha: o banco manda a cobrança). Vale a do banco, e o último pagamento vai
// para a data dela. Cada cobrança desfaz uma baixa só: duas contas com textos parecidos não somem com uma cobrança.
// Devolve quantas.
func desfazerDuplicadas(ctx context.Context, q banco.DBTX, hoje tipos.Data,
	carregarRegras func() (map[int64]bool, error)) (int, error) {
	linhas, err := q.QueryContext(ctx, `SELECT p."TransacaoId", a."Id", a."ContaId", a."TextoCobranca", p."Vencimento",
		t."Data", p."CriadoEm", a."UltimoPagamentoEm"
		FROM "PagamentoCofrinho" AS p
		INNER JOIN "ContaAgendada" AS a ON a."Id" = p."ContaAgendadaId"
		INNER JOIN "Transacao" AS t ON t."Id" = p."TransacaoId"
		WHERE a."FormaCobranca" <> ? AND a."TextoCobranca" IS NOT NULL AND t."Data" >= ?
		ORDER BY p."Id"`, FormaAMao, hoje.SomarDias(-diasDaDuplicata))
	if err != nil {
		return 0, fmt.Errorf("ler as baixas pelo cofrinho: %w", err)
	}
	var baixas []baixaPeloCofrinho
	for linhas.Next() {
		var b baixaPeloCofrinho
		if err := linhas.Scan(&b.transacao, &b.conta, &b.contaId, &b.textoCobranca, &b.vencimento, &b.data, &b.criadoEm,
			&b.ultimo); err != nil {
			linhas.Close()
			return 0, fmt.Errorf("ler baixa pelo cofrinho: %w", err)
		}
		baixas = append(baixas, b)
	}
	linhas.Close()
	if err := linhas.Err(); err != nil {
		return 0, fmt.Errorf("ler as baixas pelo cofrinho: %w", err)
	}
	if len(baixas) == 0 {
		return 0, nil
	}
	regrasPorValor, err := carregarRegras()
	if err != nil {
		return 0, err
	}
	desfeitas := 0
	usadas := map[int64]bool{}
	for _, b := range baixas {
		procurado := texto.ParaRegras(b.textoCobranca)
		if procurado == "" {
			continue
		}
		candidatas, err := lerCandidatas(ctx, q, b.vencimento.SomarDias(-DiasAntesDoVencimento),
			b.vencimento.SomarDias(DiasAntesDoVencimento), b.contaId, regrasPorValor)
		if err != nil {
			return desfeitas, err
		}
		var depois []candidata
		for _, x := range candidatas {
			if x.criadoEm.Tempo().After(b.criadoEm.Tempo()) && !usadas[x.id] {
				depois = append(depois, x)
			}
		}
		casada := maisRecente(depois, procurado)
		if casada == nil {
			continue
		}
		usadas[casada.id] = true
		if err := transacoes.Excluir(ctx, q, b.transacao); err != nil {
			return desfeitas, err
		}
		if b.ultimo != nil && b.ultimo.Igual(b.data) {
			if _, err := q.ExecContext(ctx, `UPDATE "ContaAgendada" SET "UltimoPagamentoEm" = ? WHERE "Id" = ?`,
				casada.data, b.conta); err != nil {
				return desfeitas, fmt.Errorf("gravar o pagamento da conta a pagar %d: %w", b.conta, err)
			}
		}
		desfeitas++
	}
	return desfeitas, nil
}

// pagamento é a última baixa pelo cofrinho de uma conta a pagar.
type pagamento struct {
	id                      int64
	transacao               *int64
	vencimento, proximo     tipos.Data
	ultimoPagamentoAnterior *tipos.Data
}

func ultimoPagamento(ctx context.Context, q banco.DBTX, conta int64) (*pagamento, error) {
	var p pagamento
	err := q.QueryRowContext(ctx, `SELECT "Id", "TransacaoId", "Vencimento", "ProximoVencimento", "UltimoPagamentoAnterior"
		FROM "PagamentoCofrinho" WHERE "ContaAgendadaId" = ? ORDER BY "Id" DESC LIMIT 1`, conta).Scan(&p.id, &p.transacao,
		&p.vencimento, &p.proximo, &p.ultimoPagamentoAnterior)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ler o último pagamento pelo cofrinho da conta a pagar %d: %w", conta, err)
	}
	return &p, nil
}

// desfazivel diz se a última baixa pelo cofrinho ainda pode ser desfeita: a conta a pagar continua no vencimento que a
// baixa deixou e, se está encerrada, foi a própria baixa que a encerrou (a última, ou a que passou da data limite), e
// não a pessoa depois. Toda baixa é de conta ativa (PagarPeloCofrinho), então desfazer a deixa ativa.
func desfazivel(proximoAtual tipos.Data, ativa bool, recorrencia int64, dataLimite *tipos.Data, p pagamento) bool {
	if !proximoAtual.Igual(p.proximo) {
		return false
	}
	if ativa {
		return true
	}
	_, continuouAtiva := Avancar(p.vencimento, recorrencia, dataLimite)
	return !continuouAtiva
}

// DesfazerPagamento desfaz a última baixa pelo cofrinho da conta a pagar: apaga o lançamento e volta o vencimento, o
// último pagamento e a conta ativa, como estavam. Só enquanto a conta a pagar não mudou depois dela (ErrPagamentoMudou:
// outro vencimento, ou encerrada pela pessoa); sem baixa com lançamento, ErrSemPagamento.
func DesfazerPagamento(ctx context.Context, q banco.DBTX, conta int64) error {
	p, err := ultimoPagamento(ctx, q, conta)
	if err != nil {
		return err
	}
	if p == nil || p.transacao == nil {
		return ErrSemPagamento
	}
	var proximo tipos.Data
	var ativa bool
	var recorrencia int64
	var dataLimite *tipos.Data
	if err := q.QueryRowContext(ctx, `SELECT "ProximoVencimento", "Ativa", "Recorrencia", "DataLimite" FROM "ContaAgendada"
		WHERE "Id" = ?`, conta).Scan(&proximo, &ativa, &recorrencia, &dataLimite); err != nil {
		return fmt.Errorf("ler a conta a pagar %d: %w", conta, err)
	}
	if !desfazivel(proximo, ativa, recorrencia, dataLimite, *p) {
		return ErrPagamentoMudou
	}
	if err := transacoes.Excluir(ctx, q, *p.transacao); err != nil {
		return err
	}
	if _, err := q.ExecContext(ctx, `UPDATE "ContaAgendada" SET "ProximoVencimento" = ?, "UltimoPagamentoEm" = ?, "Ativa" = 1
		WHERE "Id" = ?`, p.vencimento, p.ultimoPagamentoAnterior, conta); err != nil {
		return fmt.Errorf("voltar a conta a pagar %d: %w", conta, err)
	}
	return nil
}

// AjustarPagamento muda o valor da última baixa pelo cofrinho (a conta de consumo que variou): o lançamento passa a
// ser -valor. Sem baixa com lançamento, ErrSemPagamento.
func AjustarPagamento(ctx context.Context, q banco.DBTX, conta int64, valor tipos.Decimal, agora tipos.DataHora) error {
	p, err := ultimoPagamento(ctx, q, conta)
	if err != nil {
		return err
	}
	if p == nil || p.transacao == nil {
		return ErrSemPagamento
	}
	if _, err := q.ExecContext(ctx, `UPDATE "Transacao" SET "Valor" = ?, "AtualizadoEm" = ? WHERE "Id" = ?`,
		tipos.DeDecimal(valor.Decimal().Abs().Neg()), agora, *p.transacao); err != nil {
		return fmt.Errorf("ajustar o pagamento pelo cofrinho da conta a pagar %d: %w", conta, err)
	}
	return nil
}

// UltimoPeloCofrinho é a última baixa pelo cofrinho de uma conta a pagar, para a lista mostrar (com o lançamento).
type UltimoPeloCofrinho struct {
	TransacaoId  int64         `json:"transacaoId"`
	Data         tipos.Data    `json:"data"`
	Valor        tipos.Decimal `json:"valor"`
	ReservaNome  string        `json:"reservaNome"`
	Automatico   bool          `json:"automatico"`
	PodeDesfazer bool          `json:"podeDesfazer"`
}

// UltimosPeloCofrinho são as últimas baixas pelo cofrinho de cada conta a pagar que ainda têm o lançamento, pelo id
// da conta a pagar. O valor sai positivo.
func UltimosPeloCofrinho(ctx context.Context, q banco.DBTX) (map[int64]UltimoPeloCofrinho, error) {
	linhas, err := q.QueryContext(ctx, `SELECT p."ContaAgendadaId", t."Id", t."Data", t."Valor", t."ReservaNome",
		p."Automatico", p."Vencimento", p."ProximoVencimento", a."ProximoVencimento", a."Ativa", a."Recorrencia", a."DataLimite"
		FROM "PagamentoCofrinho" AS p
		INNER JOIN "ContaAgendada" AS a ON a."Id" = p."ContaAgendadaId"
		INNER JOIN "Transacao" AS t ON t."Id" = p."TransacaoId"
		WHERE p."Id" = (SELECT MAX(x."Id") FROM "PagamentoCofrinho" AS x WHERE x."ContaAgendadaId" = p."ContaAgendadaId")`)
	if err != nil {
		return nil, fmt.Errorf("ler os pagamentos pelo cofrinho: %w", err)
	}
	defer linhas.Close()
	ultimos := map[int64]UltimoPeloCofrinho{}
	for linhas.Next() {
		var conta, recorrencia int64
		var u UltimoPeloCofrinho
		var reserva *string
		var p pagamento
		var proximoAtual tipos.Data
		var ativa bool
		var dataLimite *tipos.Data
		if err := linhas.Scan(&conta, &u.TransacaoId, &u.Data, &u.Valor, &reserva, &u.Automatico, &p.vencimento, &p.proximo,
			&proximoAtual, &ativa, &recorrencia, &dataLimite); err != nil {
			return nil, fmt.Errorf("ler pagamento pelo cofrinho: %w", err)
		}
		u.PodeDesfazer = desfazivel(proximoAtual, ativa, recorrencia, dataLimite, p)
		if reserva != nil {
			u.ReservaNome = *reserva
		}
		u.Valor = tipos.DeDecimal(u.Valor.Decimal().Abs())
		ultimos[conta] = u
	}
	return ultimos, linhas.Err()
}
