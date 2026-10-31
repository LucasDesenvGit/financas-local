// Package conferenciabanco é o "Conferir com o banco" (#172, ConferenciaBancoService do C#): para cada conta, fatura do mês
// e investimento, o valor que o site usa e as pistas do que pode fazer o app do banco mostrar outro número: quando o banco
// atualizou, reservas, pendentes, parcelas, estornos, compras repetidas. Quem compara é a tela: você digita o valor do app, e
// a pista com o mesmo valor da diferença aparece primeiro. Os textos não levam valores em reais: a tela mostra o Valor pelo
// pipe que respeita "ocultar valores".
package conferenciabanco

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/bancos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cambio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/carteira"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/contas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/faturas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/reservas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/servidor"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

const (
	// diasDasComprasRepetidas é até quantos dias para trás as compras repetidas no mesmo dia são procuradas nos cartões.
	diasDasComprasRepetidas = 45

	tipoContaCartao      = 3
	origemOpenFinance    = 2
	tipoDespesa          = 1
	statusPendente       = 2
	rotaDasTransacoes    = "/transacoes"
	rotaDosInvestimentos = "/investimentos"
	formatoDaData        = "02/01/2006"
)

// centavo: a dívida acima da fatura por menos que isso é arredondamento.
var centavo = decimalDe("0.01")

func decimalDe(texto string) tipos.Decimal {
	d, err := tipos.NovoDecimal(texto)
	if err != nil {
		panic(err) // só com constantes do pacote
	}
	return d
}

// listaDeReservas e as outras interfaces são o que este pacote usa dos outros assuntos.
type listaDeReservas interface {
	Listar(ctx context.Context) ([]reservas.ReservaDto, error)
}

type conversao interface {
	ObterConversor(ctx context.Context, data *tipos.Data) (*cambio.Conversor, error)
}

type valorDaCarteira interface {
	ValorDaCarteira(ctx context.Context) (tipos.Decimal, error)
}

// Servico monta a conferência.
type Servico struct {
	db       *sql.DB
	reservas listaDeReservas
	carteira valorDaCarteira
	cambio   conversao
	relogio  relogio.Relogio
	log      *slog.Logger
}

// NovoServico monta o serviço com as dependências.
func NovoServico(db *sql.DB, r listaDeReservas, c valorDaCarteira, m conversao, rel relogio.Relogio, log *slog.Logger) *Servico {
	return &Servico{db, r, c, m, rel, log}
}

// PistaDto é o PistaConferenciaDto.
type PistaDto struct {
	Texto      string            `json:"texto"`
	Valor      *tipos.Decimal    `json:"valor"`
	Rota       *string           `json:"rota"`
	Parametros map[string]string `json:"parametros"`
}

// ContaDto é o ContaConferenciaDto.
type ContaDto struct {
	Id          int64         `json:"id"`
	Nome        string        `json:"nome"`
	Banco       *string       `json:"banco"`
	OpenFinance bool          `json:"openFinance"`
	Saldo       tipos.Decimal `json:"saldo"`
	Pistas      []PistaDto    `json:"pistas"`
}

// CartaoDto é o CartaoConferenciaDto: a fatura que fecha no mês (a mesma do Painel) e a dívida total do cartão.
type CartaoDto struct {
	Id          int64         `json:"id"`
	Nome        string        `json:"nome"`
	Banco       *string       `json:"banco"`
	FaturaMes   tipos.Decimal `json:"faturaMes"`
	Situacao    *string       `json:"situacao"`
	Fechamento  *tipos.Data   `json:"fechamento"`
	Vencimento  *tipos.Data   `json:"vencimento"`
	DividaTotal tipos.Decimal `json:"dividaTotal"`
	Pistas      []PistaDto    `json:"pistas"`
}

// InvestimentoDto é o InvestimentoConferenciaDto: do Open Finance, reserva ou a carteira lançada por você, em reais.
type InvestimentoDto struct {
	Chave  string        `json:"chave"`
	Nome   string        `json:"nome"`
	Origem string        `json:"origem"`
	Saldo  tipos.Decimal `json:"saldo"`
	Pistas []PistaDto    `json:"pistas"`
}

