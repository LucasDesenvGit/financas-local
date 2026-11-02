package emprestimos

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// EmprestimoDto é um contrato no painel de empréstimos, na ordem do record EmprestimoDto do C#.
type EmprestimoDto struct {
	Id                  int64          `json:"id"`
	Natureza            int64          `json:"natureza"`
	Produto             string         `json:"produto"`
	Modalidade          *string        `json:"modalidade"`
	ModalidadeDescricao string         `json:"modalidadeDescricao"`
	NumeroContrato      *string        `json:"numeroContrato"`
	NomeInstituicao     *string        `json:"nomeInstituicao"`
	Moeda               string         `json:"moeda"`
	ValorContratado     tipos.Decimal  `json:"valorContratado"`
	SaldoDevedor        tipos.Decimal  `json:"saldoDevedor"`
	ValorParcela        *tipos.Decimal `json:"valorParcela"`
	ParcelaMensal       tipos.Decimal  `json:"parcelaMensal"`
	ParcelasTotal       *int64         `json:"parcelasTotal"`
	ParcelasPagas       *int64         `json:"parcelasPagas"`
	ParcelasAVencer     *int64         `json:"parcelasAVencer"`
	ParcelasEmAtraso    int64          `json:"parcelasEmAtraso"`
	Periodicidade       *string        `json:"periodicidade"`
	SistemaAmortizacao  *string        `json:"sistemaAmortizacao"`
	TaxaJuros           *tipos.Decimal `json:"taxaJuros"`
	PeriodicidadeTaxa   *string        `json:"periodicidadeTaxa"`
	Indexador           *string        `json:"indexador"`
	Cet                 *tipos.Decimal `json:"cet"`
	DataContratacao     *tipos.Data    `json:"dataContratacao"`
	DataVencimentoFinal *tipos.Data    `json:"dataVencimentoFinal"`
	ProximoVencimento   *tipos.Data    `json:"proximoVencimento"`
	TotalPago           tipos.Decimal  `json:"totalPago"`
	UltimoPagamento     *tipos.Data    `json:"ultimoPagamento"`
	Quitado             bool           `json:"quitado"`
	AtualizadoEm        tipos.DataHora `json:"atualizadoEm"`
}

// ResumoCreditoDto são os totais de uma natureza (financiamentos ou empréstimos).
type ResumoCreditoDto struct {
	Natureza          int64         `json:"natureza"`
	ContratosAbertos  int           `json:"contratosAbertos"`
	ContratosQuitados int           `json:"contratosQuitados"`
	SaldoDevedor      tipos.Decimal `json:"saldoDevedor"`
	ValorContratado   tipos.Decimal `json:"valorContratado"`
	ParcelaMensal     tipos.Decimal `json:"parcelaMensal"`
	TotalPago         tipos.Decimal `json:"totalPago"`
	ParcelasEmAtraso  int64         `json:"parcelasEmAtraso"`
}

// PagamentosMesCreditoDto é o que se pagou num mês, por natureza.
type PagamentosMesCreditoDto struct {
	Ano            int           `json:"ano"`
	Mes            int           `json:"mes"`
	Financiamentos tipos.Decimal `json:"financiamentos"`
	Emprestimos    tipos.Decimal `json:"emprestimos"`
}

// PainelEmprestimosDto é o painel de empréstimos (GET /api/emprestimos).
type PainelEmprestimosDto struct {
	Financiamentos   ResumoCreditoDto          `json:"financiamentos"`
	Emprestimos      ResumoCreditoDto          `json:"emprestimos"`
	Contratos        []EmprestimoDto           `json:"contratos"`
	PagamentosPorMes []PagamentosMesCreditoDto `json:"pagamentosPorMes"`
}

// contratoDoPainel é a linha de Emprestimo com o que o painel precisa.
type contratoDoPainel struct {
	dto                 EmprestimoDto
	dataPrimeiraParcela *tipos.Data
	pagamentos          []tipos.Decimal
}

