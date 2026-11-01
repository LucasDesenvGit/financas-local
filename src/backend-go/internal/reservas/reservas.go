package reservas

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"unicode"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cambio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// conversao é o que as reservas usam do câmbio (o CambioService do C#): a cotação de uma data, o histórico do dólar e o
// conversor. A interface é de quem usa: o serviço de verdade é o internal/cambio.
type conversao interface {
	ObterCotacao(ctx context.Context, moeda string, data *tipos.Data) (*cambio.Cotacao, error)
	GarantirHistorico(ctx context.Context, moeda string, desde tipos.Data) error
	ObterConversor(ctx context.Context, historicoDesde *tipos.Data) (*cambio.Conversor, error)
}

// Servico é o ReservaService.
type Servico struct {
	db      *sql.DB
	cambio  conversao
	relogio relogio.Relogio
	log     *slog.Logger
}

// NovoServico monta o serviço com o banco, o câmbio, o relógio e o log.
func NovoServico(db *sql.DB, c conversao, r relogio.Relogio, log *slog.Logger) *Servico {
	return &Servico{db: db, cambio: c, relogio: r, log: log}
}

// ReservaDto é o do C#, na ordem do record.
type ReservaDto struct {
	ContaId              int64          `json:"contaId"`
	NomeConta            string         `json:"nomeConta"`
	Nome                 string         `json:"nome"`
	Saldo                tipos.Decimal  `json:"saldo"`
	Guardado             tipos.Decimal  `json:"guardado"`
	Retirado             tipos.Decimal  `json:"retirado"`
	Movimentos           int            `json:"movimentos"`
	PrimeiroMovimento    *tipos.Data    `json:"primeiroMovimento"`
	UltimoMovimento      *tipos.Data    `json:"ultimoMovimento"`
	SaldoConferido       *tipos.Decimal `json:"saldoConferido"`
	DataConferencia      *tipos.Data    `json:"dataConferencia"`
	PodeTerSaldoAnterior bool           `json:"podeTerSaldoAnterior"`
	ConferenciaAntiga    bool           `json:"conferenciaAntiga"`
	InformadoPeloBanco   bool           `json:"informadoPeloBanco"`
	Moeda                string         `json:"moeda"`
	Quantidade           *tipos.Decimal `json:"quantidade"`
	Cotacao              *tipos.Decimal `json:"cotacao"`
	ForaDaListaDoBanco   bool           `json:"foraDaListaDoBanco"`
	RestanteDoBanco      bool           `json:"restanteDoBanco"`
	TotalDoBanco         *tipos.Decimal `json:"totalDoBanco"`
}

// movimento é um lançamento de reserva do extrato.
type movimento struct {
	conta     int64
	nomeConta string
	descricao string
	data      tipos.Data
	valor     tipos.Decimal
	nome      string // o nome da reserva (ExtrairNome)
}

// conferida é uma reserva conferida (ReservaConferencia).
type conferida struct {
	conta     int64
	nomeConta string
	nome      string
	saldo     tipos.Decimal
	data      tipos.Data
}

// De onde vêm os lançamentos de reserva: contas do Open Finance que entram no patrimônio, com a conexão ativa (ou
// sem conexão), e a descrição do banco (que não muda quando você renomeia a transação). O LIKE é o filtro grosso do
// C#; a regra exata fica em ExtrairNome e EhMeliDolar.
const deMovimentos = `SELECT t."ContaId", c."Nome", COALESCE(t."DescricaoOriginal", t."Descricao"), t."Data", t."Valor"
	FROM "Transacao" AS t
	INNER JOIN "Conta" AS c ON c."Id" = t."ContaId"
	LEFT JOIN "ConexaoBancaria" AS b ON b."Id" = c."ConexaoBancariaId"
	WHERE t."Status" = 1 AND c."Origem" = 2 AND NOT c."Arquivada" AND c."IncluirNoSaldo" AND (b."Id" IS NULL OR b."Ativa")`