// ConferenciaDto é o ConferenciaBancoDto.
type ConferenciaDto struct {
	Contas        []ContaDto        `json:"contas"`
	Cartoes       []CartaoDto       `json:"cartoes"`
	Investimentos []InvestimentoDto `json:"investimentos"`
}

func pista(texto string) PistaDto { return PistaDto{Texto: texto} }

func comValor(texto string, valor tipos.Decimal) PistaDto {
	return PistaDto{Texto: texto, Valor: &valor}
}

func comLink(p PistaDto, rota string, parametros map[string]string) PistaDto {
	p.Rota, p.Parametros = &rota, parametros
	return p
}

func daConta(p PistaDto, id int64) PistaDto {
	return comLink(p, rotaDasTransacoes, map[string]string{"conta": strconv.FormatInt(id, 10)})
}

func nasReservas(p PistaDto) PistaDto {
	return comLink(p, rotaDosInvestimentos, map[string]string{"aba": "reservas"})
}

func quantos(quantidade int, um, varios string) string {
	if quantidade == 1 {
		return "1 " + um
	}
	return strconv.Itoa(quantidade) + " " + varios
}

// dataLocal é a data no fuso de Brasília de um instante gravado em UTC.
func dataLocal(instante tipos.DataHora) string {
	return relogio.ParaHoraLocal(instante.Tempo()).Format(formatoDaData)
}

// numeroN4 é o ToString("N4") da cultura pt-BR: 4 casas, com o ponto no milhar.
func numeroN4(d tipos.Decimal) string {
	inteiro, casas, _ := strings.Cut(d.Decimal().StringFixed(4), ".")
	var grupos []string
	for len(inteiro) > 3 {
		grupos = append([]string{inteiro[len(inteiro)-3:]}, grupos...)
		inteiro = inteiro[:len(inteiro)-3]
	}
	return strings.Join(append([]string{inteiro}, grupos...), ".") + "," + casas
}

func valorAbsoluto(d tipos.Decimal) tipos.Decimal { return tipos.DeDecimal(d.Decimal().Abs()) }

func positivo(d tipos.Decimal) bool { return d.Decimal().IsPositive() }

// contaLida é a conta com o que a tela precisa.
type contaLida struct {
	id, tipo, origem  int64
	nome              string
	atualizadoEm      tipos.DataHora
	atualizadoNoBanco *tipos.DataHora
}

func (s *Servico) lerContas(ctx context.Context) ([]contaLida, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT c."Id", c."Nome", c."Tipo", c."Origem", c."AtualizadoEm", x."UltimaAtualizacaoInstituicao"
		FROM "Conta" AS c LEFT JOIN "ConexaoBancaria" AS x ON c."ConexaoBancariaId" = x."Id" WHERE NOT c."Arquivada" ORDER BY c."Nome"`)
	if err != nil {
		return nil, fmt.Errorf("ler as contas: %w", err)
	}
	defer linhas.Close()
	var lista []contaLida
	for linhas.Next() {
		var c contaLida
		if err := linhas.Scan(&c.id, &c.nome, &c.tipo, &c.origem, &c.atualizadoEm, &c.atualizadoNoBanco); err != nil {
			return nil, fmt.Errorf("ler conta: %w", err)
		}
		lista = append(lista, c)
	}
	return lista, linhas.Err()
}

// pendencia é o que uma conta tem de lançamentos pendentes.
type pendencia struct {
	quantidade int
	total      tipos.Decimal
}

func (s *Servico) lerPendentes(ctx context.Context) (map[int64]pendencia, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT "ContaId", "Valor" FROM "Transacao" WHERE "Status" = ?`, statusPendente)
	if err != nil {
		return nil, fmt.Errorf("ler os pendentes: %w", err)
	}
	defer linhas.Close()
	pendentes := map[int64]pendencia{}
	for linhas.Next() {
		var conta int64
		var valor tipos.Decimal
		if err := linhas.Scan(&conta, &valor); err != nil {
			return nil, fmt.Errorf("ler pendente: %w", err)
		}
		p, tem := pendentes[conta]
		if !tem {
			p.total = tipos.Zero
		}
		pendentes[conta] = pendencia{p.quantidade + 1, tipos.Somar(p.total, valor)}
	}
	return pendentes, linhas.Err()
}

