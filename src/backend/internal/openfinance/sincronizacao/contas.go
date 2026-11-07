package sincronizacao

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/movimentacao/contas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/openfinance/pluggy"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// Tipos de conta (o enum TipoConta do C#).
const (
	contaCorrente int64 = 1
	poupanca      int64 = 2
	cartaoCredito int64 = 3
)

const corPadrao = "#0f766e"

// contaLocal é a conta da Pluggy já gravada: o que a sincronização usa depois.
type contaLocal struct {
	id       int64
	pluggyId string
	ehCartao bool
	nova     bool
}

// gravarConta cria ou atualiza a conta. Os campos do usuário (nome, banco, conta vinculada) ficam como estão.
func (r *rodada) gravarConta(ctx context.Context, conexao int64, pc pluggy.Conta, corDaConexao *string) (contaLocal, error) {
	ehCartao := pc.Type != nil && strings.EqualFold(*pc.Type, "CREDIT")
	tipo := contaCorrente
	switch {
	case ehCartao:
		tipo = cartaoCredito
	case pc.Subtype != nil && strings.EqualFold(*pc.Subtype, "SAVINGS_ACCOUNT"):
		tipo = poupanca
	}
	saldo, err := arredondar(pc.Balance)
	if err != nil {
		return contaLocal{}, err
	}
	if ehCartao {
		saldo = tipos.DeDecimal(saldo.Decimal().Neg())
	}
	agora := r.agora()
	campos := []string{`"Tipo" = ?`, `"Numero" = ?`, `"Moeda" = ?`, `"Saldo" = ?`, `"AtualizadoEm" = ?`}
	valores := []any{tipo, limitar(pc.Number, 60), normalizarMoeda(pc.CurrencyCode), saldo, agora}
	if ehCartao && pc.CreditData != nil {
		limite, err := arredondarNulo(pc.CreditData.CreditLimit)
		if err != nil {
			return contaLocal{}, err
		}
		disponivel, err := arredondarNulo(pc.CreditData.AvailableCreditLimit)
		if err != nil {
			return contaLocal{}, err
		}
		campos = append(campos, `"LimiteCredito" = ?`, `"LimiteDisponivel" = ?`)
		valores = append(valores, limite, disponivel)
		if d := pc.CreditData.BalanceCloseDate; d != nil {
			campos, valores = append(campos, `"DiaFechamento" = ?`), append(valores, d.Data().Dia())
		}
		if d := pc.CreditData.BalanceDueDate; d != nil {
			campos, valores = append(campos, `"DiaVencimento" = ?`), append(valores, d.Data().Dia())
		}
	}
	if !ehCartao {
		// #168: o dinheiro separado em reservas e caixinhas, quando a instituição informa.
		reservado, informadas, nomes, err := reservasDoOpenFinance(pc.BankData)
		if err != nil {
			return contaLocal{}, err
		}
		campos = append(campos, `"SaldoReservado" = ?`, `"ReservasInformadas" = ?`)
		valores = append(valores, reservado, informadas)
		if len(nomes) > 0 {
			r.log.Info("reservas informadas pela instituição", "origem", "Open Finance", "campos", strings.Join(nomes, ", "))
		}
	}

	conta := contaLocal{pluggyId: pc.Id, ehCartao: ehCartao}
	err = banco.EmTransacao(ctx, r.db, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, `SELECT "Id" FROM "Conta" WHERE "PluggyAccountId" = ?`, pc.Id).Scan(&conta.id)
		if errors.Is(err, sql.ErrNoRows) {
			conta.nova = true
			conta.id, err = inserirConta(ctx, tx, conexao, pc, corDaConexao, tipo, agora)
		}
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE "Conta" SET `+strings.Join(campos, ", ")+` WHERE "Id" = ?`, append(valores, conta.id)...)
		return err
	})
	if err != nil {
		return contaLocal{}, fmt.Errorf("gravar a conta: %w", err)
	}
	return conta, nil
}

func inserirConta(ctx context.Context, tx *sql.Tx, conexao int64, pc pluggy.Conta, corDaConexao *string, tipo int64, agora tipos.DataHora) (int64, error) {
	nome := pc.MarketingName
	if nome == nil {
		nome = pc.Name
	}
	titulo := "Conta"
	if nome != nil {
		titulo = limitado(*nome, 120)
	}
	cor := corPadrao
	if corDaConexao != nil && len(*corDaConexao) == 7 {
		cor = *corDaConexao
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO "Conta" ("ConexaoBancariaId", "PluggyAccountId", "Nome", "Tipo", "Origem", "Moeda", "SaldoInicial", "Saldo", "Cor",
		"IncluirNoSaldo", "Arquivada", "CriadoEm", "AtualizadoEm") VALUES (?, ?, ?, ?, ?, 'BRL', ?, ?, ?, 1, 0, ?, ?)`,
		conexao, pc.Id, titulo, tipo, contas.OpenFinance, tipos.Zero, tipos.Zero, cor, agora, agora)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// sincronizarFaturas grava as faturas do cartão e devolve o id de cada uma pelo id da Pluggy.