func (s *Servico) movimentos(ctx context.Context, filtro string) ([]movimento, error) {
	linhas, err := s.db.QueryContext(ctx, deMovimentos+` AND (`+filtro+`)`)
	if err != nil {
		return nil, fmt.Errorf("ler os lançamentos de reserva: %w", err)
	}
	defer linhas.Close()
	var lista []movimento
	for linhas.Next() {
		var m movimento
		if err := linhas.Scan(&m.conta, &m.nomeConta, &m.descricao, &m.data, &m.valor); err != nil {
			return nil, fmt.Errorf("ler lançamento de reserva: %w", err)
		}
		lista = append(lista, m)
	}
	return lista, linhas.Err()
}

func (s *Servico) conferidas(ctx context.Context) ([]conferida, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT r."ContaId", c."Nome", r."Nome", r."SaldoConferido", r."DataConferencia"
		FROM "ReservaConferencia" AS r INNER JOIN "Conta" AS c ON c."Id" = r."ContaId"
		WHERE NOT c."Arquivada" AND c."IncluirNoSaldo"`)
	if err != nil {
		return nil, fmt.Errorf("ler as reservas conferidas: %w", err)
	}
	defer linhas.Close()
	var lista []conferida
	for linhas.Next() {
		var c conferida
		if err := linhas.Scan(&c.conta, &c.nomeConta, &c.nome, &c.saldo, &c.data); err != nil {
			return nil, fmt.Errorf("ler reserva conferida: %w", err)
		}
		lista = append(lista, c)
	}
	return lista, linhas.Err()
}

// inicioDaJanela é o primeiro lançamento importado de cada conta: o começo do que a sincronização trouxe.
func (s *Servico) inicioDaJanela(ctx context.Context, contas []int64) (map[int64]tipos.Data, error) {
	inicio := map[int64]tipos.Data{}
	if len(contas) == 0 {
		return inicio, nil
	}
	argumentos := make([]any, len(contas))
	for i, c := range contas {
		argumentos[i] = c
	}
	linhas, err := s.db.QueryContext(ctx, `SELECT "ContaId", MIN("Data") FROM "Transacao" WHERE "ContaId" IN (`+
		strings.TrimSuffix(strings.Repeat("?, ", len(contas)), ", ")+`) GROUP BY "ContaId"`, argumentos...)
	if err != nil {
		return nil, fmt.Errorf("ler o começo do extrato das contas: %w", err)
	}
	defer linhas.Close()
	for linhas.Next() {
		var conta int64
		var data tipos.Data
		if err := linhas.Scan(&conta, &data); err != nil {
			return nil, fmt.Errorf("ler o começo do extrato: %w", err)
		}
		inicio[conta] = data
	}
	return inicio, linhas.Err()
}

// janela calcula o "pode ter saldo anterior" (sem conferência, o primeiro lançamento perto do começo do extrato) e o
// "conferência antiga" (mais de 30 dias).
func janela(primeiro *tipos.Data, inicio tipos.Data, temInicio bool, conferencia *tipos.Data, hoje tipos.Data) (anterior, antiga bool) {
	anterior = conferencia == nil && primeiro != nil && temInicio && dias(inicio, *primeiro) <= toleranciaInicioJanela
	antiga = conferencia != nil && dias(*conferencia, hoje) > DiasConferenciaAntiga
	return anterior, antiga
}

// dias é o DayNumber de b menos o de a.
func dias(a, b tipos.Data) int { return b.NumeroDoDia() - a.NumeroDoDia() }

// somas são o que foi guardado (as saídas da conta) e o que foi retirado (as entradas), sem o sinal.
func somas(lista []movimento) (guardado, retirado tipos.Decimal) {
	guardado, retirado = tipos.Zero, tipos.Zero
	for _, m := range lista {
		switch m.valor.Decimal().Sign() {
		case -1:
			guardado = tipos.Subtrair(guardado, m.valor)
		case 1:
			retirado = tipos.Somar(retirado, m.valor)
		}
	}
	return guardado, retirado
}

func depoisDe(lista []movimento, c *conferida) []movimento {
	if c == nil {
		return lista
	}
	var depois []movimento
	for _, m := range lista {
		if m.data.Depois(c.data) {
			depois = append(depois, m)
		}
	}
	return depois
}

func extremos(lista []movimento) (primeiro, ultimo *tipos.Data) {
	for i := range lista {
		d := lista[i].data
		if primeiro == nil || d.Antes(*primeiro) {
			primeiro = &d
		}
		if ultimo == nil || d.Depois(*ultimo) {
			ultimo = &d
		}
	}
	return primeiro, ultimo
}

type chave struct {
	conta int64
	nome  string // em maiúsculas (o ToUpperInvariant do C#)
}

// Listar é o ListarAsync: as reservas das contas que entram no patrimônio, com o saldo (nunca negativo), da maior
// para a menor (e pelo nome, na ordem da cultura, no empate).
func (s *Servico) Listar(ctx context.Context) ([]ReservaDto, error) {
	movimentos, err := s.movimentos(ctx, `COALESCE(t."DescricaoOriginal", t."Descricao") LIKE 'Dinheiro reservado %'
		OR COALESCE(t."DescricaoOriginal", t."Descricao") LIKE 'Dinheiro retirado %'`)
	if err != nil {
		return nil, err
	}
	conferidas, err := s.conferidas(ctx)
	if err != nil {
		return nil, err
	}
	var contas []int64
	for _, m := range movimentos {
		if !slices.Contains(contas, m.conta) {
			contas = append(contas, m.conta)
		}
	}
	inicio, err := s.inicioDaJanela(ctx, contas)
	if err != nil {
		return nil, err
	}
	hoje := relogio.Hoje(s.relogio)

	// O GroupBy do C# (na ordem do primeiro de cada grupo), com a conta e o nome em maiúsculas.
	grupos := map[chave][]movimento{}
	var chaves []chave
	for _, m := range movimentos {
		nome := ExtrairNome(m.descricao)
		if nome == nil {
			continue
		}
		m.nome = *nome
		k := chave{m.conta, texto.MaiusculasInvariantes(m.nome)}
		if _, ok := grupos[k]; !ok {
			chaves = append(chaves, k)
		}
		grupos[k] = append(grupos[k], m)
	}
	for _, c := range conferidas {
		if k := (chave{c.conta, texto.MaiusculasInvariantes(c.nome)}); !ehNomeMeliDolar(c.nome) && !slices.Contains(chaves, k) {
			chaves = append(chaves, k)
		}
	}

	var reservas []ReservaDto
	for _, k := range chaves {
		lista := grupos[k]
		var conferencia *conferida
		for i := range conferidas {
			if conferidas[i].conta == k.conta && texto.MaiusculasInvariantes(conferidas[i].nome) == k.nome {
				conferencia = &conferidas[i]
				break
			}
		}
		guardado, retirado := somas(depoisDe(lista, conferencia))
		saldo := tipos.Subtrair(tipos.Somar(tipos.Zero, guardado), retirado)
		if conferencia != nil {
			saldo = tipos.Subtrair(tipos.Somar(conferencia.saldo, guardado), retirado)
		}
		primeiro, ultimo := extremos(lista)
		var dataConferencia *tipos.Data
		var saldoConferido *tipos.Decimal
		d := ReservaDto{ContaId: k.conta, Movimentos: len(lista), PrimeiroMovimento: primeiro, UltimoMovimento: ultimo,
			Saldo: tipos.Arredondar(tipos.Maior(tipos.Zero, saldo), 2), Guardado: tipos.Arredondar(guardado, 2),
			Retirado: tipos.Arredondar(retirado, 2), Moeda: cambio.MoedaBase}
		if conferencia != nil {
			d.NomeConta, d.Nome = conferencia.nomeConta, conferencia.nome
			saldoConferido, dataConferencia = &conferencia.saldo, &conferencia.data
		} else {
			// O nome do lançamento mais recente (o primeiro dele, no empate).
			d.NomeConta = lista[0].nomeConta
			maisRecente := 0
			for i := range lista {
				if lista[i].data.Depois(lista[maisRecente].data) {
					maisRecente = i
				}
			}
			d.Nome = lista[maisRecente].nome
		}
		d.SaldoConferido, d.DataConferencia = saldoConferido, dataConferencia
		inicioConta, temInicio := inicio[k.conta]
		d.PodeTerSaldoAnterior, d.ConferenciaAntiga = janela(primeiro, inicioConta, temInicio, dataConferencia, hoje)
		reservas = append(reservas, d)
	}

	// O reservado que o banco informa é em reais e não inclui o Méli Dólar: ele entra depois da substituição.
	doBanco, err := s.informadasPeloBanco(ctx)
	if err != nil {
		return nil, err
	}
	meli, err := s.meliDolar(ctx)
	if err != nil {
		return nil, err
	}
	resultado := append(substituirPeloBanco(reservas, doBanco), meli...)
	slices.SortStableFunc(resultado, func(a, b ReservaDto) int {
		if c := b.Saldo.Decimal().Cmp(a.Saldo.Decimal()); c != 0 {
			return c
		}
		return texto.CompararNaCultura(a.Nome, b.Nome)
	})
	if resultado == nil {
		resultado = []ReservaDto{}
	}
	return resultado, nil
}

// Total é o TotalAsync: a soma dos saldos (em reais).
func (s *Servico) Total(ctx context.Context) (tipos.Decimal, error) {
	lista, err := s.Listar(ctx)
	if err != nil {
		return tipos.Zero, err
	}
	total := tipos.Zero
	for _, r := range lista {
		total = tipos.Somar(total, r.Saldo)
	}
	return total, nil
}

// meliDolar é o MeliDolarAsync (#169, ADR 0038): o Méli Dólar de cada conta. A quantidade em dólar vem da descrição
// ("MUSD 32,09") ou do valor em reais pela PTAX do dia; compra soma, venda subtrai. A conferência guarda a quantidade
// em dólar, e os lançamentos seguintes somam a partir dela. O saldo em reais é a quantidade pela última PTAX guardada.
func (s *Servico) meliDolar(ctx context.Context) ([]ReservaDto, error) {
	// O LIKE do SQLite ignora a caixa só em ASCII: o "_" cobre o "é" de "Méli" e o "ó" de "Dólar" (#567).
	todos, err := s.movimentos(ctx, `COALESCE(t."DescricaoOriginal", t."Descricao") LIKE '%m_li d_lar%'`)
	if err != nil {
		return nil, err
	}
	var movimentos []movimento
	for _, m := range todos {
		if EhMeliDolar(m.descricao) {
			movimentos = append(movimentos, m)
		}
	}
	todas, err := s.conferidas(ctx)
	if err != nil {
		return nil, err
	}
	var conferidas []conferida
	for _, c := range todas {
		if ehNomeMeliDolar(c.nome) {
			conferidas = append(conferidas, c)
		}
	}
	if len(movimentos) == 0 && len(conferidas) == 0 {
		return nil, nil
	}

	hoje := relogio.Hoje(s.relogio)
	desde := hoje
	if primeiro, _ := extremos(movimentos); primeiro != nil {
		desde = *primeiro
	}
	if err := s.cambio.GarantirHistorico(ctx, MoedaMeliDolar, desde); err != nil {
		return nil, err
	}
	conversor, err := s.cambio.ObterConversor(ctx, &desde)
	if err != nil {
		return nil, err
	}
	cotacao := conversor.Taxa(MoedaMeliDolar)
	um := tipos.DecimalDeInteiro(1)
	dolares := func(m movimento) (tipos.Decimal, error) {
		if q := QuantidadeNaDescricao(m.descricao); q != nil {
			return *q, nil
		}
		// O ParaReais(1, "USD", data) do C#: a taxa da data, arredondada a 2 casas.
		taxa := conversor.ParaReaisNaData(um, MoedaMeliDolar, m.data)
		if taxa == nil || !taxa.Decimal().IsPositive() {
			return tipos.Zero, nil
		}
		return tipos.Dividir(tipos.DeDecimal(m.valor.Decimal().Abs()), *taxa)
	}

	var contas []int64
	for _, m := range movimentos {
		if !slices.Contains(contas, m.conta) {
			contas = append(contas, m.conta)
		}
	}
	for _, c := range conferidas {
		if !slices.Contains(contas, c.conta) {
			contas = append(contas, c.conta)
		}
	}
	inicio, err := s.inicioDaJanela(ctx, contas)
	if err != nil {
		return nil, err
	}

	var resultado []ReservaDto
	for _, conta := range contas {
		var lista []movimento
		for _, m := range movimentos {
			if m.conta == conta {
				lista = append(lista, m)
			}
		}
		// O MaxBy da data: a primeira de data maior.
		var conferencia *conferida
		for i := range conferidas {
			if conferidas[i].conta == conta && (conferencia == nil || conferidas[i].data.Depois(conferencia.data)) {
				conferencia = &conferidas[i]
			}
		}
		considerados := depoisDe(lista, conferencia)
		quantidade := tipos.Zero
		if conferencia != nil {
			quantidade = conferencia.saldo
		}
		soma := tipos.Zero
		for _, m := range considerados {
			q, err := dolares(m)
			if err != nil {
				return nil, err
			}
			if m.valor.Decimal().IsNegative() {
				soma = tipos.Somar(soma, q) // compra de dólar
			} else {
				soma = tipos.Subtrair(soma, q) // venda
			}
		}
		quantidade = tipos.Maior(tipos.Zero, tipos.Somar(quantidade, soma))
		guardado, retirado := somas(considerados)
		// Sem nenhuma cotação do dólar guardada (nunca houve internet), vale o que entrou menos o que saiu.
		saldo := tipos.Maior(tipos.Zero, tipos.Subtrair(guardado, retirado))
		if cotacao != nil {
			saldo = tipos.Multiplicar(quantidade, *cotacao)
		}
		primeiro, ultimo := extremos(lista)
		quantidadeArredondada := tipos.Arredondar(quantidade, 2)
		d := ReservaDto{ContaId: conta, Nome: NomeMeliDolar, Saldo: tipos.Arredondar(saldo, 2), Guardado: tipos.Arredondar(guardado, 2),
			Retirado: tipos.Arredondar(retirado, 2), Movimentos: len(lista), PrimeiroMovimento: primeiro, UltimoMovimento: ultimo,
			Moeda: MoedaMeliDolar, Quantidade: &quantidadeArredondada, Cotacao: cotacao}
		var dataConferencia *tipos.Data
		if conferencia != nil {
			d.NomeConta, d.SaldoConferido, dataConferencia = conferencia.nomeConta, &conferencia.saldo, &conferencia.data
		} else {
			d.NomeConta = lista[0].nomeConta
		}
		d.DataConferencia = dataConferencia
		inicioConta, temInicio := inicio[conta]
		d.PodeTerSaldoAnterior, d.ConferenciaAntiga = janela(primeiro, inicioConta, temInicio, dataConferencia, hoje)
		resultado = append(resultado, d)
	}
	return resultado, nil
}

// informada é a reserva que a instituição informa pelo Open Finance (#168), como fica gravada em
// Conta.ReservasInformadas.
type informada struct {
	Nome  string        `json:"nome"`
	Valor tipos.Decimal `json:"valor"`
}

type doBanco struct {
	conta     int64
	nomeConta string
	total     tipos.Decimal
	reservas  []informada
}

// informadasPeloBanco são as contas com o reservado informado pela instituição: o total e, quando houver, as reservas
// com nome.
func (s *Servico) informadasPeloBanco(ctx context.Context) ([]doBanco, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT c."Id", c."Nome", c."SaldoReservado", c."ReservasInformadas"
		FROM "Conta" AS c LEFT JOIN "ConexaoBancaria" AS b ON b."Id" = c."ConexaoBancariaId"
		WHERE c."SaldoReservado" IS NOT NULL AND c."Origem" = 2 AND NOT c."Arquivada" AND c."IncluirNoSaldo"
		AND (b."Id" IS NULL OR b."Ativa")`)
	if err != nil {
		return nil, fmt.Errorf("ler o reservado informado pelo banco: %w", err)
	}
	defer linhas.Close()
	var lista []doBanco
	for linhas.Next() {
		var d doBanco
		var reservas *string
		if err := linhas.Scan(&d.conta, &d.nomeConta, &d.total, &reservas); err != nil {
			return nil, fmt.Errorf("ler o reservado de uma conta: %w", err)
		}
		if reservas != nil && *reservas != "" {
			if err := json.Unmarshal([]byte(*reservas), &d.reservas); err != nil {
				return nil, fmt.Errorf("ler as reservas informadas da conta %d: %w", d.conta, err)
			}
		}
		lista = append(lista, d)
	}
	return lista, linhas.Err()
}

