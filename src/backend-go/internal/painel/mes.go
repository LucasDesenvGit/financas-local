package painel

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/carteira"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/configuracao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/contas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/emprestimos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/extrato"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/faturas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/poucoinformativa"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relatorios"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
)

const (
	tipoContaInvestimento = 4
	// diasPoucoInformativas é a janela do aviso de descrições pouco informativas (#9).
	diasPoucoInformativas = 90
	// transacoesNoPainel é quantas últimas (e quantas futuras) o painel mostra.
	transacoesNoPainel = 8
	// orcamentosNoPainel é quantos orçamentos o card do painel mostra.
	orcamentosNoPainel = 6
	// tamanhoDoExemplo é o maior exemplo de descrição pouco informativa, em chars (UTF-16), como no C#.
	tamanhoDoExemplo = 60
	nivelErro        = 4 // NivelErro.Erro: Erro e Crítico entram no aviso; Aviso, não

	chaveDiasVencimentos     = "Painel.DiasProximosVencimentos"
	chaveDiasConsentimento   = "Painel.DiasAvisoConsentimento"
	chaveUltimaSincronizacao = "Sincronizacao.UltimaExecucao"
)

// PainelDto é o PainelDto do C#.
type PainelDto struct {
	SaldoContas                       tipos.Decimal                  `json:"saldoContas"`
	DividaCartoes                     tipos.Decimal                  `json:"dividaCartoes"`
	TotalInvestido                    tipos.Decimal                  `json:"totalInvestido"`
	PatrimonioLiquido                 tipos.Decimal                  `json:"patrimonioLiquido"`
	RendaVariavel                     tipos.Decimal                  `json:"rendaVariavel"`
	PatrimonioSemRendaVariavel        tipos.Decimal                  `json:"patrimonioSemRendaVariavel"`
	ReceitasMes                       tipos.Decimal                  `json:"receitasMes"`
	DespesasMes                       tipos.Decimal                  `json:"despesasMes"`
	GastosPorCategoria                []relatorios.ValorCategoriaDto `json:"gastosPorCategoria"`
	Evolucao                          []relatorios.MesValoresDto     `json:"evolucao"`
	Orcamentos                        []relatorios.OrcamentoDto      `json:"orcamentos"`
	ProximasContas                    []relatorios.ContaAgendadaDto  `json:"proximasContas"`
	UltimasTransacoes                 []extrato.TransacaoDto         `json:"ultimasTransacoes"`
	Alertas                           []string                       `json:"alertas"`
	UltimaSincronizacao               *configuracao.Instante         `json:"ultimaSincronizacao"`
	DespesasEmergenciaMes             tipos.Decimal                  `json:"despesasEmergenciaMes"`
	DividaFinanciamentos              tipos.Decimal                  `json:"dividaFinanciamentos"`
	DividaEmprestimos                 tipos.Decimal                  `json:"dividaEmprestimos"`
	DiasProximosVencimentos           int                            `json:"diasProximosVencimentos"`
	FaturasDoMes                      []faturas.DoMes                `json:"faturasDoMes"`
	TotalFaturasDoMes                 tipos.Decimal                  `json:"totalFaturasDoMes"`
	PoucoInformativas                 int                            `json:"poucoInformativas"`
	PoucoInformativasDesde            *tipos.Data                    `json:"poucoInformativasDesde"`
	ExemploPoucoInformativa           *string                        `json:"exemploPoucoInformativa"`
	CreditosCorretoraComoReceita      int                            `json:"creditosCorretoraComoReceita"`
	TotalCreditosCorretoraComoReceita tipos.Decimal                  `json:"totalCreditosCorretoraComoReceita"`
	TrocosAutomaticos                 *carteira.TrocosAutomaticosDto `json:"trocosAutomaticos"`
	OrcamentosEstourados              []relatorios.OrcamentoDto      `json:"orcamentosEstourados"`
	TransacoesFuturas                 []extrato.TransacaoDto         `json:"transacoesFuturas"`
	QuantidadeFuturas                 int                            `json:"quantidadeFuturas"`
}

// AvisosDto são os avisos da barra do topo (#327).
type AvisosDto struct {
	Ano                     int                       `json:"ano"`
	Mes                     int                       `json:"mes"`
	Alertas                 []string                  `json:"alertas"`
	OrcamentosEstourados    []relatorios.OrcamentoDto `json:"orcamentosEstourados"`
	PoucoInformativas       int                       `json:"poucoInformativas"`
	PoucoInformativasDesde  *tipos.Data               `json:"poucoInformativasDesde"`
	ExemploPoucoInformativa *string                   `json:"exemploPoucoInformativa"`
}

