package painel

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/contas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/extrato"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relatorios"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
)

const (
	tipoContaCartao = 3
	// diasMaximoDoPeriodo é o maior período dos relatórios de categorias e de fluxo (uns 10 anos).
	diasMaximoDoPeriodo = 3660
	maioresDespesas     = 15
)

// nomesDoTipo são os nomes do enum TipoTransacao, em minúsculas, para a query (?tipo=Receita).
var nomesDoTipo = map[string]int64{"despesa": transacoes.Despesa, "receita": transacoes.Receita,
	"transferencia": transacoes.Transferencia}

// consulta lê a query como o model binding do ASP.NET: cada parâmetro que não vira o tipo é um erro do 400 automático,
// com o nome do parâmetro.
type consulta struct {
	r     *http.Request
	erros map[string][]string
}

func (c *consulta) data(nome string) tipos.Data {
	valor, ok := respostas.ParametroDaQuery(c.r, nome)
	if !ok {
		return tipos.NovaData(1, 1, 1) // o DateOnly sem valor
	}
	if strings.TrimSpace(valor) == "" {
		c.erros[nome] = []string{respostas.MensagemDeValorInvalido(valor)}
		return tipos.NovaData(1, 1, 1)
	}
	d, ok := respostas.DataDaQuery(valor)
	if !ok {
		c.erros[nome] = []string{respostas.MensagemDaQuery(valor)}
	}
	return d
}

func (c *consulta) inteiro(nome string, padrao int) int {
	valor, ok := respostas.ParametroDaQuery(c.r, nome)
	if !ok {
		return padrao
	}
	n, ok := respostas.InteiroDaQuery(valor)
	if !ok {
		c.erros[nome] = []string{respostas.MensagemDaQuery(valor)}
		return padrao
	}
	return int(n)
}

// inteiroOpcional é o int? da query: vazio é null.
func (c *consulta) inteiroOpcional(nome string) *int {
	valor, ok := respostas.ParametroDaQuery(c.r, nome)
	if !ok || strings.TrimSpace(valor) == "" {
		return nil
	}
	n, ok := respostas.InteiroDaQuery(valor)
	if !ok {
		c.erros[nome] = []string{respostas.MensagemDaQuery(valor)}
		return nil
	}
	v := int(n)
	return &v
}

func (c *consulta) booleano(nome string, padrao bool) bool {
	valor, ok := respostas.ParametroDaQuery(c.r, nome)
	if !ok {
		return padrao
	}
	b, ok := respostas.BoolDaQuery(valor)
	if !ok {
		c.erros[nome] = []string{respostas.MensagemDaQuery(valor)}
		return padrao
	}
	return b
}

func (c *consulta) tipo(nome string, padrao int64) int64 {
	valor, ok := respostas.ParametroDaQuery(c.r, nome)
	if !ok {
		return padrao
	}
	v, converte, definido := respostas.EnumDaQuery(valor, nomesDoTipo)
	switch {
	case !converte:
		c.erros[nome] = []string{respostas.MensagemDaQuery(valor)}
	case !definido:
		c.erros[nome] = []string{respostas.MensagemDeValorInvalido(valor)}
	default:
		return v
	}
	return padrao
}

// ok responde o 400 automático, se houve erro de leitura. Devolve false se já respondeu.
func (c *consulta) ok(w http.ResponseWriter) bool {
	if len(c.erros) > 0 {
		respostas.Validacao(w, c.erros)
		return false
	}
	return true
}

func novaConsulta(r *http.Request) *consulta { return &consulta{r: r, erros: map[string][]string{}} }

// periodoInvalido: o fim antes do início ou mais de 3660 dias entre os dois.
func periodoInvalido(inicio, fim tipos.Data) bool {
	return fim.NumeroDoDia() < inicio.NumeroDoDia() || fim.NumeroDoDia()-inicio.NumeroDoDia() > diasMaximoDoPeriodo
}