func mesmoNome(a, b string) bool {
	return texto.MaiusculasInvariantes(a) == texto.MaiusculasInvariantes(b)
}

// substituirPeloBanco é o SubstituirPeloBanco (#168, ADR 0037; #237): nas contas em que a instituição informa o
// reservado, ele vale no lugar da estimativa. Só a reserva que veio, pelo nome, na lista detalhada é "informada pelo
// banco" (o nome do extrato fica, quando bate, para as metas ligadas a ele). As outras do extrato continuam estimadas,
// nunca zeradas: com a lista detalhada, marcadas como fora dela; só com o total, ele vira a linha "Reservado" com o que
// passa da soma das reservas do extrato.
func substituirPeloBanco(estimadas []ReservaDto, banco []doBanco) []ReservaDto {
	if len(banco) == 0 {
		return estimadas
	}
	ehDoBanco := func(conta int64) bool {
		return slices.ContainsFunc(banco, func(d doBanco) bool { return d.conta == conta })
	}
	var resultado []ReservaDto
	for _, r := range estimadas {
		if !ehDoBanco(r.ContaId) {
			resultado = append(resultado, r)
		}
	}
	for _, d := range banco {
		var daConta []ReservaDto
		for _, r := range estimadas {
			if r.ContaId == d.conta {
				daConta = append(daConta, r)
			}
		}
		if len(d.reservas) > 0 {
			for _, i := range d.reservas {
				r := ReservaDto{ContaId: d.conta, NomeConta: d.nomeConta, Nome: i.Nome, Saldo: tipos.Arredondar(i.Valor, 2),
					Guardado: tipos.Zero, Retirado: tipos.Zero, Moeda: cambio.MoedaBase, InformadoPeloBanco: true}
				if j := slices.IndexFunc(daConta, func(e ReservaDto) bool { return mesmoNome(e.Nome, i.Nome) }); j >= 0 {
					e := daConta[j]
					r.Nome, r.Guardado, r.Retirado, r.Movimentos = e.Nome, e.Guardado, e.Retirado, e.Movimentos
					r.PrimeiroMovimento, r.UltimoMovimento = e.PrimeiroMovimento, e.UltimoMovimento
				}
				resultado = append(resultado, r)
			}
			for _, r := range daConta {
				if !slices.ContainsFunc(d.reservas, func(i informada) bool { return mesmoNome(i.Nome, r.Nome) }) {
					r.ForaDaListaDoBanco = true
					resultado = append(resultado, r)
				}
			}
			continue
		}
		resultado = append(resultado, daConta...)
		soma := tipos.Zero
		for _, r := range daConta {
			soma = tipos.Somar(soma, r.Saldo)
		}
		restante := tipos.Arredondar(tipos.Subtrair(d.total, soma), 2)
		if !restante.Decimal().IsPositive() {
			continue
		}
		nome := NomeRestanteDoBanco
		if slices.ContainsFunc(daConta, func(r ReservaDto) bool { return mesmoNome(r.Nome, NomeRestanteDoBanco) }) {
			nome = NomeRestanteDoBanco + " (restante)"
		}
		total := d.total
		resultado = append(resultado, ReservaDto{ContaId: d.conta, NomeConta: d.nomeConta, Nome: nome, Saldo: restante,
			Guardado: tipos.Zero, Retirado: tipos.Zero, Moeda: cambio.MoedaBase, RestanteDoBanco: true, TotalDoBanco: &total})
	}
	return resultado
}