// movimento é um lançamento de cartão, para as pistas de estorno e de compra repetida.
type movimento struct {
	conta, tipo int64
	data        tipos.Data
	valor       tipos.Decimal
	descricao   string
	parcela     *int64
}

func (s *Servico) lerMovimentosDeCartao(ctx context.Context, desde tipos.Data) ([]movimento, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT t."ContaId", t."Data", t."Valor", t."Descricao", t."Tipo", t."ParcelaNumero" FROM "Transacao" AS t
		INNER JOIN "Conta" AS c ON t."ContaId" = c."Id" WHERE c."Tipo" = ? AND t."Data" >= ? AND t."TransferenciaGrupo" IS NULL`, tipoContaCartao, desde)
	if err != nil {
		return nil, fmt.Errorf("ler os lançamentos dos cartões: %w", err)
	}
	defer linhas.Close()
	var lista []movimento
	for linhas.Next() {
		var m movimento
		if err := linhas.Scan(&m.conta, &m.data, &m.valor, &m.descricao, &m.tipo, &m.parcela); err != nil {
			return nil, fmt.Errorf("ler lançamento do cartão: %w", err)
		}
		lista = append(lista, m)
	}
	return lista, linhas.Err()
}

// pistasDaConta são as pistas de uma conta (sem os cartões).
func pistasDaConta(c contaLida, lista []reservas.ReservaDto, pendentes map[int64]pendencia) []PistaDto {
	var pistas []PistaDto
	if c.origem == origemOpenFinance {
		em := c.atualizadoEm
		if c.atualizadoNoBanco != nil {
			em = *c.atualizadoNoBanco
		}
		pistas = append(pistas, pista("O saldo vem do banco, atualizado em "+dataLocal(em)+". O que entrou ou saiu depois ainda não chegou."))
	} else {
		pistas = append(pistas, daConta(pista("Conta manual: o saldo é o inicial mais os lançamentos. Falta algum, ou há um a mais?"), c.id))
	}
	reservado := tipos.Zero
	for _, r := range lista {
		if r.ContaId == c.id {
			reservado = tipos.Somar(reservado, r.Saldo)
		}
	}
	if positivo(reservado) {
		pistas = append(pistas, nasReservas(comValor("Reservas e caixinhas desta conta (o app pode mostrar o saldo com elas)", reservado)))
	}
	if p, tem := pendentes[c.id]; tem {
		pistas = append(pistas, daConta(comValor(quantos(p.quantidade, "lançamento pendente", "lançamentos pendentes")+" (o app pode já contar ou não)", valorAbsoluto(p.total)), c.id))
	}
	return pistas
}

// estornosDoMes são os créditos do cartão no mês que contam como despesa.
func estornosDoMes(doCartao []movimento, inicio, fim tipos.Data) tipos.Decimal {
	soma := tipos.Zero
	for _, m := range doCartao {
		if positivo(m.valor) && m.tipo == tipoDespesa && !m.data.Antes(inicio) && !m.data.Depois(fim) {
			soma = tipos.Somar(soma, m.valor)
		}
	}
	return soma
}

// comprasRepetidas é quantos grupos de compras têm o mesmo dia, valor e descrição, e o que veio a mais neles. Parcelas ficam
// de fora.
func comprasRepetidas(doCartao []movimento) (int, tipos.Decimal) {
	type grupo struct {
		valor      tipos.Decimal
		quantidade int64
	}
	var chaves []string
	grupos := map[string]*grupo{}
	for _, m := range doCartao {
		if !m.valor.Decimal().IsNegative() || m.parcela != nil {
			continue
		}
		chave := m.data.String() + "|" + m.valor.Decimal().String() + "|" + texto.MaiusculasInvariantes(strings.TrimSpace(m.descricao))
		g, tem := grupos[chave]
		if !tem {
			g = &grupo{valor: m.valor}
			grupos[chave] = g
			chaves = append(chaves, chave)
		}
		g.quantidade++
	}
	repetidas, aMais := 0, tipos.Zero
	for _, chave := range chaves {
		if g := grupos[chave]; g.quantidade > 1 {
			repetidas++
			aMais = tipos.Somar(aMais, tipos.Multiplicar(tipos.Subtrair(tipos.Zero, g.valor), tipos.DecimalDeInteiro(g.quantidade-1)))
		}
	}
	return repetidas, aMais
}

func pistasDoCartao(id int64, fatura *faturas.DoMes, valorFatura, divida tipos.Decimal, pendentes map[int64]pendencia, doCartao []movimento, inicio, fim tipos.Data) []PistaDto {
	var pistas []PistaDto
	if fatura != nil && fatura.Situacao != nil && (*fatura.Situacao == faturas.Fechada || *fatura.Situacao == faturas.Paga) {
		pistas = append(pistas, pista("Fatura "+*fatura.Situacao+": vale o total que o banco informou."))
	} else {
		pistas = append(pistas, pista("Fatura aberta: é a soma das compras do ciclo até agora. O app pode já mostrar compras que ainda não chegaram."))
	}
	if sobra := tipos.Subtrair(divida, valorFatura); sobra.Decimal().GreaterThan(centavo.Decimal()) {
		pistas = append(pistas, comValor("Dívida total do cartão, com as parcelas futuras, menos a fatura do mês (se o app mostra a dívida total)", sobra))
	}
	if p, tem := pendentes[id]; tem {
		pistas = append(pistas, daConta(comValor(quantos(p.quantidade, "compra pendente", "compras pendentes")+" (o app pode já contar na fatura)", valorAbsoluto(p.total)), id))
	}
	if fatura != nil && positivo(fatura.ParcelasMes) {
		pistas = append(pistas, comValor("Parcelas nesta fatura (o app pode mostrá-las à parte)", fatura.ParcelasMes))
	}
	if estornos := estornosDoMes(doCartao, inicio, fim); positivo(estornos) {
		pistas = append(pistas, daConta(comValor("Estornos no mês (o site já desconta; o app pode mostrar a fatura antes deles)", estornos), id))
	}
	if repetidas, aMais := comprasRepetidas(doCartao); repetidas > 0 {
		pistas = append(pistas, daConta(comValor(quantos(repetidas, "compra repetida", "compras repetidas")+
			" no mesmo dia e com o mesmo valor: pode ser compra em dobro de verdade ou importada duas vezes (o que veio a mais)", aMais), id))
	}
	return pistas
}

// investimentoLido é um investimento do Open Finance.
type investimentoLido struct {
	id                int64
	nome, tipo, moeda string
	emissor, personal *string
	saldo             tipos.Decimal
	atualizadoEm      tipos.DataHora
	atualizadoNoBanco *tipos.DataHora
}

func (s *Servico) lerInvestimentos(ctx context.Context) ([]investimentoLido, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT i."Id", i."Nome", i."Tipo", i."Emissor", i."NomePersonalizado", i."Saldo", i."Moeda", i."AtualizadoEm",
		c."UltimaAtualizacaoInstituicao" FROM "Investimento" AS i INNER JOIN "ConexaoBancaria" AS c ON i."ConexaoBancariaId" = c."Id" WHERE c."Ativa"`)
	if err != nil {
		return nil, fmt.Errorf("ler os investimentos: %w", err)
	}
	defer linhas.Close()
	var lista []investimentoLido
	for linhas.Next() {
		var i investimentoLido
		if err := linhas.Scan(&i.id, &i.nome, &i.tipo, &i.emissor, &i.personal, &i.saldo, &i.moeda, &i.atualizadoEm, &i.atualizadoNoBanco); err != nil {
			return nil, fmt.Errorf("ler investimento: %w", err)
		}
		lista = append(lista, i)
	}
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("ler os investimentos: %w", err)
	}
	slices.SortStableFunc(lista, func(a, b investimentoLido) int { return b.saldo.Decimal().Cmp(a.saldo.Decimal()) })
	return lista, nil
}