// Painel é o PainelAsync do C#: os contratos das conexões ativas (em aberto primeiro, pelo maior saldo devedor), os
// totais de cada natureza e os pagamentos dos últimos 12 meses, o atual incluído (ADR 0014).
//
// O total pago de um contrato e a soma dos pagamentos de um mês são feitos pelo EF Core no SQLite (o ef_sum): saem no
// formato em que ele grava ("0.0###"), e não com a escala da soma em C# (NaEscalaDoBanco). Sem pagamento, o total pago
// é "0.0" (o "?? 0m" vira COALESCE no SQL) e o mês é 0 (o Sum do LINQ, em memória, sobre nenhum grupo).
func (s *Servico) Painel(ctx context.Context, hoje tipos.Data) (PainelEmprestimosDto, error) {
	contratos, err := s.contratosDoPainel(ctx)
	if err != nil {
		return PainelEmprestimosDto{}, err
	}
	lista := make([]EmprestimoDto, 0, len(contratos))
	for _, c := range contratos {
		d := c.dto
		d.ModalidadeDescricao = DescreverModalidade(d.Modalidade, d.Natureza)
		if d.ParcelaMensal, err = ParcelaMensal(d.ValorParcela, d.Periodicidade); err != nil {
			return PainelEmprestimosDto{}, fmt.Errorf("parcela mensal do contrato %d: %w", d.Id, err)
		}
		d.ProximoVencimento = proximoVencimento(d, c.dataPrimeiraParcela, hoje)
		// O "?? 0m" vai para o SQL (COALESCE(ef_sum(...), '0.0')): sem pagamento, o zero também sai como "0.0".
		d.TotalPago = somaDoBanco(c.pagamentos).NaEscalaDoBanco()
		d.Quitado = !d.SaldoDevedor.Decimal().IsPositive()
		lista = append(lista, d)
	}
	// OrderBy(Quitado).ThenByDescending(SaldoDevedor): estável, o empate fica na ordem do Id.
	slices.SortStableFunc(lista, func(a, b EmprestimoDto) int {
		if a.Quitado != b.Quitado {
			if a.Quitado {
				return 1
			}
			return -1
		}
		return b.SaldoDevedor.Decimal().Cmp(a.SaldoDevedor.Decimal())
	})
	meses, err := s.pagamentosPorMes(ctx, hoje)
	if err != nil {
		return PainelEmprestimosDto{}, err
	}
	return PainelEmprestimosDto{Financiamentos: resumir(lista, NaturezaFinanciamento),
		Emprestimos: resumir(lista, NaturezaEmprestimo), Contratos: lista,
		PagamentosPorMes: meses}, nil
}