func (s *Servico) porCategoria(w http.ResponseWriter, r *http.Request) {
	c := novaConsulta(r)
	inicio, fim := c.data("inicio"), c.data("fim")
	tipo := c.tipo("tipo", transacoes.Despesa)
	agrupar := c.booleano("agrupar", true)
	if !c.ok(w) {
		return
	}
	if periodoInvalido(inicio, fim) {
		respostas.Mensagem(w, http.StatusBadRequest, "Período inválido.")
		return
	}
	if tipo == transacoes.Transferencia {
		respostas.Mensagem(w, http.StatusBadRequest, "Tipo inválido.")
		return
	}
	lista, err := s.relatorios.TotaisPorCategoria(r.Context(), inicio, fim, tipo, agrupar, false)
	if err != nil {
		s.erroInterno(w, r, "por categoria", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, lista)
}

func (s *Servico) fluxo(w http.ResponseWriter, r *http.Request) {
	c := novaConsulta(r)
	inicio, fim := c.data("inicio"), c.data("fim")
	if !c.ok(w) {
		return
	}
	if periodoInvalido(inicio, fim) {
		respostas.Mensagem(w, http.StatusBadRequest, "Período inválido.")
		return
	}
	lista, err := s.relatorios.FluxoMensal(r.Context(), inicio, fim)
	if err != nil {
		s.erroInterno(w, r, "fluxo", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, lista)
}

func (s *Servico) anual(w http.ResponseWriter, r *http.Request) {
	c := novaConsulta(r)
	ano, comparar := c.inteiroOpcional("ano"), c.inteiroOpcional("comparar")
	if !c.ok(w) {
		return
	}
	a := relogio.Hoje(s.relogio).Ano()
	if ano != nil {
		a = *ano
	}
	cmp := a - 1
	if comparar != nil {
		cmp = *comparar
	}
	if a < 2000 || a > 2100 || cmp < 2000 || cmp > 2100 {
		respostas.Mensagem(w, http.StatusBadRequest, "Ano inválido.")
		return
	}
	patrimonio, err := s.patrimonio(r.Context())
	if err != nil {
		s.erroInterno(w, r, "patrimônio", err)
		return
	}
	painel, err := s.relatorios.PainelAnual(r.Context(), a, cmp, patrimonio)
	if err != nil {
		s.erroInterno(w, r, "visão anual", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, painel)
}

func (s *Servico) projecao(w http.ResponseWriter, r *http.Request) {
	c := novaConsulta(r)
	meses := c.inteiro("meses", 6)
	if !c.ok(w) {
		return
	}
	if meses < 1 || meses > 24 {
		respostas.Mensagem(w, http.StatusBadRequest, "Informe entre 1 e 24 meses.")
		return
	}
	saldos, err := contas.Saldos(r.Context(), s.db)
	if err != nil {
		s.erroInterno(w, r, "projeção", err)
		return
	}
	p, err := s.relatorios.Projecao(r.Context(), meses, saldos)
	if err != nil {
		s.erroInterno(w, r, "projeção", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, p)
}

func (s *Servico) maioresDespesas(w http.ResponseWriter, r *http.Request) {
	c := novaConsulta(r)
	inicio, fim := c.data("inicio"), c.data("fim")
	if !c.ok(w) {
		return
	}
	if fim.NumeroDoDia() < inicio.NumeroDoDia() {
		respostas.Mensagem(w, http.StatusBadRequest, "Período inválido.")
		return
	}
	lista, err := extrato.MaioresDespesas(r.Context(), s.db, inicio, fim, maioresDespesas)
	if err != nil {
		s.erroInterno(w, r, "maiores despesas", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, lista)
}

// patrimonio é o PatrimonioAtualAsync do C# e a reserva líquida da saúde financeira, com os totais de crédito:
//   - patrimônio: as contas que entram no saldo (o cartão só com a dívida: −max(0, −saldo)) mais os investimentos
//     (carteira, Open Finance e reservas) menos o saldo devedor de financiamentos e de empréstimos;
//   - reserva líquida: as contas que não são cartão (só o saldo positivo), mais os investimentos do Open Finance das
//     conexões ativas que não são renda variável (em reais; sem cotação, fica de fora), mais as reservas (#172).
func (s *Servico) patrimonio(ctx context.Context) (relatorios.Patrimonio, error) {
	credito, err := s.emprestimos.Totais(ctx)
	if err != nil {
		return relatorios.Patrimonio{}, err
	}
	saldos, err := contas.Saldos(ctx, s.db)
	if err != nil {
		return relatorios.Patrimonio{}, err
	}
	lista, err := s.relatorios.ContasNoSaldo(ctx)
	if err != nil {
		return relatorios.Patrimonio{}, err
	}
	saldo := func(id int64) tipos.Decimal {
		if v, ok := saldos[id]; ok {
			return v
		}
		return tipos.Zero
	}
	nasContas, positivas := tipos.Zero, tipos.Zero
	for _, c := range lista {
		if c.Tipo == tipoContaCartao {
			nasContas = tipos.Somar(nasContas, tipos.Subtrair(tipos.Zero, tipos.Maior(tipos.Zero, tipos.Subtrair(tipos.Zero, saldo(c.Id)))))
			continue
		}
		nasContas = tipos.Somar(nasContas, saldo(c.Id))
		positivas = tipos.Somar(positivas, tipos.Maior(tipos.Zero, saldo(c.Id)))
	}
	composicao, err := s.carteira.Composicao(ctx)
	if err != nil {
		return relatorios.Patrimonio{}, err
	}
	atual := tipos.Subtrair(tipos.Subtrair(tipos.Somar(nasContas, composicao.Total()), credito.SaldoFinanciamentos),
		credito.SaldoEmprestimos)
	rendaFixa, err := s.rendaFixaDoOpenFinance(ctx)
	if err != nil {
		return relatorios.Patrimonio{}, err
	}
	reservas, err := s.reservas.Total(ctx)
	if err != nil {
		return relatorios.Patrimonio{}, err
	}
	return relatorios.Patrimonio{
		Atual: atual, Reserva: tipos.Somar(tipos.Somar(positivas, rendaFixa), reservas),
		Credito: relatorios.Credito{SaldoFinanciamentos: credito.SaldoFinanciamentos, SaldoEmprestimos: credito.SaldoEmprestimos,
			ParcelaMensalFinanciamentos: credito.ParcelaMensalFinanciamentos, ParcelaMensalEmprestimos: credito.ParcelaMensalEmprestimos},
	}, nil
}

// rendaFixaDoOpenFinance soma, em reais, os investimentos das conexões ativas que não são renda variável (ações e
// ETFs). Sem cotação da moeda, o investimento fica de fora.
func (s *Servico) rendaFixaDoOpenFinance(ctx context.Context) (tipos.Decimal, error) {
	conversor, err := s.cambio.ObterConversor(ctx, nil)
	if err != nil {
		return tipos.Zero, err
	}
	linhas, err := s.db.QueryContext(ctx, `SELECT i."Saldo", i."Moeda", i."Tipo" FROM "Investimento" i
		JOIN "ConexaoBancaria" c ON c."Id" = i."ConexaoBancariaId" WHERE c."Ativa" ORDER BY i."Id"`)
	if err != nil {
		return tipos.Zero, fmt.Errorf("ler os investimentos da reserva: %w", err)
	}
	defer linhas.Close()
	soma := tipos.Zero
	for linhas.Next() {
		var saldo tipos.Decimal
		var moeda, tipo string
		if err := linhas.Scan(&saldo, &moeda, &tipo); err != nil {
			return tipos.Zero, fmt.Errorf("ler investimento da reserva: %w", err)
		}
		if t := texto.MaiusculasInvariantes(tipo); t == "EQUITY" || t == "ETF" {
			continue
		}
		emReais := tipos.Zero
		if v := conversor.ParaReais(saldo, moeda); v != nil {
			emReais = *v
		}
		soma = tipos.Somar(soma, emReais)
	}
	return soma, linhas.Err()
}