func dtoDoOpenFinance(i investimentoLido, conversor *cambio.Conversor) InvestimentoDto {
	em := i.atualizadoEm
	if i.atualizadoNoBanco != nil {
		em = *i.atualizadoNoBanco
	}
	pistas := []PistaDto{pista("Valor do banco, atualizado em " + dataLocal(em) + ". O Meu Pluggy atualiza cerca de uma vez por dia: o rendimento de hoje pode não ter chegado.")}
	if i.moeda != cambio.MoedaBase {
		pistas = append(pistas, pista("Em "+i.moeda+", convertido pela PTAX; o app usa a cotação dele."))
	}
	saldo := tipos.Zero
	if r := conversor.ParaReais(i.saldo, i.moeda); r != nil {
		saldo = tipos.Arredondar(*r, 2)
	}
	return InvestimentoDto{"of-" + strconv.FormatInt(i.id, 10), carteira.NomeDeExibicao(i.nome, i.tipo, i.emissor, i.personal), "Open Finance", saldo, pistas}
}

func dtoDaReserva(r reservas.ReservaDto) InvestimentoDto {
	var pistas []PistaDto
	switch {
	case r.InformadoPeloBanco:
		pistas = append(pistas, pista("Valor que o banco informa pelo Open Finance."))
	case r.RestanteDoBanco:
		pistas = append(pistas, nasReservas(pista("Parte do total reservado que o banco informa pelo Open Finance e que não está nas outras reservas da conta. Conferir as outras ajusta este valor.")))
	case r.DataConferencia != nil:
		dia := time.Date(r.DataConferencia.Ano(), r.DataConferencia.Mes(), r.DataConferencia.Dia(), 0, 0, 0, 0, time.UTC)
		pistas = append(pistas, nasReservas(pista("Conferido em "+dia.Format(formatoDaData)+" e, depois, estimado pelos lançamentos. O rendimento não aparece no extrato: confira de novo.")))
	default:
		pistas = append(pistas, nasReservas(pista("Estimado pelo extrato, sem conferência: pode faltar o dinheiro de antes do primeiro lançamento e o rendimento.")))
	}
	if r.ForaDaListaDoBanco {
		pistas = append(pistas, nasReservas(pista("Não está na lista de reservas que o banco informa. Se ela já aparece com outro nome, confira esta com zero.")))
	}
	if r.Moeda != cambio.MoedaBase && r.Cotacao != nil {
		pistas = append(pistas, pista("Em dólar, pela PTAX de "+numeroN4(*r.Cotacao)+" (a cotação do Banco Central); o app usa a cotação dele."))
	}
	return InvestimentoDto{"reserva-" + strconv.FormatInt(r.ContaId, 10) + "-" + r.Nome, r.Nome + " (" + r.NomeConta + ")", "Reserva", r.Saldo, pistas}
}