// ErrContaInvalida é o "Conta inválida." da conferência.
var ErrContaInvalida = errors.New("Conta inválida.")

// ErrSemCotacao é a mensagem do Méli Dólar informado em reais sem a PTAX da data.
var ErrSemCotacao = errors.New("Não há cotação do dólar para essa data. Informe o saldo em dólares (US$).")

// Conferir é o ConferirAsync: grava (ou substitui) o saldo conferido de uma reserva. O Méli Dólar (#169) guarda a
// quantidade em dólar: informado em reais (moedaInformada "BRL"), é convertido pela PTAX de compra da data, e guarda 8
// casas (convertida de volta pela mesma PTAX, dá o valor informado em reais). Os erros que voltam com a mensagem para
// quem usa são ErrContaInvalida e ErrSemCotacao.
func (s *Servico) Conferir(ctx context.Context, conta int64, nome string, saldo tipos.Decimal, data tipos.Data, moedaInformada *string) error {
	var existe bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM "Conta" WHERE "Id" = ?)`, conta).Scan(&existe); err != nil {
		return fmt.Errorf("conferir a conta: %w", err)
	}
	if !existe {
		return ErrContaInvalida
	}
	nome = strings.TrimFunc(nome, unicode.IsSpace)
	casas := int32(2)
	if ehNomeMeliDolar(nome) {
		nome, casas = NomeMeliDolar, 8
		if moedaInformada != nil && *moedaInformada == cambio.MoedaBase {
			cotacao, err := s.cambio.ObterCotacao(ctx, MoedaMeliDolar, &data)
			if err != nil {
				return err
			}
			if cotacao == nil || !cotacao.Compra.Decimal().IsPositive() {
				return ErrSemCotacao
			}
			if saldo, err = tipos.Dividir(saldo, cotacao.Compra); err != nil {
				return err
			}
		}
	}
	linhas, err := s.db.QueryContext(ctx, `SELECT "Id", "Nome" FROM "ReservaConferencia" WHERE "ContaId" = ?`, conta)
	if err != nil {
		return fmt.Errorf("ler as conferências da conta: %w", err)
	}
	var id *int64
	for linhas.Next() {
		var i int64
		var n string
		if err := linhas.Scan(&i, &n); err != nil {
			linhas.Close()
			return fmt.Errorf("ler conferência: %w", err)
		}
		if id == nil && mesmoNome(n, nome) {
			id = &i
		}
	}
	linhas.Close()
	if err := linhas.Err(); err != nil {
		return fmt.Errorf("ler as conferências da conta: %w", err)
	}
	conferido := tipos.Arredondar(saldo, casas)
	agora := tipos.NovaDataHora(s.relogio.Agora())
	if id == nil {
		_, err = s.db.ExecContext(ctx, `INSERT INTO "ReservaConferencia" ("ContaId", "Nome", "SaldoConferido", "DataConferencia",
			"AtualizadoEm") VALUES (?, ?, ?, ?, ?)`, conta, nome, conferido, data, agora)
	} else {
		_, err = s.db.ExecContext(ctx, `UPDATE "ReservaConferencia" SET "SaldoConferido" = ?, "DataConferencia" = ?,
			"AtualizadoEm" = ? WHERE "Id" = ?`, conferido, data, agora, *id)
	}
	if err != nil {
		return fmt.Errorf("gravar a conferência: %w", err)
	}
	return nil
}

// RemoverConferencia é o RemoverConferenciaAsync: apaga a conferência (o nome aparado, sem diferença de maiúsculas);
// o saldo volta a ser só a soma dos lançamentos do extrato. False se não havia.
func (s *Servico) RemoverConferencia(ctx context.Context, conta int64, nome string) (bool, error) {
	nome = strings.TrimFunc(nome, unicode.IsSpace)
	linhas, err := s.db.QueryContext(ctx, `SELECT "Id", "Nome" FROM "ReservaConferencia" WHERE "ContaId" = ?`, conta)
	if err != nil {
		return false, fmt.Errorf("ler as conferências da conta: %w", err)
	}
	var ids []any
	for linhas.Next() {
		var i int64
		var n string
		if err := linhas.Scan(&i, &n); err != nil {
			linhas.Close()
			return false, fmt.Errorf("ler conferência: %w", err)
		}
		if mesmoNome(n, nome) {
			ids = append(ids, i)
		}
	}
	linhas.Close()
	if err := linhas.Err(); err != nil {
		return false, fmt.Errorf("ler as conferências da conta: %w", err)
	}
	if len(ids) == 0 {
		return false, nil
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM "ReservaConferencia" WHERE "Id" IN (`+
		strings.TrimSuffix(strings.Repeat("?, ", len(ids)), ", ")+`)`, ids...)
	if err != nil {
		return false, fmt.Errorf("apagar a conferência: %w", err)
	}
	n, err := res.RowsAffected()
	return n > 0, err
}