func (s *Servico) obter(w http.ResponseWriter, r *http.Request) {
	c := novaConsulta(r)
	ano, mes := c.inteiroOpcional("ano"), c.inteiroOpcional("mes")
	if !c.ok(w) {
		return
	}
	hoje := relogio.Hoje(s.relogio)
	a, m := hoje.Ano(), int(hoje.Mes())
	if ano != nil {
		a = *ano
	}
	if mes != nil {
		m = *mes
	}
	if a < 2000 || a > 2100 || m < 1 || m > 12 {
		respostas.Mensagem(w, http.StatusBadRequest, "Período inválido.")
		return
	}
	p, err := s.Painel(r.Context(), a, m)
	if err != nil {
		s.erroInterno(w, r, "painel", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, p)
}

// Painel é o Obter do PainelController: o resumo do mês escolhido.
//   - saldo: as contas que entram no saldo; o cartão vira dívida (só o que é negativo) e a conta de investimento soma
//     no total investido, com a carteira, o Open Finance e as reservas (sem contar duas vezes o mesmo ativo);
//   - patrimônio: saldo + investido − dívida dos cartões − financiamentos − empréstimos;
//   - o mês: receitas e despesas, gastos por categoria (no pai), o fluxo dos 12 meses até ele, os orçamentos (6 no
//     card, e os estourados à parte, do pior para o melhor), as próximas contas, as últimas e as futuras transações, as
//     faturas que fecham no mês, os gastos de emergência e os avisos.
func (s *Servico) Painel(ctx context.Context, ano, mes int) (PainelDto, error) {
	hoje := relogio.Hoje(s.relogio)
	inicio := tipos.NovaData(ano, time.Month(mes), 1)
	fim := relogio.UltimoDiaMes(ano, time.Month(mes))

	saldos, err := contas.Saldos(ctx, s.db)
	if err != nil {
		return PainelDto{}, err
	}
	lista, err := s.relatorios.ContasNoSaldo(ctx)
	if err != nil {
		return PainelDto{}, err
	}
	saldoContas, dividaCartoes, investidoManual := tipos.Zero, tipos.Zero, tipos.Zero
	for _, c := range lista {
		saldo := tipos.Zero
		if v, ok := saldos[c.Id]; ok {
			saldo = v
		}
		switch c.Tipo {
		case tipoContaCartao:
			dividaCartoes = tipos.Somar(dividaCartoes, tipos.Maior(tipos.Zero, tipos.Subtrair(tipos.Zero, saldo)))
		case tipoContaInvestimento:
			investidoManual = tipos.Somar(investidoManual, saldo)
		default:
			saldoContas = tipos.Somar(saldoContas, saldo)
		}
	}
	composicao, err := s.carteira.Composicao(ctx)
	if err != nil {
		return PainelDto{}, err
	}
	totalInvestido := tipos.Somar(investidoManual, composicao.Total())
	credito, err := s.emprestimos.Totais(ctx)
	if err != nil {
		return PainelDto{}, err
	}
	patrimonio := tipos.Subtrair(tipos.Subtrair(tipos.Subtrair(tipos.Somar(saldoContas, totalInvestido), dividaCartoes),
		credito.SaldoFinanciamentos), credito.SaldoEmprestimos)

	p := PainelDto{SaldoContas: saldoContas, DividaCartoes: dividaCartoes, TotalInvestido: totalInvestido,
		PatrimonioLiquido: patrimonio, RendaVariavel: composicao.RendaVariavel(),
		PatrimonioSemRendaVariavel: tipos.Subtrair(patrimonio, composicao.RendaVariavel()),
		DividaFinanciamentos:       credito.SaldoFinanciamentos, DividaEmprestimos: credito.SaldoEmprestimos}
	if p.ReceitasMes, p.DespesasMes, err = s.relatorios.Totais(ctx, inicio, fim); err != nil {
		return PainelDto{}, err
	}
	if p.GastosPorCategoria, err = s.relatorios.TotaisPorCategoria(ctx, inicio, fim, transacoes.Despesa, true, false); err != nil {
		return PainelDto{}, err
	}
	if p.Evolucao, err = s.relatorios.FluxoMensal(ctx, inicio.SomarMeses(-11), fim); err != nil {
		return PainelDto{}, err
	}
	orcamentos, err := s.relatorios.OrcamentosDoMes(ctx, ano, mes)
	if err != nil {
		return PainelDto{}, err
	}
	if p.DiasProximosVencimentos, err = s.parametros.ObterInteiro(ctx, chaveDiasVencimentos); err != nil {
		return PainelDto{}, err
	}
	if p.ProximasContas, err = s.relatorios.ContasAgendadas(ctx, &p.DiasProximosVencimentos); err != nil {
		return PainelDto{}, err
	}
	if p.UltimasTransacoes, p.TransacoesFuturas, p.QuantidadeFuturas, err = extrato.TransacoesDoPainel(ctx, s.db, hoje,
		transacoesNoPainel); err != nil {
		return PainelDto{}, err
	}
	p.Orcamentos = orcamentos[:min(len(orcamentos), orcamentosNoPainel)]
	p.OrcamentosEstourados = estourados(orcamentos)
	if p.Alertas, err = s.alertas(ctx, p.ProximasContas, composicao.MoedasSemCotacao); err != nil {
		return PainelDto{}, err
	}
	if p.DespesasEmergenciaMes, err = s.emergencias(ctx, inicio, fim); err != nil {
		return PainelDto{}, err
	}
	if p.UltimaSincronizacao, err = configuracao.Novo(s.db, s.relogio).ObterData(ctx, chaveUltimaSincronizacao); err != nil {
		return PainelDto{}, err
	}
	doMes, err := faturas.DoMesDosCartoes(ctx, s.db, inicio, fim, hoje)
	if err != nil {
		return PainelDto{}, err
	}
	p.FaturasDoMes, p.TotalFaturasDoMes = []faturas.DoMes{}, tipos.Zero
	for _, f := range doMes {
		if f.ValorMes.Decimal().IsPositive() || f.FaturaId != nil {
			p.FaturasDoMes = append(p.FaturasDoMes, f)
			p.TotalFaturasDoMes = tipos.Somar(p.TotalFaturasDoMes, f.ValorMes)
		}
	}
	var desde tipos.Data
	if p.PoucoInformativas, desde, p.ExemploPoucoInformativa, err = s.poucoInformativas(ctx, hoje); err != nil {
		return PainelDto{}, err
	}
	p.PoucoInformativasDesde = &desde
	if p.CreditosCorretoraComoReceita, p.TotalCreditosCorretoraComoReceita, err = s.carteira.CreditosComoReceita(ctx); err != nil {
		return PainelDto{}, err
	}
	if p.TrocosAutomaticos, err = s.carteira.ResumoTrocosAutomaticos(ctx); err != nil {
		return PainelDto{}, err
	}
	return p, nil
}

// estourados são os orçamentos acima de 100%, do pior para o melhor (#206).
func estourados(orcamentos []relatorios.OrcamentoDto) []relatorios.OrcamentoDto {
	cem := tipos.DecimalDeInteiro(100).Decimal()
	lista := []relatorios.OrcamentoDto{}
	for _, o := range orcamentos {
		if o.Percentual.Decimal().GreaterThan(cem) {
			lista = append(lista, o)
		}
	}
	slices.SortStableFunc(lista, func(a, b relatorios.OrcamentoDto) int {
		return b.Percentual.Decimal().Cmp(a.Percentual.Decimal())
	})
	return lista
}

func (s *Servico) avisos(w http.ResponseWriter, r *http.Request) {
	a, err := s.Avisos(r.Context())
	if err != nil {
		s.erroInterno(w, r, "avisos", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, a)
}

// Avisos são os avisos e os orçamentos estourados do mês atual, para as notificações da barra do topo (#327): o mesmo
// que o painel calcula, sem o resto dele.
func (s *Servico) Avisos(ctx context.Context) (AvisosDto, error) {
	hoje := relogio.Hoje(s.relogio)
	a := AvisosDto{Ano: hoje.Ano(), Mes: int(hoje.Mes())}
	orcamentos, err := s.relatorios.OrcamentosDoMes(ctx, a.Ano, a.Mes)
	if err != nil {
		return AvisosDto{}, err
	}
	a.OrcamentosEstourados = estourados(orcamentos)
	dias, err := s.parametros.ObterInteiro(ctx, chaveDiasVencimentos)
	if err != nil {
		return AvisosDto{}, err
	}
	proximas, err := s.relatorios.ContasAgendadas(ctx, &dias)
	if err != nil {
		return AvisosDto{}, err
	}
	composicao, err := s.carteira.Composicao(ctx)
	if err != nil {
		return AvisosDto{}, err
	}
	var desde tipos.Data
	if a.PoucoInformativas, desde, a.ExemploPoucoInformativa, err = s.poucoInformativas(ctx, hoje); err != nil {
		return AvisosDto{}, err
	}
	a.PoucoInformativasDesde = &desde
	if a.Alertas, err = s.alertas(ctx, proximas, composicao.MoedasSemCotacao); err != nil {
		return AvisosDto{}, err
	}
	return a, nil
}

// emergencias é o valor absoluto das saídas do mês marcadas como emergência (sem as ignoradas e as transferências),
// somadas como o SumAsync do EF Core: no formato do banco ("0.0" sem nenhuma).
func (s *Servico) emergencias(ctx context.Context, inicio, fim tipos.Data) (tipos.Decimal, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT "Valor" FROM "Transacao" WHERE "Data" >= ? AND "Data" <= ?
		AND "Emergencia" AND NOT "IgnorarRelatorios" AND "Tipo" <> ? ORDER BY "Id"`, inicio, fim, transacoes.Transferencia)
	if err != nil {
		return tipos.Zero, fmt.Errorf("ler as emergências do mês: %w", err)
	}
	defer linhas.Close()
	var valores []tipos.Decimal
	for linhas.Next() {
		var v tipos.Decimal
		if err := linhas.Scan(&v); err != nil {
			return tipos.Zero, fmt.Errorf("ler emergência do mês: %w", err)
		}
		if v.Decimal().IsNegative() {
			valores = append(valores, v)
		}
	}
	if err := linhas.Err(); err != nil {
		return tipos.Zero, fmt.Errorf("ler as emergências do mês: %w", err)
	}
	return tipos.DeDecimal(relatorios.SomaDoSumAsync(valores).Decimal().Abs()), nil
}

// poucoInformativas é o PoucoInformativasAsync (#9): as transações dos últimos 90 dias com descrição pouco informativa
// ainda não revisadas: quantas, desde quando e um exemplo (a descrição mais repetida, sem diferença de maiúsculas e
// sem os espaços das pontas; no empate, a que apareceu primeiro), com até 60 chars.
func (s *Servico) poucoInformativas(ctx context.Context, hoje tipos.Data) (int, tipos.Data, *string, error) {
	desde := hoje.SomarDias(-(diasPoucoInformativas - 1))
	minimo, err := s.parametros.ObterInteiro(ctx, poucoinformativa.ChaveMinimoLetras)
	if err != nil {
		return 0, desde, nil, err
	}
	// O C# lê sem ORDER BY: o SQLite percorre o período pelo índice IX_Transacao_Data (a data mais nova primeiro).
	linhas, err := s.db.QueryContext(ctx, `SELECT t."Descricao" FROM "Transacao" AS t WHERE t."Data" >= ? AND t."Data" <= ?
		AND `+poucoinformativa.Candidatas+` ORDER BY t."Data" DESC, t."Id"`, desde, hoje)
	if err != nil {
		return 0, desde, nil, fmt.Errorf("ler as descrições recentes: %w", err)
	}
	defer linhas.Close()
	var lista []string
	for linhas.Next() {
		var d string
		if err := linhas.Scan(&d); err != nil {
			return 0, desde, nil, fmt.Errorf("ler descrição recente: %w", err)
		}
		if poucoinformativa.EhPoucoInformativa(d, minimo) {
			lista = append(lista, d)
		}
	}
	if err := linhas.Err(); err != nil {
		return 0, desde, nil, fmt.Errorf("ler as descrições recentes: %w", err)
	}
	if len(lista) == 0 {
		return 0, desde, nil, nil
	}
	var ordem []string
	contagem, primeira := map[string]int{}, map[string]string{}
	for _, d := range lista {
		k := texto.MaiusculasInvariantes(strings.TrimSpace(d))
		if _, ok := contagem[k]; !ok {
			ordem = append(ordem, k)
			primeira[k] = strings.TrimSpace(d)
		}
		contagem[k]++
	}
	melhor := ordem[0]
	for _, k := range ordem[1:] {
		if contagem[k] > contagem[melhor] {
			melhor = k
		}
	}
	exemplo := cortarUtf16(primeira[melhor], tamanhoDoExemplo)
	return len(lista), desde, &exemplo, nil
}

// cortarUtf16 corta o texto em até n chars UTF-16 (o s[..n] do C#).
func cortarUtf16(s string, n int) string {
	unidades := utf16.Encode([]rune(s))
	if len(unidades) <= n {
		return s
	}
	return string(utf16.Decode(unidades[:n]))
}

// alertas são os avisos do painel e das notificações (#327), na ordem do C#.
func (s *Servico) alertas(ctx context.Context, proximas []relatorios.ContaAgendadaDto, moedasSemCotacao []string) ([]string, error) {
	alertas := []string{}
	for _, p := range proximas {
		if p.DiasParaVencer < 0 {
			alertas = append(alertas, fmt.Sprintf("Conta \"%s\" vencida desde %02d/%02d.", p.Descricao,
				p.ProximoVencimento.Dia(), int(p.ProximoVencimento.Mes())))
		}
	}
	if len(moedasSemCotacao) > 0 {
		alertas = append(alertas, "Investimentos em "+strings.Join(moedasSemCotacao, ", ")+
			" ficaram fora do patrimônio: não há cotação de câmbio guardada para essa moeda.")
	}
	outraMoeda, err := s.contasEmOutraMoeda(ctx)
	if err != nil {
		return nil, err
	}
	alertas = append(alertas, outraMoeda...)

	// #158 (decisão do usuário): no painel, só "não veio" e "cartão arquivado com cobranças"; os outros, na tela.
	cobrancas, err := s.cobrancas.Avisos(ctx, relogio.Hoje(s.relogio))
	if err != nil {
		return nil, err
	}
	for _, a := range cobrancas {
		if a.Tipo == "nao-veio" || a.Tipo == "cartao-arquivado" {
			alertas = append(alertas, a.Texto+" Veja em Contas a pagar.")
		}
	}

	listaDeReservas, err := s.reservas.Listar(ctx)
	if err != nil {
		return nil, err
	}
	var aConferir []string
	for _, r := range listaDeReservas {
		if r.PodeTerSaldoAnterior || r.ConferenciaAntiga {
			aConferir = append(aConferir, r.Nome)
		}
	}
	if len(aConferir) > 0 {
		alertas = append(alertas, "Confira o saldo da(s) reserva(s) "+strings.Join(aConferir, ", ")+
			" em Investimentos › Reservas: o valor estimado pelo extrato pode estar diferente do real.")
	}

	conexoes, err := s.avisosDasConexoes(ctx)
	if err != nil {
		return nil, err
	}
	alertas = append(alertas, conexoes...)

	erros, err := s.errosAbertos(ctx)
	if err != nil {
		return nil, err
	}
	switch {
	case erros == 1:
		alertas = append(alertas, "1 erro do sistema nos últimos 7 dias. Veja em Configurações › Erros.")
	case erros > 1:
		alertas = append(alertas, strconv.Itoa(erros)+" erros do sistema nos últimos 7 dias. Veja em Configurações › Erros.")
	}

	atrasados, err := s.contratosEmAtraso(ctx)
	if err != nil {
		return nil, err
	}
	return append(alertas, atrasados...), nil
}

// contasEmOutraMoeda: as contas são somadas como reais (#172, ADR 0008); a conta em outra moeda entraria sem conversão.
func (s *Servico) contasEmOutraMoeda(ctx context.Context) ([]string, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT "Nome", "Moeda", "IncluirNoSaldo" FROM "Conta"
		WHERE NOT "Arquivada" AND "Moeda" <> 'BRL' ORDER BY "Id"`)
	if err != nil {
		return nil, fmt.Errorf("ler as contas em outra moeda: %w", err)
	}
	defer linhas.Close()
	var lista []string
	for linhas.Next() {
		var nome, moeda string
		var noSaldo bool
		if err := linhas.Scan(&nome, &moeda, &noSaldo); err != nil {
			return nil, fmt.Errorf("ler conta em outra moeda: %w", err)
		}
		texto := "A conta " + nome + " é em " + moeda + ", mas o site trata as contas como reais, sem conversão"
		if noSaldo {
			texto += ": o saldo dela entra no total como se fosse em reais. Desmarque \"Incluir no saldo\" em Contas e cartões."
		} else {
			texto += "."
		}
		lista = append(lista, texto+" As transações dela entram nos relatórios do mesmo jeito.")
	}
	return lista, linhas.Err()
}

// avisosDasConexoes: o consentimento do Open Finance que vence em breve (ou já venceu), a falha da última
// sincronização e a conexão que espera uma ação no Meu Pluggy, das conexões ativas.
func (s *Servico) avisosDasConexoes(ctx context.Context) ([]string, error) {
	dias, err := s.parametros.ObterInteiro(ctx, chaveDiasConsentimento)
	if err != nil {
		return nil, err
	}
	agora := s.relogio.Agora()
	limite := agora.AddDate(0, 0, dias)
	nomes, err := contas.NomesDasConexoes(ctx, s.db)
	if err != nil {
		return nil, err
	}
	linhas, err := s.db.QueryContext(ctx, `SELECT "Id", "ConsentimentoExpiraEm", "UltimoErro", "Status" FROM "ConexaoBancaria"
		WHERE "Ativa" ORDER BY "Id"`)
	if err != nil {
		return nil, fmt.Errorf("ler as conexões: %w", err)
	}
	defer linhas.Close()
	var lista []string
	for linhas.Next() {
		var id int64
		var expira *tipos.DataHora
		var ultimoErro, status *string
		if err := linhas.Scan(&id, &expira, &ultimoErro, &status); err != nil {
			return nil, fmt.Errorf("ler conexão: %w", err)
		}
		nome, ok := nomes[id]
		if !ok {
			nome = "Instituição"
		}
		if expira != nil && !expira.Tempo().After(limite) {
			if !expira.Tempo().After(agora) {
				lista = append(lista, "O consentimento do Open Finance de "+nome+" expirou. Renove no Meu Pluggy.")
			} else {
				lista = append(lista, "O consentimento do Open Finance de "+nome+" expira em "+
					relogio.ParaHoraLocal(expira.Tempo()).Format("02/01/2006")+". Renove no Meu Pluggy.")
			}
		}
		switch {
		case ultimoErro != nil && *ultimoErro != "":
			lista = append(lista, "Falha na última sincronização de "+nome+". Veja em Configurações › Open Finance.")
		case status != nil && (*status == "LOGIN_ERROR" || *status == "WAITING_USER_ACTION"):
			lista = append(lista, nome+" precisa de uma ação sua no Meu Pluggy para voltar a sincronizar.")
		}
	}
	return lista, linhas.Err()
}

// errosAbertos: os erros (não os avisos) da última semana ainda não marcados como resolvidos (#11).
func (s *Servico) errosAbertos(ctx context.Context) (int, error) {
	var n int
	desde := tipos.NovaDataHora(s.relogio.Agora().AddDate(0, 0, -7))
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM "ErroSistema" WHERE NOT "Resolvido" AND "Nivel" >= ?
		AND "UltimaEm" >= ?`, nivelErro, desde).Scan(&n); err != nil {
		return 0, fmt.Errorf("contar os erros abertos: %w", err)
	}
	return n, nil
}

// contratosEmAtraso: os contratos em aberto das conexões ativas com parcela em atraso (o saldo devedor é conferido em
// Go: dinheiro é texto no banco).
func (s *Servico) contratosEmAtraso(ctx context.Context) ([]string, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT e."Produto", e."Natureza", e."ParcelasEmAtraso", e."SaldoDevedor"
		FROM "Emprestimo" e JOIN "ConexaoBancaria" c ON c."Id" = e."ConexaoBancariaId"
		WHERE c."Ativa" AND e."ParcelasEmAtraso" > 0 ORDER BY e."Id"`)
	if err != nil {
		return nil, fmt.Errorf("ler os contratos em atraso: %w", err)
	}
	defer linhas.Close()
	var lista []string
	for linhas.Next() {
		var produto string
		var natureza, atraso int64
		var saldo tipos.Decimal
		if err := linhas.Scan(&produto, &natureza, &atraso, &saldo); err != nil {
			return nil, fmt.Errorf("ler contrato em atraso: %w", err)
		}
		if !saldo.Decimal().IsPositive() {
			continue
		}
		tipo := "Empréstimo"
		if natureza == emprestimos.NaturezaFinanciamento {
			tipo = "Financiamento"
		}
		lista = append(lista, fmt.Sprintf("%s \"%s\" com %d parcela(s) em atraso.", tipo, produto, atraso))
	}
	return lista, linhas.Err()
}