func dtoDaCarteira(valor tipos.Decimal) InvestimentoDto {
	return InvestimentoDto{"carteira", "Carteira de renda variável (lançada por você)", "Carteira", valor, []PistaDto{
		pista("Pela última cotação guardada; o app da corretora usa a cotação do momento."),
		comLink(pista("Cotas diferentes das da corretora aparecem em “Conferir investimentos”."), rotaDosInvestimentos, map[string]string{"conferir": "1"})}}
}

// investimentos é o que o Open Finance informa, as reservas e a carteira lançada por você.
func (s *Servico) investimentos(ctx context.Context, lista []reservas.ReservaDto) ([]InvestimentoDto, error) {
	conversor, err := s.cambio.ObterConversor(ctx, nil)
	if err != nil {
		return nil, err
	}
	doOpenFinance, err := s.lerInvestimentos(ctx)
	if err != nil {
		return nil, err
	}
	dtos := []InvestimentoDto{}
	for _, i := range doOpenFinance {
		dtos = append(dtos, dtoDoOpenFinance(i, conversor))
	}
	for _, r := range lista {
		dtos = append(dtos, dtoDaReserva(r))
	}
	valor, err := s.carteira.ValorDaCarteira(ctx)
	if err != nil {
		return nil, err
	}
	if positivo(valor) {
		dtos = append(dtos, dtoDaCarteira(valor))
	}
	return dtos, nil
}