func (s *Servico) contratosDoPainel(ctx context.Context) ([]contratoDoPainel, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT e."Id", e."Natureza", e."Produto", e."Modalidade", e."NumeroContrato",
		c."NomeInstituicao", e."Moeda", e."ValorContratado", e."SaldoDevedor", e."ValorParcela", e."ParcelasTotal",
		e."ParcelasPagas", e."ParcelasAVencer", e."ParcelasEmAtraso", e."Periodicidade", e."SistemaAmortizacao",
		e."TaxaJuros", e."PeriodicidadeTaxa", e."Indexador", e."Cet", e."DataContratacao", e."DataPrimeiraParcela",
		e."DataVencimentoFinal", e."AtualizadoEm",
		(SELECT MAX(p."Data") FROM "PagamentoEmprestimo" p WHERE p."EmprestimoId" = e."Id")
		FROM "Emprestimo" e JOIN "ConexaoBancaria" c ON c."Id" = e."ConexaoBancariaId" WHERE c."Ativa" = 1 ORDER BY e."Id"`)
	if err != nil {
		return nil, fmt.Errorf("ler os contratos do painel de empréstimos: %w", err)
	}
	var lista []contratoDoPainel
	indice := map[int64]int{}
	for linhas.Next() {
		var c contratoDoPainel
		var atraso *int64
		d := &c.dto
		if err := linhas.Scan(&d.Id, &d.Natureza, &d.Produto, &d.Modalidade, &d.NumeroContrato, &d.NomeInstituicao,
			&d.Moeda, &d.ValorContratado, &d.SaldoDevedor, &d.ValorParcela, &d.ParcelasTotal, &d.ParcelasPagas,
			&d.ParcelasAVencer, &atraso, &d.Periodicidade, &d.SistemaAmortizacao, &d.TaxaJuros, &d.PeriodicidadeTaxa,
			&d.Indexador, &d.Cet, &d.DataContratacao, &c.dataPrimeiraParcela, &d.DataVencimentoFinal, &d.AtualizadoEm,
			&d.UltimoPagamento); err != nil {
			linhas.Close()
			return nil, fmt.Errorf("ler contrato do painel de empréstimos: %w", err)
		}
		if atraso != nil {
			d.ParcelasEmAtraso = *atraso
		}
		indice[d.Id] = len(lista)
		lista = append(lista, c)
	}
	linhas.Close()
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("ler os contratos do painel de empréstimos: %w", err)
	}
	// Os pagamentos de cada contrato, para o total pago (somado em Go: dinheiro é texto no banco).
	pagamentos, err := s.db.QueryContext(ctx, `SELECT "EmprestimoId", "Valor" FROM "PagamentoEmprestimo" ORDER BY "Id"`)
	if err != nil {
		return nil, fmt.Errorf("ler os pagamentos dos empréstimos: %w", err)
	}
	defer pagamentos.Close()
	for pagamentos.Next() {
		var id int64
		var valor tipos.Decimal
		if err := pagamentos.Scan(&id, &valor); err != nil {
			return nil, fmt.Errorf("ler pagamento de empréstimo: %w", err)
		}
		if i, ok := indice[id]; ok {
			lista[i].pagamentos = append(lista[i].pagamentos, valor)
		}
	}
	if err := pagamentos.Err(); err != nil {
		return nil, fmt.Errorf("ler os pagamentos dos empréstimos: %w", err)
	}
	return lista, nil
}

// somaDoBanco é o Sum feito pelo EF Core no SQLite (ef_sum): no formato em que ele grava, ou 0 sem linhas.
func somaDoBanco(valores []tipos.Decimal) tipos.Decimal {
	if len(valores) == 0 {
		return tipos.Zero
	}
	total := valores[0]
	for _, v := range valores[1:] {
		total = tipos.Somar(total, v)
	}
	return total.NaEscalaDoBanco()
}

// resumir são os totais de uma natureza: abertos e quitados, e as somas em memória (o Sum do LINQ, a partir de 0).
func resumir(lista []EmprestimoDto, natureza int64) ResumoCreditoDto {
	r := ResumoCreditoDto{Natureza: natureza, SaldoDevedor: tipos.Zero, ValorContratado: tipos.Zero, TotalPago: tipos.Zero}
	parcela := tipos.Zero
	for _, e := range lista {
		if e.Natureza != natureza {
			continue
		}
		r.TotalPago = tipos.Somar(r.TotalPago, e.TotalPago)
		if e.Quitado {
			r.ContratosQuitados++
			continue
		}
		r.ContratosAbertos++
		r.SaldoDevedor = tipos.Somar(r.SaldoDevedor, e.SaldoDevedor)
		r.ValorContratado = tipos.Somar(r.ValorContratado, e.ValorContratado)
		parcela = tipos.Somar(parcela, e.ParcelaMensal)
		r.ParcelasEmAtraso += e.ParcelasEmAtraso
	}
	r.ParcelaMensal = tipos.Arredondar(parcela, casasDaParcelaMensal)
	return r
}

// pagamentosPorMes são os pagamentos dos últimos 12 meses (o atual incluído) das conexões ativas, por natureza. Cada
// mês e natureza é uma soma do EF Core no SQLite (o GroupBy com Sum), no formato em que ele grava; sem pagamento, 0.
func (s *Servico) pagamentosPorMes(ctx context.Context, hoje tipos.Data) ([]PagamentosMesCreditoDto, error) {
	inicio := tipos.NovaData(hoje.Ano(), hoje.Mes(), 1).SomarMeses(-11)
	linhas, err := s.db.QueryContext(ctx, `SELECT p."Data", p."Valor", e."Natureza" FROM "PagamentoEmprestimo" p
		JOIN "Emprestimo" e ON e."Id" = p."EmprestimoId" JOIN "ConexaoBancaria" c ON c."Id" = e."ConexaoBancariaId"
		WHERE p."Data" >= ? AND p."Data" <= ? AND c."Ativa" = 1 ORDER BY p."Id"`, inicio, hoje)
	if err != nil {
		return nil, fmt.Errorf("ler os pagamentos por mês: %w", err)
	}
	defer linhas.Close()
	type chave struct {
		ano, mes int
		natureza int64
	}
	grupos := map[chave][]tipos.Decimal{}
	for linhas.Next() {
		var data tipos.Data
		var valor tipos.Decimal
		var natureza int64
		if err := linhas.Scan(&data, &valor, &natureza); err != nil {
			return nil, fmt.Errorf("ler pagamento do mês: %w", err)
		}
		k := chave{data.Ano(), int(data.Mes()), natureza}
		grupos[k] = append(grupos[k], valor)
	}
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("ler os pagamentos por mês: %w", err)
	}
	meses := []PagamentosMesCreditoDto{}
	for m := inicio; m.NumeroDoDia() <= hoje.NumeroDoDia(); m = m.SomarMeses(1) {
		soma := func(natureza int64) tipos.Decimal {
			// O Sum do LINQ (a partir de 0) sobre os grupos do mês: há no máximo um.
			return tipos.Somar(tipos.Zero, somaDoBanco(grupos[chave{m.Ano(), int(m.Mes()), natureza}]))
		}
		meses = append(meses, PagamentosMesCreditoDto{Ano: m.Ano(), Mes: int(m.Mes()),
			Financiamentos: soma(NaturezaFinanciamento), Emprestimos: soma(NaturezaEmprestimo)})
	}
	return meses, nil
}

// proximoVencimento é a previsão da próxima parcela, só para contrato mensal (MONTHLY) em aberto: a 1ª parcela mais as
// parcelas já vencidas (pagas ou em atraso). Depois do vencimento final, ou mais de um ano atrás, não há previsão.
func proximoVencimento(e EmprestimoDto, primeira *tipos.Data, hoje tipos.Data) *tipos.Data {
	if !e.SaldoDevedor.Decimal().IsPositive() || primeira == nil || e.ParcelasPagas == nil {
		return nil
	}
	if e.Periodicidade == nil || !strings.EqualFold(*e.Periodicidade, "MONTHLY") {
		return nil
	}
	proxima := primeira.SomarMeses(int(*e.ParcelasPagas + e.ParcelasEmAtraso))
	if e.DataVencimentoFinal != nil && proxima.NumeroDoDia() > e.DataVencimentoFinal.NumeroDoDia() {
		return nil
	}
	if proxima.NumeroDoDia() < hoje.SomarAnos(-1).NumeroDoDia() {
		return nil
	}
	return &proxima
}

// modalidades são as descrições das modalidades conhecidas do Open Finance Brasil.
var modalidades = map[string]string{
	"FINANCIAMENTO_HABITACIONAL_SFH":        "Financiamento habitacional (SFH)",
	"FINANCIAMENTO_HABITACIONAL_EXCETO_SFH": "Financiamento habitacional",
	"AQUISICAO_BENS_VEICULOS_AUTOMOTORES":   "Financiamento de veículo",
	"AQUISICAO_BENS_OUTROS_BENS":            "Financiamento de bens",
	"CREDITO_PESSOAL_COM_CONSIGNACAO":       "Crédito consignado",
	"CREDITO_PESSOAL_SEM_CONSIGNACAO":       "Crédito pessoal",
	"HOME_EQUITY":                           "Empréstimo com garantia de imóvel",
	"CHEQUE_ESPECIAL":                       "Cheque especial",
	"CONTA_GARANTIDA":                       "Conta garantida",
	"MICROCREDITO_PRODUTIVO_ORIENTADO":      "Microcrédito",
	"MICROCREDITO":                          "Microcrédito",
}

// DescreverModalidade é o ClassificacaoCredito.DescreverModalidade do C#: o nome em português das modalidades
// conhecidas; sem modalidade, "Financiamento" ou "Empréstimo" pela natureza; qualquer outra, o código em título
// ("FINANCIAMENTO_RURAL" → "Financiamento Rural", o ToTitleCase pt-BR).
func DescreverModalidade(modalidade *string, natureza int64) string {
	if modalidade == nil || *modalidade == "" {
		if natureza == NaturezaFinanciamento {
			return "Financiamento"
		}
		return "Empréstimo"
	}
	if d, ok := modalidades[strings.ToUpper(*modalidade)]; ok {
		return d
	}
	return titulo(strings.ToLower(strings.ReplaceAll(*modalidade, "_", " ")))
}

// titulo é o TextInfo.ToTitleCase do .NET para um texto já em minúsculas: a primeira letra de cada palavra em
// maiúscula. Uma palavra começa depois de um caractere que não é letra nem dígito.
func titulo(s string) string {
	var b strings.Builder
	anterior := ' '
	for _, r := range s {
		atual := r
		if unicode.IsLetter(r) && !unicode.IsLetter(anterior) && !unicode.IsDigit(anterior) {
			r = unicode.ToUpper(r)
		}
		b.WriteRune(r)
		anterior = atual
	}
	return b.String()
}