func (r *rodada) sincronizarFaturas(ctx context.Context, conta contaLocal) (map[string]int64, error) {
	lista, err := r.pluggy.ListarFaturas(ctx, r.sessao, conta.pluggyId)
	if err != nil {
		return nil, err
	}
	existentes := map[string]int64{}
	err = banco.EmTransacao(ctx, r.db, func(tx *sql.Tx) error {
		linhas, err := tx.QueryContext(ctx, `SELECT "PluggyBillId", "Id" FROM "FaturaCartao" WHERE "ContaId" = ? AND "PluggyBillId" IS NOT NULL`, conta.id)
		if err != nil {
			return err
		}
		defer linhas.Close()
		for linhas.Next() {
			var chave string
			var id int64
			if err := linhas.Scan(&chave, &id); err != nil {
				return err
			}
			existentes[chave] = id
		}
		if err := linhas.Err(); err != nil {
			return err
		}
		for _, pf := range lista {
			if strings.TrimSpace(pf.Id) == "" || pf.DueDate == nil {
				continue
			}
			if err := r.gravarFatura(ctx, tx, conta.id, pf, existentes); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("gravar as faturas: %w", err)
	}
	return existentes, nil
}

func (r *rodada) gravarFatura(ctx context.Context, tx *sql.Tx, contaId int64, pf pluggy.Fatura, existentes map[string]int64) error {
	total, err := arredondar(pf.TotalAmount)
	if err != nil {
		return err
	}
	minimo, err := arredondarNulo(pf.MinimumPaymentAmount)
	if err != nil {
		return err
	}
	var pago *tipos.Decimal
	if len(pf.Payments) > 0 {
		soma, err := pluggy.SomaDosPagamentos(pf.Payments)
		if err != nil {
			return err
		}
		arredondado := tipos.ArredondarLongeDoZero(soma, 2)
		pago = &arredondado
	}
	if id, ok := existentes[pf.Id]; ok {
		_, err := tx.ExecContext(ctx, `UPDATE "FaturaCartao" SET "DataVencimento" = ?, "DataFechamento" = ?, "ValorTotal" = ?, "ValorMinimo" = ?, "ValorPago" = ?,
			"AtualizadoEm" = ? WHERE "Id" = ?`, pf.DueDate.Data(), dataDe(pf.BillClosingDate), total, minimo, pago, r.agora(), id)
		return err
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO "FaturaCartao" ("ContaId", "PluggyBillId", "DataVencimento", "DataFechamento", "ValorTotal", "ValorMinimo",
		"ValorPago", "AtualizadoEm") VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, contaId, pf.Id, pf.DueDate.Data(), dataDe(pf.BillClosingDate), total, minimo, pago, r.agora())
	if err != nil {
		return err
	}
	existentes[pf.Id], err = res.LastInsertId()
	return err
}

// tamanhoDoNomeDaReserva é o TamanhoMaximoNome do ReservaService.
const tamanhoDoNomeDaReserva = 100

// reservaInformada é o ReservaInformada do C#.
type reservaInformada struct {
	nome  string
	valor tipos.Decimal
}

// reservasDoOpenFinance é o ReservaService.DoOpenFinance (#168): o total reservado (nil: a instituição não informa), as
// reservas com nome em JSON e os nomes dos campos de cada item, para o log: a documentação da Pluggy não os descreve.
// Sem os detalhes, vale o "investido automaticamente".
func reservasDoOpenFinance(dados *pluggy.DadosBancarios) (*tipos.Decimal, *string, []string, error) {
	if dados == nil {
		return nil, nil, nil, nil
	}
	var reservas []reservaInformada
	vistos := map[string]bool{}
	for _, item := range dados.ReservedBalances {
		pares, ok := propriedades(item)
		if !ok {
			continue
		}
		var valor *tipos.Decimal
		var nome *string
		for _, p := range pares {
			vistos[p.nome] = true
			chave := strings.ToLower(p.nome)
			if valor == nil && slices.Contains([]string{"amount", "balance", "value", "total", "reservedamount", "reservedbalance"}, chave) {
				v, err := numeroDaReserva(p.valor)
				if err != nil {
					return nil, nil, nil, err
				}
				valor = v
			}
			if nome == nil && slices.Contains([]string{"name", "description", "label", "goal", "type", "category"}, chave) {
				var texto string
				if len(p.valor) > 0 && p.valor[0] == '"' && json.Unmarshal(p.valor, &texto) == nil {
					nome = &texto
				}
			}
		}
		if valor == nil {
			continue
		}
		titulo := "Reservado"
		if nome != nil && strings.TrimSpace(*nome) != "" {
			titulo = limitado(strings.TrimSpace(*nome), tamanhoDoNomeDaReserva)
		}
		reservas = append(reservas, reservaInformada{titulo, tipos.Arredondar(*valor, 2)})
	}
	campos := make([]string, 0, len(vistos))
	for c := range vistos {
		campos = append(campos, c)
	}
	slices.Sort(campos)
	if len(reservas) > 0 {
		total := tipos.Zero
		for _, reserva := range reservas {
			total = tipos.Somar(total, reserva.valor)
		}
		texto := reservasEmJson(reservas)
		return &total, &texto, campos, nil
	}
	if dados.AutomaticallyInvestedBalance != nil {
		d, err := dados.AutomaticallyInvestedBalance.Decimal()
		if err != nil {
			return nil, nil, nil, err
		}
		if d.Decimal().IsPositive() {
			arredondado := tipos.Arredondar(d, 2)
			return &arredondado, nil, campos, nil
		}
	}
	return nil, nil, campos, nil
}

// numeroDaReserva lê o número do campo, ou o de { "value": 10.0, "currency": "BRL" }. O que não for número é nil.
func numeroDaReserva(cru json.RawMessage) (*tipos.Decimal, error) {
	if len(cru) == 0 {
		return nil, nil
	}
	if cru[0] == '{' {
		pares, _ := propriedades(cru)
		for _, p := range pares {
			if chave := strings.ToLower(p.nome); (chave == "value" || chave == "amount") && ehNumero(p.valor) {
				return decimalDoJson(p.valor)
			}
		}
		return nil, nil
	}
	if !ehNumero(cru) {
		return nil, nil
	}
	return decimalDoJson(cru)
}

func ehNumero(cru json.RawMessage) bool {
	return len(cru) > 0 && (cru[0] == '-' || (cru[0] >= '0' && cru[0] <= '9'))
}

func decimalDoJson(cru json.RawMessage) (*tipos.Decimal, error) {
	d, err := tipos.NovoDecimal(string(cru))
	if err != nil {
		return nil, errors.New("valor da reserva fora do formato")
	}
	return &d, nil
}

type propriedade struct {
	nome  string
	valor json.RawMessage
}

// propriedades lê o objeto na ordem em que veio: a primeira propriedade que serve é a que vale. ok é false fora de um objeto.
func propriedades(cru json.RawMessage) ([]propriedade, bool) {
	dec := json.NewDecoder(strings.NewReader(string(cru)))
	inicio, err := dec.Token()
	if err != nil || inicio != json.Delim('{') {
		return nil, false
	}
	var lista []propriedade
	for dec.More() {
		chave, err := dec.Token()
		if err != nil {
			return nil, false
		}
		var valor json.RawMessage
		if err := dec.Decode(&valor); err != nil {
			return nil, false
		}
		lista = append(lista, propriedade{chave.(string), valor})
	}
	return lista, true
}

// reservasEmJson escreve a lista como o System.Text.Json: "nome" e "valor", com o texto escapado pelo JavaScriptEncoder
// padrão, porque o .NET e o Go convivem na mesma coluna.
func reservasEmJson(reservas []reservaInformada) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, reserva := range reservas {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(`{"nome":"` + escaparComoONet(reserva.nome) + `","valor":` + reserva.valor.String() + `}`)
	}
	b.WriteByte(']')
	return b.String()
}

// escaparComoONet é o JavaScriptEncoder.Default: só o ASCII visível passa, fora os caracteres perigosos em HTML.
func escaparComoONet(texto string) string {
	var b strings.Builder
	for _, c := range texto {
		switch {
		case c == '\\':
			b.WriteString(`\\`)
		case c == '\n':
			b.WriteString(`\n`)
		case c == '\r':
			b.WriteString(`\r`)
		case c == '\t':
			b.WriteString(`\t`)
		case c == '\b':
			b.WriteString(`\b`)
		case c == '\f':
			b.WriteString(`\f`)
		case c > 0xFFFF:
			r := c - 0x10000
			fmt.Fprintf(&b, `\u%04X\u%04X`, 0xD800+(r>>10), 0xDC00+(r&0x3FF))
		case c < 0x20 || c > 0x7E || strings.ContainsRune("\"&'+<>`", c):
			fmt.Fprintf(&b, `\u%04X`, c)
		default:
			b.WriteRune(c)
		}
	}
	return b.String()
}