// Montar é o MontarAsync do ConferenciaBancoService.
func (s *Servico) Montar(ctx context.Context) (*ConferenciaDto, error) {
	hoje := relogio.Hoje(s.relogio)
	saldos, err := contas.Saldos(ctx, s.db)
	if err != nil {
		return nil, err
	}
	nomesDosBancos, err := bancos.Carregar(ctx, s.db)
	if err != nil {
		return nil, err
	}
	listaDeReservas, err := s.reservas.Listar(ctx)
	if err != nil {
		return nil, err
	}
	lidas, err := s.lerContas(ctx)
	if err != nil {
		return nil, err
	}
	pendentes, err := s.lerPendentes(ctx)
	if err != nil {
		return nil, err
	}
	// Cartões: a fatura que fecha neste mês (a mesma do Painel e de Contas e cartões).
	inicio, fim := relogio.PrimeiroDiaMes(hoje.Ano(), hoje.Mes()), relogio.UltimoDiaMes(hoje.Ano(), hoje.Mes())
	doMes, err := faturas.DoMesDosCartoes(ctx, s.db, inicio, fim, hoje)
	if err != nil {
		return nil, err
	}
	movimentos, err := s.lerMovimentosDeCartao(ctx, hoje.SomarDias(-diasDasComprasRepetidas))
	if err != nil {
		return nil, err
	}
	d := &ConferenciaDto{Contas: []ContaDto{}, Cartoes: []CartaoDto{}}
	for _, c := range lidas {
		saldo, tem := saldos[c.id]
		if !tem {
			saldo = tipos.Zero
		}
		if c.tipo != tipoContaCartao {
			d.Contas = append(d.Contas, ContaDto{c.id, c.nome, nomesDosBancos[c.id], c.origem == origemOpenFinance, saldo, pistasDaConta(c, listaDeReservas, pendentes)})
			continue
		}
		var fatura *faturas.DoMes
		if i := slices.IndexFunc(doMes, func(f faturas.DoMes) bool { return f.ContaId == c.id }); i >= 0 {
			fatura = &doMes[i]
		}
		doCartao := slices.DeleteFunc(slices.Clone(movimentos), func(m movimento) bool { return m.conta != c.id })
		cartao := CartaoDto{Id: c.id, Nome: c.nome, Banco: nomesDosBancos[c.id], FaturaMes: tipos.Zero, DividaTotal: tipos.Maior(tipos.Zero, tipos.Subtrair(tipos.Zero, saldo))}
		if fatura != nil {
			cartao.FaturaMes, cartao.Situacao, cartao.Fechamento, cartao.Vencimento = fatura.ValorMes, fatura.Situacao, fatura.Fechamento, fatura.Vencimento
		}
		cartao.Pistas = pistasDoCartao(c.id, fatura, cartao.FaturaMes, cartao.DividaTotal, pendentes, doCartao, inicio, fim)
		d.Cartoes = append(d.Cartoes, cartao)
	}
	if d.Investimentos, err = s.investimentos(ctx, listaDeReservas); err != nil {
		return nil, err
	}
	return d, nil
}

// Registrar liga a rota da conferência.
func Registrar(mux *http.ServeMux, s *Servico) {
	mux.HandleFunc("GET /api/conferencia-banco", s.obter)
}

func (s *Servico) obter(w http.ResponseWriter, r *http.Request) {
	d, err := s.Montar(r.Context())
	if errors.Is(err, context.Canceled) || r.Context().Err() != nil {
		return
	}
	if err != nil {
		s.log.ErrorContext(r.Context(), "erro na conferência do banco", "origem", "API", "erro", err)
		servidor.ResponderErroInterno(w)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, d)
}
